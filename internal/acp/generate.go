package acp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
)

var acpClientInfo = map[string]string{"name": ClientName, "version": ClientVersion}

var acpInitializeParams = map[string]any{
	"protocolVersion": ProtocolVersion,
	"clientInfo":      acpClientInfo,
	"clientCapabilities": map[string]any{
		"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
		"terminal": false,
	},
}

const defaultGenerationTimeout = 5 * time.Minute

const acpPromptCancelAckTimeout = time.Second

const acpSessionCloseTimeout = time.Second

type ModelUpdater interface {
	SetModels(models []string) error
}

type GenerationJob struct {
	Req          genui.GenerateRequest
	AgentCommand string
	Model        string
	ViewPath     string
	Now          func() time.Time
	Models       ModelUpdater
	Timeout      time.Duration

	Env []string

	BeforeWrite func() error

	Validate func(context.Context, []byte) error

	Observe func(event string, payload any)

	ThoughtLevel string

	AgentCache *ProcessCache

	Prepare func(context.Context, genui.GenerateRequest, string) (string, func([]byte) ([]byte, error), error)
}

func ResolveAgentCommand(command string) (string, error) {
	if resolved, ok := fsutil.ResolveExecutable(command, nil, fsutil.DefaultUserBinDirs()); ok {
		return resolved, nil
	}
	return "", genui.NewFailure(genui.FailAgentNotFound, fmt.Sprintf("Could not find the agent command %q.", command), genui.HintAgentNotFound)
}

