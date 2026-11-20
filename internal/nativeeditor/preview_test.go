package nativeeditor_test

import (
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderedPreviewAcceptsPointerInputAndCrossBlockSelection(t *testing.T) {
	const src = "# Read\r\n\r\n- **音乐** and _echo_ &amp; \\*\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 600, 400)
	tt.Frame()
	node, ok := tt.Access("Markdown Preview")
	if !ok || node.Value != "Read\n\n• 音乐 and echo & *\n" {
		t.Fatalf("accessible Preview %q %v", node.Value, ok)
	}
	at := func(before string) ui.Rect {
		p := strings.Index(src, before)
		if p < 0 {
			t.Fatal(before)
		}
		r, ok := d.RectUTF16(utf8.RuneCountInString(src[:p]))
		if !ok {
			t.Fatalf("position missing for %q", before)
		}
		return r
	}
	begin, end := at("Read"), at("乐")
	tt.Press(begin.X+.1, begin.Y+begin.H/2)
	tt.Move(end.X+.1, end.Y+end.H/2)
	tt.Release(end.X+.1, end.Y+end.H/2)
	tt.Command("copy")
	if got := tt.Clipboard(); got != "Read\n\n• 音" {
		t.Fatalf("copied %q", got)
	}
	md, err := tt.ClipboardData().Read(transfer.Markdown)
	if err != nil || !strings.Contains(string(md), "\r\n\r\n- **音") {
		t.Fatalf("source flavor %q %v", md, err)
	}
	tt.ClickAt(end.X+.1, end.Y+end.H/2)
	tt.Type("新")
	if got := d.Source(); got != strings.Replace(src, "音乐", "音新乐", 1) {
		t.Fatalf("inserted at wrong character %q", got)
	}
	d.SetMode(nativeeditor.Markdown)
	tt.Frame()
	if node, ok := tt.Access("Markdown source"); !ok || !strings.Contains(node.Value, "**音新乐**") {
		t.Fatalf("accessible Markdown %q", node.Value)
	}
}
