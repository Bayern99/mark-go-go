package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestSessionStoreRoundTripOrderAndViewMode(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	a := filepath.Join(root, "a.md")
	b := filepath.Join(root, "b.md")
	st := &settingsStore{dir: dir}
	want := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:   "ws-1",
		Root: root,
		Tabs: []sessionTab{
			{ID: "doc-a", Path: a, ViewMode: "markdown"},
			{ID: "doc-b", Path: b, ViewMode: "genui"},
		},
		ActiveID: "doc-b",
	}}}
	if err := st.saveSession(want); err != nil {
		t.Fatal(err)
	}
	got := st.loadSession()
	if len(got.Workspaces) != 1 {
		t.Fatalf("workspaces = %d", len(got.Workspaces))
	}
	w := got.Workspaces[0]
	if w.Root != root || w.ActiveID != "doc-b" {
		t.Fatalf("workspace = %+v", w)
	}
	if len(w.Tabs) != 2 || w.Tabs[0].ID != "doc-a" || w.Tabs[1].ID != "doc-b" {
		t.Fatalf("tabs = %+v", w.Tabs)
	}
	if w.Tabs[0].ViewMode != "markdown" || w.Tabs[1].ViewMode != "genui" {
		t.Fatalf("view modes = %+v", w.Tabs)
	}
	raw, err := os.ReadFile(st.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	var check sessionFileData
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatal(err)
	}
	if len(check.Workspaces) != 1 || check.Workspaces[0].ActiveID != "doc-b" {
		t.Fatalf("disk = %s", raw)
	}
}

func TestSessionSelectedWorkspaceSurvivesRoundTrip(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	file := sessionFileData{Workspaces: []sessionWorkspace{
		{ID: "research", Root: t.TempDir()},
		{ID: "music", Root: t.TempDir(), Active: true},
	}}
	if err := st.saveSession(file); err != nil {
		t.Fatal(err)
	}
	loaded := st.loadSession()
	if len(loaded.Workspaces) != 2 || !loaded.Workspaces[1].Active || loaded.Workspaces[0].Active {
		t.Fatalf("selected root did not round-trip: %+v", loaded.Workspaces)
	}
	plan := planSessionRestore(loaded, restoreLive{})
	if plan.ActiveWorkspaceID != "music" {
		t.Fatalf("restore selected root = %q, want music", plan.ActiveWorkspaceID)
	}
}

func TestRestoreReusesWindowOpenedBeforeSessionLayout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		launch bool
		nested bool
	}{
		{name: "named recovery"},
		{name: "launch file", launch: true},
		{name: "recovery under a nested directory", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanRoot, savedRoot, emptyRoot := t.TempDir(), t.TempDir(), t.TempDir()
			liveRoot := savedRoot
			if tc.nested {
				liveRoot = filepath.Join(savedRoot, "nested")
			}
			path := filepath.Join(liveRoot, "notes.md")
			win := &mygo.Window{}
			liveRootState := &workspace{id: "native-root", root: liveRoot, win: win, tabs: []string{"native-doc"}, active: "native-doc"}
			h := &documentHost{
				byWindow:       map[*mygo.Window]*workspace{win: liveRootState},
				byWorkspace:    map[string]*workspace{liveRootState.id: liveRootState},
				workspaceOrder: []string{liveRootState.id},
				settings:       &settingsStore{dir: t.TempDir()},
				disableWatch:   true,
				restoring:      true,
			}
			doc := &docSession{id: "native-doc", host: h, ws: liveRootState, win: win, pathFn: func() string { return path }}
			h.byID = map[string]*docSession{doc.id: doc}
			storedID := doc.id
			if tc.launch {
				storedID = "previous-doc"
			}
			file := sessionFileData{Workspaces: []sessionWorkspace{
				{ID: "clean", Root: cleanRoot},
				{ID: "saved-root", Root: savedRoot, Tabs: []sessionTab{{ID: storedID, Path: path, ViewMode: "markdown"}}, ActiveID: storedID, Active: true},
				{ID: "empty", Root: emptyRoot},
			}}
			h.applyRestorePlan(planSessionRestore(file, h.liveRestoreState(nil)))
			snap, err := h.snapshotWorkspace(win)
			if err != nil {
				t.Fatal(err)
			}
			if len(h.byWindow) != 1 || len(snap.Workspaces) != 3 {
				t.Fatalf("restore produced %d windows and roots %+v", len(h.byWindow), snap.Workspaces)
			}
			wantRoots := []string{cleanRoot, savedRoot, emptyRoot}
			for i, want := range wantRoots {
				if !sameRoot(snap.Workspaces[i].Root, want) {
					t.Fatalf("root order = %+v, want %v", snap.Workspaces, wantRoots)
				}
			}
			if !sameRoot(snap.Root, savedRoot) || snap.ActiveID != doc.id || doc.win != win || doc.viewMode != "markdown" {
				t.Fatalf("restore lost the live tab or selection: snapshot=%+v, mode=%q", snap, doc.viewMode)
			}
		})
	}
}

