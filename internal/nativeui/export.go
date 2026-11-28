package nativeui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/export"
	markdownexport "github.com/Bayern99/mark-go-go/internal/export/markdown"
	"github.com/Bayern99/mark-go-go/internal/export/pandoc"
	"github.com/Bayern99/mark-go-go/internal/export/pdf"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/macos"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo"
)

const (
	ReaderNotesPreference = "mgo.export.readerNotes"
	KeepDarkPDFPreference = "mgo.export.keepDarkPDF"
	PaperSizePreference   = "mgo.export.paperSize"
	PandocPathPreference  = "mgo.export.pandocPath"
)

var AfterReaderNotesChange func(bool)
var AfterExportPreferenceChange func()
var PrintPDF = func(ctx context.Context, html []byte, opts pdf.Options) ([]byte, error) {
	return pdf.Print(ctx, html, opts)
}
var ProbePandoc = pandoc.Probe
var RenderPandoc = pandoc.Render
var saveExportDialog = func(opts mygo.SaveDialogOptions) (string, error) { return mygo.Dialog.Save(opts) }
var writeExportFile = fsutil.WriteFileAtomic
var supportsPDFBackground = macos.SupportsPDFBackground
var pandocDialogShown func()

type pandocCacheEntry struct {
	configured string
	status     pandoc.Status
	checking   bool
	generation uint64
	cancel     context.CancelFunc
}

var pandocCache = struct {
	sync.RWMutex
	entries map[string]pandocCacheEntry
}{entries: map[string]pandocCacheEntry{}}

func ProfilePandocStatus(dir string) pandoc.Status {
	pandocCache.RLock()
	defer pandocCache.RUnlock()
	return pandocCache.entries[dir].status
}

func ProfilePandocChecking(dir string) bool {
	pandocCache.RLock()
	defer pandocCache.RUnlock()
	return pandocCache.entries[dir].checking
}

func (w *Window) RefreshPandoc() {
	if w == nil {
		return
	}
	dir := w.store().dir
	configured := w.Preference(PandocPathPreference)
	ctx := w.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	generation, probeCtx := beginPandocProbe(dir, configured, ctx)
	go func() {
		status := ProbePandoc(probeCtx, configured)
		if probeCtx.Err() != nil {
			if cancelPandocProbe(dir, configured, generation) {
				w.pandocStatusChanged()
			}
			return
		}
		if finishPandocProbe(dir, configured, generation, status) {
			w.pandocStatusChanged()
		}
	}()
}

func beginPandocProbe(dir, configured string, parent context.Context) (uint64, context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	probeCtx, cancel := context.WithCancel(parent)
	pandocCache.Lock()
	defer pandocCache.Unlock()
	entry := pandocCache.entries[dir]
	if entry.cancel != nil {
		entry.cancel()
	}
	entry.configured = configured
	entry.checking = true
	entry.generation++
	entry.cancel = cancel
	pandocCache.entries[dir] = entry
	return entry.generation, probeCtx
}

func cancelPandocProbe(dir, configured string, generation uint64) bool {
	pandocCache.Lock()
	defer pandocCache.Unlock()
	entry := pandocCache.entries[dir]
	if entry.generation != generation || entry.configured != configured || !entry.checking {
		return false
	}
	entry.checking = false
	entry.cancel = nil
	pandocCache.entries[dir] = entry
	return true
}

func finishPandocProbe(dir, configured string, generation uint64, status pandoc.Status) bool {
	pandocCache.Lock()
	defer pandocCache.Unlock()
	entry := pandocCache.entries[dir]
	if entry.generation != generation || entry.configured != configured {
		return false
	}
	entry.status = status
	entry.checking = false
	entry.cancel = nil
	pandocCache.entries[dir] = entry
	return true
}

func (w *Window) pandocStatusChanged() {
	update := func() {
		if w.preferences != nil {
			for window := range w.preferences.windows {
				if window.win != nil {
					window.win.Invalidate()
				}
			}
		} else if w.win != nil {
			w.win.Invalidate()
		}
		if AfterExportPreferenceChange != nil {
			AfterExportPreferenceChange()
		}
	}
	if w.Native() != nil {
		w.Native().Update(update)
		return
	}
	update()
}

