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

func TestMaterialHairlinesRemainVisibleAtOneAndTwoX(t *testing.T) {
	for _, tc := range []struct {
		name    string
		surface portrait.Surface
	}{
		{"Raised", portrait.Materials.Raised},
		{"Pop", portrait.Materials.Pop},
		{"Sheet", portrait.Materials.Sheet},
		{"Page", portrait.Materials.Page},
		{"Sunken", portrait.Materials.Sunken},
	} {
		for _, scale := range []float32{1, 2} {
			t.Run(tc.name, func(t *testing.T) {
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Fill().Background(portrait.Colors.Chrome).Padding(40).Children(func() {
						tc.surface.Apply(ui.Row(c).Size(120, 60).Radius(8)).Label(tc.name)
					})
				}, 200, 160)
				tt.SetScale(scale)
				tt.Frame()
				info, ok := tt.Access(tc.name)
				if !ok || info.Bounds.W != 120 || info.Bounds.H != 60 {
					t.Fatalf("surface bounds = %+v, found %v", info.Bounds, ok)
				}
				x, y := int(info.Bounds.X*scale), int((info.Bounds.Y+info.Bounds.H/2)*scale)
				edge := tt.Image().RGBAAt(x, y)
				face := tt.Image().RGBAAt(x+int(2*scale), y)
				chrome := tt.Image().RGBAAt(int(10*scale), y)
				if edge == face || edge == chrome {
					t.Fatalf("scale %v: edge %v, face %v, chrome %v; hairline is not visible", scale, edge, face, chrome)
				}
			})
		}
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
