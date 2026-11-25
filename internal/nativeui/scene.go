package nativeui

import (
	"context"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// SceneWindow builds a Window around recorded workspace state, for the scene
// exports and the contract tests that must draw the interface a real window
// draws. It has no host and no native window: every dialog it shows comes from
// the same drawDialog a running window uses.
func SceneWindow(snapshot workspace.WorkspaceSnapshot, navigation workspace.NavSnapshot, folders []workspace.NavEntry, source string, space portrait.Space, dark bool) *Window {
	if space.Name == "" {
		space = portrait.DefaultSpace
	}
	portrait.Use(space, dark)
	w := &Window{
		snapshot:     snapshot,
		navigation:   navigation,
		documents:    map[string]*openDocument{},
		sceneFolders: folders,
		ctx:          context.Background(),
	}
	if snapshot.ActiveID != "" {
		doc := nativeeditor.New(source)
		w.documents[snapshot.ActiveID] = &openDocument{source: doc, published: doc.Version()}
	}
	return w
}

// sceneInfo is the first document of the recorded workspace.
func (w *Window) sceneInfo() workspace.DocumentInfo {
	if w.snapshot.ActiveID != "" {
		return w.documentInfo(w.snapshot.ActiveID)
	}
	if len(w.snapshot.Documents) > 0 {
		return w.snapshot.Documents[0]
	}
	return workspace.DocumentInfo{ID: "untitled", Name: "Untitled"}
}

// SceneDialog opens one production dialog by the name the acceptance scenes
// use. It returns false for a name this window does not know.
func (w *Window) SceneDialog(kind string) bool {
	doc := w.sceneInfo()
	w.dialog = nil
	switch kind {
	case "unsaved_single":
		w.showDialog(w.closeDecision("closeDocument", w.snapshot.Root, []string{doc.ID}, workspace.ClosePlan{Prompt: []workspace.DocumentInfo{doc}}))
	case "unsaved_multi":
		docs := w.snapshot.Documents
		if len(docs) < 2 {
			docs = append(append([]workspace.DocumentInfo(nil), docs...), workspace.DocumentInfo{ID: "untitled", Name: "Untitled 2"})
		}
		ids := infoIDs(docs)
		w.showDialog(w.closeDecision("closeWorkspace", w.snapshot.Root, ids, workspace.ClosePlan{Prompt: docs}))
	case "conflict_decision":
		w.openConflict(doc.ID)
	case "readonly_decision":
		w.openReadOnly(doc.ID)
	case "rename_prompt":
		w.openRename(doc.ID)
	case "move_picker":
		w.openMove(doc.ID)
	case "revert_confirm":
		w.openRevert(doc.ID)
	case "quit_failed":
		w.ShowAlert("Could not quit Mark Go Go", "Mark Go Go could not safely prepare all open documents for quitting.\n\nYour documents are still open. Resolve the issue, then choose Quit again.")
	default:
		return false
	}
	return w.dialog != nil
}

// DrawDialog renders the open dialog inside the Space's theme, so its fields,
// titles and Backdrop take the tokens the shell paints with rather than the
// system appearance's.
func (w *Window) DrawDialog(c *ui.Context) {
	if w.dialog == nil {
		return
	}
	t := portrait.Theme()
	themed(c, &t).Absolute().Size(0, 0).Children(func() { w.drawDialog(c) })
}

// DialogOpen reports whether a dialog is up, for a test or a scene.
func (w *Window) DialogOpen() bool { return w.dialog != nil }
