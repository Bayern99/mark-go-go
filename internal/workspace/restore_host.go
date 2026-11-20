package workspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
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

type resolvedTab struct {
	planned string
	id      string
}

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

func (h *Service) SetViewMode(ctx context.Context, documentID, mode string) error {
	_ = ctx
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	h.mu.Lock()
	s.viewMode = normalizeViewMode(mode)
	w := s.ws
	h.mu.Unlock()
	h.persistSession()
	h.emitWorkspace(w)
	return nil
}

const quitAckTimeout = 20 * time.Second

var errStaleQuitAttempt = errors.New("stale quit attempt")

func (h *Service) FlushQuit(ctx context.Context, documentID, text string, attempt int) error {
	win := desktop.Window(ctx)
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
	s.doc.Set(text)
	h.mu.Unlock()
	h.reindexOpenDocument(s)
	return nil
}

func (h *Service) PrepareQuit(ctx context.Context, attempt int, failure string) error {
	win := desktop.Window(ctx)
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

func (h *Service) beginQuitAttempt() (int, []*mygo.Window, bool) {
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

func (h *Service) ackQuitWindow(win *mygo.Window, attempt int) (done bool, file sessionFileData) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ackQuitWindowLocked(win, attempt)
}

func (h *Service) ackQuitWindowLocked(win *mygo.Window, attempt int) (done bool, file sessionFileData) {
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

func (h *Service) saveQuitAttempt(
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

func (h *Service) completeQuitAttempt(attempt int, file sessionFileData, win *mygo.Window) {
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

func (h *Service) abortQuitAttempt(attempt int, win *mygo.Window, allowSaving bool) bool {
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

func (h *Service) failQuitAttempt(attempt int, win *mygo.Window, err error, allowSaving, persist bool) {
	if !h.abortQuitAttempt(attempt, win, allowSaving) {
		return
	}
	if persist {
		h.persistSession()
	}
	h.showQuitFailure(win, err)
}

func (h *Service) showQuitFailure(win *mygo.Window, err error) {
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

func (h *Service) OnBeforeQuit(e *mygo.QuitEvent) {
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
		if err := CommandRequested.Emit(win, NativeCommand{Attempt: attempt, Command: "flushQuit"}); err != nil {
			h.failQuitAttempt(attempt, win, fmt.Errorf("could not ask a window to flush: %w", err), false, true)
			return
		}
	}
}

func (h *Service) RestoreAfterStart() {
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
	if plan.ShowWelcome || empty {
		h.showWelcomeIfEmpty()
	}
}

func (h *Service) applyRestorePlan(plan restorePlan) {
	restoreWin := h.preferredWindow()
	restoredRoots := map[*workspace]bool{}
	var restoredOrder []string
	var activeWorkspace *workspace
	for _, pw := range plan.Workspaces {
		dest := h.workspaceForPlan(pw, restoreWin)
		if dest == nil {
			continue
		}
		if restoreWin == nil {
			restoreWin = dest.win
		}
		restoredRoots[dest] = true
		restoredOrder = append(restoredOrder, dest.id)
		if plan.ActiveWorkspaceID != "" && pw.ID == plan.ActiveWorkspaceID {
			activeWorkspace = dest
		}
		h.mu.Lock()
		dest.restoreError = pw.RestoreError
		h.mu.Unlock()
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
	h.dropRestoreLeftovers(restoredRoots)
	if len(restoredOrder) > 0 {
		h.mu.Lock()
		h.workspaceOrder = orderedTabs(restoredOrder, h.workspaceOrder)
		if activeWorkspace == nil {
			activeWorkspace = h.byWindow[restoreWin]
		}
		h.mu.Unlock()
		h.activateWorkspace(activeWorkspace)
	}
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

func (h *Service) dropRestoreLeftovers(restoredRoots map[*workspace]bool) {
	h.mu.Lock()
	emptyRoots := map[*mygo.Window][]*workspace{}
	keepWindow := map[*mygo.Window]bool{}
	for _, w := range h.allWorkspacesLocked() {
		win := w.win
		if win == nil || win.IsDestroyed() {
			continue
		}
		if w.root != "" && len(w.tabs) == 0 && !restoredRoots[w] {
			emptyRoots[win] = append(emptyRoots[win], w)
		} else {
			keepWindow[win] = true
		}
	}
	h.mu.Unlock()
	for win, roots := range emptyRoots {
		if !keepWindow[win] {
			win.Destroy()
			continue
		}
		for _, w := range roots {
			h.removeWorkspaceRoot(w)
		}
	}
}

func (h *Service) workspaceForPlan(pw plannedWorkspace, preferred *mygo.Window) *workspace {
	if pw.Root != "" {
		type rootCandidate struct {
			ws   *workspace
			root string
		}
		h.mu.Lock()
		all := h.allWorkspacesLocked()
		candidates := make([]rootCandidate, 0, len(all))
		for _, w := range all {
			if w.root != "" && w.win != nil && !w.win.IsDestroyed() {
				candidates = append(candidates, rootCandidate{ws: w, root: w.root})
			}
		}
		h.mu.Unlock()
		for _, c := range candidates {
			if fsutil.SameRoot(c.root, pw.Root) {
				return c.ws
			}
		}
		if preferred != nil && !preferred.IsDestroyed() {
			return h.addWorkspaceToWindow(pw.ID, pw.Root, filepath.Base(pw.Root), preferred)
		}
		win := h.openWorkspaceWindow(pw.ID, pw.Root, filepath.Base(pw.Root))
		h.mu.Lock()
		w := h.byWindow[win]
		h.mu.Unlock()
		return w
	}
	if preferred != nil && !preferred.IsDestroyed() {
		h.mu.Lock()
		for _, w := range h.windowWorkspacesLocked(preferred) {
			if w.root == "" {
				h.mu.Unlock()
				return w
			}
		}
		h.mu.Unlock()
		return h.addWorkspaceToWindow(pw.ID, "", "Untitled", preferred)
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
	for _, w := range h.allWorkspacesLocked() {
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
