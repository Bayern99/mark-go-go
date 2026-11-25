package ipc_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/ipc"
)

type fakeApp struct {
	t    *testing.T
	ln   net.Listener
	path string

	mu        sync.Mutex
	hellos    []ipc.Hello
	handles   []string
	accepts   int
	onConnect func(index int, conn net.Conn, hello ipc.Hello, in *bufio.Reader)
}

// shortTempDir keeps a socket path inside the 104 byte limit Darwin gives Unix
// sockets.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mgipc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func startFakeApp(t *testing.T, onConnect func(index int, conn net.Conn, hello ipc.Hello, in *bufio.Reader)) *fakeApp {
	t.Helper()
	dir := shortTempDir(t)
	path := filepath.Join(dir, ipc.SocketName)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	app := &fakeApp{t: t, ln: ln, path: path, onConnect: onConnect}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			app.mu.Lock()
			index := app.accepts
			app.accepts++
			app.mu.Unlock()
			go app.serve(index, conn)
		}
	}()
	return app
}

func (a *fakeApp) serve(index int, conn net.Conn) {
	defer conn.Close()
	in := bufio.NewReader(conn)
	line, err := ipc.ReadMessage(in)
	if err != nil {
		return
	}
	var hello ipc.Hello
	if err := json.Unmarshal(line, &hello); err != nil {
		a.t.Errorf("hello frame: %v", err)
		return
	}
	a.mu.Lock()
	a.hellos = append(a.hellos, hello)
	a.mu.Unlock()
	if err := ipc.WriteMessage(conn, ipc.HelloReply{OK: true, Root: hello.CWD, AppVersion: "0.5.0"}); err != nil {
		return
	}
	if a.onConnect != nil {
		a.onConnect(index, conn, hello, in)
	}
}

func (a *fakeApp) recorded() (hellos []ipc.Hello, handles []string, accepts int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]ipc.Hello(nil), a.hellos...), append([]string(nil), a.handles...), a.accepts
}

func (a *fakeApp) recordHandle(method string) {
	a.mu.Lock()
	a.handles = append(a.handles, method)
	a.mu.Unlock()
}

func readRequest(t *testing.T, in *bufio.Reader) map[string]any {
	t.Helper()
	line, err := ipc.ReadMessage(in)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(line, &msg); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	return msg
}

func TestUserDataDirFollowsTheEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ipc.UserDataEnv, dir)
	got, err := ipc.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("UserDataDir() = %q, want %q", got, dir)
	}
	if socket := ipc.SocketPath(got); socket != filepath.Join(dir, ipc.SocketName) {
		t.Fatalf("SocketPath() = %q", socket)
	}
	t.Setenv(ipc.UserDataEnv, "")
	got, err = ipc.UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != ipc.AppName {
		t.Fatalf("UserDataDir() = %q, want a %q directory", got, ipc.AppName)
	}
}

func TestClientAnnouncesItsCanonicalWorkingDirectoryAndProtocolVersion(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "work")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	var app *fakeApp
	app = startFakeApp(t, func(_ int, conn net.Conn, _ ipc.Hello, in *bufio.Reader) {
		msg := readRequest(t, in)
		app.recordHandle(msg["method"].(string))
		params, _ := msg["params"].(map[string]any)
		meta, _ := params["_meta"].(map[string]any)
		if meta[ipc.MetaProtocolVersion] != ipc.ModernProtocolVersion {
			t.Errorf("protocol version = %v, want %s", meta[ipc.MetaProtocolVersion], ipc.ModernProtocolVersion)
		}
		client, _ := meta[ipc.MetaClientInfo].(map[string]any)
		if client["name"] != "markgogo-test" {
			t.Errorf("client info = %v", meta[ipc.MetaClientInfo])
		}
		_ = ipc.WriteMessage(conn, map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": map[string]any{"ok": true}})
	})

	client := ipc.Client{SocketPath: app.path, CWD: link, Name: "markgogo-test", Version: "1.0.0", Timeout: 5 * time.Second}
	result, err := client.Call("tools/list", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(string(result), `"ok":true`) {
		t.Fatalf("result = %s", result)
	}
	hellos, handles, _ := app.recorded()
	if len(hellos) != 1 || hellos[0].CWD != ipc.CanonicalPath(root) {
		t.Fatalf("hellos = %+v, want one canonical %q", hellos, ipc.CanonicalPath(root))
	}
	if len(handles) != 1 || handles[0] != "tools/list" {
		t.Fatalf("handled = %v", handles)
	}
}

func TestClientReportsAppNotRunning(t *testing.T) {
	client := ipc.Client{SocketPath: filepath.Join(shortTempDir(t), ipc.SocketName), CWD: t.TempDir(), Timeout: time.Second}
	_, err := client.Call("tools/list", nil)
	if err == nil {
		t.Fatal("Call succeeded without an app")
	}
	if code := ipc.CodeOf(err); code != ipc.CodeAppNotRunning {
		t.Fatalf("code = %q, want %q (err %v)", code, ipc.CodeAppNotRunning, err)
	}
	if !strings.Contains(err.Error(), ipc.UserDataEnv) {
		t.Errorf("error %q does not say how to find another user data directory", err)
	}
}

func TestClientReportsARefusedWorkspace(t *testing.T) {
	dir := shortTempDir(t)
	path := filepath.Join(dir, ipc.SocketName)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = ipc.ReadMessage(bufio.NewReader(conn))
		_ = ipc.WriteMessage(conn, ipc.HelloReply{OK: false, Error: &ipc.AppError{
			Code:    ipc.CodeWorkspaceNotOpen,
			Message: "No workspace contains that directory.",
			Hint:    "Open the folder in Mark Go Go first.",
		}})
	}()

	client := ipc.Client{SocketPath: path, CWD: dir, Timeout: 5 * time.Second}
	_, err = client.Call("tools/list", nil)
	if code := ipc.CodeOf(err); code != ipc.CodeWorkspaceNotOpen {
		t.Fatalf("code = %q, want %q (err %v)", code, ipc.CodeWorkspaceNotOpen, err)
	}
}

