package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	// sessionWriteMu serializes snapshots and writes, including the final quit save.
	sessionWriteMu sync.Mutex

	byWindow       map[*mygo.Window]*workspace
	byID           map[string]*docSession
	workspaceOrder []string
	// claims are the files a session has reserved for a path change that
	// has not touched the disk yet, by canonical path: two windows cannot
	// both take one target.
	claims map[string]string

	docs *mygo.Documents[string]

	settings     *settingsStore
	runs         sync.WaitGroup
	disableWatch bool
	quitting     bool
	restoring    bool
	sessionReady bool
	quitPending  map[*mygo.Window]bool
	quitAttempt  int
	quitSequence int
	quitSaving   bool
	quitTimer    *time.Timer

	// agentCache, when non-nil, keeps at most one idle ACP agent process
	// for sequential generations; nil starts one process per run.
	agentCache *acpProcessCache

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
	pathFn       func() string
	viewMode     string
	restoreError string
	// actionError is why the last native command on this document failed,
	// shown in its window, guarded by documentHost.mu.
	actionError string

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
	settings := &settingsStore{dir: dir}
	// The settings file is the app's one store: tell every window when it
	// changes, so a panel in another document does not show an old value.
	settings.onChange = func(s Settings) {
		_ = settingsChanged.Broadcast(s)
	}
	return &documentHost{
		byWindow:   make(map[*mygo.Window]*workspace),
		byID:       make(map[string]*docSession),
		claims:     make(map[string]string),
		settings:   settings,
		agentCache: newACPProcessCache(),
		// Recovery opens documents before restoreAfterStart reads the
		// session. Their windows must not be written over session.json
		// before the stored layout is planned from it.
		restoring: true,
	}
}

// closeAgentCache stops the idle agent process and removes its private
// folder. Active runs are stopped through their contexts first.
func (h *documentHost) closeAgentCache() {
	if h.agentCache != nil {
		h.agentCache.close()
	}
}

