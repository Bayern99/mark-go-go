package nativeeditor_test

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
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
	if got := tint(tt.Image(), r.X-8, r.Y); got != 0 {
		t.Fatalf("rounded highlight painted its top-left outer corner (%d)", got)
	}
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

func toggleOffset(t *testing.T, tt *ui.Tester, d *nativeeditor.Document) float32 {
	t.Helper()
	b, ok := tt.Access("Fold Two")
	if !ok {
		t.Fatal("next heading has no fold toggle")
	}
	return b.Bounds.Y - headingY(t, d)
}

const frame = time.Second / 60

func TestFoldingLandsInOneFrameWithNothingDrawnOverWhatFollows(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	open, offset := headingY(t, d), toggleOffset(t, tt, d)
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	landed := headingY(t, d)
	if landed >= open {
		t.Fatalf("next heading stayed at %v after folding, want it above %v", landed, open)
	}
	for elapsed := time.Duration(0); elapsed <= 2*portrait.Motion.Expand; elapsed += frame {
		if y := headingY(t, d); y != landed {
			t.Fatalf("at %v the next heading was at %v, want %v from the first frame", elapsed, y, landed)
		}
		if got := toggleOffset(t, tt, d); got != offset {
			t.Fatalf("at %v the next heading's toggle sat %v from it, want %v as when settled", elapsed, got, offset)
		}
		for _, text := range tt.Texts() {
			if strings.Contains(text, "First body") {
				t.Fatalf("at %v the collapsed section's text was still drawn", elapsed)
			}
		}
		tt.Advance(frame)
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

func TestExpandingShowsTheWholeSectionFromTheFirstFrame(t *testing.T) {
	d := nativeeditor.New(foldSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 600)
	tt.Frame()
	open := headingY(t, d)
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Expand One"); err != nil {
		t.Fatal(err)
	}
	full := bodyInk(t, tt, d)
	if full == 0 {
		t.Fatal("the expanded section drew no ink")
	}
	for elapsed := time.Duration(0); elapsed <= 2*portrait.Motion.Expand; elapsed += frame {
		if y := headingY(t, d); y != open {
			t.Fatalf("at %v the next heading was at %v, want %v from the first frame", elapsed, y, open)
		}
		if got := bodyInk(t, tt, d); got != full {
			t.Fatalf("at %v the expanded body drew %d ink, want %d from the first frame", elapsed, got, full)
		}
		tt.Advance(frame)
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

func accentPixels(img *image.RGBA, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			if int(c.G) > int(c.R)+8 && int(c.G) > int(c.B)+5 {
				n++
			}
		}
	}
	return n
}

const composerSource = "$$\nE=mc^2\n$$\n"

func composerDark(t *testing.T, tt *ui.Tester) int {
	t.Helper()
	r, ok := tt.Access("Markdown block editor")
	if !ok {
		t.Fatal("composer is not open")
	}
	img := tt.Image()
	n := 0
	for y := int(r.Bounds.Y); y < int(r.Bounds.Y+r.Bounds.H); y++ {
		for x := int(r.Bounds.X); x < int(r.Bounds.X+r.Bounds.W); x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 660 {
				n++
			}
		}
	}
	return n
}

func TestBlockComposerSlidesInWithExpandMotion(t *testing.T) {
	d := nativeeditor.New(composerSource)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 720)
	tt.Frame()
	if err := tt.Click("Edit math"); err != nil {
		t.Fatal(err)
	}
	start := composerDark(t, tt)
	tt.Advance(time.Second)
	end := composerDark(t, tt)
	if !(start*2 < end) {
		t.Fatalf("composer drew %d then %d dark pixels; want it to enter over the expand duration", start, end)
	}
}

func TestBlockComposerLandsAtOnceWithReducedMotion(t *testing.T) {
	d := nativeeditor.New(composerSource)
	tt := reduced(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 720)
	tt.Frame()
	if err := tt.Click("Edit math"); err != nil {
		t.Fatal(err)
	}
	start := composerDark(t, tt)
	tt.Advance(time.Second)
	if end := composerDark(t, tt); start < end*9/10 || end == 0 {
		t.Fatalf("reduced motion composer drew %d then %d dark pixels, want it in place at once", start, end)
	}
}

func TestCopiedCheckMorphsIn(t *testing.T) {
	const src = "```go\nalpha()\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	b, ok := tt.Access("Copy code")
	if !ok {
		t.Fatal("no copy button")
	}
	if err := tt.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	first := accentPixels(tt.Image(), b.Bounds)
	tt.Advance(time.Second)
	if settled := accentPixels(tt.Image(), b.Bounds); first >= settled/2 {
		t.Fatalf("the copied check drew %d then %d accent pixels; want it to morph in", first, settled)
	}
}

func TestCopiedCheckLandsAtOnceWithReducedMotion(t *testing.T) {
	const src = "```go\nalpha()\n```\n"
	d := nativeeditor.New(src)
	tt := reduced(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
	tt.Frame()
	b, ok := tt.Access("Copy code")
	if !ok {
		t.Fatal("no copy button")
	}
	if err := tt.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	first := accentPixels(tt.Image(), b.Bounds)
	tt.Advance(time.Second)
	if settled := accentPixels(tt.Image(), b.Bounds); first < settled*9/10 {
		t.Fatalf("the copied check drew %d then %d accent pixels under reduced motion", first, settled)
	}
}

func TestOutlineRevealPositionsHeadingNearTopInPreviewAndMarkdown(t *testing.T) {
	var sb strings.Builder
	for i := range 10 {
		sb.WriteString("# Section ")
		sb.WriteByte(byte('0' + i))
		sb.WriteString("\n\n")
		for range 8 {
			sb.WriteString("A paragraph of prose under this heading so the document scrolls across several viewports.\n\n")
		}
	}
	src := sb.String()
	midOff := utf16At(t, src, "# Section 5")
	lastOff := utf16At(t, src, "# Section 9")
	firstOff := utf16At(t, src, "# Section 0")

	for _, mode := range []nativeeditor.Mode{nativeeditor.Preview, nativeeditor.Markdown} {
		t.Run(string(mode), func(t *testing.T) {
			d := nativeeditor.New(src)
			d.SetMode(mode)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 400)
			tt.Frame()

			if err := d.Reveal(midOff); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			r, ok := d.RectUTF16(midOff)
			if !ok {
				t.Fatal("middle heading not laid out after Reveal")
			}
			maxTop := float32(portrait.Sizes.Large) + 24
			if r.Y < 0 || r.Y > maxTop {
				t.Fatalf("%s downward Reveal landed Section 5 at Y=%v, want near top [0, %v]", mode, r.Y, maxTop)
			}

			if err := d.Reveal(lastOff); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			lastRect, ok := d.RectUTF16(lastOff)
			if !ok || lastRect.Y < 0 || lastRect.Y+lastRect.H > 400 {
				t.Fatalf("%s EOF Reveal put Section 9 at %v (ok=%v), want visible inside 400pt viewport", mode, lastRect, ok)
			}

			if err := d.Reveal(firstOff); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if y := d.ScrollY(); y != 0 {
				t.Fatalf("%s Reveal to Section 0 left ScrollY=%v, want 0", mode, y)
			}
		})
	}
}
