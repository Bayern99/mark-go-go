package main

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

// Snapshot is one document at one version. Version starts at 1 when the
// document opens and grows by one every time its content is read from
// disk again. ID is the stable document identity.
type Snapshot struct {
	ID      string       `json:"id"`
	Text    string       `json:"text"`
	Version int          `json:"version"`
	State   DocumentInfo `json:"state"`
}

// docReloaded tells the page that MyGo read the file again.
var docReloaded = mygo.NewEvent[Snapshot]("docReloaded")

// docState tells the page that a document's state changed, without
// activating it.
var docState = mygo.NewEvent[DocumentInfo]("docState")

// documentHost is the bound WorkspaceHost: workspace navigation and
// per-document commands keyed by document id, through MyGo's document
// as the only authority.
type documentHost struct {
	mu sync.Mutex

	byWindow map[*mygo.Window]*workspace
	byID     map[string]*docSession

	docs *mygo.Documents[string]

	settings *settingsStore
	runs     sync.WaitGroup

	// prepareJob, when set, adjusts a generation job after settings and
	// path are captured. Tests inject a fake agent through it.
	prepareJob func(*generationJob)
}

// docSession is one open document, permanently bound to its id.
type docSession struct {
	host *documentHost
	id   string
	doc  *mygo.Document[string]
	win  *mygo.Window
	ws   *workspace

	version    int
	offReload  func()
	offState   func()
	offClosed  func()
	generating bool
	// genSlot is this document's one generation; nil when idle. Concurrent
	// documents each have their own. cancel stops the agent; abandoned is
	// set on a real close so a finishing run cannot write a view.
	genSlot *generationSlot
	closed  bool
	// pathFn, when set, is this session's path for tests without a MyGo
	// document. Production sessions always have doc and leave this nil.
	pathFn func() string

	// The view file the page follows (viewfile.go), guarded by
	// documentHost.mu: following says the page asked to, viewPath is the
	// file watched, stopView ends its watch, and viewGen counts
	// retargets. emitMu, taken before documentHost.mu, orders the reads
	// of the file with the events that carry them. emit, when set,
	// receives those snapshots instead of the page (tests).
	following bool
	viewPath  string
	stopView  func()
	viewGen   int
	emitMu    sync.Mutex
	emit      func(ViewSnapshot)
}

// generationSlot is one in-flight generation of a document.
type generationSlot struct {
	cancel    context.CancelFunc
	path      string
	abandoned bool
}

func newDocumentHost() *documentHost {
	dir, _ := mygo.App.Path(mygo.PathUserData)
	return &documentHost{
		byWindow: make(map[*mygo.Window]*workspace),
		byID:     make(map[string]*docSession),
		settings: &settingsStore{dir: dir},
	}
}

func (s *docSession) info() DocumentInfo {
	st := s.documentState()
	generating := false
	if s.host != nil {
		s.host.mu.Lock()
		generating = s.generating
		s.host.mu.Unlock()
	}
	return s.makeInfo(st, generating)
}

// infoLocked is info while documentHost.mu is already held.
func (s *docSession) infoLocked() DocumentInfo {
	return s.makeInfo(s.documentState(), s.generating)
}

func (s *docSession) documentState() mygo.DocumentState {
	if s.doc != nil {
		return s.doc.State()
	}
	path := s.currentPath()
	name := filepath.Base(path)
	if name == "." || name == "/" || name == "" {
		name = "untitled.md"
	}
	return mygo.DocumentState{Name: name, Path: path}
}

func (s *docSession) currentPath() string {
	if s.pathFn != nil {
		return s.pathFn()
	}
	if s.doc != nil {
		return s.doc.Path()
	}
	return ""
}

func (s *docSession) makeInfo(st mygo.DocumentState, generating bool) DocumentInfo {
	return DocumentInfo{
		ID:         s.id,
		Name:       st.Name,
		Path:       st.Path,
		Edited:     st.Edited,
		Conflict:   st.Conflict,
		Deleted:    st.Deleted,
		ReadOnly:   st.ReadOnly,
		Generating: generating,
	}
}

func (s *docSession) snapshot() Snapshot {
	s.host.mu.Lock()
	version := s.version
	s.host.mu.Unlock()
	return Snapshot{
		ID:      s.id,
		Text:    s.doc.Content(),
		Version: version,
		State:   s.info(),
	}
}

