package nativeui

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/pandoc"
	"github.com/Bayern99/mark-go-go/internal/export/pdf"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestReaderNotesPreferenceDefaultsOffAndSynchronizes(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { delete(preferenceStores, dir) })
	first := &Window{preferences: preferencesFor(dir)}
	second := &Window{preferences: preferencesFor(dir)}
	if first.IncludeReaderNotes() || second.IncludeReaderNotes() {
		t.Fatal("Reader Notes default to enabled")
	}
	previous := AfterReaderNotesChange
	previousExport := AfterExportPreferenceChange
	t.Cleanup(func() {
		AfterReaderNotesChange = previous
		AfterExportPreferenceChange = previousExport
	})
	changes := 0
	exportChanges := 0
	AfterReaderNotesChange = func(bool) { changes++ }
	AfterExportPreferenceChange = func() { exportChanges++ }
	first.SetPreference(ReaderNotesPreference, "on")
	if !second.IncludeReaderNotes() || changes != 1 || exportChanges != 1 {
		t.Fatalf("shared preference = %v, changes = %d, export changes = %d", second.IncludeReaderNotes(), changes, exportChanges)
	}
	data, err := os.ReadFile(filepath.Join(dir, "portrait.json"))
	if err != nil || !strings.Contains(string(data), `"mgo.export.readerNotes":"on"`) {
		t.Fatalf("portrait.json = %s, %v", data, err)
	}
	second.SetPreference(ReaderNotesPreference, "off")
	if first.IncludeReaderNotes() || changes != 2 || exportChanges != 2 {
		t.Fatalf("disabled preference = %v, changes = %d, export changes = %d", first.IncludeReaderNotes(), changes, exportChanges)
	}
	first.SetPreference(KeepDarkPDFPreference, "on")
	if !second.KeepDarkPDFEnabled() || !ProfileKeepDarkPDFEnabled(dir) || exportChanges != 3 {
		t.Fatalf("Keep Dark preference = %v, profile = %v, export changes = %d", second.KeepDarkPDFEnabled(), ProfileKeepDarkPDFEnabled(dir), exportChanges)
	}
	first.ResetPreferences(KeepDarkPDFPreference)
	if second.KeepDarkPDFEnabled() || exportChanges != 4 {
		t.Fatalf("Keep Dark reset = %v, export changes = %d", second.KeepDarkPDFEnabled(), exportChanges)
	}
}

func TestPandocPathPreferenceTriggersAsyncProbeAndMenuRefresh(t *testing.T) {
	w := pluginWindow(map[string]string{})
	w.preferences.windows[w] = true
	previousProbe := ProbePandoc
	previousChange := AfterExportPreferenceChange
	t.Cleanup(func() {
		ProbePandoc = previousProbe
		AfterExportPreferenceChange = previousChange
	})
	ProbePandoc = func(_ context.Context, path string, _ ...[]string) pandoc.Status {
		return pandoc.Status{Available: true, Path: path, Version: "3.13"}
	}
	refreshed := make(chan struct{}, 2)
	AfterExportPreferenceChange = func() { refreshed <- struct{}{} }
	w.SetPreference(PandocPathPreference, "/custom/pandoc")
	for i := 0; i < 2; i++ {
		select {
		case <-refreshed:
		case <-time.After(5 * time.Second):
			t.Fatal("Pandoc preference did not refresh the menu and probe")
		}
	}
	status := ProfilePandocStatus("")
	if !status.Available || status.Path != "/custom/pandoc" || status.Version != "3.13" {
		t.Fatalf("cached Pandoc status = %+v", status)
	}
}

