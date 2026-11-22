package acp

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

type traceRecorder struct {
	mu     sync.Mutex
	events []traceRecord
}

type traceRecord struct {
	event   string
	payload map[string]any
}

func (tr *traceRecorder) observe(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return
	}
	tr.mu.Lock()
	tr.events = append(tr.events, traceRecord{event: event, payload: m})
	tr.mu.Unlock()
}

func (tr *traceRecorder) all(event string) []map[string]any {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var out []map[string]any
	for _, e := range tr.events {
		if e.event == event {
			out = append(out, e.payload)
		}
	}
	return out
}

func (tr *traceRecorder) last(t *testing.T, event string) map[string]any {
	t.Helper()
	events := tr.all(event)
	if len(events) == 0 {
		t.Fatalf("no %s event; events: %v", event, tr.names())
	}
	return events[len(events)-1]
}

func (tr *traceRecorder) names() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	out := make([]string, len(tr.events))
	for i, e := range tr.events {
		out[i] = e.event
	}
	return out
}

func (tr *traceRecorder) finalEvent(t *testing.T) string {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.events) == 0 {
		t.Fatal("the observer received no events")
	}
	return tr.events[len(tr.events)-1].event
}

func traceText(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func traceBool(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}

func findTraceConfigSet(t *testing.T, tr *traceRecorder, configID string) map[string]any {
	t.Helper()
	var found map[string]any
	for _, e := range tr.all(TraceConfigSet) {
		if traceText(e, "configId") == configID {
			found = e
		}
	}
	if found == nil {
		t.Fatalf("no config.set for %s; events: %v", configID, tr.names())
	}
	return found
}

func TestGenerationTraceProvesSessionConfigAndCommittedView(t *testing.T) {
	tr := &traceRecorder{}
	spec := fakeSpec{
		Messages:       []string{"The view is ready."},
		Out:            minimalOut,
		ThoughtValues:  []string{"low", "high"},
		ThoughtCurrent: "high",
		ConfigUpdate:   true,
	}
	err, job, log := driveGeneration(t, spec, nil, func(job *GenerationJob) {
		job.Observe = tr.observe
		job.ThoughtLevel = "low"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	if got := tr.finalEvent(t); got != TraceRunEnd {
		t.Fatalf("last event = %s, want %s; events: %v", got, TraceRunEnd, tr.names())
	}
	if got := len(tr.all(TraceRunStart)); got != 1 {
		t.Fatalf("run.start events = %d, want 1", got)
	}
	if got := len(tr.all(TraceRunEnd)); got != 1 {
		t.Fatalf("run.end events = %d, want 1", got)
	}

	start := tr.last(t, TraceAgentStart)
	fakePid := int(logEntries(log, "pid")[0]["pid"].(float64))
	fakeCwd := logEntries(log, "cwd")[0]["cwd"].(string)
	if got := int(start["pid"].(float64)); got != fakePid {
		t.Errorf("agent.start pid = %d, want the fake's pid %d", got, fakePid)
	}
	if got := traceText(start, "path"); got != os.Args[0] {
		t.Errorf("agent.start path = %q, want %q", got, os.Args[0])
	}
	args, _ := start["args"].([]any)
	if len(args) < 2 || args[1] != "acp" {
		t.Errorf("agent.start args = %v, want the executable and \"acp\"", args)
	}
	if got := traceText(start, "cwd"); got != fakeCwd {
		t.Errorf("agent.start cwd = %q, want the fake's %q", got, fakeCwd)
	}

	init := tr.last(t, TraceInitialize)
	clientInfo, _ := init["clientInfo"].(map[string]any)
	if traceText(clientInfo, "name") != ClientName {
		t.Errorf("clientInfo = %v, want name %q", init["clientInfo"], ClientName)
	}
	if init["requestProtocolVersion"].(float64) != ProtocolVersion || init["agentProtocolVersion"].(float64) != ProtocolVersion {
		t.Errorf("protocol versions = %v/%v, want %d", init["requestProtocolVersion"], init["agentProtocolVersion"], ProtocolVersion)
	}
	result, _ := init["result"].(map[string]any)
	if _, ok := result["agentCapabilities"]; !ok {
		t.Errorf("initialize result = %v, want the full raw answer", init["result"])
	}

	session := tr.last(t, TraceSessionNew)
	if got := traceText(session, "sessionId"); got != "fake-session" {
		t.Errorf("session.new sessionId = %q, want fake-session", got)
	}
	sessionResult, _ := session["result"].(map[string]any)
	if _, ok := sessionResult["configOptions"]; !ok {
		t.Errorf("session.new result = %v, want configOptions", session["result"])
	}
	options := tr.last(t, TraceConfigOptions)
	modelOptions, _ := options["model"].(map[string]any)
	if got := traceText(modelOptions, "currentValue"); got != "gemini-4-argon" {
		t.Errorf("session/new model currentValue = %q, want gemini-4-argon", got)
	}
	thoughtOptions, _ := options["thoughtLevel"].(map[string]any)
	if got := traceText(thoughtOptions, "currentValue"); got != "high" {
		t.Errorf("session/new thought_level currentValue = %q, want high", got)
	}
	if values, _ := thoughtOptions["values"].([]any); len(values) != 2 || values[0] != "low" {
		t.Errorf("thought_level values = %v, want low and high", values)
	}

	modelSet := findTraceConfigSet(t, tr, traceModelOption)
	if !traceBool(modelSet, "confirmed") || traceText(modelSet, "currentValue") != "gemini-3.8-flash" {
		t.Errorf("model config.set = %v, want confirmed gemini-3.8-flash", modelSet)
	}
	thoughtSet := findTraceConfigSet(t, tr, traceThoughtID)
	if !traceBool(thoughtSet, "confirmed") || traceText(thoughtSet, "currentValue") != "low" {
		t.Errorf("thought_level config.set = %v, want confirmed low", thoughtSet)
	}
	if got := len(tr.all(TraceConfigUpdate)); got != 2 {
		t.Errorf("config.option_update events = %d, want 2 (model and thought_level)", got)
	}

	sentInitialize := false
	sentSessionNew := false
	sentPrompt := false
	for _, e := range tr.all(TraceACPSent) {
		line, _ := e["line"].(map[string]any)
		params, _ := line["params"].(map[string]any)
		switch line["method"] {
		case "initialize":
			sentInitialize = true
			if _, ok := params["clientInfo"]; !ok {
				t.Errorf("initialize line has no clientInfo: %v", line)
			}
		case "session/new":
			sentSessionNew = true
		case "session/set_config_option":
			if got := traceText(params, "sessionId"); got != "fake-session" {
				t.Errorf("set_config_option sessionId = %q, want fake-session", got)
			}
		case "session/prompt":
			sentPrompt = true
			if got := traceText(params, "sessionId"); got != "fake-session" {
				t.Errorf("prompt sessionId = %q, want fake-session", got)
			}
		}
	}
	if !sentInitialize || !sentSessionNew || !sentPrompt {
		t.Errorf("sent initialize=%v session/new=%v prompt=%v; want all three", sentInitialize, sentSessionNew, sentPrompt)
	}
	if got := len(tr.all(TraceACPReceived)); got == 0 {
		t.Error("no received ACP line was recorded")
	}
	if got := tr.last(t, TracePrompt); !strings.Contains(traceText(got, "text"), "<format>") || traceText(got, "sessionId") != "fake-session" {
		t.Errorf("prompt payload = %v, want the full prompt of fake-session", got)
	}
	turned := tr.last(t, TracePromptResult)
	if got := traceText(turned, "stopReason"); got != "end_turn" {
		t.Errorf("prompt.result stopReason = %q, want end_turn", got)
	}
	if _, ok := turned["usage"]; !ok {
		t.Errorf("prompt.result = %v, want usage", turned)
	}
	if len(tr.all(TracePhase)) == 0 {
		t.Error("no phase event was recorded")
	}

	artifacts := map[string]map[string]any{}
	for _, e := range tr.all(TraceArtifact) {
		artifacts[traceText(e, "name")] = e
	}
	source, hasSource := artifacts["source.md"]
	if !hasSource || source["content"] != loadVectors(t).Document {
		t.Errorf("source.md artifact = %v, want the captured source", source)
	}
	out, hasOut := artifacts["out.json"]
	if !hasOut || out["content"] != minimalOut {
		t.Errorf("out.json artifact = %v, want the agent's exact output", out)
	}
	viewBytes, err := os.ReadFile(job.ViewPath)
	if err != nil {
		t.Fatalf("read committed view: %v", err)
	}
	view, hasView := artifacts["view"]
	if !hasView || view["content"] != string(viewBytes) {
		t.Errorf("view artifact = %v, want the committed file %q", view, viewBytes)
	}
	if !hasView || view["path"] != job.ViewPath {
		t.Errorf("view artifact path = %v, want %s", view["path"], job.ViewPath)
	}

	end := tr.last(t, TraceRunEnd)
	if !traceBool(end, "ok") || end["error"] != nil || end["failure"] != nil {
		t.Errorf("run.end = %v, want success without a failure", end)
	}
	if !traceBool(end, "viewWritten") || traceText(end, "sessionId") != "fake-session" {
		t.Errorf("run.end = %v, want the written view of fake-session", end)
	}
	if traceText(end, "actualModel") != "gemini-3.8-flash" || !traceBool(end, "actualModelConfirmed") || !traceBool(end, "modelApplied") {
		t.Errorf("run.end model evidence = %v, want confirmed gemini-3.8-flash", end)
	}
	if traceText(end, "actualThoughtLevel") != "low" || !traceBool(end, "thoughtLevelConfirmed") || !traceBool(end, "thoughtLevelApplied") {
		t.Errorf("run.end thought evidence = %v, want confirmed low", end)
	}
}

func TestGenerationTraceFailureIsNeverSuccess(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		tr := &traceRecorder{}
		err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
			job.Observe = tr.observe
			job.Req.Mode = "no-such-mode"
		})
		if err == nil {
			t.Fatal("RunGeneration succeeded, want the mode failure")
		}
		if got := tr.finalEvent(t); got != TraceRunEnd {
			t.Fatalf("last event = %s, want %s", got, TraceRunEnd)
		}
		if got := len(tr.all(TraceRunEnd)); got != 1 {
			t.Fatalf("run.end events = %d, want 1", got)
		}
		if len(tr.all(TraceAgentStart)) != 0 || len(tr.all(TraceSessionNew)) != 0 {
			t.Errorf("events = %v, want no agent or session for invalid input", tr.names())
		}
		end := tr.last(t, TraceRunEnd)
		if traceBool(end, "ok") || !strings.Contains(traceText(end, "error"), "Unknown generation mode") || traceBool(end, "viewWritten") {
			t.Errorf("run.end = %v, want the terminal input failure", end)
		}
	})

	t.Run("auth error", func(t *testing.T) {
		tr := &traceRecorder{}
		spec := fakeSpec{Errors: map[string]fakeRPCError{
			"session/new": {Code: -32000, Message: "Authentication required"},
		}}
		err, _, _ := driveGeneration(t, spec, nil, func(job *GenerationJob) { job.Observe = tr.observe })
		if err == nil {
			t.Fatal("RunGeneration succeeded, want the auth failure")
		}
		end := tr.last(t, TraceRunEnd)
		if traceBool(end, "ok") || traceText(end, "error") == "" || traceBool(end, "viewWritten") {
			t.Errorf("run.end = %v, want a failure without a written view", end)
		}
		failure, _ := end["failure"].(map[string]any)
		if traceText(failure, "kind") != "auth-required" {
			t.Errorf("run.end failure = %v, want kind auth-required", end["failure"])
		}
		if artifacts := tr.all(TraceArtifact); len(artifacts) != 1 || traceText(artifacts[0], "name") != "source.md" {
			t.Errorf("artifacts = %v, want only source.md for a run with no out.json", artifacts)
		}
	})

	t.Run("model refused", func(t *testing.T) {
		tr := &traceRecorder{}
		spec := fakeSpec{Out: minimalOut, Errors: map[string]fakeRPCError{
			"session/set_config_option": {Code: -32602, Message: "Unknown model ID: gemini-3.8-flash"},
		}}
		err, _, _ := driveGeneration(t, spec, nil, func(job *GenerationJob) { job.Observe = tr.observe })
		if err == nil {
			t.Fatal("RunGeneration succeeded, want the model failure")
		}
		set := findTraceConfigSet(t, tr, traceModelOption)
		if traceBool(set, "confirmed") || traceText(set, "error") == "" {
			t.Errorf("config.set = %v, want an unconfirmed error", set)
		}
		end := tr.last(t, TraceRunEnd)
		if traceBool(end, "ok") || traceText(end, "actualModel") != "" || traceBool(end, "actualModelConfirmed") || traceBool(end, "modelApplied") {
			t.Errorf("run.end = %v, want no confirmed actual model", end)
		}
		failure, _ := end["failure"].(map[string]any)
		if traceText(failure, "kind") != "model-invalid" {
			t.Errorf("run.end failure = %v, want kind model-invalid", end["failure"])
		}
	})

	t.Run("thought level not offered", func(t *testing.T) {
		tr := &traceRecorder{}
		spec := fakeSpec{Out: minimalOut}
		err, _, log := driveGeneration(t, spec, nil, func(job *GenerationJob) {
			job.Observe = tr.observe
			job.ThoughtLevel = "low"
		})
		if err != nil {
			t.Fatalf("RunGeneration: %v", err)
		}
		set := findTraceConfigSet(t, tr, traceThoughtID)
		if !traceBool(set, "skipped") || !strings.Contains(traceText(set, "reason"), "does not offer") {
			t.Errorf("config.set = %v, want a skipped thought level with the offered values", set)
		}
		if sets := logRecv(log, "session/set_config_option"); len(sets) != 1 || sets[0]["configId"] != traceModelOption {
			t.Errorf("set_config_option = %v, want only the model set", sets)
		}
		end := tr.last(t, TraceRunEnd)
		if !traceBool(end, "ok") || traceBool(end, "thoughtLevelApplied") || traceBool(end, "thoughtLevelConfirmed") || !traceBool(end, "viewWritten") {
			t.Errorf("run.end = %v, want a successful run with thought_level honestly unconfirmed", end)
		}
		if traceText(end, "actualThoughtLevel") != "" || traceText(end, "thoughtLevelNote") == "" {
			t.Errorf("run.end = %v, want no forged actual thought level and a note", end)
		}
	})
}

