package workspace

import (
	"context"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/acp"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type Snapshot struct {
	ID      string       `json:"id"`
	Text    string       `json:"text"`
	Version int          `json:"version"`
	State   DocumentInfo `json:"state"`
}

var docReloaded = mygo.NewEvent[Snapshot]("docReloaded")

var docState = mygo.NewEvent[DocumentInfo]("docState")

type Service struct {
	mu             sync.Mutex
	sessionWriteMu sync.Mutex

	byWindow       map[*mygo.Window]*workspace
	byWorkspace    map[string]*workspace
	byID           map[string]*docSession
	workspaceOrder []string
	claims         map[string]string

	docs *mygo.Documents[string]

	settings     *settingsStore
	runs         sync.WaitGroup
	sessionWG    sync.WaitGroup
	disableWatch bool
	quitting     bool
	restoring    bool
	sessionReady bool
	quitPending  map[*mygo.Window]bool
	quitAttempt  int
	quitSequence int
	quitSaving   bool
	quitTimer    *time.Timer

	agentCache *acp.ProcessCache

	prepareJob func(*acp.GenerationJob)
}

type docSession struct {
	host *Service
	id   string
	doc  *mygo.Document[string]
	win  *mygo.Window
	ws   *workspace

	version      int
	offReload    func()
	offState     func()
	offClosed    func()
	generating   bool
	genSlot      *generationSlot
	closed       bool
	pathFn       func() string
	textFn       func() string
	edited       bool
	viewMode     string
	restoreError string
	actionError  string

	view *genui.ViewFollower
}

type generationSlot struct {
	cancel    context.CancelFunc
	path      string
	abandoned bool
}

func NewService() *Service {
	dir, _ := mygo.App.Path(mygo.PathUserData)
	settings := &settingsStore{dir: dir}
	settings.onChange = func(s Settings) {
		_ = settingsChanged.Broadcast(s)
	}
	return &Service{
		byWindow:    make(map[*mygo.Window]*workspace),
		byWorkspace: make(map[string]*workspace),
		byID:        make(map[string]*docSession),
		claims:      make(map[string]string),
		settings:    settings,
		agentCache:  acp.NewProcessCache(),
		restoring:   true,
	}
}

