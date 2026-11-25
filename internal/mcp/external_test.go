package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

type socketHarness struct {
	svc      *workspace.Service
	roots    []string
	userData string
	docs     map[string]workspace.DocumentMCPState
	opened   []openCall

	mu         sync.Mutex
	stateCalls int
}

func (h *socketHarness) MCPWorkspaceState(root string) (workspace.WorkspaceMCPState, error) {
	h.mu.Lock()
	h.stateCalls++
	h.mu.Unlock()
	state, err := h.svc.MCPWorkspaceState(root)
	if err != nil {
		return state, err
	}
	for _, doc := range h.docs {
		if doc.Path == "" || !fsutil.FolderContains(state.Root, doc.Path) {
			continue
		}
		state.OpenDocuments = append(state.OpenDocuments, workspace.DocumentInfo{
			WorkspaceRoot: state.Root,
			ID:            doc.ID,
			Name:          doc.Name,
			Path:          doc.Path,
			Edited:        doc.Edited,
			Conflict:      doc.Conflict,
			Deleted:       doc.Deleted,
			ReadOnly:      doc.ReadOnly,
		})
		for i := range state.Files {
			if state.Files[i].RelPath == doc.RelPath {
				state.Files[i].Open = true
				state.Files[i].DocumentID = doc.ID
			}
		}
	}
	return state, nil
}

func (h *socketHarness) workspaceStateCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stateCalls
}

func (h *socketHarness) MCPOpenRoots() []string { return h.roots }

func (h *socketHarness) MCPOpenDocuments(root string) ([]workspace.DocumentInfo, error) {
	canonRoot := fsutil.CanonicalPath(root)
	if !slices.Contains(h.roots, canonRoot) {
		return nil, workspace.ErrWorkspaceNotOpen
	}
	docs := make([]workspace.DocumentInfo, 0, len(h.docs))
	for _, doc := range h.docs {
		if !fsutil.FolderContains(canonRoot, doc.Path) {
			continue
		}
		docs = append(docs, workspace.DocumentInfo{
			WorkspaceRoot: canonRoot,
			ID:            doc.ID,
			Name:          doc.Name,
			Path:          doc.Path,
			Edited:        doc.Edited,
			Conflict:      doc.Conflict,
			Deleted:       doc.Deleted,
			ReadOnly:      doc.ReadOnly,
		})
	}
	slices.SortFunc(docs, func(a, b workspace.DocumentInfo) int { return strings.Compare(a.ID, b.ID) })
	return docs, nil
}

func (h *socketHarness) MCPDocument(root, idOrPath string) (workspace.DocumentMCPState, error) {
	for _, doc := range h.docs {
		if doc.ID == idOrPath || doc.RelPath == idOrPath || fsutil.SameFile(doc.Path, idOrPath) {
			return doc, nil
		}
	}
	return h.svc.MCPDocument(root, idOrPath)
}

func (h *socketHarness) MCPOpenDocument(ctx context.Context, root, idOrPath string, revealOffset int, hasReveal bool) (workspace.DocumentInfo, error) {
	h.opened = append(h.opened, openCall{IDOrPath: idOrPath, RevealOffset: revealOffset, HasReveal: hasReveal})
	doc, err := h.MCPDocument(root, idOrPath)
	if err != nil {
		return workspace.DocumentInfo{}, err
	}
	return workspace.DocumentInfo{WorkspaceRoot: fsutil.CanonicalPath(root), ID: doc.ID, Name: doc.Name, Path: doc.Path}, nil
}

func (h *socketHarness) UserDataDir() string { return h.userData }

