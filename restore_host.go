package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

// resolvedTab is a planned tab with the id its document has after
// restore: a file opened from disk gets a new MyGo id, a recovery copy
// keeps the id the session stored.
type resolvedTab struct {
	planned string
	id      string
}

// activeAfterRestore picks the window's active document: the real id of
// the tab the session named active, else the first tab that came back.
// It never invents an id that no open document has.
func activeAfterRestore(resolved []resolvedTab, want string) string {
	for _, r := range resolved {
		if want != "" && r.planned == want {
			return r.id
		}
	}
	if len(resolved) > 0 {
		return resolved[0].id
	}
	return ""
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

const quitAckTimeout = 20 * time.Second

var errStaleQuitAttempt = errors.New("stale quit attempt")

// FlushQuit delivers the editor text while this window still owes a quit ack.
func (h *documentHost) FlushQuit(ctx context.Context, documentID, text string, attempt int) error {
	win := mygo.CallerWindow(ctx)
	h.mu.Lock()
	if win == nil || attempt != h.quitAttempt || !h.quitPending[win] {
		h.mu.Unlock()
		return errStaleQuitAttempt
	}
	s := h.byID[documentID]
	if s == nil || s.win != win || s.closed {
		h.mu.Unlock()
		return errNoDocument
	}
	// A retry cannot invalidate the attempt between the check and the write.
	s.doc.Set(text)
	h.mu.Unlock()
	h.reindexOpenDocument(s)
	return nil
}

// PrepareQuit records one window's flush, or reports why that window
// could not flush. The session is written only after every window acks.
func (h *documentHost) PrepareQuit(ctx context.Context, attempt int, failure string) error {
	win := mygo.CallerWindow(ctx)
	if failure != "" {
		if win != nil {
			h.failQuitAttempt(attempt, win, errors.New(failure), false, true)
		}
		return nil
	}
	done, file := h.ackQuitWindow(win, attempt)
	if !done {
		return nil
	}
	h.completeQuitAttempt(attempt, file, win)
	return nil
}

func (h *documentHost) beginQuitAttempt() (int, []*mygo.Window, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessionReady || h.quitAttempt != 0 {
		return 0, nil, false
	}
	h.quitSequence++
	if h.quitSequence == 0 {
		h.quitSequence++
	}
	attempt := h.quitSequence
	pending := make(map[*mygo.Window]bool, len(h.byWindow))
	wins := make([]*mygo.Window, 0, len(h.byWindow))
	for win := range h.byWindow {
		if win != nil && !win.IsDestroyed() {
			pending[win] = true
			wins = append(wins, win)
		}
	}
	h.quitAttempt = attempt
	h.quitPending = pending
	h.quitSaving = false
	h.quitTimer = time.AfterFunc(quitAckTimeout, func() {
		h.failQuitAttempt(attempt, nil, errors.New("timed out waiting for documents to finish flushing"), false, true)
	})
	return attempt, wins, true
}

// ackQuitWindow marks win flushed for attempt. It completes only once,
// after every window in that attempt has acked.
func (h *documentHost) ackQuitWindow(win *mygo.Window, attempt int) (done bool, file sessionFileData) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ackQuitWindowLocked(win, attempt)
}

func (h *documentHost) ackQuitWindowLocked(win *mygo.Window, attempt int) (done bool, file sessionFileData) {
	if attempt == 0 || attempt != h.quitAttempt || h.quitPending == nil || h.quitSaving {
		return false, sessionFileData{}
	}
	if win != nil {
		if !h.quitPending[win] {
			return false, sessionFileData{}
		}
		h.quitPending[win] = false
	}
	for _, pending := range h.quitPending {
		if pending {
			return false, sessionFileData{}
		}
	}
	for win := range h.byWindow {
		if _, included := h.quitPending[win]; !included {
			return false, sessionFileData{}
		}
	}
	h.quitSaving = true
	if h.quitTimer != nil {
		h.quitTimer.Stop()
		h.quitTimer = nil
	}
	return true, h.captureSessionLocked()
}

