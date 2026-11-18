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
	"os/exec"
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

func setStored(t *testing.T, st *settingsStore, model, command string) {
	t.Helper()
	if _, err := st.apply(SettingsPatch{Model: &model, AgentCommand: &command}); err != nil {
		t.Fatalf("apply settings: %v", err)
	}
}

const (
	fakeAgentEnv      = "MARKGOGO_FAKE_AGENT"
	fakeSpecEnv       = "MARKGOGO_FAKE_SPEC"
	fakeLogEnv        = "MARKGOGO_FAKE_LOG"
	fakePipeHolderEnv = "MARKGOGO_FAKE_PIPE_HOLDER"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeAgentEnv) != "" {
		runFakeAgent()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type fakeToolCallSpec struct {
	Update     bool     `json:"update"`
	ToolCallID string   `json:"toolCallId"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Locations  []string `json:"locations"`
}

type fakePermissionSpec struct {
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	Locations []string `json:"locations"`
	Symlink   string   `json:"symlink"`
}

type fakeSpec struct {
	Thought              bool                    `json:"thought"`
	Messages             []string                `json:"messages"`
	ToolCalls            []fakeToolCallSpec      `json:"toolCalls"`
	Permissions          []fakePermissionSpec    `json:"permissions"`
	Models               []string                `json:"models"`
	ThoughtValues        []string                `json:"thoughtValues"`
	ThoughtCurrent       string                  `json:"thoughtCurrent"`
	ConfigUpdate         bool                    `json:"configUpdate"`
	Out                  string                  `json:"out"`
	OutSize              int64                   `json:"outSize"`
	OutSymlink           string                  `json:"outSymlink"`
	Hang                 bool                    `json:"hang"`
	HoldPipes            bool                    `json:"holdPipes"`
	AnswerCancel         bool                    `json:"answerCancel"`
	CancelReplyDelayMS   int                     `json:"cancelReplyDelayMs"`
	ExitAfterPrompt      bool                    `json:"exitAfterPrompt"`
	UniqueSessions       bool                    `json:"uniqueSessions"`
	SessionClose         bool                    `json:"sessionClose"`
	Silent               string                  `json:"silent"`
	EmptySessionID       bool                    `json:"emptySessionId"`
	ModelMismatch        bool                    `json:"modelMismatch"`
	BareConfigResponse   bool                    `json:"bareConfigResponse"`
	ConfigUpdateModel    string                  `json:"configUpdateModel"`
	ForeignSessionEvents bool                    `json:"foreignSessionEvents"`
	Crash                string                  `json:"crash"`
	Errors               map[string]fakeRPCError `json:"errors"`
}

type fakeRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

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
	sessionCwd := cwd
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
	thoughtValues := spec.ThoughtValues
	if len(thoughtValues) == 0 {
		thoughtValues = []string{"high"}
	}
	thoughtCurrent := spec.ThoughtCurrent
	if thoughtCurrent == "" {
		thoughtCurrent = thoughtValues[0]
	}
	currentModel := models[0]
	sessionID := "fake-session"
	sessionSeq := 0
	nextID := int64(100)
	pendingPermissions := make(map[int64]bool)

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
	thoughtOptions := func() []map[string]string {
		opts := make([]map[string]string, len(thoughtValues))
		for i, v := range thoughtValues {
			opts[i] = map[string]string{"value": v, "name": strings.ToUpper(v)}
		}
		return opts
	}
	optionSet := func(model, thought string) []map[string]any {
		return []map[string]any{
			{"id": "model", "type": "select", "currentValue": model, "options": modelOptions()},
			{"id": "thought_level", "type": "select", "currentValue": thought, "options": thoughtOptions()},
		}
	}
	configOptions := func() []map[string]any { return optionSet(currentModel, thoughtCurrent) }

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
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		if msg.Method != "" {
			logf(map[string]any{"type": "recv", "method": msg.Method, "params": msg.Params})
		}
		if msg.ID != nil && msg.Method == "" {
			if pendingPermissions[*msg.ID] {
				var res struct {
					Outcome struct {
						Outcome  string `json:"outcome"`
						OptionID string `json:"optionId"`
					} `json:"outcome"`
				}
				_ = json.Unmarshal(msg.Result, &res)
				logf(map[string]any{"type": "foreign-permission-answer", "outcome": res.Outcome.Outcome, "optionId": res.Outcome.OptionID})
				delete(pendingPermissions, *msg.ID)
			}
			continue
		}
		if spec.Silent != "" && msg.Method == spec.Silent {
			logf(map[string]any{"type": "silent", "method": msg.Method})
			for in.Scan() {
			}
			os.Exit(0)
		}
		if e, ok := spec.Errors[msg.Method]; ok && msg.ID != nil {
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "error": e})
			continue
		}
		switch {
		case msg.ID != nil && msg.Method == "initialize":
			capabilities := map[string]any{}
			if spec.SessionClose {
				capabilities["sessionCapabilities"] = map[string]any{"close": map[string]any{}}
			}
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": capabilities,
				"authMethods":       []any{},
			}})
		case msg.ID != nil && msg.Method == "session/new":
			var params struct {
				CWD string `json:"cwd"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if params.CWD != "" {
				sessionCwd = params.CWD
			}
			previousSessionID := sessionID
			if spec.UniqueSessions {
				sessionSeq++
				sessionID = fmt.Sprintf("fake-session-%d", sessionSeq)
			}
			answeredSessionID := sessionID
			if spec.EmptySessionID {
				answeredSessionID = ""
			}
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{
				"sessionId":     answeredSessionID,
				"modes":         []any{},
				"models":        []any{},
				"configOptions": configOptions(),
			}})
			if spec.ForeignSessionEvents && spec.UniqueSessions && sessionSeq >= 2 && previousSessionID != "" {
				fakeEmitForeignSession(previousSessionID, &nextID, pendingPermissions, send, logf)
			}
		case msg.ID != nil && msg.Method == "session/close":
			var params struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			logf(map[string]any{"type": "session-closed", "sessionId": params.SessionID})
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{}})
		case msg.ID != nil && msg.Method == "session/set_config_option":
			var params struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			known := false
			switch params.ConfigID {
			case "model":
				for _, m := range models {
					if m == params.Value {
						known = true
					}
				}
				if known && !spec.ModelMismatch {
					currentModel = params.Value
				}
			case "thought_level":
				for _, v := range thoughtValues {
					if v == params.Value {
						known = true
					}
				}
				if known {
					thoughtCurrent = params.Value
				}
			}
			if known {
				result := map[string]any{}
				if !spec.BareConfigResponse {
					result["configOptions"] = configOptions()
				}
				send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": result})
				if spec.ConfigUpdate {
					updateModel := currentModel
					if params.ConfigID == "model" && spec.ConfigUpdateModel != "" {
						updateModel = spec.ConfigUpdateModel
					}
					send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
						"sessionId": sessionID,
						"update": map[string]any{
							"sessionUpdate": "config_option_update",
							"configOptions": optionSet(updateModel, thoughtCurrent),
						},
					}})
				}
			} else {
				send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "error": map[string]any{"code": -32602, "message": "Unknown " + params.ConfigID + " value: " + params.Value}})
			}
		case msg.ID != nil && msg.Method == "session/prompt":
			if spec.Hang {
				if spec.HoldPipes {
					startFakePipeHolder(logf)
				}
				promptID := *msg.ID
				answered := false
				for in.Scan() {
					var m struct {
						Method string `json:"method"`
					}
					if json.Unmarshal(bytes.TrimSpace(in.Bytes()), &m) != nil || m.Method != "session/cancel" {
						continue
					}
					logf(map[string]any{"type": "cancelled"})
					if !spec.AnswerCancel {
						os.Exit(0)
					}
					if spec.CancelReplyDelayMS > 0 {
						time.Sleep(time.Duration(spec.CancelReplyDelayMS) * time.Millisecond)
					}
					logf(map[string]any{"type": "cancel-reply", "stopReason": "cancelled"})
					send(map[string]any{"jsonrpc": "2.0", "id": promptID, "result": map[string]any{"stopReason": "cancelled", "usage": map[string]any{}}})
					answered = true
					break
				}
				if !answered {
					os.Exit(0)
				}
				continue
			}
			fakePlayScenario(&spec, sessionCwd, sessionID, &nextID, send, in, logf)
			logf(map[string]any{"type": "files", "files": fakeDirList(sessionCwd)})
			send(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": map[string]any{"stopReason": "end_turn", "usage": map[string]any{}}})
			if spec.ExitAfterPrompt {
				os.Exit(0)
			}
		case msg.Method == "session/cancel":
			logf(map[string]any{"type": "cancelled"})
			os.Exit(0)
		}
	}
}

