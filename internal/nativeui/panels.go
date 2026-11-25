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
	Settings                             []SettingsPage
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
	if id == "" || w.Source() == nil || w.viewMode(w.Active()) == mode {
		return
	}
	if mode != "genui" && mode != "preview" && mode != "markdown" {
		return
	}
	if mode == "genui" && !w.PluginEnabled("genui") {
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

// ToggleMode switches between reading and Markdown without exposing the
// editor's Command-/ comment shortcut on the window's view command.
func (w *Window) ToggleMode() {
	mode := "preview"
	if w.viewMode(w.Active()) != "markdown" {
		mode = "markdown"
	}
	w.SetMode(mode)
}

// pluginsShippedOn are the plugins a reader keeps until they turn them off.
// GenUI ships on so a reader who already saved views keeps them after the
// plugin switch arrives.
var pluginsShippedOn = map[string]bool{"genui": true}

func (w *Window) PluginEnabled(name string) bool {
	flag := w.Preference("mgo.plugin." + name)
	if pluginsShippedOn[name] {
		return flag != "off"
	}
	return flag == "on"
}

// fallbackMode keeps GenUI out of reach while its plugin is off: a saved or
// restored GenUI mode reads as Preview instead.
func fallbackMode(mode string, genui bool) string {
	if mode == "genui" && !genui {
		return "preview"
	}
	return mode
}

func (w *Window) viewMode(doc workspace.DocumentInfo) string {
	return fallbackMode(doc.ViewMode, w.PluginEnabled("genui"))
}

// syncEditorMode moves the active document's editor to the view it shows when
// GenUI is turned off, so typing and undo work before another refresh.
func (w *Window) syncEditorMode() {
	doc := w.Active()
	if doc.ID == "" {
		return
	}
	mode := w.viewMode(doc)
	if mode == doc.ViewMode {
		return
	}
	if d := w.documents[doc.ID]; d != nil {
		d.source.SetMode(nativeeditor.Mode(mode))
	}
}

// pluginSetting is one row of Settings / Plugins. The switch writes the
// plugin preference; Configure opens the plugin's own settings page.
type pluginSetting struct {
	name, title, description, keyword, configure string
	on                                           bool
	set                                          func(bool)
}

func pluginSettings(m Model, a Actions) []pluginSetting {
	return []pluginSetting{
		{"Feynman", "Feynman Research", "Side-by-side AI research assistant, models, and MCP tools.", "feynman plugin", "models", m.Feynman, a.SetFeynman},
		{"GenUI", "GenUI", "Read-only reading views derived from the Markdown.", "genui generation view plugin", "genui", m.GenUI, a.SetGenUI},
		{"Git", "Git Checkpoints", "Save, diff, and restore local document snapshots.", "git plugin checkpoints", "", m.Git, a.SetGit},
	}
}

func (p pluginSetting) matches(q SettingsQuery) bool {
	return q.Matches(p.title, p.description, p.keyword)
}

// settingsPageOpen reports whether a plugin page is registered and visible, so
// Plugins never links to a destination the Settings Center would not show.
func settingsPageOpen(pages []SettingsPage, id string) bool {
	if id == "" || !settingsPageVisible(pages, id) {
		return false
	}
	page := settingsPageByID(pages, id)
	return page.Visible == nil || page.Visible()
}

func (w *Window) Preference(key string) string { return w.store().values[key] }

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
	if len(p.Settings) != 0 {
		w.slots.Settings = append(w.slots.Settings, p.Settings...)
	}
	if p.Changed != nil {
		w.changed = append(w.changed, p.Changed)
	}
	if p.Closed != nil {
		w.win.OnClosed(p.Closed)
	}
}
