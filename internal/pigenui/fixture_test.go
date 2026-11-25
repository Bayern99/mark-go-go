package pigenui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/pirpc"
)

// The fake Pi in this file is the test binary itself: TestMain sees
// MGO_FIXTURE_PI and behaves like `pi`. Every test writes its expectations into
// the log the fixture appends to, so a test can prove what the app actually
// launched and which commands it sent.

const fixtureLogEnv = "MGO_FIXTURE_LOG"

type fixtureRecord map[string]any

func appendFixtureLog(record fixtureRecord) {
	path := os.Getenv(fixtureLogEnv)
	if path == "" {
		return
	}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = file.Write(append(data, '\n'))
	_ = file.Close()
}

func runPiFixture() {
	args := os.Args[1:]
	cwd, _ := os.Getwd()
	appendFixtureLog(fixtureRecord{
		"event": "start",
		"pid":   os.Getpid(),
		"args":  args,
		"cwd":   cwd,
		"guard": map[string]string{
			"root":    os.Getenv("MGO_GUARD_ROOT"),
			"out":     os.Getenv("MGO_GUARD_OUT"),
			"token":   os.Getenv("MGO_GUARD_TOKEN"),
			"version": os.Getenv("MGO_GUARD_VERSION"),
		},
	})
	if slices.Contains(args, "auth") {
		runFixtureAuthCheck(args)
		return
	}
	if os.Getenv("MGO_FIXTURE_BROKEN_GUARD") == "1" {
		// What Pi itself does when an explicit -e fails to load: it says why on
		// stderr and exits before any session starts.
		_, _ = os.Stderr.WriteString(`Error: Failed to load extension: ParseError: Unexpected token`)
		os.Exit(1)
	}
	defer appendFixtureLog(fixtureRecord{"event": "exit", "pid": os.Getpid()})
	runFixtureRPC()
}

func runFixtureAuthCheck(args []string) {
	status := os.Getenv("MGO_FIXTURE_AUTH_STATUS")
	if status == "" {
		status = "ready"
	}
	code := 0
	if raw := os.Getenv("MGO_FIXTURE_AUTH_EXIT"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			code = parsed
		}
	}
	provider := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--provider" {
			provider = args[i+1]
		}
	}
	answer := map[string]any{"status": status, "provider": provider}
	if reason := os.Getenv("MGO_FIXTURE_AUTH_REASON"); reason != "" {
		answer["reason"] = reason
	}
	if code == 2 && status == "ready" {
		answer["status"] = "invalid"
	}
	if code != 0 && status == "ready" {
		answer["status"] = "not_ready"
	}
	data, _ := json.Marshal(answer)
	_, _ = os.Stdout.Write(append(data, '\n'))
	if code != 0 {
		os.Exit(code)
	}
	os.Exit(0)
}

