package nativeui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type openDocument struct {
	returnToGenUI        bool
	crumbPath, crumbRoot string
	crumbs               []workspace.Crumb
	source               *nativeeditor.Document
	readingPreferenceKey string
	scrollRestore        uint64
	published            uint64
	reloadVersion        int
	outlineVersion       uint64
	sections             []workspace.Section
}

type Window struct {
	host                          *workspace.Service
	win                           *mygo.Window
	ctx                           context.Context
	cancel                        context.CancelFunc
	snapshot                      workspace.WorkspaceSnapshot
	documents                     map[string]*openDocument
	focusSource                   bool
	failure                       string
	toasts                        []portrait.ToastSpec
	notices                       []toastNotice
	noticeMu                      sync.Mutex
	shell                         Shell
	slots                         Slots
	navigation                    workspace.NavSnapshot
	search                        workspace.NavSearchResult
	recents                       []workspace.RecentWorkspace
	changed                       []func()
	preferences                   *preferenceStore
	pendingReveal                 *workspace.DocReveal
	menuKnown, menuUndo, menuRedo bool
	dialog                        *windowDialog
	alerts                        []*windowDialog
	focusReturn                   ui.Handle
	closing                       map[string]bool
	sceneFolders                  []workspace.NavEntry
}

var (
	windowRegistry = map[*mygo.Window]*Window{}
	windowOrder    []*mygo.Window
)

// AfterWorkspaceChange runs once a window has refreshed for a workspace
// change. The app installs its Open Recent refresh here: the submenu is the
// app's own, and MyGo only refreshes the role-based one.
var AfterWorkspaceChange func()

func WindowFor(native *mygo.Window) *Window {
	return windowRegistry[native]
}

// FrontWindow returns a window Portrait has registered, preferring the one the
// app has focused, so a menu command that arrives before its window is
// registered still runs in a real Portrait window.
func FrontWindow() *Window {
	if w := WindowFor(mygo.FocusedWindow()); w != nil {
		return w
	}
	for i := len(windowOrder) - 1; i >= 0; i-- {
		if w := WindowFor(windowOrder[i]); w != nil {
			return w
		}
	}
	return nil
}

// ResetAllMenuStates invalidates cached Undo/Redo menu enablement across all
// open Portrait windows after the application menu is rebuilt.
func ResetAllMenuStates() {
	for _, w := range windowRegistry {
		if w != nil {
			w.menuKnown = false
			w.updateMenu()
		}
	}
}

func (w *Window) OpenSettings(tab string) {
	if w == nil {
		return
	}
	w.shell.OpenSettings(tab)
	if w.win != nil {
		w.win.Invalidate()
	}
}

func (w *Window) OpenCheckpoints() {
	if w == nil || !w.PluginEnabled("git") {
		return
	}
	w.shell.checkpoints = true
	if w.win != nil {
		w.win.Invalidate()
	}
}

func (w *Window) OpenCommandBar() {
	if w == nil {
		return
	}
	trigger := w.shell.command.trigger
	w.shell.command = commandBar{open: true, focus: true, capture: true, trigger: trigger}
	if w.win != nil {
		w.win.Invalidate()
	}
}

func (w *Window) ToggleSidebar() {
	if w == nil {
		return
	}
	width, _ := w.win.ContentSize()
	w.shell.toggleSidebar(float32(width))
	w.shell.sidebarInstant = true
	w.shell.leftKeys.Flip()
	w.shell.rightKeys.Flip()
	if float32(width) >= sidebarDocksAt {
		if w.shell.hideSidebar {
			w.setPreference(sidebarHiddenPreference, "on")
		} else {
			w.setPreference(sidebarHiddenPreference, "")
		}
	}
	if w.win != nil {
		w.win.Invalidate()
	}
}

