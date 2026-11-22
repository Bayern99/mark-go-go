package acp

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

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
)

const (
	fakeAgentEnv      = "MARKGOGO_FAKE_AGENT"
	fakeSpecEnv       = "MARKGOGO_FAKE_SPEC"
	fakeLogEnv        = "MARKGOGO_FAKE_LOG"
	fakePipeHolderEnv = "MARKGOGO_FAKE_PIPE_HOLDER"
	fakeFolderInTitle = "{folder}"
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
				"title":         strings.ReplaceAll(tc.Title, fakeFolderInTitle, filepath.Base(cwd)),
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
			Start   int               `json:"start"`
			End     int               `json:"end"`
			Exclude []genui.TextRange `json:"exclude"`
			Hash    string            `json:"hash"`
		} `json:"sections"`
	} `json:"notes"`
}

func loadVectors(t *testing.T) sectionVectors {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "markdown", "section-vectors.json"))
	if err != nil {
		t.Fatalf("read section vectors: %v", err)
	}
	var v sectionVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse section vectors: %v", err)
	}
	return v
}

func sectionsOf(v sectionVectors) []genui.SectionRef {
	sections := make([]genui.SectionRef, len(v.Sections))
	for i, s := range v.Sections {
		sections[i] = genui.SectionRef{Heading: s.Heading, Occurrence: s.Occurrence, Start: s.Start, End: s.End}
	}
	return sections
}

type progressCollector struct {
	mu    sync.Mutex
	steps []genui.GenerateProgress
}

func (pc *progressCollector) collect(p genui.GenerateProgress) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.steps = append(pc.steps, p)
	return nil
}

func (pc *progressCollector) got() []genui.GenerateProgress {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return append([]genui.GenerateProgress(nil), pc.steps...)
}

