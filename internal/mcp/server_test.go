package mcp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func postMCP(t *testing.T, urlStr, token, sessionID string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", urlStr, err)
	}
	return resp
}

func TestLoopbackHTTPAuthAndIsolatedMCPJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	repoRoot := t.TempDir()
	// Initialize a git repository to prove zero workspace-repo pollution.
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	userData := t.TempDir()
	workDir := review.WorkDir(userData, repoRoot)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}

	wsSvc := workspace.NewService()
	revSvc := review.NewService()
	srv := NewServer(Dependencies{Workspace: wsSvc, Review: revSvc})
	defer func() { _ = srv.Close() }()

	if err := srv.BeforeResearchStart(repoRoot, workDir); err != nil {
		t.Fatalf("BeforeResearchStart failed: %v", err)
	}

	u, err := url.Parse(srv.URL())
	if err != nil {
		t.Fatalf("invalid server URL %q: %v", srv.URL(), err)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", u.Host, err)
	}
	if host != "127.0.0.1" || port == "0" || port == "" {
		t.Fatalf("server bound to %q, want 127.0.0.1:<ephemeral>", u.Host)
	}

	// Verify <work>/.pi/mcp.json exists with mode 0600 and expected structure.
	mcpPath := filepath.Join(workDir, ".pi", "mcp.json")
	info, err := os.Stat(mcpPath)
	if err != nil {
		t.Fatalf("stat %s: %v", mcpPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mcp.json mode = %o, want 0600", perm)
	}
	rawCfg, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Type        string            `json:"type"`
			URL         string            `json:"url"`
			Headers     map[string]string `json:"headers"`
			Exposure    string            `json:"exposure"`
			Description string            `json:"description"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		t.Fatalf("unmarshal mcp.json: %v", err)
	}
	entry, ok := cfg.MCPServers["markgogo"]
	if !ok {
		t.Fatalf("missing mcpServers.markgogo in %s", rawCfg)
	}
	if entry.Type != "http" || entry.URL != srv.URL() || entry.Exposure != "direct" {
		t.Fatalf("unexpected mcpServers.markgogo entry: %+v", entry)
	}
	authHeader := entry.Headers["Authorization"]
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || len(token) != 64 {
		t.Fatalf("unexpected Authorization header %q (want 32-byte hex Bearer token)", authHeader)
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Fatalf("token is not hex: %v", err)
	}

	// Verify zero repo pollution and untouched global ~/.feynman/agent/mcp.json.
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoRoot
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		t.Fatalf("workspace git repo was polluted:\n%s", statusOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".feynman", "agent", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("global ~/.feynman/agent/mcp.json must remain untouched, stat err=%v", err)
	}

	// Unauthenticated and wrong-token requests must return 401.
	for _, badToken := range []string{"", "wrong-token", strings.Repeat("0", 64)} {
		resp := postMCP(t, srv.URL(), badToken, "", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q status = %d, want 401", badToken, resp.StatusCode)
		}
	}

	// Non-loopback Origin must be rejected with 403.
	rawInit, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	badOriginReq, _ := http.NewRequest(http.MethodPost, srv.URL(), bytes.NewReader(rawInit))
	badOriginReq.Header.Set("Authorization", "Bearer "+token)
	badOriginReq.Header.Set("Content-Type", "application/json")
	badOriginReq.Header.Set("Origin", "https://evil.example")
	badOriginResp, err := http.DefaultClient.Do(badOriginReq)
	if err != nil {
		t.Fatal(err)
	}
	badOriginResp.Body.Close()
	if badOriginResp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-loopback Origin status = %d, want 403", badOriginResp.StatusCode)
	}

	// Valid initialize handshake + Mcp-Session-Id + notifications/initialized + GET 405 + DELETE 204.
	initResp := postMCP(t, srv.URL(), token, "", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	})
	defer initResp.Body.Close()
	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200", initResp.StatusCode)
	}
	sid := initResp.Header.Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("missing Mcp-Session-Id response header")
	}

	notifResp := postMCP(t, srv.URL(), token, sid, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	notifResp.Body.Close()
	if notifResp.StatusCode != http.StatusAccepted {
		t.Fatalf("notifications/initialized status = %d, want 202", notifResp.StatusCode)
	}

	getReq, _ := http.NewRequest(http.MethodGet, srv.URL(), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("Mcp-Session-Id", sid)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp status = %d, want 405", getResp.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL(), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delReq.Header.Set("Mcp-Session-Id", sid)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /mcp status = %d, want 204", delResp.StatusCode)
	}

	afterDelResp := postMCP(t, srv.URL(), token, sid, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "ping"})
	afterDelResp.Body.Close()
	if afterDelResp.StatusCode != http.StatusNotFound {
		t.Fatalf("request with deleted Mcp-Session-Id status = %d, want 404", afterDelResp.StatusCode)
	}

	// AfterResearchStop revokes the Bearer token immediately.
	srv.AfterResearchStop(repoRoot, workDir)
	revokedResp := postMCP(t, srv.URL(), token, "", map[string]any{"jsonrpc": "2.0", "id": 3, "method": "ping"})
	revokedResp.Body.Close()
	if revokedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("request with revoked token status = %d, want 401", revokedResp.StatusCode)
	}
}
