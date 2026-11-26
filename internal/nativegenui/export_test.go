package nativegenui

import (
	"context"
	"encoding/json"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/pdf"
	"github.com/Bayern99/mark-go-go/internal/macos"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestOptionsShareReaderNotesPreference(t *testing.T) {
	win := &nativeui.Window{}
	panel := &panel{win: win}
	view := &documentView{}
	app := ui.NewTester(func(c *ui.Context) {
		panel.drawOptions(c, view, workspace.DocumentInfo{Path: "guide.md"}, false)
	}, 640, 400)
	app.Frame()
	if !app.HasText("Export HTML…") || !app.HasText("Export PDF…") || !app.HasText("Include reader notes") {
		t.Fatalf("export options = %v", app.Texts())
	}
	if err := app.Click("Include reader notes"); err != nil {
		t.Fatal(err)
	}
	if !win.IncludeReaderNotes() {
		t.Fatal("GenUI checkbox did not persist the shared preference")
	}
}

func exportStatementView(t *testing.T, statements ...string) *Surface {
	t.Helper()
	components, _ := json.Marshal(statements)
	view, err := Open(`{"version":3,"language":"openui-lang/1.0","sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":` + string(components) + `}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func captureGenUISave(t *testing.T) chan export.Result {
	t.Helper()
	previousChoose := genUIChoosePath
	previous := genUISave
	saved := make(chan export.Result, 1)
	genUIChoosePath = func(_ *nativeui.Window, defaultPath, _ string) (string, error) { return defaultPath, nil }
	genUISave = func(_ *nativeui.Window, _ string, result export.Result) { saved <- result }
	t.Cleanup(func() {
		genUIChoosePath = previousChoose
		genUISave = previous
	})
	return saved
}

func captureGenUISavePDF(t *testing.T) chan export.Result {
	t.Helper()
	previousChoose := genUIChoosePath
	previous := genUISavePDF
	saved := make(chan export.Result, 1)
	genUIChoosePath = func(_ *nativeui.Window, defaultPath, _ string) (string, error) { return defaultPath, nil }
	genUISavePDF = func(_ *nativeui.Window, _ string, result export.Result) { saved <- result }
	t.Cleanup(func() {
		genUIChoosePath = previousChoose
		genUISavePDF = previous
	})
	return saved
}

func waitGenUISave(t *testing.T, saved <-chan export.Result) export.Result {
	t.Helper()
	select {
	case result := <-saved:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("export did not reach the save callback")
		return export.Result{}
	}
}

func TestLegacyViewExportIsRejectedBeforeSaving(t *testing.T) {
	files := map[string]string{
		"v1": `{"version":1,"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Legacy"}]}}]}`,
		"v2": `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Legacy"}]}}]}`,
	}
	for name, file := range files {
		t.Run(name, func(t *testing.T) {
			if !json.Valid([]byte(file)) {
				t.Fatal("invalid legacy fixture")
			}
			surface := &Surface{}
			if name == "v2" {
				var err error
				surface, err = Open(file)
				if err != nil {
					t.Fatal(err)
				}
			}
			view := &documentView{options: true}
			view.reading.Load(surface)
			win := &nativeui.Window{}
			panel := &panel{win: win}
			saved := captureGenUISave(t)
			previousNotify := genUINotify
			var notification portrait.ToastSpec
			genUINotify = func(_ *nativeui.Window, spec portrait.ToastSpec) { notification = spec }
			t.Cleanup(func() { genUINotify = previousNotify })
			panel.exportHTML(view, workspace.DocumentInfo{Path: "guide.md"})
			if notification.ID != "genui-export" || notification.Tone != portrait.Tones.Warning || notification.Title != "Export currently supports GenUI v3 only. Regenerate the view to export." {
				t.Fatalf("notification = %+v", notification)
			}
			select {
			case <-saved:
				t.Fatal("legacy export invoked the save callback")
			default:
			}
		})
	}
}