func TestExportOptionsCapturePortraitAndDocument(t *testing.T) {
	w := pluginWindow(map[string]string{ReaderNotesPreference: "on", spacePreference: "Water"})
	w.shell.dark = true
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: "doc", Documents: []workspace.DocumentInfo{{ID: "doc", Name: "Guide.md", Path: filepath.Join("docs", "Guide.md")}}}
	options := w.ExportOptions("Guide")
	if options.SpaceName != "Water" || !options.Dark || !options.PrintLight || !options.IncludeNotes || options.BaseDir != "docs" || options.Title != "Guide" {
		t.Fatalf("options = %+v", options)
	}
	for _, tc := range []struct{ ext, want string }{{".html", filepath.Join("docs", "Guide.html")}, {".pdf", filepath.Join("docs", "Guide.pdf")}} {
		if got := exportPath(filepath.Join("docs", "Guide.md"), "Guide.md", tc.ext); got != tc.want {
			t.Fatalf("exportPath(%q) = %q, want %q", tc.ext, got, tc.want)
		}
	}
}

func TestExportPDFOptions(t *testing.T) {
	previous := supportsPDFBackground
	t.Cleanup(func() { supportsPDFBackground = previous })
	for _, tc := range []struct {
		name                      string
		dark, keep, support       bool
		wantLight, wantBackground bool
	}{
		{name: "light appearance", keep: true, support: true, wantLight: true},
		{name: "dark defaults to light paper", dark: true, support: true, wantLight: true},
		{name: "dark preference preserves dark", dark: true, keep: true, support: true, wantBackground: true},
		{name: "old macOS forces light paper", dark: true, keep: true, wantLight: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supportsPDFBackground = func() bool { return tc.support }
			value := "off"
			if tc.keep {
				value = "on"
			}
			w := pluginWindow(map[string]string{KeepDarkPDFPreference: value, PaperSizePreference: "letter"})
			w.shell.dark = tc.dark
			exportOpts, pdfOpts := w.ExportPDFOptions("Guide")
			if exportOpts.Dark != tc.dark || exportOpts.PrintLight != tc.wantLight || !exportOpts.OfflineImages || pdfOpts.Background != tc.wantBackground || pdfOpts.Paper != pdf.PaperLetter {
				t.Fatalf("export = %+v, PDF = %+v", exportOpts, pdfOpts)
			}
		})
	}
	if PaperSize("") != pdf.PaperA4 || PaperSize("a4") != pdf.PaperA4 || PaperSize("letter") != pdf.PaperLetter || PaperSize("invalid") != pdf.PaperA4 {
		t.Fatal("paper size preference did not map to A4 or Letter")
	}
}

func TestMarkdownPDFExportRendersPrintsAndWritesPDF(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("built-in PDF export is registered only on macOS")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "Guide.md")
	w := pluginWindow(map[string]string{KeepDarkPDFPreference: "on", PaperSizePreference: "letter"})
	w.shell.dark = true
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: "doc", Documents: []workspace.DocumentInfo{{ID: "doc", Name: "Guide.md", Path: path}}}
	w.documents = map[string]*openDocument{"doc": {source: nativeeditor.New("# Guide\n\nHello PDF.")}}

	previousSupport := supportsPDFBackground
	previousPrint := PrintPDF
	previousDialog := saveExportDialog
	previousWrite := writeExportFile
	t.Cleanup(func() {
		supportsPDFBackground = previousSupport
		PrintPDF = previousPrint
		saveExportDialog = previousDialog
		writeExportFile = previousWrite
	})
	supportsPDFBackground = func() bool { return true }
	var printedHTML string
	var printedOpts pdf.Options
	PrintPDF = func(_ context.Context, html []byte, opts pdf.Options) ([]byte, error) {
		printedHTML, printedOpts = string(html), opts
		return []byte("%PDF-1.7\nfake"), nil
	}
	target := filepath.Join(dir, "Chosen")
	saveExportDialog = func(opts mygo.SaveDialogOptions) (string, error) {
		if opts.DefaultPath != filepath.Join(dir, "Guide.pdf") || len(opts.Filters) != 1 || opts.Filters[0].Name != "PDF Document" || len(opts.Filters[0].Extensions) != 1 || opts.Filters[0].Extensions[0] != "pdf" {
			t.Errorf("save options = %+v", opts)
		}
		return target, nil
	}
	done := make(chan struct{})
	writeExportFile = func(path string, data []byte) error {
		err := previousWrite(path, data)
		close(done)
		return err
	}
	w.ExportMarkdown("markdown-pdf")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Markdown PDF export did not finish")
	}
	if !strings.Contains(printedHTML, "<html") || !strings.Contains(printedHTML, "Hello PDF.") || strings.Contains(printedHTML, "requestAnimationFrame") {
		t.Fatalf("printed HTML = %s", printedHTML)
	}
	if printedOpts.Paper != pdf.PaperLetter || !printedOpts.Background {
		t.Fatalf("printed options = %+v", printedOpts)
	}
	data, err := os.ReadFile(target + ".pdf")
	if err != nil || string(data) != "%PDF-1.7\nfake" {
		t.Fatalf("saved PDF = %q, %v", data, err)
	}
}

