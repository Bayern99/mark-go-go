package acp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

const (
	TraceRunStart      = "run.start"
	TraceAgentStart    = "agent.start"
	TraceAgentReused   = "agent.reused"
	TraceACPSent       = "acp.sent"
	TraceACPReceived   = "acp.received"
	TraceInitialize    = "initialize"
	TraceInitCached    = "initialize.cached"
	TraceSessionNew    = "session.new"
	TraceConfigOptions = "config.options"
	TraceConfigSet     = "config.set"
	TraceConfigUpdate  = "config.option_update"
	TracePrompt        = "prompt"
	TracePromptResult  = "prompt.result"
	TracePhase         = "phase"
	TraceArtifact      = "artifact"
	TraceRunEnd        = "run.end"
)

const (
	traceModelOption = "model"
	traceThoughtID   = "thought_level"
)

type traceRunStartPayload struct {
	Dir                   string             `json:"dir"`
	ViewPath              string             `json:"viewPath"`
	Mode                  string             `json:"mode"`
	ModelRequested        string             `json:"modelRequested,omitempty"`
	ThoughtLevelRequested string             `json:"thoughtLevelRequested,omitempty"`
	AgentCommand          string             `json:"agentCommand"`
	Request               string             `json:"request,omitempty"`
	Source                string             `json:"source"`
	Format                string             `json:"format"`
	ReplaceIDs            []string           `json:"replaceIds,omitempty"`
	Sections              []genui.SectionRef `json:"sections,omitempty"`
}

type traceAgentStartPayload struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	PID  int      `json:"pid"`
	Cwd  string   `json:"cwd"`
}

type traceAgentReusePayload struct {
	Path      string   `json:"path"`
	Args      []string `json:"args"`
	PID       int      `json:"pid"`
	Cwd       string   `json:"cwd"`
	StartedAt string   `json:"startedAt"`
	IdleMS    int64    `json:"idleMs"`
}

type traceACPPayload struct {
	Direction string          `json:"direction"`
	Line      json.RawMessage `json:"line"`
}

type traceInitializePayload struct {
	ClientInfo        map[string]string `json:"clientInfo"`
	RequestProtocol   int               `json:"requestProtocolVersion"`
	AgentProtocol     int               `json:"agentProtocolVersion"`
	AgentInfo         json.RawMessage   `json:"agentInfo,omitempty"`
	AgentCapabilities json.RawMessage   `json:"agentCapabilities,omitempty"`
	AuthMethods       json.RawMessage   `json:"authMethods,omitempty"`
	Result            json.RawMessage   `json:"result"`
}

type traceInitializeCachedPayload struct {
	ClientInfo        map[string]string `json:"clientInfo"`
	RequestProtocol   int               `json:"requestProtocolVersion"`
	AgentProtocol     int               `json:"agentProtocolVersion"`
	AgentInfo         json.RawMessage   `json:"agentInfo,omitempty"`
	AgentCapabilities json.RawMessage   `json:"agentCapabilities,omitempty"`
	AuthMethods       json.RawMessage   `json:"authMethods,omitempty"`
	Result            json.RawMessage   `json:"result"`
	ProcessPID        int               `json:"processPid"`
	ProcessStartedAt  string            `json:"processStartedAt"`
	ProcessCwd        string            `json:"processCwd"`
}

type traceSessionNewPayload struct {
	SessionID string          `json:"sessionId"`
	Result    json.RawMessage `json:"result"`
}

type traceOptionValue struct {
	ID           string          `json:"id"`
	CurrentValue json.RawMessage `json:"currentValue,omitempty"`
	Values       []string        `json:"values,omitempty"`
}

type traceConfigOptionsPayload struct {
	Source        string            `json:"source"`
	ConfigOptions json.RawMessage   `json:"configOptions,omitempty"`
	Model         *traceOptionValue `json:"model,omitempty"`
	ThoughtLevel  *traceOptionValue `json:"thoughtLevel,omitempty"`
}

type traceConfigSetPayload struct {
	ConfigID     string          `json:"configId"`
	Value        string          `json:"value"`
	Skipped      bool            `json:"skipped,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Error        string          `json:"error,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	CurrentValue json.RawMessage `json:"currentValue,omitempty"`
	Confirmed    bool            `json:"confirmed"`
	Note         string          `json:"note,omitempty"`
}

