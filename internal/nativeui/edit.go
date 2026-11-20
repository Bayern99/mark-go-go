package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func (w *Window) editorModeSelector(c *ui.Context, d *openDocument) {
	ui.Row(c).Gap(4).Children(func() {
		for _, choice := range []struct {
			mode  nativeeditor.Mode
			label string
		}{{nativeeditor.Preview, "Preview"}, {nativeeditor.Markdown, "Markdown"}} {
			if ui.Button(c, choice.label).Selected(d.source.Mode() == choice.mode).Clicked() && d.source.SetMode(choice.mode) {
				w.focusSource = true
				id := w.snapshot.ActiveID
				go w.action(func() error { return w.host.SetViewMode(w.ctx, id, string(choice.mode)) })
			}
		}
	})
}

func (w *Window) updateMenu() {
	if w.win == nil || !w.win.IsFocused() {
		return
	}
	d := w.documents[w.snapshot.ActiveID]
	var undo, redo bool
	if d != nil && d.source.Focused() {
		undo, redo = d.source.CanUndo(), d.source.CanRedo()
	} else {
		undo, redo = w.win.CanEditContent("undo"), w.win.CanEditContent("redo")
	}
	if w.menuKnown && w.menuUndo == undo && w.menuRedo == redo {
		return
	}
	workspace.ApplyNativeEditMenu(undo, redo)
	w.menuKnown, w.menuUndo, w.menuRedo = true, undo, redo
}

func (w *Window) editCommand(cmd string, d *openDocument) {
	if d == nil || !d.source.Focused() {
		if w.win.CanEditContent(cmd) {
			w.win.EditContent(cmd)
		}
		return
	}
	if cmd == "undo" {
		d.source.Undo()
	} else {
		d.source.Redo()
	}
}
