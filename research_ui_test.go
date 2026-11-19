package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestResearchExtensionUI(t *testing.T) {
	root := t.TempDir()
	h := newResearchHost(&documentHost{settings: &settingsStore{dir: t.TempDir()}})
	h.command = func(_, work string) (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestResearchSession$")
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "MGO_FIXTURE_RESEARCH=1")
		return cmd, nil
	}
	events := make(chan ResearchUpdate, 64)
	if err := h.connect(&workspace{root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}
	defer h.close()
	wait := func(kind string) ResearchUpdate {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case e := <-events:
				if e.Kind == kind {
					return e
				}
			case <-deadline:
				t.Fatalf("Missing research UI event %q", kind)
			}
		}
	}
	wait("commands")
	s := h.sessions[canonicalPath(root)]
	send := func(req researchUIRequest) net.Conn {
		t.Helper()
		conn, err := net.Dial("unix", s.ui.listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err := json.NewEncoder(conn).Encode(req); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	reply := func(conn net.Conn) map[string]any {
		t.Helper()
		defer conn.Close()
		var out map[string]any
		if err := json.NewDecoder(conn).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	bad := send(researchUIRequest{Method: "commands", Commands: []ResearchCommand{{Name: "forged"}}})
	if json.NewDecoder(bad).Decode(new(any)) == nil {
		t.Fatal("Accepted unauthenticated extension commands")
	}
	bad.Close()
	req := researchUIRequest{Token: s.ui.token, Method: "commands", Commands: []ResearchCommand{
		{Name: "lit"}, {Name: "tools", Description: "Inspect tool parameters"}, {Name: "new"}, {Name: "quit"}, {Name: "exit"},
	}}
	reply(send(req))
	commands := wait("commands").Commands
	if len(commands) != 2 || commands[0].Name != "lit" || commands[0].Source != "" || commands[1].Name != "tools" || commands[1].Source != "extension" || commands[1].Description == "" {
		t.Fatal(commands)
	}
	req.Commands = []ResearchCommand{{Name: "run"}}
	reply(send(req))
	commands = wait("commands").Commands
	if len(commands) != 2 || !slices.ContainsFunc(commands, func(c ResearchCommand) bool { return c.Name == "run" }) {
		t.Fatal(commands)
	}
	ctx := context.Background()
	if err := h.SelectSession(ctx, root, ""); err != nil {
		t.Fatal(err)
	}
	if commands = wait("commands").Commands; len(commands) != 1 || commands[0].Name != "lit" {
		t.Fatalf("The previous conversation's extensions survived the switch: %v", commands)
	}
	done := make(chan error, 1)
	go func() { _, err := h.Prompt(ctx, root, "Review inputs", "", ""); done <- err }()
	permission := wait("permission")
	input := send(researchUIRequest{Token: s.ui.token, Method: "editor", Title: "Draft", Prefill: "draft text"})
	if err := h.AnswerText(ctx, root, permission.RequestID, "yes", false); err == nil {
		t.Fatal("Text accepted for a permission choice")
	}
	if err := h.Answer(ctx, root, permission.RequestID, "no-such-option"); err == nil {
		t.Fatal("Unknown option was accepted")
	}
	if err := h.Cancel(ctx, root); err != nil {
		t.Fatal(err)
	}
	if out := reply(input); out["cancelled"] != true {
		t.Fatal(out)
	}
	if err := <-done; err == nil {
		t.Fatal("The interrupted prompt completed")
	}

	go func() { _, err := h.Prompt(ctx, root, "Dismiss extension choice", "", ""); done <- err }()
	if err := h.Answer(ctx, root, wait("permission").RequestID, ""); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Dismissing a choice should leave the turn running: %v", err)
	}

	go func() { _, err := h.Prompt(ctx, root, "Exercise extension forms", "", ""); done <- err }()
	wait("assistant")
	input = send(researchUIRequest{Token: s.ui.token, Method: "editor", Title: "Draft", Prefill: "initial text"})
	question := wait("editor")
	if question.Text != "Draft" || question.Prefill != "initial text" {
		t.Fatal(question)
	}
	if err := h.Answer(ctx, root, question.RequestID, "yes"); err == nil {
		t.Fatal("Choice accepted for a free text request")
	}
	if err := h.AnswerText(ctx, root, question.RequestID, "", false); err != nil {
		t.Fatal(err)
	}
	if out := reply(input); out["value"] != "" || out["cancelled"] != nil {
		t.Fatalf("Empty text should be a valid submitted answer: %v", out)
	}
	wait("permissionDone")
	input = send(researchUIRequest{Token: s.ui.token, Method: "input", Title: "Question", Placeholder: "Answer"})
	question = wait("input")
	if question.Placeholder != "Answer" {
		t.Fatal(question)
	}
	input.Close()
	wait("permissionDone")
	if err := h.AnswerText(ctx, root, question.RequestID, "late", false); err == nil {
		t.Fatal("A disconnected requester accepted an answer")
	}
	reply(send(researchUIRequest{Token: s.ui.token, Method: "set_editor_text", Text: "/lit "}))
	if next := wait("editorText"); next.Text != "/lit " {
		t.Fatal(next)
	}
	reply(send(researchUIRequest{Token: s.ui.token, Method: "activity", Key: "status:search", Text: "Reading"}))
	if next := wait("activity"); next.Key != "status:search" || next.Text != "Reading" {
		t.Fatal(next)
	}
	reply(send(researchUIRequest{Token: s.ui.token, Method: "message", Text: "## Agents"}))
	if next := wait("assistant"); next.Text != "## Agents" || next.MessageID == "" {
		t.Fatal(next)
	}
	if err := h.Cancel(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("The interrupted prompt completed")
	}
	if err := h.Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.ui.dir); !os.IsNotExist(err) {
		t.Fatalf("Private research UI directory survived disconnect: %v", err)
	}
}
