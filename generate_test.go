package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
)

// The tests drive the generator through a fake ACP agent: the test
// binary re-runs itself (TestMain) when MARKGOGO_FAKE_AGENT is set. It
// then speaks JSON-RPC 2.0 over its stdio like agy acp does, logs
// every message it receives to the file MARKGOGO_FAKE_LOG names, and
// plays the scenario the JSON in MARKGOGO_FAKE_SPEC describes.
const (
	fakeAgentEnv = "MARKGOGO_FAKE_AGENT"
	fakeSpecEnv  = "MARKGOGO_FAKE_SPEC"
	fakeLogEnv   = "MARKGOGO_FAKE_LOG"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeAgentEnv) != "" {
		runFakeAgent()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeToolCallSpec is one tool_call update the fake emits; Update
// makes it a tool_call_update, whose fields the fake sends as given.
type fakeToolCallSpec struct {
	Update     bool     `json:"update"`
	ToolCallID string   `json:"toolCallId"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Locations  []string `json:"locations"`
}

// fakePermissionSpec is one permission request the fake sends.
type fakePermissionSpec struct {
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	Locations []string `json:"locations"`
	// Symlink, when set, is a symbolic link the fake creates in its
	// working directory before it asks, named after the first
	// location, pointing at it.
	Symlink string `json:"symlink"`
}

// fakeSpec is the scenario the fake agent plays for one prompt turn.
type fakeSpec struct {
	Thought     bool                 `json:"thought"`
	Messages    []string             `json:"messages"`
	ToolCalls   []fakeToolCallSpec   `json:"toolCalls"`
	Permissions []fakePermissionSpec `json:"permissions"`
	// Models are the model IDs the fake offers; empty offers the
	// default three.
	Models []string `json:"models"`
	// Out is written to out.json in the fake's working directory
	// before the turn ends; OutSize writes that many bytes instead,
	// and OutSymlink makes out.json a symbolic link to its target.
	Out        string `json:"out"`
	OutSize    int64  `json:"outSize"`
	OutSymlink string `json:"outSymlink"`
	// Hang never answers the prompt: the run can only end by
	// cancellation.
	Hang bool `json:"hang"`
	// Crash writes the text to stderr and exits 1 right away.
	Crash string `json:"crash"`
	// Errors answers the requests of these methods with these
	// JSON-RPC errors instead.
	Errors map[string]fakeRPCError `json:"errors"`
}

// fakeRPCError is a JSON-RPC error the fake answers with.
type fakeRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// runFakeAgent plays the agent until the client goes away.
func runFakeAgent() {
	var spec fakeSpec
	if s := os.Getenv(fakeSpecEnv); s != "" {
		if err := json.Unmarshal([]byte(s), &spec); err != nil {
			fmt.Fprintln(os.Stderr, "fake agent: bad spec:", err)
			os.Exit(2)
		}
	}
	logf, closeLog := fakeLogger()
	defer closeLog()

	cwd, _ := os.Getwd()
	logf(map[string]any{"type": "args", "args": os.Args})
	logf(map[string]any{"type": "cwd", "cwd": cwd})
	logf(map[string]any{"type": "pid", "pid": os.Getpid()})
	logf(map[string]any{"type": "files", "files": fakeDirList(cwd)})
	if spec.Crash != "" {
		fmt.Fprintln(os.Stderr, spec.Crash)
		os.Exit(1)
	}

	models := spec.Models
	if len(models) == 0 {
		models = []string{"gemini-4-argon", "gemini-3.8-flash", "claude-opus-5-5"}
	}
	const sessionID = "fake-session"
	nextID := int64(100)

	send := func(v any) {
		data, _ := json.Marshal(v)
		fmt.Fprintln(os.Stdout, string(data))
	}
	modelOptions := func() []map[string]string {
		opts := make([]map[string]string, len(models))
		for i, m := range models {
			opts[i] = map[string]string{"value": m, "name": m}
		}
		return opts
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for in.Scan() {
		line := bytes.TrimSpace(in.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		if msg.Method != "" {
			logf(map[string]any{"type": "recv", "method": msg.Method, "params": msg.Params})
		}
		if e, ok := spec.Errors[msg.Method]; ok && msg.ID != nil {
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "error": e})
			continue
		}
		switch {
		case msg.ID != nil && msg.Method == "initialize":
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": map[string]any{},
				"authMethods":       []any{},
			}})
		case msg.ID != nil && msg.Method == "session/new":
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{
				"sessionId": sessionID,
				"modes":     []any{},
				"models":    []any{},
				"configOptions": []map[string]any{
					{"id": "model", "type": "select", "currentValue": models[0], "options": modelOptions()},
					{"id": "thought_level", "type": "select", "currentValue": "high", "options": []map[string]string{{"value": "high", "name": "High"}}},
				},
			}})
		case msg.ID != nil && msg.Method == "session/set_config_option":
			var params struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			known := false
			for _, m := range models {
				if m == params.Value {
					known = true
				}
			}
			if params.ConfigID == "model" && known {
				send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{"configOptions": []any{}}})
			} else {
				send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "error": map[string]any{"code": -32602, "message": "Unknown model ID: " + params.Value}})
			}
		case msg.ID != nil && msg.Method == "session/prompt":
			if spec.Hang {
				// Keep running until the client cancels or closes.
				for in.Scan() {
					var m struct {
						Method string `json:"method"`
					}
					if json.Unmarshal(bytes.TrimSpace(in.Bytes()), &m) == nil && m.Method == "session/cancel" {
						logf(map[string]any{"type": "cancelled"})
						os.Exit(0)
					}
				}
				os.Exit(0)
			}
			fakePlayScenario(&spec, cwd, sessionID, &nextID, send, in, logf)
			logf(map[string]any{"type": "files", "files": fakeDirList(cwd)})
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{"stopReason": "end_turn", "usage": map[string]any{}}})
		case msg.Method == "session/cancel":
			logf(map[string]any{"type": "cancelled"})
			os.Exit(0)
		}
	}
}

// fakePlayScenario plays a prompt turn: a thought, the tool calls, the
// message chunks, the permission requests, and out.json.
func fakePlayScenario(spec *fakeSpec, cwd, sessionID string, nextID *int64, send func(any), in *bufio.Scanner, logf func(map[string]any)) {
	fakeToolSeq := 0
	if spec.Thought {
		send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
			"sessionId": sessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_thought_chunk",
				"content":       map[string]any{"type": "text", "text": "Considering the document."},
			},
		}})
	}
	for _, tc := range spec.ToolCalls {
		sessionUpdate := "tool_call"
		if tc.Update {
			sessionUpdate = "tool_call_update"
		}
		id := tc.ToolCallID
		if id == "" {
			id = fmt.Sprintf("tool-%d", fakeToolSeq)
			fakeToolSeq++
		}
		status := tc.Status
		if status == "" {
			status = "in_progress"
		}
		send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
			"sessionId": sessionID,
			"update": map[string]any{
				"sessionUpdate": sessionUpdate,
				"toolCallId":    id,
				"name":          tc.Name,
				"kind":          tc.Kind,
				"title":         tc.Title,
				"status":        status,
				"locations":     fakeLocations(cwd, tc.Locations),
			},
		}})
	}
	for _, m := range spec.Messages {
		send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
			"sessionId": sessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": m},
			},
		}})
	}
	for _, p := range spec.Permissions {
		if p.Symlink != "" && len(p.Locations) > 0 {
			_ = os.Symlink(p.Symlink, fakeResolve(cwd, p.Locations[0]))
		}
		*nextID++
		id := *nextID
		send(map[string]any{"jsonrpc": "2.0", "id": id, "method": "session/request_permission", "params": map[string]any{
			"sessionId": sessionID,
			"toolCall": map[string]any{
				"kind":      p.Kind,
				"title":     p.Title,
				"locations": fakeLocations(cwd, p.Locations),
			},
			"options": []map[string]any{
				{"optionId": "allow", "name": "Allow once", "kind": "allow_once"},
				{"optionId": "reject", "name": "Reject once", "kind": "reject_once"},
			},
		}})
		// The client's answer is the next message with our id.
		for in.Scan() {
			var m struct {
				ID     *int64          `json:"id"`
				Method string          `json:"method"`
				Result json.RawMessage `json:"result"`
			}
			if json.Unmarshal(bytes.TrimSpace(in.Bytes()), &m) != nil || m.ID == nil || *m.ID != id || m.Method != "" {
				continue
			}
			var res struct {
				Outcome struct {
					Outcome  string `json:"outcome"`
					OptionID string `json:"optionId"`
				} `json:"outcome"`
			}
			_ = json.Unmarshal(m.Result, &res)
			logf(map[string]any{"type": "outcome", "kind": p.Kind, "outcome": res.Outcome.Outcome, "optionId": res.Outcome.OptionID})
			break
		}
	}
	switch {
	case spec.OutSymlink != "":
		_ = os.Remove(filepath.Join(cwd, "out.json"))
		_ = os.Symlink(spec.OutSymlink, filepath.Join(cwd, "out.json"))
	case spec.OutSize > 0:
		_ = os.WriteFile(filepath.Join(cwd, "out.json"), bytes.Repeat([]byte("a"), int(spec.OutSize)), 0o644)
	case spec.Out != "":
		_ = os.WriteFile(filepath.Join(cwd, "out.json"), []byte(spec.Out), 0o644)
	}
}

// fakeLogger opens the fake's log file and returns a writer for it.
func fakeLogger() (func(map[string]any), func()) {
	f, err := os.OpenFile(os.Getenv(fakeLogEnv), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake agent: cannot open log:", err)
		os.Exit(2)
	}
	return func(v map[string]any) {
		data, _ := json.Marshal(v)
		fmt.Fprintln(f, string(data))
	}, func() { f.Close() }
}

// fakeResolve makes a location absolute: relative names are of the
// agent's working directory.
func fakeResolve(cwd, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(cwd, p)
}

func fakeLocations(cwd string, locations []string) []map[string]any {
	locs := make([]map[string]any, len(locations))
	for i, l := range locations {
		locs[i] = map[string]any{"path": fakeResolve(cwd, l)}
	}
	return locs
}

func fakeDirList(cwd string) []string {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// ---- tests ----

// fakeNow is the injectable clock of the generation tests.
var fakeNow = time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)

// happyOut is what the fake agent writes on the happy path: valid
// sources with one unknown heading, one view component, and a field
// the view must drop.
const happyOut = `{"extra":true,"sources":{"s1":{"heading":"Report","occurrence":1},"s2":{"heading":"Überblick 概要 🚀","occurrence":2},"s3":{"heading":"No Such Section"}},"a2ui":[{"id":"t1","component":{"type":"Title","text":{"i":["Report"]}}}]}`

// minimalOut is a small valid out.json.
const minimalOut = `{"a2ui":[{"type":"Text"}],"sources":{}}`

// sectionVectors is the shared section/hash fixture.
type sectionVectors struct {
	Document string `json:"document"`
	Sections []struct {
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		Start      int    `json:"start"`
		End        int    `json:"end"`
		Text       string `json:"text"`
		Hash       string `json:"hash"`
	} `json:"sections"`
	Hashes []struct {
		Text string `json:"text"`
		Hash string `json:"hash"`
	} `json:"hashes"`
	Normalize []struct {
		Text string `json:"text"`
		Key  string `json:"key"`
	} `json:"normalize"`
	Notes struct {
		Document string `json:"document"`
		Sections []struct {
			Start   int         `json:"start"`
			End     int         `json:"end"`
			Exclude []TextRange `json:"exclude"`
			Hash    string      `json:"hash"`
		} `json:"sections"`
	} `json:"notes"`
}

func loadVectors(t *testing.T) sectionVectors {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("web", "test", "fixtures", "section-vectors.json"))
	if err != nil {
		t.Fatalf("read section vectors: %v", err)
	}
	var v sectionVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse section vectors: %v", err)
	}
	return v
}

func sectionsOf(v sectionVectors) []SectionRef {
	sections := make([]SectionRef, len(v.Sections))
	for i, s := range v.Sections {
		sections[i] = SectionRef{Heading: s.Heading, Occurrence: s.Occurrence, Start: s.Start, End: s.End}
	}
	return sections
}

// progressCollector records the progress a run sent.
type progressCollector struct {
	mu    sync.Mutex
	steps []GenerateProgress
}

func (pc *progressCollector) collect(p GenerateProgress) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.steps = append(pc.steps, p)
	return nil
}

func (pc *progressCollector) got() []GenerateProgress {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return append([]GenerateProgress(nil), pc.steps...)
}

// driveGeneration runs runGeneration with a fake agent playing spec.
// The job defaults to the section vectors' document as the text; View
// is the view file to write, left empty for one in a fresh temporary
// folder. mutate adjusts the job before the run.
func driveGeneration(t *testing.T, spec fakeSpec, progress func(GenerateProgress) error, mutate func(*generationJob)) (error, generationJob, []map[string]any) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("encode fake spec: %v", err)
	}
	vectors := loadVectors(t)
	job := generationJob{
		Req: GenerateRequest{
			Text:     vectors.Document,
			Mode:     "full",
			Request:  "Make the intro friendlier.",
			Format:   `{"name":"genui-test","components":[{"type":"Text"}]}`,
			Sections: sectionsOf(vectors),
		},
		AgentCommand: os.Args[0],
		Model:        "gemini-3.8-flash",
		ViewPath:     filepath.Join(dir, ".notes.md.genui.json"),
		Now:          func() time.Time { return fakeNow },
		Env: append(os.Environ(),
			fakeAgentEnv+"=1",
			fakeSpecEnv+"="+string(specData),
			fakeLogEnv+"="+logPath,
		),
	}
	if mutate != nil {
		mutate(&job)
	}
	if progress == nil {
		progress = func(GenerateProgress) error { return nil }
	}
	err = runGeneration(context.Background(), job, progress)
	return err, job, readFakeLog(t, logPath)
}

func readFakeLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // the agent never ran
	}
	if err != nil {
		t.Fatalf("read fake agent log: %v", err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("parse fake agent log line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func logEntries(entries []map[string]any, typ string) []map[string]any {
	var out []map[string]any
	for _, e := range entries {
		if e["type"] == typ {
			out = append(out, e)
		}
	}
	return out
}

// logRecv returns the logged params of the client messages the fake
// received for method.
func logRecv(entries []map[string]any, method string) []map[string]any {
	var out []map[string]any
	for _, e := range logEntries(entries, "recv") {
		if e["method"] == method {
			if params, ok := e["params"].(map[string]any); ok {
				out = append(out, params)
			}
		}
	}
	return out
}

// waitForLog waits until the fake's log contains substr.
func waitForLog(t *testing.T, path, substr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), substr) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the fake agent log %s never contained %q", path, substr)
}

// tempDirGone asserts the fake's working directory, the run's
// temporary folder, is gone.
func tempDirGone(t *testing.T, log []map[string]any) {
	t.Helper()
	cwd := logEntries(log, "cwd")[0]["cwd"].(string)
	if _, err := os.Stat(cwd); !os.IsNotExist(err) {
		t.Errorf("the temporary folder %s still exists", cwd)
	}
}

func compactJSON(t *testing.T, s string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		t.Fatalf("compact %q: %v", s, err)
	}
	return buf.String()
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

func TestHashAndNormalizeVectors(t *testing.T) {
	vectors := loadVectors(t)
	for _, h := range vectors.Hashes {
		got, ok := hashUTF16Range(h.Text, 0, utf16Len(h.Text))
		if !ok || got != h.Hash {
			t.Errorf("hashUTF16Range(%q) = %q, %v; want %s", h.Text, got, ok, h.Hash)
		}
	}
	for _, n := range vectors.Normalize {
		if got := normalizeKey(n.Text); got != n.Key {
			t.Errorf("normalizeKey(%q) = %q, want %q", n.Text, got, n.Key)
		}
	}
	for _, sec := range vectors.Sections {
		got, ok := hashUTF16Range(vectors.Document, sec.Start, sec.End)
		if !ok || got != sec.Hash {
			t.Errorf("hashUTF16Range(document, %d, %d) = %q, %v; want %s", sec.Start, sec.End, got, ok, sec.Hash)
		}
	}
}

func TestHashLeavesOutNotes(t *testing.T) {
	notes := loadVectors(t).Notes
	if len(notes.Sections) == 0 {
		t.Fatal("no note vectors")
	}
	for _, sec := range notes.Sections {
		got, ok := hashSection(notes.Document, SectionRef{Start: sec.Start, End: sec.End, Exclude: sec.Exclude})
		if !ok || got != sec.Hash {
			t.Errorf("hashSection(%d, %d, %v) = %q, %v; want %s", sec.Start, sec.End, sec.Exclude, got, ok, sec.Hash)
		}
	}
}

func TestHashRejectsRangesOutsideTheSection(t *testing.T) {
	for _, exclude := range [][]TextRange{
		{{Start: 0, End: 5}},                     // starts before the section
		{{Start: 3, End: 20}},                    // ends after the section
		{{Start: 4, End: 3}},                     // backwards
		{{Start: 3, End: 5}, {Start: 4, End: 6}}, // overlapping
	} {
		if got, ok := hashSection("0123456789", SectionRef{Start: 2, End: 8, Exclude: exclude}); ok {
			t.Errorf("hashSection with %v = %q, want it refused", exclude, got)
		}
	}
}

func TestGenerateHappyPath(t *testing.T) {
	vectors := loadVectors(t)
	spec := fakeSpec{
		Thought:  true,
		Messages: []string{"The view is", " ready."},
		ToolCalls: []fakeToolCallSpec{
			{Name: "read_file", Kind: "read", Title: "File source.md", Locations: []string{"source.md"}},
			{Name: "write_file", Kind: "edit", Title: "File out.json", Locations: []string{"out.json"}},
		},
		Permissions: []fakePermissionSpec{
			{Kind: "edit", Title: "Write out.json", Locations: []string{"out.json"}},
		},
		Out: happyOut,
	}
	progress := &progressCollector{}
	err, job, log := driveGeneration(t, spec, progress.collect, nil)
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}

	raw, err := os.ReadFile(job.ViewPath)
	if err != nil {
		t.Fatalf("read view file: %v", err)
	}
	text := string(raw)

	// The keys come in this order: version, generatedAt, model,
	// request, sources, a2ui.
	last := -1
	for _, key := range []string{`"version"`, `"generatedAt"`, `"model"`, `"request"`, `"sources"`, `"a2ui"`} {
		i := strings.Index(text, key)
		if i < 0 {
			t.Fatalf("view file has no %s: %s", key, text)
		}
		if i <= last {
			t.Errorf("view file key %s out of order: %s", key, text)
		}
		last = i
	}

	var view struct {
		Version     int                        `json:"version"`
		GeneratedAt string                     `json:"generatedAt"`
		Model       string                     `json:"model"`
		Request     string                     `json:"request"`
		Sources     map[string]json.RawMessage `json:"sources"`
		A2UI        json.RawMessage            `json:"a2ui"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("parse view file: %v", err)
	}
	if view.Version != 2 {
		t.Errorf("version = %d, want 2", view.Version)
	}
	if view.GeneratedAt != fakeNow.UTC().Format(time.RFC3339) {
		t.Errorf("generatedAt = %q, want %q", view.GeneratedAt, fakeNow.UTC().Format(time.RFC3339))
	}
	if view.Model != "gemini-3.8-flash" {
		t.Errorf("model = %q", view.Model)
	}
	if view.Request != "Make the intro friendlier." {
		t.Errorf("request = %q", view.Request)
	}

	// The a2ui array is the agent's, verbatim; the extra field is
	// dropped.
	var agentOut map[string]json.RawMessage
	if err := json.Unmarshal([]byte(happyOut), &agentOut); err != nil {
		t.Fatalf("parse happy out: %v", err)
	}
	if got, want := compactJSON(t, string(view.A2UI)), compactJSON(t, string(agentOut["a2ui"])); got != want {
		t.Errorf("a2ui = %s, want %s", got, want)
	}
	if strings.Contains(text, "extra") {
		t.Errorf("view file kept the agent's extra field: %s", text)
	}

	// The sources carry the fixture hashes; the unknown heading gets
	// no hash, and its occurrence defaults to 1, written out.
	wantSources := map[string]string{
		"s1": `{"heading":"Report","occurrence":1,"hash":"fnv1a32:d04be74a"}`,
		"s2": `{"heading":"Überblick 概要 🚀","occurrence":2,"hash":"fnv1a32:9da8db51"}`,
		"s3": `{"heading":"No Such Section","occurrence":1}`,
	}
	if len(view.Sources) != len(wantSources) {
		t.Errorf("sources = %v", view.Sources)
	}
	for key, want := range wantSources {
		if got := compactJSON(t, string(view.Sources[key])); got != want {
			t.Errorf("source %s = %s, want %s", key, got, want)
		}
	}

	// The prompt carried the format and the reader's request.
	prompts := logRecv(log, "session/prompt")
	if len(prompts) != 1 {
		t.Fatalf("the fake got %d prompts", len(prompts))
	}
	prompt := prompts[0]["prompt"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"Mark Go Go", "source.md", "out.json", "a2ui", job.Req.Format, job.Req.Request} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not mention %q", want)
		}
	}

	// The agent ran with no arguments but "acp".
	args := logEntries(log, "args")[0]["args"].([]any)
	if len(args) != 2 || args[1] != "acp" {
		t.Errorf("agent command line = %v, want the binary and \"acp\"", args)
	}

	// The agent's folder held only source.md, then out.json with it.
	files := logEntries(log, "files")
	if got := files[0]["files"].([]any); len(got) != 1 || got[0] != "source.md" {
		t.Errorf("agent folder at start = %v, want only source.md", got)
	}
	if got := files[1]["files"].([]any); len(got) != 2 {
		t.Errorf("agent folder at end = %v, want out.json and source.md", got)
	}
	tempDirGone(t, log)

	// The page got one progress step per phase change.
	wantProgress := []GenerateProgress{
		{Phase: "starting", Text: "Starting the agent"},
		{Phase: "starting", Text: "Agent ready"},
		{Phase: "thinking", Text: "Thinking"},
		{Phase: "reading", Text: "File source.md"},
		{Phase: "writing", Text: "File out.json"},
		{Phase: "checking", Text: "Checking the view"},
	}
	gotProgress := progress.got()
	if len(gotProgress) != len(wantProgress) {
		t.Fatalf("progress = %+v, want %+v", gotProgress, wantProgress)
	}
	for i, p := range gotProgress {
		if p != wantProgress[i] {
			t.Errorf("progress %d = %+v, want %+v", i, p, wantProgress[i])
		}
	}

	// The allowed permission got an allow_once answer.
	outcomes := logEntries(log, "outcome")
	if len(outcomes) != 1 {
		t.Fatalf("got %d permission outcomes: %v", len(outcomes), outcomes)
	}
	if outcomes[0]["outcome"] != "selected" || outcomes[0]["optionId"] != "allow" {
		t.Errorf("permission outcome = %v %v, want selected allow", outcomes[0]["outcome"], outcomes[0]["optionId"])
	}

	// The document's text went nowhere but source.md and the view: the
	// view file names no section text the agent could have leaked
	// through another channel.
	for _, sec := range vectors.Sections {
		if strings.Contains(text, strings.TrimRight(sec.Text, "\n")) {
			t.Errorf("view file contains section %q body verbatim", sec.Heading)
		}
	}
}

