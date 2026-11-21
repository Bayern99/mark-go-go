package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func luma(tt *ui.Tester, x, y int) uint32 {
	r, g, b, _ := tt.Image().At(x, y).RGBA()
	return r + g + b
}

func TestRaisedSurfaceIsLighterAtTheTopAndCastsAShadow(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Chrome).Padding(40).Children(func() {
			portrait.Materials.Raised.Apply(ui.Row(c).Size(120, 60).Radius(8))
		})
	}, 200, 160)
	tt.Frame()
	if top, bottom := luma(tt, 100, 43), luma(tt, 100, 96); top <= bottom {
		t.Fatalf("top %d not lighter than bottom %d", top, bottom)
	}
	if below, chrome := luma(tt, 100, 102), luma(tt, 100, 150); below >= chrome {
		t.Fatalf("no shadow under the surface: %d vs chrome %d", below, chrome)
	}
}
