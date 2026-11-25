package mcp

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const auditFileName = "mcp-audit.jsonl"

// auditEvent is one JSONL line in the user data folder: what a write tool
// changed, over which channel, and how it ended.
type auditEvent struct {
	Time    string `json:"time"`
	Channel string `json:"channel"`
	Root    string `json:"root"`
	Tool    string `json:"tool"`
	Target  string `json:"target,omitempty"`
	Code    string `json:"code"`
}

var auditMu sync.Mutex

func (s *Server) audit(binding *sessionBinding, tool string, result mcpCallToolResult) {
	if s.deps.Workspace == nil {
		return
	}
	dir := s.deps.Workspace.UserDataDir()
	if dir == "" {
		return
	}
	code := "OK"
	if failure := appErrorOf(result); failure != nil {
		code = failure.Code
	}
	line, err := json.Marshal(auditEvent{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Channel: binding.source(),
		Root:    binding.root,
		Tool:    tool,
		Target:  result.auditTarget,
		Code:    code,
	})
	if err != nil {
		log.Printf("markgogo: record MCP audit event: %v", err)
		return
	}
	auditMu.Lock()
	defer auditMu.Unlock()
	f, err := os.OpenFile(filepath.Join(dir, auditFileName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("markgogo: open the MCP audit log: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		log.Printf("markgogo: write the MCP audit log: %v", err)
	}
}
