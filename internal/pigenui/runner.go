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

	mu          sync.Mutex
	client      *pirpc.Client
	progress    func(genui.GenerateProgress) error
	settled     chan struct{}
	settledDone bool
	guard       chan string
	reply       string
	model       string
	failure     error
	lastCommand string
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
	client := r.client
	r.client = nil
	r.mu.Unlock()
	if client != nil {
		client.Shutdown(r.cfg.abortGrace())
	}
	return nil
}

func (r *Runner) ExecuteTurn(ctx context.Context, turn genui.Turn) (genui.TurnResult, error) {
	var none genui.TurnResult
	progress := turn.Progress
	if progress == nil {
		progress = func(genui.GenerateProgress) error { return nil }
	}

	guardPath, cleanupGuard, err := writeGuard()
	if err != nil {
		return none, err
	}
	defer cleanupGuard()

	token := newToken()
	cmd, err := piCommand(r.cfg, turn.Dir, guardPath, token)
	if err != nil {
		return none, err
	}
	client, err := pirpc.New(pirpc.Config{Command: cmd, Name: "Pi"})
	if err != nil {
		return none, newStartFailure(r.cfg.Command, err)
	}
	r.begin(client, progress)
	client.SetHandlers(r.handleEvent, r.handleUIRequest)
	client.Start()
	defer func() {
		client.SetHandlers(nil, nil)
		r.end(client)
		client.Shutdown(r.cfg.abortGrace())
	}()
	// A turn that ends because it was cancelled, timed out or lost its window
	// asks Pi to stop before the process is closed, so nothing keeps working
	// after the reader has moved on.
	defer func() {
		if ctx.Err() != nil {
			r.cancel()
		}
	}()

	if err := r.awaitGuard(ctx, client, token); err != nil {
		return none, err
	}
	if err := r.step("starting", "Agent ready"); err != nil {
		return none, err
	}
	if err := r.applyModel(ctx); err != nil {
		return none, err
	}
	if err := r.applyThoughtLevel(ctx, turn.ThoughtLevel); err != nil {
		return none, err
	}
	disposition, err := r.submitPrompt(ctx, turn.Prompt)
	if err != nil {
		return none, err
	}
	// "handled" means an extension consumed the prompt, so no run started and
	// no agent_settled will follow.
	if disposition != "handled" {
		select {
		case <-r.settledCh():
		case <-client.Exited():
			return none, r.goneFailure(client)
		case <-ctx.Done():
			return none, genui.ErrCancelled
		}
	}
	if failure := r.turnFailure(); failure != nil {
		return none, failure
	}
	return genui.TurnResult{Model: r.modelStamp(), Reply: r.turnReply()}, nil
}

func (r *Runner) begin(client *pirpc.Client, progress func(genui.GenerateProgress) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.client = client
	r.progress = progress
	r.settled = make(chan struct{})
	r.settledDone = false
	r.guard = make(chan string, 4)
	r.reply, r.model, r.failure, r.lastCommand = "", "", nil, ""
}

func (r *Runner) end(client *pirpc.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == client {
		r.client = nil
	}
	// The guard channel is dropped rather than closed: a late status report
	// from a process that is already leaving must not panic a sender.
	r.guard = nil
}

// awaitGuard waits for the guard extension to report in. Without that report
// this app has no evidence that the tool calls are constrained, so no prompt is
// sent: a Pi process that is not guarded must never generate.
func (r *Runner) awaitGuard(ctx context.Context, client *pirpc.Client, token string) error {
	timer := time.NewTimer(r.cfg.guardGrace())
	defer timer.Stop()
	select {
	case status, ok := <-r.guardStatus():
		if !ok {
			return r.guardFailure(client)
		}
		return readGuardStatus(status).accept(token)
	case <-client.Exited():
		return r.guardFailure(client)
	case <-timer.C:
		client.Kill()
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access did not report in, so nothing was generated.", guardHint)
	case <-ctx.Done():
		client.Kill()
		return genui.ErrCancelled
	}
}

func (r *Runner) guardFailure(client *pirpc.Client) error {
	tail := strings.TrimSpace(client.StderrTail())
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
func (r *Runner) applyModel(ctx context.Context) error {
	model := strings.TrimSpace(r.cfg.Model)
	if model == "" || strings.TrimSpace(r.cfg.Provider) == "" {
		return nil
	}
	var applied struct {
		ID string `json:"id"`
	}
	if err := r.call(ctx, "set_model", map[string]any{
		"provider": r.cfg.Provider,
		"modelId":  model,
	}, &applied); err != nil {
		return err
	}
	r.mu.Lock()
	r.model = applied.ID
	r.mu.Unlock()
	return nil
}

// applyThoughtLevel asks for the level only when the current model offers it.
// A level the model does not declare is left unwritten so the run keeps Pi's own
// default instead of failing.
func (r *Runner) applyThoughtLevel(ctx context.Context, level string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		return nil
	}
	var offered struct {
		Levels []string `json:"levels"`
	}
	if err := r.call(ctx, "get_available_thinking_levels", nil, &offered); err != nil {
		return err
	}
	if !slices.Contains(offered.Levels, level) {
		return nil
	}
	return r.call(ctx, "set_thinking_level", map[string]any{"level": level}, nil)
}