// openWindow is DocumentsOptions.Window: reuse the workspace that should
// own this document, or make one. MyGo then attaches the document to it.
func (h *documentHost) openWindow(doc *mygo.Document[string]) *mygo.Window {
	if win := h.existingWindow(doc); win != nil {
		h.adopt(doc, win)
		return win
	}
	root := ""
	if p := doc.Path(); p != "" {
		root = filepath.Dir(p)
	}
	win := h.newWorkspaceWindow(root, doc.Name())
	h.adopt(doc, win)
	return win
}

func (h *documentHost) existingWindow(doc *mygo.Document[string]) *mygo.Window {
	path := doc.Path()
	h.mu.Lock()
	defer h.mu.Unlock()
	if path != "" {
		var roots []string
		byRoot := map[string]*workspace{}
		for _, w := range h.byWindow {
			if w.root == "" || w.win == nil || w.win.IsDestroyed() {
				continue
			}
			clean := filepath.Clean(w.root)
			roots = append(roots, w.root)
			byRoot[clean] = w
		}
		if best := deepestRoot(path, roots); best != "" {
			return byRoot[filepath.Clean(best)].win
		}
		return nil
	}
	// Untitled documents join the focused workspace when there is one,
	// else the existing untitled window, else a new untitled window.
	if fw := mygo.FocusedWindow(); fw != nil {
		if w := h.byWindow[fw]; w != nil && w.win != nil && !w.win.IsDestroyed() {
			return w.win
		}
	}
	for _, w := range h.byWindow {
		if w.root == "" && w.win != nil && !w.win.IsDestroyed() {
			return w.win
		}
	}
	return nil
}

func (h *documentHost) adopt(doc *mygo.Document[string], win *mygo.Window) {
	id := doc.ID()
	h.mu.Lock()
	w := h.byWindow[win]
	if w == nil {
		h.mu.Unlock()
		return
	}
	if _, exists := h.byID[id]; exists {
		h.mu.Unlock()
		return
	}
	s := &docSession{host: h, id: id, doc: doc, win: win, ws: w, version: 1}
	h.byID[id] = s
	found := false
	for _, tab := range w.tabs {
		if tab == id {
			found = true
			break
		}
	}
	if !found {
		w.tabs = append(w.tabs, id)
	}
	w.active = id
	h.mu.Unlock()

	s.offReload = doc.OnReload(func() {
		h.mu.Lock()
		s.version++
		h.mu.Unlock()
		_ = docReloaded.Emit(win, s.snapshot())
	})
	s.offState = doc.OnStateChange(func(state mygo.DocumentState) {
		_ = docState.Emit(win, s.info())
		s.viewMoved(state.Path)
		h.migrateIfNeeded(s)
		h.emitWorkspace(s.ws)
	})
	s.offClosed = doc.OnClosed(func() {
		h.dropSession(s)
	})
	if h.docs != nil {
		h.docs.SetActive(doc)
	}
	h.emitWorkspace(w)
}

func (h *documentHost) dropSession(s *docSession) {
	if s == nil {
		return
	}
	if s.offReload != nil {
		s.offReload()
		s.offReload = nil
	}
	if s.offState != nil {
		s.offState()
		s.offState = nil
	}
	if s.offClosed != nil {
		s.offClosed()
		s.offClosed = nil
	}
	h.mu.Lock()
	s.closed = true
	h.mu.Unlock()
	h.cancelGeneration(s)
	s.unfollowView()
	h.mu.Lock()
	delete(h.byID, s.id)
	w := s.ws
	if w != nil {
		tabs := w.tabs[:0]
		for _, id := range w.tabs {
			if id != s.id {
				tabs = append(tabs, id)
			}
		}
		w.tabs = tabs
		if w.active == s.id {
			if n := len(w.tabs); n > 0 {
				w.active = w.tabs[n-1]
			} else {
				w.active = ""
			}
		}
	}
	h.mu.Unlock()
	s.ws = nil
	if w != nil && w.root == "" && len(w.tabs) == 0 && w.win != nil && !w.win.IsDestroyed() {
		w.win.Destroy()
		return
	}
	h.emitWorkspace(w)
}

func (h *documentHost) migrateIfNeeded(s *docSession) {
	path := s.doc.Path()
	if path == "" {
		return
	}
	dest := h.workspaceForPath(path)
	h.mu.Lock()
	src := s.ws
	h.mu.Unlock()
	if dest == nil {
		parent := filepath.Dir(path)
		win := h.newWorkspaceWindow(parent, filepath.Base(parent))
		h.docs.Attach(s.doc, win)
		h.mu.Lock()
		dest = h.byWindow[win]
		h.mu.Unlock()
		h.moveTab(s, dest)
		return
	}
	if src == dest {
		return
	}
	h.docs.Attach(s.doc, dest.win)
	h.moveTab(s, dest)
}

