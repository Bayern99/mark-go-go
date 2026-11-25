package feynman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo"
)

const maxResearchUIBytes = 1 << 20

var Updated = desktop.Event[ResearchUpdate]{}

type WorkspaceRef struct {
	Root   string
	Window *mygo.Window
}

type WorkspaceLookup interface {
	WindowWorkspace(ctx context.Context, root string) (WorkspaceRef, error)
	EditableDocument(ctx context.Context, root, documentID string) error
	UserDataDir() string
}

type focusWatcher struct {
	root string
	off  func()
}

type ResearchManager struct {
	workspaces     WorkspaceLookup
	dataDir        string
	command        func(root, work string) (*exec.Cmd, error)
	cliCommand     func(ctx context.Context, work string, args ...string) (*exec.Cmd, error)
	openTerminal   func(ctx context.Context, command, profile string) error
	onWindowClosed func(*mygo.Window, func()) func()
	beforeStart    func(root, work string) error
	afterStop      func(root, work string)
	mu             sync.Mutex
	sessions       map[string]*researchSession
	focusWatchers  map[*mygo.Window]*focusWatcher
	closed         bool
}

type researchSession struct {
	sources        map[string]os.FileInfo
	imagePrompts   bool
	work           string
	win            *mygo.Window
	rpc            *rpcClient
	sessionsDir    string
	id             string
	sessionFile    string
	sessionName    string
	state          ResearchSessionState
	models         []ResearchModel
	thinkingLevels []string
	thinkingLevel  string
	commands       []ResearchCommand
	config         []ResearchConfig
	queue          ResearchQueueState
	busy           bool
	configuring    bool
	switching      bool
	turn           chan struct{}
	settled        chan bool
	legacyEnded    chan struct{}
	settleMu       sync.Mutex
	settleTimer    *time.Timer
	streamMu       sync.Mutex
	streamMsgID    string
	streamedText   bool
	streamedThink  bool
	turnErr        error
	cancelled      bool
	pending        *researchQuestion
	questionSlot   chan struct{}
	emit           func(ResearchUpdate)
	stopped        chan struct{}
	drained        chan struct{}
	ready          chan struct{}
	startErr       error
	offWindow      func()
}

type uiDialogAnswer struct {
	payload map[string]any
}

type researchQuestion struct {
	id      string
	piID    string
	method  string
	options []ResearchOption
	answer  chan uiDialogAnswer
}

func NewResearchManager(workspaces WorkspaceLookup) *ResearchManager {
	dataDir := ""
	if workspaces != nil {
		dataDir = workspaces.UserDataDir()
	}
	h := &ResearchManager{
		workspaces:     workspaces,
		dataDir:        dataDir,
		cliCommand:     feynmanCLICommand,
		openTerminal:   openSystemTerminal,
		onWindowClosed: (*mygo.Window).OnClosed,
		sessions:       make(map[string]*researchSession),
		focusWatchers:  make(map[*mygo.Window]*focusWatcher),
	}
	h.command = h.researchCommand
	return h
}

func (h *ResearchManager) SetBeforeStart(fn func(root, work string) error) {
	h.mu.Lock()
	h.beforeStart = fn
	h.mu.Unlock()
}

func (h *ResearchManager) SetAfterStop(fn func(root, work string)) {
	h.mu.Lock()
	h.afterStop = fn
	h.mu.Unlock()
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
	return h.connect(w, func(e ResearchUpdate) { _ = Updated.Emit(w.Window, e) })
}

func (h *ResearchManager) connect(w WorkspaceRef, emit func(ResearchUpdate)) error {
	root := fsutil.CanonicalPath(w.Root)
	work := review.WorkDir(h.dataDir, root)
	if err := h.checkPrivateWorkspace(root, work); err != nil {
		return err
	}
	sessionsDir := h.profileSessionsDir()
	var s *researchSession
	var beforeStart func(root, work string) error
	for {
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			return errors.New("Research is closing.")
		}
		if live := h.sessions[root]; live != nil {
			select {
			case <-live.stopped:
				drained := live.drained
				h.mu.Unlock()
				<-drained
				continue
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
		s = &researchSession{
			work:         work,
			win:          w.Window,
			sessionsDir:  sessionsDir,
			imagePrompts: true,
			questionSlot: make(chan struct{}, 1),
			ready:        make(chan struct{}),
			stopped:      make(chan struct{}),
			drained:      make(chan struct{}),
			emit:         func(e ResearchUpdate) { e.Root = root; emit(e) },
		}
		beforeStart = h.beforeStart
		h.sessions[root] = s
		h.mu.Unlock()
		break
	}

	failSetup := func(err error) error {
		h.mu.Lock()
		s.startErr = err
		close(s.ready)
		h.mu.Unlock()
		h.stop(root, s, "")
		return err
	}

	windowClosed := make(chan struct{})
	if w.Window != nil {
		onClosed := sync.OnceFunc(func() {
			close(windowClosed)
			go h.stop(root, s, "")
		})
		off := h.onWindowClosed(w.Window, onClosed)
		h.mu.Lock()
		select {
		case <-s.stopped:
			h.mu.Unlock()
			off()
		default:
			s.offWindow = off
			h.mu.Unlock()
		}
		if w.Window.IsDestroyed() {
			onClosed()
		}
	}
	startCancelled := func() bool {
		select {
		case <-windowClosed:
			return true
		case <-s.stopped:
			return true
		default:
			return false
		}
	}
	if startCancelled() {
		return failSetup(errors.New("Research disconnected."))
	}

	if err := os.MkdirAll(work, 0700); err != nil {
		return failSetup(err)
	}
	contextFile := filepath.Join(filepath.Dir(work), "AGENTS.md")
	contextText := fmt.Sprintf("The reader's saved project files are in %q. Read existing inputs there. Write new research artifacts and proposed revisions at the corresponding relative paths in your working directory. Mark Go Go will publish new Markdown files and review changes to existing files.\n", root)
	if err := fsutil.WriteFileAtomicMode(contextFile, []byte(contextText), 0600); err != nil {
		return failSetup(err)
	}
	if beforeStart != nil {
		if err := beforeStart(root, work); err != nil {
			return failSetup(err)
		}
	}
	cmd, err := h.command(root, work)
	if err != nil {
		return failSetup(err)
	}
	for i := 0; i+1 < len(cmd.Args); i++ {
		if cmd.Args[i] == "--session-dir" && cmd.Args[i+1] != "" {
			sessionsDir = fsutil.CanonicalPath(cmd.Args[i+1])
			break
		}
	}
	if startCancelled() {
		return failSetup(errors.New("Research disconnected."))
	}
	rpc, err := newRPCClient(cmd)
	if err != nil {
		return failSetup(err)
	}
	rpc.SetHandlers(
		func(eventType string, raw json.RawMessage) { h.handleRPCEvent(s, eventType, raw) },
		func(raw json.RawMessage) { h.handleExtensionUIRequest(s, raw) },
	)

	h.mu.Lock()
	if h.closed || startCancelled() {
		h.mu.Unlock()
		rpc.Start()
		stopResearchProcess(cmd)
		rpc.Kill()
		abortErr := errors.New("Research disconnected.")
		h.mu.Lock()
		s.startErr = abortErr
		close(s.ready)
		h.mu.Unlock()
		h.stop(root, s, "")
		return abortErr
	}
	s.sessionsDir = sessionsDir
	s.rpc = rpc
	h.mu.Unlock()

	if w.Window != nil && !w.Window.IsDestroyed() {
		h.ensureFocusWatch(w)
	}

	rpc.Start()
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
			<-rpc.Exited()
			h.stop(root, s, "Feynman disconnected. Reconnect to continue.")
		}()
	}
	return err
}