func TestGeneratePermissionPolicy(t *testing.T) {
	spec := fakeSpec{
		Permissions: []fakePermissionSpec{
			{Kind: "read", Title: "Read source", Locations: []string{"source.md"}},
			{Kind: "search", Title: "Search", Locations: []string{"source.md", "out.json"}},
			{Kind: "edit", Title: "Write out", Locations: []string{"out.json"}},
			{Kind: "read", Title: "Read outside", Locations: []string{"/etc/hosts"}},
			{Kind: "read", Title: "Escape", Locations: []string{"../escape.md"}},
			{Kind: "read", Title: "Link out", Locations: []string{"outside-link"}, Symlink: "/etc/hosts"},
			{Kind: "read", Title: "Other file", Locations: []string{"other.md"}},
			{Kind: "execute", Title: "Run", Locations: []string{"source.md"}},
			{Kind: "edit", Title: "Edit source", Locations: []string{"source.md"}},
			{Kind: "edit", Title: "Edit many", Locations: []string{"out.json", "source.md"}},
			{Kind: "read", Title: "No locations"},
			{Kind: "something", Title: "Odd kind"},
		},
		Out: minimalOut,
	}
	err, _, log := driveGeneration(t, spec, nil, nil)
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	want := []string{"allow", "allow", "allow", "reject", "reject", "reject", "reject", "reject", "reject", "reject", "reject", "reject"}
	outcomes := logEntries(log, "outcome")
	if len(outcomes) != len(want) {
		t.Fatalf("got %d outcomes: %v", len(outcomes), outcomes)
	}
	for i, o := range outcomes {
		wantID := "reject"
		if want[i] == "allow" {
			wantID = "allow"
		}
		if o["outcome"] != "selected" || o["optionId"] != wantID {
			t.Errorf("outcome %d (kind %v) = %v %v, want selected %s", i, o["kind"], o["outcome"], o["optionId"], wantID)
		}
	}
}

