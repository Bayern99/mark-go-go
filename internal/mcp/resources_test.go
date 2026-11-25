package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

type testWorkspaceHarness struct {
	svc      *workspace.Service
	root     string
	openDocs map[string]workspace.DocumentMCPState
	activeID string
	opened   []openCall
}

type openCall struct {
	IDOrPath     string
	RevealOffset int
	HasReveal    bool
}

func (h *testWorkspaceHarness) MCPWorkspaceState(root string) (workspace.WorkspaceMCPState, error) {
	state, err := h.svc.MCPWorkspaceState(root)
	if err != nil {
		return state, err
	}
	state.ActiveDocumentID = h.activeID
	for _, d := range h.openDocs {
		state.OpenDocuments = append(state.OpenDocuments, workspace.DocumentInfo{
			WorkspaceRoot: state.Root,
			ID:            d.ID,
			Name:          d.Name,
			Path:          d.Path,
			Edited:        d.Edited,
			Conflict:      d.Conflict,
			Deleted:       d.Deleted,
			ReadOnly:      d.ReadOnly,
		})
		for i := range state.Files {
			if state.Files[i].RelPath == d.RelPath {
				state.Files[i].Open = true
				state.Files[i].DocumentID = d.ID
			}
		}
	}
	return state, nil
}

func (h *testWorkspaceHarness) MCPOpenRoots() []string {
	return []string{fsutil.CanonicalPath(h.root)}
}

func (h *testWorkspaceHarness) MCPOpenDocuments(root string) ([]workspace.DocumentInfo, error) {
	if !fsutil.SameFile(h.root, root) {
		return nil, workspace.ErrWorkspaceNotOpen
	}
	ids := make([]string, 0, len(h.openDocs))
	for id := range h.openDocs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	docs := make([]workspace.DocumentInfo, 0, len(ids))
	for _, id := range ids {
		d := h.openDocs[id]
		docs = append(docs, workspace.DocumentInfo{
			WorkspaceRoot: fsutil.CanonicalPath(root),
			ID:            d.ID,
			Name:          d.Name,
			Path:          d.Path,
			Edited:        d.Edited,
			Conflict:      d.Conflict,
			Deleted:       d.Deleted,
			ReadOnly:      d.ReadOnly,
		})
	}
	return docs, nil
}

func (h *testWorkspaceHarness) MCPDocument(root, idOrPath string) (workspace.DocumentMCPState, error) {
	for _, d := range h.openDocs {
		if d.ID == idOrPath || d.RelPath == idOrPath || fsutil.SameFile(d.Path, idOrPath) {
			return d, nil
		}
	}
	return h.svc.MCPDocument(root, idOrPath)
}

func (h *testWorkspaceHarness) MCPOpenDocument(ctx context.Context, root, idOrPath string, revealOffset int, hasReveal bool) (workspace.DocumentInfo, error) {
	h.opened = append(h.opened, openCall{IDOrPath: idOrPath, RevealOffset: revealOffset, HasReveal: hasReveal})
	doc, err := h.MCPDocument(root, idOrPath)
	if err != nil {
		return workspace.DocumentInfo{}, err
	}
	if doc.ID == "" {
		doc.ID = "opened-" + doc.RelPath
		doc.Open = true
		h.openDocs[doc.ID] = doc
	}
	h.activeID = doc.ID
	return workspace.DocumentInfo{
		WorkspaceRoot: fsutil.CanonicalPath(root),
		ID:            doc.ID,
		Name:          doc.Name,
		Path:          doc.Path,
	}, nil
}

func (h *testWorkspaceHarness) UserDataDir() string {
	return h.svc.UserDataDir()
}

func callMCPJSON(t *testing.T, urlStr, token, sid, method string, params any) rpcResponse {
	t.Helper()
	resp := postMCP(t, urlStr, token, sid, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s HTTP status = %d, want 200", method, resp.StatusCode)
	}
	var out rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	return out
}