type traceConfigUpdatePayload struct {
	Source        string            `json:"source"`
	Method        string            `json:"method"`
	Params        json.RawMessage   `json:"params"`
	ConfigOptions json.RawMessage   `json:"configOptions,omitempty"`
	Model         *traceOptionValue `json:"model,omitempty"`
	ThoughtLevel  *traceOptionValue `json:"thoughtLevel,omitempty"`
}

type tracePromptPayload struct {
	SessionID string `json:"sessionId"`
	Text      string `json:"text"`
}

type tracePromptResultPayload struct {
	StopReason string          `json:"stopReason,omitempty"`
	Usage      json.RawMessage `json:"usage,omitempty"`
	Result     json.RawMessage `json:"result"`
}

type tracePhasePayload struct {
	Phase string `json:"phase"`
	Text  string `json:"text,omitempty"`
}

type traceArtifactPayload struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Content string `json:"content"`
}

type traceRunEndPayload struct {
	OK                    bool                   `json:"ok"`
	Error                 string                 `json:"error,omitempty"`
	Failure               *genui.GenerateFailure `json:"failure,omitempty"`
	SessionID             string                 `json:"sessionId,omitempty"`
	ViewPath              string                 `json:"viewPath"`
	ViewWritten           bool                   `json:"viewWritten"`
	ModelRequested        string                 `json:"modelRequested,omitempty"`
	ActualModel           json.RawMessage        `json:"actualModel,omitempty"`
	ActualModelConfirmed  bool                   `json:"actualModelConfirmed"`
	ModelApplied          bool                   `json:"modelApplied"`
	ActualModelSource     string                 `json:"actualModelSource,omitempty"`
	ModelNote             string                 `json:"modelNote,omitempty"`
	ThoughtLevelRequested string                 `json:"thoughtLevelRequested,omitempty"`
	ActualThoughtLevel    json.RawMessage        `json:"actualThoughtLevel,omitempty"`
	ThoughtLevelConfirmed bool                   `json:"thoughtLevelConfirmed"`
	ThoughtLevelApplied   bool                   `json:"thoughtLevelApplied"`
	ThoughtLevelSource    string                 `json:"thoughtLevelSource,omitempty"`
	ThoughtLevelNote      string                 `json:"thoughtLevelNote,omitempty"`
}

type traceOptionState struct {
	actual    json.RawMessage
	source    string
	confirmed bool
	applied   bool
	note      string
}

type generationTrace struct {
	observe          func(event string, payload any)
	viewPath         string
	modelRequested   string
	thoughtRequested string

	mu            sync.Mutex
	sessionID     string
	viewCommitted bool
	ended         bool
	model         traceOptionState
	thought       traceOptionState
}

func newGenerationTrace(job *GenerationJob) *generationTrace {
	if job.Observe == nil {
		return nil
	}
	return &generationTrace{
		observe:          job.Observe,
		viewPath:         job.ViewPath,
		modelRequested:   job.Model,
		thoughtRequested: job.ThoughtLevel,
	}
}

func (t *generationTrace) emit(event string, payload any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ended {
		return
	}
	t.observe(event, payload)
}

func (t *generationTrace) stateFor(configID string) *traceOptionState {
	if t == nil {
		return nil
	}
	switch configID {
	case traceModelOption:
		return &t.model
	case traceThoughtID, "thoughtLevel":
		return &t.thought
	}
	return nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func configString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, true
	}
	return string(raw), true
}

type acpConfigOption struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Category     string          `json:"category"`
	CurrentValue json.RawMessage `json:"currentValue"`
	Options      []struct {
		Value json.RawMessage `json:"value"`
		Name  string          `json:"name"`
	} `json:"options"`
}

func (o acpConfigOption) current() (json.RawMessage, bool) {
	if len(o.CurrentValue) == 0 || string(o.CurrentValue) == "null" {
		return nil, false
	}
	return o.CurrentValue, true
}

func (o acpConfigOption) values() []string {
	out := make([]string, 0, len(o.Options))
	for _, v := range o.Options {
		if s, ok := configString(v.Value); ok {
			out = append(out, s)
		}
	}
	return out
}

func (o acpConfigOption) valueOf() *traceOptionValue {
	current, hasCurrent := o.current()
	if !hasCurrent && len(o.Options) == 0 {
		return nil
	}
	v := &traceOptionValue{ID: o.ID, Values: o.values()}
	if hasCurrent {
		v.CurrentValue = cloneRaw(current)
	}
	return v
}

