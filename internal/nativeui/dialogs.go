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
	open, busy     bool
	plan           workspace.ClosePlan
	docs           []workspace.DocumentInfo
	ids            []string
	selected       []bool
	folders        []workspace.NavEntry
	value, dir     string
	title, body    string
}

func isCloseDialog(d *windowDialog) bool {
	return d != nil && strings.HasPrefix(d.kind, "close")
}

// setClosing protects documents whose edits must not reach the model while a
// close is being resolved, and releases them once it is over.
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

// cancelDialog closes the dialog that is up. Only the close dialog owns
// closing flags, so a rename or move prompt never strands another dialog's
// documents: those flags outlive every replacement and are released by the
// close flow itself.
func (w *Window) cancelDialog() {
	if w.dialog == nil {
		return
	}
	if isCloseDialog(w.dialog) {
		w.setClosing(closeDialogIDs(w.dialog), false)
	}
	w.dialog = nil
	w.showNextDialog()
}

// showDialog replaces the dialog that is up. A close decision owns its
// documents until it resolves, so a later alert waits its turn instead of
// replacing the sheet and stranding the flags; every other request waits too,
// as a sheet blocks its window's commands.
func (w *Window) showDialog(d *windowDialog) {
	if w.dialog != nil && isCloseDialog(w.dialog) {
		if d.kind == "alert" {
			w.alerts = append(w.alerts, d)
		}
		return
	}
	w.dialog = d
}

