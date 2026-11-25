// Package ipc is the local channel between the Mark Go Go app and the
// markgogo CLI: a Unix socket that speaks newline-delimited JSON-RPC, opened by
// a first frame that names the caller's working directory.
package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// AppName is the user data directory the app keeps its files in.
	AppName = "Mark Go Go"
	// SocketName is the socket file inside the user data directory.
	SocketName = "ipc.sock"
	// UserDataEnv points the app and the CLI at another user data directory.
	UserDataEnv = "MARKGOGO_USER_DATA"

	// ChannelExternal names the local socket channel in proposal sources and
	// audit records.
	ChannelExternal = "external-ipc"

	// ModernProtocolVersion is the MCP revision this CLI and shim ask for.
	ModernProtocolVersion = "2026-07-28"
	// LegacyProtocolVersion is the newest handshake-based MCP revision.
	LegacyProtocolVersion = "2025-11-25"

	// DialTimeout bounds how long a caller waits for the app to accept.
	DialTimeout = 3 * time.Second
)

// Error codes the app and the CLI report in a stable, machine-readable form.
const (
	CodeAppNotRunning    = "APP_NOT_RUNNING"
	CodeWorkspaceNotOpen = "WORKSPACE_NOT_OPEN"
	CodeVersionConflict  = "VERSION_CONFLICT"
	CodeNotFound         = "NOT_FOUND"
	CodePathDenied       = "PATH_DENIED"
	CodeInvalidArgs      = "INVALID_ARGS"
)

// AppError is a failure the caller can act on, keyed by a stable code.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *AppError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

// Errorf builds an AppError.
func Errorf(code, format string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeOf reports the stable code of err, or "" when err carries none.
func CodeOf(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("json-rpc error %d: %s", e.Code, e.Message)
}

// Hello is the first frame a caller writes to the socket.
type Hello struct {
	CWD string `json:"cwd"`
}

// HelloReply answers the first frame: the workspace the caller's working
// directory resolves to, or the reason it resolves to none.
type HelloReply struct {
	OK               bool      `json:"ok"`
	Root             string    `json:"root,omitempty"`
	AppVersion       string    `json:"appVersion,omitempty"`
	ProtocolVersions []string  `json:"protocolVersions,omitempty"`
	Error            *AppError `json:"error,omitempty"`
}

// UserDataDir is the directory the app keeps user data in: the
// MARKGOGO_USER_DATA override, else the platform config directory joined with
// the app name.
func UserDataDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(UserDataEnv)); dir != "" {
		return filepath.Clean(dir), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// SocketPath is the socket inside a user data directory.
func SocketPath(userDataDir string) string {
	return filepath.Join(userDataDir, SocketName)
}

// CanonicalPath resolves symlinks and makes a path absolute, so that a caller's
// working directory compares equal to the app's workspace roots.
func CanonicalPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs)
}

// WriteMessage writes one JSON-RPC message as a single line.
func WriteMessage(w io.Writer, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = w.Write(append(bytes.TrimRight(data, "\r\n"), '\n'))
	return err
}

// ReadMessage reads one newline-delimited message.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	trimmed := bytes.TrimSpace(line)
	if err != nil && (err != io.EOF || len(trimmed) == 0) {
		return nil, err
	}
	return trimmed, nil
}

type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Client asks the running app for work over the local socket.
type Client struct {
	SocketPath string
	CWD        string
	Name       string
	Version    string
	Timeout    time.Duration
}

// Connect opens the socket and completes the first frame.
func (c Client) Connect() (net.Conn, *HelloReply, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DialTimeout
	}
	conn, err := net.DialTimeout("unix", c.SocketPath, timeout)
	if err != nil {
		return nil, nil, NotRunning(c.SocketPath, err)
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if err := WriteMessage(conn, Hello{CWD: CanonicalPath(c.CWD)}); err != nil {
		conn.Close()
		return nil, nil, NotRunning(c.SocketPath, err)
	}
	line, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		conn.Close()
		return nil, nil, NotRunning(c.SocketPath, err)
	}
	var reply HelloReply
	if err := json.Unmarshal(line, &reply); err != nil {
		conn.Close()
		return nil, nil, &AppError{Code: CodeAppNotRunning, Message: fmt.Sprintf("%s answered with a frame the CLI does not understand.", c.SocketPath)}
	}
	if !reply.OK {
		conn.Close()
		if reply.Error != nil {
			return nil, &reply, reply.Error
		}
		return nil, &reply, Errorf(CodeWorkspaceNotOpen, "The app did not open a workspace for %s.", c.CWD)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, &reply, nil
}