// newSocketHarness builds a workspace with one open document and one file on
// disk, plus a user data folder the test owns.
func newSocketHarness(t *testing.T) (*socketHarness, string) {
	t.Helper()
	// Keep the app's real user data folder out of these tests.
	t.Setenv("HOME", t.TempDir())
	root := fsutil.CanonicalPath(t.TempDir())
	path := filepath.Join(root, "report.md")
	text := "# Report\n\n## Summary\n\nGenerated findings.\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "closed.md"), []byte("# Closed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Darwin gives Unix sockets 104 bytes of path, so the user data folder of
	// these tests stays short.
	userData, err := os.MkdirTemp("", "mgipc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(userData) })
	if err := os.Chmod(userData, 0o755); err != nil {
		t.Fatal(err)
	}
	return &socketHarness{
		svc:      workspace.NewService(),
		roots:    []string{root},
		userData: userData,
		docs: map[string]workspace.DocumentMCPState{
			"doc-report": {
				ID: "doc-report", Name: "report.md", Path: path, RelPath: "report.md",
				Open: true, Version: 5, EditorVersion: 2, Text: text,
			},
		},
	}, root
}

func startExternal(t *testing.T, h *socketHarness) *ExternalServer {
	t.Helper()
	srv, _ := startExternalWithReview(t, h)
	return srv
}

func startExternalWithReview(t *testing.T, h *socketHarness) (*ExternalServer, *review.Service) {
	t.Helper()
	reviews := review.NewService()
	srv := NewExternalServer(Dependencies{Workspace: h, Review: reviews})
	t.Cleanup(func() { _ = srv.Close() })
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return srv, reviews
}

type socketClient struct {
	t    *testing.T
	conn net.Conn
	in   *bufio.Reader
}

func connectExternal(t *testing.T, srv *ExternalServer, cwd string) (*socketClient, ipc.HelloReply) {
	t.Helper()
	conn, err := net.DialTimeout("unix", srv.SocketPath(), ipc.DialTimeout)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := ipc.WriteMessage(conn, ipc.Hello{CWD: cwd}); err != nil {
		t.Fatal(err)
	}
	in := bufio.NewReader(conn)
	line, err := ipc.ReadMessage(in)
	if err != nil {
		t.Fatalf("read hello reply: %v", err)
	}
	var reply ipc.HelloReply
	if err := json.Unmarshal(line, &reply); err != nil {
		t.Fatalf("decode hello reply %s: %v", line, err)
	}
	return &socketClient{t: t, conn: conn, in: in}, reply
}

func (c *socketClient) raw(payload map[string]any) rpcResponse {
	c.t.Helper()
	if err := ipc.WriteMessage(c.conn, payload); err != nil {
		c.t.Fatalf("write request: %v", err)
	}
	line, err := ipc.ReadMessage(c.in)
	if err != nil {
		c.t.Fatalf("read response: %v", err)
	}
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		c.t.Fatalf("decode response %s: %v", line, err)
	}
	return resp
}

func modernParams(params map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range params {
		out[key] = value
	}
	out["_meta"] = map[string]any{ipc.MetaProtocolVersion: modernProtocolVersion}
	return out
}

func (c *socketClient) call(method string, params map[string]any) rpcResponse {
	c.t.Helper()
	return c.raw(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": modernParams(params)})
}

func (c *socketClient) tool(name string, args map[string]any) mcpCallToolResult {
	c.t.Helper()
	resp := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	if resp.Error != nil {
		c.t.Fatalf("tools/call(%s) protocol error: %+v", name, resp.Error)
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		c.t.Fatal(err)
	}
	var out mcpCallToolResult
	if err := json.Unmarshal(raw, &out); err != nil {
		c.t.Fatalf("decode tools/call(%s): %v", name, err)
	}
	return out
}

func structured[T any](t *testing.T, result mcpCallToolResult) T {
	t.Helper()
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structuredContent %s: %v", raw, err)
	}
	return out
}

