package mcp

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

const (
	maxRequestBodyBytes = 8 << 20
	serverName          = "mark-go-go"
	serverVersion       = "1.0.0"
)

type WorkspaceBridge interface {
	MCPWorkspaceState(root string) (workspace.WorkspaceMCPState, error)
	MCPDocument(root, idOrPath string) (workspace.DocumentMCPState, error)
	MCPOpenDocument(ctx context.Context, root, idOrPath string, revealOffset int, hasReveal bool) (workspace.DocumentInfo, error)
	UserDataDir() string
}

type ReviewBridge interface {
	SubmitProposal(input review.ProposeInput) (review.ChangeProposal, error)
}

type Dependencies struct {
	Workspace WorkspaceBridge
	Review    ReviewBridge
}

type sessionBinding struct {
	token string
	root  string
	work  string
}

type Server struct {
	deps Dependencies

	mu              sync.Mutex
	listener        net.Listener
	httpSrv         *http.Server
	addr            string
	closed          bool
	sessionsByToken map[string]*sessionBinding
	tokenByRoot     map[string]string
	mcpSessions     map[string]string
}

func NewServer(deps Dependencies) *Server {
	return &Server{
		deps:            deps,
		sessionsByToken: make(map[string]*sessionBinding),
		tokenByRoot:     make(map[string]string),
		mcpSessions:     make(map[string]string),
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("mcp server is closed")
	}
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/", s.handleFallback)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.listener = ln
	s.httpSrv = srv
	s.addr = ln.Addr().String()
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addr == "" {
		return ""
	}
	return "http://" + s.addr + "/mcp"
}

func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	srv := s.httpSrv
	ln := s.listener
	s.httpSrv = nil
	s.listener = nil
	s.addr = ""
	clear(s.sessionsByToken)
	clear(s.tokenByRoot)
	clear(s.mcpSessions)
	s.mu.Unlock()

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

func randomHexToken(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) RegisterSession(root, work string) (token string, endpointURL string, err error) {
	if err := s.Start(); err != nil {
		return "", "", err
	}
	tok, err := randomHexToken(32)
	if err != nil {
		return "", "", err
	}
	canonRoot := fsutil.CanonicalPath(root)
	canonWork := fsutil.CanonicalPath(work)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", "", errors.New("mcp server is closed")
	}
	s.revokeRootLocked(canonRoot)
	s.sessionsByToken[tok] = &sessionBinding{
		token: tok,
		root:  canonRoot,
		work:  canonWork,
	}
	s.tokenByRoot[canonRoot] = tok
	return tok, "http://" + s.addr + "/mcp", nil
}

func (s *Server) RevokeSession(root string) {
	canonRoot := fsutil.CanonicalPath(root)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokeRootLocked(canonRoot)
}

func (s *Server) revokeRootLocked(canonRoot string) {
	oldTok, ok := s.tokenByRoot[canonRoot]
	if !ok {
		return
	}
	delete(s.tokenByRoot, canonRoot)
	delete(s.sessionsByToken, oldTok)
	for sid, tok := range s.mcpSessions {
		if tok == oldTok {
			delete(s.mcpSessions, sid)
		}
	}
}

func (s *Server) BeforeResearchStart(root, work string) error {
	tok, endpointURL, err := s.RegisterSession(root, work)
	if err != nil {
		return err
	}
	if err := writeIsolatedPiMCPConfig(work, endpointURL, tok); err != nil {
		s.RevokeSession(root)
		return err
	}
	return nil
}

func (s *Server) AfterResearchStop(root, _ string) {
	s.RevokeSession(root)
}

