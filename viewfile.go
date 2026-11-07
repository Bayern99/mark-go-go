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

// maxViewBytes bounds what the host reads of a view file: a view is a few
// kilobytes of components, so anything larger is not one.
const maxViewBytes = 1 << 20

// ViewSnapshot is a document's view file, as the host read it. Exists is
// false when there is none, including for an untitled document; Error says
// why an existing file could not be read. The page validates the text.
// CanUndo says the view before the last generation is kept, so that
// UndoView can go back to it.
type ViewSnapshot struct {
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

// ViewEvent is a view snapshot scoped to a document identity.
type ViewEvent struct {
	ID      string `json:"id"`
	Exists  bool   `json:"exists"`
	Text    string `json:"text"`
	Error   string `json:"error"`
	CanUndo bool   `json:"canUndo"`
}

// viewChanged tells a page that follows a document's view file that the
// file changed, appeared or went away.
var viewChanged = mygo.NewEvent[ViewEvent]("viewChanged")

// viewPath is the view file of the document at docPath: a hidden sibling,
// .<name>.genui.json, so that it moves with the document's folder. An
// untitled document has none.
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

// readViewText reads the view file at path, at most maxViewBytes of it.
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

// WatchView makes the page follow the document's view file.
// viewChanged carries every snapshot, in the order the host read them:
// one as soon as follow starts, one after the watch is running, then
// each change, and when the document is renamed, moved or first saved.
// The first event does not wait for WatchFiles to return. Calling it
// again restarts the watch. A snapshot already sent is not withdrawn; a
// watch that cannot start is still returned on this call, so the page
// does not treat the follow as live. documentId is required; the follow
// does not retarget through the active tab.
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

// UnwatchView stops following the document's view file.
func (h *documentHost) UnwatchView(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.unfollowView()
	return nil
}

// UndoView puts back the view the last generation replaced, once: the
// kept view becomes the view file and nothing is kept any more. The
// page sees it through the view watch.
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

// viewMoved retargets the watch when the document's path changed, and
// tells the page what the view file at the new path is. It runs on the
// main thread, so followView starts the watch on a goroutine of its own.
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

// followView sends the view as it is now, starts watching it, then sends
// it again. The first event is not delayed by WatchFiles; the second
// closes the gap between that read and the watch's baseline, so a write,
// replacement or delete in between is not lost. An older generation
// sends nothing and does not start a watch.
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

// retargetLocked stops the session's watch and makes path the file
// watched, under a new generation that startViewWatch and emitView check:
// work of an older generation stops and sends nothing. h.mu must be held.
func (s *docSession) retargetLocked(path string) int {
	if s.stopView != nil {
		s.stopView()
		s.stopView = nil
	}
	s.viewGen++
	s.viewPath = path
	return s.viewGen
}

// currentLocked says whether generation gen is still the one the page
// follows. h.mu must be held.
func (s *docSession) currentLocked(gen int) bool {
	return s.following && s.viewGen == gen
}

// startViewWatch watches the directory of the view file at path for
// generation gen. Ignore keeps the scan off every other name in that
// folder; WatchFiles still enrolls the folder itself. When the watch
// cannot start, the session forgets path, so that the next state change
// tries again.
func (s *docSession) startViewWatch(path string, gen int) error {
	if path == "" {
		return nil
	}
	h := s.host
	// The kept view, there to go back to, comes and goes with the view.
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
			// A batch, or the end of the watch because the folder went
			// away: either way the page gets the file as it is now.
			if !s.emitView(path, gen) || err != nil {
				return
			}
		}
	}()
	return nil
}

// emitView sends the page the view file at path, unless generation gen
// ended, which also cancels its watch; it says whether it sent it.
// emitMu orders the read with the send, so that a snapshot read earlier
// never follows a later one, and the check and the send hold h.mu, so
// that nothing of an older generation follows a newer one. Emit only
// queues the message.
func (s *docSession) emitView(path string, gen int) bool {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	snap := readView(path)
	h := s.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if !s.currentLocked(gen) {
		return false
	}
	if s.emit != nil {
		s.emit(snap)
	} else {
		_ = viewChanged.Emit(s.win, ViewEvent{
			ID:      s.id,
			Exists:  snap.Exists,
			Text:    snap.Text,
			Error:   snap.Error,
			CanUndo: snap.CanUndo,
		})
	}
	return true
}
