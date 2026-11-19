package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

const maxViewBytes = 1 << 20

type ViewSnapshot struct {
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

type ViewEvent struct {
	ID      string `json:"id"`
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

var viewChanged = mygo.NewEvent[ViewEvent]("viewChanged")

func viewPath(docPath string) string {
	if docPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(docPath), "."+filepath.Base(docPath)+".genui.json")
}

func readView(path string) ViewSnapshot {
	if path == "" {
		return ViewSnapshot{}
	}
	snap := readViewText(path)
	if fi, err := os.Lstat(prevViewPath(path)); err == nil && fi.Mode().IsRegular() {
		snap.CanUndo = true
	}
	return snap
}

func readViewText(path string) ViewSnapshot {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ViewSnapshot{}
	}
	if err != nil {
		return ViewSnapshot{Exists: true, Error: err.Error()}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxViewBytes+1))
	if err != nil {
		return ViewSnapshot{Exists: true, Error: err.Error()}
	}
	if len(data) > maxViewBytes {
		return ViewSnapshot{Exists: true, Error: "the view file is larger than 1 MiB"}
	}
	return ViewSnapshot{Exists: true, Text: string(data)}
}

func (h *documentHost) WatchView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	h.mu.Lock()
	s.following = true
	path := viewPath(s.doc.State().Path)
	gen := s.retargetLocked(path)
	h.mu.Unlock()
	return s.followView(path, gen)
}

func (h *documentHost) UnwatchView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.unfollowView()
	return nil
}

func (h *documentHost) UndoView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	path := viewPath(s.doc.State().Path)
	if path == "" {
		return errors.New("There is no earlier view to go back to.")
	}
	return undoView(path)
}

func (s *docSession) unfollowView() {
	h := s.host
	h.mu.Lock()
	s.following = false
	s.retargetLocked("")
	h.mu.Unlock()
}

func (s *docSession) viewMoved(docPath string) {
	path := viewPath(docPath)
	h := s.host
	h.mu.Lock()
	if !s.following || path == s.viewPath {
		h.mu.Unlock()
		return
	}
	gen := s.retargetLocked(path)
	h.mu.Unlock()
	go func() {
		if err := s.followView(path, gen); err != nil {
			log.Print(err)
		}
	}()
}

func (s *docSession) followView(path string, gen int) error {
	return s.followViewWith(path, gen, s.startViewWatch)
}

func (s *docSession) followViewWith(path string, gen int, start func(string, int) error) error {
	if !s.emitView(path, gen) {
		return nil
	}
	if err := start(path, gen); err != nil {
		return err
	}
	s.emitView(path, gen)
	return nil
}

func (s *docSession) retargetLocked(path string) int {
	if s.stopView != nil {
		s.stopView()
		s.stopView = nil
	}
	s.viewGen++
	s.viewPath = path
	return s.viewGen
}

func (s *docSession) currentLocked(gen int) bool {
	return s.following && s.viewGen == gen
}

func (s *docSession) startViewWatch(path string, gen int) error {
	if path == "" {
		return nil
	}
	h := s.host
	name, prev := filepath.Base(path), filepath.Base(prevViewPath(path))
	ctx, cancel := context.WithCancel(context.Background())
	w, err := mygo.WatchFiles(ctx, filepath.Dir(path), mygo.FileWatchOptions{
		Ignore: func(p string, _ bool) bool { return p != name && p != prev },
	})
	if err != nil {
		cancel()
		h.mu.Lock()
		if s.currentLocked(gen) {
			s.viewPath = ""
		}
		h.mu.Unlock()
		return fmt.Errorf("markgogo: follow the view file: %w", err)
	}

	h.mu.Lock()
	if !s.currentLocked(gen) {
		h.mu.Unlock()
		cancel()
		return nil
	}
	s.stopView = cancel
	h.mu.Unlock()

	go func() {
		for {
			_, err := w.Next(ctx)
			if !s.emitView(path, gen) || err != nil {
				return
			}
		}
	}()
	return nil
}

func (s *docSession) emitView(path string, gen int) bool {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	snap := readView(path)
	h := s.host
	h.mu.Lock()
	if !s.currentLocked(gen) {
		h.mu.Unlock()
		return false
	}
	win, id, emit := s.win, s.id, s.emit
	h.mu.Unlock()
	if emit != nil {
		emit(snap)
	} else if win != nil && !win.IsDestroyed() {
		_ = viewChanged.Emit(win, ViewEvent{
			ID:      id,
			Exists:  snap.Exists,
			Text:    snap.Text,
			Error:   snap.Error,
			CanUndo: snap.CanUndo,
		})
	}
	return true
}
