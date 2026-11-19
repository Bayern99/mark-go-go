package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type ResearchOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ResearchUpdate struct {
	Root      string           `json:"root"`
	Kind      string           `json:"kind"`
	Text      string           `json:"text,omitempty"`
	Commands  []string         `json:"commands,omitempty"`
	RequestID string           `json:"requestId,omitempty"`
	Options   []ResearchOption `json:"options,omitempty"`
}

var researchUpdate = mygo.NewEvent[ResearchUpdate]("researchUpdate")

type researchHost struct {
	documents *documentHost
	dataDir   string
	command   func(root, work string) (*exec.Cmd, error)
	mu        sync.Mutex
	sessions  map[string]*researchSession
	closed    bool
}

type researchSession struct {
	win       *mygo.Window
	agent     *acpAgent
	id        string
	busy      bool
	turn      chan struct{}
	cancelled bool
	pending   *researchQuestion
	emit      func(ResearchUpdate)
	stopped   chan struct{}
	drained   chan struct{}
	ready     chan struct{}
	startErr  error
	offWindow func()
}

type researchQuestion struct {
	id      string
	options []ResearchOption
	answer  chan string
}

func newResearchHost(documents *documentHost) *researchHost {
	return &researchHost{documents: documents, dataDir: documents.settings.dir, command: researchCommand, sessions: make(map[string]*researchSession)}
}

func researchWorkDir(dataDir, root string) string {
	return filepath.Join(dataDir, "research", fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalPath(root)))), "work")
}

func (h *researchHost) workspace(ctx context.Context, root string) (*workspace, error) {
	win := mygo.CallerWindow(ctx)
	if win != nil && root != "" && filepath.IsAbs(root) {
		h.documents.mu.Lock()
		defer h.documents.mu.Unlock()
		for _, w := range h.documents.windowWorkspacesLocked(win) {
			if w.root == root {
				return w, nil
			}
		}
	}
	return nil, errNoWorkspace
}

func (h *researchHost) Connect(ctx context.Context, root string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	return h.connect(w, func(e ResearchUpdate) { _ = researchUpdate.Emit(w.win, e) })
}

func (h *researchHost) connect(w *workspace, emit func(ResearchUpdate)) error {
	root := canonicalPath(w.root)
	if h.dataDir == "" {
		return errors.New("The research data folder is unavailable.")
	}
	work := researchWorkDir(h.dataDir, root)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return errors.New("Research is closing.")
	}
	if live := h.sessions[root]; live != nil {
		select {
		case <-live.stopped:
			h.mu.Unlock()
			return errors.New("Feynman is disconnecting.")
		default:
		}
		h.mu.Unlock()
		if live.win != w.win {
			return errors.New("This workspace is connected in another window.")
		}
		<-live.ready
		return live.startErr
	}
	if err := os.MkdirAll(work, 0700); err != nil {
		h.mu.Unlock()
		return err
	}
	contextFile := filepath.Join(filepath.Dir(work), "AGENTS.md")
	contextText := fmt.Sprintf("The reader's saved project files are in %q. Read existing inputs there. Write new research artifacts and proposed revisions at the corresponding relative paths in your working directory. Mark Go Go will publish new Markdown files and review changes to existing files.\n", root)
	if err := writeFileAtomicMode(contextFile, []byte(contextText), 0600); err != nil {
		h.mu.Unlock()
		return err
	}
	cmd, err := h.command(root, work)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	agent, err := newACPAgent(cmd, newLimitBuffer(4096))
	if err != nil {
		h.mu.Unlock()
		return err
	}
	s := &researchSession{agent: agent, win: w.win, ready: make(chan struct{}), stopped: make(chan struct{}), drained: make(chan struct{}), emit: func(e ResearchUpdate) { e.Root = root; emit(e) }}
	h.sessions[root] = s
	agent.setHandlers(func(method string, params json.RawMessage) (any, *acpError) { return h.request(s, method, params) }, func(method string, params json.RawMessage) { h.update(s, method, params) }, nil)
	if w.win != nil {
		s.offWindow = w.win.OnClosed(func() { go h.stop(root, s, "") })
	}
	s.emit(ResearchUpdate{Kind: "reset"})
	agent.start()
	h.mu.Unlock()
	timeout := time.AfterFunc(45*time.Second, func() { h.stop(root, s, "") })
	err = h.initialize(s, work)
	timeout.Stop()
	h.mu.Lock()
	select {
	case <-s.stopped:
		if err == nil {
			err = errors.New("Research disconnected.")
		}
	default:
	}
	s.startErr = err
	close(s.ready)
	h.mu.Unlock()
	if err != nil {
		h.stop(root, s, "")
	} else {
		go func() {
			<-agent.exited
			h.stop(root, s, "Feynman disconnected. Reconnect to continue.")
		}()
	}
	return err
}

