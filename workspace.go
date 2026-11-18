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

// WorkspaceSummary is one root available in a native window.
type WorkspaceSummary struct {
	ID   string `json:"id"`
	Root string `json:"root"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// WorkspaceSnapshot is one native window. Root/Name/Kind describe the root
// whose navigation is selected; Documents is the window's single tab group.
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

// DocumentInfo is one open document in a workspace. ID is stable across
// path, tab and window.
type DocumentInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Edited     bool   `json:"edited"`
	Conflict   bool   `json:"conflict"`
	Deleted    bool   `json:"deleted"`
	ReadOnly   bool   `json:"readOnly"`
	Generating bool   `json:"generating"`
	ViewMode   string `json:"viewMode,omitempty"`
	// ActionError is why the last native command on this document failed,
	// shown in its window. Empty when there is none.
	ActionError  string `json:"actionError,omitempty"`
	RestoreError string `json:"restoreError,omitempty"`
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
	Attempt int    `json:"attempt,omitempty"`
}

var (
	workspaceChanged = mygo.NewEvent[WorkspaceSnapshot]("workspaceChanged")
	nativeCommand    = mygo.NewEvent[NativeCommand]("nativeCommand")
)

// defaultWindowWidth and defaultWindowHeight size a new workspace window
// in logical points. They are the window MyGo creates for every workspace:
// new folders, restored sessions and the Welcome window alike. A window's
// geometry is not stored in the app session, so these values never override
// a size the user chose at runtime; MyGo's own StateKey persistence stays
// unused.
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

// workspace is one root's navigation and tab ownership. Several workspace
// values may share win; byWindow points at the one whose navigation is shown.
type workspace struct {
	id           string
	windowID     string
	root         string // empty = untitled workspace
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

// canonicalPath is an absolute path with existing symlink prefixes
// resolved. Alias roots such as macOS /tmp versus /private/tmp compare
// equal so Open reuses the workspace and keeps documentId.
func canonicalPath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return resolveToolPath(abs, abs)
}

func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if ca, cb := canonicalPath(a), canonicalPath(b); ca != "" && ca == cb {
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

func sameRoot(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return canonicalPath(a) == canonicalPath(b)
}

func folderContains(root, file string) bool {
	if root == "" || file == "" {
		return false
	}
	root = canonicalPath(root)
	file = canonicalPath(file)
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func deepestRoot(file string, roots []string) string {
	var best, bestCanon string
	for _, root := range roots {
		if !folderContains(root, file) {
			continue
		}
		canon := canonicalPath(root)
		if best == "" || len(canon) > len(bestCanon) {
			best = root
			bestCanon = canon
		}
	}
	return best
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

// workspaceByIDLocked also supports the small hand-built hosts in tests that
// predate byWorkspace; production registers every root there.
func (h *documentHost) workspaceByIDLocked(id string) *workspace {
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

// allWorkspacesLocked returns roots in their stable window/tab order.
func (h *documentHost) allWorkspacesLocked() []*workspace {
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

func (h *documentHost) windowWorkspacesLocked(win *mygo.Window) []*workspace {
	out := make([]*workspace, 0, 1)
	for _, w := range h.allWorkspacesLocked() {
		if w.win == win {
			out = append(out, w)
		}
	}
	return out
}

func (h *documentHost) preferredWindow() *mygo.Window {
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

func (h *documentHost) docSession(id string) (*docSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.byID[id]
	if s == nil {
		return nil, errNoDocument
	}
	return s, nil
}

func (h *documentHost) snapshotWorkspace(win *mygo.Window) (WorkspaceSnapshot, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotWorkspaceLocked(win)
}

func (h *documentHost) snapshotWorkspaceLocked(win *mygo.Window) (WorkspaceSnapshot, error) {
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

func (h *documentHost) emitWorkspace(w *workspace) {
	if w == nil {
		return
	}
	// Queue snapshots in the same order that the host reads the selection.
	h.mu.Lock()
	defer h.mu.Unlock()
	win := w.win
	if win == nil || win.IsDestroyed() {
		return
	}
	if snap, err := h.snapshotWorkspaceLocked(win); err == nil {
		_ = workspaceChanged.Emit(win, snap)
	}
}

// Snapshot is the calling window's workspace.
func (h *documentHost) Snapshot(ctx context.Context) (WorkspaceSnapshot, error) {
	win := mygo.CallerWindow(ctx)
	if win == nil {
		return WorkspaceSnapshot{}, errNoWorkspace
	}
	return h.snapshotWorkspace(win)
}

// Open opens path with global routing: existing session, else deepest
// open folder workspace, else a workspace at the parent directory.
func (h *documentHost) Open(ctx context.Context, path string) (DocumentInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	abs = canonicalPath(abs)
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

// OpenWorkspace selects an existing root or adds it to the calling window.
// A new native window is created only when the app has no window to reuse.
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
	abs = canonicalPath(abs)
	h.mu.Lock()
	for _, w := range h.allWorkspacesLocked() {
		if w.root != "" && sameRoot(w.root, abs) {
			h.mu.Unlock()
			h.activateWorkspace(w)
			w.win.Show()
			w.win.Focus()
			h.rememberRoot(abs)
			return nil
		}
	}
	h.mu.Unlock()
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

// ActivateWorkspace selects a root's navigation and its most recent document.
func (h *documentHost) ActivateWorkspace(ctx context.Context, workspaceID string) error {
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

func (h *documentHost) activateWorkspace(w *workspace) {
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
		applyNativeEditMenu(false, false)
	}
	h.emitWorkspace(w)
	h.persistSession()
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
	h.noteAction(s, err)
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
		h.byWindow[s.win] = s.ws
	}
	w := s.ws
	h.mu.Unlock()
	h.emitWorkspace(w)
	h.persistSession()
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
		h.noteAction(s, nil)
		return CloseResult{Canceled: true}, nil
	}
	if err != nil {
		h.noteAction(s, err)
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

// CloseWorkspaceRoot closes only one root's documents and removes its
// navigation. The native window and other roots stay open. Closing the last
// root leaves the existing window at Welcome.
func (h *documentHost) CloseWorkspaceRoot(ctx context.Context, workspaceID string) (CloseResult, error) {
	caller, callerErr := h.workspaceOf(ctx)
	h.mu.Lock()
	w := h.workspaceByIDLocked(workspaceID)
	h.mu.Unlock()
	if w == nil || (callerErr == nil && caller.win != w.win) {
		return CloseResult{}, errNoWorkspace
	}
	var docs []*mygo.Document[string]
	for _, id := range w.tabs {
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

func (h *documentHost) removeWorkspaceRoot(w *workspace) {
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

func (h *documentHost) interceptCommand(role mygo.MenuRole, doc *mygo.Document[string], win *mygo.Window) bool {
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
		// Selecting an empty root must not leave File/Edit commands aimed at
		// the previously active document in another root.
		return true
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
	if active != documentID {
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
	h.mu.Lock()
	skip := h.quitting || h.sessionReady
	h.mu.Unlock()
	if skip {
		return false
	}
	_ = nativeCommand.Emit(win, NativeCommand{Command: "closeWorkspace"})
	return true
}

func (h *documentHost) newWorkspaceWindow(root, title string) *mygo.Window {
	if root != "" {
		root = canonicalPath(root)
	}
	return h.openWorkspaceWindow("", root, title)
}

func (h *documentHost) openWorkspaceWindow(id, root, title string) *mygo.Window {
	if root != "" {
		root = canonicalPath(root)
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
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     title,
		URL:       "/",
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

// addWorkspaceToWindow registers one root without creating another native
// window. The first root also supplies the stable window identity.
func (h *documentHost) addWorkspaceToWindow(id, root, title string, win *mygo.Window) *workspace {
	if root != "" {
		root = canonicalPath(root)
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
		// Opening the first folder replaces an empty Welcome root instead of
		// leaving a meaningless second workspace in the switcher.
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

func (h *documentHost) forgetWindow(win *mygo.Window) {
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
			h.dropSession(s)
		}
	}
	h.persistSession()
	if done {
		h.completeQuitAttempt(attempt, file, win)
	}
}
