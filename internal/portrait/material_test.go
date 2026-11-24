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

func TestRaisedSurfaceIsFlatPaperAndCastsAShadow(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Chrome).Padding(40).Children(func() {
			portrait.Materials.Raised.Apply(ui.Row(c).Size(120, 60).Radius(8))
		})
	}, 200, 160)
	tt.Frame()
	if top, bottom := luma(tt, 100, 43), luma(tt, 100, 96); top != bottom {
		t.Fatalf("top %d and bottom %d differ; the raised surface is one flat fill", top, bottom)
	}
	if face, chrome := luma(tt, 100, 70), luma(tt, 100, 150); face <= chrome {
		t.Fatalf("raised face %d is not lighter than the chrome %d", face, chrome)
	}
	if below, chrome := luma(tt, 100, 101), luma(tt, 100, 150); below >= chrome {
		t.Fatalf("no shadow under the surface: %d vs chrome %d", below, chrome)
	}
}

func TestPageIsPaperFloatingOverEveryScenery(t *testing.T) {
	eachLook(t, func(t *testing.T, space portrait.Space, _ bool) {
		tt := ui.NewTester(func(c *ui.Context) {
			space.Paint(ui.Column(c).Fill().Padding(40), false).Children(func() {
				portrait.Materials.Page.Apply(ui.Row(c).Size(120, 60).Radius(portrait.Shell.PageRadius))
			})
		}, 200, 160)
		tt.Frame()
		r, g, b, _ := tt.Image().At(100, 70).RGBA()
		if p := portrait.Colors.Paper; r>>8 != uint32(p.R) || g>>8 != uint32(p.G) || b>>8 != uint32(p.B) {
			t.Errorf("%s: page face is %d,%d,%d, want opaque paper %+v", space.Name, r>>8, g>>8, b>>8, p)
		}
		if below, scenery := luma(tt, 100, 102), luma(tt, 100, 150); below >= scenery {
			t.Errorf("%s: no shadow under the page: %d vs scenery %d", space.Name, below, scenery)
		}
	})
}
