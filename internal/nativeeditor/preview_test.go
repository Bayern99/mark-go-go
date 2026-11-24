package nativeeditor_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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

func TestPreviewTaskCheckboxEditsSourceAndSharesUndo(t *testing.T) {
	const src = "- [ ] Finish task\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 560, 300)
	tt.Frame()
	if err := tt.Click("Task: Finish task"); err != nil {
		t.Fatal(err)
	}
	if got, want := d.Source(), "- [x] Finish task\r\n"; got != want {
		t.Fatalf("checking task wrote %q; want %q", got, want)
	}
	if node, ok := tt.Access("Task: Finish task"); !ok || !node.Checked {
		t.Fatalf("checked task is not exposed as a checked control: %+v", node)
	}
	if !d.CanUndo() || !d.Undo() || d.Source() != src {
		t.Fatalf("Undo did not restore task source bytes: %q", d.Source())
	}
	if !d.Redo() || d.Source() != "- [x] Finish task\r\n" {
		t.Fatal("Redo did not restore the task toggle")
	}
}

func TestPreviewTaskCheckboxUsesAccentCheckedColors(t *testing.T) {
	d := nativeeditor.New("- [X] Done\r\n- [ ] Next\r\n")
	theme := portrait.Theme()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Themed(c, &theme).Fill().Children(func() { d.View(c, false).Fill() })
	}, 560, 300)
	tt.Frame()

	checked, ok := tt.AccessByRole(ui.RoleCheckBox, "Task: Done")
	if !ok || !checked.Checked || checked.Disabled {
		t.Fatalf("checked task accessibility = %+v, %v", checked, ok)
	}
	unchecked, ok := tt.AccessByRole(ui.RoleCheckBox, "Task: Next")
	if !ok || unchecked.Checked || unchecked.Disabled {
		t.Fatalf("unchecked task accessibility = %+v, %v", unchecked, ok)
	}

	rgba := func(c ui.Color) color.RGBA { return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }
	over := func(front, back ui.Color) color.RGBA {
		blend := func(a, b uint8) uint8 {
			return uint8((int(a)*int(front.A) + int(b)*(255-int(front.A)) + 127) / 255)
		}
		return color.RGBA{R: blend(front.R, back.R), G: blend(front.G, back.G), B: blend(front.B, back.B), A: 255}
	}
	pixel := func(r ui.Rect, x, y float32) color.RGBA {
		return tt.Image().RGBAAt(int(r.X+x), int(r.Y+y))
	}
	if got, want := pixel(checked.Bounds, checked.Bounds.W/2, 3), rgba(portrait.Tones.Accent.Solid); got != want {
		t.Errorf("checked task fill = %v; want accent color %v", got, want)
	}
	if got, want := pixel(checked.Bounds, 0, checked.Bounds.H/2), rgba(portrait.Tones.Accent.Solid); got != want {
		t.Errorf("checked task border = %v; want accent color %v", got, want)
	}
	mark := rgba(portrait.Tones.Accent.OnSolid)
	foundMark := false
	for y := int(checked.Bounds.Y); y < int(checked.Bounds.Y+checked.Bounds.H); y++ {
		for x := int(checked.Bounds.X); x < int(checked.Bounds.X+checked.Bounds.W); x++ {
			px := tt.Image().RGBAAt(x, y)
			if x < int(checked.Bounds.X+ui.LightTheme().Space(4)) {
				foundMark = foundMark || px == mark
			}
		}
	}
	if !foundMark {
		t.Error("checked task has no paper-colored check mark")
	}

	if got, want := pixel(unchecked.Bounds, unchecked.Bounds.W/2, unchecked.Bounds.H/2), rgba(theme.Background); got != want {
		t.Errorf("unchecked task fill = %v; want %v", got, want)
	}
	if got, want := pixel(unchecked.Bounds, 0, unchecked.Bounds.H/2), over(portrait.Colors.LineStrong, portrait.Colors.Paper); got != want {
		t.Errorf("unchecked task border = %v; want %v", got, want)
	}
	tt.Move(unchecked.Bounds.X+unchecked.Bounds.W/2, unchecked.Bounds.Y+unchecked.Bounds.H/2)
	tt.Frame()
	if got, want := pixel(unchecked.Bounds, 0, unchecked.Bounds.H/2), rgba(theme.Accent); got != want {
		t.Errorf("hovered unchecked task border = %v; want %v", got, want)
	}

	if glyph, ok := d.RectUTF16(6); ok {
		boxCenter := checked.Bounds.Y + checked.Bounds.H/2
		textCenter := glyph.Y + glyph.H/2
		if delta := float32(math.Abs(float64(boxCenter - textCenter))); delta > 4 {
			t.Errorf("checkbox center %.1f is %.1f points from text center %.1f", boxCenter, delta, textCenter)
		}
	}

	if d.Source() != "- [X] Done\r\n- [ ] Next\r\n" || d.Modified() {
		t.Fatal("drawing or hovering task checkboxes changed the source")
	}
}