func runFixtureRPC() {
	send := func(record map[string]any) {
		data, err := json.Marshal(record)
		if err != nil {
			return
		}
		_, _ = os.Stdout.Write(append(data, '\n'))
	}
	if os.Getenv("MGO_FIXTURE_NO_GUARD") != "1" {
		status := os.Getenv("MGO_FIXTURE_GUARD_STATUS")
		if status == "" {
			status = "ready:{version}:{token}"
		}
		status = strings.NewReplacer(
			"{version}", os.Getenv("MGO_GUARD_VERSION"),
			"{token}", os.Getenv("MGO_GUARD_TOKEN"),
		).Replace(status)
		send(map[string]any{
			"type": "extension_ui_request", "id": "status-1", "method": "setStatus",
			"statusKey": "mgo-guard", "statusText": status,
		})
	}

	models := []map[string]any{
		{"id": "claude-sonnet-4-5", "name": "Claude Sonnet 4.5", "provider": "anthropic"},
		{"id": "gpt-5.4", "name": "GPT-5.4", "provider": "openai"},
	}
	if raw := os.Getenv("MGO_FIXTURE_MODELS"); raw != "" {
		models = nil
		_ = json.Unmarshal([]byte(raw), &models)
	}
	levels := []string{"off", "low", "high", "max"}
	if raw := os.Getenv("MGO_FIXTURE_LEVELS"); raw != "" {
		levels = strings.Split(raw, ",")
	}
	currentModel := ""
	if len(models) > 0 {
		currentModel, _ = models[0]["id"].(string)
	}
	ignoreAbort := os.Getenv("MGO_FIXTURE_IGNORE_ABORT") == "1"
	dialog := os.Getenv("MGO_FIXTURE_DIALOG") == "1"

	respond := func(id, command string, data any) {
		answer := map[string]any{"id": id, "type": "response", "command": command, "success": true}
		if data != nil {
			answer["data"] = data
		}
		send(answer)
	}

	prompted := false
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64<<10), pirpc.MaxRecordBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		command, _ := record["type"].(string)
		id, _ := record["id"].(string)
		appendFixtureLog(fixtureRecord{"event": "command", "type": command, "id": id})
		switch command {
		case "extension_ui_response":
			appendFixtureLog(fixtureRecord{"event": "uiResponse", "id": id, "cancelled": record["cancelled"]})
		case "get_available_models":
			respond(id, command, map[string]any{"models": models})
		case "get_state":
			respond(id, command, map[string]any{
				"model":               map[string]any{"id": currentModel, "provider": "anthropic"},
				"thinkingLevel":       "low",
				"isStreaming":         prompted,
				"sessionId":           "fixture",
				"messageCount":        0,
				"pendingMessageCount": 0,
			})
		case "set_model":
			if id == "" {
				continue
			}
			provider, _ := record["provider"].(string)
			modelID, _ := record["modelId"].(string)
			appendFixtureLog(fixtureRecord{"event": "setModel", "provider": provider, "model": modelID})
			currentModel = modelID
			respond(id, command, map[string]any{"id": modelID, "provider": provider})
		case "get_available_thinking_levels":
			respond(id, command, map[string]any{"levels": levels})
		case "set_thinking_level":
			respond(id, command, nil)
		case "prompt":
			prompted = true
			respond(id, command, map[string]any{"disposition": "started"})
			if dialog {
				send(map[string]any{
					"type": "extension_ui_request", "id": "dialog-1", "method": "select",
					"title": "Allow this?", "options": []string{"Allow", "Block"},
				})
				continue
			}
			send(map[string]any{
				"type": "message_update",
				"assistantMessageEvent": map[string]any{
					"type": "thinking_delta", "contentIndex": 0, "delta": "Considering the source.",
				},
			})
			if ignoreAbort {
				continue
			}
			send(map[string]any{"type": "tool_execution_start", "toolCallId": "call-1", "toolName": "read", "args": map[string]any{"path": "source.md"}})
			send(map[string]any{"type": "tool_execution_start", "toolCallId": "call-2", "toolName": "write", "args": map[string]any{"path": "out.json"}})
			send(map[string]any{
				"type": "message_end",
				"message": map[string]any{
					"role":       "assistant",
					"content":    []map[string]any{{"type": "text", "text": "Wrote the view."}},
					"stopReason": "stop",
				},
			})
			send(map[string]any{"type": "agent_settled", "aborted": false})
		case "abort":
			if !ignoreAbort {
				respond(id, command, nil)
				send(map[string]any{"type": "agent_settled", "aborted": true})
			}
		}
	}
}

// fixtureLog reads what the fixture recorded, oldest first.
func fixtureLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture log: %v", err)
	}
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("fixture log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func fixtureCommands(records []map[string]any) []string {
	var commands []string
	for _, record := range records {
		if record["event"] == "command" {
			if command, ok := record["type"].(string); ok {
				commands = append(commands, command)
			}
		}
	}
	return commands
}

func fixtureStarts(records []map[string]any) []map[string]any {
	var starts []map[string]any
	for _, record := range records {
		if record["event"] == "start" {
			starts = append(starts, record)
		}
	}
	return starts
}

func fixtureStrings(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		if text, ok := entry.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func describe(commands []string) string { return fmt.Sprint(commands) }
