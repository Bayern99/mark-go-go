package feynman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/acp"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"maps"
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
	Kind string `json:"kind,omitempty"`
}

type ResearchUpdate struct {
	Root        string              `json:"root"`
	Kind        string              `json:"kind"`
	Text        string              `json:"text,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
	Prefill     string              `json:"prefill,omitempty"`
	Commands    []ResearchCommand   `json:"commands,omitempty"`
	Config      []ResearchConfig    `json:"config,omitempty"`
	Tool        *ResearchTool       `json:"tool,omitempty"`
	Plan        []ResearchPlanEntry `json:"plan,omitempty"`
	Used        int64               `json:"used,omitempty"`
	Size        int64               `json:"size,omitempty"`
	Image       string              `json:"image,omitempty"`
	MessageID   string              `json:"messageId,omitempty"`
	Key         string              `json:"key,omitempty"`
	RequestID   string              `json:"requestId,omitempty"`
	Options     []ResearchOption    `json:"options,omitempty"`
}

var researchUpdate = mygo.NewEvent[ResearchUpdate]("researchUpdate")

type WorkspaceRef struct {
	Root   string
	Window *mygo.Window
}

type WorkspaceLookup interface {
	WindowWorkspace(ctx context.Context, root string) (WorkspaceRef, error)
	EditableDocument(ctx context.Context, root, documentID string) error
	UserDataDir() string
}

type ResearchManager struct {
	workspaces  WorkspaceLookup
	dataDir     string
	relayScript []byte
	command     func(root, work string) (*exec.Cmd, error)
	mu          sync.Mutex
	sessions    map[string]*researchSession
	console     map[string]bool
	closed      bool
}

type researchSession struct {
	sources      map[string]os.FileInfo
	imagePrompts bool
	win          *mygo.Window
	agent        *acp.Agent
	ui           *researchUI
	id           string
	busy         bool
	configuring  bool
	switching    bool
	updateMu     sync.Mutex
	replay       []json.RawMessage
	commands     []ResearchCommand
	extensions   []ResearchCommand
	config       []ResearchConfig
	turn         chan struct{}
	cancelled    bool
	pending      *researchQuestion
	questionSlot chan struct{}
	emit         func(ResearchUpdate)
	stopped      chan struct{}
	drained      chan struct{}
	ready        chan struct{}
	startErr     error
	offWindow    func()
}

type researchQuestion struct {
	id      string
	options []ResearchOption
	answer  chan *string
}

func NewResearchManager(workspaces WorkspaceLookup, relayScript []byte) *ResearchManager {
	dataDir := ""
	if workspaces != nil {
		dataDir = workspaces.UserDataDir()
	}
	return &ResearchManager{
		workspaces:  workspaces,
		dataDir:     dataDir,
		relayScript: relayScript,
		command:     researchCommand,
		sessions:    make(map[string]*researchSession),
		console:     make(map[string]bool),
	}
}

func (h *ResearchManager) Close() {
	h.close()
}

func (h *ResearchManager) workspace(ctx context.Context, root string) (WorkspaceRef, error) {
	if h.workspaces != nil && root != "" && filepath.IsAbs(root) {
		return h.workspaces.WindowWorkspace(ctx, root)
	}
	return WorkspaceRef{}, errors.New("markgogo: this window is not a workspace")
}

func (h *ResearchManager) Connect(ctx context.Context, root string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	return h.connect(w, func(e ResearchUpdate) { _ = researchUpdate.Emit(w.Window, e) })
}

func (h *ResearchManager) connect(w WorkspaceRef, emit func(ResearchUpdate)) error {
	root := fsutil.CanonicalPath(w.Root)
	if h.dataDir == "" {
		return errors.New("The research data folder is unavailable.")
	}
	work := review.WorkDir(h.dataDir, root)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return errors.New("Research is closing.")
	}
	if h.console[root] {
		h.mu.Unlock()
		return errors.New("Close the Feynman console before starting ACP research.")
	}
	if live := h.sessions[root]; live != nil {
		select {
		case <-live.stopped:
			h.mu.Unlock()
			return errors.New("Feynman is disconnecting.")
		default:
		}
		h.mu.Unlock()
		if live.win != w.Window {
			return errors.New("This workspace is connected in another window.")
		}
		<-live.ready
		h.mu.Lock()
		defer h.mu.Unlock()
		if live.startErr != nil {
			return live.startErr
		}
		select {
		case <-live.stopped:
			return errors.New("Feynman disconnected.")
		default:
			return nil
		}
	}
	s := &researchSession{
		win: w.Window, questionSlot: make(chan struct{}, 1),
		ready: make(chan struct{}), stopped: make(chan struct{}), drained: make(chan struct{}),
		emit: func(e ResearchUpdate) { e.Root = root; emit(e) },
	}
	h.sessions[root] = s
	h.mu.Unlock()

	failSetup := func(err error) error {
		h.mu.Lock()
		s.startErr = err
		close(s.ready)
		h.mu.Unlock()
		h.stop(root, s, "")
		return err
	}

	if err := os.MkdirAll(work, 0700); err != nil {
		return failSetup(err)
	}
	contextFile := filepath.Join(filepath.Dir(work), "AGENTS.md")
	contextText := fmt.Sprintf("The reader's saved project files are in %q. Read existing inputs there. Write new research artifacts and proposed revisions at the corresponding relative paths in your working directory. Mark Go Go will publish new Markdown files and review changes to existing files.\n", root)
	if err := fsutil.WriteFileAtomicMode(contextFile, []byte(contextText), 0600); err != nil {
		return failSetup(err)
	}
	cmd, err := h.command(root, work)
	if err != nil {
		return failSetup(err)
	}
	ui, err := newResearchUI(cmd, h.relayScript)
	if err != nil {
		return failSetup(err)
	}
	agent, err := acp.NewAgent(cmd, acp.NewLimitBuffer(4096))
	if err != nil {
		ui.close()
		return failSetup(err)
	}
	agent.SetHandlers(func(method string, params json.RawMessage) (any, *acp.RPCError) { return h.request(s, method, params) }, func(method string, params json.RawMessage) { h.routeSessionUpdate(s, method, params) }, nil)

	h.mu.Lock()
	aborted := h.closed
	if !aborted {
		select {
		case <-s.stopped:
			aborted = true
		default:
		}
	}
	if aborted {
		h.mu.Unlock()
		agent.Start()
		ui.close()
		stopResearchProcess(cmd)
		agent.Kill()
		abortErr := errors.New("Research disconnected.")
		h.mu.Lock()
		s.startErr = abortErr
		close(s.ready)
		h.mu.Unlock()
		h.stop(root, s, "")
		return abortErr
	}
	s.ui = ui
	s.agent = agent
	h.mu.Unlock()

	if w.Window != nil {
		off := w.Window.OnClosed(func() { go h.stop(root, s, "") })
		h.mu.Lock()
		select {
		case <-s.stopped:
			h.mu.Unlock()
			off()
		default:
			s.offWindow = off
			h.mu.Unlock()
		}
	}

	agent.Start()
	go ui.serve(h, s)
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
			<-agent.Exited()
			h.stop(root, s, "Feynman disconnected. Reconnect to continue.")
		}()
	}
	return err
}

func (h *ResearchManager) reserveConsole(root string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("Research is closing.")
	}
	if h.console[root] {
		return errors.New("The Feynman console is already open for this workspace.")
	}
	if h.sessions[root] != nil {
		return errors.New("Close the ACP research conversation before opening the Feynman console.")
	}
	h.console[root] = true
	return nil
}

func (h *ResearchManager) releaseConsole(root string) {
	h.mu.Lock()
	delete(h.console, root)
	h.mu.Unlock()
}

func (h *ResearchManager) initialize(s *researchSession, work string) error {
	var init struct {
		ProtocolVersion   int
		AgentCapabilities struct {
			LoadSession        bool
			PromptCapabilities struct{ Image bool }
		}
	}
	err := s.agent.Call("initialize", map[string]any{"protocolVersion": acp.ProtocolVersion, "clientInfo": map[string]string{"name": acp.ClientName, "version": acp.ClientVersion}, "clientCapabilities": map[string]any{}}, &init)
	if err != nil {
		return err
	}
	if init.ProtocolVersion != acp.ProtocolVersion {
		return fmt.Errorf("Unsupported ACP protocol version %d.", init.ProtocolVersion)
	}
	h.mu.Lock()
	s.imagePrompts = init.AgentCapabilities.PromptCapabilities.Image
	h.mu.Unlock()
	record := filepath.Join(filepath.Dir(work), "session")
	id, err := os.ReadFile(record)
	if err == nil {
		if len(id) == 0 || !init.AgentCapabilities.LoadSession {
			return errors.New("Feynman cannot restore this research session.")
		}
		err = h.openConversation(s, work, string(id), "")
		var rpc *acp.RPCError
		if !errors.As(err, &rpc) || rpc.Code != -32602 || !strings.Contains(rpc.Message, "Unknown sessionId") {
			return err
		}
		return h.openConversation(s, work, "", "The previous Feynman session is no longer available. Starting a new conversation.")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return h.openConversation(s, work, "", "")
}

func (h *ResearchManager) activeLocked(ctx context.Context, root string) (*researchSession, error) {
	if h.closed {
		return nil, errors.New("Research is closing.")
	}
	s := h.sessions[fsutil.CanonicalPath(root)]
	if s != nil && s.win == mygo.CallerWindow(ctx) {
		select {
		case <-s.stopped:
		default:
			return s, nil
		}
	}
	return nil, errors.New("Connect Feynman to this workspace first.")
}

func (h *ResearchManager) Prompt(ctx context.Context, root, text, documentID, buffer string) (string, error) {
	return h.PromptWithSources(ctx, root, text, documentID, buffer, nil)
}

func (h *ResearchManager) PromptWithSources(ctx context.Context, root, text, documentID, buffer string, sources []string) (string, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", errors.New("Enter a research question or command.")
	}
	switch parts[0] {
	case "/new":
		return "", errors.New("Open Conversation history and choose New to start a workspace conversation.")
	case "/exit", "/quit":
		return "", errors.New("Use the sidebar button to hide Feynman, or turn it off in Plugins.")
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
	if s.busy || s.configuring || s.switching {
		h.mu.Unlock()
		return "", errors.New("Wait for the current research operation to finish.")
	}
	approved := maps.Clone(s.sources)
	imagePrompts := s.imagePrompts
	sessionID := s.id
	s.busy = true
	s.cancelled = false
	s.turn = make(chan struct{})
	h.mu.Unlock()
	defer func() { h.mu.Lock(); s.busy = false; close(s.turn); h.mu.Unlock() }()
	sourceBlocks, linkedImages, err := researchSourceBlocks(approved, sources, imagePrompts)
	if err != nil {
		return "", err
	}
	work := review.WorkDir(h.dataDir, root)
	before, err := review.PrepareArtifacts(work, root)
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
	prompt = append(prompt, sourceBlocks...)
	message := text
	if len(sources) > 0 {
		message += "\n\nSources:\n" + strings.Join(sources, "\n")
	}
	s.emit(ResearchUpdate{Kind: "user", Text: message})
	if len(linkedImages) > 0 {
		s.emit(ResearchUpdate{Kind: "notice", Text: "Attached as file references without an inline image: " + strings.Join(linkedImages, ", ")})
	}
	var resp struct{ StopReason string }
	if err := s.agent.Call("session/prompt", map[string]any{"sessionId": sessionID, "prompt": prompt}, &resp); err != nil {
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
	if err := review.FinishArtifacts(work, root, before); err != nil {
		return "", err
	}
	if documentID != "" {
		return h.readStagedDocument(root)
	}
	return "", nil
}

func (h *ResearchManager) update(s *researchSession, method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p struct {
		SessionID string
		Update    json.RawMessage
	}
	if json.Unmarshal(params, &p) != nil || p.SessionID == "" {
		return
	}
	h.mu.Lock()
	valid := p.SessionID == s.id
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
	e := decodeResearchUpdate(p.Update)
	if e.Kind == "" || (live && e.Kind == "user") {
		return
	}
	if e.Kind == "config" {
		h.setResearchConfig(s, e.Config)
		return
	}
	if e.Kind == "commands" {
		s.commands = e.Commands
		s.emit(s.commandsUpdateLocked())
		return
	}
	s.emit(e)
}

func (h *ResearchManager) request(s *researchSession, method string, params json.RawMessage) (any, *acp.RPCError) {
	if method != "session/request_permission" {
		return nil, &acp.RPCError{Code: -32601, Message: "Unsupported research request."}
	}
	var p struct {
		SessionID string
		ToolCall  struct {
			Title    string
			RawInput struct{ Message string }
		}
		Options []acp.PermissionOption
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acp.RPCError{Code: -32602, Message: "Invalid research permission."}
	}
	event := ResearchUpdate{Kind: "permission", Text: p.ToolCall.Title}
	for _, o := range p.Options {
		if o.OptionID != "" {
			event.Options = append(event.Options, ResearchOption{ID: o.OptionID, Name: o.Name, Kind: o.Kind})
		}
	}
	outcome := map[string]string{"outcome": "cancelled"}
	if msg := p.ToolCall.RawInput.Message; msg != "" && msg != event.Text {
		event.Text = strings.TrimSpace(event.Text + "\n" + msg)
	}
	if p.SessionID != "" && len(event.Options) > 0 {
		if choice := h.askResearchQuestion(context.Background(), s, p.SessionID, event); choice != nil {
			outcome = map[string]string{"outcome": "selected", "optionId": *choice}
		}
	}
	return map[string]any{"outcome": outcome}, nil
}

func (h *ResearchManager) Answer(ctx context.Context, root, requestID, optionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	if q := s.pending; q != nil && len(q.options) > 0 && !s.cancelled && q.id == requestID {
		if optionID == "" {
			s.pending = nil
			q.answer <- nil
			return nil
		}
		for _, opt := range q.options {
			if opt.ID == optionID {
				s.pending = nil
				q.answer <- &optionID
				return nil
			}
		}
	}
	return errors.New("This research permission or choice is no longer available.")
}

func (h *ResearchManager) Cancel(ctx context.Context, root string) error {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil || !s.busy {
		h.mu.Unlock()
		return err
	}
	s.cancelled = true
	if q := s.pending; q != nil {
		select {
		case q.answer <- nil:
		default:
		}
	}
	turn, id := s.turn, s.id
	h.mu.Unlock()
	s.agent.CancelTurn(id)
	go func() {
		select {
		case <-turn:
		case <-s.stopped:
		case <-time.After(3 * time.Second):
			h.stop(fsutil.CanonicalPath(root), s, "Feynman stopped. Reconnect to continue.")
		}
	}()
	return nil
}

func (h *ResearchManager) Disconnect(ctx context.Context, root string) error {
	h.mu.Lock()
	s := h.sessions[fsutil.CanonicalPath(root)]
	h.mu.Unlock()
	if s != nil && s.win == mygo.CallerWindow(ctx) {
		h.stop(fsutil.CanonicalPath(root), s, "")
	}
	return nil
}

func (h *ResearchManager) stop(root string, s *researchSession, message string) {
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
	if s.busy || s.switching || s.configuring {
		turn = s.turn
	}
	offWindow := s.offWindow
	s.offWindow = nil
	ui := s.ui
	agent := s.agent
	h.mu.Unlock()
	if offWindow != nil {
		offWindow()
	}
	if agent != nil && agent.Command() != nil {
		stopResearchProcess(agent.Command())
	}
	if ui != nil {
		ui.close()
	}
	if agent != nil {
		agent.Kill()
	}
	<-s.ready
	if turn != nil {
		<-turn
	}
	if message != "" {
		s.emit(ResearchUpdate{Kind: "error", Text: message})
	}
	h.mu.Lock()
	if h.sessions[root] == s {
		delete(h.sessions, root)
	}
	close(s.drained)
	h.mu.Unlock()
}

func (h *ResearchManager) close() {
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
