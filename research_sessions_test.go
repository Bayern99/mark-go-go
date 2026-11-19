package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestResearchConversationsBelongToOneWorkspace(t *testing.T) {
	if os.Getenv("MGO_TEST_CONVERSATIONS") == "1" {
		serveConversationFixture()
		os.Exit(0)
	}
	root, data := t.TempDir(), t.TempDir()
	h := newResearchHost(&documentHost{settings: &settingsStore{dir: data}})
	h.command = func(_, cwd string) (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestResearchConversationsBelongToOneWorkspace$")
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "MGO_TEST_CONVERSATIONS=1")
		return cmd, nil
	}
	defer h.close()
	events := make(chan ResearchUpdate, 80)
	connect := func() {
		t.Helper()
		if err := h.connect(&workspace{root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
			t.Fatal(err)
		}
	}
	connect()
	early := ""
	for len(events) > 0 {
		e := <-events
		if e.Kind == "assistant" {
			early += e.Text
		}
	}
	if early != "Created first" {
		t.Fatalf("initial history: %q", early)
	}
	ctx := context.Background()
	records, err := h.Sessions(ctx, root)
	if err != nil || len(records) != 2 || !records[0].Current || records[1].Title != "Earlier research" {
		t.Fatalf("history: %+v %v", records, err)
	}
	if err := h.SelectSession(ctx, root, "foreign"); err == nil {
		t.Fatal("opened a session from a different workspace")
	}
	if err := h.DeleteSession(ctx, root, "foreign"); err == nil {
		t.Fatal("deleted a session from a different workspace")
	}
	if err := h.DeleteSession(ctx, root, "first"); err == nil {
		t.Fatal("deleted active session")
	}
	if err := h.SelectSession(ctx, root, "older"); err != nil {
		t.Fatal(err)
	}
	history := []string{}
	for len(events) > 0 {
		e := <-events
		if e.Kind == "reset" {
			history = nil
		}
		if e.Kind == "assistant" {
			history = append(history, e.Text)
		}
	}
	if len(history) != 1 || history[0] != "Restored older" {
		t.Fatalf("unexpected replay: %v", history)
	}
	record := filepath.Join(filepath.Dir(researchWorkDir(data, root)), "session")
	id, err := os.ReadFile(record)
	if err != nil || string(id) != "older" {
		t.Fatalf("switch not persisted: %q %v", id, err)
	}
	if err := h.SelectSession(ctx, root, ""); err != nil {
		t.Fatal(err)
	}
	id, _ = os.ReadFile(record)
	if string(id) != "next" {
		t.Fatalf("new session id = %q", id)
	}
	if err := h.DeleteSession(ctx, root, "older"); err != nil {
		t.Fatal(err)
	}
	rows, err := h.Sessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == "older" {
			t.Fatal("deleted session remains")
		}
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	connect()
	for len(events) > 0 {
		<-events
	}
	if err := h.SelectSession(ctx, root, "broken"); err == nil {
		t.Fatal("unavailable session switched successfully")
	}
	id, _ = os.ReadFile(record)
	if string(id) != "next" {
		t.Fatal("failed switch replaced durable session selection")
	}
	for len(events) > 0 {
		e := <-events
		if e.Kind == "reset" || e.Kind == "assistant" || e.Kind == "config" {
			t.Fatalf("failed switch changed visible conversation: %+v", e)
		}
	}
	connect()
	if err := h.SetConfig(ctx, root, "model", "unknown"); err == nil {
		t.Fatal("unavailable config accepted after reconnect")
	}
	work := researchWorkDir(data, root)
	if err := os.WriteFile(filepath.Join(work, "pause-load"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.SelectSession(ctx, root, "broken") }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(work, "loading")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture never received session/load")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := h.Prompt(ctx, root, "overlap", "", ""); err == nil {
		t.Fatal("prompt accepted during restore")
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disconnected restore reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("disconnected restore did not drain")
	}
	id, err = os.ReadFile(record)
	if err != nil || string(id) != "next" {
		t.Fatalf("lost previous selection: %q %v", id, err)
	}
	connect()

}

func serveConversationFixture() {
	reader, writer := bufio.NewScanner(os.Stdin), json.NewEncoder(os.Stdout)
	respond := func(id int, value any) {
		_ = writer.Encode(map[string]any{"id": id, "jsonrpc": "2.0", "result": value})
	}
	replay := func(id, text string) {
		_ = writer.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": id, "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}}}})
	}
	first, deleted := true, false
	for reader.Scan() {
		var req struct {
			ID     int
			Method string
			Params struct{ Cwd, Cursor, SessionID string }
		}
		if json.Unmarshal(reader.Bytes(), &req) != nil {
			return
		}
		switch req.Method {
		case "initialize":
			respond(req.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true, "sessionCapabilities": map[string]any{"list": map[string]any{}, "delete": map[string]any{}}}})
		case "session/new":
			id := "next"
			if first {
				id = "first"
				first = false
			}
			replay("foreign", "Leaked before creation")
			replay(id, "Created "+id)
			respond(req.ID, map[string]string{"sessionId": id})
		case "session/load":
			if req.Params.SessionID == "broken" {
				replay("broken", "Unconfirmed broken transcript")
				if _, err := os.Stat(filepath.Join(req.Params.Cwd, "pause-load")); err == nil {
					if err := os.WriteFile(filepath.Join(req.Params.Cwd, "loading"), nil, 0600); err != nil {
						panic(err)
					}
					continue
				}
				_ = writer.Encode(map[string]any{"id": req.ID, "error": map[string]any{"code": -32602, "message": "unavailable"}})
				continue
			}
			replay("first", "Leaked former session")
			replay(req.Params.SessionID, "Restored "+req.Params.SessionID)
			respond(req.ID, map[string]string{})
		case "session/list":
			entries := []map[string]string{}
			row := func(id, title, cwd string) {
				entries = append(entries, map[string]string{"sessionId": id, "cwd": cwd, "title": title})
			}
			if req.Params.Cursor == "" {
				row("first", "First research", req.Params.Cwd)
				row("foreign", "Outside", filepath.Dir(req.Params.Cwd))
				respond(req.ID, map[string]any{"sessions": entries, "nextCursor": "2"})
			} else {
				if !deleted {
					row("older", "Earlier research", req.Params.Cwd)
				}
				if first {
					row("broken", "Broken", req.Params.Cwd)
				}
				respond(req.ID, map[string]any{"sessions": entries})
			}
		case "session/delete":
			if req.Params.SessionID != "older" {
				panic(errors.New("unexpected delete"))
			}
			deleted = true
			respond(req.ID, map[string]any{})
		}
	}
}
