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
type ViewSnapshot struct {
	Exists bool   `json:"exists"`
	Text   string `json:"text"`
	Error  string `json:"error"`
}

// viewChanged tells a page that follows its document's view file that the
// file changed, appeared or went away.
var viewChanged = mygo.NewEvent[ViewSnapshot]("viewChanged")

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

// WatchView makes the calling page follow its document's view file: the
// host sends viewChanged with the file as it is once the watch runs, then
// whenever the file changes, and when the document is renamed, moved or
// first saved. Every snapshot comes as that event, in the order the host
// read them. Calling it again restarts the watch.
func (h *documentHost) WatchView(ctx context.Context) error {
	s, err := h.session(ctx)
	if err != nil {
		return err
	}
	h.mu.Lock()
	s.following = true
	// The path is read under h.mu, so that a move either comes before it
	// or finds the page following and retargets. Taking the document's
	// lock inside h.mu is safe: MyGo calls the session's callbacks
	// without it.
	path := viewPath(s.doc.State().Path)
	gen := s.retargetLocked(path)
	h.mu.Unlock()
	if err := s.startViewWatch(path, gen); err != nil {
		return err
	}
	s.emitView(path, gen)
	return nil
}

// UnwatchView stops following the calling page's view file.
func (h *documentHost) UnwatchView(ctx context.Context) error {
	s, err := h.session(ctx)
	if err != nil {
		return err
	}
	s.unfollowView()
	return nil
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
// main thread, so the watch starts on a goroutine of its own.
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
		if err := s.startViewWatch(path, gen); err != nil {
			log.Print(err)
		}
		s.emitView(path, gen)
	}()
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
// generation gen. When the watch cannot start, the session forgets path,
// so that the next state change tries again.
func (s *docSession) startViewWatch(path string, gen int) error {
	if path == "" {
		return nil
	}
	h := s.host
	name := filepath.Base(path)
	ctx, cancel := context.WithCancel(context.Background())
	w, err := mygo.WatchFiles(ctx, filepath.Dir(path), mygo.FileWatchOptions{
		Ignore: func(p string, _ bool) bool { return p != name },
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
	_ = viewChanged.Emit(s.win, snap)
	return true
}
