package nativeeditor_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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
	if r, g, b := sample("Note alert block"); !(b > r+4 && b > g+4 && r < 255) {
		t.Errorf("note alert fill %d,%d,%d, want the soft accent blue", r, g, b)
	}
	if r, g, b := sample("Caution alert block"); !(r > g+4 && g < 255) {
		t.Errorf("caution alert fill %d,%d,%d, want the soft caution red", r, g, b)
	}
	if r, g, b := sample("math block"); [3]int{r, g, b} != [3]int{int(portrait.Colors.Code.R), int(portrait.Colors.Code.G), int(portrait.Colors.Code.B)} {
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
	if got, want := at(code), [3]uint8{portrait.Colors.Code.R, portrait.Colors.Code.G, portrait.Colors.Code.B}; got != want {
		t.Errorf("beside code %v, want the code surface %v", got, want)
	}
	if got := at(plain); got != [3]uint8{0xff, 0xff, 0xff} {
		t.Errorf("beside prose %v, want paper", got)
	}
}

func TestCodePanelInsetsTextAndStaysInsideReadingColumn(t *testing.T) {
	const src = "Before\n\n```go\nalpha()\nbeta()\n```\n\nAfter\n"
	for _, width := range []int{800, 480} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			d := nativeeditor.New(src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, width, 400)
			tt.Frame()
			code, _ := d.RectUTF16(utf16At(t, src, "alpha"))
			beta, _ := d.RectUTF16(utf16At(t, src, "beta"))
			plain, _ := d.RectUTF16(utf16At(t, src, "After"))
			left := plain.X
			columnWidth := min(float32(624), float32(width-96))
			right := left + columnWidth
			if code.X != left+16 {
				t.Errorf("code first column x = %v, panel left = %v; want a 16 DIP inset", code.X, left)
			}
			pixel := func(x, y float32) [3]uint8 {
				c := tt.Image().RGBAAt(int(x), int(y))
				return [3]uint8{c.R, c.G, c.B}
			}
			paper, surface := [3]uint8{0xff, 0xff, 0xff}, [3]uint8{portrait.Colors.Code.R, portrait.Colors.Code.G, portrait.Colors.Code.B}
			for name, at := range map[string]struct {
				x, y float32
			}{
				"left inside":  {left + 4, code.Y + code.H/2},
				"right inside": {right - 4, code.Y + code.H/2},
				"joined rows":  {left + 4, beta.Y},
			} {
				if got := pixel(at.x, at.y); got != surface {
					t.Errorf("%s = %v, want code surface %v", name, got, surface)
				}
			}
			for name, at := range map[string]struct {
				x, y float32
			}{
				"left outside":  {left - 2, code.Y + code.H/2},
				"right outside": {right + 2, code.Y + code.H/2},
			} {
				if got := pixel(at.x, at.y); got != paper {
					t.Errorf("%s = %v, want paper %v", name, got, paper)
				}
			}
			header, ok := tt.Access("Code block: go")
			if !ok {
				t.Fatal("code header is missing")
			}
			if got := pixel(left+4, header.Bounds.Y+header.Bounds.H/2); got != surface {
				t.Errorf("code header is outside the panel surface: %v", got)
			}
			label, ok := tt.Find("go")
			if !ok || ink(tt.Image(), label) == 0 {
				t.Error("code panel background covered the language label")
			}
		})
	}
}

func TestCodePanelRightInsetWrapsAtTheInsetFirstColumn(t *testing.T) {
	src := "```text\n" + strings.Repeat("x", 140) + "\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 480, 400)
	tt.Frame()
	firstAt := utf16At(t, src, "xxx")
	first, _ := d.RectUTF16(firstAt)
	for offset := firstAt + 1; offset < firstAt+140; offset++ {
		r, ok := d.RectUTF16(offset)
		if ok && r.Y > first.Y {
			if r.X != first.X {
				t.Fatalf("wrapped code starts at x %v, first row starts at %v", r.X, first.X)
			}
			return
		}
	}
	t.Fatal("long code did not wrap inside its inset panel")
}

func TestCodePanelInsetPreservesUTF16ClickAndSelection(t *testing.T) {
	const src = "```go\r\n甲🎵乙\r\n```\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 560, 300)
	tt.Frame()
	at := utf16At(t, src, "🎵")
	r, ok := d.RectUTF16(at)
	if !ok {
		t.Fatal("missing code glyph")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	if anchor, head := d.SelectionUTF16(); anchor != at || head != at {
		t.Fatalf("click selected UTF-16 %d..%d, want %d", anchor, head, at)
	}
	tt.Key(ui.Shift, ui.KeyRight)
	tt.Command("copy")
	if got := tt.Clipboard(); got != "🎵" {
		t.Fatalf("inset code selection copied %q, want 🎵", got)
	}
	if d.Source() != src || d.Modified() {
		t.Fatal("code selection changed the original bytes or history")
	}
}

func TestQuoteBarUsesLineInsetAtWideAndNarrowWidths(t *testing.T) {
	const src = "> Quoted line\n\nPlain\n"
	for _, width := range []int{800, 480} {
		d := nativeeditor.New(src)
		tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, width, 300)
		tt.Frame()
		bar, _ := d.RectUTF16(0)
		text, _ := d.RectUTF16(utf16At(t, src, "Quoted"))
		plain, _ := d.RectUTF16(utf16At(t, src, "Plain"))
		if bar.X != plain.X+14 {
			t.Errorf("width %d puts quote bar at %v, reading column at %v; want a 14 DIP inset", width, bar.X, plain.X)
		}
		if text.X <= bar.X+2 {
			t.Errorf("width %d puts quote text at %v on its bar at %v", width, text.X, bar.X)
		}
	}
}

func TestHTMLBlockUsesTheStripeLook(t *testing.T) {
	const src = "Before.\n\n<div>Example</div>\n\nAfter.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	info, ok := tt.Access("HTML block")
	if !ok {
		t.Fatal("missing HTML block")
	}
	img := tt.Image()
	stripes, bright := 0, 0
	for y := int(info.Bounds.Y) + 3; y < int(info.Bounds.Y)+7; y++ {
		for x := int(info.Bounds.X) + 16; x < int(info.Bounds.X+info.Bounds.W)-16; x++ {
			c := img.RGBAAt(x, y)
			if c.R < 245 && c.R == c.G && c.G == c.B {
				stripes++
			}
			if c.R == 255 && c.G == 255 && c.B == 255 {
				bright++
			}
		}
	}
	if stripes == 0 || bright == 0 {
		t.Fatalf("HTML block padding has %d stripe pixels and %d paper pixels; want the stripe look, not the code surface", stripes, bright)
	}
}