func startFakePipeHolder(logf func(map[string]any)) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakeACPDescendantPipeHolder$")
	cmd.Env = make([]string, 0, len(os.Environ())+1)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, fakeAgentEnv+"=") || strings.HasPrefix(env, fakeSpecEnv+"=") || strings.HasPrefix(env, fakeLogEnv+"=") || strings.HasPrefix(env, fakePipeHolderEnv+"=") {
			continue
		}
		cmd.Env = append(cmd.Env, env)
	}
	cmd.Env = append(cmd.Env, fakePipeHolderEnv+"=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		logf(map[string]any{"type": "descendant-error", "error": err.Error()})
		return
	}
	logf(map[string]any{"type": "descendant-pid", "pid": cmd.Process.Pid})
}

func TestFakeACPDescendantPipeHolder(t *testing.T) {
	if os.Getenv(fakePipeHolderEnv) == "1" {
		time.Sleep(30 * time.Second)
	}
}

func fakeEmitForeignSession(sessionID string, nextID *int64, pending map[int64]bool, send func(any), logf func(map[string]any)) {
	send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
		"sessionId": sessionID,
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": "foreign-late"},
		},
	}})
	send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
		"sessionId": sessionID,
		"update": map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    "foreign-tool",
			"name":          "write_file",
			"kind":          "edit",
			"title":         "Foreign write",
			"status":        "in_progress",
			"locations":     []map[string]any{{"path": "/etc/hosts"}},
		},
	}})
	send(map[string]any{"jsonrpc": "2.0", "method": "session/config_option_update", "params": map[string]any{
		"sessionId": sessionID,
		"configOptions": []map[string]any{
			{"id": "model", "type": "select", "currentValue": "claude-opus-5-5", "options": []map[string]string{{"value": "claude-opus-5-5", "name": "claude-opus-5-5"}}},
		},
	}})
	*nextID++
	id := *nextID
	pending[id] = true
	logf(map[string]any{"type": "foreign-permission", "id": id, "sessionId": sessionID})
	send(map[string]any{"jsonrpc": "2.0", "id": id, "method": "session/request_permission", "params": map[string]any{
		"sessionId": sessionID,
		"toolCall": map[string]any{
			"kind":      "edit",
			"title":     "Foreign write",
			"locations": []map[string]any{{"path": "/etc/hosts"}},
		},
		"options": []map[string]any{
			{"optionId": "allow", "name": "Allow once", "kind": "allow_once"},
			{"optionId": "reject", "name": "Reject once", "kind": "reject_once"},
		},
	}})
}

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

var fakeNow = time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)

const happyOut = `{"extra":true,"sources":{"s1":{"heading":"Report","occurrence":1},"s2":{"heading":"Überblick 概要 🚀","occurrence":2},"s3":{"heading":"No Such Section"}},"a2ui":[{"id":"t1","component":{"type":"Title","text":{"i":["Report"]}}}]}`

const minimalOut = `{"a2ui":[{"type":"Text"}],"sources":{}}`

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
		Validate:     func(context.Context, []byte) error { return nil },
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
		return nil
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

func killFakeDescendants(t *testing.T, path string) {
	t.Helper()
	for _, e := range logEntries(readFakeLog(t, path), "descendant-pid") {
		pid := int(e["pid"].(float64))
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill fake descendant %d: %v", pid, err)
		}
		waitForFakeDeath(t, pid)
	}
}

func killFakeProcesses(t *testing.T, path string) {
	t.Helper()
	for _, e := range logEntries(readFakeLog(t, path), "pid") {
		pid := int(e["pid"].(float64))
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill fake agent %d: %v", pid, err)
		}
		waitForFakeDeath(t, pid)
	}
}