func TestExternalChannelBindsToTheDeepestOpenWorkspaceContainingTheCaller(t *testing.T) {
	h, root := newSocketHarness(t)
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "beta.md"), []byte("# Beta\n\nInner text.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.roots = []string{root, fsutil.CanonicalPath(inner)}
	srv := startExternal(t, h)

	client, reply := connectExternal(t, srv, inner)
	if !reply.OK || reply.Root != fsutil.CanonicalPath(inner) {
		t.Fatalf("hello reply = %+v, want the inner root", reply)
	}
	if reply.AppVersion != appversion.Version || len(reply.ProtocolVersions) != 2 {
		t.Fatalf("hello reply = %+v", reply)
	}
	read := client.tool(toolReadDocument, map[string]any{"path": "beta.md"})
	if read.IsError {
		t.Fatalf("read_document failed: %+v", read.Content)
	}
	if doc := structured[map[string]any](t, read); doc["text"] != "# Beta\n\nInner text.\n" {
		t.Fatalf("read_document text = %v", doc["text"])
	}

	workspaceOnly, reply := connectExternal(t, srv, filepath.Join(root, "inner"))
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	if got := structured[map[string]any](t, workspaceOnly.tool(toolGetWorkspace, nil)); got["root"] != fsutil.CanonicalPath(inner) {
		t.Fatalf("get_workspace root = %v, want the inner root", got["root"])
	}

	outer, reply := connectExternal(t, srv, root)
	if !reply.OK || reply.Root != root {
		t.Fatalf("hello reply = %+v, want the outer root", reply)
	}
	if got := structured[map[string]any](t, outer.tool(toolGetWorkspace, nil)); got["root"] != root {
		t.Fatalf("get_workspace root = %v, want %q", got["root"], root)
	}
}

func TestExternalChannelKeepsItsSocketPrivateAndClearsAStaleOne(t *testing.T) {
	h, _ := newSocketHarness(t)
	srv := startExternal(t, h)

	dirInfo, err := os.Stat(h.userData)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("user data folder mode = %o, want 0700", perm)
	}
	socketInfo, err := os.Lstat(srv.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := socketInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket mode = %o, want 0600", perm)
	}
	if socketInfo.Mode()&fs.ModeSocket == 0 {
		t.Fatalf("socket path %s is not a socket", srv.SocketPath())
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Lstat(srv.SocketPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("socket file survived Close: %v", err)
	}
	closedClient := ipc.Client{SocketPath: srv.SocketPath(), CWD: h.roots[0]}
	if _, err := closedClient.Call("tools/list", nil); ipc.CodeOf(err) != ipc.CodeAppNotRunning {
		t.Fatalf("call after Close = %v, want APP_NOT_RUNNING", err)
	}

	// A leftover socket file from a crashed run must not stop the next start.
	restarted := startExternal(t, h)
	client, reply := connectExternal(t, restarted, h.roots[0])
	if !reply.OK {
		t.Fatalf("hello reply after restart = %+v", reply)
	}
	if resp := client.call("tools/list", nil); resp.Error != nil {
		t.Fatalf("tools/list after restart: %+v", resp.Error)
	}
}

