package nativeeditor_test

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func previewRhythmDocument(t *testing.T, src string) (*nativeeditor.Document, *ui.Tester) {
	t.Helper()
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 900, 600)
	tt.Frame()
	return d, tt
}

func previewRectBefore(t *testing.T, d *nativeeditor.Document, src, before string) ui.Rect {
	t.Helper()
	r, ok := d.RectUTF16(utf16At(t, src, before))
	if !ok {
		t.Fatalf("missing Preview rectangle before %q", before)
	}
	return r
}

func assertPreviewGap(t *testing.T, d *nativeeditor.Document, src, before, after string, want float32) {
	t.Helper()
	a := previewRectBefore(t, d, src, before)
	b := previewRectBefore(t, d, src, after)
	if got := b.Y - (a.Y + a.H); got != want {
		t.Errorf("gap from %q to %q = %v, want %v (rects %v, %v)", before, after, got, want, a, b)
	}
}

func TestPreviewLineBreaksAndBlockGaps(t *testing.T) {
	t.Run("automatic wrap", func(t *testing.T) {
		src := strings.Repeat("wrap ", 40) + "\n"
		d, _ := previewRhythmDocument(t, src)
		first := previewRectBefore(t, d, src, "wrap")
		for offset := 1; offset < len(src); offset++ {
			r, ok := d.RectUTF16(offset)
			if ok && r.Y > first.Y {
				if got := r.Y - (first.Y + first.H); got != 0 {
					t.Fatalf("wrapped row gap = %v, want 0", got)
				}
				return
			}
		}
		t.Fatal("paragraph did not wrap")
	})
	t.Run("soft break", func(t *testing.T) {
		const src = "soft\nbreak\n"
		d, _ := previewRhythmDocument(t, src)
		a := previewRectBefore(t, d, src, "soft")
		b := previewRectBefore(t, d, src, "break")
		if got := b.Y - (a.Y + a.H); got != 0 {
			t.Fatalf("soft break added vertical space %v: %v, %v", got, a, b)
		}
	})
	t.Run("hard break", func(t *testing.T) {
		const src = "hard  \nbreak\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "hard", "break", 0)
	})
	t.Run("paragraph", func(t *testing.T) {
		const src = "First\n\nSecond\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "First", "Second", portrait.Editor.ParagraphGap)
	})
	t.Run("additional blank stays full height", func(t *testing.T) {
		const src = "First\n\n\nSecond\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "First", "Second", portrait.Editor.ParagraphGap+portrait.Editor.Body.Leading)
	})
	t.Run("tight list", func(t *testing.T) {
		const src = "- one\n- two\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "one", "two", portrait.Editor.ListGap)
	})
	t.Run("loose list", func(t *testing.T) {
		const src = "- one\n\n- two\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "one", "two", portrait.Editor.ParagraphGap)
	})
	t.Run("nested list", func(t *testing.T) {
		const src = "- parent\n  - child\n  - next\n- sibling\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "parent", "child", portrait.Editor.NestedListGap)
		assertPreviewGap(t, d, src, "child", "next", portrait.Editor.ListGap)
		assertPreviewGap(t, d, src, "next", "sibling", portrait.Editor.ListGap)
	})
}

func assertPreviewGapNear(t *testing.T, d *nativeeditor.Document, src, before, after string, want float32) {
	t.Helper()
	a := previewRectBefore(t, d, src, before)
	b := previewRectBefore(t, d, src, after)
	if got := b.Y - (a.Y + a.H); math.Abs(float64(got-want)) > 1 {
		t.Errorf("gap from %q to %q = %v, want %v within caret rounding (rects %v, %v)", before, after, got, want, a, b)
	}
}

func TestPreviewHeadingRhythmAndFoldAlignment(t *testing.T) {
	const src = "# First\n\nBody\n\n## Second\n### Third\n"
	d, _ := previewRhythmDocument(t, src)
	first := previewRectBefore(t, d, src, "First")
	if first.Y != 45 {
		t.Errorf("document's first heading starts at y %v, want the page margin at 45 without heading-before", first.Y)
	}
	assertPreviewGap(t, d, src, "First", "Body", portrait.Editor.HeadingAfter[0])
	assertPreviewGapNear(t, d, src, "Body", "Second", portrait.Editor.HeadingBefore[1])
	wantAdjacent := max(portrait.Editor.HeadingAfter[1], portrait.Editor.HeadingBefore[2]/2)
	assertPreviewGap(t, d, src, "Second", "Third", wantAdjacent)

	const unspaced = "Body\n# Heading\n\nText\n"
	d, tt := previewRhythmDocument(t, unspaced)
	assertPreviewGapNear(t, d, unspaced, "Body", "Heading", portrait.Editor.HeadingBefore[0])
	heading := previewRectBefore(t, d, unspaced, "Heading")
	fold, ok := tt.Access("Fold Heading")
	if !ok {
		t.Fatal("heading fold control is missing")
	}
	if delta := math.Abs(float64((fold.Bounds.Y + fold.Bounds.H/2) - (heading.Y + heading.H/2))); delta > 1 {
		t.Errorf("spacing block displaced the fold control by %v points: fold %v, heading %v", delta, fold.Bounds, heading)
	}
}