func (w *Window) ToggleResearch() {
	if w == nil || !w.PluginEnabled("feynman") {
		return
	}
	width, _ := w.win.ContentSize()
	w.shell.toggleResearch(float32(width))
	w.shell.leftKeys.Flip()
	w.shell.rightKeys.Flip()
	if float32(width) >= researchDocksAt {
		if w.shell.hideResearch {
			w.setPreference(researchHiddenPreference, "on")
		} else {
			w.setPreference(researchHiddenPreference, "")
		}
	}
	if w.win != nil {
		w.win.Invalidate()
	}
}

func (w *Window) SetSpace(name string) {
	if w == nil {
		return
	}
	w.setPreference(spacePreference+w.snapshot.Root, name)
}

func (w *Window) SetAppearance(a portrait.Appearance) {
	if w == nil {
		return
	}
	w.setPreference(appearancePreference, string(a))
}

const (
	spacePreference      = "mgo.space:"
	appearancePreference = "mgo.appearance"
)

func NewWindow(host *workspace.Service, opts mygo.WindowOptions, extensions ...func(*Window) Panels) *mygo.Window {
	w := &Window{host: host, documents: make(map[string]*openDocument)}
	opts.Content = ui.View(w.View)
	if runtime.GOOS == "darwin" {
		opts.TitleBarStyle = mygo.TitleBarHidden
		opts.TrafficLightPosition = &mygo.Point{X: 16, Y: 21}
	}
	mygo.RunOnMain(func() {
		if err := portrait.RegisterFonts(); err != nil {
			w.failure = err.Error()
		}
		w.preferences = preferencesFor(host.UserDataDir())
		if w.preferences.loadError != nil {
			w.failure = w.preferences.loadError.Error()
		}
		w.win = mygo.NewWindow(opts)
		w.ctx, w.cancel = context.WithCancel(desktop.WithWindow(context.Background(), w.win))
		workspace.WorkspaceChanged.Listen(w.win, func(_ workspace.WorkspaceSnapshot) {
			w.refresh()
			if AfterWorkspaceChange != nil {
				AfterWorkspaceChange()
			}
		})
		workspace.DocumentReloaded.Listen(w.win, w.reload)
		workspace.CommandRequested.Listen(w.win, w.command)
		workspace.QuitFailed.Listen(w.win, func(f workspace.QuitFailure) { w.ShowAlert(f.Title, f.Body) })
		w.win.OnFocus(func() {
			w.menuKnown = false
			w.updateMenu()
			if AfterWorkspaceChange != nil {
				AfterWorkspaceChange()
			}
		})
		w.win.OnShow(w.refresh)
		workspace.NavigationChanged.Listen(w.win, func(n workspace.NavSnapshot) {
			if n.Root == w.snapshot.Root {
				w.navigation = n
			}
		})
		workspace.SearchReady.Listen(w.win, func(r workspace.NavSearchResult) {
			if r.Root == w.snapshot.Root && r.Query == strings.TrimSpace(w.shell.searchQuery(r.Root)) {
				w.search = r
			}
		})
		workspace.DocumentRevealed.Listen(w.win, func(r workspace.DocReveal) { w.pendingReveal = &r; w.applyReveal() })
		for _, attach := range extensions {
			w.install(attach(w))
		}
		w.preferences.windows[w] = true
		windowRegistry[w.win] = w
		windowOrder = append(windowOrder, w.win)
		w.win.OnClosed(func() {
			if w.cancel != nil {
				w.cancel()
			}
			delete(w.preferences.windows, w)
			delete(windowRegistry, w.win)
			windowOrder = slices.DeleteFunc(windowOrder, func(native *mygo.Window) bool { return native == w.win })
		})
	})
	if w.win == nil {
		// RunOnMain drops its closure once the loop has shut down. Ask mygo
		// for the same destroyed placeholder it returns in that case, rather
		// than handing nil to callers that expect a window.
		w.win = mygo.NewWindow(opts)
	}
	w.RefreshPandoc()
	return w.win
}

