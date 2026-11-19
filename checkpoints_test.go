package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentCheckpointPreservesContentsBranchAndStaging(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write("draft.md", "committed\n")
	write("notes.md", "other\n")
	git("add", ".")
	git("commit", "-qm", "first")
	write("draft.md", "previously staged\n")
	write("notes.md", "unrelated staging\n")
	git("add", ".")
	const first = "saved draft\r\n\r\n"
	write("draft.md", first)
	head, index, status := git("rev-parse", "HEAD"), git("ls-files", "--stage"), git("status", "--porcelain")
	ctx := context.Background()
	doc, err := openCheckpointFile(ctx, root, filepath.Join(root, "draft.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, first); err != nil {
		t.Fatal(err)
	}
	versions, err := doc.list(ctx)
	if err != nil || len(versions) != 1 {
		t.Fatalf("history %v: %v", versions, err)
	}
	write("draft.md", "later\n")
	if err := doc.create(ctx, first); err == nil {
		t.Fatal("checkpoint accepted a stale document buffer")
	}
	if err := doc.create(ctx, "later\n"); err != nil {
		t.Fatal(err)
	}
	restored, err := doc.read(ctx, versions[0])
	if err != nil || restored != first {
		t.Fatalf("read historical bytes %q: %v", restored, err)
	}
	current, err := os.ReadFile(filepath.Join(root, "draft.md"))
	if err != nil || string(current) != "later\n" || git("rev-parse", "HEAD") != head || git("ls-files", "--stage") != index || git("status", "--porcelain") != status {
		t.Fatal("checkpoint or preview changed the current file, branch, or staging")
	}
}

func TestCheckpointInNewRepositoryGuardsIgnoredAndReplacedDocuments(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for name, value := range map[string]string{".gitignore": "Research/\n", "draft.md": "new\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ignored := filepath.Join(root, "Research", "paper.md")
	if err := os.Mkdir(filepath.Dir(ignored), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openCheckpointFile(ctx, root, ignored); err == nil {
		t.Fatal("offered to record a Git-ignored document")
	}
	path := filepath.Join(root, "draft.md")
	doc, err := openCheckpointFile(ctx, root, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, "new\n"); err != nil {
		t.Fatal("explicit checkpoint of an untracked document:", err)
	}
	if _, err := doc.read(ctx, "../../heads/main"); err == nil {
		t.Fatal("accepted a revision outside this document's checkpoints")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ignored, path); err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, "new\n"); err == nil {
		t.Fatal("recorded a document replaced by a symlink")
	}
}

func TestGitIgnoreRuleCoversExistingDocumentCheckpoints(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "paper.md")
	if err := os.WriteFile(path, []byte("paper"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", root, "add", "paper.md").Run(); err != nil {
		t.Fatal(err)
	}
	doc, err := openCheckpointFile(ctx, root, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, "paper"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("paper.md\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openCheckpointFile(ctx, root, path); err == nil {
		t.Fatal("plugin exposed checkpoints of a document now covered by ignore rules")
	}
}
