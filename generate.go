package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/egoist/mygo"
)

var acpClientInfo = map[string]string{"name": acpClientName, "version": acpClientVersion}

var acpInitializeParams = map[string]any{
	"protocolVersion": acpProtocolVersion,
	"clientInfo":      acpClientInfo,
	"clientCapabilities": map[string]any{
		"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
		"terminal": false,
	},
}

type GenerateRequest struct {
	Text       string       `json:"text"`
	Mode       string       `json:"mode"`
	ReplaceIDs []string     `json:"replaceIds,omitempty"`
	Request    string       `json:"request,omitempty"`
	Format     string       `json:"format"`
	Sections   []SectionRef `json:"sections"`
}

type SectionRef struct {
	Heading    string      `json:"heading"`
	Occurrence int         `json:"occurrence"`
	Start      int         `json:"start"`
	End        int         `json:"end"`
	Exclude    []TextRange `json:"exclude,omitempty"`
}

type TextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type GenerateProgress struct {
	Phase   string           `json:"phase"`
	Text    string           `json:"text"`
	Failure *GenerateFailure `json:"failure,omitempty"`
}

var errCancelled = errors.New("generation cancelled")

var errTimedOut = errors.New("generation timed out")

const defaultGenerationTimeout = 5 * time.Minute

const acpPromptCancelAckTimeout = time.Second

const acpSessionCloseTimeout = time.Second

func (h *documentHost) Generate(ctx context.Context, documentID string, req GenerateRequest, progress *mygo.Channel[GenerateProgress]) error {
	send := func(p GenerateProgress) error {
		if progress == nil {
			return nil
		}
		return progress.Send(p)
	}
	if err := h.generate(ctx, documentID, req, send); err != nil {
		return reportFailure(err, send)
	}
	return nil
}

var errPathChanged = newFailure(failAgentError, "The document's path changed while the agent worked, so the view was not written.", "Generate again.")

var errAbandoned = newFailure(failCancelled, "Generation was cancelled.", "")

const errGenerating = "A view is being generated; wait for it to finish, or cancel it."

func (h *documentHost) generate(ctx context.Context, documentID string, req GenerateRequest, send func(GenerateProgress) error) error {
	h.runs.Add(1)
	defer h.runs.Done()
	s, err := h.docSession(documentID)
	if err != nil {
		return err
	}
	docPath := s.currentPath()
	if docPath == "" {
		return newFailure(failUnsaved, "Save the document first: the view is stored next to it.", "")
	}
	var settings Settings
	if h.settings != nil {
		settings = h.settings.get()
	}
	h.mu.Lock()
	if s.closed {
		h.mu.Unlock()
		return errNoDocument
	}
	if s.generating {
		h.mu.Unlock()
		return newFailure(failBusy, "A view is already being generated.", hintBusy)
	}
	runCtx, cancel := context.WithCancel(ctx)
	slot := &generationSlot{cancel: cancel, path: docPath}
	s.generating = true
	s.genSlot = slot
	w := s.ws
	win := s.win
	h.mu.Unlock()
	h.emitWorkspace(w)
	if win != nil {
		_ = docState.Emit(win, s.info())
	}
	h.refreshPathMenus()
	defer h.releaseGeneration(s, slot)

	job := generationJob{
		Req:          req,
		AgentCommand: settings.AgentCommand,
		Model:        settings.Model,
		ViewPath:     viewPath(docPath),
		Now:          time.Now,
		Settings:     h.settings,
		Validate:     validateGeneratedCandidate,
		AgentCache:   h.agentCache,
		BeforeWrite: func() error {
			return h.commitGenerationWrite(s, slot)
		},
	}
	if h.prepareJob != nil {
		h.prepareJob(&job)
	}
	return runGeneration(runCtx, job, send)
}

func (h *documentHost) commitGenerationWrite(s *docSession, slot *generationSlot) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s.closed || slot.abandoned || s.genSlot != slot {
		return errAbandoned
	}
	now := s.currentPath()
	if now == "" || now != slot.path {
		return errPathChanged
	}
	return nil
}