func TestPreviewTableAndCodeOuterPaperGaps(t *testing.T) {
	t.Run("table", func(t *testing.T) {
		const src = "Before\n\n| A | B |\n| - | - |\n| x | y |\n\nAfter\n"
		d, tt := previewRhythmDocument(t, src)
		before := previewRectBefore(t, d, src, "Before")
		after := previewRectBefore(t, d, src, "After")
		table, ok := tt.Access("Markdown table")
		if !ok {
			t.Fatal("Markdown table is missing")
		}
		if got := table.Bounds.Y - (before.Y + before.H); got != portrait.Editor.ParagraphGap {
			t.Errorf("gap above table = %v, want %v", got, portrait.Editor.ParagraphGap)
		}
		if got := after.Y - (table.Bounds.Y + table.Bounds.H); got != portrait.Editor.ParagraphGap {
			t.Errorf("gap below table = %v, want %v", got, portrait.Editor.ParagraphGap)
		}
	})
	t.Run("code", func(t *testing.T) {
		const src = "Before\n\n```go\nalpha()\n```\n\nAfter\n"
		d, tt := previewRhythmDocument(t, src)
		before := previewRectBefore(t, d, src, "Before")
		code := previewRectBefore(t, d, src, "alpha")
		after := previewRectBefore(t, d, src, "After")
		header, ok := tt.Access("Code block: go")
		if !ok {
			t.Fatal("code header is missing")
		}
		if got := header.Bounds.Y - (before.Y + before.H); got != portrait.Editor.ParagraphGap {
			t.Errorf("paper gap above code = %v, want %v", got, portrait.Editor.ParagraphGap)
		}
		panelBottom := code.Y + code.H + portrait.Editor.CodeBlockBottom
		if got := after.Y - panelBottom; got != portrait.Editor.ParagraphGap {
			t.Errorf("paper gap below code = %v, want %v (code %v, after %v)", got, portrait.Editor.ParagraphGap, code, after)
		}
		x := int(before.X + 4)
		footer := tt.Image().RGBAAt(x, int(code.Y+code.H+portrait.Editor.CodeBlockBottom/2))
		wantSurface := color.RGBA{R: portrait.Colors.Code.R, G: portrait.Colors.Code.G, B: portrait.Colors.Code.B, A: 255}
		if footer != wantSurface {
			t.Errorf("code footer = %v, want code surface %v", footer, wantSurface)
		}
		paper := tt.Image().RGBAAt(x, int(panelBottom+portrait.Editor.ParagraphGap/2))
		if paper != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
			t.Errorf("paper below code = %v, want white", paper)
		}
	})
	t.Run("code internal blank", func(t *testing.T) {
		const src = "```go\nalpha()\n\nbeta()\n```\n"
		d, _ := previewRhythmDocument(t, src)
		assertPreviewGap(t, d, src, "alpha", "beta", portrait.Type.Code.Leading)
	})
}

