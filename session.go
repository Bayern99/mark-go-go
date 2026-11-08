package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// session.json in PathUserData: workspaces the user did not explicitly
// close. Untitled empty Welcome windows are not stored. MyGo recovery
// copies remain the authority for unsaved bytes.
const sessionFile = "session.json"

// sessionFileData is the on-disk session.
type sessionFileData struct {
	Workspaces []sessionWorkspace `json:"workspaces"`
}

// sessionWorkspace is one window to restore.
type sessionWorkspace struct {
	ID       string       `json:"id"`
	Root     string       `json:"root"`
	Tabs     []sessionTab `json:"tabs"`
	ActiveID string       `json:"activeId"`
}

// sessionTab is one document in tab order.
type sessionTab struct {
	ID       string `json:"id"`
	Path     string `json:"path,omitempty"`
	ViewMode string `json:"viewMode,omitempty"`
}

// restoreLive is what recovery and this launch already opened.
type restoreLive struct {
	OpenByID   map[string]restoreDoc
	OpenByPath map[string]restoreDoc
	RootOK     func(string) bool
	FileOK     func(string) bool
	Launch     []string
}

type restoreDoc struct {
	ID   string
	Path string
}

// restorePlan is the public restore seam: layout, reuse vs open, the
// problems the windows report, and launch focus. It never starts a
// generation.
type restorePlan struct {
	Workspaces  []plannedWorkspace
	FocusID     string
	FocusPath   string
	ShowWelcome bool
}

type plannedWorkspace struct {
	ID           string
	Root         string
	Kind         string
	RestoreError string
	Tabs         []plannedTab
	ActiveID     string
}

type plannedTab struct {
	ID           string
	Path         string
	ViewMode     string
	ReuseID      string // already-open document to keep
	OpenPath     string // disk path to Open when ReuseID is empty
	RestoreError string
}

func (st *settingsStore) sessionPath() string {
	if st == nil || st.dir == "" {
		return ""
	}
	return filepath.Join(st.dir, sessionFile)
}