func (h *documentHost) cancelGeneration(s *docSession) {
	h.mu.Lock()
	slot := s.genSlot
	if slot != nil {
		slot.abandoned = true
		if slot.cancel != nil {
			slot.cancel()
		}
	}
	h.mu.Unlock()
}

func (h *documentHost) releaseGeneration(s *docSession, slot *generationSlot) {
	h.mu.Lock()
	if s.genSlot == slot {
		s.genSlot = nil
		s.generating = false
	}
	w := s.ws
	win := s.win
	closed := s.closed
	h.mu.Unlock()
	if closed {
		return
	}
	h.emitWorkspace(w)
	if win != nil {
		_ = docState.Emit(win, s.info())
	}
	h.refreshPathMenus()
}

func (h *documentHost) rejectPathChange(s *docSession) error {
	h.mu.Lock()
	busy := s.generating
	h.mu.Unlock()
	if busy {
		return errors.New(errGenerating)
	}
	return nil
}

func (h *documentHost) waitForGenerations(limit time.Duration) {
	done := make(chan struct{})
	go func() {
		h.runs.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(limit):
	}
}

func (h *documentHost) GetSettings() (Settings, error) {
	return h.settings.get(), nil
}

func (h *documentHost) SetSettings(p SettingsPatch) (Settings, error) {
	return h.settings.apply(p)
}

type generationJob struct {
	Req          GenerateRequest
	AgentCommand string
	Model        string
	ViewPath     string
	Now          func() time.Time
	Settings     *settingsStore
	Timeout      time.Duration

	Env []string

	BeforeWrite func() error

	Validate func(context.Context, []byte) error

	Observe func(event string, payload any)

	ThoughtLevel string

	AgentCache *acpProcessCache

	Prepare func(context.Context, GenerateRequest, string) (string, func([]byte) ([]byte, error), error)
}

func runGeneration(ctx context.Context, job generationJob, progress func(GenerateProgress) error) (err error) {
	trace := newGenerationTrace(&job)
	if trace != nil {
		defer func() { trace.end(err) }()
	}

	var base *baseView
	switch job.Req.Mode {
	case "full":
	case "update":
		b, err := readBaseView(job.ViewPath, job.Req.ReplaceIDs)
		if err != nil {
			return err
		}
		base = b
	default:
		return agentErrorf("Unknown generation mode %q: only \"full\" and \"update\" are supported.", job.Req.Mode)
	}
	if job.Now == nil {
		job.Now = time.Now
	}
	if job.Timeout <= 0 {
		job.Timeout = defaultGenerationTimeout
	}
	ctx, stop := context.WithTimeoutCause(ctx, job.Timeout, errTimedOut)
	defer stop()
	interrupted := func() error {
		if context.Cause(ctx) == errTimedOut {
			return timedOut(job.Timeout)
		}
		return newFailure(failCancelled, "Generation was cancelled.", "")
	}

	dir, err := os.MkdirTemp("", "markgogo-genui-")
	if err != nil {
		return agentErrorf("Could not make the agent's folder: %v.", err)
	}
	defer os.RemoveAll(dir)
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return agentErrorf("Could not make the agent's folder: %v.", err)
	}

	trace.runStart(&job, dir)
	if err := os.WriteFile(filepath.Join(dir, "source.md"), []byte(job.Req.Text), 0o644); err != nil {
		return agentErrorf("Could not write the agent's source file: %v.", err)
	}
	if trace != nil {
		trace.artifact("source.md", filepath.Join(dir, "source.md"), job.Req.Text)
	}
	if base != nil {
		if err := os.WriteFile(filepath.Join(dir, "view.json"), base.raw, 0o644); err != nil {
			return agentErrorf("Could not write the agent's copy of the view: %v.", err)
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
			return agentErrorf("Could not prepare the controlled reading view: %v.", err)
		}
	}

	resolved, err := resolveAgentCommand(job.AgentCommand)
	if err != nil {
		return err
	}

	run := &genRun{job: job, dir: dir, send: send, base: base, trace: trace, promptText: promptText, assembleOutput: assembleOutput, abortCh: make(chan struct{}), toolCalls: make(map[string]toolCallState)}
	var agent *acpAgent
	if job.AgentCache != nil {
		lease, err := job.AgentCache.lease(resolved, job.Env)
		if err != nil {
			return newFailure(failAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", job.AgentCommand, err), hintAgentNotFound)
		}
		run.lease = lease
		agent = lease.agent
		if lease.reused {
			trace.agentReused(resolved, lease.args, agent.cmd.Process.Pid, lease.cwd, lease.startedAt, lease.idleSince)
		} else {
			trace.agentStart(resolved, lease.args, agent.cmd.Process.Pid, lease.cwd)
		}
	} else {
		cmd := exec.Command(resolved, "acp")
		cmd.Dir = dir
		cmd.Env = job.Env
		agent, err = newACPAgent(cmd, newLimitBuffer(4<<10))
		if err != nil {
			return newFailure(failAgentNotFound, fmt.Sprintf("Could not start the agent command %q: %v.", job.AgentCommand, err), hintAgentNotFound)
		}
		trace.agentStart(resolved, cmd.Args, agent.cmd.Process.Pid, dir)
	}
	run.agent = agent
	var observe func(direction string, line []byte)
	if trace != nil {
		observe = func(direction string, line []byte) {
			if direction == acpReceived && !trace.acceptsInbound(line) {
				return
			}
			event := traceACPSent
			if direction == acpReceived {
				event = traceACPReceived
			}
			trace.emit(event, &traceACPPayload{Direction: direction, Line: json.RawMessage(line)})
		}
	}
	agent.setHandlers(run.handleRequest, run.handleNotification, observe)
	if run.lease == nil || !run.lease.reused {
		agent.start()
	}
	defer run.finishAgent()

	err = run.execute(ctx)
	if err == nil {
		return nil
	}
	if errors.Is(err, errCancelled) {
		run.stopAfterCancel()
		return interrupted()
	}
	return err
}

