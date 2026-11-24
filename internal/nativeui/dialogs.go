package nativeui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type windowDialog struct {
	kind, id, root string
	open           bool
	docs           []workspace.DocumentInfo
	ids            []string
	selected       []bool
	value, dir     string
	title, body    string
}

func (w *Window) setClosing(ids []string, closing bool) {
	if w.closing == nil {
		w.closing = map[string]bool{}
	}
	for _, id := range ids {
		if closing {
			w.closing[id] = true
		} else {
			delete(w.closing, id)
		}
	}
}

func (w *Window) cancelDialog() {
	if w.dialog == nil {
		return
	}
	if strings.HasPrefix(w.dialog.kind, "close") {
		w.setClosing(closeDialogIDs(w.dialog), false)
	}
	w.dialog = nil
}

func closeDialogIDs(d *windowDialog) []string {
	if d == nil {
		return nil
	}
	if len(d.ids) > 0 {
		return append([]string(nil), d.ids...)
	}
	if len(d.docs) > 0 {
		ids := make([]string, len(d.docs))
		for i, doc := range d.docs {
			ids[i] = doc.ID
		}
		return ids
	}
	if d.id != "" {
		return []string{d.id}
	}
	return nil
}

func (w *Window) requestClose(kind, root string, ids []string) {
	edited, err := w.host.ClosePreflight(w.ctx, ids)
	if err != nil {
		w.failure = err.Error()
		return
	}
	if len(edited) == 0 {
		w.performClose(&windowDialog{kind: kind, root: root, id: first(ids), ids: ids})
		return
	}
	w.setClosing(ids, true)
	d := &windowDialog{kind: kind, root: root, id: first(ids), ids: ids, open: true, docs: edited, selected: make([]bool, len(edited))}
	for i := range d.selected {
		d.selected[i] = true
	}
	if len(edited) == 1 {
		d.title = "Save changes to “" + edited[0].Name + "”?"
	} else {
		d.title = fmt.Sprintf("Save changes to %d documents?", len(edited))
	}
	d.body = "Your changes will be lost if you don't save them."
	w.dialog = d
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (w *Window) performClose(d *windowDialog) {
	w.runDialogAction(d, func() error {
		switch d.kind {
		case "closeDocument":
			_, err := w.host.CloseDocument(w.ctx, d.id)
			return err
		case "closeWorkspace":
			_, err := w.host.CloseWorkspace(w.ctx)
			return err
		case "closeRoot":
			_, err := w.host.CloseWorkspaceRoot(w.ctx, d.root)
			return err
		}
		return nil
	}, portrait.ToastSpec{})
}

func (w *Window) resolveClose(d *windowDialog, save bool) {
	w.dialog = nil
	ids := closeDialogIDs(d)
	w.runDialogAction(d, func() error {
		for i, doc := range d.docs {
			if doc.ID == "" {
				continue
			}
			shouldSave := save && (len(d.selected) == 0 || i >= len(d.selected) || d.selected[i])
			var err error
			if shouldSave {
				err = w.host.SaveForClose(w.ctx, doc.ID)
			} else {
				err = w.host.DiscardDocument(w.ctx, doc.ID)
			}
			if err != nil {
				return err
			}
		}
		switch d.kind {
		case "closeDocument":
			_, err := w.host.CloseDocument(w.ctx, first(ids))
			return err
		case "closeWorkspace":
			_, err := w.host.CloseWorkspace(w.ctx)
			return err
		case "closeRoot":
			_, err := w.host.CloseWorkspaceRoot(w.ctx, d.root)
			return err
		}
		return nil
	}, portrait.ToastSpec{})
}

func (w *Window) runDialogAction(d *windowDialog, run func() error, success portrait.ToastSpec) {
	ids := closeDialogIDs(d)
	go func() {
		err := run()
		w.win.Update(func() {
			w.setClosing(ids, false)
			w.failure = ""
			if err != nil && !errors.Is(err, mygo.ErrCanceled) {
				w.failure = err.Error()
			}
			if err == nil && success.Title != "" {
				w.toasts = append(w.toasts, success)
			}
		})
	}()
}

func (w *Window) requestSave(id string) {
	doc := w.documentInfo(id)
	if doc.Conflict {
		w.openConflict(id)
		return
	}
	if doc.ReadOnly {
		w.openReadOnly(id)
		return
	}
	go w.actionToast(func() error { return w.host.Save(w.ctx, id) }, portrait.ToastSpec{ID: "save:" + id, Tone: portrait.Tones.Success, Title: "Saved"})
}

func (w *Window) documentInfo(id string) workspace.DocumentInfo {
	for _, doc := range w.snapshot.Documents {
		if doc.ID == id {
			return doc
		}
	}
	return workspace.DocumentInfo{ID: id}
}

func (w *Window) openConflict(id string) {
	doc := w.documentInfo(id)
	w.dialog = &windowDialog{kind: "conflict", id: id, open: true, title: "“" + doc.Name + "” changed on disk", body: "Save your version over the disk file, or revert to the disk version. Your unsaved text remains open until you choose."}
}

func (w *Window) openReadOnly(id string) {
	doc := w.documentInfo(id)
	w.dialog = &windowDialog{kind: "readonly", id: id, open: true, title: "“" + doc.Name + "” is read only", body: "Choose Save As to save your changes to a new file."}
}

func (w *Window) openRename(id string) {
	doc := w.documentInfo(id)
	name := doc.Name
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	w.dialog = &windowDialog{kind: "rename", id: id, open: true, value: name, title: "Rename “" + name + "”", dir: fmt.Sprint(utf8.RuneCountInString(stem))}
}

func (w *Window) openRevert(id string) {
	doc := w.documentInfo(id)
	w.dialog = &windowDialog{kind: "revert", id: id, open: true, title: "Revert to saved?", body: "Revert “" + doc.Name + "” to the bytes on disk? Unsaved changes will be lost."}
}

func (w *Window) openMove(id string) {
	doc := w.documentInfo(id)
	w.dialog = &windowDialog{kind: "move", id: id, open: true, title: "Move “" + doc.Name + "”", body: "Choose a folder in this workspace, or choose another folder."}
}

func (w *Window) chooseMove(d *windowDialog, directory string) {
	if directory == "" {
		return
	}
	target := workspace.MoveDestination(w.documentInfo(d.id).Path, directory)
	if target != "" && target != w.documentInfo(d.id).Path {
		if _, err := os.Lstat(target); err == nil {
			d.kind, d.dir, d.open = "moveReplace", directory, true
			d.title, d.body = "Replace “"+filepath.Base(target)+"”?", "A file with that name already exists in the destination. Replacing it cannot be undone."
			w.dialog = d
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			w.failure = err.Error()
			return
		}
	}
	w.dialog = nil
	w.runDialogAction(d, func() error { return w.host.MoveToFolder(w.ctx, d.id, directory, false) }, portrait.ToastSpec{})
}

// ShowAlert presents errors in the Calm Portrait window rather than NSAlert.
func (w *Window) ShowAlert(title, body string) {
	if w == nil {
		return
	}
	w.dialog = &windowDialog{kind: "alert", open: true, title: title, body: body}
	w.win.Invalidate()
}

// OpenFiles and OpenRecentDocument keep Open errors inside the Portrait UI.
func (w *Window) OpenFiles() {
	go func() {
		err := w.host.OpenFiles(w.ctx)
		w.win.Update(func() {
			if err != nil && !errors.Is(err, mygo.ErrCanceled) {
				w.ShowAlert("Could not open document", err.Error())
			}
		})
	}()
}

func (w *Window) OpenRecentDocument(path string) {
	go func() {
		_, err := w.host.Open(w.ctx, path)
		w.win.Update(func() {
			if err != nil {
				w.ShowAlert("Could not open recent document", err.Error())
			}
		})
	}()
}

func (w *Window) drawDialog(c *ui.Context) {
	d := w.dialog
	if d == nil {
		return
	}
	switch d.kind {
	case "closeDocument", "closeWorkspace", "closeRoot":
		docs := make([]portrait.DecisionDocument, len(d.docs))
		for i, doc := range d.docs {
			docs[i] = portrait.DecisionDocument{ID: doc.ID, Name: doc.Name, Untitled: doc.Path == ""}
		}
		save, discard := "Save", "Don't Save"
		if len(d.docs) > 1 {
			save, discard = "Save Selected", "Discard All"
		} else if len(d.docs) == 1 && d.docs[0].Path == "" {
			save = "Save…"
		}
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, Documents: docs, Selected: &d.selected, SaveLabel: save, DiscardLabel: discard, OnSave: func() { w.resolveClose(d, true) }, OnDiscard: func() { w.resolveClose(d, false) }, OnCancel: w.cancelDialog})
	case "conflict":
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, SaveLabel: "Save Anyway", DiscardLabel: "Revert to Disk", OnSave: func() {
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.SaveAnyway(w.ctx, d.id) }, portrait.ToastSpec{ID: "save:" + d.id, Tone: portrait.Tones.Success, Title: "Saved"})
		}, OnDiscard: func() {
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.Revert(w.ctx, d.id) }, portrait.ToastSpec{ID: "restore:" + d.id, Tone: portrait.Tones.Success, Title: "Restored"})
		}, OnCancel: w.cancelDialog})
	case "readonly":
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, SaveLabel: "Save As…", HideDiscard: true, OnSave: func() {
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.SaveAs(w.ctx, d.id) }, portrait.ToastSpec{ID: "save:" + d.id, Tone: portrait.Tones.Success, Title: "Saved"})
		}, OnCancel: w.cancelDialog})
	case "rename":
		selectTo, _ := strconv.Atoi(d.dir)
		portrait.Prompt(c, &d.open, portrait.PromptSpec{Title: d.title, Label: "File name", Value: &d.value, SelectTo: selectTo, SubmitLabel: "Rename", Validate: func(name string) string {
			if err := workspace.ValidateDocumentName(name); err != nil {
				return err.Error()
			}
			return ""
		}, OnSubmit: func() {
			name := d.value
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.RenameTo(w.ctx, d.id, name) }, portrait.ToastSpec{})
		}})
	case "revert":
		portrait.Confirm(c, &d.open, portrait.ConfirmSpec{Title: d.title, Body: d.body, ConfirmLabel: "Revert", OnConfirm: func() {
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.Revert(w.ctx, d.id) }, portrait.ToastSpec{ID: "restore:" + d.id, Tone: portrait.Tones.Success, Title: "Restored"})
		}})
	case "moveReplace":
		portrait.Confirm(c, &d.open, portrait.ConfirmSpec{Title: d.title, Body: d.body, ConfirmLabel: "Replace", OnConfirm: func() {
			w.dialog = nil
			w.runDialogAction(d, func() error { return w.host.MoveToFolder(w.ctx, d.id, d.dir, true) }, portrait.ToastSpec{})
		}})
	case "move":
		portrait.DialogShell(c, "move document", &d.open, portrait.DialogSpec{Label: d.title, Width: portrait.Form.DialogWidth}, func() {
			portrait.Type.Title.Apply(ui.Text(c, d.title)).FontWeight(600).TextColor(portrait.Colors.Ink).Wrap()
			portrait.Type.Small.Apply(ui.Text(c, d.body)).TextColor(portrait.Colors.Ink2).Wrap()
			ui.Scroll(c).MaxHeight(portrait.Form.ListMax).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(portrait.Form.PopoverGap).Children(func() {
					if portrait.ButtonWith(c, w.snapshot.Name, "Move to "+w.snapshot.Root, portrait.ButtonSpec{Look: portrait.Quiet, Leading: portrait.IconFolder}).AlignSelf(ui.Start).Clicked() {
						w.chooseMove(d, w.snapshot.Root)
					}
					for _, entry := range w.navigation.Entries {
						if entry.Kind == "folder" {
							entry := entry
							label := strings.Repeat("  ", entry.Depth) + entry.Name
							if portrait.ButtonWith(c.Key(entry.Path), label, "Move to "+entry.Path, portrait.ButtonSpec{Look: portrait.Quiet, Leading: portrait.IconFolder}).AlignSelf(ui.Start).Clicked() {
								w.chooseMove(d, entry.Path)
							}
						}
					}
				})
			})
			ui.Row(c).Justify(ui.End).Gap(portrait.Form.DialogGap).Children(func() {
				if portrait.QuietButton(c, "Cancel", "Cancel move").Clicked() {
					d.open = false
				}
				if portrait.Button(c, "Choose another folder…", "Choose another folder").Clicked() {
					go func() {
						directory, err := w.host.ChooseFolder(w.ctx)
						w.win.Update(func() {
							if err == nil {
								w.chooseMove(d, directory)
							} else if !errors.Is(err, mygo.ErrCanceled) {
								w.failure = err.Error()
							}
						})
					}()
				}
			})
		}).Padding(portrait.Form.DialogPad).Gap(portrait.Form.DialogGap)
	case "alert":
		portrait.DialogShell(c, "portrait alert", &d.open, portrait.DialogSpec{Label: d.title, Width: portrait.Form.DialogWidth, Role: ui.RoleAlertDialog}, func() {
			portrait.Type.Title.Apply(ui.Text(c, d.title)).FontWeight(600).TextColor(portrait.Colors.Ink).Wrap()
			portrait.Type.Small.Apply(ui.Text(c, d.body)).TextColor(portrait.Colors.Ink2).Wrap()
			ui.Row(c).Justify(ui.End).Children(func() {
				if portrait.AccentButton(c, "OK", "Dismiss alert").AutoFocus().Clicked() {
					d.open = false
				}
			})
		}).Padding(portrait.Form.DialogPad).Gap(portrait.Form.DialogGap)
	}
	if w.dialog == d && !d.open {
		w.cancelDialog()
	}
}