func waitForFakeDeath(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("fake process %d is still running", pid)
	}
}

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
		{{Start: 0, End: 5}},
		{{Start: 3, End: 20}},
		{{Start: 4, End: 3}},
		{{Start: 3, End: 5}, {Start: 4, End: 6}},
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

	args := logEntries(log, "args")[0]["args"].([]any)
	if len(args) != 2 || args[1] != "acp" {
		t.Errorf("agent command line = %v, want the binary and \"acp\"", args)
	}

	files := logEntries(log, "files")
	if got := files[0]["files"].([]any); len(got) != 1 || got[0] != "source.md" {
		t.Errorf("agent folder at start = %v, want only source.md", got)
	}
	if got := files[1]["files"].([]any); len(got) != 2 {
		t.Errorf("agent folder at end = %v, want out.json and source.md", got)
	}
	tempDirGone(t, log)

	outcomes := logEntries(log, "outcome")
	if len(outcomes) != 1 {
		t.Fatalf("got %d permission outcomes: %v", len(outcomes), outcomes)
	}
	if outcomes[0]["outcome"] != "selected" || outcomes[0]["optionId"] != "allow" {
		t.Errorf("permission outcome = %v %v, want selected allow", outcomes[0]["outcome"], outcomes[0]["optionId"])
	}

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
		Out: happyOut,
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
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, nil)
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	sets := logRecv(log, "session/set_config_option")
	if len(sets) != 1 || sets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("set_config_option = %v, want one with the model", sets)
	}

	err, _, log = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) { job.Model = "" })
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	if sets := logRecv(log, "session/set_config_option"); len(sets) != 0 {
		t.Errorf("set_config_option = %v, want none", sets)
	}

	err, _, _ = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) { job.Model = "no-such-model" })
	f := failureFrom(t, err)
	wantModels := []string{"gemini-4-argon", "gemini-3.8-flash", "claude-opus-5-5"}
	if f.Kind != failModelInvalid || f.Message != `The agent does not know the model "no-such-model".` || f.Hint != hintModelInvalid || strings.Join(f.Models, ",") != strings.Join(wantModels, ",") {
		t.Errorf("failure = %+v, want model-invalid with %v", f, wantModels)
	}
}

