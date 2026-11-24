package nativeui

import (
	"context"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type Panels struct {
	GenUI, Research, Checkpoints, Review func(*ui.Context)
	FeynmanSettings                      func(*ui.Context, string)
	Changed, Closed                      func()
}

func (w *Window) Active() workspace.DocumentInfo         { return activeDocument(w.snapshot) }
func (w *Window) Workspace() workspace.WorkspaceSnapshot { return w.snapshot }
func (w *Window) Context() context.Context               { return w.ctx }
func (w *Window) Native() *mygo.Window                   { return w.win }
func (w *Window) Source() *nativeeditor.Document {
	if d := w.documents[w.snapshot.ActiveID]; d != nil {
		return d.source
	}
	return nil
}
func (w *Window) Publish() error { return w.publish(w.snapshot.ActiveID, 0) }
func (w *Window) Reveal(offset int) {
	d := w.Source()
	if d == nil {
		return
	}
	if err := d.Reveal(offset); err != nil {
		w.failure = err.Error()
		return
	}
	if w.Active().ViewMode == "genui" {
		w.SetMode("preview")
		w.documents[w.snapshot.ActiveID].returnToGenUI = true
	}
	w.focusSource = true
	w.win.Invalidate()
}
func (w *Window) SetMode(mode string) {
	id := w.snapshot.ActiveID
	if d := w.documents[id]; d != nil {
		d.returnToGenUI = false
	}
	if id == "" || w.Source() == nil || w.Active().ViewMode == mode {
		return
	}
	if mode != "genui" && mode != "preview" && mode != "markdown" {
		return
	}
	if err := w.host.SetViewMode(w.ctx, id, mode); err != nil {
		w.failure = err.Error()
		return
	}
	if mode != "genui" {
		w.Source().SetMode(nativeeditor.Mode(mode))
		w.focusSource = true
	}
	if path := w.Active().Path; path != "" {
		w.setPreference("mgo.viewMode:"+path, mode)
	}
}
func (w *Window) PluginEnabled(name string) bool { return w.Preference("mgo.plugin."+name) == "on" }
func (w *Window) Preference(key string) string   { return w.store().values[key] }

func (w *Window) install(p Panels) {
	if p.GenUI != nil {
		w.slots.GenUI = p.GenUI
	}
	if p.Research != nil {
		w.slots.Research = p.Research
	}
	if p.Checkpoints != nil {
		w.slots.Checkpoints = p.Checkpoints
	}
	if p.Review != nil {
		w.slots.Review = p.Review
	}
	if p.FeynmanSettings != nil {
		w.slots.FeynmanSettings = p.FeynmanSettings
	}
	if p.Changed != nil {
		w.changed = append(w.changed, p.Changed)
	}
	if p.Closed != nil {
		w.win.OnClosed(p.Closed)
	}
}