func TestGenerateAbortsOutsideToolCall(t *testing.T) {
	dir := t.TempDir()
	viewPath := filepath.Join(dir, ".notes.md.genui.json")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	spec := fakeSpec{
		ToolCalls: []fakeToolCallSpec{
			{Name: "read_file", Kind: "read", Title: "File source.md", Locations: []string{"source.md"}},
			{Name: "run_command", Kind: "execute", Title: "Run ls", Locations: []string{}},
		},
		Out: happyOut, // must never reach the view file
	}
	err, _, log := driveGeneration(t, spec, nil, func(job *generationJob) { job.ViewPath = viewPath })
	if err == nil || !strings.Contains(err.Error(), `The agent tried to use "Run ls" outside its folder`) {
		t.Fatalf("error = %v, want the outside-folder abort", err)
	}
	got, err := os.ReadFile(viewPath)
	if err != nil || string(got) != string(sentinel) {
		t.Errorf("view file = %q, %v; want the sentinel untouched", got, err)
	}
	tempDirGone(t, log)
}

func TestGenerateModel(t *testing.T) {
	// A model is chosen with set_config_option.
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, nil)
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	sets := logRecv(log, "session/set_config_option")
	if len(sets) != 1 || sets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("set_config_option = %v, want one with the model", sets)
	}

	// An empty model is not sent.
	err, _, log = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) { job.Model = "" })
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	if sets := logRecv(log, "session/set_config_option"); len(sets) != 0 {
		t.Errorf("set_config_option = %v, want none", sets)
	}

	// An unknown model is model-invalid, with the models offered.
	err, _, _ = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) { job.Model = "no-such-model" })
	f := failureFrom(t, err)
	wantModels := []string{"gemini-4-argon", "gemini-3.8-flash", "claude-opus-5-5"}
	if f.Kind != failModelInvalid || f.Message != `The agent does not know the model "no-such-model".` || f.Hint != hintModelInvalid || strings.Join(f.Models, ",") != strings.Join(wantModels, ",") {
		t.Errorf("failure = %+v, want model-invalid with %v", f, wantModels)
	}
}

