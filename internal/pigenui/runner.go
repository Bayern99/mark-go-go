package pigenui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/pirpc"
)

// Runner is Pi's GenUI backend. One Runner serves one generation: ExecuteTurn
// starts a process, finishes the turn and ends that process again, so nothing
// outlives the turn that needed it.
type Runner struct {
	cfg Config

	mu   sync.Mutex
	live *turn
}

// Probe answers what can be learned without starting a session that could
// generate: whether the CLI resolves and whether the selected provider's
// credentials are ready.
func (r *Runner) Probe(ctx context.Context) (genui.ProbeResult, error) {
	return Probe(ctx, r.cfg)
}

// ListModels reads the models Pi offers and the levels the current one
// supports, without submitting a prompt.
func (r *Runner) ListModels(ctx context.Context) (genui.ModelCatalog, error) {
	return ListModels(ctx, r.cfg)
}

func (r *Runner) Close() error {
	r.mu.Lock()
	live := r.live
	r.live = nil
	r.mu.Unlock()
	if live != nil {
		live.shutdown(r.cfg.abortGrace())
	}
	return nil
}

func (r *Runner) ExecuteTurn(ctx context.Context, turn genui.Turn) (genui.TurnResult, error) {
	var none genui.TurnResult
	live, cleanupGuard, err := r.start(turn.Dir, turn.Progress)
	if err != nil {
		return none, err
	}
	defer cleanupGuard()
	defer r.stop(live)
	// A turn that ends because it was cancelled, timed out or lost its window
	// asks Pi to stop before the process is closed, so nothing keeps working
	// after the reader has moved on.
	defer func() {
		if ctx.Err() != nil {
			live.cancel(r.cfg.abortGrace())
		}
	}()

	if err := live.awaitGuard(ctx, r.cfg.guardGrace()); err != nil {
		return none, err
	}
	if err := live.step("starting", "Agent ready"); err != nil {
		return none, err
	}
	if err := live.applyModel(ctx, r.cfg.Provider, r.cfg.Model); err != nil {
		return none, err
	}
	if err := live.applyThoughtLevel(ctx, turn.ThoughtLevel); err != nil {
		return none, err
	}
	disposition, err := live.submitPrompt(ctx, turn.Prompt)
	if err != nil {
		return none, err
	}
	// "handled" means an extension consumed the prompt, so no run started and
	// no agent_settled will follow.
	if disposition != "handled" {
		select {
		case <-live.settledCh():
		case <-live.client.Exited():
			return none, live.goneFailure()
		case <-ctx.Done():
			return none, genui.ErrCancelled
		}
	}
	if failure := live.failureErr(); failure != nil {
		return none, failure
	}
	return genui.TurnResult{Model: live.modelStamp(), Reply: live.replyText()}, nil
}

// start begins the turn's own process and returns the cleanup that removes the
// guard it was launched with.
func (r *Runner) start(dir string, progress func(genui.GenerateProgress) error) (*turn, func(), error) {
	live, cleanupGuard, err := startTurn(r.cfg, dir, progress)
	if err != nil {
		return nil, func() {}, err
	}
	r.mu.Lock()
	r.live = live
	r.mu.Unlock()
	return live, cleanupGuard, nil
}

func (r *Runner) stop(live *turn) {
	r.mu.Lock()
	if r.live == live {
		r.live = nil
	}
	r.mu.Unlock()
	live.shutdown(r.cfg.abortGrace())
}

// turn owns one Pi process for one generation: the guard handshake, the model
// and level writes, the prompt run and what Pi said while it ran. Nothing in it
// is shared with another turn.
type turn struct {
	client   *pirpc.Client
	token    string
	progress func(genui.GenerateProgress) error

	mu          sync.Mutex
	settled     chan struct{}
	settledDone bool
	guard       chan string
	reply       string
	model       string
	failure     error
	command     string
}

// startTurn writes this app's guard, starts Pi in dir, and installs the turn's
// handlers before any record is read.
func startTurn(cfg Config, dir string, progress func(genui.GenerateProgress) error) (*turn, func(), error) {
	guardPath, cleanupGuard, err := writeGuard()
	if err != nil {
		return nil, func() {}, err
	}
	token := newToken()
	cmd, err := piCommand(cfg, dir, guardPath, token)
	if err != nil {
		cleanupGuard()
		return nil, func() {}, err
	}
	client, err := pirpc.New(pirpc.Config{Command: cmd, Name: "Pi"})
	if err != nil {
		cleanupGuard()
		return nil, func() {}, newStartFailure(cfg.Command, err)
	}
	if progress == nil {
		progress = func(genui.GenerateProgress) error { return nil }
	}
	live := &turn{
		client:   client,
		token:    token,
		progress: progress,
		settled:  make(chan struct{}),
		guard:    make(chan string, 4),
	}
	client.SetHandlers(live.handleEvent, live.handleUIRequest)
	client.Start()
	return live, cleanupGuard, nil
}

