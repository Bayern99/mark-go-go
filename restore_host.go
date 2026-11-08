package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

func appendWorkspaceOrder(order []string, id string) []string {
	for _, existing := range order {
		if existing == id {
			return order
		}
	}
	return append(order, id)
}

func removeWorkspaceOrder(order []string, id string) []string {
	out := order[:0]
	for _, existing := range order {
		if existing != id {
			out = append(out, existing)
		}
	}
	return out
}

// SetViewMode stores this document's view mode in the session. Bound to
// documentID, never the active tab.
func (h *documentHost) SetViewMode(ctx context.Context, documentID, mode string) error {
	_ = ctx
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	h.mu.Lock()
	s.viewMode = mode
	w := s.ws
	h.mu.Unlock()
	h.persistSession()
	h.emitWorkspace(w)
	return nil
}

// PrepareQuit records that the calling window flushed for quit. The
// session is written only after every workspace window has acked, then
// quit continues. Close events after that must not rewrite an empty
// layout over the saved session.
func (h *documentHost) PrepareQuit(ctx context.Context) error {
	done, file := h.ackQuitWindow(mygo.CallerWindow(ctx))
	if !done {
		return nil
	}
	h.persistQuitAndRequit(file)
	return nil
}

func (h *documentHost) persistQuitAndRequit(file sessionFileData) {
	if h.settings != nil {
		_ = h.settings.saveSession(file)
	}
	go mygo.App.Quit()
}

// ackQuitWindow marks win flushed. done is true only when every pending
// workspace window has acked (or none were pending).
func (h *documentHost) ackQuitWindow(win *mygo.Window) (done bool, file sessionFileData) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ackQuitWindowLocked(win)
}

func (h *documentHost) ackQuitWindowLocked(win *mygo.Window) (done bool, file sessionFileData) {
	if h.sessionReady {
		return false, sessionFileData{}
	}
	if h.quitPending != nil {
		if win != nil {
			delete(h.quitPending, win)
		}
		if len(h.quitPending) > 0 {
			return false, sessionFileData{}
		}
	}
	file = h.captureSessionLocked()
	h.sessionReady = true
	h.quitting = true
	h.quitPending = nil
	return true, file
}

func (h *documentHost) onBeforeQuit(e *mygo.QuitEvent) {
	h.mu.Lock()
	if h.sessionReady {
		h.mu.Unlock()
		return
	}
	if h.quitPending != nil {
		h.mu.Unlock()
		e.PreventDefault()
		return
	}
	pending := make(map[*mygo.Window]bool, len(h.byWindow))
	wins := make([]*mygo.Window, 0, len(h.byWindow))
	for win := range h.byWindow {
		if win != nil && !win.IsDestroyed() {
			pending[win] = true
			wins = append(wins, win)
		}
	}
	h.quitPending = pending
	h.mu.Unlock()
	e.PreventDefault()
	if len(wins) == 0 {
		done, file := h.ackQuitWindow(nil)
		if done {
			h.persistQuitAndRequit(file)
		}
		return
	}
	for _, win := range wins {
		_ = nativeCommand.Emit(win, NativeCommand{Command: "flushQuit"})
	}
}

// restoreAfterStart lays out workspaces after MyGo recovery and launch
// files. It does not start generations.
func (h *documentHost) restoreAfterStart() {
	h.mu.Lock()
	h.restoring = true
	h.mu.Unlock()
	var file sessionFileData
	if h.settings != nil {
		file = h.settings.loadSession()
	}
	plan := planSessionRestore(file, h.liveRestoreState(launchPathsFromArgs(os.Args[1:])))
	h.applyRestorePlan(plan)
	h.mu.Lock()
	empty := len(h.byWindow) == 0
	h.restoring = false
	h.mu.Unlock()
	// The stored session had nothing to restore, or every window it
	// planned failed: say Welcome. A launch file that could not be
	// opened must not leave the app without a window either.
	if plan.ShowWelcome || empty {
		h.showWelcomeIfEmpty()
	}
}

