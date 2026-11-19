package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResearchSession(t *testing.T) {
	if os.Getenv("MGO_FIXTURE_RESEARCH") == "1" {
		runResearchFixture()
		os.Exit(0)
	}
	root := t.TempDir()
	h := newResearchHost(&documentHost{settings: &settingsStore{dir: t.TempDir()}})
	defer h.close()
	h.command = func(_, work string) (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestResearchSession$")
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "MGO_FIXTURE_RESEARCH=1")
		return cmd, nil
	}
	events := make(chan ResearchUpdate, 32)
	connect := func() {
		t.Helper()
		if err := h.connect(&workspace{root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(kind string) ResearchUpdate {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case e := <-events:
				if e.Kind == kind {
					return e
				}
			case <-timeout:
				t.Fatalf("Missing %s", kind)
			}
		}
	}
	ctx := context.Background()
	for _, command := range []string{"/new", "/quit", "/exit"} {
		if _, err := h.Prompt(ctx, root, command, "", ""); err == nil || !strings.Contains(err.Error(), "Conversation history") && !strings.Contains(err.Error(), "Plugins") {
			t.Fatalf("%s should point to its workspace control, got %v", command, err)
		}
	}
	connect()
	if cmds := wait("commands").Commands; len(cmds) != 1 || cmds[0].Name != "lit" {
		t.Fatal(cmds)
	}
	if err := h.SetConfig(ctx, root, "model", "missing"); err == nil {
		t.Fatal("unadvertised model accepted")
	}
	if err := h.SetConfig(ctx, root, "model", "other"); err != nil {
		t.Fatal(err)
	}
	for {
		config := wait("config").Config
		if len(config) == 1 && config[0].CurrentValue == "other" {
			break
		}
	}
	done := make(chan error, 1)
	go func() { _, err := h.Prompt(ctx, root, "/lit fixture", "", ""); done <- err }()
	ask := wait("permission")
	if err := h.SetConfig(ctx, root, "model", "initial"); err == nil {
		t.Fatal("changed model mid-turn")
	}
	if !strings.Contains(ask.Text, "Save the fixture?") {
		t.Fatalf("missing confirmation detail: %q", ask.Text)
	}
	if err := h.Answer(ctx, root, ask.RequestID, "no-such-option"); err == nil {
		t.Fatal("unknown option accepted")
	}
	if err := h.Answer(ctx, root, ask.RequestID, "yes"); err != nil {
		t.Fatal(err)
	}
	if text := wait("assistant").Text; text != "saved" {
		t.Fatal(text)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	connect()
	if text := wait("assistant").Text; text != "replayed" {
		t.Fatal(text)
	}
	go func() { _, err := h.Prompt(ctx, root, "cancel this", "", ""); done <- err }()
	wait("permission")
	if err := h.Cancel(ctx, root); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled prompt succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled permission hung")
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(filepath.Dir(researchWorkDir(h.dataDir, root)), "session")
	if err := os.WriteFile(record, []byte("removed"), 0600); err != nil {
		t.Fatal(err)
	}
	connect()
	if e := wait("notice"); e.Text == "" {
		t.Fatal("no session recovery notice")
	}
	if cmds := wait("commands").Commands; len(cmds) != 1 {
		t.Fatal(cmds)
	}

	go func() { done <- h.SetConfig(ctx, root, "model", "hold") }()
	if wait("assistant").Text != "Changing model" {
		t.Fatal("did not start config change")
	}
	if _, err := h.Prompt(ctx, root, "overlap", "", ""); err == nil {
		t.Fatal("allowed prompt while model was changing")
	}
	if err := h.SelectSession(ctx, root, ""); err == nil {
		t.Fatal("allowed session switch while model was changing")
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disconnected model change reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("disconnected model change did not drain")
	}
}

func runResearchFixture() {
	encoder := json.NewEncoder(os.Stdout)
	reply := func(id int, result any) {
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	update := func(v any) {
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "fixture", "update": v}})
	}
	config := func(value string) []ResearchConfig {
		return []ResearchConfig{{ID: "model", Name: "Model", Type: "select", Category: "model", CurrentValue: value, Options: []ResearchConfigValue{{Value: "initial", Name: "First model"}, {Value: "other", Name: "Another model"}, {Value: "hold", Name: "Slow model"}}}}
	}
	msg := func(text string) {
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}})
	}
	sc := bufio.NewScanner(os.Stdin)
	turn := 0
	finishOnDismiss := false
	for sc.Scan() {
		var r struct {
			ID     int
			Method string
			Params json.RawMessage
			Result json.RawMessage
		}
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		switch r.Method {
		case "initialize":
			reply(r.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true}})
		case "session/new", "session/load":
			var p struct {
				Cwd       string
				SessionID string
			}
			_ = json.Unmarshal(r.Params, &p)
			cwd, _ := os.Getwd()
			if !sameRoot(p.Cwd, cwd) || filepath.Base(cwd) != "work" {
				os.Exit(3)
			}
			if r.Method == "session/load" {
				if p.SessionID != "fixture" {
					_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32602, "message": "Unknown sessionId: " + p.SessionID}})
					continue
				}
				msg("replayed")
			}
			reply(r.ID, map[string]any{"sessionId": "fixture", "configOptions": config("initial")})
			update(map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []map[string]string{{"name": "lit"}}})
		case "session/set_config_option":
			var p struct{ SessionID, ConfigID, Value string }
			if json.Unmarshal(r.Params, &p) != nil || p.SessionID != "fixture" || p.ConfigID != "model" || (p.Value != "other" && p.Value != "hold") {
				os.Exit(4)
			}
			if p.Value == "hold" {
				msg("Changing model")
				continue
			}
			reply(r.ID, map[string]any{"configOptions": config(p.Value)})
		case "session/prompt":
			turn = r.ID
			finishOnDismiss = strings.Contains(string(r.Params), "Dismiss extension choice")
			if strings.Contains(string(r.Params), "Exercise extension forms") {
				msg("Extension ready")
				continue
			}
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 29, "method": "session/request_permission", "params": map[string]any{"sessionId": "fixture", "toolCall": map[string]any{"title": "Apply?", "rawInput": map[string]string{"message": "Save the fixture?"}}, "options": []map[string]string{{"name": "Accept", "kind": "allow_once", "optionId": "yes"}}}})
		case "session/cancel":
			reply(turn, map[string]string{"stopReason": "cancelled"})
		case "":
			if r.ID == 29 && strings.Contains(string(r.Result), "\"yes\"") {
				msg("saved")
				reply(turn, map[string]string{"stopReason": "end_turn"})
			}
			if r.ID == 29 && finishOnDismiss && strings.Contains(string(r.Result), "\"cancelled\"") {
				reply(turn, map[string]string{"stopReason": "end_turn"})
			}
		}
	}
}