func (t *turn) shutdown(grace time.Duration) {
	t.client.SetHandlers(nil, nil)
	// The guard channel is dropped rather than closed: a late status report from
	// a process that is already leaving must not panic a sender.
	t.mu.Lock()
	t.guard = nil
	t.mu.Unlock()
	t.client.Shutdown(grace)
}

// awaitGuard waits for the guard extension to report in. Without that report
// this app has no evidence that the tool calls are constrained, so no prompt is
// sent: a Pi process that is not guarded must never generate.
func (t *turn) awaitGuard(ctx context.Context, grace time.Duration) error {
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case status, ok := <-t.guardChannel():
		if !ok {
			return t.guardFailure()
		}
		return readGuardStatus(status).accept(t.token)
	case <-t.client.Exited():
		return t.guardFailure()
	case <-timer.C:
		t.client.Kill()
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access did not report in, so nothing was generated.", guardHint)
	case <-ctx.Done():
		t.client.Kill()
		return genui.ErrCancelled
	}
}

func (t *turn) guardFailure() error {
	tail := strings.TrimSpace(t.client.StderrTail())
	failure := genui.NewFailure(genui.FailAgentError,
		"Pi stopped before the guard that restricts its file access loaded, so nothing was generated.", guardHint)
	if tail != "" {
		failure.Detail = genui.CutRunes(tail, genui.MaxDetailRunes)
	}
	return failure
}

// guardState is the guard extension's own word on whether it is enforcing the
// paths this app handed it.
type guardState struct {
	state   string
	version string
	token   string
}

func readGuardStatus(text string) guardState {
	parts := strings.SplitN(text, ":", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return guardState{state: parts[0], version: parts[1], token: parts[2]}
}

// guardHint is what a reader is told when nothing ran: the refusal is the safe
// outcome, and only a broken installation can explain it.
const guardHint = "Nothing was generated. Reinstall Mark Go Go if this keeps happening."

func (g guardState) accept(token string) error {
	switch {
	case g.state == "ready" && g.version == GuardVersion && g.token == token:
		return nil
	case g.state == "ready":
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access is not the version this app expects, so nothing was generated.", guardHint)
	case g.state == "mismatch":
		return genui.NewFailure(genui.FailAgentError,
			fmt.Sprintf("The guard that restricts Pi's file access reported version %q, which this app does not speak, so nothing was generated.", g.version), guardHint)
	case g.state == "unconfigured":
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access started without the paths this turn allows, so nothing was generated.", guardHint)
	default:
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access did not report a known state, so nothing was generated.", guardHint)
	}
}

// applyModel asks Pi for the model the reader chose. Pi names both the model and
// its provider, and keeps its own current model when this app recorded no
// provider for it, rather than writing a value the agent would refuse.
func (t *turn) applyModel(ctx context.Context, provider, model string) error {
	model = strings.TrimSpace(model)
	if model == "" || strings.TrimSpace(provider) == "" {
		return nil
	}
	var applied struct {
		ID string `json:"id"`
	}
	if err := t.call(ctx, "set_model", map[string]any{
		"provider": provider,
		"modelId":  model,
	}, &applied); err != nil {
		return err
	}
	t.mu.Lock()
	t.model = applied.ID
	t.mu.Unlock()
	return nil
}

// applyThoughtLevel asks for the level only when the current model offers it. A
// level the model does not declare is left unwritten so the run keeps Pi's own
// default instead of failing.
func (t *turn) applyThoughtLevel(ctx context.Context, level string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		return nil
	}
	var offered struct {
		Levels []string `json:"levels"`
	}
	if err := t.call(ctx, "get_available_thinking_levels", nil, &offered); err != nil {
		return err
	}
	if !slices.Contains(offered.Levels, level) {
		return nil
	}
	return t.call(ctx, "set_thinking_level", map[string]any{"level": level}, nil)
}

func (t *turn) submitPrompt(ctx context.Context, prompt string) (string, error) {
	var accepted struct {
		Disposition string `json:"disposition"`
	}
	if err := t.call(ctx, "prompt", map[string]any{"message": prompt}, &accepted); err != nil {
		return "", err
	}
	return accepted.Disposition, nil
}

// call runs one command with the turn's cancellation: Pi answers quickly except
// for a prompt, which is acknowledged before the run continues as events.
func (t *turn) call(ctx context.Context, command string, fields map[string]any, result any) error {
	if err := ctx.Err(); err != nil {
		return genui.ErrCancelled
	}
	t.mu.Lock()
	t.command = command
	t.mu.Unlock()
	answered := make(chan error, 1)
	go func() { answered <- t.client.Call(command, fields, result) }()
	select {
	case err := <-answered:
		if err == nil {
			return nil
		}
		return genui.AgentError(err.Error())
	case <-ctx.Done():
		return genui.ErrCancelled
	case <-t.client.Exited():
		return t.goneFailure()
	}
}

