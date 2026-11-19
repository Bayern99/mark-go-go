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
	if len(found) != 4 {
		t.Fatalf("got %#v", found)
	}
	for _, file := range found {
		if file.Missing != (file.Path == "papers/removed.md") || file.Proposal != (file.Path == report) || file.Target != filepath.Join(root, file.Path) {
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
	restored, err := recoverResearchArtifact(work, root, "papers/removed.md")
	if err != nil || restored != filepath.Join(root, "papers/removed.md") {
		t.Fatalf("restore: %q %v", restored, err)
	}
	checkFile("papers/removed.md", "a draft")
	put(root, "papers/removed.md", "typed after recovery")
	if _, err := recoverResearchArtifact(work, root, "papers/removed.md"); err == nil {
		t.Fatal("recover overwrote an existing document")
	}
	checkFile("papers/removed.md", "typed after recovery")
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

func TestResearchCandidateRecoveryHonoursCompletedContentAndAcceptance(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(t.TempDir(), "work")
	name := "report.md"
	target := filepath.Join(root, name)
	stage := filepath.Join(work, name)
	write := func(p, text string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	item := func() ResearchArtifact {
		t.Helper()
		files, err := collectResearchArtifacts(work, root)
		if err != nil || len(files) != 1 {
			t.Fatalf("results %v, %v", files, err)
		}
		return files[0]
	}
	write(target, "user text")
	before, err := prepareResearchArtifacts(work, root)
	if err != nil {
		t.Fatal(err)
	}
	write(stage, "completed replacement")
	if err := finishResearchArtifacts(work, root, before); err != nil {
		t.Fatal(err)
	}
	if file := item(); !file.Proposal || file.Target != target || file.Error != "" {
		t.Fatal(file)
	}
	write(stage, "interrupted replacement")
	if file := item(); file.Proposal || file.Missing || file.Target != target || file.Error == "" {
		t.Fatalf("unavailable candidate: %+v", file)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if file := item(); file.Target != "" || file.Error == "" {
		t.Fatalf("incomplete bytes offered for recovery: %+v", file)
	}
	if _, err := recoverResearchArtifact(work, root, name); err == nil {
		t.Fatal("recovered unapproved content")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("recreated deleted file")
	}
	write(stage, "completed replacement")
	if file := item(); !file.Missing || file.Error != "" || file.Target != target {
		t.Fatal(file)
	}
	if _, err := recoverResearchArtifact(work, root, name); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stage); err != nil {
		t.Fatal(err)
	}
	if file := item(); file.Proposal || file.Missing || file.Target != target || file.Error != "" {
		t.Fatalf("accepted content re-offered: %+v", file)
	}
	write(target, "edited after accepting")
	if file := item(); file.Proposal || file.Missing || file.Error != "" {
		t.Fatalf("old proposal reappeared: %+v", file)
	}
}