func initTestSession(t *testing.T, srv *Server, root, work string) (token string, sid string) {
	t.Helper()
	if err := srv.BeforeResearchStart(root, work); err != nil {
		t.Fatalf("BeforeResearchStart: %v", err)
	}
	rawCfg, err := os.ReadFile(filepath.Join(work, ".pi", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		t.Fatal(err)
	}
	token = strings.TrimPrefix(cfg.MCPServers["markgogo"].Headers["Authorization"], "Bearer ")
	resp := postMCP(t, srv.URL(), token, "", map[string]any{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": "2025-03-26"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d", resp.StatusCode)
	}
	sid = resp.Header.Get("Mcp-Session-Id")
	return token, sid
}

func TestMCPResourcesEndToEnd(t *testing.T) {
	root := fsutil.CanonicalPath(t.TempDir())
	work := review.WorkDir(t.TempDir(), root)
	if err := os.MkdirAll(filepath.Join(root, "papers"), 0o755); err != nil {
		t.Fatal(err)
	}
	livePath := filepath.Join(root, "draft.md")
	diskOnlyPath := filepath.Join(root, "papers", "study.md")
	if err := os.WriteFile(livePath, []byte("# Draft\n\nOn disk.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diskOnlyPath, []byte("# Study\n\nSaved body.\n\n<!-- note: Check citation -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	liveUnsaved := "# Draft\n\nUnsaved buffer.\n\n## Methods\n\nDetails.\n\n<!-- note: Needs proof -->\n"
	harness := &testWorkspaceHarness{
		svc:      workspace.NewService(),
		root:     root,
		activeID: "doc-1",
		openDocs: map[string]workspace.DocumentMCPState{
			"doc-1": {
				ID:      "doc-1",
				Name:    "draft.md",
				Path:    livePath,
				RelPath: "draft.md",
				Open:    true,
				Edited:  true,
				Version: 4,
				Text:    liveUnsaved,
			},
		},
	}
	srv := NewServer(Dependencies{Workspace: harness, Review: review.NewService()})
	defer func() { _ = srv.Close() }()
	token, sid := initTestSession(t, srv, root, work)

	listResp := callMCPJSON(t, srv.URL(), token, sid, "resources/list", nil)
	if listResp.Error != nil {
		t.Fatalf("resources/list error: %+v", listResp.Error)
	}
	rawList, _ := json.Marshal(listResp.Result)
	var listed struct {
		Resources []mcpResource `json:"resources"`
	}
	_ = json.Unmarshal(rawList, &listed)
	var uris []string
	for _, r := range listed.Resources {
		uris = append(uris, r.URI)
	}
	for _, want := range []string{
		"markgogo://workspace",
		"markgogo://document/doc-1",
		"markgogo://document/doc-1/sections",
		"markgogo://document/doc-1/notes",
	} {
		if !slices.Contains(uris, want) {
			t.Fatalf("resources/list missing %q in %v", want, uris)
		}
	}

	wsRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://workspace"})
	if wsRead.Error != nil {
		t.Fatalf("read workspace error: %+v", wsRead.Error)
	}
	var wsRes struct {
		Contents []mcpTextResourceContents `json:"contents"`
	}
	rawWS, _ := json.Marshal(wsRead.Result)
	_ = json.Unmarshal(rawWS, &wsRes)
	var wsPayload workspace.WorkspaceMCPState
	if err := json.Unmarshal([]byte(wsRes.Contents[0].Text), &wsPayload); err != nil {
		t.Fatal(err)
	}
	if wsPayload.Root != root || wsPayload.ActiveDocumentID != "doc-1" || !slices.Equal(wsPayload.Folders, []string{"papers"}) {
		t.Fatalf("unexpected workspace payload: %+v", wsPayload)
	}

	docRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/doc-1"})
	rawDoc, _ := json.Marshal(docRead.Result)
	var docContents struct {
		Contents []mcpTextResourceContents `json:"contents"`
	}
	_ = json.Unmarshal(rawDoc, &docContents)
	var docPayload workspace.DocumentMCPState
	_ = json.Unmarshal([]byte(docContents.Contents[0].Text), &docPayload)
	if !docPayload.Open || !docPayload.Edited || docPayload.Version != 4 || docPayload.Text != liveUnsaved {
		t.Fatalf("expected live unsaved buffer, got %+v", docPayload)
	}

	diskRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/papers/study.md"})
	if diskRead.Error != nil {
		t.Fatalf("read disk fallback error: %+v", diskRead.Error)
	}
	rawDisk, _ := json.Marshal(diskRead.Result)
	_ = json.Unmarshal(rawDisk, &docContents)
	_ = json.Unmarshal([]byte(docContents.Contents[0].Text), &docPayload)
	if docPayload.Open || docPayload.RelPath != "papers/study.md" || !strings.Contains(docPayload.Text, "Saved body.") {
		t.Fatalf("expected disk fallback payload, got %+v", docPayload)
	}

	secRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/doc-1/sections"})
	if secRead.Error != nil {
		t.Fatalf("read sections error: %+v", secRead.Error)
	}
	rawSec, _ := json.Marshal(secRead.Result)
	_ = json.Unmarshal(rawSec, &docContents)
	var secPayload struct {
		Sections []workspace.Section `json:"sections"`
	}
	_ = json.Unmarshal([]byte(docContents.Contents[0].Text), &secPayload)
	if len(secPayload.Sections) != 2 || secPayload.Sections[0].Heading != "Draft" || secPayload.Sections[1].Heading != "Methods" {
		t.Fatalf("unexpected sections: %+v", secPayload.Sections)
	}
	if secPayload.Sections[1].ID != secPayload.Sections[1].Hash || !strings.HasPrefix(secPayload.Sections[1].Hash, "fnv1a32:") {
		t.Fatalf("expected fnv1a32 section ID/hash, got %+v", secPayload.Sections[1])
	}

	notesRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/doc-1/notes"})
	if notesRead.Error != nil {
		t.Fatalf("read notes error: %+v", notesRead.Error)
	}
	rawNotes, _ := json.Marshal(notesRead.Result)
	_ = json.Unmarshal(rawNotes, &docContents)
	var notesPayload struct {
		Notes []workspace.Note `json:"notes"`
	}
	_ = json.Unmarshal([]byte(docContents.Contents[0].Text), &notesPayload)
	if len(notesPayload.Notes) != 1 || notesPayload.Notes[0].Text != "Needs proof" || notesPayload.Notes[0].SectionHeading != "Methods" {
		t.Fatalf("unexpected notes: %+v", notesPayload.Notes)
	}

	badRead := callMCPJSON(t, srv.URL(), token, sid, "resources/read", map[string]any{"uri": "markgogo://document/../etc/passwd"})
	if badRead.Error == nil {
		t.Fatal("expected error for escaping document URI")
	}
}