func TestExternalChannelRefusesADirectoryWithoutAnOpenWorkspace(t *testing.T) {
	h, _ := newSocketHarness(t)
	srv := startExternal(t, h)
	outside := t.TempDir()

	client, reply := connectExternal(t, srv, outside)
	if reply.OK || reply.Error == nil || reply.Error.Code != ipc.CodeWorkspaceNotOpen {
		t.Fatalf("hello reply = %+v, want WORKSPACE_NOT_OPEN", reply)
	}
	if reply.Error.Hint == "" {
		t.Fatalf("refusal %+v has no hint", reply.Error)
	}
	_ = ipc.WriteMessage(client.conn, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	if _, err := ipc.ReadMessage(client.in); err == nil {
		t.Fatal("the server kept a refused connection open")
	}

	_, err := ipc.Client{SocketPath: srv.SocketPath(), CWD: outside}.Call("tools/list", nil)
	if code := ipc.CodeOf(err); code != ipc.CodeWorkspaceNotOpen {
		t.Fatalf("client error code = %q, want WORKSPACE_NOT_OPEN (%v)", code, err)
	}
}

func TestExternalChannelServesBothProtocolEras(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	discover := client.call("server/discover", nil)
	if discover.Error != nil {
		t.Fatalf("server/discover: %+v", discover.Error)
	}
	raw, err := json.Marshal(discover.Result)
	if err != nil {
		t.Fatal(err)
	}
	var found struct {
		ResultType        string         `json:"resultType"`
		SupportedVersions []string       `json:"supportedVersions"`
		Capabilities      map[string]any `json:"capabilities"`
		Instructions      string         `json:"instructions"`
		Meta              map[string]any `json:"_meta"`
		TTLMs             int            `json:"ttlMs"`
		CacheScope        string         `json:"cacheScope"`
	}
	if err := json.Unmarshal(raw, &found); err != nil {
		t.Fatal(err)
	}
	if found.ResultType != "complete" || found.CacheScope != "private" || found.TTLMs <= 0 {
		t.Fatalf("discover result = %s", raw)
	}
	if !slices.Equal(found.SupportedVersions, []string{modernProtocolVersion, latestLegacyProtocolVersion}) {
		t.Fatalf("supportedVersions = %v", found.SupportedVersions)
	}
	if found.Instructions == "" || found.Capabilities["tools"] == nil {
		t.Fatalf("discover result lacks instructions or tools: %s", raw)
	}
	serverInfo, _ := found.Meta[ipc.MetaServerInfo].(map[string]any)
	if serverInfo["version"] != appversion.Version {
		t.Fatalf("serverInfo = %v, want version %q", serverInfo, appversion.Version)
	}

	if resp := client.call("tools/list", nil); resp.Error != nil {
		t.Fatalf("modern tools/list: %+v", resp.Error)
	}

	unsupported := client.raw(map[string]any{
		"jsonrpc": "2.0", "id": 9, "method": "tools/list",
		"params": map[string]any{"_meta": map[string]any{ipc.MetaProtocolVersion: "1900-01-01"}},
	})
	if unsupported.Error == nil || unsupported.Error.Code != errCodeUnsupportedProtocolVersion {
		t.Fatalf("unsupported version response = %+v", unsupported.Error)
	}
	rawError, _ := json.Marshal(unsupported.Error.Data)
	var data struct {
		Supported []string `json:"supported"`
		Requested string   `json:"requested"`
	}
	if err := json.Unmarshal(rawError, &data); err != nil || data.Requested != "1900-01-01" || !slices.Contains(data.Supported, modernProtocolVersion) {
		t.Fatalf("unsupported version data = %s", rawError)
	}

	legacy, legacyReply := connectExternal(t, srv, root)
	if !legacyReply.OK {
		t.Fatalf("hello reply = %+v", legacyReply)
	}
	init := legacy.raw(map[string]any{"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-03-26"}})
	if init.Error != nil {
		t.Fatalf("initialize: %+v", init.Error)
	}
	rawInit, _ := json.Marshal(init.Result)
	var negotiated struct {
		ProtocolVersion string `json:"protocolVersion"`
		Instructions    string `json:"instructions"`
		ServerInfo      struct {
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(rawInit, &negotiated); err != nil {
		t.Fatal(err)
	}
	if negotiated.ProtocolVersion != "2025-03-26" || negotiated.Instructions == "" || negotiated.ServerInfo.Version != appversion.Version {
		t.Fatalf("initialize result = %s", rawInit)
	}
	if resp := legacy.raw(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}); resp.Error != nil {
		t.Fatalf("legacy tools/list: %+v", resp.Error)
	}

	unknown := legacy.raw(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "initialize", "params": map[string]any{"protocolVersion": "1900-01-01"}})
	rawUnknown, _ := json.Marshal(unknown.Result)
	var fallback struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(rawUnknown, &fallback); err != nil {
		t.Fatal(err)
	}
	if fallback.ProtocolVersion != latestLegacyProtocolVersion {
		t.Fatalf("unknown version negotiated %q, want %q", fallback.ProtocolVersion, latestLegacyProtocolVersion)
	}
}