func findConfigOption(raw json.RawMessage, ids ...string) (acpConfigOption, bool) {
	if len(raw) == 0 {
		return acpConfigOption{}, false
	}
	var opts []acpConfigOption
	if json.Unmarshal(raw, &opts) != nil {
		return acpConfigOption{}, false
	}
	for _, id := range ids {
		for _, o := range opts {
			if o.ID == id {
				return o, true
			}
		}
	}
	return acpConfigOption{}, false
}

func configOptionList(raw json.RawMessage) []acpConfigOption {
	if len(raw) == 0 {
		return nil
	}
	var opts []acpConfigOption
	if json.Unmarshal(raw, &opts) != nil {
		return nil
	}
	return opts
}

func findConfigOptionIn(raw json.RawMessage, ids, categories []string) (acpConfigOption, bool) {
	opts := configOptionList(raw)
	for _, id := range ids {
		for _, o := range opts {
			if o.ID == id {
				return o, true
			}
		}
	}
	for _, category := range categories {
		for _, o := range opts {
			if o.Category == category {
				return o, true
			}
		}
	}
	return acpConfigOption{}, false
}

func configOptionsField(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var top struct {
		ConfigOptions json.RawMessage `json:"configOptions"`
	}
	if json.Unmarshal(raw, &top) != nil {
		return nil
	}
	return top.ConfigOptions
}

func (t *generationTrace) runStart(job *GenerationJob, dir string) {
	if t == nil {
		return
	}
	t.emit(TraceRunStart, &traceRunStartPayload{
		Dir:                   dir,
		ViewPath:              job.ViewPath,
		Mode:                  job.Req.Mode,
		ModelRequested:        job.Model,
		ThoughtLevelRequested: job.ThoughtLevel,
		AgentCommand:          job.AgentCommand,
		Request:               job.Req.Request,
		Source:                job.Req.Text,
		Format:                job.Req.Format,
		ReplaceIDs:            job.Req.ReplaceIDs,
		Sections:              job.Req.Sections,
	})
}

func (t *generationTrace) agentStart(path string, cmdArgs []string, pid int, cwd string) {
	if t == nil {
		return
	}
	t.emit(TraceAgentStart, &traceAgentStartPayload{Path: path, Args: cmdArgs, PID: pid, Cwd: cwd})
}

func (t *generationTrace) agentReused(path string, cmdArgs []string, pid int, cwd string, startedAt, idleSince time.Time) {
	if t == nil {
		return
	}
	payload := &traceAgentReusePayload{
		Path:      path,
		Args:      cmdArgs,
		PID:       pid,
		Cwd:       cwd,
		StartedAt: startedAt.UTC().Format(time.RFC3339Nano),
	}
	if !idleSince.IsZero() {
		payload.IdleMS = time.Since(idleSince).Milliseconds()
	}
	t.emit(TraceAgentReused, payload)
}

func (t *generationTrace) initialized(result json.RawMessage, clientInfo map[string]string, requestVersion, agentVersion int, agentInfo, capabilities, authMethods json.RawMessage) {
	if t == nil {
		return
	}
	t.emit(TraceInitialize, &traceInitializePayload{
		ClientInfo:        clientInfo,
		RequestProtocol:   requestVersion,
		AgentProtocol:     agentVersion,
		AgentInfo:         agentInfo,
		AgentCapabilities: capabilities,
		AuthMethods:       authMethods,
		Result:            result,
	})
}

func (t *generationTrace) initializeCached(clientInfo map[string]string, requestVersion, agentVersion int, agentInfo, capabilities, authMethods, result json.RawMessage, pid int, startedAt time.Time, cwd string) {
	if t == nil {
		return
	}
	t.emit(TraceInitCached, &traceInitializeCachedPayload{
		ClientInfo:        clientInfo,
		RequestProtocol:   requestVersion,
		AgentProtocol:     agentVersion,
		AgentInfo:         agentInfo,
		AgentCapabilities: capabilities,
		AuthMethods:       authMethods,
		Result:            result,
		ProcessPID:        pid,
		ProcessStartedAt:  startedAt.UTC().Format(time.RFC3339Nano),
		ProcessCwd:        cwd,
	})
}

func (t *generationTrace) acceptsInbound(line []byte) bool {
	if t == nil {
		return true
	}
	var msg struct {
		Method string `json:"method"`
		Params struct {
			SessionID string `json:"sessionId"`
		} `json:"params"`
	}
	if json.Unmarshal(line, &msg) != nil || msg.Params.SessionID == "" {
		return true
	}
	t.mu.Lock()
	current := t.sessionID
	t.mu.Unlock()
	return current != "" && msg.Params.SessionID == current
}