func (st *settingsStore) loadSession() sessionFileData {
	var zero sessionFileData
	if st == nil {
		return zero
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	path := st.sessionPath()
	if path == "" {
		return zero
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return zero
	}
	var file sessionFileData
	if json.Unmarshal(data, &file) != nil {
		return zero
	}
	out := sessionFileData{Workspaces: make([]sessionWorkspace, 0, len(file.Workspaces))}
	for _, w := range file.Workspaces {
		tabs := make([]sessionTab, 0, len(w.Tabs))
		for _, tab := range w.Tabs {
			if tab.ID == "" && tab.Path == "" {
				continue
			}
			tabs = append(tabs, tab)
		}
		out.Workspaces = append(out.Workspaces, sessionWorkspace{
			ID:       w.ID,
			Root:     w.Root,
			Tabs:     tabs,
			ActiveID: w.ActiveID,
		})
	}
	return out
}

func (st *settingsStore) saveSession(file sessionFileData) error {
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	path := st.sessionPath()
	if path == "" {
		return nil
	}
	if file.Workspaces == nil {
		file.Workspaces = []sessionWorkspace{}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

func sessionPathKey(path string) string {
	if path == "" {
		return ""
	}
	return canonicalPath(path)
}

func missingRootMessage(root string) string {
	return fmt.Sprintf("The folder %s is missing or unreadable.", root)
}

func missingFileMessage(path string) string {
	return fmt.Sprintf("The file %s is missing or unreadable.", path)
}

func untitledLostMessage() string {
	return "An untitled document could not be restored."
}

// appendRestoreMessage joins a new problem to what the window reports,
// so one banner explains every tab that could not come back.
func appendRestoreMessage(have, add string) string {
	if add == "" {
		return have
	}
	if have == "" {
		return add
	}
	return have + " " + add
}

// orderedTabs is the planned tab order followed by any live tab the plan
// did not name: a restore never hides a document that is still open.
func orderedTabs(planned, live []string) []string {
	out := make([]string, 0, len(planned)+len(live))
	seen := make(map[string]bool, len(planned)+len(live))
	for _, id := range append(append([]string{}, planned...), live...) {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// planSessionRestore turns a stored session and live recovery/launch docs
// into windows and tabs. A document appears in at most one tab. Missing
// roots or files become issues; other workspaces continue. Empty folder
// workspaces are kept. Untitled empty windows are not planned (Welcome).
func planSessionRestore(file sessionFileData, live restoreLive) restorePlan {
	if live.OpenByID == nil {
		live.OpenByID = map[string]restoreDoc{}
	}
	if live.OpenByPath == nil {
		live.OpenByPath = map[string]restoreDoc{}
	}
	if live.RootOK == nil {
		live.RootOK = func(string) bool { return true }
	}
	if live.FileOK == nil {
		live.FileOK = func(p string) bool {
			if p == "" {
				return false
			}
			_, err := os.Stat(p)
			return err == nil
		}
	}

	if len(file.Workspaces) == 0 {
		return restorePlan{ShowWelcome: len(live.OpenByID) == 0 && len(live.Launch) == 0}
	}

	usedID := map[string]bool{}
	claimedPath := map[string]bool{}
	plan := restorePlan{Workspaces: make([]plannedWorkspace, 0, len(file.Workspaces))}

	for _, w := range file.Workspaces {
		pw := plannedWorkspace{
			ID:   w.ID,
			Root: w.Root,
			Kind: "folder",
		}
		if w.Root == "" {
			pw.Kind = "untitled"
		} else if !live.RootOK(w.Root) {
			pw.RestoreError = missingRootMessage(w.Root)
		}

		for _, tab := range w.Tabs {
			pt := plannedTab{ID: tab.ID, Path: tab.Path, ViewMode: tab.ViewMode}
			if tab.ID != "" {
				if doc, ok := live.OpenByID[tab.ID]; ok && !usedID[doc.ID] {
					pt.ReuseID = doc.ID
					if pt.Path == "" {
						pt.Path = doc.Path
					}
					usedID[doc.ID] = true
					if k := sessionPathKey(doc.Path); k != "" {
						claimedPath[k] = true
					}
					if pt.Path != "" && !live.FileOK(pt.Path) {
						// The recovery copy holds the bytes; the file behind
						// this tab is gone, so say so on the tab.
						pt.RestoreError = missingFileMessage(pt.Path)
					}
					pw.Tabs = append(pw.Tabs, pt)
					continue
				}
			}
			key := sessionPathKey(tab.Path)
			if key != "" {
				if doc, ok := live.OpenByPath[key]; ok && !usedID[doc.ID] {
					pt.ReuseID = doc.ID
					pt.ID = doc.ID
					usedID[doc.ID] = true
					claimedPath[key] = true
					pw.Tabs = append(pw.Tabs, pt)
					continue
				}
				if !claimedPath[key] && live.FileOK(tab.Path) {
					claimedPath[key] = true
					pt.OpenPath = tab.Path
					pw.Tabs = append(pw.Tabs, pt)
					continue
				}
				pw.RestoreError = appendRestoreMessage(pw.RestoreError, missingFileMessage(tab.Path))
				continue
			}
			pw.RestoreError = appendRestoreMessage(pw.RestoreError, untitledLostMessage())
		}

		if w.ActiveID != "" {
			for _, tab := range pw.Tabs {
				id := tab.ReuseID
				if id == "" {
					id = tab.ID
				}
				if id == w.ActiveID || tab.ID == w.ActiveID {
					pw.ActiveID = id
					break
				}
			}
		}
		if pw.ActiveID == "" && len(pw.Tabs) > 0 {
			id := pw.Tabs[0].ReuseID
			if id == "" {
				id = pw.Tabs[0].ID
			}
			pw.ActiveID = id
		}

		// Drop vanished untitled windows that have nothing left.
		if pw.Kind == "untitled" && len(pw.Tabs) == 0 {
			continue
		}
		plan.Workspaces = append(plan.Workspaces, pw)
	}

	for _, launch := range live.Launch {
		key := sessionPathKey(launch)
		if key == "" {
			continue
		}
		plan.FocusPath = launch
		if doc, ok := live.OpenByPath[key]; ok {
			plan.FocusID = doc.ID
		}
		break
	}
	plan.ShowWelcome = len(plan.Workspaces) == 0 && len(live.OpenByID) == 0 && len(live.Launch) == 0
	return plan
}

func (h *documentHost) captureSessionLocked() sessionFileData {
	file := sessionFileData{Workspaces: []sessionWorkspace{}}
	seen := map[string]bool{}
	appendWS := func(w *workspace) {
		if w == nil || seen[w.id] {
			return
		}
		if w.win != nil && w.win.IsDestroyed() {
			return
		}
		// Untitled empty is Welcome, not a restored workspace.
		if w.root == "" && len(w.tabs) == 0 {
			return
		}
		seen[w.id] = true
		sw := sessionWorkspace{ID: w.id, Root: w.root, ActiveID: w.active, Tabs: []sessionTab{}}
		for _, id := range w.tabs {
			s := h.byID[id]
			if s == nil {
				continue
			}
			sw.Tabs = append(sw.Tabs, sessionTab{
				ID:       s.id,
				Path:     s.currentPath(),
				ViewMode: s.viewMode,
			})
		}
		file.Workspaces = append(file.Workspaces, sw)
	}
	for _, id := range h.workspaceOrder {
		for _, w := range h.byWindow {
			if w != nil && w.id == id {
				appendWS(w)
				break
			}
		}
	}
	for _, w := range h.byWindow {
		appendWS(w)
	}
	return file
}

func (h *documentHost) persistSession() {
	if h == nil {
		return
	}
	h.mu.Lock()
	if h.quitting || h.restoring || h.sessionReady || h.quitPending != nil {
		h.mu.Unlock()
		return
	}
	file := h.captureSessionLocked()
	st := h.settings
	h.mu.Unlock()
	if st != nil {
		_ = st.saveSession(file)
	}
}

func (h *documentHost) liveRestoreState(launch []string) restoreLive {
	live := restoreLive{
		OpenByID:   map[string]restoreDoc{},
		OpenByPath: map[string]restoreDoc{},
		Launch:     launch,
		RootOK: func(root string) bool {
			if root == "" {
				return true
			}
			info, err := os.Stat(root)
			return err == nil && info.IsDir()
		},
		FileOK: func(path string) bool {
			if path == "" {
				return false
			}
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.byID {
		if s == nil {
			continue
		}
		doc := restoreDoc{ID: id, Path: s.currentPath()}
		live.OpenByID[id] = doc
		if key := sessionPathKey(doc.Path); key != "" {
			live.OpenByPath[key] = doc
		}
	}
	return live
}

func launchPathsFromArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		abs, err := filepath.Abs(a)
		if err != nil {
			continue
		}
		switch ext := strings.ToLower(filepath.Ext(abs)); ext {
		case ".md", ".markdown":
			out = append(out, abs)
		}
	}
	return out
}