func (w *Window) refresh() {
	next, err := w.host.Snapshot(w.ctx)
	if err != nil {
		return
	}
	if w.snapshot.ActiveID != next.ActiveID {
		w.focusSource = true
	}
	rootChanged := w.snapshot.ID == "" || next.Root != w.snapshot.Root
	w.snapshot = next
	if rootChanged {
		w.navigation, _ = w.host.NavSnapshot(w.ctx)
		w.search, _ = w.host.SetFilter(w.ctx, w.shell.queries[next.Root])
		w.recents, _ = w.host.RecentWorkspaces(w.ctx)
	}
	for i := range w.snapshot.Documents {
		doc := &w.snapshot.Documents[i]
		if doc.ViewMode == "" && doc.Path != "" {
			doc.ViewMode = w.Preference("mgo.viewMode:" + doc.Path)
		}
		if doc.ViewMode == "" {
			doc.ViewMode = w.Preference(defaultViewPreference)
		}
		if doc.ViewMode != "markdown" && doc.ViewMode != "genui" {
			doc.ViewMode = "preview"
		}
	}
	for id := range w.documents {
		if !slices.ContainsFunc(next.Documents, func(info workspace.DocumentInfo) bool { return info.ID == id }) {
			delete(w.documents, id)
			delete(w.shell.scrolls, "outline:"+id)
			delete(w.shell.openCheckpoints, id)
			delete(w.shell.selectedOutline, id)
		}
	}
	if id := next.ActiveID; id != "" && w.documents[id] == nil {
		snap, err := w.host.Load(w.ctx, id)
		if err != nil {
			w.failure = err.Error()
			return
		}
		d := nativeeditor.New(snap.Text)
		d.SetScrollY(snap.ScrollY)
		w.documents[id] = &openDocument{source: d, published: d.Version(), reloadVersion: snap.Version, scrollRestore: snap.State.ScrollRestore}
	}
	for _, info := range next.Documents {
		if d := w.documents[info.ID]; d != nil {
			directory := ""
			if info.Path != "" {
				directory = filepath.Dir(info.Path)
			}
			d.source.SetImageDirectory(directory)
			if info.ScrollRestore != 0 && d.scrollRestore != info.ScrollRestore {
				d.scrollRestore = info.ScrollRestore
				d.source.SetScrollY(info.RestoredScrollY)
			}
		}
	}
	if doc := w.Active(); doc.ID != "" && w.documents[doc.ID] != nil {
		open := w.documents[doc.ID]
		key := doc.Path
		if key == "" {
			key = doc.ID
		}
		if open.readingPreferenceKey != key {
			open.readingPreferenceKey = key
			open.source.SetReadingPreferences(w.Preference("mgo.folds:"+key), w.Preference("mgo.table-widths:"+key), func(name, value string) { w.setPreference("mgo."+name+":"+key, value) })
		}
	}
	if doc := w.Active(); doc.ID != "" && w.documents[doc.ID] != nil {
		if mode := w.viewMode(doc); mode != "genui" {
			w.documents[doc.ID].source.SetMode(nativeeditor.Mode(mode))
		}
	}
	w.applyReveal()
	for _, notify := range w.changed {
		notify()
	}
	for _, info := range next.Documents {
		if d := w.documents[info.ID]; d != nil && !info.Edited && !info.Conflict && d.published == d.source.Version() {
			d.source.MarkSaved()
		}
	}
	w.updateMenu()
}

func (w *Window) reload(s workspace.Snapshot) {
	if d := w.documents[s.ID]; d != nil && s.Version > d.reloadVersion {
		d.source.Reload(s.Text)
		d.reloadVersion, d.published = s.Version, d.source.Version()
		w.updateMenu()
	}
}

func (w *Window) publish(id string, attempt int) error {
	defer w.updateMenu()
	d := w.documents[id]
	if d == nil {
		return nil
	}
	if w.closing[id] {
		return nil
	}
	// Keep the session's scroll position current without writing it to disk
	// here: every frame would then write the session. The tab, view and quit
	// persistence paths save whichever value is latest.
	w.host.SetDocumentScrollY(id, d.source.ScrollY())
	if d.published == d.source.Version() {
		return nil
	}
	var err error
	if attempt == 0 {
		err = w.host.Update(w.ctx, id, d.source.Source())
	} else {
		err = w.host.FlushQuit(w.ctx, id, d.source.Source(), attempt)
	}
	if err == nil {
		d.published = d.source.Version()
	}
	return err
}