func (r *Runner) submitPrompt(ctx context.Context, prompt string) (string, error) {
	var accepted struct {
		Disposition string `json:"disposition"`
	}
	if err := r.call(ctx, "prompt", map[string]any{"message": prompt}, &accepted); err != nil {
		return "", err
	}
	return accepted.Disposition, nil
}

// call runs one command with the turn's cancellation: Pi answers quickly except
// for a prompt, which is acknowledged before the run continues as events.
func (r *Runner) call(ctx context.Context, command string, fields map[string]any, result any) error {
	if err := ctx.Err(); err != nil {
		return genui.ErrCancelled
	}
	client := r.pirpcClient()
	if client == nil {
		return genui.AgentError("Pi is not running.")
	}
	r.mu.Lock()
	r.lastCommand = command
	r.mu.Unlock()
	ch := make(chan error, 1)
	go func() { ch <- client.Call(command, fields, result) }()
	select {
	case err := <-ch:
		if err == nil {
			return nil
		}
		return genui.AgentError(err.Error())
	case <-ctx.Done():
		return genui.ErrCancelled
	case <-client.Exited():
		return r.goneFailure(client)
	}
}

// cancel asks Pi to stop and gives it a short grace before the whole process
// group is killed, so a cancelled turn cannot leave work running.
func (r *Runner) cancel() {
	client := r.pirpcClient()
	if client == nil {
		return
	}
	// The abort is written before the wait starts, so the grace period is spent
	// on Pi's own work rather than on this app getting around to asking.
	_ = client.Send("abort", nil)
	timer := time.NewTimer(r.cfg.abortGrace())
	defer timer.Stop()
	select {
	case <-r.settledCh():
	case <-client.Exited():
	case <-timer.C:
		client.Kill()
	}
}

func (r *Runner) goneFailure(client *pirpc.Client) error {
	command := r.commandInFlight()
	tail := strings.TrimSpace(client.StderrTail())
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

func (r *Runner) step(phase, text string) error {
	r.mu.Lock()
	progress := r.progress
	r.mu.Unlock()
	if progress == nil {
		return nil
	}
	return progress(genui.GenerateProgress{Phase: phase, Text: text})
}

func (r *Runner) settle() {
	r.mu.Lock()
	settled := r.settled
	if settled == nil || r.settledDone {
		r.mu.Unlock()
		return
	}
	r.settledDone = true
	r.mu.Unlock()
	close(settled)
}

func (r *Runner) settledCh() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settled
}

func (r *Runner) guardStatus() <-chan string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.guard
}

func (r *Runner) pirpcClient() *pirpc.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.client
}

func (r *Runner) turnReply() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reply
}

func (r *Runner) modelStamp() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.model
}

func (r *Runner) turnFailure() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failure
}

func (r *Runner) commandInFlight() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastCommand
}

// handleEvent turns Pi's session events into what the reader sees. Pi nests the
// streaming deltas in message_update.assistantMessageEvent and reports tool runs
// as top-level tool_execution records.
func (r *Runner) handleEvent(eventType string, raw json.RawMessage) {
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
			_ = r.step("thinking", "Thinking")
		case "error":
			if message := strings.TrimSpace(update.Event.Error); message != "" {
				r.fail(genui.NewFailure(genui.FailAgentMessage, "Pi's model stopped: "+message, genui.HintAgentMessage))
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
		r.mu.Lock()
		r.reply = assistantText(ended.Message.Content)
		r.mu.Unlock()
		switch ended.Message.StopReason {
		case "error":
			message := strings.TrimSpace(ended.Message.ErrorMessage)
			if message == "" {
				message = "Pi's model stopped before answering."
			}
			r.fail(genui.NewFailure(genui.FailAgentMessage, message, genui.HintAgentMessage))
		case "aborted":
			r.fail(genui.ErrCancelled)
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
			_ = r.step("reading", "Reading its files")
		case "write", "edit":
			_ = r.step("writing", "Writing the view")
		}

	case "agent_settled":
		r.settle()
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

func (r *Runner) fail(err error) {
	r.mu.Lock()
	if r.failure == nil {
		r.failure = err
	}
	r.mu.Unlock()
}

// handleUIRequest answers Pi's extension UI records. The guard's own status
// report is how this app learns the guard is active; a dialog this app cannot
// show is declined and the run is aborted; notifications are dropped, because a
// generation has no window to show them in.
func (r *Runner) handleUIRequest(raw json.RawMessage) {
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
		client := r.pirpcClient()
		if client == nil {
			return
		}
		if request.ID != "" {
			_ = client.SendUIResponse(request.ID, map[string]any{"cancelled": true})
		}
		_ = client.Send("abort", nil)
		r.settle()
	case "setStatus":
		if request.StatusKey != guardStatusKey {
			return
		}
		r.mu.Lock()
		guard := r.guard
		r.mu.Unlock()
		if guard == nil {
			return
		}
		select {
		case guard <- request.StatusText:
		default:
		}
	}
}