func TestGenerateModelMustBeConfirmed(t *testing.T) {
	viewPath, sentinel := writeSentinelView(t)
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut, ModelMismatch: true}, nil, func(job *generationJob) { job.ViewPath = viewPath })
	f := failureFrom(t, err)
	if f.Kind != failAgentError || !strings.Contains(f.Message, "not the requested") || f.Hint != hintAgentError {
		t.Errorf("failure = %+v, want the model mismatch", f)
	}
	if prompts := logRecv(log, "session/prompt"); len(prompts) != 0 {
		t.Errorf("session/prompt = %v; a mismatched model must not run the turn", prompts)
	}
	sentinelKept(t, viewPath, sentinel)

	err, job, _ := driveGeneration(t, fakeSpec{Out: minimalOut, BareConfigResponse: true}, nil, nil)
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	view, err := os.ReadFile(job.ViewPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(view, []byte(`"model"`)) {
		t.Errorf("view = %s, want no unconfirmed model", view)
	}

	err, job, _ = driveGeneration(t, fakeSpec{Out: minimalOut, BareConfigResponse: true, ConfigUpdate: true}, nil, nil)
	if err != nil {
		t.Fatalf("runGeneration with a confirmed update: %v", err)
	}
	view, err = os.ReadFile(job.ViewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(view, []byte(`"model": "gemini-3.8-flash"`)) {
		t.Errorf("view = %s, want the confirmed model", view)
	}

	viewPath, sentinel = writeSentinelView(t)
	err, _, _ = driveGeneration(t, fakeSpec{Out: minimalOut, ConfigUpdate: true, ConfigUpdateModel: "claude-opus-5-5"}, nil, func(job *generationJob) { job.ViewPath = viewPath })
	f = failureFrom(t, err)
	if f.Kind != failAgentError || !strings.Contains(f.Message, "not the requested") {
		t.Errorf("failure = %+v, want the update's model mismatch", f)
	}
	sentinelKept(t, viewPath, sentinel)
}

func failureFrom(t *testing.T, err error) GenerateFailure {
	t.Helper()
	var ge *genError
	if !errors.As(err, &ge) {
		t.Fatalf("error = %v (%T), want a generation failure", err, err)
	}
	return ge.GenerateFailure
}

func writeSentinelView(t *testing.T) (string, []byte) {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	return viewPath, sentinel
}

func sentinelKept(t *testing.T, viewPath string, sentinel []byte) {
	t.Helper()
	if got, err := os.ReadFile(viewPath); err != nil || string(got) != string(sentinel) {
		t.Errorf("view file = %q, %v; want the sentinel untouched", got, err)
	}
}

func TestGenerateAgentWords(t *testing.T) {
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
	spec := fakeSpec{
		ToolCalls: []fakeToolCallSpec{
			{Name: "read_file", Kind: "read", Title: "File source.md", Locations: []string{"source.md"}},
			{Update: true, ToolCallID: "tool-0", Status: "completed"},
			{Name: "write_file", Kind: "edit", Title: "File out.json", Locations: []string{"out.json"}},
			{Update: true, ToolCallID: "tool-1", Kind: "edit", Title: "File out.json", Locations: []string{"out.json"}, Status: "completed"},
			{Update: true, ToolCallID: "unknown", Status: "pending"},
			{Name: "unlabeled", Title: "No kind"},
			{Name: "collect", ToolCallID: "tool-2", Kind: "read", Title: "Read source", Locations: []string{"source.md"}},
			{Update: true, ToolCallID: "tool-2", Kind: "edit", Status: "in_progress"},
			{Update: true, ToolCallID: "tool-2", Status: "completed"},
		},
		Out: minimalOut,
	}
	if err, _, _ := driveGeneration(t, spec, nil, nil); err != nil {
		t.Fatalf("runGeneration: %v", err)
	}

	dir := t.TempDir()
	viewPath := filepath.Join(dir, ".notes.md.genui.json")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	badPath := fakeSpec{
		ToolCalls: []fakeToolCallSpec{
			{Update: true, ToolCallID: "bad", Kind: "edit", Title: "Outside write", Locations: []string{"/etc/hosts"}, Status: "in_progress"},
		},
		Out: minimalOut,
	}
	err, _, _ := driveGeneration(t, badPath, nil, func(job *generationJob) { job.ViewPath = viewPath })
	if err == nil || !strings.Contains(err.Error(), `The agent tried to use "Outside write" outside its folder`) {
		t.Fatalf("error = %v, want the outside-folder abort", err)
	}
	got, err := os.ReadFile(viewPath)
	if err != nil || string(got) != string(sentinel) {
		t.Errorf("view file = %q, %v; want the sentinel untouched", got, err)
	}

	badKind := fakeSpec{
		ToolCalls: []fakeToolCallSpec{{Name: "run", Kind: "execute", Title: "Run a command"}},
		Out:       minimalOut,
	}
	err, _, _ = driveGeneration(t, badKind, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `The agent tried to use "Run a command" outside its folder`) {
		t.Fatalf("error = %v, want the outside-folder abort for the operation", err)
	}

	badNoKind := fakeSpec{
		ToolCalls: []fakeToolCallSpec{
			{Update: true, ToolCallID: "where", Title: "Outside unknown", Locations: []string{"/etc/hosts"}, Status: "in_progress"},
		},
		Out: minimalOut,
	}
	err, _, _ = driveGeneration(t, badNoKind, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `The agent tried to use "Outside unknown" outside its folder`) {
		t.Fatalf("error = %v, want the outside-folder abort for the path", err)
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

	err, _, _ := driveGeneration(t, fakeSpec{Out: "not json", Messages: []string{"Done."}}, nil, nil)
	if f := failureFrom(t, err); f.Kind != failInvalidOutput || f.Detail != "Done." {
		t.Errorf("failure = %+v, want invalid-output with the reply as detail", f)
	}
}

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
	err, _ := hangingRun(t, 0, func(cancel context.CancelFunc) { cancel() })
	if f := failureFrom(t, err); f.Kind != failCancelled || f.Message != "Generation was cancelled." {
		t.Errorf("failure = %+v, want the cancellation", f)
	}
}

func TestGenerateCancelAcknowledgedWithoutProcessExit(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(fakeSpec{Hang: true, AnswerCancel: true, CancelReplyDelayMS: 150})
	if err != nil {
		t.Fatal(err)
	}
	viewPath, sentinel := writeSentinelView(t)
	job := generationJob{
		Req:          GenerateRequest{Text: "hello", Mode: "full", Format: "{}"},
		AgentCommand: os.Args[0],
		Model:        "gemini-3.8-flash",
		ViewPath:     viewPath,
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

	started := time.Now()
	cancel()
	var runErr error
	select {
	case runErr = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runGeneration never returned after cancellation")
	}
	elapsed := time.Since(started)

	if elapsed < 100*time.Millisecond {
		t.Errorf("the run ended %s after cancel, before the agent answered the turn; cancellation must wait for the prompt-turn answer", elapsed)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("cancellation took %s; an acknowledged cancel must complete on the prompt-turn answer, not wait for the process to exit", elapsed)
	}
	if f := failureFrom(t, runErr); f.Kind != failCancelled || f.Message != "Generation was cancelled." {
		t.Errorf("failure = %+v, want the cancellation", f)
	}
	log := readFakeLog(t, logPath)
	if got := len(logEntries(log, "cancelled")); got != 1 {
		t.Errorf("the agent got session/cancel %d times, want once", got)
	}
	if got := len(logEntries(log, "cancel-reply")); got != 1 {
		t.Errorf("the agent answered the cancelled turn %d times, want once", got)
	}
	pid := int(logEntries(log, "pid")[0]["pid"].(float64))
	if syscall.Kill(pid, 0) == nil {
		t.Errorf("the one-shot run left its agent process %d running", pid)
	}
	sentinelKept(t, viewPath, sentinel)
}

func TestACPCancelDoesNotWaitForDescendantPipesOrBlockedWrite(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(fakeSpec{Hang: true, HoldPipes: true})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "acp")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fakeAgentEnv+"=1",
		fakeSpecEnv+"="+string(specData),
		fakeLogEnv+"="+logPath,
	)
	agent, err := newACPAgent(cmd, newLimitBuffer(4<<10))
	if err != nil {
		t.Fatal(err)
	}
	agent.start()
	t.Cleanup(func() {
		killFakeDescendants(t, logPath)
		agent.kill()
	})

	if err := agent.call("initialize", map[string]any{"protocolVersion": 1}, nil); err != nil {
		t.Fatal(err)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	if err := agent.call("session/new", map[string]any{}, &opened); err != nil {
		t.Fatal(err)
	}
	promptDone := make(chan error, 1)
	go func() {
		promptDone <- agent.call("session/prompt", map[string]any{"sessionId": opened.SessionID}, nil)
	}()
	waitForLog(t, logPath, "descendant-pid")
	waitForLog(t, logPath, "session/prompt")

	agent.writeMu.Lock()
	writeLocked := true
	t.Cleanup(func() {
		if writeLocked {
			agent.writeMu.Unlock()
		}
	})
	started := time.Now()
	agent.cancelTurn(opened.SessionID)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Errorf("cancelTurn waited %s for a blocked write", elapsed)
	}
	killDone := make(chan struct{})
	go func() {
		agent.kill()
		close(killDone)
	}()
	select {
	case <-killDone:
		if elapsed := time.Since(started); elapsed > 2*time.Second {
			t.Errorf("agent teardown took %s with a descendant holding the pipes", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent teardown waited for a descendant holding the pipes")
	}
	agent.writeMu.Unlock()
	writeLocked = false
	select {
	case err := <-promptDone:
		if err == nil {
			t.Error("the cancelled prompt unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Error("the pending prompt did not observe agent exit")
	}
}

func TestACPCallKeepsResponseWhenAgentExits(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(fakeSpec{Out: minimalOut, ExitAfterPrompt: true})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "acp")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fakeAgentEnv+"=1",
		fakeSpecEnv+"="+string(specData),
		fakeLogEnv+"="+logPath,
	)
	agent, err := newACPAgent(cmd, newLimitBuffer(4<<10))
	if err != nil {
		t.Fatal(err)
	}
	agent.start()
	t.Cleanup(agent.kill)
	if err := agent.call("initialize", map[string]any{"protocolVersion": 1}, nil); err != nil {
		t.Fatal(err)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	if err := agent.call("session/new", map[string]any{}, &opened); err != nil {
		t.Fatal(err)
	}
	var turned struct {
		StopReason string `json:"stopReason"`
	}
	if err := agent.call("session/prompt", map[string]any{"sessionId": opened.SessionID}, &turned); err != nil {
		t.Fatalf("final prompt response was lost on process exit: %v", err)
	}
	if turned.StopReason != "end_turn" {
		t.Errorf("stop reason = %q, want end_turn", turned.StopReason)
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
	if _, err := st.apply(SettingsPatch{Model: new("m-1"), AgentCommand: new("/usr/bin/agy")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got, Settings{
		Model:        "m-1",
		AgentCommand: "/usr/bin/agy",
		Models:       wantDefaults.Models,
	}) {
		t.Errorf("round trip = %+v", got)
	}
	if err := st.setModels([]string{"claude-opus-5-5", "gemini-3.8-flash", "claude-opus-5-5", ""}); err != nil {
		t.Fatalf("setModels: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("cached models = %+v", got.Models)
	}
	if _, err := st.apply(SettingsPatch{Model: new(""), AgentCommand: new("")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); got.Model != "" || got.AgentCommand != "agy" || !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("empty model and command with stale frontend models = %+v", got)
	}

	empty := &settingsStore{}
	if got := empty.get(); !reflect.DeepEqual(got, defaultSettings()) {
		t.Errorf("store without a directory = %+v", got)
	}
	if _, err := empty.apply(SettingsPatch{Model: new("m-1")}); err == nil {
		t.Errorf("apply without a directory succeeded")
	}

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

func TestSettingsPatchKeepsUntouchedFields(t *testing.T) {
	dir := t.TempDir()
	var announced []Settings
	st := &settingsStore{dir: dir, onChange: func(s Settings) { announced = append(announced, s) }}

	if _, err := st.apply(SettingsPatch{Model: new("claude-opus-5-5")}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	committed, err := st.apply(SettingsPatch{AgentCommand: new("/opt/homebrew/bin/agy")})
	if err != nil {
		t.Fatalf("apply command: %v", err)
	}
	want := Settings{
		Model:        "claude-opus-5-5",
		AgentCommand: "/opt/homebrew/bin/agy",
		Models:       defaultModels,
	}
	if !reflect.DeepEqual(committed, want) {
		t.Errorf("committed = %+v, want %+v", committed, want)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got, want) {
		t.Errorf("stored = %+v, want %+v", got, want)
	}
	if len(announced) != 2 {
		t.Fatalf("announcements = %d, want 2: %+v", len(announced), announced)
	}
	if !reflect.DeepEqual(announced[1], want) {
		t.Errorf("last announcement = %+v, want %+v", announced[1], want)
	}

	if err := st.setModels([]string{"model-alpha", "model-beta"}); err != nil {
		t.Fatalf("setModels: %v", err)
	}
	if _, err := st.apply(SettingsPatch{Model: new("model-beta")}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	last := announced[len(announced)-1]
	if last.Model != "model-beta" || !reflect.DeepEqual(last.Models, []string{"model-alpha", "model-beta"}) {
		t.Errorf("after handshake and model change = %+v", last)
	}
	empty, err := st.apply(SettingsPatch{Model: new(""), AgentCommand: new("")})
	if err != nil {
		t.Fatalf("apply empties: %v", err)
	}
	if empty.Model != "" || empty.AgentCommand != "agy" {
		t.Errorf("empties = %+v", empty)
	}
	if last := announced[len(announced)-1]; last.Model != "" || last.AgentCommand != "agy" {
		t.Errorf("announced empties = %+v", last)
	}
}

func TestSettingsPatchesFromTwoWindowsBothSurvive(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := st.apply(SettingsPatch{Model: new("claude-opus-5-5")}); err != nil {
				t.Errorf("apply model: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := st.apply(SettingsPatch{AgentCommand: new("/opt/bin/agy")}); err != nil {
				t.Errorf("apply command: %v", err)
			}
		}()
	}
	wg.Wait()
	got := st.get()
	if got.Model != "claude-opus-5-5" || got.AgentCommand != "/opt/bin/agy" {
		t.Errorf("after concurrent patches = %+v", got)
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

func TestGenerateCapturesModelWhenSettingsChange(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
		job.Model = "gemini-3.8-flash"
	})
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	sets := logRecv(log, "session/set_config_option")
	if len(sets) != 1 || sets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("set_config_option = %v, want the captured model", sets)
	}
}

func TestGenerateConcurrentJobsKeepSeparateSourceAndOutput(t *testing.T) {
	started := make(chan struct{}, 2)
	done := make(chan struct{})
	gate := func() {
		started <- struct{}{}
		<-done
	}
	dir := t.TempDir()
	aPath := filepath.Join(dir, ".a.md.genui.json")
	bPath := filepath.Join(dir, ".b.md.genui.json")
	type result struct {
		err error
		log []map[string]any
	}
	run := func(text, viewPath, model string) result {
		err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
			job.Req.Text = text
			job.Req.Request = text
			job.ViewPath = viewPath
			job.Model = model
			job.BeforeWrite = func() error {
				gate()
				return nil
			}
		})
		return result{err, log}
	}
	aCh := make(chan result, 1)
	bCh := make(chan result, 1)
	go func() { aCh <- run("# Alpha\n", aPath, "gemini-3.8-flash") }()
	go func() { bCh <- run("# Beta\n", bPath, "claude-opus-5-5") }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("both jobs never reached BeforeWrite together")
		}
	}
	close(done)
	a := <-aCh
	b := <-bCh
	if a.err != nil {
		t.Fatalf("A: %v", a.err)
	}
	if b.err != nil {
		t.Fatalf("B: %v", b.err)
	}
	aSets := logRecv(a.log, "session/set_config_option")
	bSets := logRecv(b.log, "session/set_config_option")
	if len(aSets) != 1 || aSets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("A model = %v", aSets)
	}
	if len(bSets) != 1 || bSets[0]["value"] != "claude-opus-5-5" {
		t.Errorf("B model = %v", bSets)
	}
	aView, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	bView, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(aView, []byte(`"model": "gemini-3.8-flash"`)) {
		t.Errorf("A view missing captured model: %s", aView)
	}
	if !bytes.Contains(bView, []byte(`"model": "claude-opus-5-5"`)) {
		t.Errorf("B view missing captured model: %s", bView)
	}
	if !bytes.Contains(aView, []byte(`"request": "# Alpha\n"`)) {
		t.Errorf("A view missing captured request: %s", aView)
	}
	if !bytes.Contains(bView, []byte(`"request": "# Beta\n"`)) {
		t.Errorf("B view missing captured request: %s", bView)
	}
}

func TestGenerateBeforeWriteRefusesStalePathAndAbandonedClose(t *testing.T) {
	viewPath, sentinel := writeSentinelView(t)
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
		job.ViewPath = viewPath
		job.BeforeWrite = func() error { return errPathChanged }
	})
	f := failureFrom(t, err)
	if f.Kind != failAgentError || !strings.Contains(f.Message, "path changed") {
		t.Errorf("failure = %+v, want path-changed", f)
	}
	sentinelKept(t, viewPath, sentinel)
	tempDirGone(t, log)

	viewPath, sentinel = writeSentinelView(t)
	err, _, log = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
		job.ViewPath = viewPath
		job.BeforeWrite = func() error { return errAbandoned }
	})
	f = failureFrom(t, err)
	if f.Kind != failCancelled {
		t.Errorf("failure = %+v, want cancelled for abandoned close", f)
	}
	sentinelKept(t, viewPath, sentinel)
	tempDirGone(t, log)
}