// showNextDialog presents the first queued alert once the sheet is gone.
func (w *Window) showNextDialog() {
	for w.dialog == nil && len(w.alerts) > 0 {
		d := w.alerts[0]
		w.alerts = w.alerts[1:]
		w.dialog = d
	}
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

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// requestClose preflights every document the close targets. Documents that
// autosave in place are written first and never asked about; only untitled,
// read-only, conflicted, deleted or failed documents reach the sheet.
func (w *Window) requestClose(kind, root string, ids []string) {
	if isCloseDialog(w.dialog) {
		return
	}
	targets := w.closeTargets(kind, root, ids)
	for _, id := range targets {
		if err := w.publish(id, 0); err != nil {
			w.failure = err.Error()
			return
		}
	}
	plan, err := w.host.ClosePreflight(w.ctx, targets)
	if err != nil {
		w.failure = err.Error()
		return
	}
	if plan.NeedsDecision() {
		w.setClosing(targets, true)
		d := w.closeDecision(kind, root, targets, plan)
		w.showDialog(d)
		return
	}
	if len(plan.Silent) == 0 {
		w.performClose(&windowDialog{kind: kind, root: root, id: first(targets), ids: targets})
		return
	}
	w.setClosing(targets, true)
	d := &windowDialog{kind: kind, root: root, id: first(targets), ids: targets, plan: plan, open: true, busy: true, title: "Saving changes…"}
	w.showDialog(d)
	go w.commitClose(d, workspace.CloseSelection{Silent: infoIDs(plan.Silent)})
}

// closeTargets resolves the documents a close covers from the workspace
// service, so a restored tab the window never activated is still asked about.
func (w *Window) closeTargets(kind, root string, ids []string) []string {
	if kind == "closeDocument" {
		return ids
	}
	targets, err := w.host.CloseTargetIDs(w.ctx, kind, root)
	if err != nil || len(targets) == 0 {
		return ids
	}
	return targets
}

func (w *Window) closeDecision(kind, root string, ids []string, plan workspace.ClosePlan) *windowDialog {
	d := &windowDialog{kind: kind, root: root, id: first(ids), ids: ids, plan: plan, open: true, docs: plan.Prompt, selected: make([]bool, len(plan.Prompt))}
	for i := range d.selected {
		d.selected[i] = true
	}
	if len(plan.Prompt) == 1 {
		d.title = "Save changes to “" + plan.Prompt[0].Name + "”?"
	} else {
		d.title = fmt.Sprintf("Save changes to %d documents?", len(plan.Prompt))
	}
	d.body = "Your changes will be lost if you don't save them."
	return d
}

func (w *Window) performClose(d *windowDialog) {
	w.runDialogAction(d, func() error { _, err := w.closeDocumentTargets(d); return err }, portrait.ToastSpec{})
}

// resolveClose answers the sheet. Every fallible save runs before anything is
// discarded, and the close itself only runs once all of them succeeded. The
// sheet stays up, showing progress, so the documents it protects cannot take
// an edit the close would then drop.
func (w *Window) resolveClose(d *windowDialog, save bool) {
	d.busy, d.open = true, true
	w.dialog = d
	sel := workspace.SelectClose(d.plan, save, d.selected)
	go w.commitClose(d, sel)
}

func (w *Window) commitClose(d *windowDialog, sel workspace.CloseSelection) {
	unsaved, err := w.host.ResolveClose(w.ctx, sel)
	committed := false
	if err == nil && len(unsaved) == 0 {
		if cerr := w.host.CommitClose(w.ctx, sel); cerr != nil {
			err = cerr
		} else {
			committed = true
		}
	}
	var closeRes workspace.CloseResult
	var closeErr error
	if err == nil && len(unsaved) == 0 {
		closeRes, closeErr = w.closeDocumentTargets(d)
		if committed && (closeRes.Canceled || closeErr != nil) && len(sel.Discard) > 0 {
			w.host.RestoreDirty(w.ctx, sel.Discard)
		}
	}
	w.win.Update(func() {
		w.setClosing(closeDialogIDs(d), false)
		if w.dialog == d {
			w.dialog = nil
		}
		d.busy = false
		w.failure = ""
		if len(unsaved) > 0 {
			w.promoteClose(unsaved)
			return
		}
		switch {
		case closeRes.Canceled, errors.Is(err, mygo.ErrCanceled), errors.Is(closeErr, mygo.ErrCanceled):
			w.showNextDialog()
		case err != nil:
			w.ShowAlert("Could not close", err.Error())
		case closeErr != nil:
			w.ShowAlert("Could not close", closeErr.Error())
		}
		w.showNextDialog()
	})
}

// promoteClose sends a document whose save failed back to a Portrait sheet
// instead of a raw error string.
func (w *Window) promoteClose(unsaved []workspace.DocumentInfo) {
	if len(unsaved) == 1 {
		doc := unsaved[0]
		switch {
		case doc.Conflict:
			w.openConflict(doc.ID)
			return
		case doc.ReadOnly:
			w.openReadOnly(doc.ID)
			return
		case doc.Deleted:
			w.openConflict(doc.ID)
			return
		}
	}
	var names, reasons []string
	for _, doc := range unsaved {
		names = append(names, "“"+doc.Name+"”")
		if doc.ActionError != "" {
			reasons = append(reasons, doc.ActionError)
		}
	}
	body := "Your changes are still open. Save again from the document, or choose Don't Save to close without them."
	if len(reasons) > 0 {
		body = reasons[0] + ". " + body
	}
	w.ShowAlert("Could not save "+strings.Join(names, ", "), body)
}

// closeDocumentTargets runs the close itself, now that every edit is either
// written or explicitly discarded.
func (w *Window) closeDocumentTargets(d *windowDialog) (workspace.CloseResult, error) {
	switch d.kind {
	case "closeDocument":
		return w.host.CloseDocument(w.ctx, d.id)
	case "closeWorkspace":
		return w.host.CloseWorkspace(w.ctx)
	case "closeRoot":
		return w.host.CloseWorkspaceRootIDs(w.ctx, d.root, d.ids)
	}
	return workspace.CloseResult{}, nil
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
			w.showNextDialog()
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
	go w.actionDialog("Could not save", func() error { return w.host.Save(w.ctx, id) }, portrait.ToastSpec{ID: "save:" + id, Tone: portrait.Tones.Success, Title: "Saved"})
}

// actionDialog runs an action and routes its failure to the sheet that can
// resolve it: a conflict or read-only document gets its Portrait decision,
// and anything else an in-window alert instead of a native dialog.
func (w *Window) actionDialog(title string, run func() error, success portrait.ToastSpec) {
	err := run()
	w.win.Update(func() {
		w.failure = ""
		switch {
		case err == nil:
			if success.Title != "" {
				w.toasts = append(w.toasts, success)
			}
		case errors.Is(err, mygo.ErrCanceled):
		case errors.Is(err, workspace.ErrSaveConflict):
			if strings.HasPrefix(success.ID, "save:") {
				w.openConflict(strings.TrimPrefix(success.ID, "save:"))
				return
			}
			w.ShowAlert(title, workspace.ErrSaveConflict.Error())
		case errors.Is(err, workspace.ErrSaveReadOnly):
			if strings.HasPrefix(success.ID, "save:") {
				w.openReadOnly(strings.TrimPrefix(success.ID, "save:"))
				return
			}
			w.ShowAlert(title, workspace.ErrSaveReadOnly.Error())
		default:
			w.ShowAlert(title, err.Error())
		}
	})
}

// saveConflict is the inline conflict alert's Save Anyway: the user chose to
// keep their version, so it writes without asking again.
func (w *Window) saveConflict(id string) {
	go w.actionDialog("Could not save the document", func() error { return w.host.SaveAnyway(w.ctx, id) }, portrait.ToastSpec{ID: "save:" + id, Tone: portrait.Tones.Success, Title: "Saved"})
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
	w.showDialog(&windowDialog{kind: "conflict", id: id, open: true, title: "“" + doc.Name + "” changed on disk", body: "Save your version over the disk file, or revert to the disk version. Your unsaved text remains open until you choose."})
}

func (w *Window) openReadOnly(id string) {
	doc := w.documentInfo(id)
	w.showDialog(&windowDialog{kind: "readonly", id: id, open: true, title: "“" + doc.Name + "” is read only", body: "Choose Save As to save your changes to a new file."})
}

func (w *Window) openRename(id string) {
	doc := w.documentInfo(id)
	name := doc.Name
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	w.showDialog(&windowDialog{kind: "rename", id: id, open: true, value: name, title: "Rename “" + name + "”", dir: fmt.Sprint(utf8.RuneCountInString(stem))})
}

func (w *Window) openRevert(id string) {
	doc := w.documentInfo(id)
	w.showDialog(&windowDialog{kind: "revert", id: id, open: true, title: "Revert to saved?", body: "Revert “" + doc.Name + "” to the bytes on disk? Unsaved changes will be lost."})
}

func (w *Window) openMove(id string) {
	doc := w.documentInfo(id)
	w.showDialog(&windowDialog{kind: "move", id: id, open: true, title: "Move “" + doc.Name + "”", body: "Choose a folder in this workspace, or choose another folder.", folders: w.moveFolders()})
}

// moveFolders lists every folder of the workspace, so a move can reach one the
// sidebar has collapsed. A scene supplies the list with the rest of its data.
func (w *Window) moveFolders() []workspace.NavEntry {
	if w.host == nil {
		return w.sceneFolders
	}
	folders, _ := w.host.Folders(w.ctx)
	return folders
}

// moveCurrent reports whether directory already holds the document.
func (w *Window) moveCurrent(d *windowDialog, directory string) bool {
	doc := w.documentInfo(d.id)
	return directory != "" && filepath.Clean(directory) == filepath.Clean(filepath.Dir(doc.Path))
}

// chooseMove commits a chosen destination. A taken name asks first and is
// only replaced once the user agrees.
func (w *Window) chooseMove(d *windowDialog, directory string) {
	if directory == "" || isCloseDialog(w.dialog) {
		return
	}
	doc := w.documentInfo(d.id)
	target := workspace.MoveTarget(doc, directory)
	if target != "" && !moveSameFile(target, doc.Path) {
		if _, err := os.Lstat(target); err == nil {
			d.kind, d.dir, d.open = "moveReplace", directory, true
			d.title, d.body = "Replace “"+filepath.Base(target)+"”?", "A file with that name already exists in the destination. Replacing it cannot be undone."
			w.showDialog(d)
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			w.ShowAlert("Could not move the document", err.Error())
			return
		}
	}
	w.cancelDialog()
	go w.actionDialog("Could not move the document", func() error { return w.host.MoveToFolder(w.ctx, d.id, directory, false) }, portrait.ToastSpec{})
}

func moveSameFile(target, path string) bool {
	if target == "" || path == "" {
		return false
	}
	return filepath.Clean(target) == filepath.Clean(path)
}

// ShowAlert presents errors in the Calm Portrait window rather than NSAlert.
func (w *Window) ShowAlert(title, body string) {
	if w == nil {
		return
	}
	w.showDialog(&windowDialog{kind: "alert", open: true, title: title, body: body})
	if w.win != nil {
		w.win.Invalidate()
	}
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
		if d.busy {
			// The saves are running. Escape cannot dismiss this one: the
			// documents stay protected until the close resolves.
			open := true
			portrait.DialogShell(c, "close progress", &open, portrait.DialogSpec{Label: d.title, Width: portrait.Form.DialogWidth, Role: ui.RoleAlertDialog, Glass: true}, func() {
				portrait.Type.Title.Apply(ui.Text(c, d.title)).FontWeight(600).TextColor(portrait.Colors.Ink).Wrap()
				ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					portrait.Spinner(c, "Saving changes")
					portrait.Type.Small.Apply(ui.Text(c, "Saving your changes…")).TextColor(portrait.Colors.Ink2).Wrap()
				})
			}).Padding(portrait.Form.DialogPad).Gap(portrait.Form.DialogGap)
			return
		}
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
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, Documents: docs, Selected: &d.selected, SaveLabel: save, DiscardLabel: discard, Return: &w.focusReturn, OnSave: func() { w.resolveClose(d, true) }, OnDiscard: func() { w.resolveClose(d, false) }, OnCancel: w.cancelDialog})
	case "conflict":
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, SaveLabel: "Save Anyway", DiscardLabel: "Revert to Disk", Return: &w.focusReturn, OnSave: func() {
			w.dialog = nil
			go w.actionDialog("Could not save the document", func() error { return w.host.SaveAnyway(w.ctx, d.id) }, portrait.ToastSpec{ID: "save:" + d.id, Tone: portrait.Tones.Success, Title: "Saved"})
		}, OnDiscard: func() {
			w.dialog = nil
			w.showDialog(&windowDialog{kind: "revert", id: d.id, open: true, title: "Revert to disk?", body: "Replace your unsaved text with the version on disk? This cannot be undone."})
		}, OnCancel: w.cancelDialog})
	case "readonly":
		portrait.Decision(c, &d.open, portrait.DecisionSpec{Title: d.title, Body: d.body, SaveLabel: "Save As…", HideDiscard: true, Return: &w.focusReturn, OnSave: func() {
			w.dialog = nil
			go w.actionDialog("Could not save the document", func() error { return w.host.SaveAs(w.ctx, d.id) }, portrait.ToastSpec{ID: "save:" + d.id, Tone: portrait.Tones.Success, Title: "Saved"})
		}, OnCancel: w.cancelDialog})
	case "rename":
		selectTo, _ := strconv.Atoi(d.dir)
		portrait.Prompt(c, &d.open, portrait.PromptSpec{Title: d.title, Label: "File name", Value: &d.value, SelectTo: selectTo, SubmitLabel: "Rename", Return: &w.focusReturn, Validate: func(name string) string {
			if err := workspace.ValidateDocumentName(name); err != nil {
				return err.Error()
			}
			return ""
		}, OnSubmit: func() {
			name := d.value
			w.dialog = nil
			go w.actionDialog("Could not rename the document", func() error { return w.host.RenameTo(w.ctx, d.id, name) }, portrait.ToastSpec{})
		}})
	case "revert":
		portrait.Confirm(c, &d.open, portrait.ConfirmSpec{Title: d.title, Body: d.body, ConfirmLabel: "Revert", Return: &w.focusReturn, OnConfirm: func() {
			w.dialog = nil
			go w.actionDialog("Could not revert the document", func() error { return w.host.Revert(w.ctx, d.id) }, portrait.ToastSpec{ID: "restore:" + d.id, Tone: portrait.Tones.Success, Title: "Restored"})
		}})
	case "moveReplace":
		portrait.Confirm(c, &d.open, portrait.ConfirmSpec{Title: d.title, Body: d.body, ConfirmLabel: "Replace", Return: &w.focusReturn, OnConfirm: func() {
			w.dialog = nil
			go w.actionDialog("Could not move the document", func() error { return w.host.MoveToFolder(w.ctx, d.id, d.dir, true) }, portrait.ToastSpec{})
		}})
	case "move":
		w.drawMove(c, d)
	case "alert":
		portrait.DialogShell(c, "portrait alert", &d.open, portrait.DialogSpec{Label: d.title, Width: portrait.Form.DialogWidth, Role: ui.RoleAlertDialog, Glass: true}, func() {
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

// drawMove is the Portrait folder picker: every folder of the workspace, with
// the document's own folder marked and one primary Move action.
func (w *Window) drawMove(c *ui.Context, d *windowDialog) {
	current := filepath.Dir(w.documentInfo(d.id).Path)
	portrait.DialogShell(c, "move document", &d.open, portrait.DialogSpec{Label: d.title, Width: portrait.Form.DialogWidth, Glass: true}, func() {
		portrait.Type.Title.Apply(ui.Text(c, d.title)).FontWeight(600).TextColor(portrait.Colors.Ink).Wrap()
		portrait.Type.Small.Apply(ui.Text(c, d.body)).TextColor(portrait.Colors.Ink2).Wrap()
		ui.Scroll(c).MaxHeight(portrait.Form.ListMax).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(portrait.Form.PopoverGap).Children(func() {
				row := func(key, path, name string, depth int) {
					spec := portrait.ButtonSpec{Look: portrait.Quiet, Leading: portrait.IconFolder, Selected: d.dir == path}
					if moveSameFile(path, current) {
						spec.Trailing = portrait.IconCheck
					}
					b := portrait.ButtonWith(c.Key(key), name, "Move to "+path, spec).FillWidth().Justify(ui.Start)
					if depth > 0 {
						b.Margin(0, 0, 0, float32(depth)*portrait.Form.PadX)
					}
					if b.Clicked() {
						d.dir = path
					}
				}
				if w.snapshot.Root != "" {
					row("move-root", w.snapshot.Root, w.snapshot.Name, 0)
				}
				for _, entry := range d.folders {
					row(entry.Path, entry.Path, entry.Name, entry.Depth)
				}
			})
		})
		ui.Row(c).Justify(ui.End).Gap(portrait.Form.DialogGap).Children(func() {
			if portrait.QuietButton(c, "Cancel", "Cancel move").Clicked() {
				d.open = false
			}
			if portrait.QuietButton(c, "Choose another folder…", "Choose another folder").Clicked() {
				go func() {
					directory, err := w.host.ChooseFolder(w.ctx)
					w.win.Update(func() {
						if err == nil {
							w.chooseMove(d, directory)
						} else if !errors.Is(err, mygo.ErrCanceled) {
							w.ShowAlert("Could not choose a folder", err.Error())
						}
					})
				}()
			}
			if portrait.AccentButton(c, "Move", "Move to the chosen folder").Disabled(d.dir == "" || moveSameFile(d.dir, current)).Clicked() {
				w.chooseMove(d, d.dir)
			}
		})
	}).Padding(portrait.Form.DialogPad).Gap(portrait.Form.DialogGap)
}

func infoIDs(docs []workspace.DocumentInfo) []string {
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}