func (r *genRun) finishAgent() {
	if r.agent != nil {
		r.agent.setHandlers(nil, nil, nil)
	}
	if r.lease != nil {
		r.lease.release(r.sessionClosed)
		return
	}
	if r.agent != nil {
		r.agent.kill()
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
		r.trace.initializeCached(acpClientInfo, acpProtocolVersion, handshake.protocolVersion, handshake.agentInfo, handshake.agentCapabilities, handshake.authMethods, handshake.raw, r.lease.agent.cmd.Process.Pid, r.lease.startedAt, r.lease.cwd)
	} else {
		var initRaw json.RawMessage
		var initRawPtr *json.RawMessage
		if r.trace != nil || r.lease != nil {
			initRawPtr = &initRaw
		}
		if err := r.callAgent(ctx, "initialize", acpInitializeParams, &initialized, initRawPtr); err != nil {
			return err
		}
		r.trace.initialized(initRaw, acpClientInfo, acpProtocolVersion, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		if r.lease != nil {
			r.lease.storeInitialize(initRaw, initialized.ProtocolVersion, initialized.AgentInfo, initialized.AgentCapabilities, initialized.AuthMethods)
		}
	}
	if initialized.ProtocolVersion != acpProtocolVersion {
		return agentErrorf("The agent speaks ACP protocol version %d, but this app speaks version %d.", initialized.ProtocolVersion, acpProtocolVersion)
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
	if models := configValues(opened.ConfigOptions, "model"); len(models) > 0 && r.job.Settings != nil {
		_ = r.job.Settings.setModels(models)
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
			var ge *genError
			if models := configValues(opened.ConfigOptions, "model"); len(models) > 0 && errors.As(err, &ge) && ge.Kind == failAgentError {
				invalid := newFailure(failModelInvalid, fmt.Sprintf("The agent does not know the model %q.", r.job.Model), hintModelInvalid)
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
		promptText = generationPrompt(r.job.Req, r.base, r.dir)
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
		if errors.Is(err, errCancelled) && r.turnWasAcknowledged() {
			r.settleSession()
		}
		return err
	}
	r.trace.promptResult(turnedRaw)
	switch turned.StopReason {
	case "", "end_turn":
	case "cancelled":
		r.settleSession()
		return errCancelled
	case "refusal":
		r.settleSession()
		refused := newFailure(failAgentMessage, "The agent declined to make the view.", hintAgentMessage)
		refused.Detail = cutRunes(strings.TrimSpace(r.lastReply()), maxDetailRunes)
		return refused
	case "max_tokens", "max_turn_requests":
		r.settleSession()
		stopped := newFailure(failAgentMessage, "The agent stopped before finishing the view.", hintAgentMessage)
		stopped.Detail = cutRunes(strings.TrimSpace(r.lastReply()), maxDetailRunes)
		return stopped
	default:
		r.settleSession()
		stopped := newFailure(failAgentMessage, fmt.Sprintf("The agent stopped before finishing the view (%s).", turned.StopReason), hintAgentMessage)
		stopped.Detail = cutRunes(strings.TrimSpace(r.lastReply()), maxDetailRunes)
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
		r.agent.kill()
	} else if !r.settleSession() {
		r.agent.kill()
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
		r.trace.configSetSkipped(opt.ID, requested, fmt.Sprintf("the agent's %s option does not offer %q; it offers %s", opt.ID, requested, strings.Join(quoteAll(offered), ", ")))
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
	job  generationJob
	dir  string
	send *progressSender
	base *baseView

	trace *generationTrace

	promptText     string
	assembleOutput func([]byte) ([]byte, error)

	agent *acpAgent
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
		return errCancelled
	}
	ch := make(chan error, 1)
	go func() { ch <- r.agent.callCapturing(method, params, result, raw) }()
	select {
	case err := <-ch:
		return r.agentCallResult(method, err)
	case <-ctx.Done():
		r.cancelPromptTurn(method, ch)
		return errCancelled
	case <-r.abortCh:
		return agentError(r.abortReason)
	}
}

func (r *genRun) agentCallResult(method string, err error) error {
	if err == nil {
		return nil
	}
	var rpc *acpError
	if !errors.As(err, &rpc) {
		if isAuthError(0, err.Error()) {
			return authRequired(r.job.AgentCommand, err.Error())
		}
		return agentError(err.Error())
	}
	switch {
	case isAuthError(rpc.Code, rpc.Message):
		return authRequired(r.job.AgentCommand, rpc.Message)
	case method == "session/prompt":
		stopped := newFailure(failAgentMessage, "The agent stopped with an error.", hintAgentMessage)
		stopped.Detail = cutRunes(rpc.Message, maxDetailRunes)
		return stopped
	default:
		return agentErrorf("The agent answered %s with an error: %s", method, rpc.Message)
	}
}

func (r *genRun) cancelPromptTurn(method string, done <-chan error) {
	sessionID := r.session()
	if method != "session/prompt" || sessionID == "" || r.agent == nil {
		return
	}
	r.agent.cancelTurn(sessionID)
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
		done <- r.agent.call("session/close", map[string]any{"sessionId": sessionID}, nil)
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
	r.agent.kill()
}

func (r *genRun) abort(msg string) {
	r.abortOnce.Do(func() {
		r.abortReason = msg
		close(r.abortCh)
		if r.agent != nil {
			r.agent.cancelTurn(r.session())
			go r.agent.kill()
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
		return agentError(r.modelMismatch)
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
			Locations     []acpLocation   `json:"locations"`
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
	locations []acpLocation
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
			if !agentToolAllowed(dir, kind, []string{loc}) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func notificationPathInside(dir, loc string) bool {
	resolved := resolveToolPath(dir, loc)
	rel, err := filepath.Rel(dir, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (r *genRun) handleToolCall(id, kind, title, name string, locations []acpLocation) {
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
	label := title
	if label == "" {
		label = name
	}
	if label == "" {
		label = id
	}
	if !notificationToolAllowed(r.dir, effectiveKind, paths) {
		if label == "" {
			label = effectiveKind
		}
		if label == "" {
			label = "a tool call"
		}
		r.abort(fmt.Sprintf("The agent tried to use %q outside its folder, so the run was stopped and the view was not changed.", label))
		return
	}
	switch effectiveKind {
	case "read", "search":
		if label != "" {
			_ = r.send.step("reading", label)
		}
	case "edit":
		if label != "" {
			_ = r.send.step("writing", label)
		}
	}
}

func (r *genRun) handleRequest(method string, params json.RawMessage) (any, *acpError) {
	if method != "session/request_permission" {
		return nil, &acpError{Code: -32601, Message: "method not found"}
	}
	if !r.notificationBelongsToSession(params) {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
	}
	var req struct {
		ToolCall struct {
			Kind      string        `json:"kind"`
			Title     string        `json:"title"`
			Locations []acpLocation `json:"locations"`
		} `json:"toolCall"`
		Options []acpPermissionOption `json:"options"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &acpError{Code: -32602, Message: "invalid params"}
	}
	paths := make([]string, len(req.ToolCall.Locations))
	for i, loc := range req.ToolCall.Locations {
		paths[i] = loc.Path
	}
	option := choosePermissionOption(req.Options, agentToolAllowed(r.dir, req.ToolCall.Kind, paths))
	if option == "" {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
	}
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": option}}, nil
}

func choosePermissionOption(options []acpPermissionOption, allowed bool) string {
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

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strconv.Quote(s)
	}
	return out
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

func resolveAgentCommand(command string) (string, error) {
	if strings.ContainsRune(command, '/') || strings.ContainsRune(command, os.PathSeparator) {
		return command, nil
	}
	if path, err := exec.LookPath(command); err == nil {
		return path, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{
			filepath.Join(home, ".local", "bin"),
			"/opt/homebrew/bin",
			"/usr/local/bin",
		} {
			path := filepath.Join(dir, command)
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				return path, nil
			}
		}
	}
	return "", newFailure(failAgentNotFound, fmt.Sprintf("Could not find the agent command %q.", command), hintAgentNotFound)
}

type agentSource struct {
	Heading    string
	Occurrence int
}

func (r *genRun) collect(ctx context.Context) error {
	outPath := filepath.Join(r.dir, "out.json")
	fi, err := os.Lstat(outPath)
	if errors.Is(err, fs.ErrNotExist) {
		if reply := strings.TrimSpace(r.lastReply()); reply != "" {
			finished := newFailure(failAgentMessage, "The agent finished without writing a view.", hintAgentMessage)
			finished.Detail = cutRunes(reply, maxDetailRunes)
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
	if fi.Size() > maxViewBytes {
		return r.outputError(fmt.Sprintf("The agent's out.json is %d bytes, more than the %d bytes a view may have.", fi.Size(), maxViewBytes))
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
	sources, err := parseSources(top["sources"])
	if err != nil {
		return r.outputError(err.Error())
	}
	if r.base != nil {
		merged, messages, err := r.base.merge(a2ui, sources)
		if err != nil {
			return r.outputError(err.Error())
		}
		return r.writeView(ctx, merged, messages)
	}
	return r.writeView(ctx, sources, a2ui)
}

func parseSources(raw json.RawMessage) (map[string]agentSource, error) {
	out := map[string]agentSource{}
	if len(raw) == 0 {
		return out, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, errors.New(`The agent's out.json does not have "sources" as an object.`)
	}
	for key, v := range entries {
		var e struct {
			Heading    string `json:"heading"`
			Occurrence *int   `json:"occurrence"`
		}
		if err := json.Unmarshal(v, &e); err != nil || e.Heading == "" {
			return nil, errors.New(`The agent's "sources" entries need a heading, and an occurrence of at least one.`)
		}
		occurrence := 1
		if e.Occurrence != nil {
			if *e.Occurrence < 1 {
				return nil, errors.New(`The agent's "sources" entries need a heading, and an occurrence of at least one.`)
			}
			occurrence = *e.Occurrence
		}
		out[key] = agentSource{Heading: e.Heading, Occurrence: occurrence}
	}
	return out, nil
}

type sourceOut struct {
	Heading    string `json:"heading"`
	Occurrence int    `json:"occurrence"`
	Hash       string `json:"hash,omitempty"`
}

type viewOut struct {
	Version     int                  `json:"version"`
	GeneratedAt string               `json:"generatedAt"`
	Model       string               `json:"model,omitempty"`
	Request     string               `json:"request,omitempty"`
	Sources     map[string]sourceOut `json:"sources"`
	A2UI        json.RawMessage      `json:"a2ui"`
}

func (r *genRun) writeView(ctx context.Context, sources map[string]agentSource, a2ui json.RawMessage) error {
	if err := r.modelProblem(); err != nil {
		return err
	}
	view := viewOut{
		Version:     2,
		GeneratedAt: r.job.Now().UTC().Format(time.RFC3339),
		Model:       r.modelStamp(),
		Request:     r.job.Req.Request,
		Sources:     make(map[string]sourceOut, len(sources)),
		A2UI:        a2ui,
	}
	for key, src := range sources {
		entry := sourceOut{Heading: src.Heading, Occurrence: src.Occurrence}
		entry.Hash, _ = r.sectionHash(src)
		view.Sources[key] = entry
	}
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return r.outputError("The agent's out.json could not be turned into a view.")
	}
	content := append(data, '\n')
	if ctx.Err() != nil {
		return errCancelled
	}
	if r.job.Validate == nil {
		return validationUnavailable("no view validator is configured")
	}
	if r.trace != nil {
		r.trace.artifact("candidate", "", string(content))
		r.trace.emit("validation.start", nil)
	}
	validationErr := r.job.Validate(ctx, content)
	if ctx.Err() != nil {
		return errCancelled
	}
	if validationErr != nil {
		if r.trace != nil {
			r.trace.emit("validation.result", map[string]any{"ok": false, "error": validationErr.Error()})
		}
		return r.outputError(validationErr.Error())
	}
	if r.trace != nil {
		r.trace.emit("validation.result", map[string]any{"ok": true})
	}
	if ctx.Err() != nil {
		return errCancelled
	}
	if err := r.modelProblem(); err != nil {
		return err
	}
	var expect []byte
	if r.base != nil {
		expect = r.base.raw
	}
	if r.job.BeforeWrite != nil {
		if err := r.job.BeforeWrite(); err != nil {
			return err
		}
	}
	if err := replaceView(r.job.ViewPath, content, expect); err != nil {
		return err
	}
	r.trace.viewWritten(r.job.ViewPath, content)
	return nil
}

func (r *genRun) sectionHash(src agentSource) (hash string, ok bool) {
	key := normalizeKey(src.Heading)
	for _, sec := range r.job.Req.Sections {
		if sec.Occurrence != src.Occurrence || normalizeKey(sec.Heading) != key {
			continue
		}
		if h, valid := hashSection(r.job.Req.Text, sec); valid {
			return h, true
		}
	}
	return "", false
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func hashSection(text string, sec SectionRef) (string, bool) {
	h := fnv.New32a()
	at := sec.Start
	for _, cut := range sec.Exclude {
		if cut.Start < at || cut.End < cut.Start || cut.End > sec.End {
			return "", false
		}
		part, ok := sliceUTF16(text, at, cut.Start)
		if !ok {
			return "", false
		}
		h.Write([]byte(part))
		at = cut.End
	}
	part, ok := sliceUTF16(text, at, sec.End)
	if !ok {
		return "", false
	}
	h.Write([]byte(part))
	return fmt.Sprintf("fnv1a32:%08x", h.Sum32()), true
}

func hashUTF16Range(text string, start, end int) (string, bool) {
	return hashSection(text, SectionRef{Start: start, End: end})
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
	e := newFailure(failInvalidOutput, what, hintInvalidOutput)
	e.Detail = cutRunes(strings.TrimSpace(r.lastReply()), maxDetailRunes)
	return e
}

func cutRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

type progressSender struct {
	mu    sync.Mutex
	send  func(GenerateProgress) error
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
	if err := ps.send(GenerateProgress{Phase: phase, Text: text}); err != nil {
		return errCancelled
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return writeFileAtomicMode(path, data, mode)
}

func writeFileAtomicMode(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