func TestExportSnapshotCapturesNativeControlValues(t *testing.T) {
	surface := exportStatementView(t,
		`$bound = "initial"`,
		`$boundNum = 5`,
		`$boundTimer = 120`,
		`root = Col([Input($bound, "Bound"), Input("literal", "Literal"), Form([Input("form", "Form field")]), Timer(120, "Literal timer"), Timer($boundTimer, "Bound timer"), Input($boundNum, "Bound number", "number")])`)
	view := &documentView{}
	view.reading.Load(surface)
	app := ui.NewTester(view.reading.View, 640, 960)
	app.Frame()
	app.Frame()
	for _, edit := range []struct{ label, value string }{{"Bound", "edited bound"}, {"Literal", "edited literal"}, {"Form field", "edited form"}} {
		field, ok := app.Access(edit.label)
		if !ok {
			t.Fatalf("missing input %q", edit.label)
		}
		app.ClickAtWith(0, field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
		app.Frame()
		if !app.Focused(edit.label) {
			t.Fatalf("%q did not receive focus", edit.label)
		}
		app.Key(ui.Cmd, ui.KeyA)
		app.Type(edit.value)
		app.Frame()
		app.Frame()
		field, _ = app.Access(edit.label)
		if field.Value != edit.value {
			t.Fatalf("%q value = %q, want %q", edit.label, field.Value, edit.value)
		}
	}
	if err := app.Click("Start"); err != nil {
		t.Fatal(err)
	}
	app.Advance(2 * time.Second)
	app.Frame()
	app.Frame()
	number, ok := app.Access("Bound number")
	if !ok {
		t.Fatal("missing bound number input")
	}
	app.ClickAtWith(0, number.Bounds.X+number.Bounds.W/2, number.Bounds.Y+number.Bounds.H/2)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyA)
	app.Type("7.")
	app.Frame()
	app.Frame()
	if state := view.reading.states["root/4"]; state != nil {
		state.running = true
		state.date = time.Now().Add(-2 * time.Second)
	}

	saved := captureGenUISave(t)
	panel := &panel{win: &nativeui.Window{}}
	panel.exportHTML(view, workspace.DocumentInfo{Path: "guide.md"})
	surface.Session.Set("$bound", "changed after snapshot")
	surface.Session.SetField("", "Input", "Literal", "changed after snapshot")
	result := waitGenUISave(t, saved)
	text := string(result.Data)
	for _, want := range []string{"edited bound", "edited literal", "edited form", ">7.</div>"} {
		if !strings.Contains(text, want) {
			t.Errorf("snapshot output is missing %q: %s", want, text)
		}
	}
	if strings.Count(text, "01:58") < 2 {
		t.Errorf("expected both literal and bound running timers to render 01:58: %s", text)
	}
	if strings.Contains(text, "changed after snapshot") {
		t.Fatal("background rendering read the mutable session instead of the snapshot")
	}
}

func TestRealV3ExportLinksRepeatedIdenticalSections(t *testing.T) {
	file := `{"version":3,"language":"openui-lang/1.0","sources":{"first":{"heading":"Repeat","occurrence":1},"second":{"heading":"Repeat","occurrence":2}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Col([Source([\"first\"], [Text(\"First\")]), Source([\"second\"], [Text(\"Second\")])])"]}}]}`
	surface, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	view := &documentView{}
	view.reading.Load(surface)
	view.index("# Repeat\n\nSame.\n<!-- note: first repeated note -->\n# Repeat\n\nSame.\n<!-- note: second repeated note -->\n")
	if len(view.notes) != 2 || view.notes[0].SectionID != view.notes[1].SectionID {
		t.Fatalf("test sections do not share a content hash: %+v", view.notes)
	}
	win := &nativeui.Window{}
	win.SetPreference(nativeui.ReaderNotesPreference, "on")
	panel := &panel{win: win}
	saved := captureGenUISave(t)
	panel.exportHTML(view, workspace.DocumentInfo{Path: "guide.md"})
	result := waitGenUISave(t, saved)
	text := string(result.Data)
	for _, want := range []string{"first repeated note", "second repeated note", `id="notes-repeat-1"`, `id="notes-repeat-2"`} {
		if !strings.Contains(text, want) {
			t.Errorf("repeated-section output is missing %q: %s", want, text)
		}
	}
	forward := regexp.MustCompile(`class="g-source-link" id="([^"]+)" href="#([^"]+)"`).FindAllStringSubmatch(text, -1)
	backward := regexp.MustCompile(`class="g-note-back" href="#([^"]+)"`).FindAllStringSubmatch(text, -1)
	if len(forward) != 2 || len(backward) != 2 || forward[0][2] == forward[1][2] {
		t.Fatalf("forward = %v, backward = %v", forward, backward)
	}
	ids := map[string]bool{}
	for _, link := range forward {
		if strings.Count(text, `id="`+link[2]+`"`) != 1 || strings.Count(text, `id="`+link[1]+`"`) != 1 {
			t.Errorf("forward link does not have unique endpoints: %v", link)
		}
		ids[link[1]] = true
	}
	for _, link := range backward {
		if !ids[link[1]] {
			t.Errorf("backlink has no matching source badge: %v", link)
		}
	}
}