func (h *ResearchManager) initialize(s *researchSession, work string) error {
	record := filepath.Join(filepath.Dir(work), "session")
	savedBytes, err := os.ReadFile(record)
	if err == nil {
		savedID := strings.TrimSpace(string(savedBytes))
		if savedID == "" {
			return errors.New("Feynman cannot restore this research session.")
		}
		var initial ResearchSessionState
		if err := s.rpc.Call("get_state", nil, &initial); err != nil {
			return err
		}
		if initial.SessionID == savedID {
			return h.syncConversationState(s, work, savedID, "")
		}
		conv, ok, err := findWorkspaceConversation(h.sessionsDirFor(s), work, savedID)
		if err == nil && ok && conv.SessionFile != "" {
			if err := h.switchConversationFile(s, work, conv.SessionFile, conv.ID, ""); err == nil {
				return nil
			}
		}
		return h.syncConversationState(s, work, "", "The previous Feynman session is no longer available. Starting a new conversation.")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return h.syncConversationState(s, work, "", "")
}

func (h *ResearchManager) activeLocked(ctx context.Context, root string) (*researchSession, error) {
	if h.closed {
		return nil, errors.New("Research is closing.")
	}
	s := h.sessions[root]
	if s != nil && s.win == desktop.Window(ctx) {
		select {
		case <-s.stopped:
		default:
			return s, nil
		}
	}
	return nil, errors.New("Connect Feynman to this workspace first.")
}

func (h *ResearchManager) readySession(ctx context.Context, root string) (*researchSession, error) {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			err = s.startErr
		}
	default:
		err = errors.New("Feynman is connecting.")
	}
	h.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (h *ResearchManager) Commands(ctx context.Context, root string) ([]ResearchCommand, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Commands []ResearchCommand `json:"commands"`
	}
	if err := s.rpc.Call("get_commands", nil, &resp); err != nil {
		return nil, err
	}
	cmds := filterResearchCommands(resp.Commands)
	h.mu.Lock()
	s.commands = cmds
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "commands", Commands: cmds})
	return cmds, nil
}

func (h *ResearchManager) State(ctx context.Context, root string) (ResearchSessionState, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return ResearchSessionState{}, err
	}
	var state ResearchSessionState
	if err := s.rpc.Call("get_state", nil, &state); err != nil {
		return ResearchSessionState{}, err
	}
	h.mu.Lock()
	s.state = state
	if state.ThinkingLevel != "" {
		s.thinkingLevel = state.ThinkingLevel
	}
	if state.SessionName != "" {
		s.sessionName = state.SessionName
	}
	h.mu.Unlock()
	return state, nil
}

func (h *ResearchManager) SessionStats(ctx context.Context, root string) (ResearchSessionStats, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return ResearchSessionStats{}, err
	}
	var stats ResearchSessionStats
	if err := s.rpc.Call("get_session_stats", nil, &stats); err != nil {
		return ResearchSessionStats{}, err
	}
	h.emitStatsUpdate(s, stats)
	return stats, nil
}

func (h *ResearchManager) emitStatsUpdate(s *researchSession, stats ResearchSessionStats) {
	upd := ResearchUpdate{
		Kind:  "usage",
		Stats: &stats,
	}
	if stats.ContextUsage != nil && stats.ContextUsage.ContextWindow > 0 {
		upd.Used = stats.ContextUsage.Tokens
		upd.Size = stats.ContextUsage.ContextWindow
	} else {
		h.mu.Lock()
		var contextWindow int64
		if s.state.Model != nil {
			contextWindow = s.state.Model.ContextWindow
		}
		h.mu.Unlock()
		if contextWindow > 0 {
			upd.Used = stats.Tokens.Total
			upd.Size = contextWindow
		}
	}
	s.emit(upd)
}

func (h *ResearchManager) Compact(ctx context.Context, root, customInstructions string) (ResearchCompactionResult, error) {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return ResearchCompactionResult{}, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return ResearchCompactionResult{}, s.startErr
		}
	default:
		h.mu.Unlock()
		return ResearchCompactionResult{}, errors.New("Feynman is connecting.")
	}
	if s.busy || s.configuring || s.switching {
		h.mu.Unlock()
		return ResearchCompactionResult{}, errors.New("Wait for the current research operation to finish.")
	}
	s.configuring = true
	s.turn = make(chan struct{})
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		s.configuring = false
		close(s.turn)
		h.mu.Unlock()
	}()

	fields := map[string]any{}
	if strings.TrimSpace(customInstructions) != "" {
		fields["customInstructions"] = strings.TrimSpace(customInstructions)
	}
	var res ResearchCompactionResult
	if err := s.rpc.Call("compact", fields, &res); err != nil {
		return ResearchCompactionResult{}, err
	}
	return res, nil
}

