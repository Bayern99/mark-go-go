package main

import (
	"context"
	"errors"
	"sync"

	"github.com/egoist/mygo"
)

// DocState is the state of a window's document, as the page sees it.
type DocState struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Edited   bool   `json:"edited"`
	Conflict bool   `json:"conflict"`
	Deleted  bool   `json:"deleted"`
	ReadOnly bool   `json:"readOnly"`
}

// Snapshot is the whole document a page edits at one version. Version
// starts at 1 when the document opens and grows by one every time its
// content is read from disk again.
type Snapshot struct {
	Text    string   `json:"text"`
	Version int      `json:"version"`
	State   DocState `json:"state"`
}

// docReloaded tells a document's page that MyGo read the file again: the
// page loads the snapshot it carries.
var docReloaded = mygo.NewEvent[Snapshot]("docReloaded")

// docState tells a document's page that its state changed: edited or
// saved, renamed or moved, in conflict, or its file deleted.
var docState = mygo.NewEvent[DocState]("docState")

// documentHost is the bound service: it gives the page of a document's
// window that document, and takes its edits, through MyGo's document as
// the only authority.
type documentHost struct {
	mu       sync.Mutex
	byWindow map[*mygo.Window]*docSession
}

// docSession ties a document to its window and its page's events.
type docSession struct {
	host *documentHost
	doc  *mygo.Document[string]
	win  *mygo.Window

	// version is guarded by documentHost.mu.
	version   int
	offReload func()
	offState  func()

	// The view file the page follows (viewfile.go), guarded by
	// documentHost.mu: following says the page asked to, viewPath is the
	// file watched, stopView ends its watch, and viewGen counts
	// retargets. emitMu, taken before documentHost.mu, orders the reads
	// of the file with the events that carry them.
	following bool
	viewPath  string
	stopView  func()
	viewGen   int
	emitMu    sync.Mutex
}

func newDocumentHost() *documentHost {
	return &documentHost{byWindow: make(map[*mygo.Window]*docSession)}
}

// openWindow makes the web page window of doc. MyGo calls it on the main
// thread for every document, including the ones it recovers at startup.
func (h *documentHost) openWindow(doc *mygo.Document[string]) *mygo.Window {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     doc.Name(),
		URL:       "/",
		Width:     940,
		Height:    720,
		MinWidth:  480,
		MinHeight: 360,
	})
	h.track(doc, win)
	return win
}

// track subscribes the page of win to the document's reloads and state
// changes, and forgets them when the window closes.
func (h *documentHost) track(doc *mygo.Document[string], win *mygo.Window) {
	s := &docSession{host: h, doc: doc, win: win, version: 1}
	h.mu.Lock()
	h.byWindow[win] = s
	h.mu.Unlock()

	// Both callbacks run on the main thread after MyGo updated the
	// document, so the snapshot they carry is the new content.
	s.offReload = doc.OnReload(func() {
		h.mu.Lock()
		s.version++
		h.mu.Unlock()
		_ = docReloaded.Emit(win, s.snapshot())
	})
	s.offState = doc.OnStateChange(func(state mygo.DocumentState) {
		_ = docState.Emit(win, docStateOf(state))
		s.viewMoved(state.Path)
	})
	win.OnClosed(func() { h.forget(s) })
}

func (h *documentHost) forget(s *docSession) {
	if s.offReload != nil {
		s.offReload()
	}
	if s.offState != nil {
		s.offState()
	}
	s.unfollowView()
	h.mu.Lock()
	delete(h.byWindow, s.win)
	h.mu.Unlock()
}

// session returns the session of the calling page's window.
func (h *documentHost) session(ctx context.Context) (*docSession, error) {
	w := mygo.CallerWindow(ctx)
	h.mu.Lock()
	s := h.byWindow[w]
	h.mu.Unlock()
	if s == nil {
		return nil, errors.New("markgogo: this window shows no Markdown document")
	}
	return s, nil
}

// Load returns the content of the calling window's document, as it is
// now: the file's text, with the user's unsaved edits.
func (h *documentHost) Load(ctx context.Context) (Snapshot, error) {
	s, err := h.session(ctx)
	if err != nil {
		var zero Snapshot
		return zero, err
	}
	return s.snapshot(), nil
}

// Update replaces the content of the calling window's document with the
// full Markdown text the page composed. It edits the document, which
// MyGo autosaves; a document in conflict is still edited, so that saving
// asks which version to keep.
func (h *documentHost) Update(ctx context.Context, text string) error {
	s, err := h.session(ctx)
	if err != nil {
		return err
	}
	s.doc.Set(text)
	return nil
}

func (s *docSession) snapshot() Snapshot {
	// Read the version under the host's lock so that a reload's
	// increment and the content it announced agree.
	s.host.mu.Lock()
	version := s.version
	s.host.mu.Unlock()
	return Snapshot{
		Text:    s.doc.Content(),
		Version: version,
		State:   docStateOf(s.doc.State()),
	}
}

func docStateOf(s mygo.DocumentState) DocState {
	return DocState{
		Name:     s.Name,
		Path:     s.Path,
		Edited:   s.Edited,
		Conflict: s.Conflict,
		Deleted:  s.Deleted,
		ReadOnly: s.ReadOnly,
	}
}
