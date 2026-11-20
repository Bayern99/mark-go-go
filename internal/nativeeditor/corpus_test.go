package nativeeditor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
)

func TestMarkdownCorpusSourceAndPreview(t *testing.T) {
	paths, err := filepath.Glob("../testdata/markdown/*.md")
	if err != nil || len(paths) != 26 {
		t.Fatalf("expected 26 original Markdown fixtures, found %d: %v", len(paths), err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			source := string(data)
			d := nativeeditor.New(source)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 600, 360)
			tt.Frame()
			d.SetMode(nativeeditor.Markdown)
			tt.Frame()
			d.SetMode(nativeeditor.Preview)
			tt.Frame()
			if d.Source() != source || d.Modified() || d.CanUndo() {
				t.Fatal("switching modes changed the source")
			}
			at := strings.IndexFunc(source, func(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127 })
			if at < 0 {
				at = 0
			} else {
				_, n := utf8.DecodeRuneInString(source[at:])
				at += n
			}
			pos := len(utf16.Encode([]rune(source[:at])))
			if err = d.SelectUTF16(pos, pos); err != nil {
				t.Fatal(err)
			}
			tt.Type("文🎵e\u0301")
			tt.Frame()
			edited := source[:at] + "文🎵e\u0301" + source[at:]
			if d.Source() != edited || !d.Modified() {
				t.Fatalf("Preview changed bytes outside position %d", at)
			}
			d.SetMode(nativeeditor.Markdown)
			tt.Frame()
			if !d.Undo() || d.Source() != source || d.Modified() {
				t.Fatal("Undo failed to restore original bytes and disk baseline")
			}
			if !d.Redo() || d.Source() != edited || !d.Modified() {
				t.Fatal("Redo failed to restore edited bytes")
			}
			d.MarkSaved()
			if d.Modified() || !d.Undo() || d.Source() != source || !d.Modified() {
				t.Fatal("Undo after save failed to track disk baseline")
			}
		})
	}
}

func TestMenuUndoFollowsFocusedDraft(t *testing.T) {
	d := nativeeditor.New("body")
	draft := ""
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			d.View(c, false).Grow(1).FillWidth().AutoFocus()
			ui.TextInput(c, &draft).Label("Note draft").Height(40)
		})
	}, 500, 280)
	tt.Frame()
	tt.Type("文")
	if d.Source() != "文body" {
		t.Fatalf("document input %q", d.Source())
	}
	if err := tt.Click("Note draft"); err != nil {
		t.Fatal(err)
	}
	tt.Type("draft")
	tt.Command("undo")
	if draft != "" || d.Source() != "文body" {
		t.Fatalf("draft undo changed source %q draft %q", d.Source(), draft)
	}
	tt.Command("redo")
	if draft != "draft" || d.Source() != "文body" {
		t.Fatalf("draft redo changed source %q draft %q", d.Source(), draft)
	}
}