func (h *ResearchManager) SetModel(ctx context.Context, root, provider, modelID string) (ResearchModel, error) {
	root = fsutil.CanonicalPath(root)
	provider = strings.TrimSpace(provider)
	modelID = strings.TrimSpace(modelID)
	if provider == "" && modelID != "" {
		if idx := strings.IndexAny(modelID, "/:"); idx > 0 {
			provider = modelID[:idx]
			modelID = modelID[idx+1:]
		}
	}
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return ResearchModel{}, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return ResearchModel{}, s.startErr
		}
	default:
		h.mu.Unlock()
		return ResearchModel{}, errors.New("Feynman is connecting.")
	}
	if s.busy || s.configuring || s.switching {
		h.mu.Unlock()
		return ResearchModel{}, errors.New("Wait for the current research operation to finish.")
	}
	if provider == "" {
		for _, m := range s.models {
			if m.ID == modelID {
				provider = m.Provider
				break
			}
		}
	}
	s.configuring = true
	s.turn = make(chan struct{})
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		s.configuring = false
		close(s.turn)
		h.mu.Unlock()
	}()

	var model ResearchModel
	if err := s.rpc.Call("set_model", map[string]any{"provider": provider, "modelId": modelID}, &model); err != nil {
		return ResearchModel{}, err
	}
	var levelsResp struct {
		Levels []string `json:"levels"`
	}
	_ = s.rpc.Call("get_available_thinking_levels", nil, &levelsResp)

	h.mu.Lock()
	s.state.Model = &model
	if len(levelsResp.Levels) > 0 {
		s.thinkingLevels = levelsResp.Levels
	}
	s.imagePrompts = modelSupportsImages(&model, s.models)
	cfg := buildResearchConfig(s.state, s.models, s.thinkingLevels)
	s.config = cfg
	stateCopy := s.state
	modelsCopy := slices.Clone(s.models)
	levelsCopy := slices.Clone(s.thinkingLevels)
	h.mu.Unlock()

	s.emit(ResearchUpdate{
		Kind:           "config",
		Config:         cfg,
		Models:         modelsCopy,
		ThinkingLevels: levelsCopy,
		ThinkingLevel:  stateCopy.ThinkingLevel,
		State:          &stateCopy,
	})
	return model, nil
}

func (h *ResearchManager) SetThinkingLevel(ctx context.Context, root, level string) error {
	root = fsutil.CanonicalPath(root)
	level = strings.TrimSpace(level)
	if level == "" {
		return errors.New("Choose an available thinking level.")
	}
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return s.startErr
		}
	default:
		h.mu.Unlock()
		return errors.New("Feynman is connecting.")
	}
	if s.busy || s.configuring || s.switching {
		h.mu.Unlock()
		return errors.New("Wait for the current research operation to finish.")
	}
	s.configuring = true
	s.turn = make(chan struct{})
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		s.configuring = false
		close(s.turn)
		h.mu.Unlock()
	}()

	if err := s.rpc.Call("set_thinking_level", map[string]any{"level": level}, nil); err != nil {
		return err
	}
	h.mu.Lock()
	s.thinkingLevel = level
	s.state.ThinkingLevel = level
	cfg := buildResearchConfig(s.state, s.models, s.thinkingLevels)
	s.config = cfg
	stateCopy := s.state
	modelsCopy := slices.Clone(s.models)
	levelsCopy := slices.Clone(s.thinkingLevels)
	h.mu.Unlock()

	s.emit(ResearchUpdate{
		Kind:           "config",
		Config:         cfg,
		Models:         modelsCopy,
		ThinkingLevels: levelsCopy,
		ThinkingLevel:  level,
		State:          &stateCopy,
	})
	return nil
}

func (h *ResearchManager) SetConfig(ctx context.Context, root, id, value string) error {
	switch id {
	case "model":
		_, err := h.SetModel(ctx, root, "", value)
		return err
	case "thinking", "thinking_level", "thinkingLevel":
		return h.SetThinkingLevel(ctx, root, value)
	case "steering", "steering_mode", "steeringMode":
		s, err := h.readySession(ctx, root)
		if err != nil {
			return err
		}
		return s.rpc.Call("set_steering_mode", map[string]any{"mode": value}, nil)
	case "follow_up", "follow_up_mode", "followUpMode":
		s, err := h.readySession(ctx, root)
		if err != nil {
			return err
		}
		return s.rpc.Call("set_follow_up_mode", map[string]any{"mode": value}, nil)
	case "autocompact", "auto_compaction", "autoCompaction":
		s, err := h.readySession(ctx, root)
		if err != nil {
			return err
		}
		return s.rpc.Call("set_auto_compaction", map[string]any{"enabled": value == "on" || value == "true"}, nil)
	default:
		return errors.New("Choose an available Feynman setting.")
	}
}

func (h *ResearchManager) Prompt(ctx context.Context, root, text, documentID, buffer string) (string, error) {
	return h.PromptWithSources(ctx, root, text, documentID, buffer, nil)
}

func (h *ResearchManager) PromptWithSources(ctx context.Context, root, text, documentID, buffer string, sources []string) (string, error) {
	root = fsutil.CanonicalPath(root)
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
	s.busy = true
	s.cancelled = false
	s.turnErr = nil
	s.turn = make(chan struct{})
	settled := make(chan bool, 1)
	legacyEnded := make(chan struct{}, 1)
	s.settleMu.Lock()
	s.settled = settled
	s.legacyEnded = legacyEnded
	s.settleMu.Unlock()
	h.mu.Unlock()

	defer func() {
		s.settleMu.Lock()
		if s.settleTimer != nil {
			s.settleTimer.Stop()
			s.settleTimer = nil
		}
		s.settled = nil
		s.legacyEnded = nil
		s.settleMu.Unlock()
		h.mu.Lock()
		s.busy = false
		close(s.turn)
		h.mu.Unlock()
	}()

	sourceBlocks, linkedImages, err := researchSourceBlocks(approved, sources, imagePrompts)
	if err != nil {
		return "", err
	}
	work := review.WorkDir(h.dataDir, root)
	before, err := review.PrepareArtifacts(work, root)
	if err != nil {
		return "", err
	}
	if err := h.ensureEditableDocument(ctx, root, documentID); err != nil {
		return "", err
	}

	promptMessage := text
	if documentID != "" {
		promptMessage += fmt.Sprintf(
			"\n\nTo edit the active Markdown document (documentId: %q), call the MCP tool `propose_edit` with `documentId: %q` and either targeted `oldText`/`newText`, a `startLine`/`endLine` range with `newText` and the `baseVersion` read from the document resource, or a complete `proposedText`. Changes enter the review queue without overwriting the editor buffer directly.",
			documentID, documentID,
		)
	}
	var images []map[string]string
	var fileRefs []string
	for _, block := range sourceBlocks {
		switch block["type"] {
		case "image":
			images = append(images, map[string]string{
				"type":     "image",
				"mimeType": block["mimeType"],
				"data":     block["data"],
			})
		case "resource_link":
			if uri := block["uri"]; uri != "" {
				fileRefs = append(fileRefs, uri)
			}
		}
	}
	if len(fileRefs) > 0 {
		promptMessage += "\n\nAttached source files:\n" + strings.Join(fileRefs, "\n")
	}

	h.mu.Lock()
	cancelled := s.cancelled
	h.mu.Unlock()
	if cancelled {
		return "", errors.New("Research cancelled.")
	}

	displayMessage := text
	if len(sources) > 0 {
		displayMessage += "\n\nSources:\n" + strings.Join(sources, "\n")
	}
	s.emit(ResearchUpdate{Kind: "user", Text: displayMessage})
	if len(linkedImages) > 0 {
		s.emit(ResearchUpdate{Kind: "notice", Text: "Attached as file references without an inline image: " + strings.Join(linkedImages, ", ")})
	}

	fields := map[string]any{"message": promptMessage}
	if len(images) > 0 {
		fields["images"] = images
	}
	var promptResp struct {
		Disposition string `json:"disposition"`
	}
	if err := s.rpc.Call("prompt", fields, &promptResp); err != nil {
		return "", err
	}

	if promptResp.Disposition == "handled" {
		var st ResearchSessionState
		if err := s.rpc.Call("get_state", nil, &st); err == nil && !st.IsStreaming && st.PendingMessageCount == 0 {
			s.notifySettled(false)
		}
	}
	if promptResp.Disposition != "" {
		legacyEnded = nil
	}

	var aborted bool
	select {
	case aborted = <-settled:
	case <-legacyEnded:
	case <-s.stopped:
		return "", errors.New("Feynman disconnected.")
	case <-s.rpc.Exited():
		return "", errors.New("Feynman disconnected.")
	}

	h.mu.Lock()
	cancelled = s.cancelled
	turnErr := s.turnErr
	h.mu.Unlock()
	if cancelled || aborted {
		return "", errors.New("Research cancelled.")
	}
	if turnErr != nil {
		return "", turnErr
	}
	if err := review.FinishArtifacts(work, root, before); err != nil {
		return "", err
	}
	var stats ResearchSessionStats
	if s.rpc.Call("get_session_stats", nil, &stats) == nil {
		h.emitStatsUpdate(s, stats)
	}
	if documentID != "" {
		return buffer, nil
	}
	return "", nil
}