func writeIsolatedPiMCPConfig(work, endpointURL, token string) error {
	piDir := filepath.Join(work, ".pi")
	if err := os.MkdirAll(piDir, 0o700); err != nil {
		return err
	}
	cfgPath := filepath.Join(piDir, "mcp.json")
	doc := map[string]any{}
	if existing, err := os.ReadFile(cfgPath); err == nil {
		var parsed map[string]any
		if json.Unmarshal(existing, &parsed) == nil && parsed != nil {
			doc = parsed
		}
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	servers["markgogo"] = map[string]any{
		"type": "http",
		"url":  endpointURL,
		"headers": map[string]string{
			"Authorization": "Bearer " + token,
		},
		"exposure":    "direct",
		"description": "Mark Go Go live workspace, document buffers, reader notes, change proposals, GenUI views, and checkpoints",
	}
	doc["mcpServers"] = servers
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fsutil.WriteFileAtomicMode(cfgPath, data, 0o600)
}

func parseBearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func isLoopbackOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func (s *Server) authenticateRequest(r *http.Request) (*sessionBinding, bool) {
	provided, ok := parseBearerToken(r.Header.Get("Authorization"))
	if !ok {
		return nil, false
	}
	providedBytes := []byte(provided)

	s.mu.Lock()
	defer s.mu.Unlock()
	var matched *sessionBinding
	for tok, binding := range s.sessionsByToken {
		if subtle.ConstantTimeCompare(providedBytes, []byte(tok)) == 1 {
			copyBinding := *binding
			matched = &copyBinding
		}
	}
	return matched, matched != nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticateRequest(r); !ok {
		writeUnauthorized(w)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	binding, ok := s.authenticateRequest(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	if !isLoopbackOrigin(r.Header.Get("Origin")) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden origin"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Allow", "POST, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET stream not offered"})
		return
	case http.MethodDelete:
		sid := strings.TrimSpace(r.Header.Get("Mcp-Session-Id"))
		if sid == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing Mcp-Session-Id"})
			return
		}
		s.mu.Lock()
		tok, exists := s.mcpSessions[sid]
		if exists && tok == binding.token {
			delete(s.mcpSessions, sid)
		} else {
			exists = false
		}
		s.mu.Unlock()
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodPost:
		s.handleMCPPost(w, r, binding)
		return
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
}

func (s *Server) handleMCPPost(w http.ResponseWriter, r *http.Request, binding *sessionBinding) {
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
		return
	}

	sidHeader := strings.TrimSpace(r.Header.Get("Mcp-Session-Id"))
	if sidHeader != "" {
		s.mu.Lock()
		tok, exists := s.mcpSessions[sidHeader]
		valid := exists && tok == binding.token
		s.mu.Unlock()
		if !valid {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: errCodeParseError, Message: "failed to read request body"},
		})
		return
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		writeJSON(w, http.StatusBadRequest, rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: errCodeInvalidRequest, Message: "empty JSON-RPC payload"},
		})
		return
	}

	if strings.HasPrefix(trimmed, "[") {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil || len(batch) == 0 {
			writeJSON(w, http.StatusBadRequest, rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: errCodeParseError, Message: "invalid JSON-RPC batch"},
			})
			return
		}
		responses := make([]rpcResponse, 0, len(batch))
		for _, item := range batch {
			resp, newSID, hasResp := s.dispatchMessage(r.Context(), binding, item)
			if newSID != "" {
				w.Header().Set("Mcp-Session-Id", newSID)
			}
			if hasResp {
				responses = append(responses, resp)
			}
		}
		if len(responses) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, http.StatusOK, responses)
		return
	}

	resp, newSID, hasResp := s.dispatchMessage(r.Context(), binding, body)
	if newSID != "" {
		w.Header().Set("Mcp-Session-Id", newSID)
	}
	if !hasResp {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) dispatchMessage(ctx context.Context, binding *sessionBinding, raw []byte) (rpcResponse, string, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: errCodeParseError, Message: "invalid JSON"},
		}, "", true
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		if len(req.ID) == 0 {
			return rpcResponse{}, "", false
		}
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: errCodeInvalidRequest, Message: "invalid JSON-RPC 2.0 request"},
		}, "", true
	}
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"
	if isNotification {
		return rpcResponse{}, "", false
	}

	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		version := defaultProtocolVersion
		if supportedProtocolVersions[params.ProtocolVersion] {
			version = params.ProtocolVersion
		}
		sid, err := randomHexToken(16)
		if err != nil {
			return rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: errCodeInternalError, Message: "failed to allocate session"},
			}, "", true
		}
		s.mu.Lock()
		s.mcpSessions[sid] = binding.token
		s.mu.Unlock()
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": version,
				"capabilities": map[string]any{
					"resources": map[string]any{
						"subscribe":   false,
						"listChanged": false,
					},
					"tools": map[string]any{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]any{
					"name":    serverName,
					"version": serverVersion,
				},
			},
		}, sid, true

	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}, "", true

	case "resources/list":
		res, rpcErr := s.handleResourcesList(binding)
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rpcErr}, "", true

	case "resources/templates/list":
		res := s.handleResourceTemplatesList()
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}, "", true

	case "resources/read":
		res, rpcErr := s.handleResourcesRead(binding, req.Params)
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rpcErr}, "", true

	case "tools/list":
		res := s.handleToolsList()
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}, "", true

	case "tools/call":
		res, rpcErr := s.handleToolsCall(ctx, binding, req.Params)
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rpcErr}, "", true

	default:
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: errCodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}, "", true
	}
}
