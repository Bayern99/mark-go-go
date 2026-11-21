package portrait

import "github.com/egoist/mygo/ui"

// Shadow is one CSS box-shadow layer.
type Shadow struct {
	Inset              bool
	X, Y, Blur, Spread float32
	Color              ui.Color
}

// Shadows lists layers in CSS order: the first paints on top.
type Shadows []Shadow

// Apply adds the layers to e. MyGo paints the first added layer at the
// bottom, so layers are added in reverse.
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

// Surface is a top-to-bottom fill with its shadow layers.
type Surface struct {
	From, To ui.Color
	Shadows  Shadows
}

// Apply paints the surface on e, replacing its fill and shadows.
func (s Surface) Apply(e ui.Element) ui.Element {
	if s.From == s.To {
		e.Background(s.From)
	} else {
		e.Gradient(s.From, s.To, 180)
	}
	e.NoShadow()
	return s.Shadows.Apply(e)
}

// Glass is a translucent material: tint over the blurred backdrop
// (backdrop-filter: blur saturate brightness), a radial highlight and a rim.
type Glass struct {
	Tint                       ui.Color
	Blur, Saturate, Brightness float32
	Highlight                  ui.RadialGradient
	Rim                        Shadows
}

// MaterialSet lists the Portrait materials.
type MaterialSet struct {
	Raised, RaisedHover, RaisedPress                   Surface
	AccentRaised, AccentRaisedHover, AccentRaisedPress Surface
	Row, Sunken, Track, Sheet, Pop                     Surface
	Glass, GlassOverlay                                Glass
	GlassSelected, GlassHover                          ui.Color
}

func in(x, y, blur, spread float32, c ui.Color) Shadow {
	return Shadow{Inset: true, X: x, Y: y, Blur: blur, Spread: spread, Color: c}
}
func out(x, y, blur, spread float32, c ui.Color) Shadow {
	return Shadow{X: x, Y: y, Blur: blur, Spread: spread, Color: c}
}

var (
	raised     = [2]ui.Color{ui.Hex("#ffffff"), ui.Hex("#f6f6f8")}
	deepGreen  = func(a float32) ui.Color { return ui.RGBA(18, 52, 45, a) }
	accentGlow = func(a float32) ui.Color { return ui.RGBA(47, 111, 98, a) }
	glassShine = ui.RadialGradient{Center: ui.Point{X: 0.12, Y: 0}, Stops: []ui.GradientStop{{Color: white(0.42), At: 0}, {Color: ui.Transparent, At: 0.56}}}
)

// Materials holds the Portrait materials.
var Materials = MaterialSet{
	Raised: Surface{raised[0], raised[1], Shadows{
		in(0, 1, 0, 0, white(0.94)), in(0, -0.5, 0, 0, black(0.05)), out(0, 0, 0, 0.5, black(0.13)), out(0, 1, 2, 0, black(0.055)), out(0, 3, 8, -3, black(0.075))}},
	RaisedHover: Surface{raised[0], raised[1], Shadows{
		in(0, 1, 0, 0, white(1)), in(0, -0.5, 0, 0, black(0.06)), out(0, 0, 0, 0.5, black(0.15)), out(0, 2, 4, 0, black(0.065)), out(0, 6, 14, -4, black(0.095))}},
	RaisedPress: Surface{ui.Hex("#ededf0"), ui.Hex("#f4f4f6"), Shadows{
		in(0, 1, 2, 0, black(0.1)), in(0, -0.5, 0, 0, white(0.85)), out(0, 0, 0, 0.5, black(0.15)), out(0, 0.5, 1, 0, black(0.04)), out(0, 0, 0, 0, ui.Transparent)}},
	AccentRaised: Surface{ui.Hex("#37796b"), ui.Hex("#2c6a5d"), Shadows{
		in(0, 1, 0, 0, white(0.24)), in(0, -1, 0, 0, black(0.18)), out(0, 0, 0, 0.5, deepGreen(0.42)), out(0, 1, 2, 0, deepGreen(0.16)), out(0, 4, 10, -3, accentGlow(0.34))}},
	AccentRaisedHover: Surface{ui.Hex("#3c8273"), ui.Hex("#2e6e60"), Shadows{
		in(0, 1, 0, 0, white(0.28)), in(0, -1, 0, 0, black(0.2)), out(0, 0, 0, 0.5, deepGreen(0.48)), out(0, 2, 4, 0, deepGreen(0.18)), out(0, 7, 16, -4, accentGlow(0.38))}},
	AccentRaisedPress: Surface{ui.Hex("#2a6357"), ui.Hex("#317063"), Shadows{
		in(0, 1, 2, 0, black(0.28)), in(0, -0.5, 0, 0, white(0.14)), out(0, 0, 0, 0.5, deepGreen(0.48)), out(0, 0.5, 1, 0, deepGreen(0.14)), out(0, 0, 0, 0, ui.Transparent)}},
	Row: Surface{raised[0], raised[1], Shadows{
		in(0, 1, 0, 0, white(0.92)), out(0, 0, 0, 0.5, black(0.095)), out(0, 1, 2, 0, black(0.06))}},
	Sunken: Surface{black(0.045), black(0.045), Shadows{
		in(0, 1, 2, 0, black(0.085)), in(0, 0, 0, 0.5, black(0.055)), out(0, 1, 0, 0, white(0.72))}},
	Track: Surface{black(0.045), black(0.045), Shadows{
		in(0, 1, 1.5, 0, black(0.045)), in(0, 0, 0, 0.5, black(0.055))}},
	Sheet: Surface{white(1), white(1), Shadows{
		out(0, 0, 0, 0.5, black(0.09)), out(0, 1, 3, 0, black(0.05))}},
	Pop: Surface{white(1), white(1), Shadows{
		out(0, 0, 0, 0.5, black(0.13)), out(0, 4, 12, 0, black(0.08)), out(0, 16, 36, -8, black(0.14))}},
	Glass: Glass{Tint: white(0.72), Blur: 14, Saturate: 1.6, Brightness: 1.02, Highlight: glassShine, Rim: Shadows{
		in(0, 1, 0, 0, white(0.96)), in(1, 0, 0, 0, white(0.68)), in(0, -0.5, 0, 0, black(0.03)), out(0, 0, 0, 0.5, black(0.075)), out(0, 2, 6, 0, black(0.035)), out(0, 12, 28, -8, black(0.07))}},
	GlassOverlay: Glass{Tint: white(0.86), Blur: 22, Saturate: 1.85, Brightness: 1.03, Highlight: glassShine, Rim: Shadows{
		in(0, 1, 0, 0, white(0.98)), in(1, 0, 0, 0, white(0.76)), in(0, -0.5, 0, 0, black(0.045)), out(0, 0, 0, 0.5, black(0.10)), out(0, 4, 12, 0, black(0.07)), out(0, 18, 40, -10, black(0.13))}},
	GlassSelected: white(0.92),
	GlassHover:    white(0.84),
}

// Opaque reports whether translucent materials must be replaced by opaque
// paper: the system asks for reduced transparency or increased contrast.
func Opaque(c *ui.Context) bool {
	p := c.Preferences()
	return p.ReduceTransparency || p.HighContrast
}