func (t *generationTrace) sessionNew(result json.RawMessage, sessionID string) {
	if t == nil {
		return
	}
	opts := configOptionsField(result)
	modelOpt, hasModel := findConfigOption(opts, traceModelOption)
	thoughtOpt, hasThought := findConfigOption(opts, traceThoughtID, "thoughtLevel")

	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = sessionID

	if hasModel {
		if t.modelRequested == "" {
			if current, ok := modelOpt.current(); ok {
				t.model.actual = cloneRaw(current)
				t.model.source = "session/new"
				t.model.confirmed = true
				t.model.applied = true
			} else {
				t.model.note = "the session/new model option carried no currentValue"
			}
		} else {
			t.model.note = "the session/new model currentValue is the agent's default, before this run's set_config_option"
		}
	} else {
		t.model.note = "the session/new configOptions have no model option"
	}

	if hasThought {
		if t.thoughtRequested == "" {
			if current, ok := thoughtOpt.current(); ok {
				t.thought.actual = cloneRaw(current)
				t.thought.source = "session/new"
				t.thought.confirmed = true
				t.thought.applied = true
			} else {
				t.thought.note = "the session/new thought_level option carried no currentValue"
			}
		} else {
			t.thought.note = "the session/new thought_level currentValue is the agent's default, before this run's set_config_option"
		}
	} else if t.thoughtRequested == "" {
		t.thought.note = "the session/new configOptions have no thought_level option"
	}

	payload := &traceConfigOptionsPayload{Source: "session/new", ConfigOptions: opts}
	if hasModel {
		payload.Model = modelOpt.valueOf()
	}
	if hasThought {
		payload.ThoughtLevel = thoughtOpt.valueOf()
	}
	t.observe(TraceSessionNew, &traceSessionNewPayload{SessionID: sessionID, Result: result})
	t.observe(TraceConfigOptions, payload)
}

func (t *generationTrace) configSetSkipped(configID, value, reason string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if state := t.stateFor(configID); state != nil {
		state.note = reason
	}
	t.observe(TraceConfigSet, &traceConfigSetPayload{
		ConfigID: configID,
		Value:    value,
		Skipped:  true,
		Reason:   reason,
		Note:     reason,
	})
}

func (t *generationTrace) configSetResponse(configID, value string, result json.RawMessage) {
	if t == nil {
		return
	}
	opts := configOptionsField(result)
	opt, hasOption := findConfigOption(opts, configID)

	t.mu.Lock()
	defer t.mu.Unlock()
	payload := &traceConfigSetPayload{ConfigID: configID, Value: value, Result: result}
	state := t.stateFor(configID)
	if !hasOption {
		payload.Note = "the set_config_option answer carried no configOptions for " + configID
		if state != nil && !state.confirmed {
			state.note = payload.Note
		}
	} else if current, ok := opt.current(); !ok {
		payload.Note = "the set_config_option answer carried no currentValue for " + configID
		if state != nil && !state.confirmed {
			state.note = payload.Note
		}
	} else {
		payload.CurrentValue = cloneRaw(current)
		actual, hasActual := configString(current)
		payload.Confirmed = hasActual && actual == value
		if state != nil {
			state.actual = cloneRaw(current)
			state.source = "session/set_config_option"
			state.confirmed = true
			state.applied = payload.Confirmed
			if payload.Confirmed {
				state.note = ""
			} else {
				state.note = fmt.Sprintf("the agent reports %s, not the requested %s", string(current), value)
				payload.Note = state.note
			}
		}
	}
	t.observe(TraceConfigSet, payload)
}

func (t *generationTrace) configSetError(configID, value string, err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if state := t.stateFor(configID); state != nil {
		state.note = err.Error()
	}
	t.observe(TraceConfigSet, &traceConfigSetPayload{
		ConfigID: configID,
		Value:    value,
		Error:    err.Error(),
		Note:     err.Error(),
	})
}