func TestV3PDFExportUsesSnapshotAndPDFOptions(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("built-in PDF export is registered only on macOS")
	}
	surface := exportStatementView(t, `$text = "snapshot before"`, `root = Text($text)`)
	view := &documentView{}
	view.reading.Load(surface)
	win := &nativeui.Window{}
	win.SetPreference("mgo.appearance", "Dark")
	win.SetPreference(nativeui.KeepDarkPDFPreference, "on")
	win.SetPreference(nativeui.PaperSizePreference, "letter")
	panel := &panel{win: win}
	saved := captureGenUISavePDF(t)
	previousPrint := nativeui.PrintPDF
	t.Cleanup(func() { nativeui.PrintPDF = previousPrint })
	nativeui.PrintPDF = func(_ context.Context, html []byte, opts pdf.Options) ([]byte, error) {
		text := string(html)
		if !strings.Contains(text, "snapshot before") || strings.Contains(text, "changed after snapshot") {
			t.Errorf("PDF did not print the captured snapshot: %s", text)
		}
		if opts.Paper != pdf.PaperLetter || opts.Background != macos.SupportsPDFBackground() {
			t.Errorf("PDF options = %+v", opts)
		}
		return []byte("%PDF-genui"), nil
	}
	panel.exportPDF(view, workspace.DocumentInfo{Name: "Guide.md", Path: "guide.md"})
	surface.Session.Set("$text", "changed after snapshot")
	result := waitGenUISave(t, saved)
	if string(result.Data) != "%PDF-genui" {
		t.Fatalf("saved PDF result = %q", result.Data)
	}
}

func TestLegacyViewPDFExportIsRejectedBeforePrintingOrSaving(t *testing.T) {
	files := map[string]string{
		"v1": `{"version":1,"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Legacy"}]}}]}`,
		"v2": `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Legacy"}]}}]}`,
	}
	for name, file := range files {
		t.Run(name, func(t *testing.T) {
			surface := &Surface{}
			if name == "v2" {
				var err error
				surface, err = Open(file)
				if err != nil {
					t.Fatal(err)
				}
			}
			view := &documentView{}
			view.reading.Load(surface)
			panel := &panel{win: &nativeui.Window{}}
			saved := captureGenUISavePDF(t)
			previousPrint := nativeui.PrintPDF
			prints := 0
			nativeui.PrintPDF = func(context.Context, []byte, pdf.Options) ([]byte, error) {
				prints++
				return nil, nil
			}
			t.Cleanup(func() { nativeui.PrintPDF = previousPrint })
			previousNotify := genUINotify
			var notification portrait.ToastSpec
			genUINotify = func(_ *nativeui.Window, spec portrait.ToastSpec) { notification = spec }
			t.Cleanup(func() { genUINotify = previousNotify })
			panel.exportPDF(view, workspace.DocumentInfo{Path: "guide.md"})
			if notification.Tone != portrait.Tones.Warning || !strings.Contains(notification.Title, "GenUI v3 only") {
				t.Fatalf("notification = %+v", notification)
			}
			if prints != 0 {
				t.Fatalf("legacy export invoked printer %d times", prints)
			}
			select {
			case <-saved:
				t.Fatal("legacy PDF export invoked the save callback")
			default:
			}
		})
	}
}
