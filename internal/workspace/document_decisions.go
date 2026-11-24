package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

var (
	ErrSaveConflict          = errors.New("markgogo: the file changed on disk")
	ErrSaveReadOnly          = errors.New("markgogo: the file is read only")
	ErrMoveNeedsConfirmation = errors.New("markgogo: the move destination already exists")
)

// ClosePreflight returns every edited document that must receive an explicit
// Save / Don't Save decision. It never saves or changes document state.
func (h *Service) ClosePreflight(ctx context.Context, ids []string) ([]DocumentInfo, error) {
	if len(ids) == 0 {
		w, err := h.workspaceOf(ctx)
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		ids = append([]string(nil), w.tabs...)
		h.mu.Unlock()
	}
	var edited []DocumentInfo
	for _, id := range ids {
		s, err := h.docSession(id)
		if err != nil {
			return nil, err
		}
		stateEdited := s.edited
		if s.doc != nil {
			stateEdited = s.doc.State().Edited
		}
		if stateEdited {
			edited = append(edited, s.info())
		}
	}
	return edited, nil
}

// DiscardDocument resolves a close decision without writing to disk.
func (h *Service) DiscardDocument(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.doc.SetEdited(false)
	h.noteAction(s, nil)
	return nil
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
		h.noteAction(s, err)
		return err
	}
	h.migrateIfNeeded(s)
	h.noteAction(s, nil)
	return nil
}

// SaveForClose saves an ordinary document in place and uses the OS Save As
// panel only for an untitled or read-only document.
func (h *Service) SaveForClose(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	state := s.doc.State()
	if state.Path == "" || s.doc.IsReadOnly() {
		err = h.saveAsDocument(s)
	} else {
		err = h.SaveAnyway(ctx, documentID)
	}
	h.noteAction(s, err)
	return err
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

// MoveDestination reports the target a move would replace, if any.
func MoveDestination(documentPath, directory string) string {
	if documentPath == "" || directory == "" {
		return ""
	}
	return filepath.Join(directory, filepath.Base(documentPath))
}

// MoveToFolder performs a chosen move. An existing target is removed only
// after replace is true, so MyGo's own replace alert can never appear.
func (h *Service) MoveToFolder(ctx context.Context, documentID, directory string, replace bool) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	target := MoveDestination(s.doc.Path(), directory)
	if target != "" && target != s.doc.Path() {
		if info, statErr := os.Lstat(target); statErr == nil {
			if !replace {
				return ErrMoveNeedsConfirmation
			}
			if info.IsDir() {
				return fmt.Errorf("markgogo: move destination is a folder")
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
	}
	if err := s.doc.MoveTo(directory); err != nil {
		h.noteAction(s, err)
		return err
	}
	h.migrateIfNeeded(s)
	h.noteAction(s, nil)
	return nil
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
// instead of MyGo's document-error NSAlert.
func (h *Service) OpenFiles(ctx context.Context) error {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return err
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: w.win, Title: "Open Markdown", Filters: []mygo.FileFilter{{Name: "Markdown Document", Extensions: []string{"md", "markdown"}}}, Multiple: true})
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
