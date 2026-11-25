package workspace

import (
	"context"
	"errors"
	"fmt"
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
	liveText := "# Alpha\n\nUnsaved buffer.\n"
	svc.byID["doc-a"] = &docSession{
		host:    svc,
		id:      "doc-a",
		ws:      ws,
		version: 7,
		pathFn:  func() string { return docA },
		textFn:  func() string { return liveText },
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
	liveText = "Inserted line\n" + liveText
	updated, err := svc.MCPDocument(root, "alpha.md")
	if err != nil || updated.Text != liveText || updated.Version <= openDoc.Version {
		t.Errorf("typed text did not advance the MCP version: %+v (%v)", updated, err)
	}
	if again, err := svc.MCPDocument(root, "doc-a"); err != nil || again.Version != updated.Version {
		t.Errorf("unchanged buffer returned a different revision: %+v (%v)", again, err)
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

func newBridgeHarness(t *testing.T, roots ...string) *Service {
	t.Helper()
	svc := &Service{
		byWindow:    make(map[*mygo.Window]*workspace),
		byWorkspace: make(map[string]*workspace),
		byID:        make(map[string]*docSession),
		claims:      make(map[string]string),
	}
	for i, root := range roots {
		ws := &workspace{id: fmt.Sprintf("ws-%d", i), root: fsutil.CanonicalPath(root)}
		svc.byWorkspace[ws.id] = ws
		svc.workspaceOrder = append(svc.workspaceOrder, ws.id)
	}
	return svc
}

func TestMCPOpenRootsAndOpenDocumentsStayInsideOpenWorkspaces(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	closed := t.TempDir()
	svc := newBridgeHarness(t, outer, inner)

	if got := svc.MCPOpenRoots(); !slices.Equal(got, []string{fsutil.CanonicalPath(outer), fsutil.CanonicalPath(inner)}) {
		t.Fatalf("MCPOpenRoots() = %v", got)
	}

	docs, err := svc.MCPOpenDocuments(inner)
	if err != nil {
		t.Fatalf("MCPOpenDocuments(inner): %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("open documents = %+v, want none", docs)
	}
	if _, err := svc.MCPOpenDocuments(closed); !errors.Is(err, ErrWorkspaceNotOpen) {
		t.Fatalf("MCPOpenDocuments(closed) = %v, want ErrWorkspaceNotOpen", err)
	}
}

func TestMCPWorkspaceStateBoundsItsFileList(t *testing.T) {
	root := t.TempDir()
	for i := range MaxWorkspaceListingFiles + 5 {
		name := filepath.Join(root, fmt.Sprintf("doc-%03d.md", i))
		if err := os.WriteFile(name, []byte("# Doc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc := newBridgeHarness(t, root)
	state, err := svc.MCPWorkspaceState(root)
	if err != nil {
		t.Fatalf("MCPWorkspaceState: %v", err)
	}
	if len(state.Files) != MaxWorkspaceListingFiles || !state.Truncated || state.FilesLimit != MaxWorkspaceListingFiles {
		t.Fatalf("files = %d, truncated = %v, limit = %d", len(state.Files), state.Truncated, state.FilesLimit)
	}
}

func TestMCPRootBindingDoesNotReviveWhenAWorkspaceReopens(t *testing.T) {
	root := fsutil.CanonicalPath(t.TempDir())
	svc := newBridgeHarness(t, root)
	binding := svc.MCPBindRoot(root)
	if binding == nil || !binding() {
		t.Fatal("open workspace did not bind")
	}
	if _, err := svc.CloseWorkspaceRoot(context.Background(), "ws-0"); err != nil {
		t.Fatal(err)
	}
	if binding() {
		t.Fatal("closed workspace retained its binding")
	}
	svc.settings = &settingsStore{dir: t.TempDir()}
	svc.addWorkspaceToWindow("ws-0", root, "", nil)
	t.Cleanup(func() {
		svc.forgetWindow(nil)
		// forgetWindow persists the session in the background; let it finish
		// before the settings folder is removed.
		svc.sessionWG.Wait()
	})
	if binding() {
		t.Fatal("reopening revived an old binding")
	}
	if next := svc.MCPBindRoot(root); next == nil || !next() {
		t.Fatal("reopened workspace did not accept a new binding")
	}
}
