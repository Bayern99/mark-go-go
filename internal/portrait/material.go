package portrait

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type Shadow struct {
	Inset              bool
	X, Y, Blur, Spread float32
	Color              ui.Color
}

type Shadows []Shadow

func (s Shadows) Apply(e ui.Element) ui.Element {
	for i := len(s) - 1; i >= 0; i-- {
		l := s[i]
		if l.Inset {
			e.InsetShadow(l.X, l.Y, l.Blur, l.Spread, l.Color)
		} else {
			e.Shadow(l.X, l.Y, l.Blur, l.Spread, l.Color)
		}
	}
	return e
}

type Surface struct {
	From, To ui.Color
	Shadows  Shadows
}

func (s Surface) Apply(e ui.Element) ui.Element {
	if s.From == s.To {
		e.Background(s.From)
	} else {
		e.Gradient(s.From, s.To, 180)
	}
	e.NoShadow()
	return s.Shadows.Apply(e)
}

type Glass struct {
	Tint                       ui.Color
	Blur, Saturate, Brightness float32
	Highlight                  ui.RadialGradient
	Rim                        Shadows
}

func (g Glass) Apply(e ui.Element) ui.Element {
	e.Material(g).NoShadow()
	return g.Rim.Apply(e)
}

func (g Glass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if prefs := p.Preferences(); prefs.ReduceTransparency || prefs.HighContrast {
		p.FillRadii(box, radii, Colors.Paper)
		return
	}
	glass.Blur{Radius: g.Blur}.PaintMaterial(p, box, radii)
	p.FillRadii(box, radii, g.Tint)
	if len(g.Highlight.Stops) > 0 {
		p.FillRadialGradient(box, g.Highlight, radii[0])
	}
}

type MaterialSet struct {
	Raised, RaisedHover, RaisedPress                   Surface
	AccentRaised, AccentRaisedHover, AccentRaisedPress Surface
	Row, Sunken, Track, Sheet, Pop, Page               Surface
	Glass, GlassOverlay                                Glass
	GlassSelected, GlassHover                          ui.Color
}

func in(x, y, blur, spread float32, c ui.Color) Shadow {
	return Shadow{Inset: true, X: x, Y: y, Blur: blur, Spread: spread, Color: c}
}
func out(x, y, blur, spread float32, c ui.Color) Shadow {
	return Shadow{X: x, Y: y, Blur: blur, Spread: spread, Color: c}
}

func flat(c ui.Color, shadows ...Shadow) Surface { return Surface{c, c, shadows} }

var Materials MaterialSet

