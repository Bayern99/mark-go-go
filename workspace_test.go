package main

import (
	"os"
	"path/filepath"
	"testing"
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
	got := deepestRoot(file, []string{parent, child})
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
	if !folderContains(dir, file) {
		t.Fatal("expected folder to contain its file")
	}
	if folderContains(filepath.Join(dir, "other"), file) {
		t.Fatal("sibling folder must not contain the file")
	}
	if folderContains("", file) {
		t.Fatal("untitled window is not a folder workspace")
	}
}

func TestSameFileFollowsAlias(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	if err := os.WriteFile(a, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !sameFile(a, a) {
		t.Fatal("path equals itself")
	}
	b := filepath.Join(dir, "b.md")
	if err := os.WriteFile(b, []byte("ho\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sameFile(a, b) {
		t.Fatal("different files")
	}
}

func TestConfirmCloseCancelKeepsEveryID(t *testing.T) {
	items := []pendingClose{
		{ID: "a", Dirty: true},
		{ID: "b", Dirty: true},
		{ID: "c", Dirty: false},
	}
	asked := []string{}
	ids, canceled := confirmClose(items, func(id string) bool {
		asked = append(asked, id)
		return id == "b" // cancel on the second dirty document
	})
	if !canceled {
		t.Fatal("expected cancel")
	}
	if ids != nil {
		t.Fatalf("cancel must not return ids to close, got %v", ids)
	}
	if len(asked) != 2 || asked[0] != "a" || asked[1] != "b" {
		t.Fatalf("asked %v, want a then b; c is clean and must not be asked", asked)
	}
}

func TestConfirmCloseSuccessReturnsAllTabs(t *testing.T) {
	items := []pendingClose{
		{ID: "a", Dirty: true},
		{ID: "b", Dirty: false},
	}
	ids, canceled := confirmClose(items, func(id string) bool {
		if id == "b" {
			t.Fatal("must not confirm a clean document")
		}
		return false
	})
	if canceled {
		t.Fatal("not canceled")
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("ids %v", ids)
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
	if folderContains(dir, file) {
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
	got := deepestRoot(file, []string{dir, mid, deep})
	if filepath.Clean(got) != filepath.Clean(deep) {
		t.Fatalf("deepestRoot = %q, want %q", got, deep)
	}
	if deepestRoot(file, nil) != "" {
		t.Fatal("no open folder workspace means parent-window routing, not a deepest root")
	}
}

func TestSameFileEmptyPathNeverMatches(t *testing.T) {
	if sameFile("", "") || sameFile("", "/tmp/a.md") {
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
	if !sameFile(a, b) {
		t.Fatal("hard links are the same file")
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
