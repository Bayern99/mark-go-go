package nativeeditor_test

import (
	"image"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
)

func utf16At(t *testing.T, src, before string) int {
	t.Helper()
	p := strings.Index(src, before)
	if p < 0 {
		t.Fatalf("missing %q", before)
	}
	return utf8.RuneCountInString(src[:p])
}

func TestMarkdownSourceIsSetInCodeLeading(t *testing.T) {
	const src = "first line\nsecond line\nthird line\n"
	d := nativeeditor.New(src)
	d.SetMode(nativeeditor.Markdown)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 600, 300)
	tt.Frame()
	a, _ := d.RectUTF16(utf16At(t, src, "second"))
	b, _ := d.RectUTF16(utf16At(t, src, "third"))
	if got := b.Y - a.Y; got != 22 {
		t.Fatalf("Markdown source lines are %v apart, want 22 (IBM Plex Mono 13/22)", got)
	}
}

func TestPreviewCodeBlockIsSetInCodeLeading(t *testing.T) {
	const src = "Before\n\n```go\nalpha()\nbeta()\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	a, _ := d.RectUTF16(utf16At(t, src, "alpha"))
	b, _ := d.RectUTF16(utf16At(t, src, "beta"))
	if got := b.Y - a.Y; got != 22 {
		t.Fatalf("code lines are %v apart, want 22 (IBM Plex Mono 13/22)", got)
	}
}

const glyphScale = 4

func inkBox(img *image.RGBA, r ui.Rect) (image.Rectangle, int) {
	box, ink := image.Rectangle{}, 0
	for y := int(r.Y * glyphScale); y < int((r.Y+r.H)*glyphScale); y++ {
		for x := int(r.X * glyphScale); x < int((r.X+r.W)*glyphScale); x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 3*160 {
				ink += 255*3 - int(c.R) - int(c.G) - int(c.B)
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box.Sub(box.Min), ink
}

func referenceGlyph(t *testing.T, family, glyph string, size float32) (image.Rectangle, int) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Padding(20).Children(func() {
			ui.Text(c, glyph).Font(family).FontSize(size).TextColor(ui.Hex("#1d1d1f"))
		})
	}, 200, 120)
	tt.SetScale(glyphScale)
	tt.Frame()
	r, ok := tt.Find(glyph)
	if !ok {
		t.Fatal("reference glyph missing")
	}
	return inkBox(tt.Image(), ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4})
}

func TestPreviewSetsHanInIBMPlexSansSCBesideLatin(t *testing.T) {
	const src = "Portrait 写作 sample.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 300)
	tt.SetScale(glyphScale)
	tt.Frame()
	at := utf16At(t, src, "写")
	a, _ := d.RectUTF16(at)
	b, _ := d.RectUTF16(at + 1)
	got, gotInk := inkBox(tt.Image(), ui.Rect{X: a.X, Y: a.Y, W: b.X - a.X, H: a.H})
	want, wantInk := referenceGlyph(t, "IBM Plex Sans SC", "写", 16)
	system, _ := referenceGlyph(t, "sans-serif", "写", 16)
	if want == system {
		t.Skip("system fallback draws 写 like IBM Plex Sans SC; the test cannot tell them apart")
	}
	if !similar(got, want) || similar(got, system) || gotInk < wantInk*9/10 || gotInk > wantInk*11/10 {
		t.Fatalf("写 in Preview has ink box %v (%d), want IBM Plex Sans SC %v (%d); system fallback %v", got, gotInk, want, wantInk, system)
	}
}

func TestPreviewDisplayHeadingsUseInterDisplayAtTheirOwnSize(t *testing.T) {
	for _, tc := range []struct {
		src  string
		size float32
	}{{"# HHHHHHHHHH\n", 28}, {"## HHHHHHHHHH\n", 20}} {
		d := nativeeditor.New(tc.src)
		tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 900, 300)
		tt.Frame()
		start := utf16At(t, tc.src, "H")
		a, _ := d.RectUTF16(start)
		b, _ := d.RectUTF16(start + 10)
		ref := ui.NewTester(func(c *ui.Context) {
			ui.Row(c).Children(func() {
				ui.Text(c, "HHHHHHHHHH").Font("Inter Display").FontSize(tc.size).FontWeight(600)
			})
		}, 900, 100)
		ref.Frame()
		r, _ := ref.Find("HHHHHHHHHH")
		if got := b.X - a.X; got < r.W-0.5 || got > r.W+0.5 {
			t.Errorf("%q advances %v, want %v (Inter Display at %v)", strings.TrimSpace(tc.src), got, r.W, tc.size)
		}
	}
}

func similar(a, b image.Rectangle) bool {
	dx, dy := a.Dx()-b.Dx(), a.Dy()-b.Dy()
	return dx*dx <= 4 && dy*dy <= 4
}