func TestCommitGenerationWriteGuardsLiveSlot(t *testing.T) {
	h := &documentHost{}
	s := &docSession{host: h, generating: true}
	slot := &generationSlot{path: "/work/a.md"}
	s.genSlot = slot

	if err := h.commitGenerationWrite(s, slot); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Errorf("live empty path = %v, want path-changed", err)
	}

	s.pathFn = func() string { return slot.path }
	if err := h.commitGenerationWrite(s, slot); err != nil {
		t.Errorf("matching path = %v, want nil", err)
	}

	s.pathFn = func() string { return "/elsewhere/a.md" }
	if err := h.commitGenerationWrite(s, slot); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Errorf("moved path = %v, want path-changed", err)
	}
	s.pathFn = func() string { return slot.path }

	slot.abandoned = true
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("abandoned slot = %v, want errAbandoned", err)
	}
	slot.abandoned = false
	s.closed = true
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("closed session = %v, want errAbandoned", err)
	}
	s.closed = false
	other := &generationSlot{path: slot.path}
	s.genSlot = other
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("stale slot = %v, want errAbandoned", err)
	}
}

func TestRejectPathChangeWhileGenerating(t *testing.T) {
	h := &documentHost{}
	s := &docSession{host: h, generating: true, genSlot: &generationSlot{path: "/work/a.md"}}
	if err := h.rejectPathChange(s); err == nil || err.Error() != errGenerating {
		t.Errorf("rejectPathChange = %v, want %q", err, errGenerating)
	}
	s.generating = false
	if err := h.rejectPathChange(s); err != nil {
		t.Errorf("idle rejectPathChange = %v", err)
	}
}

