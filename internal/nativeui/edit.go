package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func (w *Window) updateMenu() {
	if w.win == nil || !w.win.IsFocused() {
		return
	}
	d := w.documents[w.snapshot.ActiveID]
	var undo, redo bool
	if d != nil && w.viewMode(w.Active()) != "genui" && d.source.Focused() {
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
	if d == nil || w.viewMode(w.Active()) == "genui" || !d.source.Focused() {
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