func PandocStatusText(status pandoc.Status, checking bool) string {
	if checking {
		return "Checking for Pandoc…"
	}
	if status.Available {
		return "Pandoc " + status.Version + " (" + status.Path + ")"
	}
	if status.Error != "" {
		return status.Error
	}
	return "Not found"
}

func (w *Window) IncludeReaderNotes() bool {
	return w != nil && w.Preference(ReaderNotesPreference) == "on"
}

func ProfileReaderNotesEnabled(dir string) bool {
	return preferencesFor(dir).values[ReaderNotesPreference] == "on"
}

func (w *Window) KeepDarkPDFEnabled() bool {
	return w != nil && w.Preference(KeepDarkPDFPreference) == "on"
}

func ProfileKeepDarkPDFEnabled(dir string) bool {
	return preferencesFor(dir).values[KeepDarkPDFPreference] == "on"
}

func PaperSize(value string) pdf.Paper {
	if value == string(pdf.PaperLetter) {
		return pdf.PaperLetter
	}
	return pdf.PaperA4
}

func (w *Window) ExportOptions(title string) export.Options {
	root := w.Workspace().Root
	space := portrait.SpaceFor(root)
	if named, ok := portrait.SpaceNamed(w.Preference(spacePreference + root)); ok {
		space = named
	}
	dark := portrait.AppearanceNamed(w.Preference(appearancePreference)).IsDark(w.shell.dark)
	baseDir := ""
	if path := w.Active().Path; path != "" {
		baseDir = filepath.Dir(path)
	}
	return export.Options{
		SpaceName: space.Name, Dark: dark, PrintLight: true,
		IncludeNotes: w.IncludeReaderNotes(), BaseDir: baseDir, Title: title,
	}
}

func (w *Window) ExportPDFOptions(title string) (export.Options, pdf.Options) {
	exportOpts := w.ExportOptions(title)
	pdfOpts := pdf.Options{Paper: PaperSize(w.Preference(PaperSizePreference))}
	keepDark := exportOpts.Dark && w.KeepDarkPDFEnabled() && supportsPDFBackground()
	exportOpts.PrintLight = !keepDark
	exportOpts.OfflineImages = true
	pdfOpts.Background = keepDark
	return exportOpts, pdfOpts
}

func (w *Window) ExportMarkdown(formatID string) {
	if w == nil {
		return
	}
	format, err := export.DefaultRegistry.Lookup(formatID)
	if err != nil || format.Scope != export.ScopeMarkdown {
		w.Notify(portrait.ToastSpec{ID: "export", Tone: portrait.Tones.Danger, Title: "Export format is unavailable"})
		return
	}
	if !format.Available {
		w.Notify(portrait.ToastSpec{ID: "export", Tone: portrait.Tones.Warning, Title: format.UnavailableReason})
		return
	}
	doc := w.Active()
	source := w.Source()
	if doc.ID == "" || source == nil {
		w.Notify(portrait.ToastSpec{ID: "export", Tone: portrait.Tones.Warning, Title: "Open a Markdown document to export"})
		return
	}
	options := w.ExportOptions(doc.Name)
	pdfOpts := pdf.Options{}
	if formatID == "markdown-pdf" {
		options, pdfOpts = w.ExportPDFOptions(doc.Name)
	}
	text := source.Source()
	defaultPath := exportPath(doc.Path, doc.Name, format.Ext)
	ctx := w.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var (
		pandocDir        string
		pandocConfigured string
		pandocGen        uint64
	)
	if format.Engine == export.EnginePandoc {
		pandocDir = w.store().dir
		pandocConfigured = w.Preference(PandocPathPreference)
		pandocGen, _ = beginPandocProbe(pandocDir, pandocConfigured, nil)
	}
	go func() {
		var pandocStatus pandoc.Status
		if format.Engine == export.EnginePandoc {
			pandocStatus = ProbePandoc(ctx, pandocConfigured)
			if ctx.Err() != nil {
				if cancelPandocProbe(pandocDir, pandocConfigured, pandocGen) {
					w.pandocStatusChanged()
				}
				return
			}
			if finishPandocProbe(pandocDir, pandocConfigured, pandocGen, pandocStatus) {
				w.pandocStatusChanged()
			}
			if !pandocStatus.Available {
				w.showPandocDialogAsync(pandocStatus)
				return
			}
		}
		target, err := w.ChooseExportPath(defaultPath, format.Ext)
		if err != nil || target == "" {
			w.exportDone(export.Result{}, "", err)
			return
		}
		if err := ctx.Err(); err != nil {
			return
		}
		if formatID == "markdown-latex" {
			assetDir, err := chooseUniqueAssetDir(filepath.Dir(target), filepath.Base(target))
			if err != nil {
				w.exportDone(export.Result{}, "", err)
				return
			}
			options.AssetDir = assetDir
		}
		var result export.Result
		if format.Engine == export.EnginePandoc {
			result, err = RenderPandoc(ctx, text, formatID, options, pandocStatus)
		} else {
			result, err = markdownexport.Render(text, options)
		}
		if err != nil {
			w.exportDone(result, "", err)
			return
		}
		if formatID == "markdown-pdf" {
			data, err := PrintPDF(ctx, result.Data, pdfOpts)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					w.exportDone(result, "", err)
				}
				return
			}
			result.Data = data
		}
		if ctx.Err() != nil {
			return
		}
		w.WriteExport(target, result)
	}()
}

