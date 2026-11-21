package nativegenui_test

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func TestDEBUGb7c1Code(t *testing.T) {
	dir := os.Getenv("DEBUG_A4F2_DIR")
	if dir == "" {
		t.Skip()
	}
	for _, variant := range []string{"", "caption"} {
		attr := ""
		if variant != "" {
			attr = `,"variant":"` + variant + `"`
		}
		view, err := nativegenui.Open(`{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Use ` + "`printf`" + ` here"` + attr + `}]}}]}`)
		if err != nil {
			t.Fatal(err)
		}
		var reader nativegenui.Reading
		reader.Load(view)
		tt := ui.NewTester(reader.View, 520, 200)
		tt.Frame()
		img := tt.Image()
		f, _ := os.Create(filepath.Join(dir, "genui-"+variant+".png"))
		png.Encode(f, img)
		f.Close()
		cells, span := codeSpan(t, tt, "printf")
		fmt.Printf("[DEBUG-b7c1] variant=%q span=%d cells=%d\n", variant, span, len(cells))
		for y := 0; y < img.Bounds().Dy(); y++ {
			n, first, last := 0, -1, -1
			for x := 0; x < img.Bounds().Dx(); x++ {
				c := img.RGBAAt(x, y)
				r, g, b := int(c.R), int(c.G), int(c.B)
				if r > 225 && r < 250 && abs(r-g) < 3 && abs(g-b) < 3 {
					n++
					if first < 0 {
						first = x
					}
					last = x
				}
			}
			if n > 0 {
				fmt.Printf("[DEBUG-b7c1] variant=%q y=%d tinted=%d x=%d..%d\n", variant, y, n, first, last)
			}
		}
	}
}

func TestDEBUGb7c1Shape(t *testing.T) {
	if os.Getenv("DEBUG_A4F2_DIR") == "" {
		t.Skip()
	}
	for _, f := range ui.FontFamilies() {
		if f.Registered {
			fmt.Printf("[DEBUG-b7c1] registered %q\n", f.Name)
		}
	}
	for _, fam := range []string{"IBM Plex Mono", "IBM Plex Mono, IBM Plex Sans SC", "IBM Plex Sans", "IBM Plex Sans, IBM Plex Sans SC", "monospace", "nonexistent"} {
		l := ui.ShapeText("printf", ui.Font{Family: fam, Size: 12}, 0)
		w, h := l.Size()
		fmt.Printf("[DEBUG-b7c1] shape %q w=%.2f h=%.2f\n", fam, w, h)
	}
}

func TestDEBUGb7c1Rich(t *testing.T) {
	if os.Getenv("DEBUG_A4F2_DIR") == "" {
		t.Skip()
	}
	for _, base := range []string{"", "IBM Plex Sans"} {
		for _, fam := range []string{"IBM Plex Mono", "IBM Plex Mono, IBM Plex Sans SC", "monospace"} {
			l := ui.ShapeRichText([]ui.Span{{Text: "Use "}, {Text: "printf", Font: fam, Size: 12}}, ui.Font{Family: base, Size: 14}, 0)
			w, _ := l.Size()
			only := ui.ShapeRichText([]ui.Span{{Text: "printf", Font: fam, Size: 12}}, ui.Font{Family: base, Size: 14}, 0)
			w2, _ := only.Size()
			fmt.Printf("[DEBUG-b7c1] rich base=%q code=%q mixed=%.2f only=%.2f\n", base, fam, w, w2)
		}
	}
}