func (h *Service) CloseAgentCache() {
	if h.agentCache != nil {
		h.agentCache.Close()
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
	return mygo.DocumentState{Name: name, Path: path, Edited: s.edited}
}

func (s *docSession) currentText() string {
	if s.doc != nil {
		return s.doc.Content()
	}
	if s.textFn != nil {
		return s.textFn()
	}
	return ""
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
	root := ""
	if s.ws != nil {
		root = s.ws.root
	}
	return DocumentInfo{
		WorkspaceRoot: root,
		ID:            s.id,
		Name:          st.Name,
		Path:          st.Path,
		Edited:        st.Edited,
		Conflict:      st.Conflict,
		Deleted:       st.Deleted,
		ReadOnly:      st.ReadOnly,
		Generating:    generating,
		ViewMode:      s.viewMode,
		ActionError:   s.actionError,
		RestoreError:  s.restoreError,
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

func (h *Service) OpenWindow(doc *mygo.Document[string]) *mygo.Window {
	if win := h.existingWindow(doc); win != nil {
		h.adopt(doc, win)
		return win
	}
	root := ""
	if p := doc.Path(); p != "" {
		root = fsutil.CanonicalPath(filepath.Dir(p))
	}
	if win := h.preferredWindow(); win != nil {
		h.addWorkspaceToWindow("", root, filepath.Base(root), win)
		h.adopt(doc, win)
		return win
	}
	win := h.newWorkspaceWindow(root, doc.Name())
	h.adopt(doc, win)
	return win
}

func (h *Service) sessionWindow(s *docSession) *mygo.Window {
	if s == nil || h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return s.win
}

func (h *Service) sessionWorkspace(s *docSession) *workspace {
	if s == nil || h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return s.ws
}

func (h *Service) existingWindow(doc *mygo.Document[string]) *mygo.Window {
	path := doc.Path()
	if path != "" {
		if w := h.matchFolderWorkspace(path, false); w != nil {
			return w.win
		}
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if fw := mygo.FocusedWindow(); fw != nil {
		if w := h.byWindow[fw]; w != nil && w.win != nil && !w.win.IsDestroyed() {
			return w.win
		}
	}
	for _, w := range h.allWorkspacesLocked() {
		if w.root == "" && w.win != nil && !w.win.IsDestroyed() {
			return w.win
		}
	}
	return nil
}

func (h *Service) folderWorkspaceCandidatesLocked() ([]string, []*workspace) {
	var roots []string
	var candidates []*workspace
	for _, w := range h.allWorkspacesLocked() {
		if w.root == "" || w.win == nil || w.win.IsDestroyed() {
			continue
		}
		roots = append(roots, w.root)
		candidates = append(candidates, w)
	}
	return roots, candidates
}

func matchWorkspaceFromCandidates(path string, roots []string, candidates []*workspace, includeParent bool) *workspace {
	if best := fsutil.DeepestRoot(path, roots); best != "" {
		for i, r := range roots {
			if r == best {
				return candidates[i]
			}
		}
	}
	if includeParent && path != "" {
		parent := filepath.Dir(path)
		for i, r := range roots {
			if fsutil.SameRoot(r, parent) {
				return candidates[i]
			}
		}
	}
	return nil
}

func (h *Service) folderWorkspaceLocked(path string, includeParent bool) *workspace {
	roots, candidates := h.folderWorkspaceCandidatesLocked()
	return matchWorkspaceFromCandidates(path, roots, candidates, includeParent)
}

func (h *Service) matchFolderWorkspace(path string, includeParent bool) *workspace {
	h.mu.Lock()
	roots, candidates := h.folderWorkspaceCandidatesLocked()
	h.mu.Unlock()
	return matchWorkspaceFromCandidates(path, roots, candidates, includeParent)
}

func (h *Service) adopt(doc *mygo.Document[string], win *mygo.Window) {
	id := doc.ID()
	var owned *workspace
	if path := doc.Path(); path != "" {
		if candidate := h.matchFolderWorkspace(path, true); candidate != nil && candidate.win == win {
			owned = candidate
		}
	}
	h.mu.Lock()
	w := h.byWindow[win]
	if owned != nil {
		w = owned
	}
	if w == nil {
		h.mu.Unlock()
		return
	}
	if existing := h.byID[id]; existing != nil {
		h.mu.Unlock()
		h.activate(existing)
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
	h.byWindow[win] = w
	h.mu.Unlock()

	offReload := doc.OnReload(func() {
		h.mu.Lock()
		s.version++
		currentWin := s.win
		h.mu.Unlock()
		if currentWin != nil {
			_ = docReloaded.Emit(currentWin, s.snapshot())
		}
	})
	offState := doc.OnStateChange(func(state mygo.DocumentState) {
		h.mu.Lock()
		currentWin := s.win
		h.mu.Unlock()
		if currentWin != nil {
			_ = docState.Emit(currentWin, s.info())
		}
		s.viewMoved(state.Path)
		h.migrateIfNeeded(s)
		h.emitWorkspace(h.sessionWorkspace(s))
		h.reindexOpenDocument(s)
	})
	offClosed := doc.OnClosed(func() {
		h.dropSession(s)
	})
	h.mu.Lock()
	if s.closed {
		h.mu.Unlock()
		offReload()
		offState()
		offClosed()
		return
	}
	s.offReload = offReload
	s.offState = offState
	s.offClosed = offClosed
	h.mu.Unlock()
	if h.docs != nil {
		h.docs.SetActive(doc)
	}
	h.emitWorkspace(w)
	h.reindexOpenDocument(s)
	h.persistSession()
}

func (h *Service) DocumentFocused(doc *mygo.Document[string]) {
	if doc == nil {
		return
	}
	if s, err := h.docSession(doc.ID()); err == nil {
		h.activate(s)
	}
}

func (h *Service) dropSession(s *docSession) {
	if s == nil {
		return
	}
	if h.dropSessionState(s, true) {
		h.persistSession()
	}
}

func (h *Service) dropSessionState(s *docSession, emit bool) bool {
	if s == nil {
		return false
	}
	h.mu.Lock()
	if s.closed {
		h.mu.Unlock()
		return false
	}
	s.closed = true
	offReload, offState, offClosed := s.offReload, s.offState, s.offClosed
	s.offReload, s.offState, s.offClosed = nil, nil, nil
	h.mu.Unlock()
	if offReload != nil {
		offReload()
	}
	if offState != nil {
		offState()
	}
	if offClosed != nil {
		offClosed()
	}
	h.cancelGeneration(s)
	s.unfollowView()
	h.mu.Lock()
	delete(h.byID, s.id)
	w := s.ws
	var closedActive bool
	if w != nil {
		tabs := w.tabs[:0]
		for _, id := range w.tabs {
			if id != s.id {
				tabs = append(tabs, id)
			}
		}
		w.tabs = tabs
		if w.active == s.id {
			closedActive = h.byWindow[w.win] == w
			if n := len(w.tabs); n > 0 {
				w.active = w.tabs[n-1]
			} else {
				w.active = ""
			}
		}
	}
	s.ws = nil
	h.mu.Unlock()
	if !emit {
		return false
	}
	if h.retireEmptyUntitled(w) {
		return false
	}
	if closedActive && w.win != nil && !w.win.IsDestroyed() {
		h.activateWorkspace(w)
		return false
	}
	h.emitWorkspace(w)
	return true
}

func (h *Service) retireEmptyUntitled(w *workspace) bool {
	if w == nil || w.root != "" {
		return false
	}
	h.mu.Lock()
	if len(w.tabs) != 0 {
		h.mu.Unlock()
		return false
	}
	roots := h.windowWorkspacesLocked(w.win)
	if len(roots) > 1 {
		delete(h.byWorkspace, w.id)
		h.workspaceOrder = removeWorkspaceOrder(h.workspaceOrder, w.id)
		current := h.byWindow[w.win]
		wasCurrent := current == w
		if wasCurrent {
			current = nil
			for _, candidate := range roots {
				if candidate != w {
					current = candidate
					h.byWindow[w.win] = current
					break
				}
			}
		}
		h.mu.Unlock()
		if wasCurrent && current != nil && w.win != nil && !w.win.IsDestroyed() {
			h.activateWorkspace(current)
			return true
		}
		h.emitWorkspace(current)
		h.persistSession()
		return true
	}
	h.mu.Unlock()
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Destroy()
	}
	return true
}

func (h *Service) migrateIfNeeded(s *docSession) {
	path := s.doc.Path()
	if path == "" {
		return
	}
	dest := h.workspaceForPath(path)
	h.mu.Lock()
	src := s.ws
	win := s.win
	h.mu.Unlock()
	if dest == nil {
		parent := filepath.Dir(path)
		if win == nil {
			win = h.preferredWindow()
		}
		if win == nil {
			win = h.newWorkspaceWindow(parent, filepath.Base(parent))
		} else {
			h.addWorkspaceToWindow("", parent, filepath.Base(parent), win)
		}
		dest = h.workspaceForPath(path)
		h.moveTab(s, dest)
		return
	}
	if src == dest {
		return
	}
	h.docs.Attach(s.doc, dest.win)
	h.moveTab(s, dest)
}

func (h *Service) workspaceForPath(path string) *workspace {
	return h.matchFolderWorkspace(path, true)
}

func (h *Service) moveTab(s *docSession, dest *workspace) {
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
	h.byWindow[dest.win] = dest
	h.mu.Unlock()
	if h.docs != nil && s.doc != nil {
		h.docs.Attach(s.doc, dest.win)
	}
	h.docs.SetActive(s.doc)
	h.emitWorkspace(src)
	h.emitWorkspace(dest)
	if h.retireEmptyUntitled(src) {
		return
	}
	h.persistSession()
}

func (h *Service) PathChangeLock(doc *mygo.Document[string]) error {
	return h.lockedPathChange(doc.ID())
}

func (h *Service) lockedPathChange(id string) error {
	h.mu.Lock()
	s := h.byID[id]
	busy := s != nil && s.generating
	h.mu.Unlock()
	if busy {
		return errors.New(errGenBusy)
	}
	return nil
}

func (h *Service) refreshPathMenus() {
	if !mygo.App.IsReady() {
		return
	}
	mygo.RefreshDocumentMenus()
}

func (h *Service) ReservePath(doc *mygo.Document[string], target string) (func(), error) {
	return h.reserveTarget(doc.ID(), doc.Path(), target)
}

func (h *Service) reserveTarget(id, path, target string) (func(), error) {
	canon := fsutil.CanonicalPath(target)
	checked := map[string]string{}
	for {
		h.mu.Lock()
		s := h.byID[id]
		if s == nil {
			h.mu.Unlock()
			return nil, nil
		}
		if s.generating && !fsutil.SameFile(target, path) {
			h.mu.Unlock()
			return nil, errors.New(errGenBusy)
		}
		if owner := h.claims[canon]; owner != "" && owner != id {
			h.mu.Unlock()
			return nil, errAlreadyOpen
		}
		var pending []struct {
			id   string
			path string
		}
		for other, sess := range h.byID {
			if other == id {
				continue
			}
			otherPath := sess.currentPath()
			if prev, ok := checked[other]; !ok || prev != otherPath {
				pending = append(pending, struct {
					id   string
					path string
				}{id: other, path: otherPath})
			}
		}
		if len(pending) == 0 {
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
		h.mu.Unlock()
		for _, item := range pending {
			if fsutil.SameFile(item.path, target) {
				return nil, errAlreadyOpen
			}
			checked[item.id] = item.path
		}
	}
}

func (h *Service) noteAction(s *docSession, err error) {
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

func actionMessage(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"markgogo: ", "mygo: "} {
		if rest, ok := strings.CutPrefix(msg, prefix); ok {
			return rest
		}
	}
	return msg
}

func (h *Service) Load(ctx context.Context, documentID string) (Snapshot, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return Snapshot{}, err
	}
	return s.snapshot(), nil
}

func (h *Service) Update(ctx context.Context, documentID, text string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	s.doc.Set(text)
	h.reindexOpenDocument(s)
	return nil
}

func (h *Service) Flush(ctx context.Context, documentID, text string) error {
	return h.Update(ctx, documentID, text)
}

func (h *Service) Save(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.saveDocument(s)
	h.noteAction(s, err)
	return err
}

func (h *Service) saveDocument(s *docSession) error {
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

func (h *Service) SaveAs(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.saveAsDocument(s)
	h.noteAction(s, err)
	return err
}

func (h *Service) saveAsDocument(s *docSession) error {
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

func (h *Service) askSavePath(s *docSession) (string, error) {
	ext := "md"
	def := s.doc.Path()
	if def == "" {
		dir, _ := mygo.App.Path(mygo.PathDocuments)
		def = filepath.Join(dir, s.doc.Name()+"."+ext)
	}
	target, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      h.sessionWindow(s),
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

func (h *Service) Rename(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.renameDocument(s)
	h.noteAction(s, err)
	return err
}

func (h *Service) renameDocument(s *docSession) error {
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.Rename(""); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *Service) MoveTo(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = h.moveDocument(s)
	h.noteAction(s, err)
	return err
}

func (h *Service) moveDocument(s *docSession) error {
	if err := h.rejectPathChange(s); err != nil {
		return err
	}
	if err := s.doc.MoveTo(""); err != nil {
		return err
	}
	h.migrateIfNeeded(s)
	return nil
}

func (h *Service) Revert(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	err = s.doc.Revert()
	h.noteAction(s, err)
	return err
}
