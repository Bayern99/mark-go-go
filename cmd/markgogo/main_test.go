package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/ipc"
)

// fakeApp serves the socket protocol so the CLI can be driven without the GUI.
type fakeApp struct {
	t       *testing.T
	path    string
	refuse  *ipc.AppError
	handler func(method string, params map[string]any) map[string]any
}

func startFakeApp(t *testing.T, handler func(method string, params map[string]any) map[string]any) *fakeApp {
	t.Helper()
	dir, err := os.MkdirTemp("", "mgcli")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv(ipc.UserDataEnv, dir)
	path := ipc.SocketPath(dir)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	app := &fakeApp{t: t, path: path, handler: handler}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go app.serve(conn)
		}
	}()
	return app
}

func (a *fakeApp) serve(conn net.Conn) {
	defer conn.Close()
	in := bufio.NewReader(conn)
	if _, err := ipc.ReadMessage(in); err != nil {
		return
	}
	reply := ipc.HelloReply{
		OK: true, Root: "/tmp/workspace", AppVersion: appversion.Version,
		ProtocolVersions: []string{ipc.ModernProtocolVersion, ipc.LegacyProtocolVersion},
	}
	if a.refuse != nil {
		reply = ipc.HelloReply{OK: false, Error: a.refuse}
	}
	if err := ipc.WriteMessage(conn, reply); err != nil || !reply.OK {
		return
	}
	for {
		line, err := ipc.ReadMessage(in)
		if err != nil {
			return
		}
		var msg struct {
			ID     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			return
		}
		if err := ipc.WriteMessage(conn, a.handler(msg.Method, msg.Params)); err != nil {
			return
		}
	}
}

func result(method string, payload any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": 1, "result": payload}
}

func runCLI(t *testing.T, stdin io.Reader, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runWith(stdin, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func decodeJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return out
}

func TestGenUIFormatNeedsNoApp(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, t.TempDir())
	code, stdout, stderr := runCLI(t, nil, "genui", "format")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"- Col(children: Node[]", "- @Format("} {
		if !strings.Contains(stdout, want) {
			t.Errorf("format lacks %q", want)
		}
	}
}

func TestReadAndOutlineWorkWithoutTheApp(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, filepath.Join(t.TempDir(), "absent"))
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	text := "# Report\n\n## Summary\n\nGenerated findings.\n\n<!-- note: Check the numbers. -->\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCLI(t, nil, "read", path)
	if code != exitOK {
		t.Fatalf("read exit = %d, stderr = %s", code, stderr)
	}
	out := decodeJSON(t, stdout)
	if out["text"] != text || out["source"] != "disk" || out["endLine"] != float64(7) {
		t.Fatalf("read output = %v", out)
	}

	code, stdout, stderr = runCLI(t, nil, "read", path, "--sections")
	if code != exitOK {
		t.Fatalf("read --sections exit = %d, stderr = %s", code, stderr)
	}
	sections, _ := decodeJSON(t, stdout)["sections"].([]any)
	if len(sections) != 2 {
		t.Fatalf("sections = %v", sections)
	}
	first := sections[0].(map[string]any)
	if first["heading"] != "Report" || !strings.HasPrefix(first["id"].(string), "fnv1a32:") {
		t.Fatalf("first section = %v", first)
	}

	code, stdout, stderr = runCLI(t, nil, "read", path, "--notes")
	if code != exitOK {
		t.Fatalf("read --notes exit = %d, stderr = %s", code, stderr)
	}
	notes, _ := decodeJSON(t, stdout)["notes"].([]any)
	if len(notes) != 1 || !strings.Contains(notes[0].(map[string]any)["text"].(string), "Check the numbers.") {
		t.Fatalf("notes = %v", notes)
	}

	code, stdout, stderr = runCLI(t, nil, "read", path, "--lines", "3:3")
	if code != exitOK {
		t.Fatalf("read --lines exit = %d, stderr = %s", code, stderr)
	}
	if out := decodeJSON(t, stdout); out["text"] != "## Summary" {
		t.Fatalf("line read = %v", out)
	}
}

