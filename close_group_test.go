package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// Closing one document runs the session drop below (OnClosed): a folder
// workspace keeps its registration, its remaining tabs and its session
// entry. It does not press the native menu: no headless seam reaches a
// menu key equivalent, so the Cmd+W route itself is verified on the real
// app. Nor can a window made for a test show a real close: its destroy is
// a no-op without a native handle, so window survival is the native run's
// claim, not this test's.
func TestClosingOneDocumentKeepsFolderWindowTabsAndSession(t *testing.T) {
	root := t.TempDir()
	h := &documentHost{
		byWindow: map[*mygo.Window]*workspace{},
		byID:     map[string]*docSession{},
	}
	win := &mygo.Window{}
	w := &workspace{id: "genui-ws", root: root, win: win, tabs: []string{"g1", "g2", "u"}, active: "u"}
	h.byWindow[win] = w
	h.workspaceOrder = []string{w.id}
	for id, path := range map[string]string{
		"g1": filepath.Join(root, "g1.md"),
		"g2": filepath.Join(root, "g2.md"),
		"u":  "",
	} {
		p := path
		h.byID[id] = &docSession{
			host:   h,
			id:     id,
			win:    win,
			ws:     w,
			pathFn: func() string { return p },
		}
	}

	h.dropSession(h.byID["u"])

	if len(w.tabs) != 2 || w.tabs[0] != "g1" || w.tabs[1] != "g2" {
		t.Fatalf("tabs after closing the untitled = %v, want the folder's own tabs", w.tabs)
	}
	if w.active != "g2" {
		t.Fatalf("active after closing the untitled = %q, want the last remaining tab", w.active)
	}
	if h.byWindow[win] != w {
		t.Fatal("the folder workspace must stay registered after one tab closes")
	}
	if _, ok := h.byID["u"]; ok {
		t.Fatal("the closed session must be gone")
	}

	h.mu.Lock()
	file := h.captureSessionLocked()
	h.mu.Unlock()
	if len(file.Workspaces) != 1 || file.Workspaces[0].ID != w.id || file.Workspaces[0].Root != root {
		t.Fatalf("session after closing one tab = %+v, want the folder workspace", file.Workspaces)
	}
	tabs := file.Workspaces[0].Tabs
	if len(tabs) != 2 || tabs[0].ID != "g1" || tabs[1].ID != "g2" || file.Workspaces[0].ActiveID != "g2" {
		t.Fatalf("session tabs = %+v active %q, want g1 and g2", tabs, file.Workspaces[0].ActiveID)
	}
}
