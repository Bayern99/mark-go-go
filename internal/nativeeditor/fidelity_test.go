package nativeeditor_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestExactLineEndingHistory(t *testing.T) {
	for _, test := range []struct {
		name, before, after string
		edit                func(*editor.Buffer)
	}{
		{"replace endings only", "a\r\nb\n", "a\nb\r\n", func(b *editor.Buffer) { b.Set("a\nb\r\n") }},
		{"delete between CR and LF", "foo\rX\nbar", "foo\r\nbar", func(b *editor.Buffer) { b.Edit(editor.Edit{Start: 4, End: 5}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := editor.NewSourceBuffer(test.before)
			check := func(want string) {
				t.Helper()
				var out strings.Builder
				b.WriteTo(&out)
				if out.String() != want || b.String() != editor.NewBuffer(want).String() {
					t.Fatalf("disk %q, editor %q, want %q", out.String(), b.String(), want)
				}
			}
			test.edit(b)
			check(test.after)
			if !b.Modified() || !b.Undo() {
				t.Fatal("missing undo")
			}
			check(test.before)
			if b.Modified() || !b.Redo() {
				t.Fatal("failed to return to saved text or redo")
			}
			check(test.after)
		})
	}
}