func TestThematicBreakAndQuoteWidthsUseEditorTokens(t *testing.T) {
	t.Run("thematic break", func(t *testing.T) {
		defer portrait.UseEditor(16, 13, 624)
		for _, size := range []float32{15, 16, 17, 18} {
			portrait.UseEditor(size, 13, 624)
			_, tt := previewRhythmDocument(t, "---\n")
			rule, ok := tt.Access("Thematic break")
			if !ok {
				t.Fatalf("reading size %v has no thematic break", size)
			}
			if got := rule.Bounds.H; got != portrait.Editor.ThematicBreakHeight {
				t.Errorf("reading size %v gives rule height %v, want %v", size, got, portrait.Editor.ThematicBreakHeight)
			}
		}
	})
	t.Run("quote", func(t *testing.T) {
		const src = ">> Quoted\n"
		d, tt := previewRhythmDocument(t, src)
		bar := previewRectBefore(t, d, src, ">")
		secondBar, ok := d.RectUTF16(1)
		if !ok || secondBar.X-bar.X != portrait.Editor.QuoteBarWidth {
			t.Errorf("adjacent quote declarations are %v apart, want %v (%v, %v)", secondBar.X-bar.X, portrait.Editor.QuoteBarWidth, bar, secondBar)
		}
		text := previewRectBefore(t, d, src, "Quoted")
		if text.X < secondBar.X+portrait.Editor.QuoteBarWidth {
			t.Errorf("quote text %v overlaps its bar %v", text, secondBar)
		}
		y := int(bar.Y + bar.H/2)
		accent := 0
		for x := int(bar.X) - 1; x <= int(secondBar.X+portrait.Editor.QuoteBarWidth)+1; x++ {
			px := tt.Image().RGBAAt(x, y)
			if px.R == portrait.Colors.Accent.R && px.G == portrait.Colors.Accent.G && px.B == portrait.Colors.Accent.B {
				accent++
			}
		}
		if want := int(2 * portrait.Editor.QuoteBarWidth); accent != want {
			t.Errorf("painted nested quote bars = %d pixels, want %d", accent, want)
		}
	})
}

func TestCompressedBlankLinesRetainEditingAndHistory(t *testing.T) {
	const original = "First\nSecond\n"
	const oneReturn = "First\n\nSecond\n"
	const twoReturns = "First\n\n\nSecond\n"
	d, tt := previewRhythmDocument(t, original)
	if err := d.SelectUTF16(len("First"), len("First")); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if d.Source() != oneReturn {
		t.Fatalf("first Return wrote %q", d.Source())
	}
	_, firstAt := d.SelectionUTF16()
	firstCaret, ok := d.RectUTF16(firstAt)
	if !ok || firstCaret.H != portrait.Editor.ParagraphGap {
		t.Fatalf("first structural blank caret = %v, %v; want visible height %v", firstCaret, ok, portrait.Editor.ParagraphGap)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if d.Source() != twoReturns {
		t.Fatalf("second Return wrote %q", d.Source())
	}
	_, secondAt := d.SelectionUTF16()
	secondCaret, ok := d.RectUTF16(secondAt)
	if !ok || secondCaret.H != portrait.Editor.Body.Leading || secondCaret.Y-firstCaret.Y != portrait.Editor.ParagraphGap {
		t.Fatalf("second blank caret = %v, %v after first %v; want a full-height visible line", secondCaret, ok, firstCaret)
	}
	tt.Key(0, ui.KeyDown)
	_, downAt := d.SelectionUTF16()
	if downAt != strings.Index(twoReturns, "Second") {
		t.Fatalf("Down skipped to source offset %d, want Second at %d", downAt, strings.Index(twoReturns, "Second"))
	}
	tt.ClickAt(firstCaret.X+.1, firstCaret.Y+firstCaret.H/2)
	_, clickAt := d.SelectionUTF16()
	if clickAt != firstAt {
		t.Fatalf("click in compressed gap selected %d, want its source line at %d", clickAt, firstAt)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != twoReturns {
		t.Fatalf("copy across compressed blank lines = %q, want %q", got, twoReturns)
	}
	for range 2 {
		tt.Command("undo")
		tt.Frame()
	}
	if d.Source() != original {
		t.Fatalf("Undo did not restore the original paragraph: %q", d.Source())
	}
	for range 2 {
		tt.Command("redo")
		tt.Frame()
	}
	if d.Source() != twoReturns {
		t.Fatalf("Redo did not restore both Returns: %q", d.Source())
	}
	at := secondAt
	if err := d.SelectUTF16(at, at); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Compose("n", 1)
	tt.Frame()
	tt.Compose("ni", 2)
	tt.Frame()
	tt.Type("你")
	tt.Frame()
	want := twoReturns[:at] + "你" + twoReturns[at:]
	if d.Source() != want {
		t.Fatalf("IME commit on the new blank line = %q, want %q", d.Source(), want)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if d.Source() != twoReturns {
		t.Fatalf("Backspace after the IME commit = %q, want %q", d.Source(), twoReturns)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	_, at = d.SelectionUTF16()
	if d.Source() != oneReturn {
		t.Fatalf("Backspace across the new blank line = %q, want %q", d.Source(), oneReturn)
	}
	if _, ok := d.RectUTF16(at); !ok {
		t.Fatal("Backspace across the compressed blank hid the caret")
	}
}
