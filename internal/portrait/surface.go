package portrait

import "github.com/egoist/mygo/ui"

// Mix returns the surface t of the way from s to o, as CSS interpolates a
// gradient's stops and a box-shadow list layer by layer.
func (s Surface) Mix(o Surface, t float32) Surface {
	return Surface{From: mix(s.From, o.From, t), To: mix(s.To, o.To, t), Shadows: s.Shadows.Mix(o.Shadows, t)}
}

// Mix returns the list t of the way from s to o. A layer missing from one
// list is a transparent one at no offset; layers of different kinds
// (inset and outer) switch halfway, as in CSS.
func (s Shadows) Mix(o Shadows, t float32) Shadows {
	out := make(Shadows, max(len(s), len(o)))
	for i := range out {
		var a, b Shadow
		switch {
		case i >= len(s):
			b = o[i]
			a = Shadow{Inset: b.Inset, Color: ui.Transparent}
		case i >= len(o):
			a = s[i]
			b = Shadow{Inset: a.Inset, Color: ui.Transparent}
		default:
			a, b = s[i], o[i]
		}
		if a.Inset != b.Inset {
			out[i] = a
			if t >= 0.5 {
				out[i] = b
			}
			continue
		}
		lerp := func(x, y float32) float32 { return x + (y-x)*t }
		out[i] = Shadow{Inset: a.Inset, X: lerp(a.X, b.X), Y: lerp(a.Y, b.Y), Blur: lerp(a.Blur, b.Blur), Spread: lerp(a.Spread, b.Spread), Color: mix(a.Color, b.Color, t)}
	}
	return out
}

// mix blends two colors; a transparent end takes the other's hue, so a
// fade to nothing does not pass through black.
func mix(a, b ui.Color, t float32) ui.Color {
	if a.A == 0 {
		a.R, a.G, a.B = b.R, b.G, b.B
	}
	if b.A == 0 {
		b.R, b.G, b.B = a.R, a.G, a.B
	}
	return a.Mix(b, t)
}
