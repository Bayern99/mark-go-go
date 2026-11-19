package review

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDocLookup struct {
	doc     CheckpointDoc
	err     error
	dataDir string
}

func (f fakeDocLookup) CheckpointDocument(context.Context, string, string) (CheckpointDoc, error) {
	return f.doc, f.err
}

func (f fakeDocLookup) UserDataDir() string {
	return f.dataDir
}

func TestCheckpointHostUsesDocumentLookupSeam(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "draft.md")
	const text = "# Draft\n\nBody.\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	dirtyHost := NewCheckpointHost(fakeDocLookup{
		doc: CheckpointDoc{WorkspaceRoot: root, Path: path, Content: text, Edited: true},
	})
	if _, err := dirtyHost.Create(context.Background(), "doc-1", path, text, "Initial draft", "user"); err == nil {
		t.Fatal("Create accepted an edited document")
	}

	host := NewCheckpointHost(fakeDocLookup{
		doc: CheckpointDoc{WorkspaceRoot: root, Path: path, Content: text},
	})
	created, err := host.Create(context.Background(), "doc-1", path, text, "Before Feynman edit", "feynman")
	if err != nil || len(created) != 1 {
		t.Fatalf("Create = %v, %v; want 1 checkpoint", created, err)
	}
	if created[0].Label != "Before Feynman edit" || created[0].Source != "feynman" || created[0].Backend != "git" || created[0].CreatedAt <= 0 {
		t.Fatalf("Create metadata = %+v; want label/source/backend/createdAt", created[0])
	}
	listed, err := host.List(context.Background(), "doc-1", path)
	if err != nil || len(listed) != 1 || listed[0] != created[0] {
		t.Fatalf("List = %+v, %v; want [%+v]", listed, err, created[0])
	}
	got, err := host.Read(context.Background(), "doc-1", path, created[0].ID)
	if err != nil || got != text {
		t.Fatalf("Read = %q, %v; want %q", got, err, text)
	}
}