func (w *Window) command(cmd workspace.NativeCommand) {
	if cmd.Command == "flushQuit" {
		failure := ""
		for id := range w.documents {
			if err := w.publish(id, cmd.Attempt); err != nil {
				failure = err.Error()
				break
			}
		}
		go w.host.PrepareQuit(w.ctx, cmd.Attempt, failure)
		return
	}
	d := w.documents[cmd.ID]
	switch cmd.Command {
	case "undo", "redo":
		w.editCommand(cmd.Command, d)
	}
	if err := w.publish(cmd.ID, 0); err != nil {
		w.failure = err.Error()
		return
	}
	switch cmd.Command {
	case "undo", "redo":
		w.updateMenu()
	case "save", "saveAs", "rename", "moveTo", "revert", "duplicate", "close", "closeWorkspace":
		if isCloseDialog(w.dialog) {
			return
		}
		switch cmd.Command {
		case "save":
			w.requestSave(cmd.ID)
		case "saveAs":
			go w.actionToast(func() error { return w.host.SaveAs(w.ctx, cmd.ID) }, portrait.ToastSpec{ID: "save:" + cmd.ID, Tone: portrait.Tones.Success, Title: "Saved"})
		case "rename":
			w.openRename(cmd.ID)
		case "moveTo":
			w.openMove(cmd.ID)
		case "revert":
			w.openRevert(cmd.ID)
		case "duplicate":
			go w.action(func() error { _, err := w.host.Duplicate(w.ctx, cmd.ID); return err })
		case "close":
			w.requestClose("closeDocument", "", []string{cmd.ID})
		case "closeWorkspace":
			w.requestClose("closeWorkspace", "", nil)
		}
	}
}

func (w *Window) action(run func() error) {
	w.actionToast(run, portrait.ToastSpec{})
}

func (w *Window) Notify(spec portrait.ToastSpec) {
	if w == nil {
		return
	}
	w.toasts = append(w.toasts, spec)
}

type toastNotice func() (portrait.ToastSpec, bool)

func (w *Window) notifyLater(notice toastNotice) {
	if w == nil {
		return
	}
	if native := w.Native(); native != nil {
		native.Update(func() { w.showNotice(notice) })
		return
	}
	w.noticeMu.Lock()
	w.notices = append(w.notices, notice)
	w.noticeMu.Unlock()
}

func (w *Window) showNotice(notice toastNotice) {
	if spec, ok := notice(); ok {
		w.Notify(spec)
	}
}

func (w *Window) showNotices() {
	w.noticeMu.Lock()
	notices := w.notices
	w.notices = nil
	w.noticeMu.Unlock()
	for _, notice := range notices {
		w.showNotice(notice)
	}
}

func (w *Window) actionToast(run func() error, success portrait.ToastSpec) {
	err := run()
	w.win.Update(func() {
		w.failure = ""
		if err != nil && !errors.Is(err, mygo.ErrCanceled) {
			w.failure = err.Error()
		}
		if err == nil && success.Title != "" {
			w.toasts = append(w.toasts, success)
		}
	})
}

func (w *Window) applyReveal() {
	r := w.pendingReveal
	if r != nil && r.ID == w.snapshot.ActiveID && w.Source() != nil {
		w.pendingReveal = nil
		w.Reveal(r.Offset)
	}
}

