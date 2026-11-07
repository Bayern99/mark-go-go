package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// WorkspaceSnapshot is one workspace window as the page lists it.
type WorkspaceSnapshot struct {
	ID        string         `json:"id"`
	Root      string         `json:"root"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	Documents []DocumentInfo `json:"documents"`
	ActiveID  string         `json:"activeId"`
}

// DocumentInfo is one open document in a workspace. ID is stable across
// path, tab and window.
type DocumentInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Edited   bool   `json:"edited"`
	Conflict bool   `json:"conflict"`
	Deleted  bool   `json:"deleted"`
	ReadOnly bool   `json:"readOnly"`
}

// CloseResult is the outcome of CloseDocument or CloseWorkspace.
type CloseResult struct {
	Closed   bool `json:"closed"`
	Canceled bool `json:"canceled"`
}

// NativeCommand asks the page to flush a captured document (or every
// document, for closeWorkspace) then call the matching host method.
type NativeCommand struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

var (
	workspaceChanged = mygo.NewEvent[WorkspaceSnapshot]("workspaceChanged")
	nativeCommand    = mygo.NewEvent[NativeCommand]("nativeCommand")
)

var (
	errNoDocument  = errors.New("markgogo: no Markdown document with that id")
	errAlreadyOpen = errors.New("markgogo: that file is already open in another document")
	errNoWorkspace = errors.New("markgogo: this window is not a workspace")
)

// workspace is one window: a folder root, or an untitled window.
type workspace struct {
	id     string
	root   string // empty = untitled window
	win    *mygo.Window
	tabs   []string
	active string
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

func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func folderContains(root, file string) bool {
	if root == "" || file == "" {
		return false
	}
	root = filepath.Clean(root)
	file = filepath.Clean(file)
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func deepestRoot(file string, roots []string) string {
	var best string
	for _, root := range roots {
		if !folderContains(root, file) {
			continue
		}
		if best == "" || len(filepath.Clean(root)) > len(filepath.Clean(best)) {
			best = root
		}
	}
	return best
}

// pendingClose is one document considered during a workspace close.
type pendingClose struct {
	ID    string
	Dirty bool
}

// confirmClose asks about every dirty document without removing any.
// If confirm reports canceled, no ids are returned.
func confirmClose(items []pendingClose, confirm func(id string) (canceled bool)) (closeIDs []string, canceled bool) {
	for _, it := range items {
		if !it.Dirty {
			continue
		}
		if confirm(it.ID) {
			return nil, true
		}
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out, false
}

func (h *documentHost) setDocuments(ds *mygo.Documents[string]) {
	h.mu.Lock()
	h.docs = ds
	h.mu.Unlock()
}

func (h *documentHost) workspaceOf(ctx context.Context) (*workspace, error) {
	return h.workspaceOfWindow(mygo.CallerWindow(ctx))
}

func (h *documentHost) workspaceOfWindow(win *mygo.Window) (*workspace, error) {
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

func (h *documentHost) docSession(id string) (*docSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.byID[id]
	if s == nil {
		return nil, errNoDocument
	}
	return s, nil
}

func (h *documentHost) snapshotWorkspace(w *workspace) WorkspaceSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotWorkspaceLocked(w)
}

func (h *documentHost) snapshotWorkspaceLocked(w *workspace) WorkspaceSnapshot {
	docs := make([]DocumentInfo, 0, len(w.tabs))
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil {
			continue
		}
		docs = append(docs, s.info())
	}
	return WorkspaceSnapshot{
		ID:        w.id,
		Root:      w.root,
		Name:      w.name(),
		Kind:      w.kind(),
		Documents: docs,
		ActiveID:  w.active,
	}
}

func (h *documentHost) emitWorkspace(w *workspace) {
	if w == nil || w.win == nil || w.win.IsDestroyed() {
		return
	}
	_ = workspaceChanged.Emit(w.win, h.snapshotWorkspace(w))
}

// Snapshot is the calling window's workspace.
func (h *documentHost) Snapshot(ctx context.Context) (WorkspaceSnapshot, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return WorkspaceSnapshot{}, err
	}
	return h.snapshotWorkspace(w), nil
}

// Open opens path with global routing: existing session, else deepest
// open folder workspace, else a workspace at the parent directory.
func (h *documentHost) Open(ctx context.Context, path string) (DocumentInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	h.mu.Lock()
	for _, s := range h.byID {
		if sameFile(s.doc.Path(), abs) {
			h.mu.Unlock()
			h.activate(s)
			if w := s.doc.Window(); w != nil {
				w.Show()
				w.Focus()
			}
			return s.info(), nil
		}
	}
	h.mu.Unlock()
	doc, err := h.docs.Open(abs)
	if err != nil {
		return DocumentInfo{}, err
	}
	s, err := h.docSession(doc.ID())
	if err != nil {
		return DocumentInfo{}, err
	}
	h.activate(s)
	return s.info(), nil
}

// OpenWorkspace focuses an existing folder workspace or opens one empty.
func (h *documentHost) OpenWorkspace(ctx context.Context, root string) error {
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
	h.mu.Lock()
	for _, w := range h.byWindow {
		if w.root != "" && filepath.Clean(w.root) == filepath.Clean(abs) {
			win := w.win
			h.mu.Unlock()
			win.Show()
			win.Focus()
			return nil
		}
	}
	h.mu.Unlock()
	win := h.newWorkspaceWindow(abs, filepath.Base(abs))
	win.Show()
	return nil
}

// New creates an untitled document. MyGo calls Window, which reuses an
// untitled workspace or makes one.
func (h *documentHost) New(ctx context.Context) (DocumentInfo, error) {
	doc := h.docs.New()
	s, err := h.docSession(doc.ID())
	if err != nil {
		return DocumentInfo{}, err
	}
	return s.info(), nil
}

// Duplicate opens an untitled copy of the document.
func (h *documentHost) Duplicate(ctx context.Context, documentID string) (DocumentInfo, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return DocumentInfo{}, err
	}
	copy := s.doc.Duplicate()
	d, err := h.docSession(copy.ID())
	if err != nil {
		return DocumentInfo{}, err
	}
	return d.info(), nil
}

// Activate makes documentID the active tab of its window.
func (h *documentHost) Activate(ctx context.Context, documentID string) error {
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	h.activate(s)
	if s.win != nil {
		s.win.Show()
		s.win.Focus()
	}
	return nil
}

func (h *documentHost) activate(s *docSession) {
	if h.docs != nil {
		h.docs.SetActive(s.doc)
	}
	h.mu.Lock()
	if s.ws != nil {
		s.ws.active = s.id
	}
	w := s.ws
	h.mu.Unlock()
	h.emitWorkspace(w)
}

// CloseDocument asks to save the document, then drops its tab. Cancel
// leaves the workspace and every tab as they were.
func (h *documentHost) CloseDocument(ctx context.Context, documentID string) (CloseResult, error) {
	s, err := h.docSession(documentID)
	if err != nil {
		return CloseResult{}, err
	}
	err = s.doc.CloseDocument()
	if errors.Is(err, mygo.ErrCanceled) {
		return CloseResult{Canceled: true}, nil
	}
	if err != nil {
		return CloseResult{}, err
	}
	// OnClosed drops the session. Empty untitled windows close there.
	return CloseResult{Closed: true}, nil
}

// CloseWorkspace asks to save every dirty document without dropping tabs.
// Cancel keeps the window and every tab. Success then closes them all.
func (h *documentHost) CloseWorkspace(ctx context.Context) (CloseResult, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return CloseResult{}, err
	}
	return h.closeWorkspace(w)
}

func (h *documentHost) closeWorkspace(w *workspace) (CloseResult, error) {
	h.mu.Lock()
	items := make([]pendingClose, 0, len(w.tabs))
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil {
			continue
		}
		items = append(items, pendingClose{ID: id, Dirty: s.doc.IsEdited()})
	}
	h.mu.Unlock()
	ids, canceled := confirmClose(items, func(id string) bool {
		s, err := h.docSession(id)
		if err != nil {
			return false
		}
		err = s.doc.ConfirmClose()
		return errors.Is(err, mygo.ErrCanceled)
	})
	if canceled {
		h.emitWorkspace(w)
		return CloseResult{Canceled: true}, nil
	}
	for _, id := range ids {
		s, err := h.docSession(id)
		if err != nil {
			continue
		}
		_ = s.doc.CloseDocument()
	}
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Destroy()
	}
	return CloseResult{Closed: true}, nil
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

func (h *documentHost) interceptCommand(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
	if doc == nil {
		return false
	}
	cmd, ok := nativeCommandName(role)
	if !ok {
		return false
	}
	_ = nativeCommand.Emit(win, NativeCommand{ID: doc.ID(), Command: cmd})
	return true
}

// SetNativeEditState enables native Undo/Redo for the focused window's
// active document. Reports from a background pane are ignored so they
// cannot steal menu state. This is editor undo, not UndoView.
func (h *documentHost) SetNativeEditState(ctx context.Context, documentID string, canUndo, canRedo bool) error {
	if _, err := h.docSession(documentID); err != nil {
		return err
	}
	win := mygo.CallerWindow(ctx)
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
	if active != "" && active != documentID {
		return nil
	}
	applyNativeEditMenu(canUndo, canRedo)
	return nil
}

func applyNativeEditMenu(canUndo, canRedo bool) {
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

func (h *documentHost) interceptWindowClose(win *mygo.Window) bool {
	_ = nativeCommand.Emit(win, NativeCommand{Command: "closeWorkspace"})
	return true
}

func (h *documentHost) newWorkspaceWindow(root, title string) *mygo.Window {
	if title == "" {
		if root == "" {
			title = "Untitled"
		} else {
			title = filepath.Base(root)
		}
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     title,
		URL:       "/",
		Width:     940,
		Height:    720,
		MinWidth:  480,
		MinHeight: 360,
	})
	w := &workspace{id: newWorkspaceID(), root: root, win: win}
	h.mu.Lock()
	h.byWindow[win] = w
	h.mu.Unlock()
	win.OnClosed(func() { h.forgetWindow(win) })
	return win
}

func (h *documentHost) forgetWindow(win *mygo.Window) {
	h.mu.Lock()
	w := h.byWindow[win]
	delete(h.byWindow, win)
	var ids []string
	if w != nil {
		ids = append([]string{}, w.tabs...)
	}
	h.mu.Unlock()
	for _, id := range ids {
		if s, err := h.docSession(id); err == nil {
			h.dropSession(s)
		}
	}
}