func (h *documentHost) workspaceForPath(path string) *workspace {
	h.mu.Lock()
	defer h.mu.Unlock()
	var roots []string
	byRoot := map[string]*workspace{}
	for _, w := range h.byWindow {
		if w.root == "" || w.win == nil || w.win.IsDestroyed() {
			continue
		}
		roots = append(roots, w.root)
		byRoot[filepath.Clean(w.root)] = w
	}
	if best := deepestRoot(path, roots); best != "" {
		return byRoot[filepath.Clean(best)]
	}
	parent := filepath.Clean(filepath.Dir(path))
	return byRoot[parent]
}

func (h *documentHost) moveTab(s *docSession, dest *workspace) {
	if dest == nil {
		return
	}
	h.mu.Lock()
	src := s.ws
	if src == dest {
		h.mu.Unlock()
		return
	}
	if src != nil {
		tabs := src.tabs[:0]
		for _, id := range src.tabs {
			if id != s.id {
				tabs = append(tabs, id)
			}
		}
		src.tabs = tabs
		if src.active == s.id {
			if n := len(src.tabs); n > 0 {
				src.active = src.tabs[n-1]
			} else {
				src.active = ""
			}
		}
	}
	found := false
	for _, id := range dest.tabs {
		if id == s.id {
			found = true
			break
		}
	}
	if !found {
		dest.tabs = append(dest.tabs, s.id)
	}
	dest.active = s.id
	s.ws = dest
	s.win = dest.win
	h.mu.Unlock()
	h.docs.SetActive(s.doc)
	h.emitWorkspace(src)
	h.emitWorkspace(dest)
	if src != nil && src.root == "" && len(src.tabs) == 0 && src.win != nil && !src.win.IsDestroyed() {
		src.win.Destroy()
	}
}

func (h *documentHost) pathTaken(path, exceptID string) bool {
	if path == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.byID {
		if id == exceptID {
			continue
		}
		if sameFile(s.doc.Path(), path) {
			return true
		}
	}
	return false
}

// Load returns the content of the document, including unsaved edits.
func (h *documentHost) Load(ctx context.Context, documentID string) (Snapshot, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return Snapshot{}, err
	}
	return s.snapshot(), nil
}

// Update replaces the document's Markdown. MyGo autosaves; a document
// in conflict is still edited, so that saving asks which version to keep.
func (h *documentHost) Update(ctx context.Context, documentID, text string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.doc.Set(text)
	return nil
}

// Flush is the last-input delivery of that document, before Save or Close.
func (h *documentHost) Flush(ctx context.Context, documentID, text string) error {
	return h.Update(ctx, documentID, text)
}

func (h *documentHost) Save(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if s.doc.Path() == "" {
		return h.SaveAs(ctx, documentID)
	}
	before := s.doc.Path()
	err = s.doc.Save()
	if err != nil {
		return err
	}
	after := s.doc.Path()
	if after != "" && after != before && h.pathTaken(after, s.id) {
		return errAlreadyOpen
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) SaveAs(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	target, err := h.askSavePath(s)
	if err != nil {
		return err
	}
	if h.pathTaken(target, s.id) {
		return errAlreadyOpen
	}
	if err := s.doc.SaveAs(target); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) askSavePath(s *docSession) (string, error) {
	ext := "md"
	def := s.doc.Path()
	if def == "" {
		dir, _ := mygo.App.Path(mygo.PathDocuments)
		def = filepath.Join(dir, s.doc.Name()+"."+ext)
	}
	target, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      s.win,
		DefaultPath: def,
		Filters:     []mygo.FileFilter{{Name: "Markdown Document", Extensions: []string{"md", "markdown"}}},
	})
	if err != nil {
		return "", err
	}
	if target == "" {
		return "", mygo.ErrCanceled
	}
	if filepath.Ext(target) == "" {
		target += "." + ext
	}
	return target, nil
}

func (h *documentHost) Rename(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	before := s.doc.Path()
	if err := s.doc.Rename(""); err != nil {
		return err
	}
	after := s.doc.Path()
	if after != "" && after != before && h.pathTaken(after, s.id) {
		return errAlreadyOpen
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) MoveTo(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	before := s.doc.Path()
	if err := s.doc.MoveTo(""); err != nil {
		return err
	}
	after := s.doc.Path()
	if after != "" && after != before && h.pathTaken(after, s.id) {
		return errAlreadyOpen
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) Revert(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	return s.doc.Revert()
}