type shimPipe struct {
	stdin  *io.PipeWriter
	stdout *bufio.Reader
	logs   *logSink
	cwd    string
	done   chan error
}

// logSink collects the shim's diagnostics without ever blocking it.
type logSink struct {
	mu   sync.Mutex
	text string
}

func (l *logSink) Write(p []byte) (int, error) {
	l.mu.Lock()
	l.text += string(p)
	l.mu.Unlock()
	return len(p), nil
}

func (l *logSink) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text
}

func runShim(t *testing.T, socketPath, cwd string) *shimPipe {
	t.Helper()
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	logs := &logSink{}
	done := make(chan error, 1)
	go func() {
		done <- ipc.RunShim(ipc.ShimOptions{
			SocketPath:  socketPath,
			CWD:         cwd,
			Name:        "markgogo-mcp",
			Version:     "0.5.0",
			StdIn:       stdinR,
			StdOut:      stdoutW,
			StdErr:      logs,
			DialTimeout: 5 * time.Second,
		})
		_ = stdinR.CloseWithError(io.EOF)
		stdoutW.Close()
	}()
	return &shimPipe{stdin: stdinW, stdout: bufio.NewReader(stdoutR), logs: logs, cwd: cwd, done: done}
}

func (p *shimPipe) write(t *testing.T, line string) {
	t.Helper()
	if _, err := io.WriteString(p.stdin, line+"\n"); err != nil {
		t.Fatalf("write to shim: %v", err)
	}
}

func (p *shimPipe) read(t *testing.T) []byte {
	t.Helper()
	type read struct {
		line []byte
		err  error
	}
	got := make(chan read, 1)
	go func() {
		line, err := ipc.ReadMessage(p.stdout)
		got <- read{line, err}
	}()
	select {
	case r := <-got:
		if r.err != nil {
			t.Fatalf("read from shim: %v", r.err)
		}
		return r.line
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the shim")
		return nil
	}
}

func TestShimPipesMessagesWithoutLeakingTheHandshake(t *testing.T) {
	app := startFakeApp(t, func(_ int, conn net.Conn, _ ipc.Hello, in *bufio.Reader) {
		for {
			line, err := ipc.ReadMessage(in)
			if err != nil {
				return
			}
			var msg map[string]any
			if err := json.Unmarshal(line, &msg); err != nil {
				return
			}
			_ = ipc.WriteMessage(conn, map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": map[string]any{"method": msg["method"]}})
		}
	})
	pipe := runShim(t, app.path, t.TempDir())
	pipe.write(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	pipe.write(t, `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`)
	for _, want := range []string{`"id":1`, `"id":2`} {
		line := string(pipe.read(t))
		if !strings.Contains(line, want) || !strings.Contains(line, `"result"`) {
			t.Fatalf("shim wrote %s, want a result for %s", line, want)
		}
	}
	pipe.stdin.Close()
	if err := <-pipe.done; err != nil {
		t.Fatalf("RunShim: %v", err)
	}
	hellos, _, _ := app.recorded()
	if len(hellos) != 1 || hellos[0].CWD != ipc.CanonicalPath(pipe.cwd) {
		t.Fatalf("the shim announced %+v, want its own canonical working directory", hellos)
	}
}

func TestShimReportsAppNotRunningAsAJSONRPCError(t *testing.T) {
	pipe := runShim(t, filepath.Join(shortTempDir(t), ipc.SocketName), t.TempDir())
	pipe.write(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"propose_edit"}}`)
	line := pipe.read(t)
	var msg struct {
		ID    int `json:"id"`
		Error struct {
			Code    int          `json:"code"`
			Message string       `json:"message"`
			Data    ipc.AppError `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		t.Fatalf("decode %s: %v", line, err)
	}
	if msg.ID != 7 || msg.Error.Data.Code != ipc.CodeAppNotRunning {
		t.Fatalf("message = %s", line)
	}
	pipe.stdin.Close()
	if err := <-pipe.done; err != nil {
		t.Fatalf("RunShim: %v", err)
	}
}

