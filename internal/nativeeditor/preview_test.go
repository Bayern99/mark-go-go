package nativeeditor_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
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

func TestOrdinaryCodeFenceKeepsItsSourceCaretInPreview(t *testing.T) {
	const src = "Before\r\n\r\n```go\r\nfmt.Println(\"iW\")\r\nsecond()\r\n```  \r\nAfter\r\n"
	if err := nativeui.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 760, 420)
	tt.Frame()
	if _, ok := tt.Find("Edit code"); ok {
		t.Fatal("ordinary code fence opened the special-block editor")
	}
	if _, ok := tt.Find("Code block: go"); !ok {
		t.Fatal("Preview did not show the code language")
	}
	if d.Source() != src || d.Modified() {
		t.Fatal("presenting code changed its source")
	}
	firstCodeRune := utf8.RuneCountInString(src[:strings.Index(src, "iW")])
	i, hasI := d.RectUTF16(firstCodeRune)
	w, hasW := d.RectUTF16(firstCodeRune + 1)
	afterW, hasAfterW := d.RectUTF16(firstCodeRune + 2)
	if !hasI || !hasW || !hasAfterW || math.Abs(float64((w.X-i.X)-(afterW.X-w.X))) > 0.1 {
		t.Fatalf("code glyphs are not laid out in equal monospace cells: %v %v %v", i, w, afterW)
	}
	pos := utf8.RuneCountInString(src[:strings.Index(src, "Print")])
	r, ok := d.RectUTF16(pos)
	if !ok {
		t.Fatal("missing Preview caret inside code")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	tt.Type("Log")
	tt.Frame()
	if want := strings.Replace(src, "Print", "LogPrint", 1); d.Source() != want {
		t.Fatalf("code edit changed bytes outside the caret: %q", d.Source())
	}
	if d.Mode() != nativeeditor.Preview {
		t.Fatal("editing code switched out of Preview")
	}
}