func materialsFor(p Palette, dark bool) MaterialSet {
	glow := func(a float32) ui.Color { return p.Accent.Alpha(a) }
	edge := func(a float32) ui.Color { return p.Accent.Mix(black(1), 0.35).Alpha(a) }
	if !dark {
		return MaterialSet{
			Raised: flat(p.Paper,
				out(0, 0, 0, 0.5, black(0.12)), out(0, 1, 2, 0, black(0.05)), out(0, 2, 6, -2, black(0.04))),
			RaisedHover: flat(p.Paper,
				out(0, 0, 0, 0.5, black(0.16)), out(0, 1, 3, 0, black(0.07)), out(0, 4, 10, -3, black(0.07))),
			RaisedPress: flat(p.Paper.Mix(p.Ink, 0.07),
				out(0, 0, 0, 0.5, black(0.18)), out(0, 0.5, 1, 0, black(0.03)), out(0, 0, 0, 0, ui.Transparent)),
			AccentRaised: flat(p.Accent,
				in(0, 1, 0, 0, white(0.18)), out(0, 0, 0, 0.5, edge(0.5)), out(0, 1, 2, 0, glow(0.25)), out(0, 4, 12, -4, glow(0.35))),
			AccentRaisedHover: flat(p.Accent.Mix(p.OnAccent, 0.08),
				in(0, 1, 0, 0, white(0.22)), out(0, 0, 0, 0.5, edge(0.5)), out(0, 2, 4, 0, glow(0.28)), out(0, 6, 16, -4, glow(0.42))),
			AccentRaisedPress: flat(p.Accent.Mix(black(1), 0.1),
				in(0, 1, 2, 0, black(0.22)), out(0, 0, 0, 0.5, edge(0.55)), out(0, 0.5, 1, 0, glow(0.2)), out(0, 0, 0, 0, ui.Transparent)),
			Row:    flat(black(0.055)),
			Sunken: flat(black(0.05), in(0, 0, 0, 0.5, black(0.05))),
			Track:  flat(black(0.055)),
			Sheet: flat(p.Paper,
				out(0, 0, 0, 0.5, black(0.08)), out(0, 1, 2, 0, black(0.04))),
			Pop: flat(p.Paper,
				out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 14, 0, black(0.08)), out(0, 18, 44, -10, black(0.16))),
			Page: flat(p.Paper,
				out(0, 0, 0, 0.5, black(0.10)), out(0, 1, 2, 0, black(0.05)), out(0, 6, 20, -6, black(0.10))),
			Glass: Glass{Tint: white(0.70), Blur: 20, Saturate: 1.8, Brightness: 1, Rim: Shadows{
				in(0, 0, 0, 0.5, white(0.55)), out(0, 0, 0, 0.5, black(0.08)), out(0, 8, 32, -8, black(0.12))}},
			GlassOverlay: Glass{Tint: white(0.85), Blur: 32, Saturate: 1.8, Brightness: 1, Rim: Shadows{
				in(0, 0, 0, 0.5, white(0.60)), out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 12, 0, black(0.06)), out(0, 24, 60, -12, black(0.22))}},
			GlassSelected: white(0.85),
			GlassHover:    white(0.50),
		}
	}
	lift := func(t float32) ui.Color { return p.Paper.Mix(p.Ink, t) }
	return MaterialSet{
		Raised: flat(lift(0.07),
			out(0, 0, 0, 0.5, white(0.10)), out(0, 1, 2, 0, black(0.30)), out(0, 2, 6, -2, black(0.24))),
		RaisedHover: flat(lift(0.10),
			out(0, 0, 0, 0.5, white(0.14)), out(0, 1, 3, 0, black(0.34)), out(0, 4, 10, -3, black(0.30))),
		RaisedPress: flat(lift(0.04),
			out(0, 0, 0, 0.5, white(0.12)), out(0, 0.5, 1, 0, black(0.20)), out(0, 0, 0, 0, ui.Transparent)),
		AccentRaised: flat(p.Accent,
			in(0, 1, 0, 0, white(0.24)), out(0, 0, 0, 0.5, edge(0.6)), out(0, 1, 2, 0, black(0.30)), out(0, 4, 12, -4, glow(0.18))),
		AccentRaisedHover: flat(p.Accent.Mix(p.OnAccent, 0.08),
			in(0, 1, 0, 0, white(0.28)), out(0, 0, 0, 0.5, edge(0.6)), out(0, 2, 4, 0, black(0.34)), out(0, 6, 16, -4, glow(0.24))),
		AccentRaisedPress: flat(p.Accent.Mix(black(1), 0.1),
			in(0, 1, 2, 0, black(0.22)), out(0, 0, 0, 0.5, edge(0.65)), out(0, 0.5, 1, 0, black(0.20)), out(0, 0, 0, 0, ui.Transparent)),
		Row:    flat(white(0.075)),
		Sunken: flat(black(0.22), in(0, 0, 0, 0.5, white(0.08))),
		Track:  flat(white(0.07)),
		Sheet: flat(p.Paper,
			out(0, 0, 0, 0.5, white(0.10)), out(0, 1, 2, 0, black(0.24))),
		Pop: flat(lift(0.04),
			out(0, 0, 0, 0.5, white(0.12)), out(0, 4, 14, 0, black(0.30)), out(0, 18, 44, -10, black(0.50))),
		Page: flat(p.Paper,
			out(0, 0, 0, 0.5, white(0.09)), out(0, 1, 2, 0, black(0.30)), out(0, 6, 20, -6, black(0.36))),
		Glass: Glass{Tint: p.Paper.Alpha(0.72), Blur: 20, Saturate: 1.6, Brightness: 1, Rim: Shadows{
			in(0, 0, 0, 0.5, white(0.10)), out(0, 0, 0, 0.5, black(0.40)), out(0, 8, 32, -8, black(0.40))}},
		GlassOverlay: Glass{Tint: lift(0.03).Alpha(0.86), Blur: 32, Saturate: 1.6, Brightness: 1, Rim: Shadows{
			in(0, 0, 0, 0.5, white(0.12)), out(0, 0, 0, 0.5, black(0.50)), out(0, 4, 12, 0, black(0.24)), out(0, 24, 60, -12, black(0.55))}},
		GlassSelected: p.Paper.Alpha(0.85),
		GlassHover:    white(0.06),
	}
}

func Opaque(c *ui.Context) bool {
	p := c.Preferences()
	return p.ReduceTransparency || p.HighContrast
}
