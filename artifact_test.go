package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResearchArtifactsPublishMarkdownWithoutOverwritingTheWorkspace(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(t.TempDir(), "work")
	put := func(base, rel, body string) {
		t.Helper()
		path := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report := "outputs/report.md"
	plan := "outputs/.plans/report.md"
	put(root, report, "my existing report")
	put(root, "papers/removed.md", "remove during run")
	before, err := prepareResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "papers/removed.md")); err != nil {
		t.Fatal(err)
	}
	put(work, report, "suggested replacement")
	put(work, plan, "research plan")
	put(work, "papers/removed.md", "a draft")
	put(work, "outputs/report.provenance.md", "evidence")
	put(work, "current-document.md", "private buffer")
	put(work, ".git/config.md", "internal")
	if err := finishResearchArtifacts(work, root, before); err != nil {
		t.Fatal(err)
	}
	found, err := collectResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("got %#v", found)
	}
	for _, file := range found {
		if file.Proposal != (file.Path == report) || file.Target != filepath.Join(root, file.Path) {
			t.Fatalf("unexpected artifact %#v", file)
		}
	}
	checkFile := func(rel, want string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", rel, got, err)
		}
	}
	checkFile(plan, "research plan")
	checkFile(report, "my existing report")
	checkFile("outputs/report.provenance.md", "evidence")
	if _, err := os.Stat(filepath.Join(root, "papers/removed.md")); !os.IsNotExist(err) {
		t.Fatal("a deleted document was recreated")
	}
	put(root, plan, "edited in Mark Go Go")
	found, err = collectResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range found {
		if file.Path == plan && file.Proposal {
			t.Fatal("unchanged research output offered to replace a later user edit")
		}
	}
	checkFile(plan, "edited in Mark Go Go")
}

func TestUnsafeResearchTargetDoesNotHideOtherResults(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(t.TempDir(), "work")
	outside := t.TempDir()
	before, err := prepareResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "papers")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "papers"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"papers/unsafe.md", "valid.md"} {
		if err := os.WriteFile(filepath.Join(work, rel), []byte("draft"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(work, "secret.md")); err != nil {
		t.Fatal(err)
	}
	if err := finishResearchArtifacts(work, root, before); err != nil {
		t.Fatal(err)
	}
	found, err := collectResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].Error == "" || found[0].Target != "" || found[1].Path != "valid.md" || found[1].Error != "" {
		t.Fatalf("unexpected results %#v", found)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("outside files: %v %v", entries, err)
	}
}

func TestOnlyCompletedTurnPublishesResearchArtifacts(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(t.TempDir(), "work")
	if _, err := prepareResearchArtifacts(work, root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "cancelled.md"), []byte("unfinished"), 0600); err != nil {
		t.Fatal(err)
	}
	if files, err := collectResearchArtifacts(work, root); err != nil || len(files) != 0 {
		t.Fatalf("published unfinished files %v, %v", files, err)
	}
	before, err := prepareResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "finished.md"), []byte("finished"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := finishResearchArtifacts(work, root, before); err != nil {
		t.Fatal(err)
	}
	files, err := collectResearchArtifacts(work, root)
	if err != nil || len(files) != 1 || files[0].Path != "finished.md" {
		t.Fatalf("results: %v, %v", files, err)
	}
	if _, err := os.Stat(filepath.Join(root, "cancelled.md")); !os.IsNotExist(err) {
		t.Fatal("another turn published a cancelled file")
	}
}