// cancel asks Pi to stop and gives it a short grace before the whole process
// group is killed, so a cancelled turn cannot leave work running.
func (t *turn) cancel(grace time.Duration) {
	// The abort is written before the wait starts, so the grace period is spent
	// on Pi's own work rather than on this app getting around to asking.
	_ = t.client.Send("abort", nil)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-t.settledCh():
	case <-t.client.Exited():
	case <-timer.C:
		t.client.Kill()
	}
}

func (t *turn) goneFailure() error {
	t.mu.Lock()
	command := t.command
	t.mu.Unlock()
	tail := strings.TrimSpace(t.client.StderrTail())
	message := "Pi stopped before it finished the view."
	if command != "" {
		message = "Pi stopped before answering " + command + "."
	}
	failure := genui.NewFailure(genui.FailAgentError, message, genui.HintAgentError)
	if tail != "" {
		failure.Detail = genui.CutRunes(tail, genui.MaxDetailRunes)
	}
	return failure
}

func (t *turn) step(phase, text string) error {
	return t.progress(genui.GenerateProgress{Phase: phase, Text: text})
}

func (t *turn) settle() {
	t.mu.Lock()
	settled := t.settled
	if settled == nil || t.settledDone {
		t.mu.Unlock()
		return
	}
	t.settledDone = true
	t.mu.Unlock()
	close(settled)
}

func (t *turn) settledCh() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settled
}

func (t *turn) guardChannel() <-chan string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.guard
}

func (t *turn) replyText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reply
}

func (t *turn) modelStamp() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.model
}

func (t *turn) failureErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failure
}

func (t *turn) fail(err error) {
	t.mu.Lock()
	if t.failure == nil {
		t.failure = err
	}
	t.mu.Unlock()
}

// handleEvent turns Pi's session events into what the reader sees. Pi nests the
// streaming deltas in message_update.assistantMessageEvent and reports tool runs
// as top-level tool_execution records.
func (t *turn) handleEvent(eventType string, raw json.RawMessage) {
	switch eventType {
	case "message_update":
		var update struct {
			Event struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
				Error string `json:"error"`
			} `json:"assistantMessageEvent"`
		}
		if json.Unmarshal(raw, &update) != nil {
			return
		}
		switch update.Event.Type {
		case "thinking_delta":
			_ = t.step("thinking", "Thinking")
		case "error":
			if message := strings.TrimSpace(update.Event.Error); message != "" {
				t.fail(genui.NewFailure(genui.FailAgentMessage, "Pi's model stopped: "+message, genui.HintAgentMessage))
			}
		}

	case "message_end":
		var ended struct {
			Message struct {
				Role         string          `json:"role"`
				Content      json.RawMessage `json:"content"`
				StopReason   string          `json:"stopReason"`
				ErrorMessage string          `json:"errorMessage"`
			} `json:"message"`
		}
		if json.Unmarshal(raw, &ended) != nil || ended.Message.Role != "assistant" {
			return
		}
		t.mu.Lock()
		t.reply = assistantText(ended.Message.Content)
		t.mu.Unlock()
		switch ended.Message.StopReason {
		case "error":
			message := strings.TrimSpace(ended.Message.ErrorMessage)
			if message == "" {
				message = "Pi's model stopped before answering."
			}
			t.fail(genui.NewFailure(genui.FailAgentMessage, message, genui.HintAgentMessage))
		case "aborted":
			t.fail(genui.ErrCancelled)
		}

	case "tool_execution_start":
		var started struct {
			ToolName string `json:"toolName"`
		}
		if json.Unmarshal(raw, &started) != nil {
			return
		}
		switch started.ToolName {
		case "read":
			_ = t.step("reading", "Reading its files")
		case "write", "edit":
			_ = t.step("writing", "Writing the view")
		}

	case "agent_settled":
		t.settle()
	}
}

// assistantText is the text of one assistant message, which Pi sends as content
// blocks.
func assistantText(content json.RawMessage) string {
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var text strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

// handleUIRequest answers Pi's extension UI records. The guard's own status
// report is how this app learns the guard is active; a dialog this app cannot
// show is declined and the run is aborted; notifications are dropped, because a
// generation has no window to show them in.
func (t *turn) handleUIRequest(raw json.RawMessage) {
	var request struct {
		ID         string `json:"id"`
		Method     string `json:"method"`
		StatusKey  string `json:"statusKey"`
		StatusText string `json:"statusText"`
	}
	if json.Unmarshal(raw, &request) != nil {
		return
	}
	switch request.Method {
	case "select", "confirm", "input", "editor":
		if request.ID != "" {
			_ = t.client.SendUIResponse(request.ID, map[string]any{"cancelled": true})
		}
		_ = t.client.Send("abort", nil)
		t.settle()
	case "setStatus":
		if request.StatusKey != guardStatusKey {
			return
		}
		t.mu.Lock()
		guard := t.guard
		t.mu.Unlock()
		if guard == nil {
			return
		}
		select {
		case guard <- request.StatusText:
		default:
		}
	}
}