// failureFrom returns the failure of a run's error, which must be one.
func failureFrom(t *testing.T, err error) GenerateFailure {
	t.Helper()
	var ge *genError
	if !errors.As(err, &ge) {
		t.Fatalf("error = %v (%T), want a generation failure", err, err)
	}
	return ge.GenerateFailure
}

// writeSentinelView writes a view file the failing runs must leave as
// it is, and returns its path and bytes.
func writeSentinelView(t *testing.T) (string, []byte) {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	return viewPath, sentinel
}

// sentinelKept asserts the view file still has the sentinel's bytes.
func sentinelKept(t *testing.T, viewPath string, sentinel []byte) {
	t.Helper()
	if got, err := os.ReadFile(viewPath); err != nil || string(got) != string(sentinel) {
		t.Errorf("view file = %q, %v; want the sentinel untouched", got, err)
	}
}

func TestGenerateAgentWords(t *testing.T) {
	// What the agent says when it fails reaches the reader verbatim:
	// as its last reply when the turn ends without out.json, or as its
	// error answer to the prompt. An agent that is not signed in gets
	// the login hint, whichever call it fails.
	const quota = "Individual quota reached.\nResets in 3h 12m."
	loginHint := fmt.Sprintf("Run `%s acp --login` in Terminal, then retry.", os.Args[0])
	cases := []struct {
		name                  string
		spec                  fakeSpec
		kind, message, detail string
		hint                  string
	}{
		{"reply without out.json", fakeSpec{Messages: []string{"Individual quota reached.\n", "Resets in 3h 12m."}},
			failAgentMessage, "The agent finished without writing a view.", quota, hintAgentMessage},
		{"prompt error", fakeSpec{Errors: map[string]fakeRPCError{"session/prompt": {Code: -32603, Message: quota}}},
			failAgentMessage, "The agent stopped with an error.", quota, hintAgentMessage},
		{"auth code on session/new", fakeSpec{Errors: map[string]fakeRPCError{"session/new": {Code: -32000, Message: "Authentication required"}}},
			failAuthRequired, "The agent is not signed in.", "Authentication required", loginHint},
		{"auth words on the prompt", fakeSpec{Errors: map[string]fakeRPCError{"session/prompt": {Code: -32603, Message: "Error: authentication required. Run 'agy acp --login' to log in."}}},
			failAuthRequired, "The agent is not signed in.", "Error: authentication required. Run 'agy acp --login' to log in.", loginHint},
		{"auth on its error output", fakeSpec{Crash: "Error: authentication required. Run 'agy acp --login' to log in."},
			failAuthRequired, "The agent is not signed in.", "The agent exited before answering initialize. Its error output: Error: authentication required. Run 'agy acp --login' to log in.", loginHint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewPath, sentinel := writeSentinelView(t)
			err, _, log := driveGeneration(t, tc.spec, nil, func(job *generationJob) { job.ViewPath = viewPath })
			f := failureFrom(t, err)
			if f.Kind != tc.kind || f.Message != tc.message || f.Detail != tc.detail || f.Hint != tc.hint {
				t.Errorf("failure = %+v, want %s %q, detail %q, hint %q", f, tc.kind, tc.message, tc.detail, tc.hint)
			}
			sentinelKept(t, viewPath, sentinel)
			tempDirGone(t, log)
		})
	}
}