func (h *researchHost) initialize(s *researchSession, work string) error {
	var init struct {
		ProtocolVersion   int
		AgentCapabilities struct{ LoadSession bool }
	}
	err := s.agent.call("initialize", map[string]any{"protocolVersion": acpProtocolVersion, "clientInfo": map[string]string{"name": acpClientName, "version": acpClientVersion}, "clientCapabilities": map[string]any{}}, &init)
	if err != nil {
		return err
	}
	if init.ProtocolVersion != acpProtocolVersion {
		return fmt.Errorf("Unsupported ACP protocol version %d.", init.ProtocolVersion)
	}
	record := filepath.Join(filepath.Dir(work), "session")
	id, err := os.ReadFile(record)
	params := map[string]any{"cwd": work, "mcpServers": []any{}}
	if err == nil {
		if len(id) == 0 || !init.AgentCapabilities.LoadSession {
			return errors.New("Feynman cannot restore this research session.")
		}
		h.mu.Lock()
		s.id = string(id)
		h.mu.Unlock()
		params["sessionId"] = string(id)
		err = s.agent.call("session/load", params, nil)
		var rpc *acpError
		if !errors.As(err, &rpc) || rpc.Code != -32602 || !strings.Contains(rpc.Message, "Unknown sessionId") {
			return err
		}
		h.mu.Lock()
		s.id = ""
		h.mu.Unlock()
		delete(params, "sessionId")
		s.emit(ResearchUpdate{Kind: "notice", Text: "The previous Feynman session is no longer available. Starting a new conversation."})
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var resp struct{ SessionID string }
	if err := s.agent.call("session/new", params, &resp); err != nil {
		return err
	}
	if resp.SessionID == "" {
		return errors.New("Feynman returned an empty session ID.")
	}
	h.mu.Lock()
	s.id = resp.SessionID
	h.mu.Unlock()
	return writeFileAtomicMode(record, []byte(resp.SessionID), 0600)
}

func (h *researchHost) activeLocked(ctx context.Context, root string) (*researchSession, error) {
	s := h.sessions[canonicalPath(root)]
	if s != nil && s.win == mygo.CallerWindow(ctx) {
		select {
		case <-s.stopped:
		default:
			return s, nil
		}
	}
	return nil, errors.New("Connect Feynman to this workspace first.")
}

func (h *researchHost) Prompt(ctx context.Context, root, text, documentID, buffer string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", errors.New("Enter a research question or command.")
	}
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return "", err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return "", s.startErr
		}
	default:
		h.mu.Unlock()
		return "", errors.New("Feynman is connecting.")
	}
	if s.busy {
		h.mu.Unlock()
		return "", errors.New("Feynman is working on the previous prompt.")
	}
	s.busy = true
	s.cancelled = false
	s.turn = make(chan struct{})
	h.mu.Unlock()
	defer func() { h.mu.Lock(); s.busy = false; close(s.turn); h.mu.Unlock() }()
	work := researchWorkDir(h.dataDir, root)
	before, err := prepareResearchArtifacts(work, root)
	if err != nil {
		return "", err
	}
	if err := h.stageDocument(ctx, root, documentID, buffer); err != nil {
		return "", err
	}
	prompt := []map[string]string{{"type": "text", "text": text}}
	if documentID != "" {
		prompt = append(prompt, map[string]string{"type": "text", "text": "\n\nThe editable working copy of the selected Markdown document is current-document.md in the session directory. Apply the requested revision to that working copy."})
	}
	h.mu.Lock()
	cancelled := s.cancelled
	h.mu.Unlock()
	if cancelled {
		return "", errors.New("Research cancelled.")
	}
	s.emit(ResearchUpdate{Kind: "user", Text: text})
	var resp struct{ StopReason string }
	if err := s.agent.call("session/prompt", map[string]any{"sessionId": s.id, "prompt": prompt}, &resp); err != nil {
		return "", err
	}
	h.mu.Lock()
	cancelled = s.cancelled
	h.mu.Unlock()
	if cancelled || resp.StopReason == "cancelled" {
		return "", errors.New("Research cancelled.")
	}
	if resp.StopReason != "end_turn" {
		return "", fmt.Errorf("Research stopped (%s).", resp.StopReason)
	}
	if err := finishResearchArtifacts(work, root, before); err != nil {
		return "", err
	}
	if documentID != "" {
		return h.readStagedDocument(root)
	}
	return "", nil
}

