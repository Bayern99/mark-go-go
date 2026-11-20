package nativeui

import (
	"context"
	"errors"
	"slices"

	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type openDocument struct {
	source        *nativeeditor.Document
	published     uint64
	reloadVersion int
}

type Window struct {
	host                          *workspace.Service
	win                           *mygo.Window
	ctx                           context.Context
	snapshot                      workspace.WorkspaceSnapshot
	documents                     map[string]*openDocument
	focusSource                   bool
	menuKnown, menuUndo, menuRedo bool
	failure                       string
}

func NewWindow(host *workspace.Service, opts mygo.WindowOptions) *mygo.Window {
	w := &Window{host: host, documents: make(map[string]*openDocument)}
	opts.Content = ui.View(w.View)
	mygo.RunOnMain(func() {
		w.win = mygo.NewWindow(opts)
		w.ctx = desktop.WithWindow(context.Background(), w.win)
		workspace.WorkspaceChanged.Listen(w.win, func(_ workspace.WorkspaceSnapshot) { w.refresh() })
		workspace.DocumentReloaded.Listen(w.win, w.reload)
		workspace.CommandRequested.Listen(w.win, w.command)
		w.win.OnFocus(func() { w.menuKnown = false; w.updateMenu() })
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
	w.snapshot = next
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
		d.SetMode(nativeeditor.Mode(snap.State.ViewMode))
		w.documents[id] = &openDocument{source: d, published: d.Version(), reloadVersion: snap.Version}
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

func (w *Window) View(c *ui.Context) {
	ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
		if len(w.snapshot.Documents) > 1 {
			ui.Row(c).Padding(8).Gap(4).Children(func() {
				for _, info := range w.snapshot.Documents {
					if ui.Button(c.Key(info.ID), info.Name).Selected(info.ID == w.snapshot.ActiveID).Clicked() {
						go w.action(func() error { return w.host.Activate(w.ctx, info.ID) })
					}
				}
			})
		}
		if w.failure != "" {
			ui.Text(c, w.failure).Role(ui.RoleAlert)
		}
		if w.snapshot.RestoreError != "" {
			ui.Text(c, w.snapshot.RestoreError).Role(ui.RoleAlert)
		}
		id := w.snapshot.ActiveID
		d := w.documents[id]
		if d == nil {
			if ui.Button(c, "New document").Clicked() {
				go w.action(func() error { _, err := w.host.New(w.ctx); return err })
			}
			return
		}
		var active workspace.DocumentInfo
		for _, info := range w.snapshot.Documents {
			if info.ID == id {
				active = info
				break
			}
		}
		if active.Conflict {
			ui.Text(c, "File changed on disk").Role(ui.RoleAlert)
		}
		if active.Deleted {
			ui.Text(c, "File deleted on disk").Role(ui.RoleAlert)
		}
		if active.ReadOnly {
			ui.Text(c, "Read only")
		}
		w.editorModeSelector(c, d)
		field := d.source.View(c.Key("source"), active.ReadOnly).Grow(1).FillWidth()
		if w.focusSource {
			field.Focus()
			w.focusSource = false
		}
		if err := w.publish(id, 0); err != nil {
			w.failure = err.Error()
		}
	})
}