func exportPath(path, name, ext string) string {
	if path != "" {
		return strings.TrimSuffix(path, filepath.Ext(path)) + ext
	}
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ext
	dir, _ := mygo.App.Path(mygo.PathDocuments)
	return filepath.Join(dir, name)
}

func (w *Window) showPandocDialogAsync(status pandoc.Status) {
	show := func() {
		body := "Pandoc 2.15.0 or later is required for this export format."
		if status.Error != "" {
			body += "\n\n" + status.Error
		}
		w.showDialog(&windowDialog{kind: "pandoc", open: true, title: "Pandoc is required", body: body})
		if w.win != nil {
			w.win.Invalidate()
		}
		if pandocDialogShown != nil {
			pandocDialogShown()
		}
	}
	if w.Native() != nil {
		w.Native().Update(show)
		return
	}
	show()
}

func (w *Window) NotifyAsync(spec portrait.ToastSpec) {
	if w == nil {
		return
	}
	if w.Native() != nil {
		w.Native().Update(func() { w.Notify(spec) })
		return
	}
	w.Notify(spec)
}

func (w *Window) ChooseExportPath(defaultPath, ext string) (string, error) {
	filterNames := map[string]string{
		".html": "HTML Document", ".pdf": "PDF Document", ".docx": "Word Document",
		".pptx": "PowerPoint Presentation", ".epub": "EPUB", ".rtf": "Rich Text Format", ".tex": "LaTeX Document",
	}
	filterName := filterNames[ext]
	if filterName == "" {
		filterName = "Export Document"
	}
	target, err := saveExportDialog(mygo.SaveDialogOptions{
		Parent: w.Native(), DefaultPath: defaultPath,
		Filters: []mygo.FileFilter{{Name: filterName, Extensions: []string{strings.TrimPrefix(ext, ".")}}},
	})
	if err != nil || target == "" {
		return "", err
	}
	if filepath.Ext(target) == "" {
		target += ext
	}
	return target, nil
}

