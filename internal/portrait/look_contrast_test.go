package portrait

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLookSurfacesMeetWCAGAA verifies that the actual button Look surfaces
// (rest, hover, press) in controls.go/button.go meet WCAG AA contrast against
// their ink colors when composited over Paper.
func TestLookSurfacesMeetWCAGAA(t *testing.T) {
	paper := lookRGBA(Colors.Paper)
	for _, tc := range []struct {
		name string
		look Look
	}{
		{"Raised", Raised},
		{"Accent", Accent},
		{"Danger", Danger},
		{"DangerOutline", DangerOutline},
		{"Link", Link},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := looks[tc.look]
			for _, state := range []struct {
				name string
				surf Surface
				ink  ui.Color
			}{
				{"rest", f.rest, f.ink},
				{"hover", f.hover, f.hoverInk},
				{"press", f.press, f.hoverInk},
			} {
				for _, edge := range []struct {
					name  string
					color ui.Color
				}{
					{"from", state.surf.From},
					{"to", state.surf.To},
				} {
					bg := lookOver(lookRGBA(edge.color), paper)
					ratio := lookContrast(lookRGBA(state.ink), bg)
					if ratio < 4.5 {
						t.Errorf("%s %s (%s) contrast = %.4f, want >= 4.5", tc.name, state.name, edge.name, ratio)
					}
				}
			}
		})
	}
}

func lookRGBA(c ui.Color) [4]float64 {
	return [4]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255, float64(c.A) / 255}
}

func lookOver(fg, bg [4]float64) [4]float64 {
	a := fg[3] + bg[3]*(1-fg[3])
	if a == 0 {
		return [4]float64{}
	}
	var out [4]float64
	for i := 0; i < 3; i++ {
		out[i] = (fg[i]*fg[3] + bg[i]*bg[3]*(1-fg[3])) / a
	}
	out[3] = a
	return out
}

func lookContrast(fg, bg [4]float64) float64 {
	fg = lookOver(fg, bg)
	l1, l2 := lookLuminance(fg), lookLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func lookLuminance(c [4]float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}
