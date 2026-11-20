package nativeui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type openDocument struct {
	crumbPath, crumbRoot string
	crumbs               []workspace.Crumb
	source               *nativeeditor.Document
	published            uint64
	reloadVersion        int
	outlineVersion       uint64
	sections             []workspace.Section
}

type Window struct {
	host          *workspace.Service
	win           *mygo.Window
	ctx           context.Context
	snapshot      workspace.WorkspaceSnapshot
	documents     map[string]*openDocument
	focusSource   bool
	failure       string
	shell         Shell
	slots         Slots
	navigation    workspace.NavSnapshot
	search        workspace.NavSearchResult
	recents       []workspace.RecentWorkspace
	changed       []func()
	preferences   *preferenceStore
	pendingReveal *workspace.DocReveal
}

func NewWindow(host *workspace.Service, opts mygo.WindowOptions, extensions ...func(*Window) Panels) *mygo.Window {
	w := &Window{host: host, documents: make(map[string]*openDocument)}
	opts.Content = ui.View(w.View)
	mygo.RunOnMain(func() {
		if err := RegisterFonts(); err != nil {
			w.failure = err.Error()
		}
		w.preferences = preferencesFor(host.UserDataDir())
		if w.preferences.loadError != nil {
			w.failure = w.preferences.loadError.Error()
		}
		w.win = mygo.NewWindow(opts)
		w.ctx = desktop.WithWindow(context.Background(), w.win)
		workspace.WorkspaceChanged.Listen(w.win, func(_ workspace.WorkspaceSnapshot) { w.refresh() })
		workspace.DocumentReloaded.Listen(w.win, w.reload)
		workspace.CommandRequested.Listen(w.win, w.command)
		w.win.OnFocus(w.updateMenu)
		w.win.OnShow(w.refresh)
		w.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			paths := slices.Clone(e.Paths)
			go w.action(func() error { return w.openDropped(paths) })
		})
		workspace.NavigationChanged.Listen(w.win, func(n workspace.NavSnapshot) {
			if n.Root == w.snapshot.Root {
				w.navigation = n
			}
		})
		workspace.SearchReady.Listen(w.win, func(r workspace.NavSearchResult) {
			if r.Root == w.snapshot.Root && r.Query == strings.TrimSpace(w.shell.queries[r.Root]) {
				w.search = r
			}
		})
		workspace.DocumentRevealed.Listen(w.win, func(r workspace.DocReveal) { w.pendingReveal = &r; w.applyReveal() })
		for _, attach := range extensions {
			w.install(attach(w))
		}
		w.preferences.windows[w] = true
		w.win.OnClosed(func() { delete(w.preferences.windows, w) })
	})
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
		if doc.ViewMode == "richtext" || doc.ViewMode == "" {
			doc.ViewMode = "preview"
		}
	}
	for id := range w.documents {
		if !slices.ContainsFunc(next.Documents, func(info workspace.DocumentInfo) bool { return info.ID == id }) {
			delete(w.documents, id)
		}
	}
	if id := next.ActiveID; id != "" && w.documents[id] == nil {
		snap, err := w.host.Load(w.ctx, id)
		if err != nil {
			w.failure = err.Error()
			return
		}
		d := nativeeditor.New(snap.Text)
		w.documents[id] = &openDocument{source: d, published: d.Version(), reloadVersion: snap.Version}
	}
	if doc := w.Active(); doc.ID != "" && w.documents[doc.ID] != nil && doc.ViewMode != "genui" {
		w.documents[doc.ID].source.SetMode(nativeeditor.Mode(doc.ViewMode))
	}
	w.applyReveal()
	for _, notify := range w.changed {
		notify()
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

func (w *Window) updateMenu() {
	if w.win == nil || !w.win.IsFocused() {
		return
	}
	d := w.documents[w.snapshot.ActiveID]
	workspace.ApplyNativeEditMenu(d != nil && d.source.CanUndo(), d != nil && d.source.CanRedo())
}

func (w *Window) publish(id string, attempt int) error {
	d := w.documents[id]
	if d == nil || d.published == d.source.Version() {
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
		w.updateMenu()
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
	if cmd.ID != "" && d == nil {
		return
	}
	switch cmd.Command {
	case "undo":
		d.source.Undo()
	case "redo":
		d.source.Redo()
	}
	if err := w.publish(cmd.ID, 0); err != nil {
		w.failure = err.Error()
		return
	}
	switch cmd.Command {
	case "undo", "redo":
		w.updateMenu()
	case "save":
		go w.action(func() error { return w.host.Save(w.ctx, cmd.ID) })
	case "saveAs":
		go w.action(func() error { return w.host.SaveAs(w.ctx, cmd.ID) })
	case "rename":
		go w.action(func() error { return w.host.Rename(w.ctx, cmd.ID) })
	case "moveTo":
		go w.action(func() error { return w.host.MoveTo(w.ctx, cmd.ID) })
	case "revert":
		go w.action(func() error { return w.host.Revert(w.ctx, cmd.ID) })
	case "duplicate":
		go w.action(func() error { _, err := w.host.Duplicate(w.ctx, cmd.ID); return err })
	case "close":
		go w.action(func() error { _, err := w.host.CloseDocument(w.ctx, cmd.ID); return err })
	case "closeWorkspace":
		for id := range w.documents {
			if err := w.publish(id, 0); err != nil {
				w.failure = err.Error()
				return
			}
		}
		go w.action(func() error { _, err := w.host.CloseWorkspace(w.ctx); return err })
	}
}

func (w *Window) action(run func() error) {
	err := run()
	w.win.Update(func() {
		w.failure = ""
		if err != nil && !errors.Is(err, mygo.ErrCanceled) {
			w.failure = err.Error()
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
			case strings.EqualFold(filepath.Ext(path), ".md") || strings.EqualFold(filepath.Ext(path), ".markdown"):
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
			w.win.Update(func() { w.pendingReveal = &workspace.DocReveal{ID: doc.ID, Offset: offset}; w.applyReveal() })
		}
		return nil
	})
}

func (w *Window) View(c *ui.Context) {
	if w.ctx == nil {
		return
	}
	if w.snapshot.ID == "" {
		w.refresh()
	}
	d := w.documents[w.snapshot.ActiveID]
	model := Model{Workspace: w.snapshot, Navigation: w.navigation, Recents: w.recents, Search: w.search, Message: w.failure, Feynman: w.PluginEnabled("feynman"), Git: w.PluginEnabled("git")}
	if d != nil {
		doc := w.Active()
		if d.crumbPath != doc.Path || d.crumbRoot != doc.WorkspaceRoot {
			d.crumbPath, d.crumbRoot = doc.Path, doc.WorkspaceRoot
			d.crumbs = workspace.Breadcrumbs(doc.WorkspaceRoot, doc.Path)
		}
		model.Crumbs = d.crumbs
		model.Version = d.reloadVersion
	}
	if d != nil && w.shell.outline {
		if d.sections == nil || d.outlineVersion != d.source.Version() {
			d.sections = workspace.ExtractSections(d.source.Source())
			d.outlineVersion = d.source.Version()
		}
		model.Sections = d.sections
	}
	slots := w.slots
	slots.Document = func(c *ui.Context) {
		if d == nil {
			return
		}
		doc := w.Active()
		field := d.source.View(c.Key(doc.ID), doc.ReadOnly).Grow(1).MinHeight(0).FillWidth()
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
		RevealPath: func(path string) {
			if err := w.host.RevealPath(w.ctx, path); err != nil {
				w.failure = err.Error()
			}
			w.shell.outline = false
		},
		Activate:          func(id string) { go w.action(func() error { return w.host.Activate(w.ctx, id) }) },
		ActivateWorkspace: func(id string) { go w.action(func() error { return w.host.ActivateWorkspace(w.ctx, id) }) },
		CloseDocument:     func(id string) { w.command(workspace.NativeCommand{Command: "close", ID: id}) },
		CloseWorkspace: func(id string) {
			for docID := range w.documents {
				if err := w.publish(docID, 0); err != nil {
					w.failure = err.Error()
					return
				}
			}
			go w.action(func() error { _, err := w.host.CloseWorkspaceRoot(w.ctx, id); return err })
		},
		OpenPath: w.openPath, Reveal: w.Reveal, SetMode: w.SetMode,
		OpenFolder:  func() { go w.action(func() error { _, err := w.host.PickFolder(w.ctx); return err }) },
		NewDocument: func() { go w.action(func() error { _, err := w.host.New(w.ctx); return err }) },
		OpenRecent:  func(path string) { go w.action(func() error { return w.host.OpenWorkspace(w.ctx, path) }) },
		ToggleFolder: func(rel string) {
			if err := w.host.ToggleFolder(w.ctx, rel); err != nil {
				w.failure = err.Error()
			}
		},
		Filter:     func(query string) { w.search, _ = w.host.SetFilter(w.ctx, query) },
		SetFeynman: func(value bool) { w.setPlugin("feynman", value) }, SetGit: func(value bool) { w.setPlugin("git", value) },
	}
	if !w.shell.ready {
		if v, err := strconv.ParseFloat(w.Preference("mgo.researchWidth"), 32); err == nil && v >= 310 && v <= 800 {
			w.shell.rightWidth = float32(v)
		}
	}
	oldWidth := w.shell.rightWidth
	w.shell.Draw(c, model, actions, slots)
	if oldWidth != 0 && oldWidth != w.shell.rightWidth {
		w.setPreference("mgo.researchWidth", strconv.Itoa(int(w.shell.rightWidth)))
	}
}