func TestMarkdownPDFCancelDialogBeforePrinting(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("built-in PDF export is registered only on macOS")
	}
	w := pluginWindow(map[string]string{})
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: "doc", Documents: []workspace.DocumentInfo{{ID: "doc", Name: "Guide.md", Path: filepath.Join(t.TempDir(), "Guide.md")}}}
	w.documents = map[string]*openDocument{"doc": {source: nativeeditor.New("# Guide")}}

	previousPrint := PrintPDF
	previousDialog := saveExportDialog
	t.Cleanup(func() {
		PrintPDF = previousPrint
		saveExportDialog = previousDialog
	})
	printed := false
	PrintPDF = func(context.Context, []byte, pdf.Options) ([]byte, error) {
		printed = true
		return nil, nil
	}
	dialogDone := make(chan struct{})
	saveExportDialog = func(mygo.SaveDialogOptions) (string, error) {
		defer close(dialogDone)
		return "", mygo.ErrCanceled
	}
	w.ExportMarkdown("markdown-pdf")
	select {
	case <-dialogDone:
	case <-time.After(5 * time.Second):
		t.Fatal("save dialog was not invoked")
	}
	time.Sleep(20 * time.Millisecond)
	if printed {
		t.Fatal("canceling the save dialog still started PDF printing")
	}
}

func TestExportSettingsPaperSizeAndReset(t *testing.T) {
	var shell Shell
	model := Model{Preferences: map[string]string{}}
	var reset []string
	actions := Actions{
		SetPreference: func(key, value string) { model.Preferences[key] = value },
		ResetPreferences: func(keys ...string) {
			reset = append(reset, keys...)
			for _, key := range keys {
				delete(model.Preferences, key)
			}
		},
	}
	page := settingsPageByID(shell.nativeSettingsPages(model, actions, Slots{}), "export")
	if page.ID != "export" || page.Section != "Workspace" || page.Label != "Export" || page.Order != 220 || page.Draw == nil || page.Reset == nil || page.Dirty == nil {
		t.Fatalf("Export settings page = %+v", page)
	}
	query := SettingsQuery{state: &settingsSearchState{}}
	app := ui.NewTester(func(c *ui.Context) { page.Draw(c, query) }, 640, 400)
	app.Frame()
	if !app.HasText("PDF") || !app.HasText("Paper size") {
		t.Fatalf("Export settings = %v", app.Texts())
	}
	if err := app.Click("US Letter"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if model.Preferences[PaperSizePreference] != "letter" || !page.Dirty() {
		t.Fatalf("paper preference = %v, dirty = %v", model.Preferences, page.Dirty())
	}
	page.Reset()
	if len(reset) != 2 || reset[0] != PaperSizePreference || reset[1] != PandocPathPreference || page.Dirty() {
		t.Fatalf("reset = %v, preferences = %v", reset, model.Preferences)
	}
	query = SettingsQuery{Text: "paper", state: &settingsSearchState{}}
	app.Frame()
	if !app.HasText("Paper size") {
		t.Fatal("Settings search did not find Paper size")
	}
	query = SettingsQuery{Text: "unrelated", state: &settingsSearchState{}}
	app.Frame()
	if app.HasText("Paper size") {
		t.Fatal("unrelated Settings search left Paper size visible")
	}
}

func TestPandocExportUnavailableShowsPortraitDialogWithoutSavePanel(t *testing.T) {
	w := pluginWindow(map[string]string{})
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: "doc", Documents: []workspace.DocumentInfo{{ID: "doc", Name: "Guide.md", Path: filepath.Join(t.TempDir(), "Guide.md")}}}
	w.documents = map[string]*openDocument{"doc": {source: nativeeditor.New("# Guide")}}
	previousProbe := ProbePandoc
	previousDialog := saveExportDialog
	previousShown := pandocDialogShown
	t.Cleanup(func() {
		ProbePandoc = previousProbe
		saveExportDialog = previousDialog
		pandocDialogShown = previousShown
	})
	ProbePandoc = func(context.Context, string, ...[]string) pandoc.Status {
		return pandoc.Status{Error: "Pandoc was not found", InstallHint: pandoc.InstallCommand}
	}
	saveCalled := make(chan struct{}, 1)
	saveExportDialog = func(mygo.SaveDialogOptions) (string, error) {
		saveCalled <- struct{}{}
		return "", nil
	}
	shown := make(chan struct{})
	pandocDialogShown = func() { close(shown) }
	w.ExportMarkdown("markdown-docx")
	select {
	case <-shown:
	case <-time.After(5 * time.Second):
		t.Fatal("Pandoc guidance was not shown")
	}
	if w.dialog == nil || w.dialog.kind != "pandoc" || !strings.Contains(w.dialog.body, "Pandoc was not found") {
		t.Fatalf("dialog = %+v", w.dialog)
	}
	select {
	case <-saveCalled:
		t.Fatal("save panel opened before Pandoc was available")
	default:
	}
}