func TestGenerateToolCallUpdate(t *testing.T) {
	// A tool_call_update that carries everything, and one that carries
	// only its id and status, falling back to what its tool_call
	// carried, both pass the policy.
	spec := fakeSpec{
		ToolCalls: []fakeToolCallSpec{
			{Name: "read_file", Kind: "read", Title: "File source.md", Locations: []string{"source.md"}},
			{Update: true, ToolCallID: "tool-0", Status: "completed"},
			{Name: "write_file", Kind: "edit", Title: "File out.json", Locations: []string{"out.json"}},
			{Update: true, ToolCallID: "tool-1", Kind: "edit", Title: "File out.json", Locations: []string{"out.json"}, Status: "completed"},
		},
		Out: minimalOut,
	}
	if err, _, _ := driveGeneration(t, spec, nil, nil); err != nil {
		t.Fatalf("runGeneration: %v", err)
	}

	// An update for an unknown id, with no kind of its own, is outside
	// the policy and stops the run with the view untouched.
	dir := t.TempDir()
	viewPath := filepath.Join(dir, ".notes.md.genui.json")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	spec.ToolCalls = append(spec.ToolCalls, fakeToolCallSpec{Update: true, ToolCallID: "unknown", Status: "completed"})
	err, _, _ := driveGeneration(t, spec, nil, func(job *generationJob) { job.ViewPath = viewPath })
	if err == nil || !strings.Contains(err.Error(), `The agent tried to use "unknown" outside its folder`) {
		t.Fatalf("error = %v, want the outside-folder abort", err)
	}
	got, err := os.ReadFile(viewPath)
	if err != nil || string(got) != string(sentinel) {
		t.Errorf("view file = %q, %v; want the sentinel untouched", got, err)
	}
}

