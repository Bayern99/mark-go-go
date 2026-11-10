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

// These initialize parameters are shared read-only by every ACP session.
var acpClientInfo = map[string]string{"name": acpClientName, "version": acpClientVersion}

var acpInitializeParams = map[string]any{
	"protocolVersion": acpProtocolVersion,
	"clientInfo":      acpClientInfo,
	"clientCapabilities": map[string]any{
		"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
		"terminal": false,
	},
}

// GenerateRequest is what the page sends when the reader clicks
// Generate: the editor text at that moment, the generation mode, the
// format definition, and the page's sections of the text.
type GenerateRequest struct {
	// Text is the editor text at that moment; the host writes it to
	// source.md for the agent to read, and hashes its sections from it.
	Text string `json:"text"`
	// Mode is "full", a new view, or "update", which replaces the
	// components ReplaceIDs names in the current view.
	Mode string `json:"mode"`
	// ReplaceIDs are the components an update replaces.
	ReplaceIDs []string `json:"replaceIds,omitempty"`
	// Request is the reader's generation request; may be empty.
	Request string `json:"request,omitempty"`
	// Format is the format document + catalog definition JSON,
	// embedded verbatim in the agent's prompt.
	Format string `json:"format"`
	// Sections are the page's heading sections of Text.
	Sections []SectionRef `json:"sections"`
}