func TestSessionStoreEmptyMeansNoWorkspaces(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	if err := st.saveSession(sessionFileData{}); err != nil {
		t.Fatal(err)
	}
	got := st.loadSession()
	if got.Workspaces == nil {
		t.Fatal("saved empty session must round-trip as empty list, not nil-missing")
	}
	if len(got.Workspaces) != 0 {
		t.Fatalf("got %+v", got)
	}
	raw, err := os.ReadFile(st.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"workspaces": []`) {
		t.Fatalf("empty session must be an empty list on disk: %s", raw)
	}
}

func TestSessionFileLayoutKeys(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	st := &settingsStore{dir: dir}
	file := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:       "ws",
		Root:     root,
		Tabs:     []sessionTab{{ID: "doc", Path: filepath.Join(root, "a.md"), ViewMode: "genui"}},
		ActiveID: "doc",
	}}}
	if err := st.saveSession(file); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(st.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	workspaces, ok := doc["workspaces"].([]any)
	if !ok || len(workspaces) != 1 {
		t.Fatalf("session = %s", raw)
	}
	w, ok := workspaces[0].(map[string]any)
	if !ok {
		t.Fatalf("workspace = %v", workspaces[0])
	}
	if _, ok := w["id"]; !ok {
		t.Fatalf("workspace id missing: %v", w)
	}
	if w["root"] != root {
		t.Fatalf("root = %v, want %s", w["root"], root)
	}
	if w["activeId"] != "doc" {
		t.Fatalf("activeId = %v", w["activeId"])
	}
	tabs, ok := w["tabs"].([]any)
	if !ok || len(tabs) != 1 {
		t.Fatalf("tabs = %v", w["tabs"])
	}
	tab, ok := tabs[0].(map[string]any)
	if !ok {
		t.Fatalf("tab = %v", tabs[0])
	}
	if tab["id"] != "doc" || tab["viewMode"] != "genui" {
		t.Fatalf("tab = %v", tab)
	}
	if tab["path"] != filepath.Join(root, "a.md") {
		t.Fatalf("tab path = %v", tab["path"])
	}
}

func TestActiveAfterRestoreUsesTheRealDocumentID(t *testing.T) {
	resolved := []resolvedTab{
		{planned: "saved-alpha", id: "native-alpha"},
		{planned: "saved-beta", id: "native-beta"},
	}
	if got := activeAfterRestore(resolved, "saved-beta"); got != "native-beta" {
		t.Fatalf("active = %q, want the real id of the stored active tab", got)
	}
	if got := activeAfterRestore(resolved, "saved-alpha"); got != "native-alpha" {
		t.Fatalf("active = %q", got)
	}
	if got := activeAfterRestore(resolved, "saved-gone"); got != "native-alpha" {
		t.Fatalf("active = %q, want the first restored tab", got)
	}
	if got := activeAfterRestore(resolved, ""); got != "native-alpha" {
		t.Fatalf("active = %q", got)
	}
	if got := activeAfterRestore(nil, "saved-alpha"); got != "" {
		t.Fatalf("active = %q, want empty", got)
	}
}

func TestOrderedTabsKeepsPlannedOrderThenLive(t *testing.T) {
	got := orderedTabs([]string{"b", "a", "b"}, []string{"a", "c", ""})
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestPlanRestoresTabOrderActiveAndEmptyFolder(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.md")
	b := filepath.Join(root, "b.md")
	if err := os.WriteFile(a, []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("# b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	file := sessionFileData{Workspaces: []sessionWorkspace{
		{
			ID:   "ws-notes",
			Root: root,
			Tabs: []sessionTab{
				{ID: "id-a", Path: a, ViewMode: "markdown"},
				{ID: "id-b", Path: b, ViewMode: "genui"},
			},
			ActiveID: "id-a",
		},
		{ID: "ws-empty", Root: empty, Tabs: nil, ActiveID: ""},
	}}
	plan := planSessionRestore(file, restoreLive{})
	if plan.ShowWelcome {
		t.Fatal("folder session must not fall back to Welcome")
	}
	if len(plan.Workspaces) != 2 {
		t.Fatalf("workspaces = %d", len(plan.Workspaces))
	}
	notes := plan.Workspaces[0]
	if len(notes.Tabs) != 2 || notes.Tabs[0].OpenPath != a || notes.Tabs[1].OpenPath != b {
		t.Fatalf("tabs = %+v", notes.Tabs)
	}
	if notes.Tabs[0].ViewMode != "markdown" || notes.ActiveID != "id-a" {
		t.Fatalf("active/mode = %+v", notes)
	}
	if plan.Workspaces[1].Root != empty || len(plan.Workspaces[1].Tabs) != 0 {
		t.Fatalf("empty folder = %+v", plan.Workspaces[1])
	}
}

func TestPlanDoesNotResurrectClosedWorkspaceOrTab(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep.md")
	if err := os.WriteFile(keep, []byte("k\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:       "ws",
		Root:     root,
		Tabs:     []sessionTab{{ID: "keep", Path: keep}},
		ActiveID: "keep",
	}}}
	plan := planSessionRestore(file, restoreLive{})
	if len(plan.Workspaces) != 1 || len(plan.Workspaces[0].Tabs) != 1 || plan.Workspaces[0].Tabs[0].ID != "keep" {
		t.Fatalf("plan = %+v", plan)
	}
	closed := planSessionRestore(sessionFileData{Workspaces: []sessionWorkspace{}}, restoreLive{})
	if !closed.ShowWelcome || len(closed.Workspaces) != 0 {
		t.Fatalf("explicit empty session = %+v", closed)
	}
}

func TestPlanPartialFailureKeepsRecoverableAndOthers(t *testing.T) {
	goodRoot := t.TempDir()
	good := filepath.Join(goodRoot, "ok.md")
	if err := os.WriteFile(good, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingRoot := filepath.Join(t.TempDir(), "gone")
	missingFile := filepath.Join(goodRoot, "missing.md")
	recoveryID := "rec-untitled"
	file := sessionFileData{Workspaces: []sessionWorkspace{
		{
			ID:   "ws-missing-root",
			Root: missingRoot,
			Tabs: []sessionTab{
				{ID: recoveryID, Path: "", ViewMode: "richtext"},
			},
			ActiveID: recoveryID,
		},
		{
			ID:   "ws-good",
			Root: goodRoot,
			Tabs: []sessionTab{
				{ID: "ok", Path: good},
				{ID: "lost", Path: missingFile},
			},
			ActiveID: "ok",
		},
	}}
	st := &settingsStore{dir: t.TempDir()}
	if err := st.saveSession(file); err != nil {
		t.Fatal(err)
	}
	plan := planSessionRestore(st.loadSession(), restoreLive{
		OpenByID: map[string]restoreDoc{recoveryID: {ID: recoveryID, Path: ""}},
	})
	if len(plan.Workspaces) != 2 {
		t.Fatalf("workspaces = %d, want both", len(plan.Workspaces))
	}
	broken := plan.Workspaces[0]
	if broken.RestoreError == "" || !strings.Contains(broken.RestoreError, missingRoot) {
		t.Fatalf("missing root error = %q", broken.RestoreError)
	}
	if len(broken.Tabs) != 1 || broken.Tabs[0].ReuseID != recoveryID {
		t.Fatalf("recoverable untitled dropped: %+v", broken.Tabs)
	}
	if broken.Tabs[0].ViewMode != "preview" {
		t.Fatalf("legacy view mode restored as %q, want preview", broken.Tabs[0].ViewMode)
	}
	goodWS := plan.Workspaces[1]
	if len(goodWS.Tabs) != 1 || goodWS.Tabs[0].OpenPath != good {
		t.Fatalf("good tab = %+v", goodWS.Tabs)
	}
	if !strings.Contains(goodWS.RestoreError, missingFile) {
		t.Fatalf("window must explain the missing file, got %q", goodWS.RestoreError)
	}
}

func TestPlanKeepsRecoveredDocumentWhoseFileIsGone(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(root, "gone.md")
	file := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:       "ws",
		Root:     root,
		Tabs:     []sessionTab{{ID: "d", Path: gone, ViewMode: "markdown"}},
		ActiveID: "d",
	}}}
	plan := planSessionRestore(file, restoreLive{
		OpenByID: map[string]restoreDoc{"d": {ID: "d", Path: gone}},
	})
	if len(plan.Workspaces) != 1 || len(plan.Workspaces[0].Tabs) != 1 {
		t.Fatalf("plan = %+v", plan.Workspaces)
	}
	tab := plan.Workspaces[0].Tabs[0]
	if tab.ReuseID != "d" {
		t.Fatalf("unsaved bytes must be reused, got %+v", tab)
	}
	if !strings.Contains(tab.RestoreError, gone) {
		t.Fatalf("tab must explain the missing file, got %q", tab.RestoreError)
	}
	if plan.Workspaces[0].ActiveID != "d" {
		t.Fatalf("active = %q", plan.Workspaces[0].ActiveID)
	}
}

func TestPlanDedupsRecoveryAndSessionSamePath(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "notes.md")
	if err := os.WriteFile(p, []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:   "ws",
		Root: root,
		Tabs: []sessionTab{
			{ID: "old", Path: p},
			{ID: "dup", Path: p},
		},
		ActiveID: "old",
	}}}
	plan := planSessionRestore(file, restoreLive{
		OpenByID:   map[string]restoreDoc{"rec-1": {ID: "rec-1", Path: p}},
		OpenByPath: map[string]restoreDoc{sessionPathKey(p): {ID: "rec-1", Path: p}},
	})
	if len(plan.Workspaces) != 1 || len(plan.Workspaces[0].Tabs) != 1 {
		t.Fatalf("tabs = %+v", plan.Workspaces[0].Tabs)
	}
	if plan.Workspaces[0].Tabs[0].ReuseID != "rec-1" {
		t.Fatalf("want recovery id, got %+v", plan.Workspaces[0].Tabs[0])
	}
}

func TestPlanLaunchFileGetsFocus(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.md")
	b := filepath.Join(root, "b.md")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	file := sessionFileData{Workspaces: []sessionWorkspace{{
		ID:       "ws",
		Root:     root,
		Tabs:     []sessionTab{{ID: "a", Path: a}, {ID: "b", Path: b}},
		ActiveID: "a",
	}}}
	plan := planSessionRestore(file, restoreLive{
		OpenByPath: map[string]restoreDoc{sessionPathKey(b): {ID: "b", Path: b}},
		Launch:     []string{b},
	})
	if plan.FocusID != "b" || plan.FocusPath != b {
		t.Fatalf("focus = %q %q", plan.FocusID, plan.FocusPath)
	}
}

func TestPlanMissingSessionKeepsLiveRecoveryWithoutWelcome(t *testing.T) {
	plan := planSessionRestore(sessionFileData{}, restoreLive{
		OpenByID: map[string]restoreDoc{"rec": {ID: "rec", Path: "/tmp/x.md"}},
	})
	if plan.ShowWelcome {
		t.Fatal("recovery live docs must not be replaced by Welcome")
	}
	if len(plan.Workspaces) != 0 {
		t.Fatal("no session must not invent layout windows")
	}
}

func TestPlanTabIsReuseOpenOrReportOnly(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.md")
	recovered := filepath.Join(root, "r.md")
	for _, path := range []string{p, recovered} {
		if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan := planSessionRestore(sessionFileData{Workspaces: []sessionWorkspace{{
		ID: "ws", Root: root, Tabs: []sessionTab{
			{ID: "a", Path: p},
			{ID: "rec", Path: recovered},
			{ID: "gone", Path: filepath.Join(root, "gone.md")},
		},
	}}}, restoreLive{
		OpenByPath: map[string]restoreDoc{
			sessionPathKey(recovered): {ID: "rec", Path: recovered},
		},
	})
	if len(plan.Workspaces) != 1 {
		t.Fatalf("workspaces = %+v", plan.Workspaces)
	}
	tabs := plan.Workspaces[0].Tabs
	if len(tabs) != 2 {
		t.Fatalf("tabs = %+v", tabs)
	}
	actions := 0
	for _, tab := range tabs {
		n := 0
		if tab.ReuseID != "" {
			n++
		}
		if tab.OpenPath != "" {
			n++
		}
		if tab.RestoreError != "" {
			n++
		}
		if n != 1 {
			t.Fatalf("tab must either reuse, open or report one problem: %+v", tab)
		}
		actions++
	}
	if actions != 2 || tabs[0].OpenPath != p || tabs[1].ReuseID != "rec" {
		t.Fatalf("tabs = %+v", tabs)
	}
}

func TestCaptureSessionSkipsWelcomeAndKeepsTabOrder(t *testing.T) {
	root := t.TempDir()
	h := &documentHost{
		byWindow: make(map[*mygo.Window]*workspace),
		byID:     map[string]*docSession{},
	}
	welcome := &workspace{id: "welcome", root: ""}
	folder := &workspace{id: "ws", root: root, tabs: []string{"a", "b"}, active: "b"}
	h.byID["a"] = &docSession{id: "a", pathFn: func() string { return filepath.Join(root, "a.md") }, viewMode: "markdown"}
	h.byID["b"] = &docSession{id: "b", pathFn: func() string { return filepath.Join(root, "b.md") }, viewMode: "genui"}
	h.workspaceOrder = []string{"welcome", "ws"}
	welcomeWin := &mygo.Window{}
	folderWin := &mygo.Window{}
	h.byWindow[welcomeWin] = welcome
	h.byWindow[folderWin] = folder
	h.mu.Lock()
	file := h.captureSessionLocked()
	h.mu.Unlock()
	if len(file.Workspaces) != 1 || file.Workspaces[0].ID != "ws" {
		t.Fatalf("captured %+v", file.Workspaces)
	}
	if file.Workspaces[0].ActiveID != "b" || file.Workspaces[0].Tabs[0].ViewMode != "markdown" || file.Workspaces[0].Tabs[1].ViewMode != "genui" {
		t.Fatalf("tabs %+v", file.Workspaces[0])
	}
}

func TestCaptureSessionKeepsUntitledTabsByID(t *testing.T) {
	h := &documentHost{
		byWindow: make(map[*mygo.Window]*workspace),
		byID:     map[string]*docSession{},
	}
	h.byID["u1"] = &docSession{id: "u1", pathFn: func() string { return "" }, viewMode: "preview"}
	h.byID["u2"] = &docSession{id: "u2", pathFn: func() string { return "" }, viewMode: "genui"}
	untitled := &workspace{id: "uws", tabs: []string{"u1", "u2"}, active: "u2"}
	win := &mygo.Window{}
	h.workspaceOrder = []string{"uws"}
	h.byWindow[win] = untitled
	h.mu.Lock()
	file := h.captureSessionLocked()
	h.mu.Unlock()
	if len(file.Workspaces) != 1 || file.Workspaces[0].Root != "" {
		t.Fatalf("captured %+v", file.Workspaces)
	}
	tabs := file.Workspaces[0].Tabs
	if len(tabs) != 2 || tabs[0].ID != "u1" || tabs[1].ID != "u2" {
		t.Fatalf("untitled tabs = %+v", tabs)
	}
	if tabs[0].Path != "" || tabs[0].ViewMode != "preview" || tabs[1].ViewMode != "genui" {
		t.Fatalf("untitled tab fields = %+v", tabs)
	}
	if file.Workspaces[0].ActiveID != "u2" {
		t.Fatalf("active = %q", file.Workspaces[0].ActiveID)
	}
	plan := planSessionRestore(file, restoreLive{
		OpenByID: map[string]restoreDoc{
			"u1": {ID: "u1"},
			"u2": {ID: "u2"},
		},
	})
	if len(plan.Workspaces) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	pw := plan.Workspaces[0]
	if pw.Kind != "untitled" || len(pw.Tabs) != 2 || pw.Tabs[0].ReuseID != "u1" || pw.ActiveID != "u2" {
		t.Fatalf("planned untitled = %+v", pw)
	}
}

func TestPersistSkippedWhenQuittingRestoringOrSessionReady(t *testing.T) {
	dir := t.TempDir()
	h := &documentHost{
		byWindow: make(map[*mygo.Window]*workspace),
		byID:     map[string]*docSession{},
		settings: &settingsStore{dir: dir},
		quitting: true,
	}
	h.persistSession()
	if _, err := os.Stat(filepath.Join(dir, sessionFile)); !os.IsNotExist(err) {
		t.Fatalf("quit must not write session, err=%v", err)
	}
	h.quitting = false
	h.restoring = true
	h.persistSession()
	if _, err := os.Stat(filepath.Join(dir, sessionFile)); !os.IsNotExist(err) {
		t.Fatalf("restore must not write session, err=%v", err)
	}
	h.restoring = false
	h.sessionReady = true
	h.persistSession()
	if _, err := os.Stat(filepath.Join(dir, sessionFile)); !os.IsNotExist(err) {
		t.Fatalf("sessionReady must not overwrite session, err=%v", err)
	}
}

func TestLaunchPathsFromArgsOnlyMarkdown(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "n.md")
	txt := filepath.Join(dir, "n.txt")
	got := launchPathsFromArgs([]string{"-psn_0_123", txt, md})
	if len(got) != 1 || filepath.Base(got[0]) != "n.md" {
		t.Fatalf("got %v", got)
	}
}

func TestAckQuitWindowWaitsForEveryWindow(t *testing.T) {
	dir := t.TempDir()
	a := &mygo.Window{}
	b := &mygo.Window{}
	h := &documentHost{
		byWindow: map[*mygo.Window]*workspace{
			a: {id: "ws-a", root: t.TempDir(), tabs: []string{"doc-a"}, active: "doc-a"},
			b: {id: "ws-b", root: t.TempDir(), tabs: []string{"doc-b"}, active: "doc-b"},
		},
		byID: map[string]*docSession{
			"doc-a": {id: "doc-a", pathFn: func() string { return "a.md" }, viewMode: "markdown"},
			"doc-b": {id: "doc-b", pathFn: func() string { return "b.md" }, viewMode: "genui"},
		},
		settings:     &settingsStore{dir: dir},
		quitAttempt:  1,
		quitSequence: 1,
		quitPending:  map[*mygo.Window]bool{a: true, b: true},
	}
	done, _ := h.ackQuitWindow(a, 1)
	if done {
		t.Fatal("first window must not persist or requit")
	}
	if _, err := os.Stat(filepath.Join(dir, sessionFile)); !os.IsNotExist(err) {
		t.Fatalf("partial ack wrote session: %v", err)
	}
	done, file := h.ackQuitWindow(b, 1)
	if !done {
		t.Fatal("last window must capture the session")
	}
	if len(file.Workspaces) != 2 {
		t.Fatalf("captured %d workspaces", len(file.Workspaces))
	}
	if !h.quitSaving || h.quitting || h.sessionReady {
		t.Fatal("last ack must wait for a successful session write before marking quit ready")
	}
	committed, err := h.saveQuitAttempt(1, file, func(sessionFileData) error {
		if h.sessionReady || h.quitting {
			t.Fatal("quit was marked ready before the session write")
		}
		return nil
	})
	if err != nil || !committed {
		t.Fatalf("saveQuitAttempt = committed %v, err %v", committed, err)
	}
	if !h.quitting || !h.sessionReady || h.quitAttempt != 0 || h.quitPending != nil {
		t.Fatal("successful session write must make quit ready and clear the attempt")
	}
}

func TestQuitAttemptRejectsStaleAckAfterRetry(t *testing.T) {
	a := &mygo.Window{}
	b := &mygo.Window{}
	other := &mygo.Window{}
	h := &documentHost{
		byWindow: map[*mygo.Window]*workspace{
			a: {id: "ws-a"},
			b: {id: "ws-b"},
		},
		byID:         map[string]*docSession{},
		quitAttempt:  2,
		quitSequence: 2,
		quitPending:  map[*mygo.Window]bool{a: true, b: true},
	}

	if done, _ := h.ackQuitWindow(a, 1); done {
		t.Fatal("an ack from the previous attempt completed a retry")
	}
	if len(h.quitPending) != 2 {
		t.Fatalf("stale ack changed pending windows: %d", len(h.quitPending))
	}
	if h.abortQuitAttempt(2, other, false) {
		t.Fatal("a failure from a window outside the attempt must be ignored")
	}
	if h.quitAttempt != 2 {
		t.Fatal("foreign failure changed the current attempt")
	}
	if done, _ := h.ackQuitWindow(a, 2); done {
		t.Fatal("first current ack must wait for the other window")
	}
	if done, _ := h.ackQuitWindow(b, 2); !done {
		t.Fatal("last current ack did not complete the attempt")
	}
}

func TestQuitAttemptIsBoundedAndDoesNotCaptureNewWindows(t *testing.T) {
	a := &mygo.Window{}
	b := &mygo.Window{}
	h := &documentHost{
		byWindow: map[*mygo.Window]*workspace{
			a: {id: "ws-a"},
		},
		byID: map[string]*docSession{},
	}
	attempt, windows, started := h.beginQuitAttempt()
	if !started || len(windows) != 1 || h.quitTimer == nil {
		if h.quitTimer != nil {
			h.quitTimer.Stop()
		}
		t.Fatal("quit attempt must start a bounded ack timer for every current window")
	}

	h.mu.Lock()
	h.byWindow[b] = &workspace{id: "ws-b"}
	h.mu.Unlock()
	done, _ := h.ackQuitWindow(a, attempt)
	if done || h.quitSaving {
		h.quitTimer.Stop()
		t.Fatal("quit captured a workspace that never acknowledged its flush")
	}
	if !h.abortQuitAttempt(attempt, nil, false) {
		h.quitTimer.Stop()
		t.Fatal("changed workspace should invalidate the attempt")
	}
	if done, _ := h.ackQuitWindow(a, attempt); done {
		t.Fatal("late ack from an invalidated attempt must be ignored")
	}
}

func TestQuitAttemptSaveFailureLeavesAppOpenAndRetryable(t *testing.T) {
	a := &mygo.Window{}
	h := &documentHost{
		byWindow:     map[*mygo.Window]*workspace{a: {id: "ws-a"}},
		byID:         map[string]*docSession{},
		quitAttempt:  4,
		quitSequence: 4,
		quitPending:  map[*mygo.Window]bool{},
		quitSaving:   true,
	}

	committed, err := h.saveQuitAttempt(4, sessionFileData{}, func(sessionFileData) error {
		return errors.New("disk full")
	})
	if err == nil || committed {
		t.Fatalf("save failure = committed %v, err %v", committed, err)
	}
	if h.sessionReady || h.quitting || h.quitSaving || h.quitPending != nil || h.quitAttempt != 0 {
		t.Fatal("failed session write must leave the app open and clear the failed attempt")
	}
	if attempt, wins, started := h.beginQuitAttempt(); !started || attempt != 5 || len(wins) != 1 {
		if h.quitTimer != nil {
			h.quitTimer.Stop()
		}
		t.Fatalf("retry = attempt %d, windows %d, started %v", attempt, len(wins), started)
	} else if h.quitTimer != nil {
		h.quitTimer.Stop()
	}
}