func TestExternalChannelReportsStableErrorCodes(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	unknown := client.call("tools/call", map[string]any{"name": "no_such_tool", "arguments": map[string]any{}})
	if unknown.Error == nil || unknown.Error.Code != errCodeInvalidParams {
		t.Fatalf("unknown tool response = %+v, want -32602", unknown.Error)
	}

	missing := client.tool(toolReadDocument, map[string]any{"path": "missing.md"})
	if missing.IsError != true || structured[appError](t, missing).Code != ipc.CodeNotFound {
		t.Fatalf("missing document result = %+v", missing)
	}

	denied := client.tool(toolReadDocument, map[string]any{"path": "../outside.md"})
	if structured[appError](t, denied).Code != ipc.CodePathDenied {
		t.Fatalf("escaping path result = %+v", denied)
	}

	invalid := client.tool(toolReadDocument, map[string]any{"path": "report.md", "heading": "Summary", "startLine": 1, "endLine": 2})
	if structured[appError](t, invalid).Code != ipc.CodeInvalidArgs {
		t.Fatalf("section and range result = %+v", invalid)
	}

	unknownField := client.tool(toolProposeEdit, map[string]any{"path": "report.md", "oldText": "Generated findings.", "newText": "New findings.", "source": "agent"})
	if structured[appError](t, unknownField).Code != ipc.CodeInvalidArgs {
		t.Fatalf("caller-supplied source was accepted: %+v", unknownField)
	}

	stale := client.tool(toolProposeEdit, map[string]any{
		"path": "report.md", "startLine": 3, "endLine": 5, "newText": "Replacement.", "baseVersion": 1,
	})
	if structured[appError](t, stale).Code != ipc.CodeVersionConflict {
		t.Fatalf("stale baseVersion result = %+v", stale)
	}
}

