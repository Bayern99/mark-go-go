package workspace

import (
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestDeepestRootPrefersChildWorkspace(t *testing.T) {
	dir := t.TempDir()
	parent := dir
	child := filepath.Join(dir, "notes")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(child, "a.md")
	if err := os.WriteFile(file, []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := fsutil.DeepestRoot(file, []string{parent, child})
	if filepath.Clean(got) != filepath.Clean(child) {
		t.Fatalf("deepestRoot = %q, want child %q", got, child)
	}
}

func TestFolderContains(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.md")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fsutil.FolderContains(dir, file) {
		t.Fatal("expected folder to contain its file")
	}
	if fsutil.FolderContains(filepath.Join(dir, "other"), file) {
		t.Fatal("sibling folder must not contain the file")
	}
	if fsutil.FolderContains("", file) {
		t.Fatal("untitled window is not a folder workspace")
	}
}

func TestSameFileFollowsAlias(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	if err := os.WriteFile(a, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fsutil.SameFile(a, a) {
		t.Fatal("path equals itself")
	}
	b := filepath.Join(dir, "b.md")
	if err := os.WriteFile(b, []byte("ho\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fsutil.SameFile(a, b) {
		t.Fatal("different files")
	}
}

func TestDocumentInfoKeepsIdentityAcrossPath(t *testing.T) {
	info := DocumentInfo{ID: "doc-1", Name: "Untitled", Path: ""}
	info.Path = "/tmp/notes/a.md"
	info.Name = "a.md"
	if info.ID != "doc-1" {
		t.Fatal("identity must not change with path")
	}
}

func TestHostSnapshotCarriesDocumentID(t *testing.T) {
	s := Snapshot{ID: "doc-keep", Text: "# hi\n", Version: 1, State: DocumentInfo{ID: "doc-keep", Name: "hi.md"}}
	if s.ID != s.State.ID {
		t.Fatal("snapshot id and state id must match")
	}
	s.State.Path = "/other/hi.md"
	if s.ID != "doc-keep" {
		t.Fatal("path change must not rewrite identity")
	}
}

func TestNativeCommandCapturesDocumentNotWindow(t *testing.T) {
	cmd := NativeCommand{ID: "doc-a", Command: "save"}
	if cmd.ID == "" || cmd.Command != "save" {
		t.Fatal("native command must capture the document id")
	}
}

func TestFolderContainsRejectsNamePrefix(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir+"-other", "x.md")
	if fsutil.FolderContains(dir, file) {
		t.Fatal("a folder must not contain a similarly prefixed sibling path")
	}
}

func TestDeepestRootPicksNestedWorkspace(t *testing.T) {
	dir := t.TempDir()
	mid := filepath.Join(dir, "work")
	deep := filepath.Join(mid, "notes")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(deep, "a.md")
	if err := os.WriteFile(file, []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := fsutil.DeepestRoot(file, []string{dir, mid, deep})
	if filepath.Clean(got) != filepath.Clean(deep) {
		t.Fatalf("deepestRoot = %q, want %q", got, deep)
	}
	if fsutil.DeepestRoot(file, nil) != "" {
		t.Fatal("no open folder workspace means parent-window routing, not a deepest root")
	}
}

func TestSameFileEmptyPathNeverMatches(t *testing.T) {
	if fsutil.SameFile("", "") || fsutil.SameFile("", "/tmp/a.md") {
		t.Fatal("untitled documents have no disk identity")
	}
}

func TestSameFileFollowsHardLink(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	if err := os.WriteFile(a, []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Skip(err)
	}
	if !fsutil.SameFile(a, b) {
		t.Fatal("hard links are the same file")
	}
}

func TestCanonicalPathAndDeepestRootFollowDirectoryAlias(t *testing.T) {
	dir := t.TempDir()
	realRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		realRoot = dir
	}
	aliasRoot := filepath.Join(t.TempDir(), "ws-alias")
	if err := os.Symlink(realRoot, aliasRoot); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(realRoot, "beta.md")
	if err := os.WriteFile(realFile, []byte("# Beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliasFile := filepath.Join(aliasRoot, "beta.md")
	if fsutil.CanonicalPath(aliasRoot) != fsutil.CanonicalPath(realRoot) {
		t.Fatalf("canonical roots differ: alias %q real %q", fsutil.CanonicalPath(aliasRoot), fsutil.CanonicalPath(realRoot))
	}
	if !fsutil.SameRoot(aliasRoot, realRoot) {
		t.Fatal("alias directory must be the same workspace root")
	}
	if !fsutil.SameFile(aliasFile, realFile) {
		t.Fatal("alias file must be the same document")
	}
	if !fsutil.FolderContains(aliasRoot, realFile) || !fsutil.FolderContains(realRoot, aliasFile) {
		t.Fatal("folderContains must see through the directory alias")
	}
	got := fsutil.DeepestRoot(aliasFile, []string{realRoot})
	if !fsutil.SameRoot(got, realRoot) {
		t.Fatalf("deepestRoot = %q, want alias of %q", got, realRoot)
	}
	host := NewService()
	host.mu.Lock()
	win := &mygo.Window{}
	w := &workspace{id: "ws-real", root: fsutil.CanonicalPath(realRoot), win: win}
	host.byWindow[win] = w
	found := host.folderWorkspaceLocked(aliasFile, false)
	host.mu.Unlock()
	if found != w {
		t.Fatalf("folderWorkspaceLocked via alias = %v, want existing workspace", found)
	}
}

func TestUntitledWorkspaceKind(t *testing.T) {
	w := &workspace{id: "ws", root: ""}
	if w.kind() != "untitled" || w.name() != "Untitled" {
		t.Fatalf("untitled window: kind=%s name=%s", w.kind(), w.name())
	}
	folder := &workspace{id: "ws2", root: filepath.Join(t.TempDir(), "notes")}
	if folder.kind() != "folder" || folder.name() != "notes" {
		t.Fatalf("folder window: kind=%s name=%s", folder.kind(), folder.name())
	}
}

func TestCloseResultCancelDoesNotClose(t *testing.T) {
	cancel := CloseResult{Canceled: true}
	if cancel.Closed || !cancel.Canceled {
		t.Fatal("cancel must keep the workspace")
	}
	ok := CloseResult{Closed: true}
	if !ok.Closed || ok.Canceled {
		t.Fatal("closed result must not also be canceled")
	}
}

func TestNativeCommandNameRoutesUndoRedo(t *testing.T) {
	undo, ok := nativeCommandName(mygo.RoleUndo)
	if !ok || undo != "undo" {
		t.Fatalf("undo = %q ok=%v", undo, ok)
	}
	redo, ok := nativeCommandName(mygo.RoleRedo)
	if !ok || redo != "redo" {
		t.Fatalf("redo = %q ok=%v", redo, ok)
	}
	save, ok := nativeCommandName(mygo.RoleSave)
	if !ok || save != "save" {
		t.Fatalf("save = %q ok=%v", save, ok)
	}
	if _, ok := nativeCommandName(mygo.RoleCut); ok {
		t.Fatal("cut is not a document native command")
	}
}

func TestDocumentInfoGeneratingIsPerDocument(t *testing.T) {
	a := DocumentInfo{ID: "a", Name: "a.md", Path: "/work/a.md", Generating: true}
	b := DocumentInfo{ID: "b", Name: "b.md", Path: "/work/b.md"}
	if !a.Generating || b.Generating {
		t.Fatal("generating belongs to A only")
	}
	a.Path = "/elsewhere/a.md"
	if a.ID != "a" {
		t.Fatal("path change must not rewrite identity")
	}
}

func TestErrGeneratingExplainsWaitOrCancel(t *testing.T) {
	if !strings.Contains(errGenBusy, "wait") || !strings.Contains(errGenBusy, "cancel") {
		t.Fatalf("errGenBusy = %q, want wait-or-cancel wording", errGenBusy)
	}
}

func TestWindowSnapshotAggregatesRootsIntoOneDocumentGroup(t *testing.T) {
	win := &mygo.Window{}
	h := &Service{
		byWindow:    map[*mygo.Window]*workspace{},
		byWorkspace: map[string]*workspace{},
		byID:        map[string]*docSession{},
	}
	research := &workspace{id: "research", windowID: "window", root: "/Research", win: win, tabs: []string{"a"}, active: "a"}
	music := &workspace{id: "music", windowID: "window", root: "/Music Department", win: win, tabs: []string{"b"}, active: "b"}
	h.byWorkspace[research.id] = research
	h.byWorkspace[music.id] = music
	h.byWindow[win] = music
	h.workspaceOrder = []string{research.id, music.id}
	h.byID["a"] = &docSession{id: "a", ws: research, pathFn: func() string { return "/Research/Discussion.md" }}
	h.byID["b"] = &docSession{id: "b", ws: music, pathFn: func() string { return "/Music Department/CC129.md" }}

	snap, err := h.snapshotWorkspace(win)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ID != "window" || snap.Root != music.root || snap.ActiveID != "b" {
		t.Fatalf("selected window/root = %+v", snap)
	}
	if len(snap.Workspaces) != 2 || snap.Workspaces[0].ID != "research" || snap.Workspaces[1].ID != "music" {
		t.Fatalf("workspace summaries = %+v", snap.Workspaces)
	}
	if len(snap.Documents) != 2 || snap.Documents[0].ID != "a" || snap.Documents[1].ID != "b" {
		t.Fatalf("shared document group = %+v", snap.Documents)
	}

	h.byWindow[win] = research
	snap, err = h.snapshotWorkspace(win)
	if err != nil || snap.Root != research.root || snap.ActiveID != "a" || len(snap.Documents) != 2 {
		t.Fatalf("window snapshot after switching roots = %+v, %v", snap, err)
	}
}

func TestFolderRoutingIncludesBackgroundRoots(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "nested")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(child, "a.md")
	if err := os.WriteFile(file, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	win := &mygo.Window{}
	selected := &workspace{id: "selected", root: t.TempDir(), win: win}
	background := &workspace{id: "background", root: child, win: win}
	h := &Service{
		byWindow:    map[*mygo.Window]*workspace{win: selected},
		byWorkspace: map[string]*workspace{selected.id: selected, background.id: background},
	}
	h.mu.Lock()
	got := h.folderWorkspaceLocked(file, false)
	h.mu.Unlock()
	if got != background {
		t.Fatalf("folderWorkspaceLocked = %v, want background deepest root", got)
	}
}

func TestRemovingLastRootKeepsWindowIdentity(t *testing.T) {
	w := &workspace{id: "root", windowID: "window-stable", root: t.TempDir()}
	h := &Service{
		byWindow:    map[*mygo.Window]*workspace{nil: w},
		byWorkspace: map[string]*workspace{w.id: w},
		byID:        map[string]*docSession{},
	}
	h.removeWorkspaceRoot(w)
	current := h.byWindow[nil]
	if current == nil || current.root != "" {
		t.Fatal("closing the last root must leave Welcome")
	}
	if snap, err := h.snapshotWorkspace(current.win); err != nil || snap.ID != "window-stable" {
		t.Fatalf("window id = %q, %v, want window-stable", snap.ID, err)
	}
}

func TestRestoreUntitledUsesPreferredSharedWindow(t *testing.T) {
	preferred := &mygo.Window{}
	other := &mygo.Window{}
	target := &workspace{id: "folder", windowID: "window", root: t.TempDir(), win: preferred}
	recovery := &workspace{id: "recovery", windowID: "other", root: "", win: other}
	h := &Service{
		byWindow:       map[*mygo.Window]*workspace{preferred: target, other: recovery},
		byWorkspace:    map[string]*workspace{target.id: target, recovery.id: recovery},
		byID:           map[string]*docSession{},
		workspaceOrder: []string{target.id, recovery.id},
	}
	got := h.workspaceForPlan(plannedWorkspace{ID: "untitled", Kind: "untitled"}, preferred)
	if got == nil || got.win != preferred || got.root != "" {
		t.Fatalf("untitled restore destination = %+v, want preferred window", got)
	}
}

func TestRetireEmptyUntitledKeepsSharedWindowAndOtherRoot(t *testing.T) {
	win := &mygo.Window{}
	untitled := &workspace{id: "welcome", root: "", win: win}
	folder := &workspace{id: "folder", root: t.TempDir(), win: win}
	h := &Service{
		byWindow:       map[*mygo.Window]*workspace{win: folder},
		byWorkspace:    map[string]*workspace{untitled.id: untitled, folder.id: folder},
		byID:           map[string]*docSession{},
		workspaceOrder: []string{untitled.id, folder.id},
	}
	if !h.retireEmptyUntitled(untitled) {
		t.Fatal("empty untitled root should retire")
	}
	if h.byWorkspace[untitled.id] != nil || h.byWindow[win] != folder {
		t.Fatal("shared folder must remain selected in the same window")
	}
	snap, err := h.snapshotWorkspace(win)
	if err != nil || len(snap.Workspaces) != 1 || snap.Workspaces[0].ID != folder.id {
		t.Fatalf("remaining roots = %+v, %v", snap.Workspaces, err)
	}
}