func TestOpenCodeFenceKeepsItsLastLineEditable(t *testing.T) {
	const src = "Before\n\n```go\nunfinished()\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 640, 340)
	tt.Frame()
	preview, ok := tt.Access("Markdown Preview")
	if !ok || !strings.Contains(preview.Value, "unfinished()") {
		t.Fatalf("open fence body disappeared from Preview: %q", preview.Value)
	}
	pos := utf8.RuneCountInString(src[:strings.Index(src, "unfinished")])
	r, ok := d.RectUTF16(pos)
	if !ok {
		t.Fatal("missing caret in an open code fence")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	tt.Type("keep")
	tt.Frame()
	if want := strings.Replace(src, "unfinished", "keepunfinished", 1); d.Source() != want {
		t.Fatalf("editing an open fence changed another byte: %q", d.Source())
	}
}

func TestCodeFenceWithMetadataKeepsItsEditLayer(t *testing.T) {
	const src = "```go linenos\r\nfmt.Println(1)\r\n```\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 600, 300)
	tt.Frame()
	if _, ok := tt.Find("Edit code"); !ok {
		t.Fatal("code fence metadata lost its in-place Markdown editor")
	}
	if _, ok := tt.Find("go linenos"); !ok {
		t.Fatal("code fence metadata disappeared from Preview")
	}
	if d.Source() != src || d.Modified() {
		t.Fatal("presenting code metadata changed the source")
	}
}

func TestInlineImageLoadsFromItsDocumentFolderAndKeepsFollowingTextEditable(t *testing.T) {
	dir := t.TempDir()
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><rect width="16" height="16" fill="#e11d48"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "chart.svg"), []byte(svg), 0600); err != nil {
		t.Fatal(err)
	}
	const src = "Before ![Chart alt](chart.svg \"title\") after."
	d := nativeeditor.New(src)
	d.SetImageDirectory(dir)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 680, 320)
	tt.Frame()

	loaded := false
	for range 100 {
		tt.Frame()
		if r, ok := tt.Find("Chart alt"); ok && r.W > 0 && r.H > 0 {
			pixel := tt.Image().RGBAAt(int(r.X+8), int(r.Y+8))
			if pixel.R > 200 && pixel.G < 80 && pixel.B < 160 {
				loaded = true
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if !loaded {
		t.Fatal("local image did not render")
	}
	pos := utf8.RuneCountInString(src[:strings.Index(src, "after")])
	r, ok := d.RectUTF16(pos)
	if !ok {
		t.Fatal("missing source caret after inline image")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	tt.Type("new ")
	tt.Frame()
	if want := strings.Replace(src, "after", "new after", 1); d.Source() != want {
		t.Fatalf("edit after image changed other source bytes: %q", d.Source())
	}

	otherDir := t.TempDir()
	const blueSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><rect width="16" height="16" fill="#2563eb"/></svg>`
	if err := os.WriteFile(filepath.Join(otherDir, "chart.svg"), []byte(blueSVG), 0600); err != nil {
		t.Fatal(err)
	}
	d.SetImageDirectory(otherDir)
	loadedBlue := false
	for range 100 {
		tt.Frame()
		if r, ok := tt.Find("Chart alt"); ok && r.W > 0 && r.H > 0 {
			pixel := tt.Image().RGBAAt(int(r.X+8), int(r.Y+8))
			if pixel.B > 200 && pixel.R < 100 {
				loadedBlue = true
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if !loadedBlue {
		t.Fatal("changing the document folder did not reload its relative image")
	}
}

func TestLinkHoverOffersOnlySafeOpenAndKeepsLabelEditable(t *testing.T) {
	const src = "Read [Example](https://example.com/docs) and [script](javascript:alert(1))."
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 680, 320)
	tt.Frame()
	label := strings.Index(src, "Example")
	r, ok := d.RectUTF16(utf8.RuneCountInString(src[:label]))
	if !ok {
		t.Fatal("missing linked text caret")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	tt.Type("Sample")
	tt.Frame()
	current := strings.Replace(src, "Example", "SampleExample", 1)
	if d.Source() != current {
		t.Fatalf("link label edit changed delimiters or neighboring bytes: %q", d.Source())
	}
	if _, ok := tt.Find("Open https://example.com/docs"); !ok {
		t.Fatal("safe link has no accessible open control")
	}
	if err := tt.Click("Open https://example.com/docs"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != "https://example.com/docs" {
		t.Fatalf("opened links %q", got)
	}
	label = strings.Index(current, "Example")
	r, ok = d.RectUTF16(utf8.RuneCountInString(current[:label]))
	if !ok {
		t.Fatal("missing edited link caret")
	}
	tt.Move(r.X+.1, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("Open link"); !ok {
		t.Fatal("Preview did not expose a safe link action on hover")
	}
	if _, ok := tt.Find("Copy link"); !ok {
		t.Fatal("link hover did not expose an accessible copy action")
	}
	if err := tt.Click("Copy link"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "https://example.com/docs" {
		t.Fatalf("copied link %q", got)
	}
	if err := tt.Click("Open link"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 2 || got[0] != "https://example.com/docs" || got[1] != "https://example.com/docs" {
		t.Fatalf("opened links %q", got)
	}

	unsafe := strings.Index(current, "script")
	unsafeRect, ok := d.RectUTF16(utf8.RuneCountInString(current[:unsafe]))
	if !ok {
		t.Fatal("missing unsafe link caret")
	}
	tt.Move(unsafeRect.X+.1, unsafeRect.Y+unsafeRect.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("Open link"); ok {
		t.Fatal("Preview offered to open a script URL")
	}
}

func TestInlineHTMLRemainsLiteralInPreview(t *testing.T) {
	const src = `Paragraph with <span class="hl">inline HTML</span> and <br> break.`
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 600, 280)
	tt.Frame()
	preview, ok := tt.Access("Markdown Preview")
	if !ok || !strings.Contains(preview.Value, `<span class="hl">inline HTML</span>`) || !strings.Contains(preview.Value, "<br>") {
		t.Fatalf("inline HTML no longer appears as literal text: %q", preview.Value)
	}
	if d.Source() != src {
		t.Fatal("presenting inline HTML changed the source")
	}
}
