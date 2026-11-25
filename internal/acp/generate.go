package acp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

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

const defaultGenerationTimeout = genui.DefaultGenerationTimeout

const acpPromptCancelAckTimeout = time.Second

const acpSessionCloseTimeout = time.Second

const statementPreviewInterval = 25 * time.Millisecond

// thoughtOptionIDs and thoughtOptionCategories are the names agents use for the
// reasoning-effort option: agy spells it thought_level, Codex reasoning_effort.
var thoughtOptionIDs = []string{traceThoughtID, "thoughtLevel", "reasoning_effort"}

var thoughtOptionCategories = []string{traceThoughtID, "thoughtLevel", "reasoning_effort", "reasoning"}

type ModelUpdater interface {
	SetModels(backend string, models []string) error
}

// SelectionUpdater records the model and thought level the agent confirmed.
type SelectionUpdater interface {
	SetSelection(backend, model, thoughtLevel string) error
}

type GenerationJob struct {
	Req          genui.GenerateRequest
	Backend      string
	AgentCommand string
	// Args are the arguments that make the command speak ACP over stdio;
	// they are empty for codex-acp and ["acp"] for agy.
	Args      []string
	Model     string
	ViewPath  string
	Now       func() time.Time
	Models    ModelUpdater
	Selection SelectionUpdater
	Timeout   time.Duration

	Env []string

	BeforeWrite func() error

	Validate func(context.Context, []byte) error

	Observe func(event string, payload any)

	ThoughtLevel string

	AgentCache *ProcessCache

	Prepare func(context.Context, genui.GenerateRequest, string) (string, func([]byte) ([]byte, error), error)
}

func ResolveAgentCommand(command string) (string, error) {
	return resolveAgentCommand(command, nil)
}