func TestGitCheckpointSupportsLabelsProvenanceAndLegacyRawBlobs(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "notes.md")
	const legacyText = "# Legacy\n\nRaw blob content.\n"
	if err := os.WriteFile(path, []byte(legacyText), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	doc, err := openCheckpointFile(ctx, root, path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	legacyOID, err := checkpointGit(ctx, root, legacyText, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	const legacyID = "1700000000000000"
	if _, err := checkpointGit(ctx, root, "", "update-ref", "--no-deref", doc.prefix+legacyID, strings.TrimSpace(legacyOID), ""); err != nil {
		t.Fatal(err)
	}

	const updatedText = "# Updated\n\nWith provenance.\n"
	if err := os.WriteFile(path, []byte(updatedText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, updatedText, "Before MCP edit", "mcp"); err != nil {
		t.Fatal(err)
	}

	checkpoints, err := doc.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 2 {
		t.Fatalf("len(checkpoints) = %d; want 2", len(checkpoints))
	}
	if checkpoints[0].Label != "Before MCP edit" || checkpoints[0].Source != "mcp" || checkpoints[0].Backend != "git" {
		t.Fatalf("newest checkpoint = %+v; want label 'Before MCP edit', source 'mcp', backend 'git'", checkpoints[0])
	}
	if checkpoints[1].ID != legacyID || checkpoints[1].Label != "" || checkpoints[1].Source != "user" || checkpoints[1].Backend != "git" || checkpoints[1].CreatedAt != 1700000000000 {
		t.Fatalf("legacy checkpoint = %+v; want backward-compatible metadata", checkpoints[1])
	}

	gotNewest, err := doc.read(ctx, checkpoints[0].ID)
	if err != nil || gotNewest != updatedText {
		t.Fatalf("read newest = %q, %v; want %q", gotNewest, err, updatedText)
	}
	gotLegacy, err := doc.read(ctx, legacyID)
	if err != nil || gotLegacy != legacyText {
		t.Fatalf("read legacy = %q, %v; want %q", gotLegacy, err, legacyText)
	}
}

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
	doc, err := openCheckpointFile(ctx, root, filepath.Join(root, "draft.md"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, first, "", "user"); err != nil {
		t.Fatal(err)
	}
	versions, err := doc.list(ctx)
	if err != nil || len(versions) != 1 {
		t.Fatalf("history %v: %v", versions, err)
	}
	write("draft.md", "later\n")
	if err := doc.create(ctx, first, "", "user"); err == nil {
		t.Fatal("checkpoint accepted a stale document buffer")
	}
	if err := doc.create(ctx, "later\n", "Second save", "user"); err != nil {
		t.Fatal(err)
	}
	restored, err := doc.read(ctx, versions[0].ID)
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
	dataDir := t.TempDir()
	if _, err := openCheckpointFile(ctx, root, ignored, dataDir); err == nil {
		t.Fatal("offered to record a Git-ignored document")
	}
	path := filepath.Join(root, "draft.md")
	doc, err := openCheckpointFile(ctx, root, path, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, "new\n", "", ""); err != nil {
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
	if err := doc.create(ctx, "new\n", "", ""); err == nil {
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
	dataDir := t.TempDir()
	doc, err := openCheckpointFile(ctx, root, path, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.create(ctx, "paper", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("paper.md\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openCheckpointFile(ctx, root, path, dataDir); err == nil {
		t.Fatal("plugin exposed checkpoints of a document now covered by ignore rules")
	}
}

func TestNonGitCheckpointFallbackAndCapAtFiftyEntries(t *testing.T) {
	root := t.TempDir()
	dataDir := t.TempDir()
	path := filepath.Join(root, "draft.md")
	const v1 = "# Non-Git Draft\n\nFirst revision.\n"
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	lookup := &fakeDocLookup{
		dataDir: dataDir,
		doc:     CheckpointDoc{WorkspaceRoot: root, Path: path, Content: v1},
	}
	host := NewCheckpointHost(lookup)

	initial, err := host.List(ctx, "doc-non-git", path)
	if err != nil || len(initial) != 0 {
		t.Fatalf("initial List = %+v, %v; want empty slice", initial, err)
	}

	afterV1, err := host.Create(ctx, "doc-non-git", path, v1, "Initial snapshot", "user")
	if err != nil || len(afterV1) != 1 {
		t.Fatalf("Create v1 = %+v, %v; want 1 checkpoint", afterV1, err)
	}
	if afterV1[0].Label != "Initial snapshot" || afterV1[0].Source != "user" || afterV1[0].Backend != "local" {
		t.Fatalf("v1 metadata = %+v; want local snapshot metadata", afterV1[0])
	}

	wsEntries, err := os.ReadDir(root)
	if err != nil || len(wsEntries) != 1 || wsEntries[0].Name() != "draft.md" {
		t.Fatalf("workspace entries = %v, %v; want only draft.md", wsEntries, err)
	}

	const v2 = "# Non-Git Draft\n\nSecond revision.\n"
	if err := os.WriteFile(path, []byte(v2), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Create(ctx, "doc-non-git", path, v1, "Stale", "user"); err == nil {
		t.Fatal("Create accepted stale buffer when on-disk file had changed")
	}

	lookup.doc.Content = v2
	afterV2, err := host.Create(ctx, "doc-non-git", path, v2, "Before Feynman edit", "feynman")
	if err != nil || len(afterV2) != 2 {
		t.Fatalf("Create v2 = %+v, %v; want 2 checkpoints", afterV2, err)
	}
	if afterV2[0].Label != "Before Feynman edit" || afterV2[0].Source != "feynman" || afterV2[0].Backend != "local" {
		t.Fatalf("v2 metadata = %+v", afterV2[0])
	}
	if afterV2[1].ID != afterV1[0].ID {
		t.Fatalf("expected newest-first ordering, got %+v", afterV2)
	}

	gotV1, err := host.Read(ctx, "doc-non-git", path, afterV1[0].ID)
	if err != nil || gotV1 != v1 {
		t.Fatalf("Read v1 = %q, %v; want %q", gotV1, err, v1)
	}
	gotV2, err := host.Read(ctx, "doc-non-git", path, afterV2[0].ID)
	if err != nil || gotV2 != v2 {
		t.Fatalf("Read v2 = %q, %v; want %q", gotV2, err, v2)
	}

	checkpointsRoot := filepath.Join(dataDir, "checkpoints")
	docDirs, err := os.ReadDir(checkpointsRoot)
	if err != nil || len(docDirs) != 1 {
		t.Fatalf("checkpointsRoot dirs = %v, %v; want 1 docKey dir", docDirs, err)
	}
	docDir := filepath.Join(checkpointsRoot, docDirs[0].Name())
	if _, err := os.Stat(filepath.Join(docDir, "index.json")); err != nil {
		t.Fatalf("expected index.json in %s: %v", docDir, err)
	}

	allIDs := []string{afterV1[0].ID, afterV2[0].ID}
	for i := 3; i <= 55; i++ {
		body := fmt.Sprintf("# Revision %d\n", i)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		lookup.doc.Content = body
		list, err := host.Create(ctx, "doc-non-git", path, body, fmt.Sprintf("Rev %d", i), "mcp")
		if err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
		allIDs = append(allIDs, list[0].ID)
	}

	capped, err := host.List(ctx, "doc-non-git", path)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 50 {
		t.Fatalf("len(capped) = %d; want 50", len(capped))
	}
	if capped[0].Label != "Rev 55" || capped[49].Label != "Rev 6" {
		t.Fatalf("capped range = [%s .. %s]; want [Rev 55 .. Rev 6]", capped[0].Label, capped[49].Label)
	}
	for _, prunedID := range allIDs[:5] {
		if _, err := host.Read(ctx, "doc-non-git", path, prunedID); err == nil {
			t.Fatalf("expected pruned checkpoint %s to be unavailable", prunedID)
		}
	}
	gotOldestRetained, err := host.Read(ctx, "doc-non-git", path, capped[49].ID)
	if err != nil || gotOldestRetained != "# Revision 6\n" {
		t.Fatalf("Read oldest retained = %q, %v; want %q", gotOldestRetained, err, "# Revision 6\n")
	}
	gotNewestRetained, err := host.Read(ctx, "doc-non-git", path, capped[0].ID)
	if err != nil || gotNewestRetained != "# Revision 55\n" {
		t.Fatalf("Read newest retained = %q, %v; want %q", gotNewestRetained, err, "# Revision 55\n")
	}
}