func TestWritesReportAppNotRunning(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, filepath.Join(t.TempDir(), "absent"))
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	if err := os.WriteFile(path, []byte("# Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCLI(t, nil, "propose", path, "--old", "# Report", "--new", "# Renamed")
	if code != exitAppNotRunning {
		t.Fatalf("exit = %d, want %d (stdout %s)", code, exitAppNotRunning, stdout)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing", stdout)
	}
	failure := decodeJSON(t, stderr)["error"].(map[string]any)
	if failure["code"] != ipc.CodeAppNotRunning || !strings.Contains(failure["hint"].(string), "Start the Mark Go Go app") {
		t.Fatalf("failure = %v", failure)
	}
}

func TestVersionAndUsage(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, t.TempDir())
	code, stdout, _ := runCLI(t, nil, "--version")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if out := decodeJSON(t, stdout); out["version"] != appversion.Version || out["name"] != "markgogo" {
		t.Fatalf("version output = %v", out)
	}
	if code, _, stderr := runCLI(t, nil, "bogus"); code != exitUsage || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("unknown command: exit %d, stderr %s", code, stderr)
	}
}

func TestStatusReportsTheRunningApp(t *testing.T) {
	startFakeApp(t, func(method string, _ map[string]any) map[string]any {
		if method != "server/discover" {
			t.Errorf("status asked for %q", method)
		}
		return result(method, map[string]any{
			"resultType":        "complete",
			"supportedVersions": []string{ipc.ModernProtocolVersion, ipc.LegacyProtocolVersion},
			"instructions":      "read, propose, write",
		})
	})
	code, stdout, stderr := runCLI(t, nil, "status")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	out := decodeJSON(t, stdout)
	if out["running"] != true || out["root"] != "/tmp/workspace" || out["instructions"] != "read, propose, write" {
		t.Fatalf("status = %v", out)
	}
	versions, _ := out["supportedVersions"].([]any)
	if len(versions) != 2 {
		t.Fatalf("supportedVersions = %v", out["supportedVersions"])
	}
}

func TestStatusReportsTheStoppedAppWithoutFailing(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, filepath.Join(t.TempDir(), "absent"))
	code, stdout, stderr := runCLI(t, nil, "status")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if out := decodeJSON(t, stdout); out["running"] != false {
		t.Fatalf("status = %v", out)
	}
}

func TestStatusReportsARunningAppWithoutAMatchingWorkspace(t *testing.T) {
	app := startFakeApp(t, func(method string, _ map[string]any) map[string]any { return result(method, nil) })
	app.refuse = &ipc.AppError{
		Code:    ipc.CodeWorkspaceNotOpen,
		Message: "No open workspace contains this folder.",
		Hint:    "Open the folder in Mark Go Go first.",
	}
	code, stdout, stderr := runCLI(t, nil, "status")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	out := decodeJSON(t, stdout)
	if out["running"] != true {
		t.Fatalf("status = %v", out)
	}
	failure, _ := out["error"].(map[string]any)
	if failure["code"] != ipc.CodeWorkspaceNotOpen {
		t.Fatalf("status error = %v", out["error"])
	}
}

func TestToolErrorCodesBecomeExitCodes(t *testing.T) {
	startFakeApp(t, func(method string, params map[string]any) map[string]any {
		if method != "tools/call" {
			t.Fatalf("unexpected method %q", method)
		}
		return result(method, map[string]any{
			"content":           []map[string]any{{"type": "text", "text": "No document matches that id or path."}},
			"structuredContent": map[string]any{"code": ipc.CodeNotFound, "message": "No document matches that id or path.", "hint": "Call get_workspace."},
			"isError":           true,
		})
	})
	code, stdout, stderr := runCLI(t, nil, "read", "report.md")
	if code != exitNotFound {
		t.Fatalf("exit = %d, want %d (stdout %s stderr %s)", code, exitNotFound, stdout, stderr)
	}
	failure := decodeJSON(t, stderr)["error"].(map[string]any)
	if failure["code"] != ipc.CodeNotFound {
		t.Fatalf("failure = %v", failure)
	}
}