func TestPandocGuidanceCopiesCommandAndOpensExportSettings(t *testing.T) {
	w := &Window{}
	w.dialog = &windowDialog{kind: "pandoc", open: true, title: "Pandoc is required", body: "Pandoc was not found"}
	app := ui.NewTester(func(c *ui.Context) { w.drawDialog(c) }, 640, 400)
	app.Frame()
	for _, text := range []string{"Pandoc is required", "Pandoc was not found", pandoc.InstallCommand, "Copy command", "Open Settings", "Close"} {
		if !app.HasText(text) {
			t.Errorf("Pandoc guidance missing %q: %v", text, app.Texts())
		}
	}
	if err := app.Click("Copy command"); err != nil {
		t.Fatal(err)
	}
	if app.Clipboard() != pandoc.InstallCommand {
		t.Fatalf("clipboard = %q", app.Clipboard())
	}
	if err := app.Click("Open Settings"); err != nil {
		t.Fatal(err)
	}
	if !w.shell.settingsOpen || w.shell.settingsTab != "export" || w.dialog != nil {
		t.Fatalf("Open Settings state = open %v, tab %q, dialog %+v", w.shell.settingsOpen, w.shell.settingsTab, w.dialog)
	}
}

func TestPandocExportRendersAfterSaveChoice(t *testing.T) {
	dir := t.TempDir()
	w := pluginWindow(map[string]string{PandocPathPreference: "/custom/pandoc"})
	w.snapshot = workspace.WorkspaceSnapshot{ActiveID: "doc", Documents: []workspace.DocumentInfo{{ID: "doc", Name: "Guide.md", Path: filepath.Join(dir, "Guide.md")}}}
	w.documents = map[string]*openDocument{"doc": {source: nativeeditor.New("# Guide")}}
	previousProbe := ProbePandoc
	previousRender := RenderPandoc
	previousDialog := saveExportDialog
	previousWrite := writeExportFile
	t.Cleanup(func() {
		ProbePandoc = previousProbe
		RenderPandoc = previousRender
		saveExportDialog = previousDialog
		writeExportFile = previousWrite
	})
	ProbePandoc = func(_ context.Context, path string, _ ...[]string) pandoc.Status {
		if path != "/custom/pandoc" {
			t.Errorf("probe path = %q", path)
		}
		return pandoc.Status{Available: true, Path: path, Version: "3.13"}
	}
	saveExportDialog = func(opts mygo.SaveDialogOptions) (string, error) {
		if opts.DefaultPath != filepath.Join(dir, "Guide.docx") || len(opts.Filters) != 1 || opts.Filters[0].Name != "Word Document" {
			t.Errorf("save options = %+v", opts)
		}
		return filepath.Join(dir, "chosen.docx"), nil
	}
	RenderPandoc = func(_ context.Context, source, formatID string, _ export.Options, status ...pandoc.Status) (export.Result, error) {
		if source != "# Guide" || formatID != "markdown-docx" || len(status) != 1 || status[0].Path != "/custom/pandoc" {
			t.Errorf("RenderPandoc arguments = %q %q %+v", source, formatID, status)
		}
		return export.Result{Data: []byte("docx")}, nil
	}
	done := make(chan struct{})
	writeExportFile = func(path string, data []byte) error {
		if path != filepath.Join(dir, "chosen.docx") || string(data) != "docx" {
			t.Errorf("write = %q %q", path, data)
		}
		close(done)
		return nil
	}
	w.ExportMarkdown("markdown-docx")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Pandoc export did not finish")
	}
}