func driveGeneration(t *testing.T, spec fakeSpec, progress func(genui.GenerateProgress) error, mutate func(*GenerationJob)) (error, GenerationJob, []map[string]any) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("encode fake spec: %v", err)
	}
	vectors := loadVectors(t)
	job := GenerationJob{
		Req: genui.GenerateRequest{
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
		progress = func(genui.GenerateProgress) error { return nil }
	}
	err = RunGeneration(context.Background(), job, progress)
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

func TestHashAndNormalizeVectors(t *testing.T) {
	vectors := loadVectors(t)
	for _, h := range vectors.Hashes {
		got, ok := hashUTF16Range(h.Text+"# Next\n", 0, utf16Len(h.Text))
		if !ok || got != h.Hash {
			t.Errorf("hashUTF16Range(%q before another section) = %q, %v; want %s", h.Text, got, ok, h.Hash)
		}
	}
	for _, n := range vectors.Normalize {
		if got := genui.NormalizeKey(n.Text); got != n.Key {
			t.Errorf("genui.NormalizeKey(%q) = %q, want %q", n.Text, got, n.Key)
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
		got, ok := hashSection(notes.Document, genui.SectionRef{Start: sec.Start, End: sec.End, Exclude: sec.Exclude})
		if !ok || got != sec.Hash {
			t.Errorf("hashSection(%d, %d, %v) = %q, %v; want %s", sec.Start, sec.End, sec.Exclude, got, ok, sec.Hash)
		}
	}
}

func TestHashRejectsRangesOutsideTheSection(t *testing.T) {
	for _, exclude := range [][]genui.TextRange{
		{{Start: 0, End: 5}},
		{{Start: 3, End: 20}},
		{{Start: 4, End: 3}},
		{{Start: 3, End: 5}, {Start: 4, End: 6}},
	} {
		if got, ok := hashSection("0123456789", genui.SectionRef{Start: 2, End: 8, Exclude: exclude}); ok {
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
		t.Fatalf("RunGeneration: %v", err)
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

func TestBlankAtTheEndOfTheDocumentLeavesTheLastSectionsSavedHashAlone(t *testing.T) {
	vectors := loadVectors(t)
	last := vectors.Sections[len(vectors.Sections)-1]
	body := strings.TrimRight(vectors.Document, "\n")
	out := fmt.Sprintf(`{"sources":{"s1":{"heading":%q,"occurrence":%d}},"a2ui":[{"type":"Text"}]}`, last.Heading, last.Occurrence)
	for _, blank := range []string{"", "\n", "\n\n\n", " \t\n"} {
		err, job, _ := driveGeneration(t, fakeSpec{Out: out}, nil, func(job *GenerationJob) {
			job.Req.Text = body + blank
			job.Req.Sections = sectionsOf(vectors)
			job.Req.Sections[len(job.Req.Sections)-1].End = utf16Len(job.Req.Text)
		})
		if err != nil {
			t.Fatalf("ending %q: RunGeneration: %v", blank, err)
		}
		raw, err := os.ReadFile(job.ViewPath)
		if err != nil {
			t.Fatal(err)
		}
		var view struct {
			Sources map[string]struct {
				Hash string `json:"hash"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatalf("parse view: %v", err)
		}
		if got := view.Sources["s1"].Hash; got != last.Hash {
			t.Errorf("ending %q: %s hash = %q, want the saved %s", blank, last.Heading, got, last.Hash)
		}
	}
}

func TestProgressSaysEachStepInWordsWithoutTheAgentsFolder(t *testing.T) {
	spec := fakeSpec{
		Thought: true,
		ToolCalls: []fakeToolCallSpec{
			{Name: "read_file", Kind: "read", Title: "File ../" + fakeFolderInTitle + "/source.md", Locations: []string{"source.md"}},
			{Name: "write_file", Kind: "edit", Title: "File ../" + fakeFolderInTitle + "/out.json", Locations: []string{"out.json"}},
		},
		Out: minimalOut,
	}
	progress := &progressCollector{}
	err, _, log := driveGeneration(t, spec, progress.collect, nil)
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	var texts []string
	for _, step := range progress.got() {
		texts = append(texts, step.Text)
	}
	want := []string{"Starting the agent", "Agent ready", "Thinking", "Reading its files", "Writing the view", "Checking the view"}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("progress texts = %q, want %q", texts, want)
	}
	folder := filepath.Base(logEntries(log, "cwd")[0]["cwd"].(string))
	for _, text := range texts {
		if strings.Contains(text, folder) {
			t.Errorf("progress text %q names the agent's temporary folder %s", text, folder)
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
		t.Fatalf("RunGeneration: %v", err)
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
	err, _, log := driveGeneration(t, spec, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
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
		t.Fatalf("RunGeneration: %v", err)
	}
	sets := logRecv(log, "session/set_config_option")
	if len(sets) != 1 || sets[0]["value"] != "gemini-3.8-flash" {
		t.Errorf("set_config_option = %v, want one with the model", sets)
	}

	err, _, log = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) { job.Model = "" })
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	if sets := logRecv(log, "session/set_config_option"); len(sets) != 0 {
		t.Errorf("set_config_option = %v, want none", sets)
	}

	err, _, _ = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) { job.Model = "no-such-model" })
	f := failureFrom(t, err)
	wantModels := []string{"gemini-4-argon", "gemini-3.8-flash", "claude-opus-5-5"}
	if f.Kind != genui.FailModelInvalid || f.Message != `The agent does not know the model "no-such-model".` || f.Hint != genui.HintModelInvalid || strings.Join(f.Models, ",") != strings.Join(wantModels, ",") {
		t.Errorf("failure = %+v, want model-invalid with %v", f, wantModels)
	}
}

func TestGenerateModelMustBeConfirmed(t *testing.T) {
	viewPath, sentinel := writeSentinelView(t)
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut, ModelMismatch: true}, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
	f := failureFrom(t, err)
	if f.Kind != genui.FailAgentError || !strings.Contains(f.Message, "not the requested") || f.Hint != genui.HintAgentError {
		t.Errorf("failure = %+v, want the model mismatch", f)
	}
	if prompts := logRecv(log, "session/prompt"); len(prompts) != 0 {
		t.Errorf("session/prompt = %v; a mismatched model must not run the turn", prompts)
	}
	sentinelKept(t, viewPath, sentinel)

	err, job, _ := driveGeneration(t, fakeSpec{Out: minimalOut, BareConfigResponse: true}, nil, nil)
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
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
		t.Fatalf("RunGeneration with a confirmed update: %v", err)
	}
	view, err = os.ReadFile(job.ViewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(view, []byte(`"model": "gemini-3.8-flash"`)) {
		t.Errorf("view = %s, want the confirmed model", view)
	}

	viewPath, sentinel = writeSentinelView(t)
	err, _, _ = driveGeneration(t, fakeSpec{Out: minimalOut, ConfigUpdate: true, ConfigUpdateModel: "claude-opus-5-5"}, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
	f = failureFrom(t, err)
	if f.Kind != genui.FailAgentError || !strings.Contains(f.Message, "not the requested") {
		t.Errorf("failure = %+v, want the update's model mismatch", f)
	}
	sentinelKept(t, viewPath, sentinel)
}

func failureFrom(t *testing.T, err error) genui.GenerateFailure {
	t.Helper()
	var ge *genui.GenError
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
			genui.FailAgentMessage, "The agent finished without writing a view.", quota, genui.HintAgentMessage},
		{"prompt error", fakeSpec{Errors: map[string]fakeRPCError{"session/prompt": {Code: -32603, Message: quota}}},
			genui.FailAgentMessage, "The agent stopped with an error.", quota, genui.HintAgentMessage},
		{"auth code on session/new", fakeSpec{Errors: map[string]fakeRPCError{"session/new": {Code: -32000, Message: "Authentication required"}}},
			genui.FailAuthRequired, "The agent is not signed in.", "Authentication required", loginHint},
		{"auth words on the prompt", fakeSpec{Errors: map[string]fakeRPCError{"session/prompt": {Code: -32603, Message: "Error: authentication required. Run 'agy acp --login' to log in."}}},
			genui.FailAuthRequired, "The agent is not signed in.", "Error: authentication required. Run 'agy acp --login' to log in.", loginHint},
		{"auth on its error output", fakeSpec{Crash: "Error: authentication required. Run 'agy acp --login' to log in."},
			genui.FailAuthRequired, "The agent is not signed in.", "The agent exited before answering initialize. Its error output: Error: authentication required. Run 'agy acp --login' to log in.", loginHint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewPath, sentinel := writeSentinelView(t)
			err, _, log := driveGeneration(t, tc.spec, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
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
		t.Fatalf("RunGeneration: %v", err)
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
	err, _, _ := driveGeneration(t, badPath, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
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
		{"too large", fakeSpec{OutSize: genui.MaxViewBytes + 1}, "more than the"},
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
			err, _, log := driveGeneration(t, tc.spec, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
			f := failureFrom(t, err)
			if f.Kind != genui.FailInvalidOutput || !strings.Contains(f.Message, tc.want) || f.Hint != genui.HintInvalidOutput {
				t.Fatalf("failure = %+v, want invalid-output containing %q", f, tc.want)
			}
			sentinelKept(t, viewPath, sentinel)
			tempDirGone(t, log)
		})
	}

	err, _, _ := driveGeneration(t, fakeSpec{Out: "not json", Messages: []string{"Done."}}, nil, nil)
	if f := failureFrom(t, err); f.Kind != genui.FailInvalidOutput || f.Detail != "Done." {
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
	job := GenerationJob{
		Req:          genui.GenerateRequest{Text: "hello", Mode: "full", Format: "{}"},
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
	go func() { errCh <- RunGeneration(ctx, job, func(genui.GenerateProgress) error { return nil }) }()
	waitForLog(t, logPath, "session/prompt")
	interrupt(cancel)

	select {
	case err = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("RunGeneration never returned")
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
	if f := failureFrom(t, err); f.Kind != genui.FailCancelled || f.Message != "Generation was cancelled." {
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
	job := GenerationJob{
		Req:          genui.GenerateRequest{Text: "hello", Mode: "full", Format: "{}"},
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
	go func() { errCh <- RunGeneration(ctx, job, func(genui.GenerateProgress) error { return nil }) }()
	waitForLog(t, logPath, "session/prompt")

	started := time.Now()
	cancel()
	var runErr error
	select {
	case runErr = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("RunGeneration never returned after cancellation")
	}
	elapsed := time.Since(started)

	if elapsed < 100*time.Millisecond {
		t.Errorf("the run ended %s after cancel, before the agent answered the turn; cancellation must wait for the prompt-turn answer", elapsed)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("cancellation took %s; an acknowledged cancel must complete on the prompt-turn answer, not wait for the process to exit", elapsed)
	}
	if f := failureFrom(t, runErr); f.Kind != genui.FailCancelled || f.Message != "Generation was cancelled." {
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
	agent, err := NewAgent(cmd, fsutil.NewLimitBuffer(4<<10))
	if err != nil {
		t.Fatal(err)
	}
	agent.Start()
	t.Cleanup(func() {
		killFakeDescendants(t, logPath)
		agent.Kill()
	})

	if err := agent.Call("initialize", map[string]any{"protocolVersion": 1}, nil); err != nil {
		t.Fatal(err)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	if err := agent.Call("session/new", map[string]any{}, &opened); err != nil {
		t.Fatal(err)
	}
	promptDone := make(chan error, 1)
	go func() {
		promptDone <- agent.Call("session/prompt", map[string]any{"sessionId": opened.SessionID}, nil)
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
	agent.CancelTurn(opened.SessionID)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Errorf("cancelTurn waited %s for a blocked write", elapsed)
	}
	killDone := make(chan struct{})
	go func() {
		agent.Kill()
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
	agent, err := NewAgent(cmd, fsutil.NewLimitBuffer(4<<10))
	if err != nil {
		t.Fatal(err)
	}
	agent.Start()
	t.Cleanup(agent.Kill)
	if err := agent.Call("initialize", map[string]any{"protocolVersion": 1}, nil); err != nil {
		t.Fatal(err)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	if err := agent.Call("session/new", map[string]any{}, &opened); err != nil {
		t.Fatal(err)
	}
	var turned struct {
		StopReason string `json:"stopReason"`
	}
	if err := agent.Call("session/prompt", map[string]any{"sessionId": opened.SessionID}, &turned); err != nil {
		t.Fatalf("final prompt response was lost on process exit: %v", err)
	}
	if turned.StopReason != "end_turn" {
		t.Errorf("stop reason = %q, want end_turn", turned.StopReason)
	}
}

func TestGenerateTimeout(t *testing.T) {
	err, _ := hangingRun(t, 300*time.Millisecond, func(context.CancelFunc) {})
	f := failureFrom(t, err)
	if f.Kind != genui.FailTimeout || f.Message != "Generation timed out after 300ms." || f.Hint != genui.HintTimeout {
		t.Errorf("failure = %+v, want the timeout", f)
	}
	if got := genui.TimedOut(defaultGenerationTimeout).Message; got != "Generation timed out after 5 minutes." {
		t.Errorf("default timeout message = %q", got)
	}
}

func TestReportFailure(t *testing.T) {
	var steps []genui.GenerateProgress
	collect := func(p genui.GenerateProgress) error {
		steps = append(steps, p)
		return errors.New("the page is gone")
	}
	err := genui.ReportFailure(genui.NewFailure(genui.FailBusy, "A view is already being generated.", genui.HintBusy), collect)
	if err.Error() != "A view is already being generated. Wait for it to finish, or cancel it." {
		t.Errorf("error = %q", err)
	}
	err = genui.ReportFailure(errors.New("no window"), collect)
	if err.Error() != "no window "+genui.HintAgentError {
		t.Errorf("error = %q", err)
	}
	if len(steps) != 2 || steps[0].Phase != "failed" || steps[0].Text != "A view is already being generated." ||
		steps[0].Failure == nil || steps[0].Failure.Kind != genui.FailBusy || steps[1].Failure == nil || steps[1].Failure.Kind != genui.FailAgentError {
		t.Errorf("steps = %+v", steps)
	}
}

type fakeModelUpdater struct {
	models []string
}

func (f *fakeModelUpdater) SetModels(models []string) error {
	f.models = append([]string(nil), models...)
	return nil
}

func TestGenerateCachesModelsFromHandshake(t *testing.T) {
	up := &fakeModelUpdater{}
	err, _, _ := driveGeneration(t, fakeSpec{
		Models: []string{"model-alpha", "model-beta"},
		Out:    happyOut,
	}, nil, func(job *GenerationJob) {
		job.Model = "model-alpha"
		job.Models = up
	})
	if err != nil {
		t.Fatalf("driveGeneration: %v", err)
	}
	if !reflect.DeepEqual(up.models, []string{"model-alpha", "model-beta"}) {
		t.Errorf("cached models after handshake = %+v, want [model-alpha model-beta]", up.models)
	}
}

func TestGenerateCommandNotFound(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{}, nil, func(job *GenerationJob) {
		job.AgentCommand = "definitely-not-an-agent-cmd"
	})
	if f := failureFrom(t, err); f.Kind != genui.FailAgentNotFound || f.Message != `Could not find the agent command "definitely-not-an-agent-cmd".` || f.Hint != genui.HintAgentNotFound {
		t.Errorf("failure = %+v, want agent-not-found", f)
	}

	missing := filepath.Join(t.TempDir(), "agy")
	err, _, _ = driveGeneration(t, fakeSpec{}, nil, func(job *GenerationJob) { job.AgentCommand = missing })
	if f := failureFrom(t, err); f.Kind != genui.FailAgentNotFound || !strings.HasPrefix(f.Message, fmt.Sprintf("Could not start the agent command %q:", missing)) {
		t.Errorf("failure = %+v, want agent-not-found for the full path", f)
	}
}

func TestGenerateBadMode(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{}, nil, func(job *GenerationJob) { job.Req.Mode = "sections" })
	if f := failureFrom(t, err); f.Kind != genui.FailAgentError || !strings.Contains(f.Message, `only "full" and "update" are supported`) {
		t.Errorf("failure = %+v, want the mode error", f)
	}
}

func TestGenerateAgentCrash(t *testing.T) {
	err, _, _ := driveGeneration(t, fakeSpec{Crash: "exploded on start"}, nil, nil)
	f := failureFrom(t, err)
	if f.Kind != genui.FailAgentError || !strings.Contains(f.Message, "The agent exited before answering initialize.") || !strings.Contains(f.Message, "exploded on start") || f.Hint != genui.HintAgentError {
		t.Errorf("failure = %+v, want agent-error with the agent's stderr", f)
	}
}

func TestGenerateCapturesModelWhenSettingsChange(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
		job.Model = "gemini-3.8-flash"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
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
		err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
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
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
		job.ViewPath = viewPath
		job.BeforeWrite = func() error {
			return genui.NewFailure(genui.FailAgentError, "The document's path changed while the agent worked, so the view was not written.", "Generate again.")
		}
	})
	f := failureFrom(t, err)
	if f.Kind != genui.FailAgentError || !strings.Contains(f.Message, "path changed") {
		t.Errorf("failure = %+v, want path-changed", f)
	}
	sentinelKept(t, viewPath, sentinel)
	tempDirGone(t, log)

	viewPath, sentinel = writeSentinelView(t)
	err, _, log = driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
		job.ViewPath = viewPath
		job.BeforeWrite = func() error { return genui.NewFailure(genui.FailCancelled, "Generation was cancelled.", "") }
	})
	f = failureFrom(t, err)
	if f.Kind != genui.FailCancelled {
		t.Errorf("failure = %+v, want cancelled for abandoned close", f)
	}
	sentinelKept(t, viewPath, sentinel)
	tempDirGone(t, log)
}

func fakeAgentJob(t *testing.T, spec fakeSpec) (func(*GenerationJob), string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fake.log")
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return func(job *GenerationJob) {
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