func (h *ResearchManager) Steer(ctx context.Context, root, text string, sources []string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("Enter a steering instruction.")
	}
	s, err := h.readySession(ctx, root)
	if err != nil {
		return err
	}
	h.mu.Lock()
	approved := maps.Clone(s.sources)
	imagePrompts := s.imagePrompts
	h.mu.Unlock()
	fields, err := researchSourceRPCFields(text, approved, sources, imagePrompts)
	if err != nil {
		return err
	}
	if err := s.rpc.Call("steer", fields, nil); err != nil {
		return err
	}
	s.emit(ResearchUpdate{Kind: "user", EventType: "steer", Text: text})
	return nil
}

func (h *ResearchManager) FollowUp(ctx context.Context, root, text string, sources []string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("Enter a follow-up instruction.")
	}
	s, err := h.readySession(ctx, root)
	if err != nil {
		return err
	}
	h.mu.Lock()
	approved := maps.Clone(s.sources)
	imagePrompts := s.imagePrompts
	h.mu.Unlock()
	fields, err := researchSourceRPCFields(text, approved, sources, imagePrompts)
	if err != nil {
		return err
	}
	return s.rpc.Call("follow_up", fields, nil)
}

func researchSourceRPCFields(text string, approved map[string]os.FileInfo, sources []string, imagePrompts bool) (map[string]any, error) {
	fields := map[string]any{"message": text}
	if len(sources) == 0 {
		return fields, nil
	}
	blocks, _, err := researchSourceBlocks(approved, sources, imagePrompts)
	if err != nil {
		return nil, err
	}
	var images []map[string]string
	var fileRefs []string
	for _, block := range blocks {
		switch block["type"] {
		case "image":
			images = append(images, map[string]string{
				"type":     "image",
				"mimeType": block["mimeType"],
				"data":     block["data"],
			})
		case "resource_link":
			if uri := block["uri"]; uri != "" {
				fileRefs = append(fileRefs, uri)
			}
		}
	}
	if len(fileRefs) > 0 {
		fields["message"] = text + "\n\nAttached source files:\n" + strings.Join(fileRefs, "\n")
	}
	if len(images) > 0 {
		fields["images"] = images
	}
	return fields, nil
}

func (h *ResearchManager) ClearQueue(ctx context.Context, root string) (ResearchQueueState, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return ResearchQueueState{}, err
	}
	var cleared ResearchQueueState
	if err := s.rpc.Call("clear_queue", nil, &cleared); err != nil {
		return ResearchQueueState{}, err
	}
	h.mu.Lock()
	s.queue = ResearchQueueState{}
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "queue", EventType: "clear_queue", Queue: &ResearchQueueState{}})
	return cleared, nil
}

func (h *ResearchManager) Abort(ctx context.Context, root string) error {
	return h.Cancel(ctx, root)
}