func chooseUniqueAssetDir(dir, baseName string) (string, error) {
	stem := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	var b strings.Builder
	for _, r := range strings.ToLower(stem) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	clean := strings.Trim(b.String(), "-_")
	if clean == "" {
		clean = "export"
	}
	base := clean + "-assets"
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	for i := 1; i < 10000; i++ {
		candidate := base
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		_, err := root.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate an unused export assets directory")
}

func writeAttachmentsExclusive(dir string, attachments []export.Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var createdFiles []string
	var createdDirs []string
	createdDirSet := map[string]bool{}
	cleanup := func() {
		for i := len(createdFiles) - 1; i >= 0; i-- {
			_ = root.Remove(createdFiles[i])
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			_ = root.Remove(createdDirs[i])
		}
	}
	mkdirs := func(relDir string) error {
		if relDir == "." || relDir == "" {
			return nil
		}
		parts := strings.Split(relDir, "/")
		current := ""
		for _, part := range parts {
			if current == "" {
				current = part
			} else {
				current += "/" + part
			}
			if createdDirSet[current] {
				continue
			}
			if err := root.Mkdir(current, 0o755); err != nil {
				return err
			}
			createdDirSet[current] = true
			createdDirs = append(createdDirs, current)
		}
		return nil
	}
	for _, attachment := range attachments {
		if !fs.ValidPath(attachment.Name) || attachment.Name == "." {
			cleanup()
			return fmt.Errorf("invalid export attachment path %q", attachment.Name)
		}
		if dirPart := filepath.ToSlash(filepath.Dir(filepath.FromSlash(attachment.Name))); dirPart != "." {
			if err := mkdirs(dirPart); err != nil {
				cleanup()
				return err
			}
		}
		file, err := root.OpenFile(attachment.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			cleanup()
			return err
		}
		_, writeErr := file.Write(attachment.Data)
		closeErr := file.Close()
		createdFiles = append(createdFiles, attachment.Name)
		if writeErr != nil {
			cleanup()
			return writeErr
		}
		if closeErr != nil {
			cleanup()
			return closeErr
		}
	}
	return nil
}

func (w *Window) WriteExport(target string, result export.Result) {
	if w == nil {
		return
	}
	err := writeAttachmentsExclusive(filepath.Dir(target), result.Attachments)
	if err == nil {
		err = writeExportFile(target, result.Data)
	}
	w.exportDone(result, target, err)
}

func formatExportWarnings(warnings []string) string {
	const maxItems = 40
	const maxItemRunes = 240
	const maxTotalBytes = 4000
	var b strings.Builder
	shown := 0
	for _, warning := range warnings {
		if shown >= maxItems {
			break
		}
		text := strings.TrimSpace(warning)
		if runes := []rune(text); len(runes) > maxItemRunes {
			text = string(runes[:maxItemRunes]) + "…"
		}
		if b.Len() > 0 && b.Len()+2+len(text) > maxTotalBytes {
			break
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
		shown++
	}
	if remaining := len(warnings) - shown; remaining > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "… and %d more warnings", remaining)
	}
	return b.String()
}

func (w *Window) SaveHTML(defaultPath string, result export.Result) {
	if w == nil {
		return
	}
	go w.saveExport(defaultPath, result, ".html")
}

func (w *Window) SavePDF(defaultPath string, result export.Result) {
	if w == nil {
		return
	}
	go w.saveExport(defaultPath, result, ".pdf")
}

func (w *Window) saveExport(defaultPath string, result export.Result, ext string) {
	target, err := w.ChooseExportPath(defaultPath, ext)
	if err != nil || target == "" {
		w.exportDone(result, "", err)
		return
	}
	w.WriteExport(target, result)
}

func (w *Window) exportDone(result export.Result, target string, err error) {
	w.notifyLater(func() (portrait.ToastSpec, bool) {
		switch {
		case err == nil && target != "":
			toast := portrait.ToastSpec{ID: "export", Tone: portrait.Tones.Success, Title: "Exported " + filepath.Base(target)}
			if len(result.Warnings) > 0 {
				toast.Tone = portrait.Tones.Warning
				toast.Title = fmt.Sprintf("Exported %s with %d warnings", filepath.Base(target), len(result.Warnings))
				toast.Action = "Details"
				toast.OnAction = func() { w.ShowAlert("Export warnings", formatExportWarnings(result.Warnings)) }
			}
			return toast, true
		case errors.Is(err, mygo.ErrCanceled), err == nil:
			return portrait.ToastSpec{}, false
		default:
			return portrait.ToastSpec{ID: "export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()}, true
		}
	})
}