func TestGenerateInvalidOutputs(t *testing.T) {
	cases := []struct {
		name string
		spec fakeSpec
		want string
	}{
		{"missing out.json", fakeSpec{}, "The agent did not write out.json."},
		{"not json", fakeSpec{Out: "not json"}, "The agent's out.json is not valid JSON."},
		{"too large", fakeSpec{OutSize: maxViewBytes + 1}, "more than the"},
		{"symlink", fakeSpec{OutSymlink: "/etc/hosts"}, "symbolic link"},
		{"no a2ui", fakeSpec{Out: `{"sources":{}}`}, `no "a2ui"`},
		{"empty a2ui", fakeSpec{Out: `{"a2ui":[],"sources":{}}`}, `non-empty "a2ui" array`},
		{"sources not object", fakeSpec{Out: `{"a2ui":[{"x":1}],"sources":[]}`}, `"sources" as an object`},
		{"source without heading", fakeSpec{Out: `{"a2ui":[{"x":1}],"sources":{"s1":{"occurrence":1}}}`}, "need a heading"},
		{"occurrence zero", fakeSpec{Out: `{"a2ui":[{"x":1}],"sources":{"s1":{"heading":"Report","occurrence":0}}}`}, "at least one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewPath, sentinel := writeSentinelView(t)
			err, _, log := driveGeneration(t, tc.spec, nil, func(job *generationJob) { job.ViewPath = viewPath })
			f := failureFrom(t, err)
			if f.Kind != failInvalidOutput || !strings.Contains(f.Message, tc.want) || f.Hint != hintInvalidOutput {
				t.Fatalf("failure = %+v, want invalid-output containing %q", f, tc.want)
			}
			sentinelKept(t, viewPath, sentinel)
			tempDirGone(t, log)
		})
	}

	// The agent's reply, when it gave one with a bad out.json, is the
	// failure's detail.
	err, _, _ := driveGeneration(t, fakeSpec{Out: "not json", Messages: []string{"Done."}}, nil, nil)
	if f := failureFrom(t, err); f.Kind != failInvalidOutput || f.Detail != "Done." {
		t.Errorf("failure = %+v, want invalid-output with the reply as detail", f)
	}
}