func (t *generationTrace) configOptionUpdate(method string, params, opts json.RawMessage) {
	if t == nil {
		return
	}
	modelOpt, hasModel := findConfigOption(opts, traceModelOption)
	thoughtOpt, hasThought := findConfigOption(opts, traceThoughtID, "thoughtLevel")

	t.mu.Lock()
	defer t.mu.Unlock()
	payload := &traceConfigUpdatePayload{
		Source:        "config.option_update",
		Method:        method,
		Params:        params,
		ConfigOptions: opts,
	}
	if hasModel {
		payload.Model = modelOpt.valueOf()
		if current, ok := modelOpt.current(); ok {
			t.model.actual = cloneRaw(current)
			t.model.source = "config.option_update"
			t.model.confirmed = true
			if t.modelRequested == "" {
				t.model.applied = true
				t.model.note = ""
			} else {
				actual, _ := configString(current)
				t.model.applied = actual == t.modelRequested
				if t.model.applied {
					t.model.note = ""
				} else {
					t.model.note = fmt.Sprintf("the agent reports %s, not the requested %s", string(current), t.modelRequested)
				}
			}
		}
	}
	if hasThought {
		payload.ThoughtLevel = thoughtOpt.valueOf()
		if current, ok := thoughtOpt.current(); ok {
			t.thought.actual = cloneRaw(current)
			t.thought.source = "config.option_update"
			t.thought.confirmed = true
			if t.thoughtRequested == "" {
				t.thought.applied = true
				t.thought.note = ""
			} else {
				actual, _ := configString(current)
				t.thought.applied = actual == t.thoughtRequested
				if t.thought.applied {
					t.thought.note = ""
				} else {
					t.thought.note = fmt.Sprintf("the agent reports %s, not the requested %s", string(current), t.thoughtRequested)
				}
			}
		}
	}
	t.observe(TraceConfigUpdate, payload)
}

func (t *generationTrace) prompt(text string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	sessionID := t.sessionID
	t.observe(TracePrompt, &tracePromptPayload{SessionID: sessionID, Text: text})
	t.mu.Unlock()
}

func (t *generationTrace) promptResult(result json.RawMessage) {
	if t == nil {
		return
	}
	var turned struct {
		StopReason string          `json:"stopReason"`
		Usage      json.RawMessage `json:"usage"`
	}
	_ = json.Unmarshal(result, &turned)
	t.emit(TracePromptResult, &tracePromptResultPayload{
		StopReason: turned.StopReason,
		Usage:      turned.Usage,
		Result:     result,
	})
}

func (t *generationTrace) phase(phase, text string) {
	if t == nil {
		return
	}
	t.emit(TracePhase, &tracePhasePayload{Phase: phase, Text: text})
}

// artifactOrView sends an orchestrator artifact to the trace, routing the
// committed view to the event that marks the run as having written one.
func (t *generationTrace) artifactOrView(name, path, content string) {
	if name == "view" {
		t.viewWritten(path, []byte(content))
		return
	}
	t.artifact(name, path, content)
}

// phases reports progress steps to the trace's phase event before handing them
// to the reader, so the trace keeps one phase entry per change.
func (t *generationTrace) phases(send func(genui.GenerateProgress) error) func(genui.GenerateProgress) error {
	var mu sync.Mutex
	phase, text := "", ""
	return func(p genui.GenerateProgress) error {
		mu.Lock()
		if p.Phase != phase || p.Text != text {
			phase, text = p.Phase, p.Text
			t.phase(p.Phase, p.Text)
		}
		mu.Unlock()
		return send(p)
	}
}

func (t *generationTrace) artifact(name, path, content string) {
	if t == nil {
		return
	}
	t.emit(TraceArtifact, &traceArtifactPayload{Name: name, Path: path, Bytes: len(content), Content: content})
}

func (t *generationTrace) viewWritten(path string, content []byte) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.viewCommitted = true
	t.observe(TraceArtifact, &traceArtifactPayload{Name: "view", Path: path, Bytes: len(content), Content: string(content)})
}

func (t *generationTrace) end(err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ended = true
	payload := &traceRunEndPayload{
		OK:                    err == nil,
		SessionID:             t.sessionID,
		ViewPath:              t.viewPath,
		ViewWritten:           t.viewCommitted,
		ModelRequested:        t.modelRequested,
		ActualModel:           t.model.actual,
		ActualModelConfirmed:  t.model.confirmed,
		ModelApplied:          t.model.applied,
		ActualModelSource:     t.model.source,
		ModelNote:             t.model.note,
		ThoughtLevelRequested: t.thoughtRequested,
		ActualThoughtLevel:    t.thought.actual,
		ThoughtLevelConfirmed: t.thought.confirmed,
		ThoughtLevelApplied:   t.thought.applied,
		ThoughtLevelSource:    t.thought.source,
		ThoughtLevelNote:      t.thought.note,
	}
	if err != nil {
		payload.Error = err.Error()
		failure := genui.FailureOf(err)
		payload.Failure = &failure
	}
	t.observe(TraceRunEnd, payload)
}