func TestCancelGenerationMarksAbandonedWithoutClearingIdleSlot(t *testing.T) {
	h := &documentHost{}
	s := &docSession{host: h}
	h.cancelGeneration(s)

	called := false
	slot := &generationSlot{cancel: func() { called = true }}
	s.genSlot = slot
	s.generating = true
	h.cancelGeneration(s)
	if !slot.abandoned || !called {
		t.Fatalf("cancelGeneration abandoned=%v called=%v", slot.abandoned, called)
	}
	if s.genSlot != slot {
		t.Fatal("cancel must not drop the slot; Generate's defer still owns cleanup")
	}
}

func TestWaitForGenerationsReturnsWhenIdle(t *testing.T) {
	h := &documentHost{}
	start := time.Now()
	h.waitForGenerations(time.Second)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("idle waitForGenerations must return immediately")
	}
}

func fakeAgentJob(t *testing.T, spec fakeSpec) (func(*generationJob), string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return func(job *generationJob) {
		job.AgentCommand = os.Args[0]
		job.Validate = func(context.Context, []byte) error { return nil }
		job.Now = func() time.Time { return fakeNow }
		job.Env = append(os.Environ(),
			fakeAgentEnv+"=1",
			fakeSpecEnv+"="+string(specData),
			fakeLogEnv+"="+logPath,
		)
	}, logPath
}

func hostSession(t *testing.T, h *documentHost, id, path string) *docSession {
	t.Helper()
	captured := path
	s := &docSession{host: h, id: id, pathFn: func() string { return captured }}
	h.mu.Lock()
	if h.byID == nil {
		h.byID = make(map[string]*docSession)
	}
	h.byID[id] = s
	h.mu.Unlock()
	return s
}