func TestTheValidationResultListsThePropertiesTheReaderDrawsWithTheirDefaults(t *testing.T) {
	const oversized = `field "/size" expects one of ["xs", "sm", "md", "lg"] but got "xl"`
	for name, test := range map[string]struct {
		statements []string
		want       []any
	}{
		"a text size the catalog does not allow": {
			[]string{`root = Col([latency])`, `latency = Source(["report"], [Text("23 ms", "xl")])`},
			[]any{map[string]any{"statement": "latency", "component": "Text", "property": "size", "message": oversized}},
		},
		"nothing degraded": {
			[]string{`root = Col([latency])`, `latency = Source(["report"], [Text("23 ms", "lg")])`},
			nil,
		},
	} {
		tr := &traceRecorder{}
		err, _, _ := driveGeneration(t, fakeSpec{Out: statementsFull(test.statements...)}, nil, func(job *GenerationJob) {
			job.Req.Language = genui.StatementLanguage
			job.Validate = nil
			job.Observe = tr.observe
		})
		if err != nil {
			t.Fatalf("%s: RunGeneration: %v", name, err)
		}
		result := tr.last(t, "validation.result")
		if !traceBool(result, "ok") {
			t.Errorf("%s: validation.result = %v, want an accepted view", name, result)
		}
		degraded, listed := result["degraded"]
		if got, _ := degraded.([]any); !reflect.DeepEqual(got, test.want) || listed != (test.want != nil) {
			t.Errorf("%s: validation.result degraded = %v (listed %t), want %v", name, degraded, listed, test.want)
		}
	}
}