func TestWriteExportWritesLaTeXAttachments(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Guide.tex")
	previousWrite := writeExportFile
	t.Cleanup(func() { writeExportFile = previousWrite })
	var writes []string
	writeExportFile = func(path string, data []byte) error {
		writes = append(writes, path)
		return previousWrite(path, data)
	}
	w := &Window{}
	w.WriteExport(target, export.Result{Data: []byte("\\includegraphics{guide-assets/image-1.png}"), Attachments: []export.Attachment{{Name: "guide-assets/image-1.png", Data: []byte("png")}}})
	if len(writes) != 1 || writes[0] != target {
		t.Fatalf("writes = %v, want only %s", writes, target)
	}
	data, err := os.ReadFile(filepath.Join(dir, "guide-assets", "image-1.png"))
	if err != nil || string(data) != "png" {
		t.Fatalf("attachment = %q, %v", data, err)
	}

	nextDir, err := chooseUniqueAssetDir(dir, "Guide.tex")
	if err != nil || nextDir != "guide-assets-2" {
		t.Fatalf("chooseUniqueAssetDir = %q, %v", nextDir, err)
	}

	conflictTarget := filepath.Join(dir, "Conflict.tex")
	writes = nil
	w.WriteExport(conflictTarget, export.Result{Data: []byte("should not write"), Attachments: []export.Attachment{{Name: "guide-assets/image-2.png", Data: []byte("shared dir")}}})
	if len(writes) != 0 {
		t.Fatalf("main file was written despite existing attachment directory: %v", writes)
	}
	if _, err := os.Stat(conflictTarget); !os.IsNotExist(err) {
		t.Fatalf("conflict target should not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "guide-assets", "image-2.png")); !os.IsNotExist(err) {
		t.Fatalf("shared attachment directory accepted a second export's file: %v", err)
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "escape-assets")); err == nil {
		escapeTarget := filepath.Join(dir, "Escape.tex")
		w.WriteExport(escapeTarget, export.Result{Data: []byte("should not write"), Attachments: []export.Attachment{{Name: "escape-assets/image-1.png", Data: []byte("x")}}})
		if _, err := os.Stat(filepath.Join(outside, "image-1.png")); !os.IsNotExist(err) {
			t.Fatal("attachment escaped through symlink")
		}
		if _, err := os.Stat(escapeTarget); !os.IsNotExist(err) {
			t.Fatal("main file was written despite symlink escape")
		}
	}
}

func TestFormatExportWarningsCapsHugeLists(t *testing.T) {
	warnings := make([]string, 60)
	for i := range warnings {
		warnings[i] = strings.Repeat("w", 400)
	}
	formatted := formatExportWarnings(warnings)
	if len(formatted) > 4100 || !strings.Contains(formatted, "more warnings") {
		t.Fatalf("formatted warnings length = %d, text = %q", len(formatted), formatted)
	}
}