func TestShimReconnectsAfterTheAppRestartsAndNeverReplaysWrites(t *testing.T) {
	var writes []string
	var writeMu sync.Mutex
	app := startFakeApp(t, func(index int, conn net.Conn, _ ipc.Hello, in *bufio.Reader) {
		line, err := ipc.ReadMessage(in)
		if err != nil {
			return
		}
		var msg map[string]any
		_ = json.Unmarshal(line, &msg)
		writeMu.Lock()
		writes = append(writes, msg["method"].(string))
		writeMu.Unlock()
		if index == 0 {
			return // die before answering, like a quit app
		}
		_ = ipc.WriteMessage(conn, map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": map[string]any{"ok": true}})
	})
	pipe := runShim(t, app.path, t.TempDir())
	pipe.write(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_checkpoint","arguments":{"path":"a.md"}}}`)

	line := pipe.read(t)
	if !strings.Contains(string(line), `"code":-32001`) || !strings.Contains(string(line), ipc.CodeAppNotRunning) {
		t.Fatalf("shim wrote %s, want an unknown-result error", line)
	}

	pipe.write(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	line = pipe.read(t)
	if !strings.Contains(string(line), `"id":2`) || !strings.Contains(string(line), `"result"`) {
		t.Fatalf("shim wrote %s, want the reconnected result", line)
	}
	_, _, accepts := app.recorded()
	if accepts != 2 {
		t.Fatalf("app connections = %d, want 2", accepts)
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if len(writes) != 2 || writes[0] != "tools/call" || writes[1] != "tools/list" {
		t.Fatalf("app saw %v, want the two messages once each", writes)
	}
}

// waitForStderr blocks until the shim writes a diagnostic that contains want,
// which tells the test that the previous connection is already dropped.
func (p *shimPipe) waitForStderr(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(p.logs.String(), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for the shim diagnostic %q; got %q", want, p.logs.String())
}

func TestShimRenegotiatesALegacySessionAfterReconnect(t *testing.T) {
	var methods []string
	var mu sync.Mutex
	app := startFakeApp(t, func(index int, conn net.Conn, _ ipc.Hello, in *bufio.Reader) {
		for {
			line, err := ipc.ReadMessage(in)
			if err != nil {
				return
			}
			var msg map[string]any
			if err := json.Unmarshal(line, &msg); err != nil {
				return
			}
			method, _ := msg["method"].(string)
			mu.Lock()
			methods = append(methods, method)
			mu.Unlock()
			if method == "initialize" && index == 0 {
				_ = ipc.WriteMessage(conn, map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": map[string]any{"protocolVersion": ipc.LegacyProtocolVersion}})
				return // the app quits right after the handshake
			}
			_ = ipc.WriteMessage(conn, map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": map[string]any{"protocolVersion": ipc.LegacyProtocolVersion, "ok": true}})
		}
	})
	pipe := runShim(t, app.path, t.TempDir())
	pipe.write(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"old","version":"1"}}}`)
	if line := string(pipe.read(t)); !strings.Contains(line, `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("shim wrote %s, want the legacy handshake", line)
	}
	pipe.waitForStderr(t, "connection closed")
	pipe.write(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if line := string(pipe.read(t)); !strings.Contains(line, `"id":2`) || !strings.Contains(line, `"result"`) {
		t.Fatalf("shim wrote %s, want the reconnected result", line)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"initialize", "initialize", "tools/list"}
	if len(methods) != len(want) {
		t.Fatalf("app saw %v, want %v", methods, want)
	}
	for i, method := range want {
		if methods[i] != method {
			t.Fatalf("app saw %v, want %v", methods, want)
		}
	}
}
