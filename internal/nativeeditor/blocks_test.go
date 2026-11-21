package nativeeditor_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
)

func TestSpecialBlocksCarryTheirPortraitSurfaces(t *testing.T) {
	const src = "> [!NOTE]\n> Remember this.\n\n> [!CAUTION]\n> Careful here.\n\n$$\nx^2\n$$\n\nText[^a].\n\n[^a]: A footnote.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 900)
	tt.Frame()
	img := tt.Image()
	sample := func(name string) (r, g, b int) {
		t.Helper()
		info, ok := tt.Access(name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		c := img.RGBAAt(int(info.Bounds.X+info.Bounds.W/2), int(info.Bounds.Y+4))
		return int(c.R), int(c.G), int(c.B)
	}
	if r, g, b := sample("Note alert block"); !(g > r+4 && g < 255) || b == 255 {
		t.Errorf("note alert fill %d,%d,%d, want the soft accent", r, g, b)
	}
	if r, g, b := sample("Caution alert block"); !(r > g+4 && g < 255) {
		t.Errorf("caution alert fill %d,%d,%d, want the soft caution red", r, g, b)
	}
	if r, g, b := sample("math block"); !(r == g && b > r && b < 255) {
		t.Errorf("math block fill %d,%d,%d, want the code surface", r, g, b)
	}
	if r, g, b := sample("a block"); !(r == g && g == b && r < 255) {
		t.Errorf("footnote fill %d,%d,%d, want the neutral fill", r, g, b)
	}
}

func TestFencedCodeSitsOnTheCodeSurface(t *testing.T) {
	const src = "Before\n\n```go\nalpha()\nbeta()\n```\n\nAfter\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	code, _ := d.RectUTF16(utf16At(t, src, "beta"))
	plain, _ := d.RectUTF16(utf16At(t, src, "After"))
	at := func(r ui.Rect) [3]uint8 {
		c := tt.Image().RGBAAt(int(r.X-6), int(r.Y+r.H/2))
		return [3]uint8{c.R, c.G, c.B}
	}
	if got := at(code); got != [3]uint8{0xf6, 0xf6, 0xf8} {
		t.Errorf("beside code %v, want the code surface #f6f6f8", got)
	}
	if got := at(plain); got != [3]uint8{0xff, 0xff, 0xff} {
		t.Errorf("beside prose %v, want paper", got)
	}
}
