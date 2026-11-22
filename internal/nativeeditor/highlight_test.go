package nativeeditor

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestMarkdownHighlighterUsesParsedCodeSpans(t *testing.T) {
	for _, source := range []string{
		"", "No code", "An unmatched `delimiter", "An escaped \\`delimiter`",
		"Hello **world** with `iW`.", "# Heading `iW`", "- List `iW`",
		"| Cell |\n| --- |\n| `iW` |", "**Strong `iW`**", "[Link `iW`](https://example.com)",
		"Code `` iW ` iW ``", "Code `iW\niW`",
	} {
		t.Run(source, func(t *testing.T) {
			buffer := editor.NewBuffer(source)
			h := newMarkdownHighlighter(buffer.Snapshot(), nil)
			h.Highlight(buffer.LineCount()-1, time.Second)
			for line := 0; line < buffer.LineCount(); line++ {
				text := buffer.Snapshot().Line(line)
				tokens, _ := h.Tokens(line, text, nil)
				start := 0
				for _, token := range tokens {
					if token.End < start || token.End > len(text) {
						t.Fatalf("invalid token range %d..%d for %q", start, token.End, text)
					}
					part := text[start:token.End]
					if strings.Contains(part, "iW") && token.Kind != editor.TokenCode {
						t.Fatalf("code %q has token kind %v", part, token.Kind)
					}
					if !strings.Contains(source, "iW") && token.Kind == editor.TokenCode {
						t.Fatalf("non-code %q has code styling", part)
					}
					start = token.End
				}
			}
		})
	}
}

func TestPreviewInlineCodeUsesPortraitFontAndPreservesBytes(t *testing.T) {
	font, err := os.ReadFile(filepath.Join("..", "portrait", "fonts", "ibm-plex-mono-400-normal.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ui.RegisterFont(font, "IBM Plex Mono"); err != nil {
		t.Fatal(err)
	}
	const source = "# Heading `iW`\r\n\r\nHello **world** with `iW`.\n\n- List `iW`\r\n\r\n| Cell |\n| --- |\n| `iW` |\n"
	d := New(source)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 900, 700)
	assertMono := func(doc *Document, offset int) {
		t.Helper()
		a, okA := doc.state.Rect(offset)
		b, okB := doc.state.Rect(offset + 1)
		z, okZ := doc.state.Rect(offset + 2)
		if !okA || !okB || !okZ || b.X <= a.X || math.Abs(float64((b.X-a.X)-(z.X-b.X))) > 0.1 {
			t.Fatalf("inline code is not monospaced: %v %v %v", a, b, z)
		}
		if got := doc.state.Theme.Tokens[editor.TokenCode].Family; got != portrait.FontMono {
			t.Fatalf("inline code font = %q", got)
		}
	}
	for round := 0; round < 2; round++ {
		d.SetMode(Preview)
		tt.Frame()
		text := d.state.Buffer.Snapshot().String()
		for offset, count := 0, 0; count < 3; count++ {
			offset += strings.Index(text[offset:], "iW")
			assertMono(d, offset)
			offset += 2
		}
		for _, table := range d.tables {
			cell := table.cells[1][0]
			assertMono(cell, strings.Index(cell.state.Buffer.Snapshot().String(), "iW"))
		}
		d.SetMode(Markdown)
		tt.Frame()
		d.MarkSaved()
		if d.Source() != source || d.Modified() || d.CanUndo() {
			t.Fatal("view round trip changed original source bytes or history")
		}
	}
	d.SetMode(Preview)
	tt.Frame()
	offset := strings.Index(d.Source(), "iW")
	if err := d.Apply(d.Version(), offset, offset+2, "iW", "Wi"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	assertMono(d, strings.Index(d.state.Buffer.Snapshot().String(), "Wi"))
}