func TestReadAndProposeReachTheApp(t *testing.T) {
	var got []string
	startFakeApp(t, func(method string, params map[string]any) map[string]any {
		if method == "tools/call" {
			name, _ := params["name"].(string)
			got = append(got, name)
			args, _ := params["arguments"].(map[string]any)
			if name == "propose_edit" {
				if _, ok := args["source"]; ok {
					t.Error("the CLI let the caller set a proposal source")
				}
				return result(method, map[string]any{
					"content":           []map[string]any{{"type": "text", "text": "{}"}},
					"structuredContent": map[string]any{"proposalId": "p-1", "revision": 2, "hunksCount": 1, "conflicted": false, "summary": "Tighten"},
				})
			}
			return result(method, map[string]any{
				"content":           []map[string]any{{"type": "text", "text": "{}"}},
				"structuredContent": map[string]any{"text": "# Report", "version": 7, "path": "/tmp/workspace/report.md"},
			})
		}
		return result(method, map[string]any{})
	})

	code, stdout, stderr := runCLI(t, nil, "read", "report.md")
	if code != exitOK {
		t.Fatalf("read exit = %d, stderr = %s", code, stderr)
	}
	if out := decodeJSON(t, stdout); out["version"] != float64(7) || out["text"] != "# Report" {
		t.Fatalf("read = %v", out)
	}

	code, stdout, stderr = runCLI(t, nil, "propose", "report.md", "--old", "a", "--new", "b", "--summary", "Tighten")
	if code != exitOK {
		t.Fatalf("propose exit = %d, stderr = %s", code, stderr)
	}
	if out := decodeJSON(t, stdout); out["proposalId"] != "p-1" || out["hunksCount"] != float64(1) {
		t.Fatalf("propose = %v", out)
	}
	if len(got) != 2 || got[0] != "read_document" || got[1] != "propose_edit" {
		t.Fatalf("app saw %v", got)
	}
}

// syncBuffer collects the shim's output while the test reads it.
type syncBuffer struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}

