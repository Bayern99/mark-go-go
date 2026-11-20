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

func TestPreviewReadingColumnAdaptsWhenTheWindowResizes(t *testing.T) {
	const source = "# Portrait report\r\n\nThe first line identifies this reading sample.\n"
	d := nativeeditor.New(source)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 1280, 820)
	for _, tc := range []struct {
		width, height int
		left, top     float32
	}{{1280, 820, 336, 45}, {480, 360, 56, 13}, {640, 600, 56, 13}, {641, 600, 56, 45}, {1280, 820, 336, 45}} {
		tt.SetSize(tc.width, tc.height)
		tt.Frame()
		title, hasTitle := d.RectUTF16(2)
		firstLine, hasFirstLine := d.RectUTF16(strings.Index(source, "The first line"))
		if !hasTitle || !hasFirstLine || title.X != tc.left || title.Y != tc.top || firstLine.X != tc.left || firstLine.Y != tc.top+64 {
			t.Errorf("%d × %d places title at %v and first paragraph at %v; want x %.0f and y %.0f, %.0f", tc.width, tc.height, title, firstLine, tc.left, tc.top, tc.top+64)
		}
	}
	if d.Source() != source || d.Modified() || d.CanUndo() {
		t.Fatal("resizing changed document content or history")
	}
}

func TestPreviewEditsAtHiddenDelimiters(t *testing.T) {
	d := nativeeditor.New("**汉字**。\r\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 510, 300)
	tt.Frame()
	if err := d.SelectUTF16(6, 6); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyBackspace)
	if got := d.Source(); got != "**汉**。\r\n" {
		t.Fatalf("backspace through bold delimiter %q", got)
	}
	tt.Command("undo")
	r, ok := d.RectUTF16(2)
	if !ok {
		t.Fatal("missing glyph")
	}
	tt.ClickAt(r.X+.15, r.Y+r.H/2)
	tt.Type("新")
	if got := d.Source(); got != "**新汉字**。\r\n" {
		t.Fatalf("click at first bold character inserted outside its run: %q", got)
	}
}
