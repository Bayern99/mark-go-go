package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

var (
	ErrSaveConflict          = errors.New("markgogo: the file changed on disk")
	ErrSaveReadOnly          = errors.New("markgogo: the file is read only")
	ErrMoveNeedsConfirmation = errors.New("markgogo: the move destination already exists")
	ErrMoveDestinationOpened = errors.New("markgogo: the move destination is open in another tab")
	ErrDocumentClosed        = errors.New("markgogo: the document is no longer open")
)

// markdownExtension is the extension a document's kind saves an untitled
// document as. Mark Go Go has one document kind, whose first extension is
// the one MyGo appends for a new document.
const markdownExtension = ".md"

// ClosePlan is what a close preflight found. Documents in Silent save
// themselves in place, so the sheet never asks about a change autosave has
// already written. Documents in Prompt cannot decide alone and need the
// user's Save / Don't Save / Cancel choice.
type ClosePlan struct {
	Silent, Prompt []DocumentInfo
}

// NeedsDecision reports whether the plan needs the Portrait decision sheet.
func (p ClosePlan) NeedsDecision() bool { return len(p.Prompt) > 0 }

// Documents returns the plan's documents, the asked ones first.
func (p ClosePlan) Documents() []DocumentInfo {
	out := make([]DocumentInfo, 0, len(p.Prompt)+len(p.Silent))
	out = append(out, p.Prompt...)
	return append(out, p.Silent...)
}

// CloseSelection is the answer a close plan received: which documents save,
// which are discarded, and which save in place without being asked.
type CloseSelection struct {
	Silent, Save, Discard []string
}

// SelectClose answers a plan with keep[i] telling, for each document of
// plan.Prompt in order, whether it should be saved. save is false for Don't
// Save, which discards every asked document.
func SelectClose(plan ClosePlan, save bool, keep []bool) CloseSelection {
	sel := CloseSelection{Silent: infoIDs(plan.Silent)}
	for i, doc := range plan.Prompt {
		if save && (i >= len(keep) || keep[i]) {
			sel.Save = append(sel.Save, doc.ID)
			continue
		}
		sel.Discard = append(sel.Discard, doc.ID)
	}
	return sel
}

func infoIDs(docs []DocumentInfo) []string {
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

// SetAutosave records how the app's documents save, so a close asks only
// about the documents that cannot save themselves.
func (h *Service) SetAutosave(a mygo.Autosave) {
	h.mu.Lock()
	h.autosave = a
	h.mu.Unlock()
}

// AutosavesInPlace reports whether ordinary documents write themselves.
func (h *Service) AutosavesInPlace() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.autosave == mygo.AutosaveInPlace
}

// CloseTargetIDs returns every document that closing kind targets, taken
// from the workspace service rather than a window's editor cache: a tab the
// window never activated still closes, so it must still be preflighted.
// workspaceID names the root for a root close and the document for a single
// document close.
func (h *Service) CloseTargetIDs(ctx context.Context, kind, workspaceID string) ([]string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch kind {
	case "closeRoot":
		w := h.workspaceByIDLocked(workspaceID)
		if w == nil {
			return nil, errNoWorkspace
		}
		return slices.Clone(w.tabs), nil
	case "closeDocument":
		if workspaceID == "" {
			return nil, nil
		}
		return []string{workspaceID}, nil
	case "closeWorkspace", "":
		win := desktop.Window(ctx)
		if win == nil {
			return nil, errNoWorkspace
		}
		return h.windowTabsLocked(win), nil
	}
	return nil, nil
}