func (h *researchHost) update(s *researchSession, method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p struct {
		SessionID string
		Update    struct {
			SessionUpdate, Title, Status string
			Content                      json.RawMessage
			AvailableCommands            []struct{ Name string }
		}
	}
	if json.Unmarshal(params, &p) != nil || p.SessionID == "" {
		return
	}
	h.mu.Lock()
	valid := s.id == "" || p.SessionID == s.id
	live := s.busy
	h.mu.Unlock()
	if !valid {
		return
	}
	select {
	case <-s.stopped:
		return
	default:
	}
	u := p.Update
	switch u.SessionUpdate {
	case "agent_message_chunk", "user_message_chunk":
		if live && u.SessionUpdate == "user_message_chunk" {
			return
		}
		var c struct{ Type, Text string }
		if json.Unmarshal(u.Content, &c) != nil || c.Type != "text" || c.Text == "" {
			return
		}
		kind := "assistant"
		if u.SessionUpdate == "user_message_chunk" {
			kind = "user"
		}
		s.emit(ResearchUpdate{Kind: kind, Text: c.Text})
	case "available_commands_update":
		names := make([]string, 0, len(u.AvailableCommands))
		for _, cmd := range u.AvailableCommands {
			if cmd.Name != "" {
				names = append(names, cmd.Name)
			}
		}
		s.emit(ResearchUpdate{Kind: "commands", Commands: names})
	case "tool_call", "tool_call_update":
		msg := u.Title
		if msg == "" {
			msg = u.Status
		}
		if msg != "" {
			s.emit(ResearchUpdate{Kind: "progress", Text: msg})
		}
	}
}

func (h *researchHost) request(s *researchSession, method string, params json.RawMessage) (any, *acpError) {
	if method != "session/request_permission" {
		return nil, &acpError{Code: -32601, Message: "Unsupported research request."}
	}
	var p struct {
		SessionID string
		ToolCall  struct {
			Title    string
			RawInput struct{ Message string }
		}
		Options []acpPermissionOption
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acpError{Code: -32602, Message: "Invalid research permission."}
	}
	q := &researchQuestion{id: newWorkspaceID(), answer: make(chan string, 1)}
	for _, o := range p.Options {
		if o.OptionID != "" {
			q.options = append(q.options, ResearchOption{ID: o.OptionID, Name: o.Name})
		}
	}
	outcome := map[string]string{"outcome": "cancelled"}
	h.mu.Lock()
	if p.SessionID != s.id || !s.busy || s.cancelled || len(q.options) == 0 {
		h.mu.Unlock()
		return map[string]any{"outcome": outcome}, nil
	}
	s.pending = q
	h.mu.Unlock()
	title := p.ToolCall.Title
	if msg := p.ToolCall.RawInput.Message; msg != "" && msg != title {
		title = strings.TrimSpace(title + "\n" + msg)
	}
	s.emit(ResearchUpdate{Kind: "permission", RequestID: q.id, Text: title, Options: q.options})
	select {
	case choice := <-q.answer:
		if choice != "" {
			outcome = map[string]string{"outcome": "selected", "optionId": choice}
		}
	case <-s.stopped:
	case <-s.agent.exited:
	}
	h.mu.Lock()
	s.pending = nil
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "permissionDone", RequestID: q.id})
	return map[string]any{"outcome": outcome}, nil
}

func (h *researchHost) Answer(ctx context.Context, root, requestID, optionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	if q := s.pending; q != nil && !s.cancelled && q.id == requestID {
		for _, opt := range q.options {
			if opt.ID == optionID {
				s.pending = nil
				q.answer <- optionID
				return nil
			}
		}
	}
	return errors.New("This research permission or choice is no longer available.")
}

func (h *researchHost) Cancel(ctx context.Context, root string) error {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil || !s.busy {
		h.mu.Unlock()
		return err
	}
	s.cancelled = true
	if q := s.pending; q != nil {
		select {
		case q.answer <- "":
		default:
		}
	}
	turn := s.turn
	h.mu.Unlock()
	s.agent.cancelTurn(s.id)
	go func() {
		select {
		case <-turn:
		case <-s.stopped:
		case <-time.After(3 * time.Second):
			h.stop(canonicalPath(root), s, "Feynman stopped. Reconnect to continue.")
		}
	}()
	return nil
}

func (h *researchHost) Disconnect(ctx context.Context, root string) error {
	h.mu.Lock()
	s := h.sessions[canonicalPath(root)]
	h.mu.Unlock()
	if s != nil && s.win == mygo.CallerWindow(ctx) {
		h.stop(canonicalPath(root), s, "")
	}
	return nil
}

func (h *researchHost) stop(root string, s *researchSession, message string) {
	h.mu.Lock()
	if h.sessions[root] != s {
		h.mu.Unlock()
		return
	}
	select {
	case <-s.stopped:
		h.mu.Unlock()
		<-s.drained
		return
	default:
		close(s.stopped)
	}
	s.cancelled = true
	var turn chan struct{}
	if s.busy {
		turn = s.turn
	}
	if s.offWindow != nil {
		s.offWindow()
	}
	stopResearchProcess(s.agent.cmd)
	h.mu.Unlock()
	s.agent.kill()
	<-s.ready
	if turn != nil {
		<-turn
	}
	if message != "" {
		s.emit(ResearchUpdate{Kind: "error", Text: message})
	}
	h.mu.Lock()
	delete(h.sessions, root)
	close(s.drained)
	h.mu.Unlock()
}

func (h *researchHost) close() {
	h.mu.Lock()
	h.closed = true
	sessions := make(map[string]*researchSession, len(h.sessions))
	for root, s := range h.sessions {
		sessions[root] = s
	}
	h.mu.Unlock()
	for root, s := range sessions {
		h.stop(root, s, "")
	}
}