func TestExternalChannelReturnsTheProposalSummaryInsteadOfTheDocument(t *testing.T) {
	h, root := newSocketHarness(t)
	srv, reviews := startExternalWithReview(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	read := client.tool(toolReadDocument, map[string]any{"documentId": "doc-report"})
	var doc struct {
		Version int64 `json:"version"`
	}
	if read.IsError {
		t.Fatalf("read_document failed: %+v", read.Content)
	}
	rawRead, _ := json.Marshal(read.StructuredContent)
	if err := json.Unmarshal(rawRead, &doc); err != nil {
		t.Fatal(err)
	}

	result := client.tool(toolProposeEdit, map[string]any{
		"documentId": "doc-report", "oldText": "Generated findings.", "newText": "Revised findings.",
		"summary": "Tighten the summary",
	})
	if result.IsError {
		t.Fatalf("propose_edit failed: %+v", result.Content)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var proposal map[string]any
	if err := json.Unmarshal(raw, &proposal); err != nil {
		t.Fatal(err)
	}
	want := []string{"conflicted", "hunksCount", "proposalId", "revision", "summary"}
	got := make([]string, 0, len(proposal))
	for key := range proposal {
		got = append(got, key)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("proposal result keys = %v, want %v", got, want)
	}
	if strings.Contains(string(raw), "Revised findings.") || strings.Contains(string(raw), "# Report") {
		t.Fatalf("proposal result echoes document text: %s", raw)
	}
	proposals := reviews.ListProposals(root)
	if len(proposals) != 1 {
		t.Fatalf("queued proposals = %+v", proposals)
	}
	if proposals[0].Source != ipc.ChannelExternal {
		t.Fatalf("proposal source = %q, want %q", proposals[0].Source, ipc.ChannelExternal)
	}
	if proposals[0].BaseText != h.docs["doc-report"].Text {
		t.Fatalf("proposal was not based on the live buffer: %q", proposals[0].BaseText)
	}
}

func TestExternalChannelCreatesDocumentsWithoutAResearchSession(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	result := client.tool(toolCreateDocument, map[string]any{
		"path": "outputs/notes.md", "content": "# Notes\n\nFrom a CLI.\n",
	})
	if result.IsError {
		t.Fatalf("create_document failed: %+v", result.Content)
	}
	created := structured[map[string]any](t, result)
	if created["relPath"] != "outputs/notes.md" || created["bytes"] != float64(21) {
		t.Fatalf("create_document result = %+v", created)
	}
	onDisk, err := os.ReadFile(filepath.Join(root, "outputs", "notes.md"))
	if err != nil || string(onDisk) != "# Notes\n\nFrom a CLI.\n" {
		t.Fatalf("created file = %q, %v", onDisk, err)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "artifacts.json")); err == nil {
		t.Fatal("create_document recorded its artifacts in the process working directory")
	}
	duplicate := client.tool(toolCreateDocument, map[string]any{"path": "outputs/notes.md", "content": "# Again\n"})
	if code := structured[appError](t, duplicate).Code; code != ipc.CodeInvalidArgs {
		t.Fatalf("duplicate create code = %q", code)
	}
}

func TestExternalChannelAppendsAnAuditRecordForEachWrite(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	if result := client.tool(toolProposeEdit, map[string]any{
		"documentId": "doc-report", "oldText": "Generated findings.", "newText": "Revised findings.",
	}); result.IsError {
		t.Fatalf("propose_edit failed: %+v", result.Content)
	}
	if result := client.tool(toolProposeEdit, map[string]any{
		"path": "report.md", "startLine": 3, "endLine": 5, "newText": "Replacement.", "baseVersion": 1,
	}); !result.IsError {
		t.Fatal("stale line range was accepted")
	}
	if result := client.tool(toolReadDocument, map[string]any{"path": "report.md"}); result.IsError {
		t.Fatalf("read_document failed: %+v", result.Content)
	}

	raw, err := os.ReadFile(filepath.Join(h.userData, auditFileName))
	if err != nil {
		t.Fatalf("read the audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("audit log has %d records, want 2:\n%s", len(lines), raw)
	}
	var first, second auditEvent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if first.Tool != toolProposeEdit || first.Code != "OK" || first.Channel != ipc.ChannelExternal || first.Root != root || first.Target != "doc-report" {
		t.Fatalf("first audit record = %+v", first)
	}
	if _, err := time.Parse(time.RFC3339, first.Time); err != nil {
		t.Fatalf("audit time %q: %v", first.Time, err)
	}
	if second.Code != ipc.CodeVersionConflict || second.Target != "report.md" {
		t.Fatalf("second audit record = %+v", second)
	}
}

func TestGenUIViewWriteReportsTheVersionItWrote(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	sources := map[string]any{"summary": map[string]any{"heading": "Summary", "occurrence": 1}}

	componentView := client.tool(toolWriteGenUIView, map[string]any{
		"path": "report.md", "sources": sources,
		"a2ui": []map[string]any{{
			"version": genui.TargetA2UIVersion,
			"createSurface": map[string]any{
				"surfaceId": "view", "catalogId": genui.CatalogID,
				"components": []map[string]any{
					{"id": "root", "component": "Column", "children": []string{"kp"}},
					{"id": "kp", "component": "KeyPoints", "items": []string{"Generated findings."}},
				},
			},
		}},
	})
	if componentView.IsError {
		t.Fatalf("write_genui_view (v2) failed: %+v", componentView.Content)
	}
	if got := structured[map[string]any](t, componentView); got["version"] != float64(2) {
		t.Fatalf("component view version = %v, want 2", got["version"])
	}

	statementView := client.tool(toolWriteGenUIView, map[string]any{
		"path": "report.md", "sources": sources,
		"a2ui": []map[string]any{{
			"version": genui.TargetA2UIVersion,
			"createSurface": map[string]any{
				"surfaceId": "view", "catalogId": genui.StatementCatalogID,
				"components": []string{
					`root = Col([point])`,
					`point = Source(["summary"], [Text("Generated findings.")])`,
				},
			},
		}},
	})
	if statementView.IsError {
		t.Fatalf("write_genui_view (v3) failed: %+v", statementView.Content)
	}
	if got := structured[map[string]any](t, statementView); got["version"] != float64(3) {
		t.Fatalf("statement view version = %v, want 3", got["version"])
	}
	stored := genui.ReadView(genui.ViewPath(filepath.Join(root, "report.md")))
	if !stored.Exists || !strings.Contains(stored.Text, `"version": 3`) || !strings.Contains(stored.Text, genui.StatementLanguage) {
		t.Fatalf("stored view = %+v", stored)
	}
}

func TestResourcesListDoesNotWalkTheWorkspace(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	before := h.workspaceStateCalls()
	resp := client.call("resources/list", nil)
	if resp.Error != nil {
		t.Fatalf("resources/list: %+v", resp.Error)
	}
	if h.workspaceStateCalls() != before {
		t.Fatalf("resources/list walked the workspace %d times", h.workspaceStateCalls()-before)
	}
	raw, _ := json.Marshal(resp.Result)
	var listed struct {
		Resources []mcpResource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	uris := make([]string, 0, len(listed.Resources))
	for _, resource := range listed.Resources {
		uris = append(uris, resource.URI)
	}
	if !slices.Contains(uris, uriWorkspace) || !slices.Contains(uris, uriDocPrefix+"doc-report") {
		t.Fatalf("resources = %v", uris)
	}
}

func TestToolsDeclareStrictSchemasAndAnnotations(t *testing.T) {
	h, _ := newSocketHarness(t)
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, h.roots[0])
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	resp := client.call("tools/list", nil)
	if resp.Error != nil {
		t.Fatalf("tools/list: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var listed struct {
		Tools []mcpTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		if tool.InputSchema["additionalProperties"] != false {
			t.Errorf("%s inputSchema does not forbid extra properties", tool.Name)
		}
		if tool.OutputSchema == nil || tool.OutputSchema["additionalProperties"] != false {
			t.Errorf("%s has no strict outputSchema", tool.Name)
		}
		if len(tool.Annotations) == 0 {
			t.Errorf("%s has no annotations", tool.Name)
		}
	}
	want := []string{
		toolCreateCheckpoint, toolCreateDocument, toolGetWorkspace, toolOpenDocument,
		toolProposeEdit, toolReadDocument, toolWriteGenUIView,
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestExternalChannelClearsAResidualFileAtItsSocketPath(t *testing.T) {
	h, root := newSocketHarness(t)
	path := ipc.SocketPath(h.userData)
	if err := os.WriteFile(path, []byte("left over from a crash"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	if resp := client.call("tools/list", nil); resp.Error != nil {
		t.Fatalf("tools/list: %+v", resp.Error)
	}
}

func TestExternalChannelRefusesToTakeOverALiveSocket(t *testing.T) {
	h, root := newSocketHarness(t)
	first := startExternal(t, h)
	second := NewExternalServer(Dependencies{Workspace: h, Review: review.NewService()})
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Start(); err == nil {
		t.Fatal("a second listener took over the socket of the first")
	}
	client, reply := connectExternal(t, first, root)
	if !reply.OK {
		t.Fatalf("the first listener stopped answering: %+v", reply)
	}
	if resp := client.call("tools/list", nil); resp.Error != nil {
		t.Fatalf("tools/list: %+v", resp.Error)
	}
}

func TestExternalChannelCanBeTurnedOffAndOnAgain(t *testing.T) {
	h, root := newSocketHarness(t)
	srv := startExternal(t, h)
	if err := srv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start after Close: %v", err)
	}
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}
	if resp := client.call("tools/list", nil); resp.Error != nil {
		t.Fatalf("tools/list: %+v", resp.Error)
	}
}

func TestReadDocumentTruncatesAtALineBoundaryAndKeepsTheRangeHonest(t *testing.T) {
	h, root := newSocketHarness(t)
	long := &strings.Builder{}
	long.WriteString("# Long\n\n")
	for i := 1; i <= 30000; i++ {
		fmt.Fprintf(long, "line %05d é\n", i)
	}
	if err := os.WriteFile(filepath.Join(root, "long.md"), []byte(long.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := startExternal(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatalf("hello reply = %+v", reply)
	}

	result := client.tool(toolReadDocument, map[string]any{"path": "long.md"})
	if result.IsError {
		t.Fatalf("read_document failed: %+v", result.Content)
	}
	var doc struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Truncated {
		t.Fatal("a 30000 line document was returned whole")
	}
	if len(doc.Text) > maxReadDocumentBytes {
		t.Fatalf("truncated text is %d bytes", len(doc.Text))
	}
	if !utf8.ValidString(doc.Text) {
		t.Fatal("truncation split a rune")
	}
	if got, want := doc.EndLine, doc.StartLine+len(documentLines(doc.Text))-1; got != want {
		t.Fatalf("endLine = %d, want %d for the returned text", got, want)
	}
}