// windowTabsLocked returns the tabs of every workspace the window hosts, in
// the order the session keeps them.
func (h *Service) windowTabsLocked(win *mygo.Window) []string {
	seenWorkspace, seenDoc := map[string]bool{}, map[string]bool{}
	var ids []string
	collect := func(w *workspace) {
		if w == nil || w.id == "" || seenWorkspace[w.id] {
			return
		}
		seenWorkspace[w.id] = true
		for _, id := range w.tabs {
			if id == "" || seenDoc[id] {
				continue
			}
			seenDoc[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range h.workspaceOrder {
		if w := h.workspaceByIDLocked(id); w != nil && w.win == win {
			collect(w)
		}
	}
	for _, w := range h.byWindow {
		if w != nil && w.win == win {
			collect(w)
		}
	}
	for _, w := range h.byWorkspace {
		if w != nil && w.win == win {
			collect(w)
		}
	}
	return ids
}

// ClosePreflight classifies every edited document among ids. It never saves,
// discards or changes document state.
func (h *Service) ClosePreflight(ctx context.Context, ids []string) (ClosePlan, error) {
	var plan ClosePlan
	if len(ids) == 0 {
		w, err := h.workspaceOf(ctx)
		if err != nil {
			return plan, err
		}
		h.mu.Lock()
		ids = slices.Clone(w.tabs)
		h.mu.Unlock()
	}
	for _, id := range ids {
		s, err := h.docSession(id)
		if err != nil {
			// A tab without a session has no edits to lose.
			continue
		}
		if !s.documentState().Edited {
			continue
		}
		info := h.closeInfo(s)
		if h.autosavesInPlace(info) && !h.changedOnDisk(s) && !h.hasSaveError(s) {
			plan.Silent = append(plan.Silent, info)
			continue
		}
		plan.Prompt = append(plan.Prompt, info)
	}
	return plan, nil
}

// autosavesInPlace reports whether a document writes its own changes before
// a close: only then can the sheet pass over it, because MyGo's autosave
// timer has already put the draft on disk or will before the tab closes.
// closeInfo is a document's state with what its file says, for a session
// whose document has not been opened yet.
func (h *Service) closeInfo(s *docSession) DocumentInfo {
	info := s.info()
	readOnly, deleted := h.fileState(s)
	if readOnly {
		info.ReadOnly = true
	}
	if deleted {
		info.Deleted = true
	}
	return info
}

func (h *Service) noteSaveError(s *docSession, err error) {
	if s == nil {
		return
	}
	msg := ""
	if err != nil && !errors.Is(err, mygo.ErrCanceled) {
		msg = actionMessage(err)
	}
	h.mu.Lock()
	s.saveError = msg
	h.mu.Unlock()
}

func (h *Service) hasSaveError(s *docSession) bool {
	if s == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return s.saveError != ""
}

// fileState reads the read-only and deleted state from the session's file,
// including live documents whose file disappeared before the watcher fired.
func (h *Service) fileState(s *docSession) (readOnly, deleted bool) {
	if s == nil {
		return false, false
	}
	path := s.currentPath()
	if path == "" {
		return false, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, true
	}
	if s.doc != nil {
		return s.doc.IsReadOnly(), false
	}
	return info.Mode().Perm()&0o200 == 0, false
}

func (h *Service) autosavesInPlace(info DocumentInfo) bool {
	return h.AutosavesInPlace() && info.Path != "" && !info.ReadOnly && !info.Conflict && !info.Deleted
}

// ResolveClose performs every fallible save of a close and reports the
// documents that still need a decision. It discards nothing: a canceled
// Save As panel or a failed save leaves every document edited.
func (h *Service) ResolveClose(ctx context.Context, sel CloseSelection) ([]DocumentInfo, error) {
	var unsaved []DocumentInfo
	for _, id := range append(slices.Clone(sel.Silent), sel.Save...) {
		if err := h.SaveForClose(ctx, id); err != nil {
			if errors.Is(err, mygo.ErrCanceled) {
				return unsaved, err
			}
			unsaved = append(unsaved, h.failedInfo(id, err))
		}
	}
	return unsaved, nil
}

// failedInfo describes a document whose save failed, with the reason and the
// conflict or read-only flag so the window promotes it to the matching sheet.
func (h *Service) failedInfo(id string, err error) DocumentInfo {
	info := DocumentInfo{ID: id}
	if s, docErr := h.docSession(id); docErr == nil {
		info = h.closeInfo(s)
	}
	if errors.Is(err, ErrSaveConflict) {
		info.Conflict = true
	}
	if errors.Is(err, ErrSaveReadOnly) {
		info.ReadOnly = true
	}
	if info.ActionError == "" {
		info.ActionError = actionMessage(err)
	}
	return info
}

// CommitClose marks the discarded documents clean. Call it only after every
// save of ResolveClose succeeded: until then the close may still abort, and
// a clean flag on an unwritten document would lose the next close's prompt.
func (h *Service) CommitClose(ctx context.Context, sel CloseSelection) error {
	for _, id := range sel.Discard {
		s, err := h.docSession(id)
		if err != nil {
			return err
		}
		if s.doc != nil {
			s.doc.SetEdited(false)
		} else {
			h.mu.Lock()
			s.edited = false
			h.mu.Unlock()
		}
		h.noteSaveError(s, nil)
		h.noteAction(s, nil)
	}
	return nil
}

// RestoreDirty re-marks open documents as edited if the close that followed
// CommitClose was canceled or refused, so an unwritten draft never stays clean.
func (h *Service) RestoreDirty(ctx context.Context, ids []string) {
	for _, id := range ids {
		s, err := h.docSession(id)
		if err != nil || s.closed {
			continue
		}
		if s.doc != nil {
			s.doc.SetEdited(true)
		} else {
			h.mu.Lock()
			s.edited = true
			h.mu.Unlock()
		}
	}
}

// DiscardDocument resolves a close decision without writing to disk.
func (h *Service) DiscardDocument(ctx context.Context, documentID string) error {
	return h.CommitClose(ctx, CloseSelection{Discard: []string{documentID}})
}

// SaveAnyway performs the non-interactive overwrite chosen by the user.
func (h *Service) SaveAnyway(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.SaveAnyway(); err != nil {
		h.noteSaveError(s, err)
		h.noteAction(s, err)
		return err
	}
	h.recordDisk(s)
	h.migrateIfNeeded(s)
	h.noteSaveError(s, nil)
	h.noteAction(s, nil)
	return nil
}

// SaveForClose saves an ordinary document in place and uses the OS Save As
// panel only for an untitled or read-only document. It rechecks the disk
// first so closing a document another app changed never overwrites it
// silently; the caller shows the Portrait decision for the sentinel's id.
func (h *Service) SaveForClose(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	state := h.closeInfo(s)
	if state.Path == "" {
		err = h.saveAsDocument(s)
		h.noteSaveError(s, err)
		h.noteAction(s, err)
		return err
	}
	if state.ReadOnly || s.doc != nil && s.doc.IsReadOnly() {
		return ErrSaveReadOnly
	}
	if state.Conflict || state.Deleted || h.changedOnDisk(s) || s.doc != nil && s.doc.HasConflict() {
		return ErrSaveConflict
	}
	if s.doc == nil {
		return ErrDocumentClosed
	}
	return h.SaveAnyway(ctx, documentID)
}

// RenameTo performs the already validated, non-interactive rename.
func (h *Service) RenameTo(ctx context.Context, documentID, name string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if err := ValidateDocumentName(name); err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.Rename(name); err != nil {
		h.noteAction(s, err)
		return err
	}
	h.recordDisk(s)
	h.migrateIfNeeded(s)
	h.noteAction(s, nil)
	return nil
}

func ValidateDocumentName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("enter a file name")
	case name == "." || name == ".." || strings.ContainsAny(name, `/\\`):
		return fmt.Errorf("the name cannot contain a path separator")
	case !fsutil.MarkdownName(name):
		return fmt.Errorf("use an .md or .markdown extension")
	}
	return nil
}

// MoveTarget reports the file a move writes into directory: a saved
// document keeps its name, and an untitled one takes its name with the
// Markdown extension MyGo appends.
func MoveTarget(doc DocumentInfo, directory string) string {
	if directory == "" {
		return ""
	}
	name := doc.Name
	if doc.Path != "" {
		return filepath.Join(directory, filepath.Base(doc.Path))
	}
	if name == "" {
		return ""
	}
	return filepath.Join(directory, name+markdownExtension)
}

// MoveToFolder performs a chosen move. A displaced destination is renamed to
// a temporary sibling first, so a move that fails in any way — a reservation,
// a permission, a cross-device copy — puts both files back instead of
// destroying the destination.
func (h *Service) MoveToFolder(ctx context.Context, documentID, directory string, replace bool) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if directory == "" {
		return fmt.Errorf("markgogo: choose a folder to move the document into")
	}
	info := s.info()
	target := MoveTarget(info, directory)
	if target == "" {
		return fmt.Errorf("markgogo: the document has no name to move")
	}
	if fsutil.SameFile(target, info.Path) {
		return nil
	}
	displaced := ""
	if existing, statErr := os.Lstat(target); statErr == nil {
		if existing.IsDir() {
			return fmt.Errorf("markgogo: move destination is a folder")
		}
		if !replace {
			return ErrMoveNeedsConfirmation
		}
		if err := h.rejectOpenPath(documentID, target); err != nil {
			return err
		}
		if displaced, err = displaceFile(target); err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := s.doc.MoveTo(directory); err != nil {
		if restoreErr := restoreFile(displaced, target, info.Path == "" || fsutil.SameFile(s.currentPath(), info.Path)); restoreErr != nil {
			err = fmt.Errorf("%w (original destination preserved at %s: %v)", err, displaced, restoreErr)
		}
		h.noteAction(s, err)
		return err
	}
	dropFile(displaced)
	h.recordDisk(s)
	h.migrateIfNeeded(s)
	h.noteAction(s, nil)
	return nil
}

// rejectOpenPath refuses a destination that another tab already has, before
// anything on disk moves.
func (h *Service) rejectOpenPath(documentID, target string) error {
	canon := fsutil.CanonicalPath(target)
	h.mu.Lock()
	defer h.mu.Unlock()
	if owner := h.claims[canon]; owner != "" && owner != documentID {
		return ErrMoveDestinationOpened
	}
	for id, sess := range h.byID {
		if id == documentID || sess == nil {
			continue
		}
		if fsutil.SameFile(sess.currentPath(), target) {
			return ErrMoveDestinationOpened
		}
	}
	return nil
}

// displaceFile renames path to an empty temporary sibling in the same folder
// and returns that sibling, so the caller can restore it or drop it once the
// replacement commits.
func displaceFile(path string) (string, error) {
	temp, err := os.CreateTemp(filepath.Dir(path), ".markgogo-replace-*")
	if err != nil {
		return "", err
	}
	name := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Rename(path, name); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// restoreFile puts a displaced destination back after a failed move. When a
// failed cross-volume copy left a partial file at destination while the
// source stayed intact, clear the partial copy first so the original returns.
func restoreFile(displaced, destination string, sourceIntact bool) error {
	if displaced == "" {
		return nil
	}
	if _, err := os.Lstat(destination); err == nil {
		if !sourceIntact {
			return fmt.Errorf("destination %s is occupied", destination)
		}
		if err := os.Remove(destination); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(displaced, destination)
}

// dropFile removes a displaced destination once the replacement committed.
func dropFile(displaced string) {
	if displaced != "" {
		_ = os.Remove(displaced)
	}
}

// ChooseFolder is the OS-only escape hatch for a destination outside the
// workspace's Portrait folder list.
func (h *Service) ChooseFolder(ctx context.Context) (string, error) {
	w, _ := h.workspaceOf(ctx)
	var parent *mygo.Window
	if w != nil {
		parent = w.win
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Move to Folder", ButtonLabel: "Move", Directory: true, CreateDirectories: true})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", mygo.ErrCanceled
	}
	return paths[0], nil
}

// OpenFiles uses the OS picker but routes all subsequent errors to the caller
// instead of MyGo's document-error NSAlert. It also works with zero windows.
func (h *Service) OpenFiles(ctx context.Context) error {
	var parent *mygo.Window
	if w, err := h.workspaceOf(ctx); err == nil && w != nil {
		parent = w.win
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Open Markdown", Filters: []mygo.FileFilter{{Name: "Markdown Document", Extensions: []string{"md", "markdown"}}}, Multiple: true})
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return mygo.ErrCanceled
	}
	for _, path := range paths {
		if _, err := h.Open(ctx, path); err != nil {
			return err
		}
	}
	return nil
}