// hangingRun starts a run whose agent never answers the prompt, waits
// until it got the prompt, lets interrupt end the run, and returns the
// run's error with the fake's log and the view file's state.
func hangingRun(t *testing.T, timeout time.Duration, interrupt func(cancel context.CancelFunc)) (error, []map[string]any) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(fakeSpec{Hang: true})
	if err != nil {
		t.Fatal(err)
	}
	viewPath, sentinel := writeSentinelView(t)
	job := generationJob{
		Req:          GenerateRequest{Text: "hello", Mode: "full", Format: "{}"},
		AgentCommand: os.Args[0],
		Model:        "gemini-3.8-flash",
		ViewPath:     viewPath,
		Timeout:      timeout,
		Env: append(os.Environ(),
			fakeAgentEnv+"=1",
			fakeSpecEnv+"="+string(specData),
			fakeLogEnv+"="+logPath,
		),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runGeneration(ctx, job, func(GenerateProgress) error { return nil }) }()
	waitForLog(t, logPath, "session/prompt")
	interrupt(cancel)

	select {
	case err = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runGeneration never returned")
	}

	log := readFakeLog(t, logPath)
	if len(logEntries(log, "cancelled")) != 1 {
		t.Errorf("the agent never got session/cancel")
	}
	pid := int(logEntries(log, "pid")[0]["pid"].(float64))
	if syscall.Kill(pid, 0) == nil {
		t.Errorf("the agent process %d is still running", pid)
	}
	tempDirGone(t, log)
	sentinelKept(t, viewPath, sentinel)
	return err, log
}

func TestGenerateCancel(t *testing.T) {
	// The page closing the channel, its window closing and the app
	// quitting all cancel the call's context.
	err, _ := hangingRun(t, 0, func(cancel context.CancelFunc) { cancel() })
	if f := failureFrom(t, err); f.Kind != failCancelled || f.Message != "Generation was cancelled." {
		t.Errorf("failure = %+v, want the cancellation", f)
	}
}

func TestGenerateTimeout(t *testing.T) {
	err, _ := hangingRun(t, 300*time.Millisecond, func(context.CancelFunc) {})
	f := failureFrom(t, err)
	if f.Kind != failTimeout || f.Message != "Generation timed out after 300ms." || f.Hint != hintTimeout {
		t.Errorf("failure = %+v, want the timeout", f)
	}
	if got := timedOut(defaultGenerationTimeout).Message; got != "Generation timed out after 5 minutes." {
		t.Errorf("default timeout message = %q", got)
	}
}

