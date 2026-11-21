package nativeeditor_test

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func reduced(view func(*ui.Context), w, h int) *ui.Tester {
	return ui.NewTesterWith(view, w, h, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
}

func tint(img *image.RGBA, x, y float32) int {
	c := img.RGBAAt(int(x), int(y))
	return 255*3 - int(c.R) - int(c.G) - int(c.B)
}

func ink(img *image.RGBA, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			n += tint(img, float32(x), float32(y))
		}
	}
	return n
}

const jumpSource = "Intro paragraph.\n\n# Target heading\n\nBody under the target.\n"

func TestOutlineJumpHighlightsTheHeadingAndFadesWithinJumpDuration(t *testing.T) {
	if err := nativeui.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	d := nativeeditor.New(jumpSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	at := utf16At(t, jumpSource, "Target")
	if err := d.Reveal(at); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != at || z != at {
		t.Fatalf("reveal selected %d..%d, want the heading at %d", a, z, at)
	}
	r, ok := d.RectUTF16(at)
	if !ok {
		t.Fatal("heading not laid out")
	}
	x, y := r.X-4, r.Y+r.H/2
	start := tint(tt.Image(), x, y)

	tt.Advance(portrait.Motion.Jump * 35 / 100)
	peak := tint(tt.Image(), x, y)
	tt.Advance(portrait.Motion.Jump * 45 / 100)
	late := tint(tt.Image(), x, y)
	tt.Advance(portrait.Motion.Jump)
	end := tint(tt.Image(), x, y)
	if !(start > 0 && peak > 0 && late < peak && end == 0) {
		t.Fatalf("highlight beside the heading went %d, %d, %d, %d; want a tint fading to white within %v", start, peak, late, end, portrait.Motion.Jump)
	}
	body, _ := d.RectUTF16(utf16At(t, jumpSource, "Body"))
	if tint(tt.Image(), body.X-4, body.Y+body.H/2) != 0 {
		t.Fatal("highlight spilled past the heading")
	}
}

func TestOutlineJumpHighlightDoesNotPlayWithReducedMotion(t *testing.T) {
	d := nativeeditor.New(jumpSource)
	tt := reduced(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	at := utf16At(t, jumpSource, "Target")
	if err := d.Reveal(at); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, _ := d.RectUTF16(at)
	if got := tint(tt.Image(), r.X-4, r.Y+r.H/2); got != 0 {
		t.Fatalf("reduced motion still tinted the heading (%d)", got)
	}
}

const foldSource = "# One\n\nFirst body paragraph.\n\nSecond body paragraph.\n\nThird body paragraph.\n\n# Two\n\nLast.\n"

func headingY(t *testing.T, d *nativeeditor.Document) float32 {
	t.Helper()
	r, ok := d.RectUTF16(utf16At(t, foldSource, "Two"))
	if !ok {
		t.Fatal("next heading not laid out")
	}
	return r.Y
}

func TestFoldingCollapsesTheSectionOverExpandDuration(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	open := headingY(t, d)
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	start := headingY(t, d)
	tt.Advance(portrait.Motion.Expand / 3)
	mid := headingY(t, d)
	tt.Advance(time.Second)
	end := headingY(t, d)
	if !(start > mid && mid > end && open-start < (open-end)/2) {
		t.Fatalf("next heading went from %v to %v, %v, %v; want it to rise smoothly as the section collapses", open, start, mid, end)
	}
	for _, text := range tt.Texts() {
		if strings.Contains(text, "First body") {
			t.Fatal("the collapsed section's text stayed on screen")
		}
	}
}

func TestFoldingLandsAtOnceWithReducedMotion(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := reduced(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	start := headingY(t, d)
	tt.Advance(time.Second)
	if end := headingY(t, d); end != start {
		t.Fatalf("reduced motion folded from %v to %v, want it in place at once", start, end)
	}
}

func bodyInk(t *testing.T, tt *ui.Tester, d *nativeeditor.Document) int {
	t.Helper()
	a := utf16At(t, foldSource, "First")
	ra, ok := d.RectUTF16(a)
	if !ok {
		t.Fatal("expanded body not laid out")
	}
	rb, _ := d.RectUTF16(a + len("First body paragraph"))
	return ink(tt.Image(), ui.Rect{X: ra.X, Y: ra.Y, W: rb.X - ra.X, H: ra.H})
}

func TestExpandingFadesTheSectionIn(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Expand One"); err != nil {
		t.Fatal(err)
	}
	start := bodyInk(t, tt, d)
	tt.Advance(portrait.Motion.Expand / 3)
	mid := bodyInk(t, tt, d)
	tt.Advance(time.Second)
	end := bodyInk(t, tt, d)
	if !(start < mid && mid < end && start < end/2) {
		t.Fatalf("expanded body ink went %d, %d, %d; want it to fade in", start, mid, end)
	}
}

func TestExpandingLandsAtOnceWithReducedMotion(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := reduced(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Expand One"); err != nil {
		t.Fatal(err)
	}
	start := bodyInk(t, tt, d)
	tt.Advance(time.Second)
	if end := bodyInk(t, tt, d); start != end || start == 0 {
		t.Fatalf("reduced motion expanded with ink %d then %d, want it all at once", start, end)
	}
}

var placeholderIcons = []string{"×", "▾", "▸", "↗", "✕", "☰", "⋯", "│"}

func TestEditorDrawsNoUnicodeIcons(t *testing.T) {
	const src = "# Heading\n\nSee [docs](https://example.com/docs).\n\n> Quoted line.\n\n## Child\n\nText.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	for _, text := range tt.Texts() {
		for _, glyph := range placeholderIcons {
			if strings.Contains(text, glyph) {
				t.Errorf("Unicode placeholder icon %q in %q", glyph, text)
			}
		}
	}
	if field, ok := tt.Access("Markdown Preview"); !ok || strings.Contains(field.Value, "│") {
		t.Errorf("Preview reads a quote bar glyph: %q", field.Value)
	}
	for _, name := range []string{"Fold Heading", "Fold Child", "Open https://example.com/docs"} {
		info, ok := tt.Access(name)
		if strings.HasPrefix(name, "Open ") {
			info, ok = tt.AccessByRole(ui.RoleLink, name)
		}
		if !ok {
			t.Fatalf("missing %q", name)
		}
		if info.Description == "" {
			t.Errorf("%q has no tooltip", name)
		}
	}
}