func TestPreviewTaskCheckboxReadOnlyKeepsStateAndSource(t *testing.T) {
	const src = "- [x] Locked\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, true).Fill() }, 560, 300)
	tt.Frame()
	node, ok := tt.AccessByRole(ui.RoleCheckBox, "Task: Locked")
	if !ok || !node.Checked || !node.Disabled {
		t.Fatalf("read-only task accessibility = %+v, %v", node, ok)
	}
	tt.ClickAt(node.Bounds.X+node.Bounds.W/2, node.Bounds.Y+node.Bounds.H/2)
	tt.Key(0, ui.KeySpace)
	if d.Source() != src || d.CanUndo() {
		t.Fatalf("read-only task changed source or history: %q, undo %v", d.Source(), d.CanUndo())
	}
}

func TestCodeFenceLanguageLabelsPreserveSourceCase(t *testing.T) {
	for _, tc := range []struct {
		name, info, label string
	}{
		{name: "lowercase", info: "bash", label: "bash"},
		{name: "mixed case", info: "Go", label: "Go"},
		{name: "metadata", info: `TypeScript title="demo.ts"`, label: "TypeScript"},
		{name: "empty", label: "Code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "```" + tc.info + "\r\nMixedCase()\r\n```\r\n"
			d := nativeeditor.New(src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 680, 300)
			tt.Frame()
			if _, ok := tt.Find(tc.label); !ok {
				t.Fatalf("code fence language %q was not shown", tc.label)
			}
			if upper := strings.ToUpper(tc.label); upper != tc.label {
				if _, ok := tt.Find(upper); ok {
					t.Fatalf("code fence language was forced to %q", upper)
				}
			}
			if d.Source() != src || d.Modified() {
				t.Fatal("presenting a code fence changed its source")
			}
			if tc.info == "bash" {
				if err := tt.Click("Copy code"); err != nil {
					t.Fatal(err)
				}
				if got := tt.Clipboard(); got != "MixedCase()" {
					t.Fatalf("copied code %q; want original body", got)
				}
			}
		})
	}
}

