package portrait_test

import (
	"image/color"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func glassOverStripes(g portrait.Glass, prefs ui.Preferences) *ui.Tester {
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() {
				for i := range 50 {
					bar := ui.Hex("#000000")
					if i%2 == 0 {
						bar = ui.Hex("#ff0000")
					}
					ui.Box(c).Width(4).FillHeight().Background(bar)
				}
			})
			g.Apply(ui.Box(c).Absolute().Top(40).Left(40).Size(120, 80).Radius(16))
		})
	}, 200, 160, ui.TesterOptions{Preferences: prefs})
	tt.Frame()
	return tt
}

func rgb(tt *ui.Tester, x, y int) [3]uint8 {
	c := color.NRGBAModel.Convert(tt.Image().At(x, y)).(color.NRGBA)
	return [3]uint8{c.R, c.G, c.B}
}

func TestGlassBlursAndTintsWhatIsBehindIt(t *testing.T) {
	for name, g := range map[string]portrait.Glass{"glass": portrait.Materials.Glass, "overlay": portrait.Materials.GlassOverlay} {
		t.Run(name, func(t *testing.T) {
			tt := glassOverStripes(g, ui.Preferences{})
			lo, hi := 255, 0
			for x := 90; x < 110; x++ {
				r := int(rgb(tt, x, 100)[0])
				lo, hi = min(lo, r), max(hi, r)
			}
			if hi-lo > 24 {
				t.Fatalf("stripes behind the glass are not blurred: red ranges %d..%d", lo, hi)
			}
			if c := rgb(tt, 100, 100); int(c[0]) <= int(c[1])+8 || c == [3]uint8{255, 255, 255} {
				t.Fatalf("content behind the glass does not show through: %v", c)
			}
		})
	}
}

func TestGlassIsOpaquePaperWhenTransparencyIsReducedOrContrastIncreased(t *testing.T) {
	for name, prefs := range map[string]ui.Preferences{"reduce transparency": {ReduceTransparency: true}, "high contrast": {HighContrast: true}} {
		for _, g := range []portrait.Glass{portrait.Materials.Glass, portrait.Materials.GlassOverlay} {
			tt := glassOverStripes(g, prefs)
			for x := 90; x < 110; x++ {
				if c := rgb(tt, x, 100); c != [3]uint8{255, 255, 255} {
					t.Fatalf("%s: glass at x=%d is %v, want opaque paper", name, x, c)
				}
			}
		}
	}
}