func (h *documentHost) applyRestorePlan(plan restorePlan) {
	plannedRoots := map[string]bool{}
	for _, pw := range plan.Workspaces {
		if pw.Root != "" {
			plannedRoots[canonicalPath(pw.Root)] = true
		}
		dest := h.workspaceForPlan(pw)
		if dest == nil {
			continue
		}
		h.mu.Lock()
		dest.restoreError = pw.RestoreError
		h.mu.Unlock()
		var ordered []string
		for _, tab := range pw.Tabs {
			var s *docSession
			if tab.ReuseID != "" {
				s, _ = h.docSession(tab.ReuseID)
			} else if tab.OpenPath != "" && h.docs != nil {
				info, err := h.Open(context.Background(), tab.OpenPath)
				if err != nil {
					h.mu.Lock()
					dest.restoreError = appendRestoreMessage(dest.restoreError, missingFileMessage(tab.OpenPath))
					h.mu.Unlock()
					continue
				}
				s, _ = h.docSession(info.ID)
			}
			if s == nil {
				continue
			}
			h.mu.Lock()
			s.viewMode = tab.ViewMode
			s.restoreError = tab.RestoreError
			h.mu.Unlock()
			h.moveTab(s, dest)
			ordered = append(ordered, s.id)
		}
		active := pw.ActiveID
		for _, tab := range pw.Tabs {
			id := tab.ReuseID
			if id == "" {
				id = tab.ID
			}
			if tab.ID == pw.ActiveID || id == pw.ActiveID {
				active = id
				break
			}
		}
		h.mu.Lock()
		if len(ordered) > 0 {
			// A recovery copy the session did not list is still open;
			// keep its tab instead of hiding unsaved bytes.
			dest.tabs = orderedTabs(ordered, dest.tabs)
		}
		if active != "" {
			dest.active = active
		} else if len(dest.tabs) > 0 {
			dest.active = dest.tabs[0]
		}
		h.mu.Unlock()
		if active != "" {
			if s, err := h.docSession(active); err == nil {
				h.activate(s)
				continue
			}
		}
		h.emitWorkspace(dest)
	}
	h.dropRestoreLeftovers(plannedRoots)
	// A file this launch opened gets focus after the layout is in place.
	if plan.FocusPath != "" && h.docs != nil {
		if info, err := h.Open(context.Background(), plan.FocusPath); err == nil {
			_ = h.Activate(context.Background(), info.ID)
		}
		return
	}
	if plan.FocusID != "" {
		_ = h.Activate(context.Background(), plan.FocusID)
	}
}

// dropRestoreLeftovers closes the folder windows recovery made for files
// that belong to a planned workspace: their tabs moved away, so nothing
// is left to show. Planned folders, even empty ones, stay.
func (h *documentHost) dropRestoreLeftovers(plannedRoots map[string]bool) {
	h.mu.Lock()
	var stale []*mygo.Window
	for win, w := range h.byWindow {
		if w.root == "" || len(w.tabs) > 0 || plannedRoots[canonicalPath(w.root)] {
			continue
		}
		if win != nil && !win.IsDestroyed() {
			stale = append(stale, win)
		}
	}
	h.mu.Unlock()
	for _, win := range stale {
		win.Destroy()
	}
}

func (h *documentHost) workspaceForPlan(pw plannedWorkspace) *workspace {
	if pw.Root != "" {
		h.mu.Lock()
		for _, w := range h.byWindow {
			if w.root != "" && sameRoot(w.root, pw.Root) && w.win != nil && !w.win.IsDestroyed() {
				h.mu.Unlock()
				return w
			}
		}
		h.mu.Unlock()
		win := h.openWorkspaceWindow(pw.ID, pw.Root, filepath.Base(pw.Root))
		h.mu.Lock()
		w := h.byWindow[win]
		h.mu.Unlock()
		return w
	}
	h.mu.Lock()
	for _, tab := range pw.Tabs {
		if tab.ReuseID == "" {
			continue
		}
		if s := h.byID[tab.ReuseID]; s != nil && s.ws != nil && s.ws.win != nil && !s.ws.win.IsDestroyed() {
			w := s.ws
			h.mu.Unlock()
			return w
		}
	}
	for _, w := range h.byWindow {
		if w.root == "" && w.win != nil && !w.win.IsDestroyed() {
			h.mu.Unlock()
			return w
		}
	}
	h.mu.Unlock()
	win := h.openWorkspaceWindow(pw.ID, "", "Untitled")
	h.mu.Lock()
	w := h.byWindow[win]
	h.mu.Unlock()
	return w
}