func (h *ResearchManager) Cancel(ctx context.Context, root string) error {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil || !s.busy {
		h.mu.Unlock()
		return err
	}
	s.cancelled = true
	if q := s.pending; q != nil {
		select {
		case q.answer <- uiDialogAnswer{payload: map[string]any{"cancelled": true}}:
		default:
		}
	}
	turn := s.turn
	rpc := s.rpc
	h.mu.Unlock()

	go func() {
		_ = rpc.Call("abort", nil, nil)
		s.notifySettled(true)
	}()
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
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	s := h.sessions[root]
	h.mu.Unlock()
	if s != nil && s.win == desktop.Window(ctx) {
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
	rpc := s.rpc
	work := s.work
	afterStop := h.afterStop
	h.mu.Unlock()

	s.cancelSettleTimer()
	if offWindow != nil {
		offWindow()
	}
	if rpc != nil {
		stopResearchProcess(rpc.Command())
		rpc.Kill()
	}
	<-s.ready
	if turn != nil {
		<-turn
	}
	if afterStop != nil && work != "" {
		afterStop(root, work)
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
	var offs []func()
	for win, fw := range h.focusWatchers {
		if fw != nil && fw.off != nil {
			offs = append(offs, fw.off)
		}
		delete(h.focusWatchers, win)
	}
	h.mu.Unlock()
	for _, off := range offs {
		off()
	}
	for root, s := range sessions {
		h.stop(root, s, "")
	}
}

func (s *researchSession) cancelSettleTimer() {
	s.settleMu.Lock()
	if s.settleTimer != nil {
		s.settleTimer.Stop()
		s.settleTimer = nil
	}
	s.settleMu.Unlock()
}

func (s *researchSession) scheduleSettleTimer(delay time.Duration) {
	s.settleMu.Lock()
	if s.settleTimer != nil {
		s.settleTimer.Stop()
	}
	if ended := s.legacyEnded; ended != nil {
		s.settleTimer = time.AfterFunc(delay, func() {
			select {
			case ended <- struct{}{}:
			default:
			}
		})
	}
	s.settleMu.Unlock()
}

func (s *researchSession) notifySettled(aborted bool) {
	s.settleMu.Lock()
	if s.settleTimer != nil {
		s.settleTimer.Stop()
		s.settleTimer = nil
	}
	settled := s.settled
	s.settleMu.Unlock()
	if settled != nil {
		select {
		case settled <- aborted:
		default:
		}
	}
}

func filterResearchCommands(commands []ResearchCommand) []ResearchCommand {
	var out []ResearchCommand
	for _, cmd := range commands {
		switch cmd.Name {
		case "", "new", "quit", "exit":
			continue
		}
		if strings.ContainsAny(cmd.Name, " \t\r\n/") {
			continue
		}
		if slices.ContainsFunc(out, func(c ResearchCommand) bool { return c.Name == cmd.Name }) {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func modelSupportsImages(current *ResearchModel, models []ResearchModel) bool {
	if current != nil {
		return len(current.Input) == 0 || slices.Contains(current.Input, "image")
	}
	for _, m := range models {
		if slices.Contains(m.Input, "image") {
			return true
		}
	}
	return true
}

func buildResearchConfig(state ResearchSessionState, models []ResearchModel, levels []string) []ResearchConfig {
	var cfg []ResearchConfig
	if len(models) > 0 {
		currentModelVal := ""
		if state.Model != nil {
			if state.Model.Provider != "" {
				currentModelVal = state.Model.Provider + "/" + state.Model.ID
			} else {
				currentModelVal = state.Model.ID
			}
		}
		opts := make([]ResearchConfigValue, 0, len(models))
		for _, m := range models {
			val := m.ID
			if m.Provider != "" {
				val = m.Provider + "/" + m.ID
			}
			label := m.Name
			if label == "" {
				label = val
			}
			opts = append(opts, ResearchConfigValue{Value: val, Name: label})
		}
		cfg = append(cfg, ResearchConfig{
			ID:           "model",
			Name:         "Model",
			Type:         "select",
			Category:     "model",
			CurrentValue: currentModelVal,
			Options:      opts,
		})
	}
	if len(levels) > 0 {
		opts := make([]ResearchConfigValue, 0, len(levels))
		for _, lvl := range levels {
			label := strings.ToUpper(lvl[:1]) + lvl[1:]
			opts = append(opts, ResearchConfigValue{Value: lvl, Name: label})
		}
		cur := state.ThinkingLevel
		if cur == "" {
			cur = "medium"
		}
		cfg = append(cfg, ResearchConfig{
			ID:           "thinking_level",
			Name:         "Thinking",
			Type:         "select",
			Category:     "thinking",
			CurrentValue: cur,
			Options:      opts,
		})
	}
	return cfg
}

func (s *researchSession) replayMessage(raw json.RawMessage) {
	var msg struct {
		Role         string          `json:"role"`
		ResponseID   string          `json:"responseId"`
		Content      json.RawMessage `json:"content"`
		Display      bool            `json:"display"`
		Summary      string          `json:"summary"`
		ToolCallID   string          `json:"toolCallId"`
		ToolName     string          `json:"toolName"`
		IsError      bool            `json:"isError"`
		StopReason   string          `json:"stopReason"`
		ErrorMessage string          `json:"errorMessage"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return
	}
	msgID := msg.ResponseID
	if msgID == "" {
		msgID = newRandomID()
	}
	switch msg.Role {
	case "user":
		for _, b := range decodeContentBlocks(msg.Content) {
			text, img := b.display()
			if text != "" || img != "" {
				s.emit(ResearchUpdate{Kind: "user", Text: text, Image: img, MessageID: msgID})
			}
		}
	case "assistant":
		for _, b := range decodeContentBlocks(msg.Content) {
			switch b.Type {
			case "thinking":
				if b.Thinking != "" {
					s.emit(ResearchUpdate{Kind: "thought", Text: b.Thinking, MessageID: msgID})
				}
			case "toolCall":
				if b.ID != "" {
					s.emit(ResearchUpdate{
						Kind: "tool",
						Tool: &ResearchTool{
							ToolCallID: b.ID,
							Title:      b.Name,
							Kind:       b.Name,
							Status:     "completed",
							Input:      researchJSONPreview(b.Arguments),
							Locations:  extractLocationsFromArgs(b.Arguments),
						},
					})
				}
			default:
				text, img := b.display()
				if text != "" || img != "" {
					s.emit(ResearchUpdate{Kind: "assistant", Text: text, Image: img, MessageID: msgID})
				}
			}
		}
	case "toolResult":
		if msg.ToolCallID != "" {
			status := "completed"
			if msg.IsError {
				status = "failed"
			}
			out, imgs := renderContentBlocks(decodeContentBlocks(msg.Content))
			s.emit(ResearchUpdate{
				Kind: "tool",
				Tool: &ResearchTool{
					ToolCallID: msg.ToolCallID,
					Title:      msg.ToolName,
					Kind:       msg.ToolName,
					Status:     status,
					Output:     out,
					Images:     imgs,
				},
			})
		}
	case "custom":
		if msg.Display {
			text := extractRawContentText(msg.Content)
			if text != "" {
				s.emit(ResearchUpdate{Kind: "assistant", Text: text, MessageID: msgID})
			}
		}
	case "compactionSummary", "branchSummary":
		if msg.Summary != "" {
			s.emit(ResearchUpdate{Kind: "notice", Text: msg.Summary})
		}
	}
}

func decodeContentBlocks(raw json.RawMessage) []researchContentBlock {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) == nil && text != "" {
			return []researchContentBlock{{Type: "text", Text: text}}
		}
		return nil
	}
	var blocks []researchContentBlock
	_ = json.Unmarshal(raw, &blocks)
	return blocks
}

func renderContentBlocks(blocks []researchContentBlock) (string, []string) {
	var parts []string
	var images []string
	for _, b := range blocks {
		text, img := b.display()
		if text != "" {
			parts = append(parts, text)
		}
		if img != "" {
			images = append(images, img)
		}
	}
	return researchPreview(strings.Join(parts, "\n")), images
}

func extractLocationsFromArgs(args json.RawMessage) []ResearchToolLocation {
	if len(args) == 0 {
		return nil
	}
	var obj struct {
		Path     string `json:"path"`
		File     string `json:"file"`
		FilePath string `json:"filePath"`
		Line     int    `json:"line"`
	}
	if json.Unmarshal(args, &obj) != nil {
		return nil
	}
	p := obj.Path
	if p == "" {
		p = obj.FilePath
	}
	if p == "" {
		p = obj.File
	}
	if p == "" {
		return nil
	}
	return []ResearchToolLocation{{Path: p, Line: obj.Line}}
}

func (h *ResearchManager) handleRPCEvent(s *researchSession, eventType string, raw json.RawMessage) {
	select {
	case <-s.stopped:
		return
	default:
	}

	switch eventType {
	case "agent_start":
		s.cancelSettleTimer()
		s.emit(ResearchUpdate{Kind: "progress", EventType: "agent_start", Text: "Working…"})

	case "turn_start":
		s.cancelSettleTimer()
		s.emit(ResearchUpdate{Kind: "progress", EventType: "turn_start", Text: "Thinking…"})

	case "turn_end":
		s.emit(ResearchUpdate{Kind: "turnEnd", EventType: "turn_end"})

	case "agent_end":
		var ev struct {
			WillRetry bool `json:"willRetry"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{Kind: "agentEnd", EventType: "agent_end", WillRetry: ev.WillRetry})
		if !ev.WillRetry {
			s.scheduleSettleTimer(15 * time.Millisecond)
		}

	case "agent_settled":
		var ev struct {
			Aborted bool `json:"aborted"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{Kind: "agentSettled", EventType: "agent_settled", Aborted: ev.Aborted})
		s.notifySettled(ev.Aborted)

	case "message_start":
		var ev struct {
			Message struct {
				Role       string `json:"role"`
				ResponseID string `json:"responseId"`
			} `json:"message"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.streamMu.Lock()
		if ev.Message.ResponseID != "" {
			s.streamMsgID = ev.Message.ResponseID
		} else {
			s.streamMsgID = newRandomID()
		}
		s.streamedText = false
		s.streamedThink = false
		s.streamMu.Unlock()

	case "message_update":
		var ev struct {
			AssistantMessageEvent struct {
				Type         string          `json:"type"`
				ContentIndex int             `json:"contentIndex"`
				Delta        string          `json:"delta"`
				Content      string          `json:"content"`
				ID           string          `json:"id"`
				ToolName     string          `json:"toolName"`
				ToolCall     json.RawMessage `json:"toolCall"`
			} `json:"assistantMessageEvent"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		ame := ev.AssistantMessageEvent
		s.streamMu.Lock()
		if s.streamMsgID == "" {
			s.streamMsgID = newRandomID()
		}
		msgID := s.streamMsgID
		switch ame.Type {
		case "text_delta":
			if ame.Delta != "" {
				s.streamedText = true
				s.streamMu.Unlock()
				s.emit(ResearchUpdate{Kind: "assistant", EventType: "message_update", Text: ame.Delta, MessageID: msgID})
				return
			}
		case "text_end":
			if !s.streamedText && ame.Content != "" {
				s.streamedText = true
				s.streamMu.Unlock()
				s.emit(ResearchUpdate{Kind: "assistant", EventType: "message_update", Text: ame.Content, MessageID: msgID})
				return
			}
		case "thinking_delta":
			// The RPC exposes thinking_delta/thinking_end without a public or
			// summary marker, so there is no reliable raw-private payload to gate.
			// Keep the established, user-disclosed reasoning stream unchanged.
			if ame.Delta != "" {
				s.streamedThink = true
				s.streamMu.Unlock()
				s.emit(ResearchUpdate{Kind: "thought", EventType: "message_update", Text: ame.Delta, MessageID: msgID})
				return
			}
		case "thinking_end":
			if !s.streamedThink && ame.Content != "" {
				s.streamedThink = true
				s.streamMu.Unlock()
				s.emit(ResearchUpdate{Kind: "thought", EventType: "message_update", Text: ame.Content, MessageID: msgID})
				return
			}
		}
		s.streamMu.Unlock()

	case "message_end":
		var ev struct {
			Message struct {
				Role         string          `json:"role"`
				ResponseID   string          `json:"responseId"`
				Content      json.RawMessage `json:"content"`
				Display      bool            `json:"display"`
				StopReason   string          `json:"stopReason"`
				ErrorMessage string          `json:"errorMessage"`
			} `json:"message"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		s.streamMu.Lock()
		msgID := ev.Message.ResponseID
		if msgID == "" {
			msgID = s.streamMsgID
		}
		if msgID == "" {
			msgID = newRandomID()
		}
		alreadyText := s.streamedText
		alreadyThink := s.streamedThink
		s.streamMsgID = ""
		s.streamedText = false
		s.streamedThink = false
		s.streamMu.Unlock()

		switch ev.Message.Role {
		case "assistant":
			for _, b := range decodeContentBlocks(ev.Message.Content) {
				switch b.Type {
				case "thinking":
					if !alreadyThink && b.Thinking != "" {
						s.emit(ResearchUpdate{Kind: "thought", EventType: "message_end", Text: b.Thinking, MessageID: msgID})
					}
				case "text":
					if !alreadyText && b.Text != "" {
						s.emit(ResearchUpdate{Kind: "assistant", EventType: "message_end", Text: b.Text, MessageID: msgID})
					}
				case "image":
					text, img := b.display()
					if text != "" || img != "" {
						s.emit(ResearchUpdate{Kind: "assistant", EventType: "message_end", Text: text, Image: img, MessageID: msgID})
					}
				}
			}
			if ev.Message.StopReason == "error" && ev.Message.ErrorMessage != "" {
				h.mu.Lock()
				s.turnErr = errors.New(ev.Message.ErrorMessage)
				h.mu.Unlock()
				s.emit(ResearchUpdate{Kind: "notice", EventType: "message_end", Text: ev.Message.ErrorMessage})
			}
		case "custom":
			if ev.Message.Display {
				if text := extractRawContentText(ev.Message.Content); text != "" {
					s.emit(ResearchUpdate{Kind: "assistant", EventType: "message_end", Text: text, MessageID: msgID})
				}
			}
		}

	case "tool_execution_start", "tool_execution_update", "tool_execution_end":
		if tool := decodeRPCToolEvent(eventType, raw); tool != nil {
			s.emit(ResearchUpdate{Kind: "tool", EventType: eventType, Tool: tool})
		}

	case "compaction_start", "auto_compaction_start":
		s.cancelSettleTimer()
		var ev struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{
			Kind:      "compactionStart",
			EventType: eventType,
			Reason:    ev.Reason,
			Text:      "Compacting conversation context…",
		})

	case "compaction_end", "auto_compaction_end":
		var ev struct {
			Reason       string                    `json:"reason"`
			Result       *ResearchCompactionResult `json:"result"`
			Aborted      bool                      `json:"aborted"`
			WillRetry    bool                      `json:"willRetry"`
			ErrorMessage string                    `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{
			Kind:       "compactionEnd",
			EventType:  eventType,
			Reason:     ev.Reason,
			Compaction: ev.Result,
			Aborted:    ev.Aborted,
			WillRetry:  ev.WillRetry,
			Text:       ev.ErrorMessage,
		})

	case "auto_retry_start", "summarization_retry_scheduled":
		s.cancelSettleTimer()
		var ev struct {
			Attempt      int    `json:"attempt"`
			MaxAttempts  int    `json:"maxAttempts"`
			DelayMs      int64  `json:"delayMs"`
			ErrorMessage string `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{
			Kind:             "retryStart",
			EventType:        eventType,
			RetryAttempt:     ev.Attempt,
			RetryMaxAttempts: ev.MaxAttempts,
			RetryDelayMs:     ev.DelayMs,
			Text:             ev.ErrorMessage,
		})

	case "auto_retry_end", "summarization_retry_finished":
		var ev struct {
			Success    bool   `json:"success"`
			Attempt    int    `json:"attempt"`
			FinalError string `json:"finalError"`
		}
		_ = json.Unmarshal(raw, &ev)
		s.emit(ResearchUpdate{
			Kind:         "retryEnd",
			EventType:    eventType,
			RetryAttempt: ev.Attempt,
			RetrySuccess: ev.Success,
			Text:         ev.FinalError,
		})

	case "queue_update":
		var ev ResearchQueueState
		if json.Unmarshal(raw, &ev) == nil {
			h.mu.Lock()
			s.queue = ev
			h.mu.Unlock()
			s.emit(ResearchUpdate{Kind: "queue", EventType: "queue_update", Queue: &ev})
		}

	case "session_info_changed":
		var ev struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &ev) == nil {
			h.mu.Lock()
			s.sessionName = ev.Name
			s.state.SessionName = ev.Name
			h.mu.Unlock()
			s.emit(ResearchUpdate{Kind: "session", EventType: "session_info_changed", Text: ev.Name})
		}

	case "thinking_level_changed":
		var ev struct {
			Level string `json:"level"`
		}
		if json.Unmarshal(raw, &ev) == nil && ev.Level != "" {
			h.mu.Lock()
			s.thinkingLevel = ev.Level
			s.state.ThinkingLevel = ev.Level
			cfg := buildResearchConfig(s.state, s.models, s.thinkingLevels)
			s.config = cfg
			stateCopy := s.state
			h.mu.Unlock()
			s.emit(ResearchUpdate{
				Kind:          "config",
				EventType:     "thinking_level_changed",
				Config:        cfg,
				ThinkingLevel: ev.Level,
				State:         &stateCopy,
			})
		}

	case "extension_error":
		var ev struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &ev) == nil && ev.Error != "" {
			s.emit(ResearchUpdate{Kind: "notice", EventType: "extension_error", Text: ev.Error})
		}
	}
}

func decodeRPCToolEvent(eventType string, raw json.RawMessage) *ResearchTool {
	var ev struct {
		ToolCallID    string          `json:"toolCallId"`
		ToolName      string          `json:"toolName"`
		Args          json.RawMessage `json:"args"`
		PartialResult json.RawMessage `json:"partialResult"`
		Result        json.RawMessage `json:"result"`
		IsError       bool            `json:"isError"`
		DurationMs    int64           `json:"durationMs"`
	}
	if json.Unmarshal(raw, &ev) != nil || ev.ToolCallID == "" {
		return nil
	}
	tool := &ResearchTool{
		ToolCallID: ev.ToolCallID,
		Title:      ev.ToolName,
		Kind:       ev.ToolName,
		DurationMs: ev.DurationMs,
	}
	if len(ev.Args) > 0 {
		tool.Input = researchJSONPreview(ev.Args)
		tool.Locations = extractLocationsFromArgs(ev.Args)
	}
	switch eventType {
	case "tool_execution_start":
		tool.Status = "in_progress"
	case "tool_execution_update":
		tool.Status = "in_progress"
		populateToolResult(tool, ev.PartialResult)
	case "tool_execution_end":
		if ev.IsError {
			tool.Status = "failed"
		} else {
			tool.Status = "completed"
		}
		populateToolResult(tool, ev.Result)
	}
	return tool
}

func populateToolResult(tool *ResearchTool, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var payload struct {
		Content json.RawMessage `json:"content"`
		Details struct {
			Diffs []ResearchToolDiff `json:"diffs"`
			Diff  *ResearchToolDiff  `json:"diff"`
		} `json:"details"`
		Diffs []ResearchToolDiff `json:"diffs"`
	}
	if json.Unmarshal(raw, &payload) == nil && (len(payload.Content) > 0 || len(payload.Details.Diffs) > 0 || payload.Details.Diff != nil || len(payload.Diffs) > 0) {
		out, imgs := renderContentBlocks(decodeContentBlocks(payload.Content))
		tool.Output = out
		tool.Images = imgs
		for _, d := range append(payload.Details.Diffs, payload.Diffs...) {
			d.OldText = researchPreview(d.OldText)
			d.NewText = researchPreview(d.NewText)
			tool.Diffs = append(tool.Diffs, d)
		}
		if payload.Details.Diff != nil {
			d := *payload.Details.Diff
			d.OldText = researchPreview(d.OldText)
			d.NewText = researchPreview(d.NewText)
			tool.Diffs = append(tool.Diffs, d)
		}
		return
	}
	tool.Output = researchJSONPreview(raw)
}

type wireExtensionUIRequest struct {
	ID              string   `json:"id"`
	Method          string   `json:"method"`
	Title           string   `json:"title"`
	Message         string   `json:"message"`
	Options         []string `json:"options"`
	Placeholder     string   `json:"placeholder"`
	Prefill         string   `json:"prefill"`
	Timeout         int64    `json:"timeout"`
	NotifyType      string   `json:"notifyType"`
	StatusKey       string   `json:"statusKey"`
	StatusText      string   `json:"statusText"`
	WidgetKey       string   `json:"widgetKey"`
	WidgetLines     []string `json:"widgetLines"`
	WidgetPlacement string   `json:"widgetPlacement"`
	Text            string   `json:"text"`
}

func (h *ResearchManager) handleExtensionUIRequest(s *researchSession, raw json.RawMessage) {
	select {
	case <-s.stopped:
		return
	default:
	}
	var req wireExtensionUIRequest
	if json.Unmarshal(raw, &req) != nil || req.Method == "" {
		return
	}
	switch req.Method {
	case "notify":
		notifyType := req.NotifyType
		if notifyType == "" {
			notifyType = "info"
		}
		s.emit(ResearchUpdate{
			Kind:       "notify",
			UIMethod:   "notify",
			RequestID:  req.ID,
			Message:    req.Message,
			Text:       req.Message,
			NotifyType: notifyType,
		})
	case "setStatus":
		s.emit(ResearchUpdate{
			Kind:      "setStatus",
			UIMethod:  "setStatus",
			RequestID: req.ID,
			StatusKey: req.StatusKey,
			Key:       "status:" + req.StatusKey,
			Text:      req.StatusText,
		})
	case "setWidget":
		s.emit(ResearchUpdate{
			Kind:            "setWidget",
			UIMethod:        "setWidget",
			RequestID:       req.ID,
			WidgetKey:       req.WidgetKey,
			Key:             "widget:" + req.WidgetKey,
			WidgetLines:     req.WidgetLines,
			WidgetPlacement: req.WidgetPlacement,
			Text:            strings.Join(req.WidgetLines, "\n"),
		})
	case "setTitle":
		s.emit(ResearchUpdate{
			Kind:      "setTitle",
			UIMethod:  "setTitle",
			RequestID: req.ID,
			Title:     req.Title,
			Text:      req.Title,
		})
	case "set_editor_text":
		s.emit(ResearchUpdate{
			Kind:      "editorText",
			UIMethod:  "set_editor_text",
			RequestID: req.ID,
			Text:      req.Text,
		})
	case "select", "confirm", "input", "editor":
		go h.runExtensionUIDialog(s, req)
	}
}

func (h *ResearchManager) runExtensionUIDialog(s *researchSession, req wireExtensionUIRequest) {
	select {
	case s.questionSlot <- struct{}{}:
		defer func() { <-s.questionSlot }()
	case <-s.stopped:
		return
	}

	reqID := req.ID
	if reqID == "" {
		reqID = newRandomID()
	}

	var options []ResearchOption
	text := req.Title
	switch req.Method {
	case "select":
		options = make([]ResearchOption, 0, len(req.Options))
		for _, opt := range req.Options {
			options = append(options, ResearchOption{ID: opt, Name: opt})
		}
	case "confirm":
		if req.Message != "" {
			if text != "" {
				text = strings.TrimSpace(text + "\n" + req.Message)
			} else {
				text = req.Message
			}
		}
		options = []ResearchOption{
			{ID: "yes", Name: "Confirm", Kind: "confirm"},
			{ID: "no", Name: "Cancel", Kind: "cancel"},
		}
	}

	q := &researchQuestion{
		id:      reqID,
		piID:    req.ID,
		method:  req.Method,
		options: options,
		answer:  make(chan uiDialogAnswer, 1),
	}

	h.mu.Lock()
	select {
	case <-s.stopped:
		h.mu.Unlock()
		return
	default:
	}
	if s.cancelled {
		h.mu.Unlock()
		_ = s.rpc.SendUIResponse(req.ID, map[string]any{"cancelled": true})
		return
	}
	s.pending = q
	rpc := s.rpc
	h.mu.Unlock()

	s.emit(ResearchUpdate{
		Kind:        req.Method,
		UIMethod:    req.Method,
		RequestID:   q.id,
		Title:       req.Title,
		Message:     req.Message,
		Text:        text,
		Placeholder: req.Placeholder,
		Prefill:     req.Prefill,
		Timeout:     req.Timeout,
		Options:     options,
	})

	var timeoutCh <-chan time.Time
	if req.Timeout > 0 {
		timer := time.NewTimer(time.Duration(req.Timeout) * time.Millisecond)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	var exited <-chan struct{}
	if rpc != nil {
		exited = rpc.Exited()
	}

	ans := uiDialogAnswer{payload: map[string]any{"cancelled": true}}
	select {
	case ans = <-q.answer:
	case <-timeoutCh:
	case <-s.stopped:
	case <-exited:
	}

	h.mu.Lock()
	if s.pending == q {
		s.pending = nil
	}
	h.mu.Unlock()

	if rpc != nil && req.ID != "" {
		_ = rpc.SendUIResponse(req.ID, ans.payload)
	}
	s.emit(ResearchUpdate{Kind: "permissionDone", RequestID: q.id})
}

func (h *ResearchManager) Answer(ctx context.Context, root, requestID, optionID string) error {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	q := s.pending
	if q == nil || q.id != requestID || s.cancelled || (q.method != "select" && q.method != "confirm") {
		return errors.New("This research permission or choice is no longer available.")
	}
	if optionID == "" {
		s.pending = nil
		q.answer <- uiDialogAnswer{payload: map[string]any{"cancelled": true}}
		return nil
	}
	if q.method == "confirm" {
		switch strings.ToLower(optionID) {
		case "yes", "true", "confirm":
			s.pending = nil
			q.answer <- uiDialogAnswer{payload: map[string]any{"confirmed": true}}
			return nil
		case "no", "false", "cancel":
			s.pending = nil
			q.answer <- uiDialogAnswer{payload: map[string]any{"confirmed": false}}
			return nil
		default:
			return errors.New("This research permission or choice is no longer available.")
		}
	}
	for i, opt := range q.options {
		if opt.ID == optionID || opt.Name == optionID || fmt.Sprintf("choice-%d", i) == optionID {
			s.pending = nil
			q.answer <- uiDialogAnswer{payload: map[string]any{"value": opt.Name}}
			return nil
		}
	}
	return errors.New("This research permission or choice is no longer available.")
}

func (h *ResearchManager) AnswerConfirm(ctx context.Context, root, requestID string, confirmed, cancelled bool) error {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	q := s.pending
	if q == nil || q.id != requestID || s.cancelled || q.method != "confirm" {
		return errors.New("This research confirmation is no longer available.")
	}
	s.pending = nil
	if cancelled {
		q.answer <- uiDialogAnswer{payload: map[string]any{"cancelled": true}}
	} else {
		q.answer <- uiDialogAnswer{payload: map[string]any{"confirmed": confirmed}}
	}
	return nil
}

func (h *ResearchManager) AnswerText(ctx context.Context, root, requestID, text string, cancelled bool) error {
	root = fsutil.CanonicalPath(root)
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	q := s.pending
	if q == nil || q.id != requestID || s.cancelled || (q.method != "input" && q.method != "editor") {
		return errors.New("This Feynman input request is no longer available.")
	}
	if !cancelled && len(text) > maxResearchUIBytes {
		return errors.New("The response exceeds the 1 MiB limit.")
	}
	s.pending = nil
	if cancelled {
		q.answer <- uiDialogAnswer{payload: map[string]any{"cancelled": true}}
	} else {
		q.answer <- uiDialogAnswer{payload: map[string]any{"value": text}}
	}
	return nil
}