func resolveAgentCommand(command string, env []string) (string, error) {
	if resolved, ok := fsutil.ResolveExecutable(command, fsutil.WithUserBinPath(env), nil); ok {
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
	runner := &turnRunner{job: job, trace: trace}
	defer func() {
		if closeErr := runner.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	orchestrator := &genui.Orchestrator{
		Runner:       runner,
		Req:          job.Req,
		ViewPath:     job.ViewPath,
		Model:        job.Model,
		ThoughtLevel: job.ThoughtLevel,
		Now:          job.Now,
		Timeout:      job.Timeout,
		Validate:     job.Validate,
		BeforeWrite:  job.BeforeWrite,
		Prompt:       job.Prepare,
		OnDir:        func(dir string) { trace.runStart(&job, dir) },
		Artifact:     trace.artifactOrView,
		Observe:      trace.emit,
	}
	var sent func(genui.GenerateProgress) error = progress
	if trace != nil {
		sent = trace.phases(progress)
	}
	return orchestrator.Run(ctx, sent)
}

// startAgent resolves and launches the backend's command with the arguments it
// declared. It runs once per turn, inside the turn's folder.
func (r *turnRunner) startAgent(dir string) error {
	if r.agent != nil {
		return nil
	}
	resolved, err := resolveAgentCommand(r.job.AgentCommand, r.job.Env)
	if err != nil {
		return err
	}
	r.resolved = resolved
	if r.job.AgentCache != nil {
		lease, err := r.job.AgentCache.lease(resolved, r.job.Args, r.job.Env)
		if err != nil {
			return genui.NewFailure(genui.FailAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", r.job.AgentCommand, err), genui.HintAgentNotFound)
		}
		r.lease = lease
		r.agent = lease.agent
		if lease.reused {
			r.trace.agentReused(resolved, lease.args, r.agent.PID(), lease.cwd, lease.startedAt, lease.idleSince)
		} else {
			r.trace.agentStart(resolved, lease.args, r.agent.PID(), lease.cwd)
		}
	} else {
		cmd := exec.Command(resolved, r.job.Args...)
		cmd.Dir = dir
		cmd.Env = fsutil.WithUserBinPath(r.job.Env)
		r.agent, err = NewAgent(cmd, fsutil.NewLimitBuffer(4<<10))
		if err != nil {
			return genui.NewFailure(genui.FailAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", r.job.AgentCommand, err), genui.HintAgentNotFound)
		}
		r.trace.agentStart(resolved, cmd.Args, r.agent.PID(), dir)
	}
	var observe func(direction string, line []byte)
	if r.trace != nil {
		observe = func(direction string, line []byte) {
			if direction == Received && !r.trace.acceptsInbound(line) {
				return
			}
			event := TraceACPSent
			if direction == Received {
				event = TraceACPReceived
			}
			r.trace.emit(event, &traceACPPayload{Direction: direction, Line: json.RawMessage(line)})
		}
	}
	r.agent.SetHandlers(r.handleRequest, r.handleNotification, observe)
	if r.lease == nil || !r.lease.reused {
		r.agent.Start()
	}
	return nil
}

// Close releases the turn's process: an idle one whose session was closed is
// kept for reuse, anything else exits.
func (r *turnRunner) Close() error {
	if r == nil {
		return nil
	}
	if r.agent != nil {
		r.agent.SetHandlers(nil, nil, nil)
	}
	if r.lease != nil {
		r.lease.release(r.sessionClosed)
		r.lease = nil
		return nil
	}
	if r.agent != nil {
		r.agent.Kill()
	}
	return nil
}

func (r *turnRunner) Probe(ctx context.Context) (genui.ProbeResult, error) {
	return Probe(ctx, r.probeConfig())
}

func (r *turnRunner) ListModels(ctx context.Context) (genui.ModelCatalog, error) {
	return ListModels(ctx, SessionConfig{ProbeConfig: r.probeConfig()})
}

func (r *turnRunner) probeConfig() ProbeConfig {
	return ProbeConfig{
		Backend: r.job.Backend,
		Command: r.job.AgentCommand,
		Args:    r.job.Args,
		Env:     r.job.Env,
	}
}

func (r *turnRunner) ExecuteTurn(ctx context.Context, turn genui.Turn) (genui.TurnResult, error) {
	var none genui.TurnResult
	r.turn = turn
	r.dir = turn.Dir
	r.abortCh = make(chan struct{})
	r.toolCalls = make(map[string]toolCallState)
	if err := r.startAgent(turn.Dir); err != nil {
		return none, err
	}
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
			return none, err
		}
		r.trace.initialized(initRaw, acpClientInfo, ProtocolVersion, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		if r.lease != nil {
			r.lease.storeInitialize(initRaw, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		}
	}
	if initialized.ProtocolVersion != ProtocolVersion {
		return none, genui.AgentErrorf("The agent speaks ACP protocol version %d, but this app speaks version %d.", initialized.ProtocolVersion, ProtocolVersion)
	}

	var opened struct {
		SessionID     string          `json:"sessionId"`
		ConfigOptions json.RawMessage `json:"configOptions"`
		Models        json.RawMessage `json:"models"`
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
		return none, err
	}
	if r.lease != nil {
		if err := r.lease.acceptSession(opened.SessionID); err != nil {
			return none, err
		}
	}
	r.setSessionID(opened.SessionID)
	r.trace.sessionNew(openedRaw, opened.SessionID)
	r.noteOptions(opened.ConfigOptions)
	r.legacyModels = legacyModelValues(opened.Models)
	if models := configValues(opened.ConfigOptions, "model"); len(models) > 0 && r.job.Models != nil {
		_ = r.job.Models.SetModels(r.job.Backend, models)
	}
	if err := r.step("starting", "Agent ready"); err != nil {
		return none, err
	}

	requestedModel, effortFromID := genui.SplitModelID(turn.Model)
	requestedThought := turn.ThoughtLevel
	if requestedThought == "" {
		requestedThought = effortFromID
	}
	if err := r.applyModel(ctx, requestedModel); err != nil {
		return none, err
	}
	if err := r.applyThoughtLevel(ctx, requestedThought); err != nil {
		return none, err
	}
	if err := r.confirmedSelection(); err != nil {
		return none, err
	}

	r.trace.prompt(turn.Prompt)
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
		"prompt":    []map[string]any{{"type": "text", "text": turn.Prompt}},
	}, &turned, turnedRawPtr); err != nil {
		if errors.Is(err, genui.ErrCancelled) && r.turnWasAcknowledged() {
			r.settleSession()
		}
		return none, err
	}
	r.trace.promptResult(turnedRaw)
	switch turned.StopReason {
	case "", "end_turn":
	case "cancelled":
		r.settleSession()
		return none, genui.ErrCancelled
	case "refusal":
		r.settleSession()
		refused := genui.NewFailure(genui.FailAgentMessage, "The agent declined to make the view.", genui.HintAgentMessage)
		refused.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return none, refused
	case "max_tokens", "max_turn_requests":
		r.settleSession()
		stopped := genui.NewFailure(genui.FailAgentMessage, "The agent stopped before finishing the view.", genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return none, stopped
	default:
		r.settleSession()
		stopped := genui.NewFailure(genui.FailAgentMessage, fmt.Sprintf("The agent stopped before finishing the view (%s).", turned.StopReason), genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(strings.TrimSpace(r.lastReply()), genui.MaxDetailRunes)
		return none, stopped
	}
	if err := r.modelProblem(); err != nil {
		r.settleSession()
		return none, err
	}
	if r.lease == nil {
		r.agent.Kill()
	} else if !r.settleSession() {
		r.agent.Kill()
	}
	return genui.TurnResult{Model: r.modelStamp(), Reply: r.lastReply()}, nil
}

func (r *turnRunner) step(phase, text string) error {
	if r.turn.Progress == nil {
		return nil
	}
	return r.turn.Progress(genui.GenerateProgress{Phase: phase, Text: text})
}

// applyModel writes the requested model through the declared model option and
// only falls back to the legacy model list when that option is missing. A
// failed write on the main path is reported as is: it never becomes a legacy
// write, because the agent already said which values it accepts.
func (r *turnRunner) applyModel(ctx context.Context, requested string) error {
	if requested == "" {
		return nil
	}
	if opt, ok := findConfigOptionIn(r.currentOptions(), []string{traceModelOption}, []string{traceModelOption}); ok {
		r.markModelRequested(requested)
		var setRaw json.RawMessage
		if err := r.callAgent(ctx, "session/set_config_option", map[string]any{
			"sessionId": r.session(),
			"configId":  opt.ID,
			"value":     requested,
		}, nil, &setRaw); err != nil {
			r.trace.configSetError(opt.ID, requested, err)
			offered := opt.values()
			var ge *genui.GenError
			if len(offered) > 0 && errors.As(err, &ge) && ge.Kind == genui.FailAgentError {
				invalid := genui.NewFailure(genui.FailModelInvalid, fmt.Sprintf("The agent does not know the model %q.", requested), genui.HintModelInvalid)
				invalid.Models = offered
				return invalid
			}
			return err
		}
		r.trace.configSetResponse(opt.ID, requested, setRaw)
		r.noteOptions(configOptionsField(setRaw))
		r.noteModelOptions(configOptionsField(setRaw))
		return r.modelProblem()
	}
	if len(r.legacyModels) == 0 {
		r.trace.configSetSkipped(traceModelOption, requested, "the agent's session/new declared no model option and no legacy model list")
		return nil
	}
	value := requested
	if !slices.Contains(r.legacyModels, value) && slices.Contains(r.legacyModels, r.job.Model) {
		// The legacy model list is the only place a compound id may be used.
		value = r.job.Model
	}
	r.markModelRequested(value)
	if err := r.callAgent(ctx, "session/set_model", map[string]any{
		"sessionId": r.session(),
		"modelId":   value,
	}, nil, nil); err != nil {
		r.trace.configSetError(traceModelOption, value, err)
		var ge *genui.GenError
		if !slices.Contains(r.legacyModels, value) && errors.As(err, &ge) && ge.Kind == genui.FailAgentError {
			invalid := genui.NewFailure(genui.FailModelInvalid, fmt.Sprintf("The agent does not know the model %q.", value), genui.HintModelInvalid)
			invalid.Models = r.legacyModels
			return invalid
		}
		return err
	}
	r.trace.configSetSkipped(traceModelOption, value, "the agent declared no model option, so the model was set through the legacy session/set_model")
	r.confirmModel(value)
	return nil
}

// applyThoughtLevel writes the reasoning effort only when the current options
// declare an option for it and offer the requested value. A backend that knows
// neither is not a failure: the session simply runs at its own default.
func (r *turnRunner) applyThoughtLevel(ctx context.Context, requested string) error {
	if requested == "" {
		return nil
	}
	opt, ok := findThoughtOption(r.currentOptions())
	if !ok {
		r.trace.configSetSkipped(traceThoughtID, requested, "the agent's current configOptions have no thought_level option")
		return nil
	}
	offered := opt.values()
	if !slices.Contains(offered, requested) {
		r.trace.configSetSkipped(opt.ID, requested, fmt.Sprintf("the agent's %s option does not offer %q; it offers %s", opt.ID, requested, strings.Join(genui.QuoteAll(offered), ", ")))
		return nil
	}
	var setRaw json.RawMessage
	if err := r.callAgent(ctx, "session/set_config_option", map[string]any{
		"sessionId": r.session(),
		"configId":  opt.ID,
		"value":     requested,
	}, nil, &setRaw); err != nil {
		r.trace.configSetError(opt.ID, requested, err)
		return err
	}
	r.trace.configSetResponse(opt.ID, requested, setRaw)
	r.noteOptions(configOptionsField(setRaw))
	return nil
}

func findThoughtOption(raw json.RawMessage) (acpConfigOption, bool) {
	return findConfigOptionIn(raw, thoughtOptionIDs, thoughtOptionCategories)
}

// confirmedSelection records what the agent reported, so the settings page and
// the next run use the values that took effect rather than the ones asked for.
func (r *turnRunner) confirmedSelection() error {
	if r.job.Selection == nil {
		return nil
	}
	r.modelMu.Lock()
	model, confirmed := r.modelActual, r.modelConfirmed
	mismatch := r.modelMismatch
	r.modelMu.Unlock()
	if mismatch != "" || !confirmed || model == "" {
		return nil
	}
	// Only the effort the agent reports back is recorded; an effort it did not
	// confirm leaves the saved one alone.
	thought := ""
	requested := strings.TrimSpace(r.turn.ThoughtLevel)
	if requested != "" {
		if actual, ok := r.thoughtActual(); ok && actual == requested {
			thought = actual
		}
	}
	_ = r.job.Selection.SetSelection(r.job.Backend, model, thought)
	return nil
}

// turnRunner is the ACP adapter behind genui.Runner: it owns the process, the
// session and the declared options, and the orchestrator owns everything a
// generation needs regardless of protocol.
var _ genui.Runner = (*turnRunner)(nil)

type turnRunner struct {
	job  GenerationJob
	dir  string
	turn genui.Turn

	trace    *generationTrace
	resolved string

	agent *Agent
	lease *acpLease

	sessionMu     sync.RWMutex
	sessionID     string
	sessionClosed bool

	cancelMu           sync.Mutex
	cancelAcknowledged bool

	modelMu        sync.Mutex
	modelRequested bool
	modelTarget    string
	modelActual    string
	modelConfirmed bool
	modelMismatch  string

	optionsMu sync.Mutex
	options   json.RawMessage

	legacyModels []string

	toolCalls map[string]toolCallState

	abortCh     chan struct{}
	abortOnce   sync.Once
	abortReason string

	replyMu sync.Mutex
	reply   strings.Builder
}

func (r *turnRunner) callAgent(ctx context.Context, method string, params any, result any, raw *json.RawMessage) error {
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

func (r *turnRunner) agentCallResult(method string, err error) error {
	if err == nil {
		return nil
	}
	var rpc *RPCError
	if !errors.As(err, &rpc) {
		if genui.IsAuthError(0, err.Error()) {
			return r.authRequired(err.Error())
		}
		return genui.AgentError(err.Error())
	}
	switch {
	case genui.IsAuthError(rpc.Code, rpc.Message):
		return r.authRequired(rpc.Message)
	case method == "session/prompt":
		stopped := genui.NewFailure(genui.FailAgentMessage, "The agent stopped with an error.", genui.HintAgentMessage)
		stopped.Detail = genui.CutRunes(rpc.Message, genui.MaxDetailRunes)
		return stopped
	default:
		return genui.AgentErrorf("The agent answered %s with an error: %s", method, rpc.Message)
	}
}

// authRequired names the sign-in the backend actually has, not always agy's.
func (r *turnRunner) authRequired(detail string) error {
	if b, ok := genui.BackendByID(r.job.Backend); ok && b.LoginHint != "" {
		e := genui.NewFailure(genui.FailAuthRequired, "The agent is not signed in.", b.LoginHint)
		e.Detail = genui.CutRunes(detail, genui.MaxDetailRunes)
		return e
	}
	return genui.AuthRequired(r.job.AgentCommand, detail)
}

func (r *turnRunner) cancelPromptTurn(method string, done <-chan error) {
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

func (r *turnRunner) markCancelAcknowledged() {
	r.cancelMu.Lock()
	r.cancelAcknowledged = true
	r.cancelMu.Unlock()
}

func (r *turnRunner) turnWasAcknowledged() bool {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	return r.cancelAcknowledged
}

func (r *turnRunner) settleSession() bool {
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

func (r *turnRunner) closeSessionBounded(sessionID string) bool {
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

func (r *turnRunner) stopAfterCancel() {
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

func (r *turnRunner) abort(msg string) {
	r.abortOnce.Do(func() {
		r.abortReason = msg
		close(r.abortCh)
		if r.agent != nil {
			r.agent.CancelTurn(r.session())
			go r.agent.Kill()
		}
	})
}

func (r *turnRunner) lastReply() string {
	r.replyMu.Lock()
	defer r.replyMu.Unlock()
	return r.reply.String()
}

func (r *turnRunner) setSessionID(id string) {
	r.sessionMu.Lock()
	r.sessionID = id
	r.sessionMu.Unlock()
}

func (r *turnRunner) session() string {
	r.sessionMu.RLock()
	defer r.sessionMu.RUnlock()
	return r.sessionID
}

func (r *turnRunner) noteOptions(options json.RawMessage) {
	if len(options) == 0 {
		return
	}
	r.optionsMu.Lock()
	r.options = cloneRaw(options)
	r.optionsMu.Unlock()
}

func (r *turnRunner) currentOptions() json.RawMessage {
	r.optionsMu.Lock()
	defer r.optionsMu.Unlock()
	return cloneRaw(r.options)
}

func (r *turnRunner) confirmModel(model string) {
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if r.modelMismatch == "" {
		r.modelActual = model
		r.modelConfirmed = true
	}
}

func (r *turnRunner) noteModelOptions(options json.RawMessage) {
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
	target := r.modelTarget
	if target == "" {
		target = r.job.Model
	}
	if actual != target {
		if r.modelMismatch == "" {
			r.modelMismatch = fmt.Sprintf("The agent reports the model %q, not the requested %q, so the view was not changed.", actual, target)
		}
		return
	}
	if r.modelMismatch == "" {
		r.modelActual = actual
		r.modelConfirmed = true
	}
}

func (r *turnRunner) markModelRequested(requested string) {
	r.modelMu.Lock()
	r.modelRequested = true
	r.modelTarget = requested
	r.modelMu.Unlock()
}

// thoughtActual reads the effort the agent currently reports, from the latest
// options it sent or its update notification.
func (r *turnRunner) thoughtActual() (string, bool) {
	opt, ok := findThoughtOption(r.currentOptions())
	if !ok {
		return "", false
	}
	current, ok := opt.current()
	if !ok {
		return "", false
	}
	return configString(current)
}

func (r *turnRunner) modelProblem() error {
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

func (r *turnRunner) modelStamp() string {
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if r.modelConfirmed && r.modelActual != "" {
		return r.modelActual
	}
	return ""
}

func (r *turnRunner) notificationBelongsToSession(params json.RawMessage) bool {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(params, &p) != nil || p.SessionID == "" {
		return true
	}
	return p.SessionID == r.session()
}

func (r *turnRunner) handleNotification(method string, params json.RawMessage) {
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
		r.noteOptions(opts)
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
		_ = r.step("thinking", "Thinking")
	case "config_option_update":
		opts := u.ConfigOptions
		if len(opts) == 0 {
			opts = configOptionsField(params)
		}
		r.trace.configOptionUpdate(method, params, opts)
		r.noteOptions(opts)
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

func (r *turnRunner) handleToolCall(id, kind, title, name string, locations []Location) {
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
		_ = r.step("reading", "Reading its files")
	case "edit":
		_ = r.step("writing", "Writing the view")
	}
}

func (r *turnRunner) handleRequest(method string, params json.RawMessage) (any, *RPCError) {
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

// legacyModelValues reads the pre-configOptions model list an agent may still
// declare. It is consulted only when session/new had no model option.
func legacyModelValues(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var listed struct {
		AvailableModels []struct {
			ModelID string `json:"modelId"`
			ID      string `json:"id"`
		} `json:"availableModels"`
	}
	if json.Unmarshal(raw, &listed) != nil {
		return nil
	}
	values := make([]string, 0, len(listed.AvailableModels))
	for _, m := range listed.AvailableModels {
		id := cmp.Or(strings.TrimSpace(m.ModelID), strings.TrimSpace(m.ID))
		if id != "" {
			values = append(values, id)
		}
	}
	return values
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

func hashSection(text string, sec genui.SectionRef) (string, bool) {
	return genui.SectionHashOf(text, sec)
}

func utf16Len(s string) int { return genui.UTF16Len(s) }

func hashUTF16Range(text string, start, end int) (string, bool) {
	return genui.SectionHashOf(text, genui.SectionRef{Start: start, End: end})
}