// Call performs one request over a fresh connection and returns its result.
func (c Client) Call(method string, params map[string]any) (json.RawMessage, error) {
	result, _, err := c.CallWithHello(method, params)
	return result, err
}

// CallWithHello also reports the first frame's answer, which names the
// workspace the call was bound to.
func (c Client) CallWithHello(method string, params map[string]any) (json.RawMessage, *HelloReply, error) {
	conn, hello, err := c.Connect()
	if err != nil {
		return nil, hello, err
	}
	defer conn.Close()
	if timeout := c.Timeout; timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
	payload := request{JSONRPC: "2.0", ID: 1, Method: method, Params: withModernMeta(params, c.Name, c.Version)}
	if err := WriteMessage(conn, payload); err != nil {
		return nil, hello, NotRunning(c.SocketPath, err)
	}
	line, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		return nil, hello, NotRunning(c.SocketPath, err)
	}
	var rpcResp response
	if err := json.Unmarshal(line, &rpcResp); err != nil {
		return nil, hello, &AppError{Code: CodeAppNotRunning, Message: "The app answered with a response the CLI does not understand."}
	}
	if rpcResp.Error != nil {
		return nil, hello, rpcResp.Error
	}
	return rpcResp.Result, hello, nil
}

// withModernMeta adds the per-request protocol version and client identity the
// modern MCP revision requires, leaving params the caller already set alone.
func withModernMeta(params map[string]any, name, version string) map[string]any {
	out := make(map[string]any, len(params)+1)
	for key, value := range params {
		out[key] = value
	}
	meta, _ := out["_meta"].(map[string]any)
	next := make(map[string]any, len(meta)+2)
	for key, value := range meta {
		next[key] = value
	}
	if _, set := next[MetaProtocolVersion]; !set {
		next[MetaProtocolVersion] = ModernProtocolVersion
	}
	if _, set := next[MetaClientInfo]; !set && name != "" {
		next[MetaClientInfo] = map[string]any{"name": name, "version": version}
	}
	out["_meta"] = next
	return out
}

// Keys of request _meta this channel reads and writes.
const (
	MetaProtocolVersion = "io.modelcontextprotocol/protocolVersion"
	MetaClientInfo      = "io.modelcontextprotocol/clientInfo"
	MetaServerInfo      = "io.modelcontextprotocol/serverInfo"
)

// NotRunning reports that nothing answered at the socket.
func NotRunning(socketPath string, cause error) error {
	message := fmt.Sprintf("Mark Go Go is not running at %s", socketPath)
	if cause != nil && !errors.Is(cause, os.ErrNotExist) {
		message = fmt.Sprintf("%s (%v)", message, cause)
	}
	return &AppError{
		Code:    CodeAppNotRunning,
		Message: message + ".",
		Hint:    "Start the Mark Go Go app; to use another user data directory, set " + UserDataEnv + ".",
	}
}

// ToolCall runs a tool and returns its structured result, turning an app error
// result into an *AppError.
func (c Client) ToolCall(name string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	result, err := c.Call("tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return nil, err
	}
	var out struct {
		Content           []struct{ Text string } `json:"content"`
		StructuredContent json.RawMessage         `json:"structuredContent"`
		IsError           bool                    `json:"isError"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, err
	}
	if !out.IsError {
		return out.StructuredContent, nil
	}
	var appErr AppError
	if err := json.Unmarshal(out.StructuredContent, &appErr); err != nil || appErr.Code == "" {
		message := "The app refused the request."
		if len(out.Content) > 0 {
			message = out.Content[0].Text
		}
		return nil, &AppError{Code: CodeInvalidArgs, Message: message}
	}
	return nil, &appErr
}
