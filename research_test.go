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
	connect()
	if cmds := wait("commands").Commands; len(cmds) != 1 || cmds[0] != "lit" {
		t.Fatal(cmds)
	}
	done := make(chan error, 1)
	go func() { _, err := h.Prompt(ctx, root, "/lit fixture", "", ""); done <- err }()
	ask := wait("permission")
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
}

func runResearchFixture() {
	encoder := json.NewEncoder(os.Stdout)
	reply := func(id int, result any) {
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	update := func(v any) {
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "fixture", "update": v}})
	}
	msg := func(text string) {
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}})
	}
	sc := bufio.NewScanner(os.Stdin)
	turn := 0
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
			reply(r.ID, map[string]any{"sessionId": "fixture"})
			update(map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []map[string]string{{"name": "lit"}}})
		case "session/prompt":
			turn = r.ID
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 29, "method": "session/request_permission", "params": map[string]any{"sessionId": "fixture", "toolCall": map[string]any{"title": "Apply?", "rawInput": map[string]string{"message": "Save the fixture?"}}, "options": []map[string]string{{"name": "Accept", "kind": "allow_once", "optionId": "yes"}}}})
		case "session/cancel":
			reply(turn, map[string]string{"stopReason": "cancelled"})
		case "":
			if r.ID == 29 && strings.Contains(string(r.Result), "\"yes\"") {
				msg("saved")
				reply(turn, map[string]string{"stopReason": "end_turn"})
			}
		}
	}
}
