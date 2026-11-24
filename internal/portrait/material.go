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

var (
	pressed    = ui.Hex("#ededf1")
	accentGlow = func(a float32) ui.Color { return Colors.Accent.Alpha(a) }
	accentEdge = func(a float32) ui.Color { return Colors.Accent.Mix(black(1), 0.35).Alpha(a) }
)

var Materials = MaterialSet{
	Raised: flat(Colors.Paper,
		out(0, 0, 0, 0.5, black(0.12)), out(0, 1, 2, 0, black(0.05)), out(0, 2, 6, -2, black(0.04))),
	RaisedHover: flat(Colors.Paper,
		out(0, 0, 0, 0.5, black(0.16)), out(0, 1, 3, 0, black(0.07)), out(0, 4, 10, -3, black(0.07))),
	RaisedPress: flat(pressed,
		out(0, 0, 0, 0.5, black(0.18)), out(0, 0.5, 1, 0, black(0.03)), out(0, 0, 0, 0, ui.Transparent)),
	AccentRaised: flat(Colors.Accent,
		in(0, 1, 0, 0, white(0.18)), out(0, 0, 0, 0.5, accentEdge(0.5)), out(0, 1, 2, 0, accentGlow(0.25)), out(0, 4, 12, -4, accentGlow(0.35))),
	AccentRaisedHover: flat(Colors.Accent.Mix(Colors.OnAccent, 0.08),
		in(0, 1, 0, 0, white(0.22)), out(0, 0, 0, 0.5, accentEdge(0.5)), out(0, 2, 4, 0, accentGlow(0.28)), out(0, 6, 16, -4, accentGlow(0.42))),
	AccentRaisedPress: flat(Colors.Accent.Mix(black(1), 0.1),
		in(0, 1, 2, 0, black(0.22)), out(0, 0, 0, 0.5, accentEdge(0.55)), out(0, 0.5, 1, 0, accentGlow(0.2)), out(0, 0, 0, 0, ui.Transparent)),
	Row:    flat(black(0.055)),
	Sunken: flat(black(0.05), in(0, 0, 0, 0.5, black(0.05))),
	Track:  flat(black(0.055)),
	Sheet: flat(Colors.Paper,
		out(0, 0, 0, 0.5, black(0.08)), out(0, 1, 2, 0, black(0.04))),
	Pop: flat(Colors.Paper,
		out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 14, 0, black(0.08)), out(0, 18, 44, -10, black(0.16))),
	Page: flat(Colors.Paper,
		out(0, 0, 0, 0.5, black(0.10)), out(0, 1, 2, 0, black(0.05)), out(0, 6, 20, -6, black(0.10))),
	Glass: Glass{Tint: white(0.70), Blur: 20, Saturate: 1.8, Brightness: 1, Rim: Shadows{
		in(0, 0, 0, 0.5, white(0.55)), out(0, 0, 0, 0.5, black(0.08)), out(0, 8, 32, -8, black(0.12))}},
	GlassOverlay: Glass{Tint: white(0.85), Blur: 32, Saturate: 1.8, Brightness: 1, Rim: Shadows{
		in(0, 0, 0, 0.5, white(0.60)), out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 12, 0, black(0.06)), out(0, 24, 60, -12, black(0.22))}},
	GlassSelected: white(0.85),
	GlassHover:    white(0.50),
}

func Opaque(c *ui.Context) bool {
	p := c.Preferences()
	return p.ReduceTransparency || p.HighContrast
}