func TestReportFailure(t *testing.T) {
	// A failed run's last step carries the failure, and the error the
	// promise rejects with is the message and the hint; an error that
	// is not a failure becomes an agent-error.
	var steps []GenerateProgress
	collect := func(p GenerateProgress) error {
		steps = append(steps, p)
		return errors.New("the page is gone")
	}
	err := reportFailure(newFailure(failBusy, "A view is already being generated.", hintBusy), collect)
	if err.Error() != "A view is already being generated. Wait for it to finish, or cancel it." {
		t.Errorf("error = %q", err)
	}
	err = reportFailure(errors.New("no window"), collect)
	if err.Error() != "no window "+hintAgentError {
		t.Errorf("error = %q", err)
	}
	if len(steps) != 2 || steps[0].Phase != "failed" || steps[0].Text != "A view is already being generated." ||
		steps[0].Failure == nil || steps[0].Failure.Kind != failBusy || steps[1].Failure == nil || steps[1].Failure.Kind != failAgentError {
		t.Errorf("steps = %+v", steps)
	}
}

func TestSettingsStore(t *testing.T) {
	dir := t.TempDir()
	st := &settingsStore{dir: dir}
	wantDefaults := Settings{
		Model:        "gemini-3.8-flash",
		AgentCommand: "agy",
		Models:       []string{"gemini-3.8-flash", "gemini-4-argon", "claude-opus-5-5"},
	}
	if got := st.get(); !reflect.DeepEqual(got, wantDefaults) {
		t.Errorf("defaults = %+v, want %+v", got, wantDefaults)
	}
	if err := st.set(Settings{Model: "m-1", AgentCommand: "/usr/bin/agy"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got, Settings{
		Model:        "m-1",
		AgentCommand: "/usr/bin/agy",
		Models:       wantDefaults.Models,
	}) {
		t.Errorf("round trip = %+v", got)
	}
	// Caching models from ACP session/new persists across reloads and survives SetSettings that only changes model.
	if err := st.setModels([]string{"claude-opus-5-5", "gemini-3.8-flash", "claude-opus-5-5", ""}); err != nil {
		t.Fatalf("setModels: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("cached models = %+v", got.Models)
	}
	if err := st.set(Settings{Model: "", AgentCommand: ""}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); got.Model != "" || got.AgentCommand != "agy" || !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("empty model and command = %+v", got)
	}

	// A store without a directory answers with the defaults and
	// refuses saves.
	empty := &settingsStore{}
	if got := empty.get(); !reflect.DeepEqual(got, defaultSettings()) {
		t.Errorf("store without a directory = %+v", got)
	}
	if err := empty.set(defaultSettings()); err == nil {
		t.Errorf("set without a directory succeeded")
	}

	// The file is written atomically: no leftovers in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != settingsFile {
			t.Errorf("stray file %q in the settings directory", e.Name())
		}
	}
}

func TestGenerateCachesModelsFromHandshake(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	err, _, _ := driveGeneration(t, fakeSpec{
		Models: []string{"model-alpha", "model-beta"},
		Out:    happyOut,
	}, nil, func(job *generationJob) {
		job.Model = "model-alpha"
		job.Settings = st
	})
	if err != nil {
		t.Fatalf("driveGeneration: %v", err)
	}
	if got := (&settingsStore{dir: st.dir}).get().Models; !reflect.DeepEqual(got, []string{"model-alpha", "model-beta"}) {
		t.Errorf("cached models after handshake = %+v, want [model-alpha model-beta]", got)
	}
}

func TestWriteFileAtomicMode(t *testing.T) {
	// A new file is readable by others, as a saved document is; a
	// replaced file keeps the permissions its owner gave it.
	dir := t.TempDir()
	modeOf := func(path string) os.FileMode {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}
	fresh := filepath.Join(dir, ".a.md.genui.json")
	if err := writeFileAtomic(fresh, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if got := modeOf(fresh); got != 0o644 {
		t.Errorf("new file mode = %v, want 0644", got)
	}
	private := filepath.Join(dir, ".b.md.genui.json")
	if err := os.WriteFile(private, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(private, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if got := modeOf(private); got != 0o600 {
		t.Errorf("replaced file mode = %v, want 0600", got)
	}
}

func TestGenerateCommandNotFound(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{}, nil, func(job *generationJob) {
		job.AgentCommand = "definitely-not-an-agent-cmd"
	})
	if f := failureFrom(t, err); f.Kind != failAgentNotFound || f.Message != `Could not find the agent command "definitely-not-an-agent-cmd".` || f.Hint != hintAgentNotFound {
		t.Errorf("failure = %+v, want agent-not-found", f)
	}

	// A full path is run as it is: one that is not there cannot start.
	missing := filepath.Join(t.TempDir(), "agy")
	err, _, _ = driveGeneration(t, fakeSpec{}, nil, func(job *generationJob) { job.AgentCommand = missing })
	if f := failureFrom(t, err); f.Kind != failAgentNotFound || !strings.HasPrefix(f.Message, fmt.Sprintf("Could not start the agent command %q:", missing)) {
		t.Errorf("failure = %+v, want agent-not-found for the full path", f)
	}
}

func TestGenerateBadMode(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{}, nil, func(job *generationJob) { job.Req.Mode = "sections" })
	if f := failureFrom(t, err); f.Kind != failAgentError || !strings.Contains(f.Message, `only "full" and "update" are supported`) {
		t.Errorf("failure = %+v, want the mode error", f)
	}
}

func TestGenerateAgentCrash(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{Crash: "exploded on start"}, nil, nil)
	f := failureFrom(t, err)
	if f.Kind != failAgentError || !strings.Contains(f.Message, "The agent exited before answering initialize.") || !strings.Contains(f.Message, "exploded on start") || f.Hint != hintAgentError {
		t.Errorf("failure = %+v, want agent-error with the agent's stderr", f)
	}
}