func RunGeneration(ctx context.Context, job GenerationJob, progress func(genui.GenerateProgress) error) (err error) {
	if progress == nil {
		progress = func(genui.GenerateProgress) error { return nil }
	}
	trace := newGenerationTrace(&job)
	if trace != nil {
		defer func() { trace.end(err) }()
	}

	var base *genui.BaseView
	switch job.Req.Mode {
	case "full":
	case "update":
		b, err := genui.ReadBaseView(job.ViewPath, job.Req.ReplaceIDs)
		if err != nil {
			return err
		}
		base = b
	default:
		return genui.AgentErrorf("Unknown generation mode %q: only \"full\" and \"update\" are supported.", job.Req.Mode)
	}
	if job.Now == nil {
		job.Now = time.Now
	}
	if job.Timeout <= 0 {
		job.Timeout = defaultGenerationTimeout
	}
	ctx, stop := context.WithTimeoutCause(ctx, job.Timeout, genui.ErrTimedOut)
	defer stop()
	interrupted := func() error {
		if context.Cause(ctx) == genui.ErrTimedOut {
			return genui.TimedOut(job.Timeout)
		}
		return genui.NewFailure(genui.FailCancelled, "Generation was cancelled.", "")
	}

	dir, err := os.MkdirTemp("", "markgogo-genui-")
	if err != nil {
		return genui.AgentErrorf("Could not make the agent's folder: %v.", err)
	}
	defer os.RemoveAll(dir)
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return genui.AgentErrorf("Could not make the agent's folder: %v.", err)
	}

	trace.runStart(&job, dir)
	if err := os.WriteFile(filepath.Join(dir, "source.md"), []byte(job.Req.Text), 0o644); err != nil {
		return genui.AgentErrorf("Could not write the agent's source file: %v.", err)
	}
	if trace != nil {
		trace.artifact("source.md", filepath.Join(dir, "source.md"), job.Req.Text)
	}
	if base != nil {
		if err := os.WriteFile(filepath.Join(dir, "view.json"), base.Raw(), 0o644); err != nil {
			return genui.AgentErrorf("Could not write the agent's copy of the view: %v.", err)
		}
	}

	send := &progressSender{send: progress, trace: trace}
	if err := send.step("starting", "Starting the agent"); err != nil {
		return interrupted()
	}

	var promptText string
	var assembleOutput func([]byte) ([]byte, error)
	if job.Prepare != nil {
		promptText, assembleOutput, err = job.Prepare(ctx, job.Req, dir)
		if err != nil {
			if ctx.Err() != nil {
				return interrupted()
			}
			return genui.AgentErrorf("Could not prepare the controlled reading view: %v.", err)
		}
	}

	resolved, err := ResolveAgentCommand(job.AgentCommand)
	if err != nil {
		return err
	}

	run := &genRun{
		job:            job,
		dir:            dir,
		send:           send,
		base:           base,
		trace:          trace,
		promptText:     promptText,
		assembleOutput: assembleOutput,
		abortCh:        make(chan struct{}),
		toolCalls:      make(map[string]toolCallState),
	}
	var agent *Agent
	if job.AgentCache != nil {
		lease, err := job.AgentCache.lease(resolved, job.Env)
		if err != nil {
			return genui.NewFailure(genui.FailAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", job.AgentCommand, err), genui.HintAgentNotFound)
		}
		run.lease = lease
		agent = lease.agent
		if lease.reused {
			trace.agentReused(resolved, lease.args, agent.PID(), lease.cwd, lease.startedAt, lease.idleSince)
		} else {
			trace.agentStart(resolved, lease.args, agent.PID(), lease.cwd)
		}
	} else {
		cmd := exec.Command(resolved, "acp")
		cmd.Dir = dir
		cmd.Env = job.Env
		agent, err = NewAgent(cmd, fsutil.NewLimitBuffer(4<<10))
		if err != nil {
			return genui.NewFailure(genui.FailAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", job.AgentCommand, err), genui.HintAgentNotFound)
		}
		trace.agentStart(resolved, cmd.Args, agent.PID(), dir)
	}
	run.agent = agent
	var observe func(direction string, line []byte)
	if trace != nil {
		observe = func(direction string, line []byte) {
			if direction == Received && !trace.acceptsInbound(line) {
				return
			}
			event := TraceACPSent
			if direction == Received {
				event = TraceACPReceived
			}
			trace.emit(event, &traceACPPayload{Direction: direction, Line: json.RawMessage(line)})
		}
	}
	agent.SetHandlers(run.handleRequest, run.handleNotification, observe)
	if run.lease == nil || !run.lease.reused {
		agent.Start()
	}
	defer run.finishAgent()

	err = run.execute(ctx)
	if err == nil {
		return nil
	}
	if errors.Is(err, genui.ErrCancelled) {
		run.stopAfterCancel()
		return interrupted()
	}
	return err
}

func (r *genRun) finishAgent() {
	if r.agent != nil {
		r.agent.SetHandlers(nil, nil, nil)
	}
	if r.lease != nil {
		r.lease.release(r.sessionClosed)
		return
	}
	if r.agent != nil {
		r.agent.Kill()
	}
}

func (r *genRun) execute(ctx context.Context) error {
	var initialized struct {
		ProtocolVersion   int             `json:"protocolVersion"`
		AgentInfo         json.RawMessage `json:"agentInfo"`
		AgentCapabilities json.RawMessage `json:"agentCapabilities"`
		AuthMethods       json.RawMessage `json:"authMethods"`
	}
	if r.lease != nil && r.lease.reused {
		handshake := r.lease.initialize
		initialized.ProtocolVersion = handshake.protocolVersion
		initialized.AgentInfo = handshake.agentInfo
		initialized.AgentCapabilities = handshake.agentCapabilities
		initialized.AuthMethods = handshake.authMethods
		r.trace.initializeCached(acpClientInfo, ProtocolVersion, handshake.protocolVersion, handshake.agentInfo, handshake.agentCapabilities, handshake.authMethods, handshake.raw, r.lease.agent.PID(), r.lease.startedAt, r.lease.cwd)
	} else {
		var initRaw json.RawMessage
		var initRawPtr *json.RawMessage
		if r.trace != nil || r.lease != nil {
			initRawPtr = &initRaw
		}
		if err := r.callAgent(ctx, "initialize", acpInitializeParams, &initialized, initRawPtr); err != nil {
			return err
		}
		r.trace.initialized(initRaw, acpClientInfo, ProtocolVersion, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		if r.lease != nil {
			r.lease.storeInitialize(initRaw, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		}
	}
	if initialized.ProtocolVersion != ProtocolVersion {
		return genui.AgentErrorf("The agent speaks ACP protocol version %d, but this app speaks version %d.", initialized.ProtocolVersion, ProtocolVersion)
	}

	var opened struct {
		SessionID     string          `json:"sessionId"`
		ConfigOptions json.RawMessage `json:"configOptions"`
	}
	var openedRaw json.RawMessage
	var openedRawPtr *json.RawMessage
	if r.trace != nil {
		openedRawPtr = &openedRaw
	}
	if err := r.callAgent(ctx, "session/new", map[string]any{
		"cwd":        r.dir,
		"mcpServers": []any{},
	}, &opened, openedRawPtr); err != nil {
		return err
	}
	if r.lease != nil {
		if err := r.lease.acceptSession(opened.SessionID); err != nil {
			return err
		}
	}
	r.setSessionID(opened.SessionID)
	r.trace.sessionNew(openedRaw, opened.SessionID)
	if models := configValues(opened.ConfigOptions, "model"); len(models) > 0 && r.job.Models != nil {
		_ = r.job.Models.SetModels(models)
	}
	if err := r.send.step("starting", "Agent ready"); err != nil {
		return err
	}

	if r.job.Model != "" {
		var setRaw json.RawMessage
		r.markModelRequested()
		if err := r.callAgent(ctx, "session/set_config_option", map[string]any{
			"sessionId": r.session(),
			"configId":  traceModelOption,
			"value":     r.job.Model,
		}, nil, &setRaw); err != nil {
			r.trace.configSetError(traceModelOption, r.job.Model, err)
			var ge *genui.GenError
			if models := configValues(opened.ConfigOptions, "model"); len(models) > 0 && errors.As(err, &ge) && ge.Kind == genui.FailAgentError {
				invalid := genui.NewFailure(genui.FailModelInvalid, fmt.Sprintf("The agent does not know the model %q.", r.job.Model), genui.HintModelInvalid)
				invalid.Models = models
				return invalid
			}
			return err
		}
		r.trace.configSetResponse(traceModelOption, r.job.Model, setRaw)
		r.noteModelOptions(configOptionsField(setRaw))
		if err := r.modelProblem(); err != nil {
			return err
		}
	}

	if err := r.applyThoughtLevel(ctx, opened.ConfigOptions); err != nil {
		return err
	}

	promptText := r.promptText
	if promptText == "" {
		promptText = genui.GenerationPrompt(r.job.Req, r.base, r.dir)
	}
	r.trace.prompt(promptText)
	var turned struct {
		StopReason string `json:"stopReason"`
	}
	var turnedRaw json.RawMessage
	var turnedRawPtr *json.RawMessage
	if r.trace != nil {
		turnedRawPtr = &turnedRaw
	}
	if err := r.callAgent(ctx, "session/prompt", map[string]any{
		"sessionId": r.session(),
		"prompt":    []map[string]any{{"type": "text", "text": promptText}},
	}, &turned, turnedRawPtr); err != nil {
		if errors.Is(err, genui.ErrCancelled) && r.turnWasAcknowledged() {
			r.settleSession()
		}
		return err
	}
	r.trace.promptResult(turnedRaw)
	switch turned.StopReason {
	case "", "end_turn":
	case "cancelled":
		r.settleSession()
		return genui.ErrCancelled
	case "refusal":
		r.settleSession()
		refused := genui.NewFailure(genui.FailAgentMessage, "The agent declined to make the view.", genui.HintAgentMessage)
		refused.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return refused
	case "max_tokens", "max_turn_requests":
		r.settleSession()
		stopped := genui.NewFailure(genui.FailAgentMessage, "The agent stopped before finishing the view.", genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return stopped
	default:
		r.settleSession()
		stopped := genui.NewFailure(genui.FailAgentMessage, fmt.Sprintf("The agent stopped before finishing the view (%s).", turned.StopReason), genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return stopped
	}
	if err := r.modelProblem(); err != nil {
		r.settleSession()
		return err
	}
	if err := r.send.step("checking", "Checking the view"); err != nil {
		r.settleSession()
		return err
	}

	if r.lease == nil {
		r.agent.Kill()
	} else if !r.settleSession() {
		r.agent.Kill()
	}
	return r.collect(ctx)
}

func (r *genRun) applyThoughtLevel(ctx context.Context, optionsRaw json.RawMessage) error {
	requested := r.job.ThoughtLevel
	if requested == "" {
		return nil
	}
	opt, ok := findConfigOption(optionsRaw, traceThoughtID, "thoughtLevel")
	if !ok {
		r.trace.configSetSkipped(traceThoughtID, requested, "the agent's session/new configOptions have no thought_level option")
		return nil
	}
	offered := opt.values()
	supported := false
	for _, v := range offered {
		if v == requested {
			supported = true
			break
		}
	}
	if !supported {
		r.trace.configSetSkipped(opt.ID, requested, fmt.Sprintf("the agent's %s option does not offer %q; it offers %s", opt.ID, requested, strings.Join(genui.QuoteAll(offered), ", ")))
		return nil
	}
	var setRaw json.RawMessage
	var setRawPtr *json.RawMessage
	if r.trace != nil {
		setRawPtr = &setRaw
	}
	if err := r.callAgent(ctx, "session/set_config_option", map[string]any{
		"sessionId": r.session(),
		"configId":  opt.ID,
		"value":     requested,
	}, nil, setRawPtr); err != nil {
		r.trace.configSetError(opt.ID, requested, err)
		return err
	}
	r.trace.configSetResponse(opt.ID, requested, setRaw)
	return nil
}

type genRun struct {
	job  GenerationJob
	dir  string
	send *progressSender
	base *genui.BaseView

	trace *generationTrace

	promptText     string
	assembleOutput func([]byte) ([]byte, error)

	agent *Agent
	lease *acpLease

	sessionMu     sync.RWMutex
	sessionID     string
	sessionClosed bool

	cancelMu           sync.Mutex
	cancelAcknowledged bool

	modelMu        sync.Mutex
	modelRequested bool
	modelActual    string
	modelConfirmed bool
	modelMismatch  string

	toolCalls map[string]toolCallState

	abortCh     chan struct{}
	abortOnce   sync.Once
	abortReason string

	replyMu sync.Mutex
	reply   strings.Builder
}

func (r *genRun) callAgent(ctx context.Context, method string, params any, result any, raw *json.RawMessage) error {
	if ctx.Err() != nil {
		return genui.ErrCancelled
	}
	ch := make(chan error, 1)
	go func() { ch <- r.agent.CallCapturing(method, params, result, raw) }()
	select {
	case err := <-ch:
		return r.agentCallResult(method, err)
	case <-ctx.Done():
		r.cancelPromptTurn(method, ch)
		return genui.ErrCancelled
	case <-r.abortCh:
		return genui.AgentError(r.abortReason)
	}
}

func (r *genRun) agentCallResult(method string, err error) error {
	if err == nil {
		return nil
	}
	var rpc *RPCError
	if !errors.As(err, &rpc) {
		if genui.IsAuthError(0, err.Error()) {
			return genui.AuthRequired(r.job.AgentCommand, err.Error())
		}
		return genui.AgentError(err.Error())
	}
	switch {
	case genui.IsAuthError(rpc.Code, rpc.Message):
		return genui.AuthRequired(r.job.AgentCommand, rpc.Message)
	case method == "session/prompt":
		stopped := genui.NewFailure(genui.FailAgentMessage, "The agent stopped with an error.", genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(rpc.Message, genui.MaxDetailRunes)
		return stopped
	default:
		return genui.AgentErrorf("The agent answered %s with an error: %s", method, rpc.Message)
	}
}

func (r *genRun) cancelPromptTurn(method string, done <-chan error) {
	sessionID := r.session()
	if method != "session/prompt" || sessionID == "" || r.agent == nil {
		return
	}
	r.agent.CancelTurn(sessionID)
	timer := time.NewTimer(acpPromptCancelAckTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			r.markCancelAcknowledged()
		}
	case <-r.abortCh:
	case <-timer.C:
	}
}

func (r *genRun) markCancelAcknowledged() {
	r.cancelMu.Lock()
	r.cancelAcknowledged = true
	r.cancelMu.Unlock()
}

func (r *genRun) turnWasAcknowledged() bool {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	return r.cancelAcknowledged
}

func (r *genRun) settleSession() bool {
	if r.lease == nil {
		return true
	}
	if r.sessionClosed {
		return true
	}
	sessionID := r.session()
	if sessionID == "" || !r.lease.supportsClose() {
		return false
	}
	if !r.closeSessionBounded(sessionID) {
		return false
	}
	r.sessionClosed = true
	return true
}

func (r *genRun) closeSessionBounded(sessionID string) bool {
	done := make(chan error, 1)
	go func() {
		done <- r.agent.Call("session/close", map[string]any{"sessionId": sessionID}, nil)
	}()
	timer := time.NewTimer(acpSessionCloseTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err == nil
	case <-timer.C:
		return false
	}
}

func (r *genRun) stopAfterCancel() {
	if r.agent == nil {
		return
	}
	if r.sessionClosed {
		return
	}
	if r.turnWasAcknowledged() && r.settleSession() {
		return
	}
	r.agent.Kill()
}

func (r *genRun) abort(msg string) {
	r.abortOnce.Do(func() {
		r.abortReason = msg
		close(r.abortCh)
		if r.agent != nil {
			r.agent.CancelTurn(r.session())
			go r.agent.Kill()
		}
	})
}

func (r *genRun) lastReply() string {
	r.replyMu.Lock()
	defer r.replyMu.Unlock()
	return r.reply.String()
}

func (r *genRun) setSessionID(id string) {
	r.sessionMu.Lock()
	r.sessionID = id
	r.sessionMu.Unlock()
}

func (r *genRun) session() string {
	r.sessionMu.RLock()
	defer r.sessionMu.RUnlock()
	return r.sessionID
}

func (r *genRun) noteModelOptions(options json.RawMessage) {
	if r.job.Model == "" || len(options) == 0 {
		return
	}
	r.modelMu.Lock()
	requested := r.modelRequested
	r.modelMu.Unlock()
	if !requested {
		return
	}
	opt, ok := findConfigOption(options, traceModelOption)
	if !ok {
		return
	}
	current, ok := opt.current()
	if !ok {
		return
	}
	actual, ok := configString(current)
	if !ok {
		return
	}
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if actual != r.job.Model {
		if r.modelMismatch == "" {
			r.modelMismatch = fmt.Sprintf("The agent reports the model %q, not the requested %q, so the view was not changed.", actual, r.job.Model)
		}
		return
	}
	if r.modelMismatch == "" {
		r.modelActual = actual
		r.modelConfirmed = true
	}
}

func (r *genRun) markModelRequested() {
	r.modelMu.Lock()
	r.modelRequested = true
	r.modelMu.Unlock()
}

func (r *genRun) modelProblem() error {
	if r.job.Model == "" {
		return nil
	}
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if r.modelMismatch != "" {
		return genui.AgentError(r.modelMismatch)
	}
	return nil
}

func (r *genRun) modelStamp() string {
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if r.modelConfirmed && r.modelActual == r.job.Model {
		return r.modelActual
	}
	return ""
}

func (r *genRun) viewModel() string {
	if r.base != nil {
		return r.base.Model()
	}
	return r.modelStamp()
}

func (r *genRun) notificationBelongsToSession(params json.RawMessage) bool {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(params, &p) != nil || p.SessionID == "" {
		return true
	}
	return p.SessionID == r.session()
}

func (r *genRun) handleNotification(method string, params json.RawMessage) {
	if !r.notificationBelongsToSession(params) {
		return
	}
	if method == "session/config_option_update" {
		var p struct {
			ConfigOptions json.RawMessage `json:"configOptions"`
		}
		_ = json.Unmarshal(params, &p)
		opts := p.ConfigOptions
		if len(opts) == 0 {
			var upd struct {
				Update struct {
					ConfigOptions json.RawMessage `json:"configOptions"`
				} `json:"update"`
			}
			_ = json.Unmarshal(params, &upd)
			opts = upd.Update.ConfigOptions
		}
		r.trace.configOptionUpdate(method, params, opts)
		r.noteModelOptions(opts)
		return
	}
	if method != "session/update" {
		return
	}
	var upd struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       *struct {
				Text string `json:"text"`
			} `json:"content"`
			ConfigOptions json.RawMessage `json:"configOptions"`
			ToolCallID    string          `json:"toolCallId"`
			Name          string          `json:"name"`
			Kind          string          `json:"kind"`
			Title         string          `json:"title"`
			Status        string          `json:"status"`
			Locations     []Location      `json:"locations"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &upd) != nil {
		return
	}
	u := upd.Update
	switch u.SessionUpdate {
	case "agent_message_chunk":
		if u.Content != nil {
			r.replyMu.Lock()
			r.reply.WriteString(u.Content.Text)
			r.replyMu.Unlock()
		}
	case "agent_thought_chunk":
		_ = r.send.step("thinking", "Thinking")
	case "config_option_update":
		opts := u.ConfigOptions
		if len(opts) == 0 {
			opts = configOptionsField(params)
		}
		r.trace.configOptionUpdate(method, params, opts)
		r.noteModelOptions(opts)
	case "tool_call", "tool_call_update":
		r.handleToolCall(u.ToolCallID, u.Kind, u.Title, u.Name, u.Locations)
	}
}

type toolCallState struct {
	kind      string
	locations []Location
}

func notificationToolAllowed(dir, kind string, locations []string) bool {
	if kind == "" {
		for _, loc := range locations {
			if !notificationPathInside(dir, loc) {
				return false
			}
		}
		return true
	}
	switch kind {
	case "think":
		return true
	case "read", "search", "edit":
		for _, loc := range locations {
			if !genui.AgentToolAllowed(dir, kind, []string{loc}) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func notificationPathInside(dir, loc string) bool {
	resolved := fsutil.ResolveToolPath(dir, loc)
	rel, err := filepath.Rel(dir, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (r *genRun) handleToolCall(id, kind, title, name string, locations []Location) {
	prior := r.toolCalls[id]
	effectiveKind := kind
	if effectiveKind == "" {
		effectiveKind = prior.kind
	}
	effectiveLocations := locations
	if len(effectiveLocations) == 0 && (kind == "" || kind == prior.kind) {
		effectiveLocations = prior.locations
	}
	state := prior
	if kind != "" {
		state.kind = kind
	}
	if len(locations) > 0 {
		state.locations = locations
	} else if kind != "" && kind != prior.kind {
		state.locations = nil
	}
	if id != "" {
		r.toolCalls[id] = state
	}

	paths := make([]string, len(effectiveLocations))
	for i, loc := range effectiveLocations {
		paths[i] = loc.Path
	}
	if !notificationToolAllowed(r.dir, effectiveKind, paths) {
		stopped := cmp.Or(title, name, id, effectiveKind, "a tool call")
		r.abort(fmt.Sprintf("The agent tried to use %q outside its folder, so the run was stopped and the view was not changed.", stopped))
		return
	}
	switch effectiveKind {
	case "read", "search":
		_ = r.send.step("reading", "Reading its files")
	case "edit":
		_ = r.send.step("writing", "Writing the view")
	}
}

func (r *genRun) handleRequest(method string, params json.RawMessage) (any, *RPCError) {
	if method != "session/request_permission" {
		return nil, &RPCError{Code: -32601, Message: "method not found"}
	}
	if !r.notificationBelongsToSession(params) {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
	}
	var req struct {
		ToolCall struct {
			Kind      string     `json:"kind"`
			Title     string     `json:"title"`
			Locations []Location `json:"locations"`
		} `json:"toolCall"`
		Options []PermissionOption `json:"options"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params"}
	}
	paths := make([]string, len(req.ToolCall.Locations))
	for i, loc := range req.ToolCall.Locations {
		paths[i] = loc.Path
	}
	option := choosePermissionOption(req.Options, genui.AgentToolAllowed(r.dir, req.ToolCall.Kind, paths))
	if option == "" {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
	}
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": option}}, nil
}

func choosePermissionOption(options []PermissionOption, allowed bool) string {
	preferred, kindPrefix := "allow_once", "allow_"
	if !allowed {
		preferred, kindPrefix = "reject_once", "reject_"
	}
	fallback := ""
	for _, o := range options {
		if o.Kind == preferred {
			return o.OptionID
		}
		if fallback == "" && strings.HasPrefix(o.Kind, kindPrefix) {
			fallback = o.OptionID
		}
	}
	return fallback
}

func configValues(raw json.RawMessage, id string) []string {
	if len(raw) == 0 {
		return nil
	}
	var opts []struct {
		ID      string `json:"id"`
		Options []struct {
			Value any `json:"value"`
		} `json:"options"`
	}
	if json.Unmarshal(raw, &opts) != nil {
		return nil
	}
	for _, opt := range opts {
		if opt.ID != id {
			continue
		}
		values := make([]string, 0, len(opt.Options))
		for _, o := range opt.Options {
			if s, ok := o.Value.(string); ok {
				values = append(values, s)
			} else if o.Value != nil {
				values = append(values, fmt.Sprintf("%v", o.Value))
			}
		}
		return values
	}
	return nil
}

func (r *genRun) collect(ctx context.Context) error {
	outPath := filepath.Join(r.dir, "out.json")
	fi, err := os.Lstat(outPath)
	if errors.Is(err, fs.ErrNotExist) {
		if reply := strings.TrimSpace(r.lastReply()); reply != "" {
			finished := genui.NewFailure(genui.FailAgentMessage, "The agent finished without writing a view.", genui.HintAgentMessage)
			finished.Detail = genui.CutRunes(reply, genui.MaxDetailRunes)
			return finished
		}
		return r.outputError("The agent did not write out.json.")
	}
	if err != nil {
		return r.outputError("The agent's out.json could not be read.")
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return r.outputError("The agent's out.json is a symbolic link, not a file.")
	}
	if !fi.Mode().IsRegular() {
		return r.outputError("The agent's out.json is not a regular file.")
	}
	if fi.Size() > genui.MaxViewBytes {
		return r.outputError(fmt.Sprintf("The agent's out.json is %d bytes, more than the %d bytes a view may have.", fi.Size(), genui.MaxViewBytes))
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return r.outputError("The agent's out.json could not be read.")
	}
	if r.trace != nil {
		r.trace.artifact("out.json", outPath, string(data))
	}
	if r.assembleOutput != nil {
		data, err = r.assembleOutput(data)
		if err != nil {
			return r.outputError(fmt.Sprintf("The content slots could not be assembled: %v.", err))
		}
		if r.trace != nil {
			r.trace.artifact("assembled.json", "", string(data))
		}
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || top == nil {
		return r.outputError("The agent's out.json is not valid JSON.")
	}
	a2ui, ok := top["a2ui"]
	if !ok {
		return r.outputError(`The agent's out.json has no "a2ui".`)
	}
	var components []json.RawMessage
	if err := json.Unmarshal(a2ui, &components); err != nil || len(components) == 0 {
		return r.outputError(`The agent's out.json does not have a non-empty "a2ui" array.`)
	}
	sources, err := genui.ParseSources(top["sources"])
	if err != nil {
		return r.outputError(err.Error())
	}
	if r.base != nil {
		merged, messages, err := r.base.Merge(a2ui, sources)
		if err != nil {
			return r.outputError(err.Error())
		}
		return r.writeView(ctx, merged, messages)
	}
	return r.writeView(ctx, sources, a2ui)
}

type sourceOut struct {
	Heading    string `json:"heading"`
	Occurrence int    `json:"occurrence"`
	Hash       string `json:"hash,omitempty"`
}

type viewOut struct {
	Version     int                  `json:"version"`
	Language    string               `json:"language,omitempty"`
	GeneratedAt string               `json:"generatedAt"`
	Model       string               `json:"model,omitempty"`
	Request     string               `json:"request,omitempty"`
	Sources     map[string]sourceOut `json:"sources"`
	A2UI        json.RawMessage      `json:"a2ui"`
}

func (r *genRun) writeView(ctx context.Context, sources map[string]genui.AgentSource, a2ui json.RawMessage) error {
	if err := r.modelProblem(); err != nil {
		return err
	}
	view := viewOut{
		Version:     2,
		GeneratedAt: r.job.Now().UTC().Format(time.RFC3339),
		Model:       r.viewModel(),
		Request:     r.job.Req.Request,
		Sources:     make(map[string]sourceOut, len(sources)),
		A2UI:        a2ui,
	}
	if genui.WritesStatements(a2ui) {
		view.Version, view.Language = genui.StatementViewVersion, genui.StatementLanguage
	}
	kept := r.base.KeptHashes(a2ui)
	for key, src := range sources {
		entry := sourceOut{Heading: src.Heading, Occurrence: src.Occurrence}
		if hash, keep := kept[key]; keep {
			entry.Hash = hash
		} else {
			entry.Hash, _ = r.sectionHash(src)
		}
		view.Sources[key] = entry
	}
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return r.outputError("The agent's out.json could not be turned into a view.")
	}
	content := append(data, '\n')
	if ctx.Err() != nil {
		return genui.ErrCancelled
	}
	if r.trace != nil {
		r.trace.artifact("candidate", "", string(content))
		r.trace.emit("validation.start", nil)
	}
	var report genui.CandidateReport
	var validationErr error
	if r.job.Validate == nil {
		report, validationErr = genui.CheckCandidate(ctx, content)
	} else {
		validationErr = r.job.Validate(ctx, content)
	}
	if ctx.Err() != nil {
		return genui.ErrCancelled
	}
	if validationErr != nil {
		if r.trace != nil {
			r.trace.emit("validation.result", map[string]any{"ok": false, "error": validationErr.Error()})
		}
		var ge *genui.GenError
		if errors.As(validationErr, &ge) {
			return r.outputError(ge.Message)
		}
		return r.outputError(validationErr.Error())
	}
	if r.trace != nil {
		accepted := map[string]any{"ok": true}
		if len(report.Degraded) > 0 {
			accepted["degraded"] = report.Degraded
		}
		r.trace.emit("validation.result", accepted)
	}
	if ctx.Err() != nil {
		return genui.ErrCancelled
	}
	if err := r.modelProblem(); err != nil {
		return err
	}
	var expect []byte
	if r.base != nil {
		expect = r.base.Raw()
	}
	if r.job.BeforeWrite != nil {
		if err := r.job.BeforeWrite(); err != nil {
			return err
		}
	}
	if err := genui.ReplaceView(r.job.ViewPath, content, expect); err != nil {
		return err
	}
	r.trace.viewWritten(r.job.ViewPath, content)
	return nil
}

func (r *genRun) sectionHash(src genui.AgentSource) (hash string, ok bool) {
	key := genui.NormalizeKey(src.Heading)
	for _, sec := range r.job.Req.Sections {
		if sec.Occurrence != src.Occurrence || genui.NormalizeKey(sec.Heading) != key {
			continue
		}
		if h, valid := hashSection(r.job.Req.Text, sec); valid {
			return h, true
		}
	}
	return "", false
}

func hashSection(text string, sec genui.SectionRef) (string, bool) {
	var hashed strings.Builder
	at := sec.Start
	for _, cut := range sec.Exclude {
		if cut.Start < at || cut.End < cut.Start || cut.End > sec.End {
			return "", false
		}
		part, ok := sliceUTF16(text, at, cut.Start)
		if !ok {
			return "", false
		}
		hashed.WriteString(part)
		at = cut.End
	}
	part, ok := sliceUTF16(text, at, sec.End)
	if !ok {
		return "", false
	}
	hashed.WriteString(part)
	return genui.SectionHash(hashed.String(), text, sec.End == utf16Len(text)), true
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		l := utf16.RuneLen(r)
		if l < 0 {
			l = 1
		}
		n += l
	}
	return n
}

func hashUTF16Range(text string, start, end int) (string, bool) {
	return hashSection(text, genui.SectionRef{Start: start, End: end})
}

func sliceUTF16(text string, start, end int) (string, bool) {
	if start < 0 || end < start {
		return "", false
	}
	byteStart, byteEnd := -1, -1
	if start == 0 {
		byteStart = 0
	}
	units := 0
	for i, ch := range text {
		if units == start {
			byteStart = i
		}
		if units == end {
			byteEnd = i
			break
		}
		l := utf16.RuneLen(ch)
		if l < 0 {
			l = 1
		}
		units += l
	}
	if byteEnd < 0 {
		if units != end {
			return "", false
		}
		byteEnd = len(text)
	}
	if byteStart < 0 && units == start {
		byteStart = len(text)
	}
	if byteStart < 0 || byteStart > byteEnd {
		return "", false
	}
	return text[byteStart:byteEnd], true
}

func (r *genRun) outputError(what string) error {
	e := genui.NewFailure(genui.FailInvalidOutput, what, genui.HintInvalidOutput)
	e.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
	return e
}

type progressSender struct {
	mu    sync.Mutex
	send  func(genui.GenerateProgress) error
	trace *generationTrace
	phase string
	text  string
}

func (ps *progressSender) step(phase, text string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if phase == ps.phase && text == ps.text {
		return nil
	}
	ps.phase, ps.text = phase, text
	ps.trace.phase(phase, text)
	if err := ps.send(genui.GenerateProgress{Phase: phase, Text: text}); err != nil {
		return genui.ErrCancelled
	}
	return nil
}