func TestOrdinaryCodeFenceKeepsItsSourceCaretInPreview(t *testing.T) {
	const src = "Before\r\n\r\n```go\r\nfmt.Println(\"iW\")\r\nsecond()\r\n```  \r\nAfter\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 760, 420)
	tt.Frame()
	if _, ok := tt.Find("Edit code"); ok {
		t.Fatal("ordinary code fence opened the special-block editor")
	}
	if _, ok := tt.Find("Code block: go"); !ok {
		t.Fatal("Preview did not show the code language")
	}
	if err := tt.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	if got, want := tt.Clipboard(), "fmt.Println(\"iW\")\nsecond()"; got != want {
		t.Fatalf("copied code %q; want %q", got, want)
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

func TestPreviewCodeFenceUsesNativeLanguageTokenColors(t *testing.T) {
	const src = "```go\nfunc main() { return 42 }\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 680, 360)
	tt.Frame()
	start := utf8.RuneCountInString(src[:strings.Index(src, "func")])
	first, ok := d.RectUTF16(start)
	if !ok {
		t.Fatal("missing code keyword bounds")
	}
	last, ok := d.RectUTF16(start + len("func"))
	if !ok {
		t.Fatal("missing code keyword end bounds")
	}
	image := tt.Image()
	accent := ui.LightTheme().Accent
	for y := int(first.Y); y < int(first.Y+first.H); y++ {
		for x := int(first.X); x <= int(last.X+2); x++ {
			px := image.RGBAAt(x, y)
			if colorNear(px, accent) {
				return
			}
		}
	}
	t.Fatal("Preview did not color the language keyword")
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
	const src = "~~~python title=\"demo.py\"\ndef f():\n    return \"~~~ inside\"\n~~~\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 720, 360)
	tt.Frame()
	if _, ok := tt.Find("Edit code"); !ok {
		t.Fatal("code fence metadata lost its in-place Markdown editor")
	}
	if _, ok := tt.Find("python"); !ok {
		t.Fatal("code fence language was not shown separately with its original case")
	}
	if _, ok := tt.Find("PYTHON"); ok {
		t.Fatal("code fence language was forced to uppercase")
	}
	if _, ok := tt.Find(`title="demo.py"`); !ok {
		t.Fatal("code fence metadata was not shown separately")
	}
	if d.Source() != src || d.Modified() {
		t.Fatal("presenting code metadata changed the source")
	}
	if err := tt.Click("Edit code"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if draft, ok := tt.Access("Markdown block editor"); !ok || draft.Value != strings.TrimSuffix(src, "\n") {
		t.Fatalf("metadata fence edit draft did not preserve the raw fence: %q", draft.Value)
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.Source() != src || d.Modified() {
		t.Fatal("canceling code metadata edit changed the source")
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

func TestStandaloneImagesUseReadingWidthAndKeepSourceCopyAndCaret(t *testing.T) {
	dir := t.TempDir()
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180" viewBox="0 0 320 180"><rect width="320" height="180" fill="#e11d48"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "figure.svg"), []byte(svg), 0600); err != nil {
		t.Fatal(err)
	}
	const src = "![Chart alt](figure.svg \"figure title\")\r\n\r\nAfter text.\r\n"
	d := nativeeditor.New(src)
	d.SetImageDirectory(dir)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 820, 560)
	tt.Frame()
	figure := waitForImage(t, tt, "Chart alt", func(px color.RGBA) bool {
		return px.R > 200 && px.G < 80 && px.B < 160
	})
	if figure.W < 300 || figure.H < 170 || figure.W > 624 {
		t.Fatalf("standalone image bounds %v; expected reading-size image within 624 DIPs", figure)
	}
	if err := d.SelectUTF16(0, utf8.RuneCountInString(src)); err != nil {
		t.Fatal(err)
	}
	tt.Command("copy")
	md, err := tt.ClipboardData().Read(transfer.Markdown)
	if err != nil || string(md) != src {
		t.Fatalf("copy changed image Markdown %q: %v", md, err)
	}
	pos := utf8.RuneCountInString(src[:strings.Index(src, "After text")])
	r, ok := d.RectUTF16(pos)
	if !ok {
		t.Fatal("missing source caret after standalone image")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	tt.Type("Next ")
	tt.Frame()
	if want := strings.Replace(src, "After text", "Next After text", 1); d.Source() != want {
		t.Fatalf("typing after the figure changed other source bytes: %q", d.Source())
	}
}

func TestRemoteAndDataImagesLoadWithinLimits(t *testing.T) {
	const remoteSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90" viewBox="0 0 160 90"><rect width="160" height="90" fill="#2563eb"/></svg>`
	const largeHead = `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90" viewBox="0 0 160 90"><rect width="160" height="90" fill="#e11d48"/><!--`
	largeSVG := largeHead + strings.Repeat(" ", 5<<20) + "--></svg>"
	pngImage := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			pngImage.SetRGBA(x, y, color.RGBA{R: 22, G: 163, B: 74, A: 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, pngImage); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "image/svg+xml")
		if r.URL.Path == "/large.svg" {
			w.(http.Flusher).Flush()
			_, _ = w.Write([]byte(largeSVG))
			return
		}
		_, _ = w.Write([]byte(remoteSVG))
	}))
	defer server.Close()
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData.Bytes())
	src := "![Remote figure](" + server.URL + "/remote.svg)\n\n![Data figure](<" + dataURL + ">)\n\n![Large fallback](" + server.URL + "/large.svg)\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 820, 700)
	tt.Frame()
	blue := waitForImage(t, tt, "Remote figure", func(px color.RGBA) bool {
		return px.B > 200 && px.R < 100
	})
	if blue.W < 150 || blue.W > 624 {
		t.Fatalf("remote figure bounds %v", blue)
	}
	green := waitForImage(t, tt, "Data figure", func(px color.RGBA) bool {
		return px.G > 150 && px.R < 80
	})
	if green.W < 110 || green.W > 624 {
		t.Fatalf("data image bounds %v", green)
	}
	if requests.Load() != 2 {
		t.Fatalf("remote image requests %d; want only the two local test-server requests", requests.Load())
	}
	large, ok := tt.Find("Large fallback")
	if !ok || large.H > 40 {
		t.Fatalf("oversized remote image did not fall back to its alt text: %v %v", large, ok)
	}
	if d.Source() != src {
		t.Fatal("loading image URLs changed the Markdown source")
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

func TestLinkDestinationCanBeEditedWithExactBytesAndSharedUndo(t *testing.T) {
	const src = "Before [label](https://old.example/a \"title\") after.\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 700, 360)
	tt.Frame()
	position := utf8.RuneCountInString(src[:strings.Index(src, "label")])
	r, ok := d.RectUTF16(position)
	if !ok {
		t.Fatal("missing link label")
	}
	tt.Move(r.X+.1, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Edit link"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Link destination"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("javascript:alert(1)")
	tt.Frame()
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	want := "Before [label](javascript:alert(1) \"title\") after.\r\n"
	if d.Source() != want {
		t.Fatalf("destination edit changed surrounding Markdown: %q", d.Source())
	}
	unsafePosition := utf8.RuneCountInString(want[:strings.Index(want, "label")])
	unsafeRect, ok := d.RectUTF16(unsafePosition)
	if !ok {
		t.Fatal("missing edited unsafe link label")
	}
	tt.Move(unsafeRect.X+.1, unsafeRect.Y+unsafeRect.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("Open link"); ok {
		t.Fatal("editing a link to an unsafe URL made it openable")
	}
	if _, ok := tt.Find("Edit link"); !ok {
		t.Fatal("unsafe URL lost its link edit action")
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("Undo did not restore link destination bytes: %q", d.Source())
	}
	if !d.Redo() || d.Source() != want {
		t.Fatalf("Redo did not restore the unsafe URL edit: %q", d.Source())
	}
}

func TestLinkCanBeRemovedWithoutChangingItsLabelOrSurroundingBytes(t *testing.T) {
	const src = "Before [**bold**](https://example.com/path \"title\") after.\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 700, 360)
	tt.Frame()
	position := utf8.RuneCountInString(src[:strings.Index(src, "bold")])
	r, ok := d.RectUTF16(position)
	if !ok {
		t.Fatal("missing link label")
	}
	tt.Move(r.X+.1, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Remove link"); err != nil {
		t.Fatal(err)
	}
	want := "Before **bold** after.\r\n"
	if d.Source() != want {
		t.Fatalf("removing link changed label or surrounding Markdown: %q", d.Source())
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("Undo did not restore link markup: %q", d.Source())
	}
}

func TestReferenceLinkDestinationCanBeEditedAtItsDefinition(t *testing.T) {
	const src = "Read [guide][docs].\r\n\r\n[docs]: <https://old.example/a> \"title\"\r\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 700, 360)
	tt.Frame()
	position := utf8.RuneCountInString(src[:strings.Index(src, "guide")])
	r, ok := d.RectUTF16(position)
	if !ok {
		t.Fatal("missing reference link label")
	}
	tt.Move(r.X+.1, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Edit link"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Link destination"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("https://new.example/b")
	tt.Frame()
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	want := "Read [guide][docs].\r\n\r\n[docs]: <https://new.example/b> \"title\"\r\n"
	if d.Source() != want {
		t.Fatalf("reference link edit changed bytes outside its definition destination: %q", d.Source())
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("Undo did not restore reference definition: %q", d.Source())
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

func waitForImage(t *testing.T, tt *ui.Tester, label string, matches func(color.RGBA) bool) ui.Rect {
	t.Helper()
	for range 300 {
		tt.Frame()
		if r, ok := tt.Find(label); ok && r.W > 0 && r.H > 0 {
			pixel := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2))
			if matches(pixel) {
				return r
			}
		}
		time.Sleep(time.Millisecond)
	}
	r, ok := tt.Find(label)
	t.Fatalf("image %q did not render: %v %v", label, r, ok)
	return ui.Rect{}
}

func colorNear(px color.RGBA, target ui.Color) bool {
	distance := absInt(int(px.R)-int(target.R)) + absInt(int(px.G)-int(target.G)) + absInt(int(px.B)-int(target.B))
	return distance < 100 && px.A > 200
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