func TestCanceledPandocProbeClearsCheckingAndPreservesStatus(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() {
		pandocCache.Lock()
		delete(pandocCache.entries, dir)
		pandocCache.Unlock()
	})
	gen1, _ := beginPandocProbe(dir, "", context.Background())
	want := pandoc.Status{Available: true, Path: "/opt/homebrew/bin/pandoc", Version: "3.13"}
	if !finishPandocProbe(dir, "", gen1, want) {
		t.Fatal("initial finishPandocProbe returned false")
	}
	gen2, _ := beginPandocProbe(dir, "", context.Background())
	if !ProfilePandocChecking(dir) {
		t.Fatal("expected checking=true during probe")
	}
	if !cancelPandocProbe(dir, "", gen2) {
		t.Fatal("cancelPandocProbe did not clear active generation")
	}
	if ProfilePandocChecking(dir) || ProfilePandocStatus(dir) != want {
		t.Fatalf("after cancel: checking=%v status=%+v", ProfilePandocChecking(dir), ProfilePandocStatus(dir))
	}
	gen3, _ := beginPandocProbe(dir, "", context.Background())
	if cancelPandocProbe(dir, "", gen2) || !ProfilePandocChecking(dir) {
		t.Fatalf("stale cancel cleared newer generation %d", gen3)
	}
}

func TestExportWarningsDetailsOpensPortraitAlert(t *testing.T) {
	w := &Window{}
	w.exportDone(export.Result{Warnings: []string{"Mermaid diagram 1 failed", "Remote image omitted"}}, filepath.Join(t.TempDir(), "Guide.docx"), nil)
	if len(w.toasts) != 1 || w.toasts[0].Action != "Details" || w.toasts[0].OnAction == nil || !strings.Contains(w.toasts[0].Title, "2 warnings") {
		t.Fatalf("warning toast = %+v", w.toasts)
	}
	w.toasts[0].OnAction()
	if w.dialog == nil || w.dialog.kind != "alert" || w.dialog.title != "Export warnings" || !strings.Contains(w.dialog.body, "Mermaid diagram 1 failed") || !strings.Contains(w.dialog.body, "Remote image omitted") {
		t.Fatalf("warning details dialog = %+v", w.dialog)
	}
}

func TestExportSettingsPandocPathStatusAndRefresh(t *testing.T) {
	var shell Shell
	model := Model{Preferences: map[string]string{}, PandocStatus: "Pandoc 3.13 (/opt/homebrew/bin/pandoc)"}
	refreshed := 0
	actions := Actions{
		SetPreference: func(key, value string) { model.Preferences[key] = value },
		ResetPreferences: func(keys ...string) {
			for _, key := range keys {
				delete(model.Preferences, key)
			}
		},
		RefreshPandoc: func() { refreshed++ },
	}
	page := settingsPageByID(shell.nativeSettingsPages(model, actions, Slots{}), "export")
	query := SettingsQuery{state: &settingsSearchState{}}
	app := ui.NewTester(func(c *ui.Context) { page.Draw(c, query) }, 640, 560)
	app.Frame()
	for _, text := range []string{"Pandoc", "Pandoc path", "Pandoc 3.13 (/opt/homebrew/bin/pandoc)", "Test & Refresh"} {
		if !app.HasText(text) {
			t.Errorf("Export settings missing %q: %v", text, app.Texts())
		}
	}
	if err := app.Click("Test & Refresh"); err != nil {
		t.Fatal(err)
	}
	if refreshed != 1 {
		t.Fatalf("RefreshPandoc calls = %d", refreshed)
	}
	field, ok := app.AccessByRole(ui.RoleTextField, "Pandoc path")
	if !ok {
		t.Fatal("Pandoc path text field is missing")
	}
	app.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
	app.Type("/custom/pandoc")
	app.Frame()
	if model.Preferences[PandocPathPreference] != "/custom/pandoc" || !page.Dirty() {
		t.Fatalf("Pandoc preferences = %v", model.Preferences)
	}
	query = SettingsQuery{Text: "pandoc", state: &settingsSearchState{}}
	app.Frame()
	if !app.HasText("Pandoc path") || app.HasText("Paper size") {
		t.Fatalf("Pandoc search results = %v", app.Texts())
	}
}