func TestMCPCommandPipesStdioThroughTheShim(t *testing.T) {
	startFakeApp(t, func(method string, _ map[string]any) map[string]any {
		return result(method, map[string]any{"tools": []any{}})
	})
	stdin, stdinWriter := io.Pipe()
	defer stdin.Close()
	stdout := &syncBuffer{}
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- runWith(stdin, []string{"mcp"}, stdout, &stderr) }()
	if _, err := io.WriteString(stdinWriter, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for stdout.String() == "" && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	_ = stdinWriter.Close()
	if code := <-done; code != exitOK {
		t.Fatalf("mcp exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id":1`) || !strings.Contains(stdout.String(), `"result"`) {
		t.Fatalf("mcp stdout = %q", stdout.String())
	}
}

func TestEveryErrorCodeHasItsOwnExitCode(t *testing.T) {
	cases := map[string]int{
		ipc.CodeInvalidArgs:      exitUsage,
		ipc.CodeAppNotRunning:    exitAppNotRunning,
		ipc.CodeWorkspaceNotOpen: exitWorkspaceNotOpen,
		ipc.CodeVersionConflict:  exitVersionConflict,
		ipc.CodeNotFound:         exitNotFound,
		ipc.CodePathDenied:       exitPathDenied,
	}
	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			startFakeApp(t, func(method string, _ map[string]any) map[string]any {
				return result(method, map[string]any{
					"content":           []map[string]any{{"type": "text", "text": code}},
					"structuredContent": map[string]any{"code": code, "message": code, "hint": "hint"},
					"isError":           true,
				})
			})
			if got, _, stderr := runCLI(t, nil, "open", "report.md"); got != want {
				t.Fatalf("%s exited %d, want %d (stderr %s)", code, got, want, stderr)
			}
		})
	}
}

func TestCreatingOpeningCheckpointingAndWritingAViewReachTheirTools(t *testing.T) {
	type call struct {
		tool string
		args map[string]any
	}
	var calls []call
	startFakeApp(t, func(method string, params map[string]any) map[string]any {
		if method == "tools/call" {
			name, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]any)
			calls = append(calls, call{name, args})
			return result(method, map[string]any{
				"content":           []map[string]any{{"type": "text", "text": "{}"}},
				"structuredContent": map[string]any{"ok": true},
			})
		}
		return result(method, map[string]any{})
	})

	if code, _, stderr := runCLI(t, nil, "new", "outputs/notes.md", "--content", "# Notes\n", "--open"); code != exitOK {
		t.Fatalf("new exit = %d, stderr = %s", code, stderr)
	}
	if code, _, stderr := runCLI(t, nil, "open", "outputs/notes.md", "--heading", "Notes"); code != exitOK {
		t.Fatalf("open exit = %d, stderr = %s", code, stderr)
	}
	if code, _, stderr := runCLI(t, nil, "checkpoint", "outputs/notes.md", "--label", "Before review"); code != exitOK {
		t.Fatalf("checkpoint exit = %d, stderr = %s", code, stderr)
	}
	candidate := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(candidate, []byte(`{"a2ui":[{"version":"1.0"}],"request":"Summarize"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCLI(t, nil, "genui", "write", "outputs/notes.md", "--candidate", candidate); code != exitOK {
		t.Fatalf("genui write exit = %d, stderr = %s", code, stderr)
	}

	wantTools := []string{"create_document", "open_document", "create_checkpoint", "write_genui_view"}
	if len(calls) != len(wantTools) {
		t.Fatalf("the app saw %+v", calls)
	}
	for i, want := range wantTools {
		if calls[i].tool != want {
			t.Fatalf("call %d = %q, want %q", i, calls[i].tool, want)
		}
	}
	if calls[0].args["open"] != true || calls[0].args["path"] != "outputs/notes.md" {
		t.Errorf("new arguments = %v", calls[0].args)
	}
	if calls[1].args["heading"] != "Notes" {
		t.Errorf("open arguments = %v", calls[1].args)
	}
	if calls[2].args["label"] != "Before review" {
		t.Errorf("checkpoint arguments = %v", calls[2].args)
	}
	if calls[3].args["request"] != "Summarize" || calls[3].args["a2ui"] == nil {
		t.Errorf("genui write arguments = %v", calls[3].args)
	}
}

func TestProposeAcceptsWholeTextFromStdin(t *testing.T) {
	var got map[string]any
	startFakeApp(t, func(method string, params map[string]any) map[string]any {
		if method == "tools/call" {
			got, _ = params["arguments"].(map[string]any)
			return result(method, map[string]any{
				"content":           []map[string]any{{"type": "text", "text": "{}"}},
				"structuredContent": map[string]any{"proposalId": "p-2"},
			})
		}
		return result(method, map[string]any{})
	})
	code, _, stderr := runCLI(t, strings.NewReader("# Whole\n"), "propose", "report.md", "--from", "-", "--summary", "Rewrite")
	if code != exitOK {
		t.Fatalf("propose exit = %d, stderr = %s", code, stderr)
	}
	if got["proposedText"] != "# Whole\n" || got["summary"] != "Rewrite" {
		t.Fatalf("arguments = %v", got)
	}
}

func TestAgentPromptMentionsTheChannelAndTheReviewFlow(t *testing.T) {
	t.Setenv(ipc.UserDataEnv, t.TempDir())
	code, stdout, stderr := runCLI(t, nil, "agent-prompt")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"ipc.sock", "propose_edit", "read_document", "VERSION_CONFLICT", "markgogo mcp"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("agent prompt lacks %q", want)
		}
	}
}