// SectionRef is one heading section of the request's text, as the
// page's shared section module found it: UTF-16 offsets [Start, End)
// into Text. The host has no Markdown parser, so the page also says
// which ranges of the section are notes.
type SectionRef struct {
	Heading    string `json:"heading"`
	Occurrence int    `json:"occurrence"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	// Exclude are ranges of the section its hash leaves out, in order.
	Exclude []TextRange `json:"exclude,omitempty"`
}

// TextRange is a UTF-16 range [Start, End) of the request's text.
type TextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// GenerateProgress is one step of a running generation, sent to the
// page while the generation runs.
type GenerateProgress struct {
	// Phase is "starting", "thinking", "reading", "writing" or
	// "checking"; "failed" is the last step of a run that failed.
	Phase string `json:"phase"`
	// Text is a short, human-readable description of the step.
	Text string `json:"text"`
	// Failure is why the run failed, on the "failed" step only.
	Failure *GenerateFailure `json:"failure,omitempty"`
}

// errCancelled says the page or its window went away, or the run took
// too long: the run stops and the agent is stopped.
var errCancelled = errors.New("generation cancelled")

// errTimedOut is the cause of a run's context that ran out of time.
var errTimedOut = errors.New("generation timed out")

// defaultGenerationTimeout is how long a run may take.
const defaultGenerationTimeout = 5 * time.Minute

// acpPromptCancelAckTimeout bounds how long a cancelled prompt turn may
// take to answer its session/cancel. The answer is the turn's own end,
// so a prompt that acknowledges the cancellation completes the run
// at once; a process that stays silent is discarded after this bound.
const acpPromptCancelAckTimeout = time.Second

// acpSessionCloseTimeout bounds the session/close exchange that ends a
// settled run's session before its process may be leased again.
const acpSessionCloseTimeout = time.Second

// Generate runs the named document's view generation: the host starts the
// user's Antigravity agent over ACP in an isolated temporary folder
// with the request's captured text, the agent writes the view as out.json,
// the host stamps metadata and section hashes onto it and writes that
// document's current view file atomically. One slot per document; other
// documents may generate at the same time. Settings (model, agent command)
// are captured at start. A failed run sends the page its failure as the
// last step, then returns it as the error. Closing the document or the
// workspace cancels only that document's slot; a cancelled close does not.
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

// generate runs Generate for the named document.
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

// commitGenerationWrite allows the run to write only while this slot is
// still this document's live job, the document is still open, and its
// path is still the one captured at start. External rename/move or a
// real close must not silently write a stale view file.
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

// cancelGeneration stops this document's in-flight job and marks it so a
// finishing collect cannot write. It does not wait; Generate's defer
// still stops the agent and removes the temporary folder.
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

// waitForGenerations waits, for at most limit, until no generation
// runs. Quitting closes every window, which cancels their runs, but the
// process exits as soon as the app's loop returns: waiting lets each
// run stop its agent and remove its temporary folder first.
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

// GetSettings returns the generation settings.
func (h *documentHost) GetSettings() (Settings, error) {
	return h.settings.get(), nil
}

// SetSettings writes the fields the patch carries and returns the committed
// settings. Settings are one process-wide store, so a change from any window
// is announced to every window through settingsChanged.
func (h *documentHost) SetSettings(p SettingsPatch) (Settings, error) {
	return h.settings.apply(p)
}

// generationJob is one run of the generation, with everything the
// bound method would take from the app or its settings injected, so
// that tests drive it without a window: the request, the agent command
// and model, the view file to write, and the clock.
type generationJob struct {
	Req          GenerateRequest
	AgentCommand string
	Model        string
	ViewPath     string
	Now          func() time.Time
	// Settings, when non-nil, caches the models the agent offers on its
	// session/new handshake.
	Settings *settingsStore
	// Timeout is how long the run may take; zero is
	// defaultGenerationTimeout.
	Timeout time.Duration

	// Env is the environment of the agent process; nil inherits this
	// process's. Tests run a fake agent through it.
	Env []string

	// BeforeWrite, when set, runs immediately before the view file is
	// written. A non-nil error leaves the view file untouched.
	BeforeWrite func() error

	// Validate must check the exact complete candidate, including merged updates,
	// before any view or Undo backup is written. Production uses the page's
	// authoritative Catalog; headless jobs inject that external verdict.
	Validate func(context.Context, []byte) error

	// Observe, when set, gets the run's evidence as it happens: the raw
	// JSON-RPC lines both ways, the handshake and config answers, the
	// phases, the exact files, and one terminal run.end event. The
	// observer owns the run ID and elapsed time. Nil costs the run
	// nothing but a nil check: no extra marshal or copy.
	Observe func(event string, payload any)

	// ThoughtLevel, when non-empty, is the thought_level config option
	// this run asks the agent for. It is sent only when the agent's own
	// session/new configOptions offer that exact value; otherwise the
	// run records that it was not supported and continues. Empty sends
	// nothing and keeps the agent's current level.
	ThoughtLevel string

	// AgentCache, when non-nil, lets the run lease one idle ACP agent
	// process and open a fresh session on it. Nil is the ordinary
	// one-shot process: started for the run and stopped with it.
	AgentCache *acpProcessCache

	// Prepare is an optional in-process experiment: it supplies a prompt and
	// compiles content-only output into the ordinary view contract. Ordinary
	// builds leave it nil and use the full A2UI generation path.
	Prepare func(context.Context, GenerateRequest, string) (string, func([]byte) ([]byte, error), error)
}

// runGeneration makes one view, as Generate does, with the job's
// injections. On any error, a *genError, the view file is untouched,
// the agent process is stopped and the temporary folder is gone.
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
	// interrupted is the failure of a run whose context ended, or
	// whose page stopped listening.
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
	// Deferred calls run last in, first out: the agent is stopped
	// before the folder with its files goes away.
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
		// A leased or one-shot process starts its readers here; an idle
		// cached process already has them.
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

// finishAgent ends the run's use of its process: a cache lease returns
// a healthy, closed process to the cache, and everything else is
// stopped. The one-shot path stops its process as before.
func (r *genRun) finishAgent() {
	if r.agent != nil {
		// The run is over: a late event of its session must not reach
		// its handlers, and an idle process answers no request of it.
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

// execute runs the whole conversation with the agent: the handshake,
// the prompt turn, and the view it produced.
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
		// A cached cold process keeps its real initialize answer for
		// every later lease, even when this first run has no observer.
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
			// The cancelled turn answered: end its session so the
			// process can be reused.
			r.settleSession()
		}
		return err
	}
	r.trace.promptResult(turnedRaw)
	switch turned.StopReason {
	case "", "end_turn":
		// The turn completed; the view may follow.
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
		// The agent keeps running after its answer; stop it before reading.
		r.agent.kill()
	} else if !r.settleSession() {
		// Closure of the fresh session cannot be proven: discard the
		// process and fall back to the old stop-before-reading path.
		r.agent.kill()
	}
	return r.collect(ctx)
}

// applyThoughtLevel asks for the job's thought level only when the
// agent's own session/new configOptions offer that exact value. An
// agent without the option, or without the value, is recorded as not
// supporting it and the run goes on with the agent's current level.
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

// genRun is the state of one running generation.
type genRun struct {
	job  generationJob
	dir  string
	send *progressSender
	// base is the current view an update replaces components of; nil
	// for a full generation.
	base *baseView

	// trace, when the job has an observer, gets this run's evidence;
	// it is nil otherwise.
	trace *generationTrace

	promptText     string
	assembleOutput func([]byte) ([]byte, error)

	agent *acpAgent
	// lease is the cached process lease; nil for a one-shot process.
	lease *acpLease

	// sessionMu guards sessionID, which the agent's reader goroutine
	// reads while the run sets it. sessionClosed records that the fresh
	// session's bounded session/close succeeded; only then may the lease
	// return the process to the cache.
	sessionMu     sync.RWMutex
	sessionID     string
	sessionClosed bool

	// cancelMu guards cancelAcknowledged: a cancelled prompt turn
	// answered its session/cancel, so no turn is running any more.
	cancelMu           sync.Mutex
	cancelAcknowledged bool

	// modelMu guards the model the agent actually reported for this
	// run: only a confirmed match may be stamped on the view, and a
	// reported mismatch fails the run instead of silently succeeding.
	modelMu        sync.Mutex
	modelRequested bool
	modelActual    string
	modelConfirmed bool
	modelMismatch  string

	// toolCalls remembers the kind and locations of each tool call by
	// id, so that a tool_call_update, whose fields are optional, is
	// checked when it omits them. Only the agent's reader goroutine
	// touches it.
	toolCalls map[string]toolCallState

	// abortCh stops the run from inside a notification handler, when a
	// tool call went outside the working folder; abortReason is the
	// message of the failure to return for it.
	abortCh     chan struct{}
	abortOnce   sync.Once
	abortReason string

	replyMu sync.Mutex
	reply   strings.Builder // the agent's last reply: its message chunks
}

// callAgent sends one request to the agent and waits for its answer,
// the end of the run's context (errCancelled), or an abort from a
// notification handler. Every other failure is a *genError: an agent
// that is not signed in, the agent's error answer to the prompt (its
// own words, as a quota running out), or an agent-error.
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

// agentCallResult turns one answer of the agent into the run's error.
func (r *genRun) agentCallResult(method string, err error) error {
	if err == nil {
		return nil
	}
	var rpc *acpError
	if !errors.As(err, &rpc) {
		// The agent exited or its answer was malformed; an agent
		// that is not signed in may say so on its error output.
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

// cancelPromptTurn asks the agent to cancel the prompt turn the run's
// context just ended and waits, for at most acpPromptCancelAckTimeout,
// for that turn to answer. The answer is the turn's own end: an agent
// that acknowledges the cancellation completes the run at once instead
// of holding it until a process exit that may never come. The report of
// a broken process is not an acknowledgement; neither is silence, which
// runGeneration resolves by discarding the process.
func (r *genRun) cancelPromptTurn(method string, done <-chan error) {
	sessionID := r.session()
	if method != "session/prompt" || sessionID == "" || r.agent == nil {
		return
	}
	// A child that stopped reading can block this write; it must not
	// delay the bounded wait below.
	r.agent.cancelTurn(sessionID)
	timer := time.NewTimer(acpPromptCancelAckTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			r.markCancelAcknowledged()
		}
	case <-r.abortCh:
		// abort kills the process; the pending call ends with it.
	case <-timer.C:
		// Silent or stuck: runGeneration discards the process.
	}
}

// markCancelAcknowledged records that the cancelled prompt turn ended.
func (r *genRun) markCancelAcknowledged() {
	r.cancelMu.Lock()
	r.cancelAcknowledged = true
	r.cancelMu.Unlock()
}

// turnWasAcknowledged reports whether the cancelled prompt turn answered.
func (r *genRun) turnWasAcknowledged() bool {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	return r.cancelAcknowledged
}

// settleSession ends this run's fresh session so the process may be
// reused: it closes the session when the agent advertised the close
// capability and the bounded close answer arrived. It reports whether
// closure was proven; false means the process must be discarded.
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

// closeSessionBounded sends session/close and waits, for at most
// acpSessionCloseTimeout, for its answer. A stuck write or answer is
// bounded; the process is then discarded and killed.
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

// stopAfterCancel tears a cancelled run's agent down without waiting
// for the agent to exit on its own. A cancelled prompt turn that
// answered is settled (and, with a cache, its session closed so the
// process can be reused); an unresponsive or uncertain turn is
// discarded, so the page completes within a bound in every case.
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

// abort stops the run from inside a notification handler: it cancels
// the agent's turn and kills it, without waiting for its exit (this
// runs on the agent's read goroutine, which must not block on the
// process: the reaper and runGeneration's kill do the waiting), and
// makes the waiting call return msg.
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

// lastReply returns the agent's reply so far: its message chunks.
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

// noteModelOptions reads the model currentValue the agent reported in
// session/new configOptions, a config answer, or a config update. A
// value different from the requested model is a mismatch the run may
// not silently pass; a match confirms the value the view may carry. No
// currentValue leaves it unconfirmed, and the view then names no model.
func (r *genRun) noteModelOptions(options json.RawMessage) {
	if r.job.Model == "" || len(options) == 0 {
		return
	}
	r.modelMu.Lock()
	requested := r.modelRequested
	r.modelMu.Unlock()
	if !requested {
		// A config notice before this run asked for its model describes
		// the session's default, not the outcome of the request.
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

// markModelRequested records that the run has sent its model request;
// config notices from before it describe the agent's default.
func (r *genRun) markModelRequested() {
	r.modelMu.Lock()
	r.modelRequested = true
	r.modelMu.Unlock()
}

// modelProblem is the failure of a reported model mismatch, if any.
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

// modelStamp is the model the view may claim: the confirmed actual
// model only, never the requested value merely because it was asked for.
func (r *genRun) modelStamp() string {
	r.modelMu.Lock()
	defer r.modelMu.Unlock()
	if r.modelConfirmed && r.modelActual == r.job.Model {
		return r.modelActual
	}
	return ""
}

// notificationBelongsToSession drops a session-scoped message for
// another session, such as a late old-session event of a reused
// process. A message without a session ID is kept: sparse updates are
// matched by their tool call instead.
func (r *genRun) notificationBelongsToSession(params json.RawMessage) bool {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(params, &p) != nil || p.SessionID == "" {
		return true
	}
	return p.SessionID == r.session()
}

// handleNotification reacts to the agent's notifications: it keeps the
// agent's last reply, sends the page the progress of the turn, and
// stops the run when a tool call goes outside the working folder.
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

// toolCallState is what the host remembers about one tool call, so
// that a tool_call_update, whose fields are optional, can still be
// checked when it omits its kind or locations.
type toolCallState struct {
	kind      string
	locations []acpLocation
}

// notificationToolAllowed judges only what a tool notification really
// reports. A missing kind or missing locations is an observation, not a
// violation; a reported path outside the folder, or a reported kind
// outside the allowed operations, still stops the run.
func notificationToolAllowed(dir, kind string, locations []string) bool {
	if kind == "" {
		// No operation to judge, but a reported path outside the folder
		// is explicit and unsafe.
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

// notificationPathInside says whether a reported location resolves to
// the working folder or below it.
func notificationPathInside(dir, loc string) bool {
	resolved := resolveToolPath(dir, loc)
	rel, err := filepath.Rel(dir, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// handleToolCall treats one tool notification as an observation: it
// remembers the kind and locations a call reports, lets a sparse update
// fall back to them, and sends the page the phase of the work. Only an
// explicitly reported illegal operation or path stops the run; missing
// kind or locations, an unknown id and any status are accepted, and no
// notification grants a permission. Permission requests keep their own
// conservative policy in handleRequest.
func (r *genRun) handleToolCall(id, kind, title, name string, locations []acpLocation) {
	prior := r.toolCalls[id]
	effectiveKind := kind
	if effectiveKind == "" {
		effectiveKind = prior.kind
	}
	effectiveLocations := locations
	if len(effectiveLocations) == 0 && (kind == "" || kind == prior.kind) {
		// A status-only update omits both fields; the call's own earlier
		// report still describes it. A new kind without paths does not
		// inherit locations that described another operation.
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

// handleRequest answers the agent's requests: a permission request
// gets the isolation policy's answer, everything else is method not
// found.
func (r *genRun) handleRequest(method string, params json.RawMessage) (any, *acpError) {
	if method != "session/request_permission" {
		return nil, &acpError{Code: -32601, Message: "method not found"}
	}
	if !r.notificationBelongsToSession(params) {
		// A request that names another session is a late old-session
		// event on a reused process: do not judge it with this run's
		// policy, and do not let it work.
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

// choosePermissionOption picks the permission option the policy's
// answer calls for: an allow_once to allow, a reject_once to deny,
// with the other kinds of the same answer as fallback. None of them
// fits, and an empty option id comes back, when the agent offered
// none.
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

// quoteAll quotes each string, for listing ids in the prompt.
func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strconv.Quote(s)
	}
	return out
}

// configValues returns the option values of one config option, such as
// the model IDs the agent offers, from a session/new configOptions.
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

// resolveAgentCommand turns the settings' agent command into an
// executable path: a command with a path separator is used as it is,
// one without is looked up on PATH and, since an app started from
// Finder has a minimal PATH, in the usual local binary folders.
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

// agentSource is one entry of the agent's out.json sources.
type agentSource struct {
	Heading    string
	Occurrence int
}

// collect reads the agent's out.json, validates it, stamps the view's
// metadata and section hashes onto it, and writes the view file
// atomically. The view file is untouched on any failure.
func (r *genRun) collect(ctx context.Context) error {
	outPath := filepath.Join(r.dir, "out.json")
	fi, err := os.Lstat(outPath)
	if errors.Is(err, fs.ErrNotExist) {
		// An agent that answers without a view says why, as when its
		// quota ran out: the reader needs its words, not ours.
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

// parseSources validates the sources of the agent's out.json: an
// object whose entries each have a non-empty heading, and an
// occurrence of at least one when it has one. Absent sources are no
// sources.
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

// sourceOut is one entry of the sources of a written view file: the
// heading, the occurrence, and the hash of the section's text when the
// section was found in the request.
type sourceOut struct {
	Heading    string `json:"heading"`
	Occurrence int    `json:"occurrence"`
	Hash       string `json:"hash,omitempty"`
}

// viewOut is a written view file, with its keys in this order.
type viewOut struct {
	Version     int                  `json:"version"`
	GeneratedAt string               `json:"generatedAt"`
	Model       string               `json:"model,omitempty"`
	Request     string               `json:"request,omitempty"`
	Sources     map[string]sourceOut `json:"sources"`
	A2UI        json.RawMessage      `json:"a2ui"`
}

// writeView stamps the view's metadata and section hashes onto the
// agent's output, every source hashed again from this run's text, and
// writes the view file atomically, keeping the view it replaces as the
// backup.
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

// sectionHash finds the request section a source names, by its
// normalized heading and occurrence, and hashes the section's text in
// UTF-16 units, as the page does. ok is false when no section matches,
// or its range does not fit the text: the entry then gets no hash.
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

// normalizeKey normalizes a heading for matching sources to sections:
// whitespace runs become one space, the ends are trimmed, and the case
// is lowered, all Unicode-aware.
func normalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// hashSection hashes a section the way the page does: the UTF-8 bytes
// of its UTF-16 range without its excluded ranges, FNV-1a 32, as
// fnv1a32:xxxxxxxx. ok is false when a range does not fit the section
// or the text, or a boundary falls inside a surrogate pair.
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

// hashUTF16Range hashes the UTF-16 range [start, end) of text, the way
// the page hashes a section without notes.
func hashUTF16Range(text string, start, end int) (string, bool) {
	return hashSection(text, SectionRef{Start: start, End: end})
}

// sliceUTF16 returns the UTF-16 range [start, end) of text. ok is false
// when the range does not fit, or a boundary falls inside a surrogate
// pair.
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
	// The loop only looks at where runes start; the end of the text is a
	// valid start too.
	if byteStart < 0 && units == start {
		byteStart = len(text)
	}
	if byteStart < 0 || byteStart > byteEnd {
		return "", false
	}
	return text[byteStart:byteEnd], true
}

// outputError explains why the agent's output could not become a view,
// with the agent's last reply, its message chunks, as the detail.
func (r *genRun) outputError(what string) error {
	e := newFailure(failInvalidOutput, what, hintInvalidOutput)
	e.Detail = cutRunes(strings.TrimSpace(r.lastReply()), maxDetailRunes)
	return e
}

// cutRunes cuts s to max runes, marking the cut with an ellipsis.
func cutRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

// progressSender sends the page the steps of a run, only when the
// phase or the text changed, so that the page is not flooded. Safe for
// concurrent use: the notifications of the agent arrive on their own
// goroutine. When the page stopped listening, step returns
// errCancelled.
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

// writeFileAtomic writes data to path through a temporary file in
// path's directory, so that a reader never sees a partial file: it
// writes, syncs, closes, and renames over path, removing the
// temporary file when anything fails. The file keeps the permissions
// of the file it replaces, or gets 0644 when there was none: the
// temporary file would otherwise leave it readable to its owner only.
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return writeFileAtomicMode(path, data, mode)
}

// writeFileAtomicMode is writeFileAtomic with the permissions given.
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