// saveQuitAttempt leaves the attempt active during the disk write. Tests
// inject save so failure and retry state do not open a native dialog.
func (h *documentHost) saveQuitAttempt(
	attempt int,
	file sessionFileData,
	save func(sessionFileData) error,
) (bool, error) {
	if err := save(file); err != nil {
		if !h.abortQuitAttempt(attempt, nil, true) {
			return false, errStaleQuitAttempt
		}
		return false, err
	}
	h.mu.Lock()
	if attempt == 0 || attempt != h.quitAttempt || !h.quitSaving {
		h.mu.Unlock()
		h.persistSession()
		return false, errStaleQuitAttempt
	}
	h.sessionReady = true
	h.quitting = true
	h.quitPending = nil
	h.quitSaving = false
	h.quitAttempt = 0
	h.mu.Unlock()
	return true, nil
}

func (h *documentHost) completeQuitAttempt(attempt int, file sessionFileData, win *mygo.Window) {
	committed, err := h.saveQuitAttempt(attempt, file, h.saveQuitSession)
	if errors.Is(err, errStaleQuitAttempt) {
		return
	}
	if err != nil {
		h.showQuitFailure(win, err)
	} else if committed {
		go mygo.App.Quit()
	}
}

func (h *documentHost) abortQuitAttempt(attempt int, win *mygo.Window, allowSaving bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if attempt == 0 || attempt != h.quitAttempt || h.sessionReady || (h.quitSaving && !allowSaving) {
		return false
	}
	if win != nil && !h.quitPending[win] {
		return false
	}
	if h.quitTimer != nil {
		h.quitTimer.Stop()
		h.quitTimer = nil
	}
	h.quitAttempt = 0
	h.quitPending = nil
	h.quitSaving = false
	h.quitting = false
	return true
}

func (h *documentHost) failQuitAttempt(attempt int, win *mygo.Window, err error, allowSaving, persist bool) {
	if !h.abortQuitAttempt(attempt, win, allowSaving) {
		return
	}
	if persist {
		h.persistSession()
	}
	h.showQuitFailure(win, err)
}

func (h *documentHost) showQuitFailure(win *mygo.Window, err error) {
	if win == nil {
		win = mygo.FocusedWindow()
	}
	detail := "Your documents are still open. Resolve the issue, then choose Quit again."
	if err != nil {
		detail = err.Error() + "\n\n" + detail
	}
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  win,
			Type:    mygo.MessageError,
			Title:   "Could not quit Mark Go Go",
			Message: "Mark Go Go could not safely prepare all open documents for quitting.",
			Detail:  detail,
			Buttons: []string{"OK"},
		})
	}()
}

func (h *documentHost) onBeforeQuit(e *mygo.QuitEvent) {
	attempt, wins, started := h.beginQuitAttempt()
	if !started {
		h.mu.Lock()
		ready := h.sessionReady
		h.mu.Unlock()
		if !ready {
			e.PreventDefault()
		}
		return
	}
	e.PreventDefault()
	if len(wins) == 0 {
		done, file := h.ackQuitWindow(nil, attempt)
		if done {
			h.completeQuitAttempt(attempt, file, nil)
		}
		return
	}
	for _, win := range wins {
		if err := nativeCommand.Emit(win, NativeCommand{Attempt: attempt, Command: "flushQuit"}); err != nil {
			h.failQuitAttempt(attempt, win, fmt.Errorf("could not ask a window to flush: %w", err), false, true)
			return
		}
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
		// Stored ids need not survive: a tab opened from disk gets the
		// MyGo document's new id, so keep each planned id next to the id
		// the document really has now.
		var resolved []resolvedTab
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
			resolved = append(resolved, resolvedTab{planned: tab.ID, id: s.id})
		}
		var ordered []string
		for _, r := range resolved {
			ordered = append(ordered, r.id)
		}
		active := activeAfterRestore(resolved, pw.ActiveID)
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