func (s *docSession) info() DocumentInfo {
	st := s.documentState()
	if s.host == nil {
		return s.makeInfo(st, false)
	}
	s.host.mu.Lock()
	defer s.host.mu.Unlock()
	return s.makeInfo(st, s.generating)
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

// makeInfo reads the session fields with documentHost.mu held for hosted documents.
func (s *docSession) makeInfo(st mygo.DocumentState, generating bool) DocumentInfo {
	return DocumentInfo{
		ID:           s.id,
		Name:         st.Name,
		Path:         st.Path,
		Edited:       st.Edited,
		Conflict:     st.Conflict,
		Deleted:      st.Deleted,
		ReadOnly:     st.ReadOnly,
		Generating:   generating,
		ViewMode:     s.viewMode,
		ActionError:  s.actionError,
		RestoreError: s.restoreError,
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
		root = canonicalPath(filepath.Dir(p))
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
		if w := h.folderWorkspaceLocked(path, false); w != nil {
			return w.win
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

// folderWorkspaceLocked finds an open folder workspace for path. deepest
// containing root wins; if includeParent and none contain the file, a
// workspace whose root is the parent directory is reused. Roots are
// compared after canonicalPath so aliases of the same directory match.
func (h *documentHost) folderWorkspaceLocked(path string, includeParent bool) *workspace {
	var roots []string
	byRoot := map[string]*workspace{}
	for _, w := range h.byWindow {
		if w.root == "" || w.win == nil || w.win.IsDestroyed() {
			continue
		}
		roots = append(roots, w.root)
		byRoot[canonicalPath(w.root)] = w
	}
	if best := deepestRoot(path, roots); best != "" {
		return byRoot[canonicalPath(best)]
	}
	if includeParent && path != "" {
		return byRoot[canonicalPath(filepath.Dir(path))]
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
		h.reindexOpenDocument(s)
	})
	s.offClosed = doc.OnClosed(func() {
		h.dropSession(s)
	})
	if h.docs != nil {
		h.docs.SetActive(doc)
	}
	h.emitWorkspace(w)
	h.reindexOpenDocument(s)
	h.persistSession()
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
	h.persistSession()
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
	return h.folderWorkspaceLocked(path, true)
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
	if h.docs != nil && s.doc != nil {
		h.docs.Attach(s.doc, dest.win)
	}
	h.docs.SetActive(s.doc)
	h.emitWorkspace(src)
	h.emitWorkspace(dest)
	if src != nil && src.root == "" && len(src.tabs) == 0 && src.win != nil && !src.win.IsDestroyed() {
		src.win.Destroy()
		return
	}
	h.persistSession()
}

// pathChangeLock is DocumentsOptions.PathChange: a document whose
// generation holds it may not change its file now, so MyGo disables Save
// As, Rename and Move To for it, and refuses a path change that still
// reaches the kernel. Ordinary Save, the document's own file, is not a
// path change and stays available.
func (h *documentHost) pathChangeLock(doc *mygo.Document[string]) error {
	return h.lockedPathChange(doc.ID())
}

func (h *documentHost) lockedPathChange(id string) error {
	h.mu.Lock()
	s := h.byID[id]
	busy := s != nil && s.generating
	h.mu.Unlock()
	if busy {
		return errors.New(errGenerating)
	}
	return nil
}

// refreshPathMenus updates the native Save As / Rename / Move roles after
// a document's generation lock changed. MyGo reads that lock from this
// host, but only a state change it knows about makes it look again; the
// page cannot wait on a menu while a dialog holds the main thread.
func (h *documentHost) refreshPathMenus() {
	if !mygo.App.IsReady() {
		return
	}
	mygo.RefreshDocumentMenus()
}

// reservePath is DocumentsOptions.ReservePath: it runs with the file a
// document is about to become, before MyGo writes to it or moves the
// document's file there. It refuses a path change the document's
// generation locks and a file another live session has, and holds the
// target until the file operation ends, so two windows cannot both take
// one file. Nothing on disk changes when it refuses.
func (h *documentHost) reservePath(doc *mygo.Document[string], target string) (func(), error) {
	return h.reserveTarget(doc.ID(), doc.Path(), target)
}

// reserveTarget is reservePath's decision: the document's own file is
// always available to save to; another file is refused while the
// document generates, and while another live session has it. A granted
// target stays claimed until the returned release runs.
func (h *documentHost) reserveTarget(id, path, target string) (func(), error) {
	h.mu.Lock()
	s := h.byID[id]
	if s == nil {
		h.mu.Unlock()
		return nil, nil
	}
	if s.generating && !sameFile(target, path) {
		h.mu.Unlock()
		return nil, errors.New(errGenerating)
	}
	canon := canonicalPath(target)
	if owner := h.claims[canon]; owner != "" && owner != id {
		h.mu.Unlock()
		return nil, errAlreadyOpen
	}
	for other, sess := range h.byID {
		if other == id {
			continue
		}
		if sameFile(sess.currentPath(), target) {
			h.mu.Unlock()
			return nil, errAlreadyOpen
		}
	}
	if h.claims == nil {
		h.claims = make(map[string]string)
	}
	h.claims[canon] = id
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		if h.claims[canon] == id {
			delete(h.claims, canon)
		}
		h.mu.Unlock()
	}, nil
}

// noteAction records how a native command on s ended: a failure goes on
// the document's state for its window to show, and a success clears the
// previous one. A canceled dialog is not a failure.
func (h *documentHost) noteAction(s *docSession, err error) {
	msg := ""
	if err != nil && !errors.Is(err, mygo.ErrCanceled) {
		msg = actionMessage(err)
	}
	h.mu.Lock()
	changed := s.actionError != msg
	s.actionError = msg
	win := s.win
	w := s.ws
	h.mu.Unlock()
	if !changed {
		return
	}
	if win != nil && !win.IsDestroyed() {
		_ = docState.Emit(win, s.info())
	}
	h.emitWorkspace(w)
}

// actionMessage is err's text as a window shows it, without the package
// prefix errors carry for logs.
func actionMessage(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"markgogo: ", "mygo: "} {
		if rest, ok := strings.CutPrefix(msg, prefix); ok {
			return rest
		}
	}
	return msg
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
	h.reindexOpenDocument(s)
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
	err = h.saveDocument(s)
	h.noteAction(s, err)
	return err
}

// saveDocument saves s as File > Save does. An untitled or read-only
// document's save asks for another file, which is a path change: a
// generation in progress refuses it before any dialog, as the kernel
// would before any write.
func (h *documentHost) saveDocument(s *docSession) error {
	path := s.currentPath()
	readOnly := s.doc != nil && s.doc.IsReadOnly()
	if path == "" || readOnly {
		if err := h.rejectPathChange(s); err != nil {
			return err
		}
	}
	if path == "" {
		return h.saveAsDocument(s)
	}
	if err := s.doc.Save(); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) SaveAs(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.saveAsDocument(s)
	h.noteAction(s, err)
	return err
}

// saveAsDocument writes s to a file the user picks. Uniqueness is the
// kernel's: it reserves the target before writing, so an open file is
// refused with nothing on disk changed.
func (h *documentHost) saveAsDocument(s *docSession) error {
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	target, err := h.askSavePath(s)
	if err != nil {
		return err
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
	err = h.renameDocument(s)
	h.noteAction(s, err)
	return err
}

// renameDocument renames s's file. The kernel reserves the target before
// it moves anything, so a file another session has is refused there.
func (h *documentHost) renameDocument(s *docSession) error {
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.Rename(""); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) MoveTo(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.moveDocument(s)
	h.noteAction(s, err)
	return err
}

func (h *documentHost) moveDocument(s *docSession) error {
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.MoveTo(""); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *documentHost) Revert(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = s.doc.Revert()
	h.noteAction(s, err)
	return err
}