func TestHostGenerateOneSlotPerDocumentConcurrentAcrossDocuments(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.md")
	bPath := filepath.Join(dir, "b.md")
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	setStored(t, h.settings, "gemini-3.8-flash", "agy")
	hostSession(t, h, "a", aPath)
	hostSession(t, h, "b", bPath)

	started := make(chan string, 2)
	release := make(chan struct{})
	prepare, logPath := fakeAgentJob(t, fakeSpec{Out: minimalOut})
	h.prepareJob = func(job *generationJob) {
		prepare(job)
		inner := job.BeforeWrite
		job.BeforeWrite = func() error {
			started <- job.Req.Request
			<-release
			if inner != nil {
				return inner()
			}
			return nil
		}
	}

	aCh := make(chan error, 1)
	bCh := make(chan error, 1)
	go func() {
		aCh <- h.generate(context.Background(), "a", GenerateRequest{Text: "# Alpha\n", Mode: "full", Request: "alpha-req", Format: "{}"}, func(GenerateProgress) error { return nil })
	}()
	go func() {
		bCh <- h.generate(context.Background(), "b", GenerateRequest{Text: "# Beta\n", Mode: "full", Request: "beta-req", Format: "{}"}, func(GenerateProgress) error { return nil })
	}()

	seen := map[string]bool{}
	for range 2 {
		select {
		case id := <-started:
			seen[id] = true
		case <-time.After(10 * time.Second):
			t.Fatal("both documents never reached BeforeWrite together")
		}
	}
	if !seen["alpha-req"] || !seen["beta-req"] {
		t.Fatalf("started = %v, want both requests", seen)
	}
	h.mu.Lock()
	aBusy, bBusy := h.byID["a"].generating, h.byID["b"].generating
	h.mu.Unlock()
	if !aBusy || !bBusy {
		t.Fatalf("generating A=%v B=%v, want both", aBusy, bBusy)
	}

	busy := h.generate(context.Background(), "a", GenerateRequest{Text: "# Alpha\n", Mode: "full", Format: "{}"}, func(GenerateProgress) error { return nil })
	if f := failureFrom(t, busy); f.Kind != failBusy {
		t.Errorf("second A generate = %v, want busy", busy)
	}

	close(release)
	if err := <-aCh; err != nil {
		t.Fatalf("A: %v", err)
	}
	if err := <-bCh; err != nil {
		t.Fatalf("B: %v", err)
	}

	aView, err := os.ReadFile(viewPath(aPath))
	if err != nil {
		t.Fatal(err)
	}
	bView, err := os.ReadFile(viewPath(bPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(aView, []byte(`"request": "alpha-req"`)) {
		t.Errorf("A view missing captured request: %s", aView)
	}
	if !bytes.Contains(bView, []byte(`"request": "beta-req"`)) {
		t.Errorf("B view missing captured request: %s", bView)
	}
	if bytes.Contains(aView, []byte("beta-req")) || bytes.Contains(bView, []byte("alpha-req")) {
		t.Fatal("requests crossed documents")
	}
	_ = logPath
}

func TestHostGenerateReusesOneProcessWithFreshSessionsAndSources(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "a.md")
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	setStored(t, h.settings, "gemini-3.8-flash", "agy")
	hostSession(t, h, "a", docPath)
	h.agentCache = &acpProcessCache{}

	prepare, logPath := fakeAgentJob(t, fakeSpec{Out: minimalOut, UniqueSessions: true, SessionClose: true})
	var traceMu sync.Mutex
	type recorded struct {
		event   string
		payload any
	}
	var events []recorded
	h.prepareJob = func(job *generationJob) {
		prepare(job)
		job.Observe = func(event string, payload any) {
			traceMu.Lock()
			events = append(events, recorded{event, payload})
			traceMu.Unlock()
		}
	}
	t.Cleanup(func() {
		h.agentCache.close()
		killFakeProcesses(t, logPath)
	})

	texts := []string{"# First source\n", "# Second source\n"}
	for i, text := range texts {
		req := GenerateRequest{Text: text, Mode: "full", Request: fmt.Sprintf("request-%d", i+1), Format: "{}"}
		if err := h.generate(context.Background(), "a", req, func(GenerateProgress) error { return nil }); err != nil {
			t.Fatalf("generation %d: %v", i+1, err)
		}
	}

	log := readFakeLog(t, logPath)
	pids := logEntries(log, "pid")
	if len(pids) != 1 {
		t.Fatalf("the agent started %d processes, want one reused process", len(pids))
	}
	pid := int(pids[0]["pid"].(float64))
	if err := syscall.Kill(pid, 0); err != nil {
		t.Errorf("the reused process %d is not alive after the settled runs: %v", pid, err)
	}
	processCwd, _ := logEntries(log, "cwd")[0]["cwd"].(string)

	news := logRecv(log, "session/new")
	if len(news) != 2 {
		t.Fatalf("session/new count = %d, want two fresh sessions", len(news))
	}
	cwds := []string{}
	for _, params := range news {
		cwd, _ := params["cwd"].(string)
		cwds = append(cwds, cwd)
	}
	if cwds[0] == "" || cwds[0] == cwds[1] {
		t.Errorf("session working folders = %q, want two different run folders", cwds)
	}
	for _, cwd := range cwds {
		if cwd == processCwd {
			t.Errorf("the process folder %q is a run folder; it must outlive the lease", processCwd)
		}
		if _, err := os.Stat(cwd); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("run folder %s still exists (%v)", cwd, err)
		}
	}
	if info, err := os.Stat(processCwd); err != nil || !info.IsDir() {
		t.Errorf("the process folder %q did not outlive the leases: %v", processCwd, err)
	}

	prompts := logRecv(log, "session/prompt")
	if len(prompts) != 2 {
		t.Fatalf("session/prompt count = %d, want two", len(prompts))
	}
	ids := []string{}
	for _, params := range prompts {
		id, _ := params["sessionId"].(string)
		ids = append(ids, id)
	}
	if ids[0] != "fake-session-1" || ids[1] != "fake-session-2" {
		t.Errorf("prompt sessions = %q, want two fresh sessions", ids)
	}
	promptText := func(params map[string]any) string {
		blocks, _ := params["prompt"].([]any)
		for _, b := range blocks {
			if block, ok := b.(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					return text
				}
			}
		}
		return ""
	}
	first, second := promptText(prompts[0]), promptText(prompts[1])
	if !strings.Contains(first, texts[0]) || strings.Contains(first, texts[1]) {
		t.Errorf("first prompt does not carry only its own source")
	}
	if !strings.Contains(second, texts[1]) || strings.Contains(second, texts[0]) {
		t.Errorf("second prompt does not carry only its own source")
	}
	if !strings.Contains(first, fmt.Sprintf("%q", cwds[0])) || !strings.Contains(second, fmt.Sprintf("%q", cwds[1])) {
		t.Errorf("prompts do not name their own run folders")
	}

	closes := logRecv(log, "session/close")
	if len(closes) != 2 || closes[0]["sessionId"] != ids[0] || closes[1]["sessionId"] != ids[1] {
		t.Errorf("session/close = %v, want each run's session %q", closes, ids)
	}

	view, err := os.ReadFile(viewPath(docPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(view, []byte(`"request": "request-2"`)) || bytes.Contains(view, []byte("request-1")) {
		t.Errorf("view = %s, want only the last run's request", view)
	}

	traceMu.Lock()
	defer traceMu.Unlock()
	count := func(name string) int {
		n := 0
		for _, e := range events {
			if e.event == name {
				n++
			}
		}
		return n
	}
	if got := count(traceAgentStart); got != 1 {
		t.Errorf("agent.start events = %d, want one cold start", got)
	}
	if got := count(traceAgentReused); got != 1 {
		t.Errorf("agent.reused events = %d, want one reuse", got)
	}
	if got := count(traceInitialize); got != 1 {
		t.Errorf("initialize events = %d, want the cold process's one", got)
	}
	if got := count(traceInitCached); got != 1 {
		t.Errorf("initialize.cached events = %d, want one", got)
	}
	if got := count(traceSessionNew); got != 2 {
		t.Errorf("session.new events = %d, want two fresh sessions", got)
	}
	var reused *traceAgentReusePayload
	var cached *traceInitializeCachedPayload
	var started *traceAgentStartPayload
	initializeSent, closeSent := 0, 0
	for _, e := range events {
		switch payload := e.payload.(type) {
		case *traceAgentStartPayload:
			started = payload
		case *traceAgentReusePayload:
			reused = payload
		case *traceInitializeCachedPayload:
			cached = payload
		case *traceACPPayload:
			if payload.Direction != acpSent {
				continue
			}
			var msg struct {
				Method string `json:"method"`
			}
			if json.Unmarshal(payload.Line, &msg) != nil {
				continue
			}
			switch msg.Method {
			case "initialize":
				initializeSent++
			case "session/close":
				closeSent++
			}
		}
	}
	if started == nil || started.PID != pid || started.Cwd != processCwd {
		t.Errorf("agent.start = %+v, want the cold process %d in %q", started, pid, processCwd)
	}
	if reused == nil || reused.PID != pid || reused.Cwd != processCwd || reused.StartedAt == "" {
		t.Errorf("agent.reused = %+v, want process %d in %q with its start", reused, pid, processCwd)
	}
	if cached == nil || cached.ProcessPID != pid || cached.ProcessCwd != processCwd || cached.ProcessStartedAt == "" || len(cached.Result) == 0 {
		t.Errorf("initialize.cached = %+v, want the cached answer of process %d in %q", cached, pid, processCwd)
	}
	if cached != nil && !bytes.Contains(cached.AgentCapabilities, []byte(`"close"`)) {
		t.Errorf("initialize.cached capabilities = %s, want the advertised close capability", cached.AgentCapabilities)
	}
	if initializeSent != 1 {
		t.Errorf("initialize wire requests = %d, want only the cold process's real one", initializeSent)
	}
	if closeSent != 2 {
		t.Errorf("session/close wire requests = %d, want one per settled session", closeSent)
	}
}

func TestHostGenerateDiscardsReusedProcessOnRepeatedSessionID(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "a.md")
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	setStored(t, h.settings, "gemini-3.8-flash", "agy")
	hostSession(t, h, "a", docPath)
	h.agentCache = &acpProcessCache{}

	prepare, logPath := fakeAgentJob(t, fakeSpec{Out: minimalOut, SessionClose: true})
	h.prepareJob = prepare
	t.Cleanup(func() {
		h.agentCache.close()
		killFakeProcesses(t, logPath)
	})

	req := func(request string) GenerateRequest {
		return GenerateRequest{Text: "# Source\n", Mode: "full", Request: request, Format: "{}"}
	}
	if err := h.generate(context.Background(), "a", req("first"), func(GenerateProgress) error { return nil }); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	err := h.generate(context.Background(), "a", req("second"), func(GenerateProgress) error { return nil })
	if err == nil {
		t.Fatal("the second generation accepted a process that repeated its old session ID")
	}
	if f := failureFrom(t, err); f.Kind != failAgentError || !strings.Contains(strings.ToLower(f.Message), "session") {
		t.Errorf("failure = %+v, want an agent-error about the repeated session", f)
	}

	log := readFakeLog(t, logPath)
	if got := len(logEntries(log, "pid")); got != 1 {
		t.Errorf("the agent started %d processes, want only the reused one", got)
	}
	if got := len(logRecv(log, "session/prompt")); got != 1 {
		t.Errorf("session/prompt count = %d; the rejected session must not run the prompt", got)
	}
	pid := int(logEntries(log, "pid")[0]["pid"].(float64))
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("the process with the repeated session ID %d is still running", pid)
	}
	view, err := os.ReadFile(viewPath(docPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(view, []byte(`"request": "first"`)) || bytes.Contains(view, []byte("second")) {
		t.Errorf("view = %s, want the first run's view untouched", view)
	}
}

func TestHostGenerateCapturesSettingsAndRefusesStalePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath(path), sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	settingsDir := t.TempDir()
	h := &documentHost{settings: &settingsStore{dir: settingsDir}}
	setStored(t, h.settings, "gemini-3.8-flash", "agy")
	current := path
	hostSession(t, h, "a", path).pathFn = func() string { return current }

	gate := make(chan struct{})
	goOn := make(chan struct{})
	prepare, logPath := fakeAgentJob(t, fakeSpec{Out: minimalOut})
	h.prepareJob = func(job *generationJob) {
		prepare(job)
		inner := job.BeforeWrite
		job.BeforeWrite = func() error {
			close(gate)
			<-goOn
			if inner != nil {
				return inner()
			}
			return nil
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.generate(context.Background(), "a", GenerateRequest{Text: "# A\n", Mode: "full", Request: "keep-me", Format: "{}"}, func(GenerateProgress) error { return nil })
	}()
	select {
	case <-gate:
	case <-time.After(10 * time.Second):
		t.Fatal("run never reached BeforeWrite")
	}
	setStored(t, h.settings, "claude-opus-5-5", "agy")
	current = filepath.Join(dir, "renamed.md")
	close(goOn)
	err := <-errCh
	f := failureFrom(t, err)
	if f.Kind != failAgentError || !strings.Contains(f.Message, "path changed") {
		t.Errorf("failure = %+v, want path-changed", f)
	}
	sentinelKept(t, viewPath(path), sentinel)
	log := readFakeLog(t, logPath)
	sets := logRecv(log, "session/set_config_option")
	if len(sets) != 1 || sets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("set_config_option = %v, want captured model", sets)
	}
	tempDirGone(t, log)
}

