package workspace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

func TestWorkspaceMCPStateAndDocumentLookup(t *testing.T) {
	root := t.TempDir()
	notesDir := filepath.Join(root, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docA := filepath.Join(root, "alpha.md")
	docB := filepath.Join(notesDir, "beta.md")
	if err := os.WriteFile(docA, []byte("# Alpha\n\nDisk text.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docB, []byte("# Beta\n\nDisk beta.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "ignored.md"), []byte("# Ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.md")
	if err := os.WriteFile(outsideFile, []byte("# Secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	canonRoot := fsutil.CanonicalPath(root)
	svc := &Service{
		byWindow:    make(map[*mygo.Window]*workspace),
		byWorkspace: make(map[string]*workspace),
		byID:        make(map[string]*docSession),
		claims:      make(map[string]string),
	}
	ws := &workspace{
		id:     "ws-1",
		root:   canonRoot,
		tabs:   []string{"doc-a"},
		active: "doc-a",
	}
	svc.byWorkspace[ws.id] = ws
	svc.workspaceOrder = []string{ws.id}
	svc.byID["doc-a"] = &docSession{
		host:    svc,
		id:      "doc-a",
		ws:      ws,
		version: 7,
		pathFn:  func() string { return docA },
		textFn:  func() string { return "# Alpha\n\nUnsaved buffer.\n" },
		edited:  true,
	}

	state, err := svc.MCPWorkspaceState(root)
	if err != nil {
		t.Fatalf("MCPWorkspaceState failed: %v", err)
	}
	if state.Root != canonRoot || state.ActiveDocumentID != "doc-a" {
		t.Fatalf("unexpected workspace state: %+v", state)
	}
	if len(state.OpenDocuments) != 1 || state.OpenDocuments[0].ID != "doc-a" || !state.OpenDocuments[0].Edited {
		t.Fatalf("unexpected open documents: %+v", state.OpenDocuments)
	}
	if !slices.Equal(state.Folders, []string{"notes"}) {
		t.Fatalf("Folders = %v, want [notes]", state.Folders)
	}
	if len(state.Files) != 2 || state.Files[0].RelPath != "alpha.md" || !state.Files[0].Open || state.Files[1].RelPath != "notes/beta.md" || state.Files[1].Open {
		t.Fatalf("unexpected Files: %+v", state.Files)
	}

	openDoc, err := svc.MCPDocument(root, "doc-a")
	if err != nil {
		t.Fatalf("MCPDocument(doc-a) failed: %v", err)
	}
	if !openDoc.Open || !openDoc.Edited || openDoc.Version != 7 || openDoc.Text != "# Alpha\n\nUnsaved buffer.\n" {
		t.Fatalf("expected live unsaved buffer, got %+v", openDoc)
	}

	diskDoc, err := svc.MCPDocument(root, "notes/beta.md")
	if err != nil {
		t.Fatalf("MCPDocument(notes/beta.md) failed: %v", err)
	}
	if diskDoc.Open || diskDoc.Text != "# Beta\n\nDisk beta.\n" || diskDoc.RelPath != "notes/beta.md" {
		t.Fatalf("expected disk fallback, got %+v", diskDoc)
	}

	for _, bad := range []string{"../secret.md", outsideFile, ".git/ignored.md", ""} {
		if _, err := svc.MCPDocument(root, bad); err == nil {
			t.Errorf("MCPDocument(%q) succeeded, want rejection", bad)
		}
	}

	if _, err := svc.MCPOpenDocument(context.Background(), root, "../secret.md", 0, false); err == nil {
		t.Fatal("MCPOpenDocument outside workspace must fail")
	}
	opened, err := svc.MCPOpenDocument(context.Background(), root, "doc-a", 12, true)
	if err != nil {
		t.Fatalf("MCPOpenDocument(doc-a) failed: %v", err)
	}
	if opened.ID != "doc-a" {
		t.Fatalf("opened.ID = %q, want doc-a", opened.ID)
	}
}
