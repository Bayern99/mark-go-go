package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo"
)

type WorkspaceSummary struct {
	ID   string `json:"id"`
	Root string `json:"root"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type WorkspaceSnapshot struct {
	ID           string             `json:"id"`
	Root         string             `json:"root"`
	Name         string             `json:"name"`
	Kind         string             `json:"kind"`
	Workspaces   []WorkspaceSummary `json:"workspaces"`
	Documents    []DocumentInfo     `json:"documents"`
	ActiveID     string             `json:"activeId"`
	RestoreError string             `json:"restoreError,omitempty"`
}

type DocumentInfo struct {
	WorkspaceRoot   string  `json:"workspaceRoot,omitempty"`
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	Edited          bool    `json:"edited"`
	Conflict        bool    `json:"conflict"`
	Deleted         bool    `json:"deleted"`
	ReadOnly        bool    `json:"readOnly"`
	Generating      bool    `json:"generating"`
	ViewMode        string  `json:"viewMode,omitempty"`
	RestoredScrollY float32 `json:"restoredScrollY,omitempty"`
	ScrollRestore   uint64  `json:"scrollRestore,omitempty"`
	ActionError     string  `json:"actionError,omitempty"`
	RestoreError    string  `json:"restoreError,omitempty"`
}

type CloseResult struct {
	Closed   bool `json:"closed"`
	Canceled bool `json:"canceled"`
}

type NativeCommand struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Attempt int    `json:"attempt,omitempty"`
}

type QuitFailure struct {
	Title, Body string
}

var (
	WorkspaceChanged = desktop.Event[WorkspaceSnapshot]{}
	CommandRequested = desktop.Event[NativeCommand]{}
	QuitFailed       = desktop.Event[QuitFailure]{}
)

const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 820

	minWindowWidth  = 480
	minWindowHeight = 360
)

var (
	errNoDocument  = errors.New("markgogo: no Markdown document with that id")
	errAlreadyOpen = errors.New("markgogo: that file is already open in another document")
	errNoWorkspace = errors.New("markgogo: this window is not a workspace")
)

type workspace struct {
	id           string
	windowID     string
	root         string
	win          *mygo.Window
	tabs         []string
	active       string
	index        *workspaceIndex
	restoreError string
}

func (w *workspace) kind() string {
	if w.root == "" {
		return "untitled"
	}
	return "folder"
}

func (w *workspace) name() string {
	if w.root == "" {
		return "Untitled"
	}
	base := filepath.Base(w.root)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return w.root
	}
	return base
}

func newWorkspaceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (h *Service) SetDocuments(ds *mygo.Documents[string]) {
	h.mu.Lock()
	h.docs = ds
	h.mu.Unlock()
}

func (h *Service) workspaceOf(ctx context.Context) (*workspace, error) {
	return h.workspaceOfWindow(desktop.Window(ctx))
}

func (h *Service) workspaceOfWindow(win *mygo.Window) (*workspace, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if win == nil {
		return nil, errNoWorkspace
	}
	w := h.byWindow[win]
	if w == nil {
		return nil, errNoWorkspace
	}
	return w, nil
}

func (h *Service) workspaceByIDLocked(id string) *workspace {
	if w := h.byWorkspace[id]; w != nil {
		return w
	}
	for _, w := range h.byWindow {
		if w != nil && w.id == id {
			return w
		}
	}
	return nil
}

func (h *Service) allWorkspacesLocked() []*workspace {
	out := make([]*workspace, 0, len(h.byWorkspace)+len(h.byWindow))
	seen := map[string]bool{}
	for _, id := range h.workspaceOrder {
		if w := h.workspaceByIDLocked(id); w != nil && !seen[id] {
			seen[id] = true
			out = append(out, w)
		}
	}
	for id, w := range h.byWorkspace {
		if w != nil && !seen[id] {
			seen[id] = true
			out = append(out, w)
		}
	}
	for _, w := range h.byWindow {
		if w != nil && !seen[w.id] {
			seen[w.id] = true
			out = append(out, w)
		}
	}
	return out
}

func (h *Service) windowWorkspacesLocked(win *mygo.Window) []*workspace {
	out := make([]*workspace, 0, 1)
	for _, w := range h.allWorkspacesLocked() {
		if w.win == win {
			out = append(out, w)
		}
	}
	return out
}

func (h *Service) preferredWindow() *mygo.Window {
	if win := mygo.FocusedWindow(); win != nil && !win.IsDestroyed() {
		h.mu.Lock()
		w := h.byWindow[win]
		h.mu.Unlock()
		if w != nil {
			return win
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, w := range h.allWorkspacesLocked() {
		if w.win != nil && !w.win.IsDestroyed() {
			return w.win
		}
	}
	return nil
}

func (h *Service) docSession(id string) (*docSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.byID[id]
	if s == nil {
		return nil, errNoDocument
	}
	return s, nil
}

func (h *Service) snapshotWorkspace(win *mygo.Window) (WorkspaceSnapshot, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotWorkspaceLocked(win)
}

func (h *Service) snapshotWorkspaceLocked(win *mygo.Window) (WorkspaceSnapshot, error) {
	w := h.byWindow[win]
	if w == nil {
		return WorkspaceSnapshot{}, errNoWorkspace
	}
	roots := h.windowWorkspacesLocked(win)
	docs := make([]DocumentInfo, 0)
	for _, root := range roots {
		for _, id := range root.tabs {
			s := h.byID[id]
			if s != nil {
				docs = append(docs, s.infoLocked())
			}
		}
	}
	summaries := make([]WorkspaceSummary, 0, len(roots))
	for _, root := range roots {
		summaries = append(summaries, WorkspaceSummary{ID: root.id, Root: root.root, Name: root.name(), Kind: root.kind()})
	}
	windowID := w.windowID
	if windowID == "" {
		windowID = w.id
	}
	return WorkspaceSnapshot{
		ID:           windowID,
		Root:         w.root,
		Name:         w.name(),
		Kind:         w.kind(),
		Workspaces:   summaries,
		Documents:    docs,
		ActiveID:     w.active,
		RestoreError: w.restoreError,
	}, nil
}

func (h *Service) emitWorkspace(w *workspace) {
	if w == nil {
		return
	}
	h.mu.Lock()
	win := w.win
	if win == nil || win.IsDestroyed() {
		h.mu.Unlock()
		return
	}
	snap, err := h.snapshotWorkspaceLocked(win)
	h.mu.Unlock()
	if err == nil {
		_ = WorkspaceChanged.Emit(win, snap)
	}
}

func (h *Service) Snapshot(ctx context.Context) (WorkspaceSnapshot, error) {
	win := desktop.Window(ctx)
	if win == nil {
		return WorkspaceSnapshot{}, errNoWorkspace
	}
	return h.snapshotWorkspace(win)
}

func (h *Service) Open(ctx context.Context, path string) (DocumentInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	abs = fsutil.CanonicalPath(abs)
	type openCandidate struct {
		session *docSession
		path    string
	}
	h.mu.Lock()
	candidates := make([]openCandidate, 0, len(h.byID))
	for _, s := range h.byID {
		if s != nil && s.doc != nil {
			candidates = append(candidates, openCandidate{session: s, path: s.doc.Path()})
		}
	}
	h.mu.Unlock()
	for _, item := range candidates {
		if fsutil.SameFile(item.path, abs) {
			h.activate(item.session)
			if w := item.session.doc.Window(); w != nil {
				w.Show()
				w.Focus()
			}
			return item.session.info(), nil
		}
	}
	s, err := h.openDocumentSession(abs)
	if err != nil {
		return DocumentInfo{}, err
	}
	h.activate(s)
	return s.info(), nil
}

func (h *Service) openDocumentSession(path string) (*docSession, error) {
	if h.openDocument != nil {
		return h.openDocument(path)
	}
	doc, err := h.docs.Open(path)
	if err != nil {
		return nil, err
	}
	return h.docSession(doc.ID())
}

func (h *Service) OpenWorkspace(ctx context.Context, root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	abs = fsutil.CanonicalPath(abs)
	type rootCandidate struct {
		ws   *workspace
		root string
	}
	h.mu.Lock()
	all := h.allWorkspacesLocked()
	candidates := make([]rootCandidate, 0, len(all))
	for _, w := range all {
		if w.root != "" {
			candidates = append(candidates, rootCandidate{ws: w, root: w.root})
		}
	}
	h.mu.Unlock()
	for _, item := range candidates {
		if fsutil.SameRoot(item.root, abs) {
			h.activateWorkspace(item.ws)
			item.ws.win.Show()
			item.ws.win.Focus()
			h.rememberRoot(abs)
			return nil
		}
	}
	var win *mygo.Window
	if current, currentErr := h.workspaceOf(ctx); currentErr == nil {
		win = current.win
	} else {
		win = h.preferredWindow()
	}
	if win == nil {
		win = h.newWorkspaceWindow(abs, filepath.Base(abs))
	} else {
		w := h.addWorkspaceToWindow("", abs, filepath.Base(abs), win)
		h.activateWorkspace(w)
	}
	win.Show()
	win.Focus()
	h.rememberRoot(abs)
	h.persistSession()
	return nil
}

func (h *Service) ActivateWorkspace(ctx context.Context, workspaceID string) error {
	h.mu.Lock()
	w := h.workspaceByIDLocked(workspaceID)
	h.mu.Unlock()
	if w == nil {
		return errNoWorkspace
	}
	h.activateWorkspace(w)
	if w.win != nil {
		w.win.Show()
		w.win.Focus()
	}
	return nil
}

func (h *Service) activateWorkspace(w *workspace) {
	if w == nil {
		return
	}
	h.mu.Lock()
	h.byWindow[w.win] = w
	active := w.active
	h.mu.Unlock()
	if active != "" {
		if s, err := h.docSession(active); err == nil && h.docs != nil {
			h.docs.SetActive(s.doc)
		}
	} else if h.docs != nil && w.win != nil && !w.win.IsDestroyed() {
		h.docs.ClearActive(w.win, w.name())
		ApplyNativeEditMenu(false, false)
	}
	h.emitWorkspace(w)
	h.emitNav(w)
	h.emitSearch(w, h.indexOf(w))
	h.persistSession()
}

func (h *Service) New(ctx context.Context) (DocumentInfo, error) {
	doc := h.docs.New()
	s, err := h.docSession(doc.ID())
	if err != nil {
		return DocumentInfo{}, err
	}
	return s.info(), nil
}

func (h *Service) Duplicate(ctx context.Context, documentID string) (DocumentInfo, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return DocumentInfo{}, err
	}
	copy := s.doc.Duplicate()
	d, err := h.docSession(copy.ID())
	h.noteAction(s, err)
	if err != nil {
		return DocumentInfo{}, err
	}
	return d.info(), nil
}

func (h *Service) Activate(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	h.activate(s)
	if win := h.sessionWindow(s); win != nil {
		win.Show()
		win.Focus()
	}
	return nil
}

func (h *Service) activate(s *docSession) {
	if h.docs != nil && s.doc != nil {
		h.docs.SetActive(s.doc)
	}
	h.mu.Lock()
	if s.ws != nil {
		s.ws.active = s.id
		if s.win != nil {
			h.byWindow[s.win] = s.ws
		}
	}
	w := s.ws
	h.mu.Unlock()
	h.emitWorkspace(w)
	h.persistSession()
}

func (h *Service) CloseDocument(ctx context.Context, documentID string) (CloseResult, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return CloseResult{}, err
	}
	err = s.doc.CloseDocument()
	if errors.Is(err, mygo.ErrCanceled) {
		h.noteAction(s, nil)
		return CloseResult{Canceled: true}, nil
	}
	if err != nil {
		h.noteAction(s, err)
		return CloseResult{}, err
	}
	return CloseResult{Closed: true}, nil
}

func (h *Service) CloseWorkspace(ctx context.Context) (CloseResult, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return CloseResult{}, err
	}
	return h.closeWorkspace(w)
}

func (h *Service) closeWorkspace(w *workspace) (CloseResult, error) {
	if w.win == nil {
		return CloseResult{}, errNoWorkspace
	}
	if err := h.docs.CloseDocumentGroup(w.win); errors.Is(err, mygo.ErrCanceled) {
		h.emitWorkspace(w)
		return CloseResult{Canceled: true}, nil
	} else if err != nil {
		return CloseResult{}, err
	}
	return CloseResult{Closed: true}, nil
}

func (h *Service) CloseWorkspaceRoot(ctx context.Context, workspaceID string) (CloseResult, error) {
	caller, callerErr := h.workspaceOf(ctx)
	h.mu.Lock()
	w := h.workspaceByIDLocked(workspaceID)
	var tabIDs []string
	if w != nil {
		tabIDs = slices.Clone(w.tabs)
	}
	h.mu.Unlock()
	if w == nil || (callerErr == nil && caller.win != w.win) {
		return CloseResult{}, errNoWorkspace
	}
	var docs []*mygo.Document[string]
	for _, id := range tabIDs {
		if s, err := h.docSession(id); err == nil && s.doc != nil {
			docs = append(docs, s.doc)
		}
	}
	if len(docs) > 0 {
		if err := h.docs.CloseDocuments(docs); errors.Is(err, mygo.ErrCanceled) {
			h.emitWorkspace(w)
			return CloseResult{Canceled: true}, nil
		} else if err != nil {
			return CloseResult{}, err
		}
	}
	h.removeWorkspaceRoot(w)
	return CloseResult{Closed: true}, nil
}

func (h *Service) removeWorkspaceRoot(w *workspace) {
	if w == nil {
		return
	}
	h.mu.Lock()
	windowID := w.windowID
	if windowID == "" {
		windowID = w.id
	}
	delete(h.byWorkspace, w.id)
	h.workspaceOrder = removeWorkspaceOrder(h.workspaceOrder, w.id)
	var next *workspace
	wasCurrent := h.byWindow[w.win] == w
	if wasCurrent {
		delete(h.byWindow, w.win)
		for _, candidate := range h.allWorkspacesLocked() {
			if candidate.win == w.win && candidate != w {
				next = candidate
				break
			}
		}
	}
	h.mu.Unlock()
	h.forgetIndex(w)
	if !wasCurrent {
		h.mu.Lock()
		current := h.byWindow[w.win]
		h.mu.Unlock()
		h.emitWorkspace(current)
		h.persistSession()
		return
	}
	if next == nil {
		next = h.addWorkspaceToWindow("", "", "Welcome", w.win)
		h.mu.Lock()
		next.windowID = windowID
		h.mu.Unlock()
	}
	h.activateWorkspace(next)
}

func nativeCommandName(role mygo.MenuRole) (string, bool) {
	switch role {
	case mygo.RoleSave:
		return "save", true
	case mygo.RoleSaveAs:
		return "saveAs", true
	case mygo.RoleDuplicate:
		return "duplicate", true
	case mygo.RoleRename:
		return "rename", true
	case mygo.RoleMoveTo:
		return "moveTo", true
	case mygo.RoleRevert:
		return "revert", true
	case mygo.RoleClose:
		return "close", true
	case mygo.RoleUndo:
		return "undo", true
	case mygo.RoleRedo:
		return "redo", true
	default:
		return "", false
	}
}

func (h *Service) InterceptCommand(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
	if doc == nil {
		return false
	}
	cmd, ok := nativeCommandName(role)
	if !ok {
		return false
	}
	h.mu.Lock()
	current := h.byWindow[win]
	matchesVisibleDocument := current != nil && current.active == doc.ID()
	h.mu.Unlock()
	if !matchesVisibleDocument {
		return true
	}
	_ = CommandRequested.Emit(win, NativeCommand{ID: doc.ID(), Command: cmd})
	return true
}

func (h *Service) SetNativeEditState(ctx context.Context, documentID string, canUndo, canRedo bool) error {
	if _, err := h.docSession(documentID); err != nil {
		return err
	}
	win := desktop.Window(ctx)
	if win == nil {
		win = mygo.FocusedWindow()
	}
	h.mu.Lock()
	w := h.byWindow[win]
	active := ""
	if w != nil {
		active = w.active
	}
	h.mu.Unlock()
	if active != documentID {
		return nil
	}
	ApplyNativeEditMenu(canUndo, canRedo)
	return nil
}

func ApplyNativeEditMenu(canUndo, canRedo bool) {
	m := mygo.App.Menu()
	if m == nil {
		return
	}
	setMenuRoleEnabled(m, mygo.RoleUndo, canUndo)
	setMenuRoleEnabled(m, mygo.RoleRedo, canRedo)
}

func setMenuRoleEnabled(m *mygo.Menu, role mygo.MenuRole, on bool) {
	if m == nil {
		return
	}
	for _, it := range m.Items() {
		if it.Role == role {
			it.SetEnabled(on)
		}
		setMenuItemRoleEnabled(it, role, on)
	}
}

func setMenuItemRoleEnabled(it *mygo.MenuItem, role mygo.MenuRole, on bool) {
	if it == nil {
		return
	}
	for _, child := range it.Submenu {
		if child.Role == role {
			child.SetEnabled(on)
		}
		setMenuItemRoleEnabled(child, role, on)
	}
}

func (h *Service) InterceptWindowClose(win *mygo.Window) bool {
	h.mu.Lock()
	skip := h.quitting || h.sessionReady
	h.mu.Unlock()
	if skip {
		return false
	}
	_ = CommandRequested.Emit(win, NativeCommand{Command: "closeWorkspace"})
	return true
}

func (h *Service) newWorkspaceWindow(root, title string) *mygo.Window {
	if root != "" {
		root = fsutil.CanonicalPath(root)
	}
	return h.openWorkspaceWindow("", root, title)
}

func (h *Service) openWorkspaceWindow(id, root, title string) *mygo.Window {
	if root != "" {
		root = fsutil.CanonicalPath(root)
	}
	if title == "" {
		if root == "" {
			title = "Untitled"
		} else {
			title = filepath.Base(root)
		}
	}
	if id == "" {
		id = newWorkspaceID()
	}
	win := h.CreateWindow(mygo.WindowOptions{
		Title:     title,
		Width:     defaultWindowWidth,
		Height:    defaultWindowHeight,
		MinWidth:  minWindowWidth,
		MinHeight: minWindowHeight,
	})
	h.addWorkspaceToWindow(id, root, title, win)
	win.OnClosed(func() { h.forgetWindow(win) })
	if h.docs != nil {
		h.docs.ClearActive(win, title)
	}
	return win
}

func (h *Service) addWorkspaceToWindow(id, root, title string, win *mygo.Window) *workspace {
	if root != "" {
		root = fsutil.CanonicalPath(root)
	}
	if title == "" {
		if root == "" {
			title = "Untitled"
		} else {
			title = filepath.Base(root)
		}
	}
	if id == "" {
		id = newWorkspaceID()
	}
	h.mu.Lock()
	windowID := id
	if current := h.byWindow[win]; current != nil {
		windowID = current.windowID
		if windowID == "" {
			windowID = current.id
		}
		if root != "" && current.root == "" && len(current.tabs) == 0 {
			delete(h.byWorkspace, current.id)
			h.workspaceOrder = removeWorkspaceOrder(h.workspaceOrder, current.id)
		}
	}
	w := &workspace{id: id, windowID: windowID, root: root, win: win}
	if h.byWorkspace == nil {
		h.byWorkspace = make(map[string]*workspace)
	}
	h.byWorkspace[id] = w
	h.byWindow[win] = w
	h.workspaceOrder = appendWorkspaceOrder(h.workspaceOrder, id)
	attempt, saving := h.quitAttempt, h.quitSaving
	h.mu.Unlock()
	if attempt != 0 {
		h.failQuitAttempt(attempt, nil, errors.New("a workspace opened while preparing to quit"), saving, !saving)
	}
	if root != "" {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			h.rememberRoot(root)
			h.startIndex(w)
		}
	}
	return w
}

func (h *Service) forgetWindow(win *mygo.Window) {
	h.mu.Lock()
	roots := h.windowWorkspacesLocked(win)
	delete(h.byWindow, win)
	var ids []string
	for _, w := range roots {
		ids = append(ids, w.tabs...)
		delete(h.byWorkspace, w.id)
		h.workspaceOrder = removeWorkspaceOrder(h.workspaceOrder, w.id)
	}
	done := false
	attempt := 0
	var file sessionFileData
	if h.quitAttempt != 0 && h.quitPending != nil {
		attempt = h.quitAttempt
		done, file = h.ackQuitWindowLocked(win, attempt)
	}
	h.mu.Unlock()
	for _, w := range roots {
		h.forgetIndex(w)
	}
	for _, id := range ids {
		if s, err := h.docSession(id); err == nil {
			h.dropSessionState(s, false)
		}
	}
	h.sessionWG.Add(1)
	go func() {
		defer h.sessionWG.Done()
		h.persistSession()
		if done {
			h.completeQuitAttempt(attempt, file, win)
		}
	}()
}

func (h *Service) WaitForSessionWrites(limit time.Duration) {
	done := make(chan struct{})
	go func() {
		h.sessionWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(limit):
	}
}