func (w *Window) openDropped(paths []string) error {
	var failures []error
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil {
			switch {
			case info.IsDir():
				err = w.host.OpenWorkspace(w.ctx, path)
			case fsutil.MarkdownName(path):
				_, err = w.host.Open(w.ctx, path)
			default:
				err = fmt.Errorf("%s is not a Markdown file", filepath.Base(path))
			}
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (w *Window) openPath(path string, offset int) {
	go w.action(func() error {
		doc, err := w.host.Open(w.ctx, path)
		if err != nil {
			return err
		}
		if offset >= 0 {
			return w.host.RevealDocument(doc.ID, offset)
		}
		return nil
	})
}

func (w *Window) View(c *ui.Context) {
	if w.ctx == nil {
		return
	}
	c.SetMenuPresentation(ui.PopoverMenus)
	if w.snapshot.ID == "" {
		w.refresh()
	}
	w.syncEditorMode()
	d := w.documents[w.snapshot.ActiveID]
	if d != nil {
		d.source.SetLineNumbers(w.Preference(lineNumbersPreference) != "off")
	}
	model := Model{Workspace: w.snapshot, Navigation: w.navigation, Recents: w.recents, Search: w.search, Message: w.failure, Feynman: w.PluginEnabled("feynman"), Git: w.PluginEnabled("git"), GenUI: w.PluginEnabled("genui"), Spaces: map[string]string{}, Appearance: portrait.AppearanceNamed(w.Preference(appearancePreference)), Preferences: w.store().values, PreferenceFile: w.store().path(), PandocStatus: PandocStatusText(ProfilePandocStatus(w.store().dir), ProfilePandocChecking(w.store().dir))}
	if d != nil {
		model.DocumentScrollY = d.source.ScrollY()
	}
	for _, root := range w.snapshot.Workspaces {
		model.Spaces[root.Root] = w.Preference(spacePreference + root.Root)
	}
	model.Spaces[w.snapshot.Root] = w.Preference(spacePreference + w.snapshot.Root)
	if d != nil {
		doc := w.Active()
		if d.crumbPath != doc.Path || d.crumbRoot != doc.WorkspaceRoot {
			d.crumbPath, d.crumbRoot = doc.Path, doc.WorkspaceRoot
			d.crumbs = workspace.Breadcrumbs(doc.WorkspaceRoot, doc.Path)
		}
		model.ReturnToGenUI = d.returnToGenUI && model.GenUI
		model.Crumbs = d.crumbs
	}
	if d != nil && w.shell.wantsSections() {
		if d.sections == nil || d.outlineVersion != d.source.Version() {
			d.sections = workspace.ExtractSections(d.source.Source())
			d.outlineVersion = d.source.Version()
		}
		model.Sections = d.sections
		_, head := d.source.SelectionUTF16()
		model.DocumentOffset = head
		model.HasDocumentOffset = true
	}
	slots := w.slots
	slots.Document = func(c *ui.Context) {
		if d == nil {
			return
		}
		doc := w.Active()
		field := d.source.View(c.Key(doc.ID), doc.ReadOnly).Grow(1).MinHeight(0).FillWidth().Bind(&w.focusReturn)
		if w.focusSource {
			if !w.shell.focusedTab {
				field.Focus()
			}
			w.shell.focusedTab = false
			w.focusSource = false
		}
		if err := w.publish(doc.ID, 0); err != nil {
			w.failure = err.Error()
		}
	}
	actions := Actions{
		Drop: func(paths []string) { go w.action(func() error { return w.openDropped(paths) }) },
		RevealPath: func(path string) {
			if err := w.host.RevealPath(w.ctx, path); err != nil {
				w.failure = err.Error()
			}
			w.shell.outline = false
			w.shell.drillTo(w.snapshot.Root, path)
		},
		Activate:          func(id string) { go w.action(func() error { return w.host.Activate(w.ctx, id) }) },
		ActivateWorkspace: func(id string) { go w.action(func() error { return w.host.ActivateWorkspace(w.ctx, id) }) },
		CloseDocument:     func(id string) { w.command(workspace.NativeCommand{Command: "close", ID: id}) },
		CloseWorkspace:    func(id string) { w.requestClose("closeRoot", id, nil) },
		OpenPath:          w.openPath, Reveal: w.Reveal, SetMode: w.SetMode,
		OpenFolder:     func() { go w.action(func() error { _, err := w.host.PickFolder(w.ctx); return err }) },
		NewDocument:    func() { go w.action(func() error { _, err := w.host.New(w.ctx); return err }) },
		OpenFiles:      w.OpenFiles,
		Find:           func() { w.win.EditContent("find") },
		Undo:           func() { w.command(workspace.NativeCommand{Command: "undo", ID: w.snapshot.ActiveID}) },
		Save:           func(id string) { w.command(workspace.NativeCommand{Command: "save", ID: id}) },
		SaveAs:         func(id string) { w.command(workspace.NativeCommand{Command: "saveAs", ID: id}) },
		Rename:         func(id string) { w.command(workspace.NativeCommand{Command: "rename", ID: id}) },
		Move:           func(id string) { w.command(workspace.NativeCommand{Command: "moveTo", ID: id}) },
		Revert:         func(id string) { w.command(workspace.NativeCommand{Command: "revert", ID: id}) },
		ExportMarkdown: w.ExportMarkdown,
		RefreshPandoc:  w.RefreshPandoc,
		CommandFocus:   func() *ui.Handle { return &w.focusReturn },
		OpenRecent:     func(path string) { go w.action(func() error { return w.host.OpenWorkspace(w.ctx, path) }) },
		ToggleFolder: func(rel string) {
			if err := w.host.ToggleFolder(w.ctx, rel); err != nil {
				w.failure = err.Error()
			}
		},
		Filter:        func(query string) { w.search, _ = w.host.SetFilter(w.ctx, query) },
		SetSpace:      func(name string) { w.setPreference(spacePreference+w.snapshot.Root, name) },
		SetAppearance: func(a portrait.Appearance) { w.setPreference(appearancePreference, string(a)) },
		SetPreference: w.setPreference, ResetPreferences: w.ResetPreferences, ResetPreferencePrefixes: w.ResetPreferencePrefixes,
		RevealPreferences: func() {
			if err := w.host.RevealPath(w.ctx, w.store().path()); err != nil {
				w.failure = err.Error()
			}
		},
		SetFeynman: func(value bool) { w.setPlugin("feynman", value) }, SetGit: func(value bool) { w.setPlugin("git", value) }, SetGenUI: func(value bool) { w.setPlugin("genui", value) },
		SaveConflict: w.saveConflict, RevertConflict: w.openRevert,
	}
	if !w.shell.ready {
		w.shell.hideSidebar = w.Preference(sidebarHiddenPreference) == "on"
		w.shell.hideResearch = w.Preference(researchHiddenPreference) == "on"
		w.shell.outline = w.Preference(sidebarViewPreference) == "outline"
		if v, err := strconv.ParseFloat(w.Preference("mgo.researchWidth"), 32); err == nil && v >= float64(portrait.Shell.ResearchMin) && v <= float64(portrait.Shell.ResearchMax) {
			w.shell.rightWidth = float32(v)
		}
	}
	oldWidth := w.shell.rightWidth
	readingSize, _ := strconv.ParseFloat(w.Preference(readingSizePreference), 32)
	codeSize, _ := strconv.ParseFloat(w.Preference(codeSizePreference), 32)
	readingWidth, _ := strconv.ParseFloat(w.Preference(readingWidthPreference), 32)
	if readingSize == 0 {
		readingSize = 16
	}
	if codeSize == 0 {
		codeSize = 13
	}
	if readingWidth == 0 {
		readingWidth = float64(portrait.Editor.Column)
	}
	portrait.UseEditor(float32(readingSize), float32(codeSize), float32(readingWidth))
	w.showNotices()
	for _, spec := range w.toasts {
		portrait.Notify(c, spec)
	}
	w.toasts = nil
	w.shell.Draw(c, model, actions, slots)
	w.DrawDialog(c)
	w.updateMenu()
	if oldWidth != 0 && oldWidth != w.shell.rightWidth {
		w.setPreference("mgo.researchWidth", strconv.Itoa(int(w.shell.rightWidth)))
	}
}