func TestHostGenerateAbandonedCloseDoesNotWriteAndCancelCloseKeepsSlot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath(path), sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	s := hostSession(t, h, "a", path)

	gate := make(chan struct{})
	goOn := make(chan struct{})
	prepare, logPath := fakeAgentJob(t, fakeSpec{Out: minimalOut})
	h.prepareJob = func(job *generationJob) {
		prepare(job)
		inner := job.BeforeWrite
		job.BeforeWrite = func() error {
			close(gate)
			<-goOn
			if inner != nil {
				return inner()
			}
			return nil
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.generate(context.Background(), "a", GenerateRequest{Text: "# A\n", Mode: "full", Format: "{}"}, func(GenerateProgress) error { return nil })
	}()
	select {
	case <-gate:
	case <-time.After(10 * time.Second):
		t.Fatal("run never reached BeforeWrite")
	}

	if !s.generating {
		t.Fatal("cancelled close must not clear generating")
	}
	h.mu.Lock()
	live := s.genSlot
	h.mu.Unlock()
	if live == nil || live.abandoned {
		t.Fatal("cancelled close must leave the live slot")
	}

	h.dropSession(s)
	close(goOn)
	err := <-errCh
	if f := failureFrom(t, err); f.Kind != failCancelled {
		t.Errorf("failure = %+v, want cancelled after real close", f)
	}
	sentinelKept(t, viewPath(path), sentinel)
	tempDirGone(t, readFakeLog(t, logPath))
}

func TestHostGenerateCloseCancelsHangingAgentAndWaitReturns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	sentinel := []byte("the view that was")
	if err := os.WriteFile(viewPath(path), sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	s := hostSession(t, h, "a", path)
	prepare, logPath := fakeAgentJob(t, fakeSpec{Hang: true})
	h.prepareJob = prepare

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Generate(context.Background(), "a", GenerateRequest{Text: "# A\n", Mode: "full", Format: "{}"}, nil)
	}()
	waitForLog(t, logPath, "session/prompt")
	h.dropSession(s)
	select {
	case err := <-errCh:
		if f := failureFrom(t, err); f.Kind != failCancelled {
			t.Errorf("failure = %+v, want cancelled", f)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Generate never returned after close")
	}
	start := time.Now()
	h.waitForGenerations(2 * time.Second)
	if time.Since(start) > time.Second {
		t.Fatal("waitForGenerations did not return after the cancelled run")
	}
	sentinelKept(t, viewPath(path), sentinel)
	log := readFakeLog(t, logPath)
	tempDirGone(t, log)
	pid := int(logEntries(log, "pid")[0]["pid"].(float64))
	if syscall.Kill(pid, 0) == nil {
		t.Errorf("the agent process %d is still running", pid)
	}
}
