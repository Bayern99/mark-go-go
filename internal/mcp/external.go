package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/ipc"
)

const helloTimeout = 5 * time.Second

var socketFiles sync.Mutex

// unknownClient is what the channel calls a request whose _meta named no
// client: the legacy handshake may not, and Settings says so rather than
// inventing a name.
const unknownClient = "An unknown client"

// ExternalCall is one request the local channel answered: which client sent
// it and when. It is what Settings / AI Integration shows as the last external
// call, so a reader can tell whether the agent they connected is the one
// talking to the app.
type ExternalCall struct {
	Source string
	At     time.Time
}

// ExternalServer is the opt-in local socket channel: newline-delimited
// JSON-RPC at <UserData>/ipc.sock, opened by a first frame that names the
// caller's working directory.
type ExternalServer struct {
	server *Server
	deps   Dependencies

	lifecycle  sync.Mutex
	mu         sync.Mutex
	ln         net.Listener
	path       string
	socketInfo fs.FileInfo
	last       ExternalCall
	conns      map[net.Conn]struct{}
	wg         sync.WaitGroup
}

func NewExternalServer(deps Dependencies) *ExternalServer {
	return &ExternalServer{
		server: NewServer(deps),
		deps:   deps,
		conns:  map[net.Conn]struct{}{},
	}
}

// SocketPath reports where the server listens once started.
func (e *ExternalServer) SocketPath() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

// Listening reports whether the socket is being served now.
func (e *ExternalServer) Listening() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ln != nil
}

// LastCall reports the most recent request the socket answered. A server that
// has answered none reports false, and the page says no agent has called yet.
func (e *ExternalServer) LastCall() (source string, at time.Time, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last.Source, e.last.At, e.last.Source != ""
}

// note records the request in line as the last thing the channel saw. A
// notification carries a method but no id, and a client that names itself in
// one request rarely repeats the name in the next: only requests count, so the
// page never reports "an unknown client" one frame after a named one. It reads
// only what the page shows, so a malformed message stays the dispatcher's
// business.
func (e *ExternalServer) note(line []byte, legacyClient *string) {
	var message struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Meta       map[string]json.RawMessage `json:"_meta"`
			ClientInfo json.RawMessage            `json:"clientInfo"`
		} `json:"params"`
	}
	if json.Unmarshal(line, &message) != nil || message.Method == "" || len(message.ID) == 0 || string(message.ID) == "null" {
		return
	}
	if message.Method == "initialize" {
		*legacyClient = clientName(message.Params.ClientInfo)
	}
	source := clientName(message.Params.Meta[ipc.MetaClientInfo])
	if len(message.Params.Meta[ipc.MetaProtocolVersion]) == 0 && source == unknownClient && *legacyClient != "" {
		source = *legacyClient
	}
	e.mu.Lock()
	e.last = ExternalCall{Source: source, At: time.Now()}
	e.mu.Unlock()
}

// clientName reads the client a request named in its _meta, or says it named
// none.
func clientName(raw json.RawMessage) string {
	var info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &info) != nil || strings.TrimSpace(info.Name) == "" {
		return unknownClient
	}
	if version := strings.TrimSpace(info.Version); version != "" {
		return strings.TrimSpace(info.Name) + " " + version
	}
	return strings.TrimSpace(info.Name)
}

// Start listens on the socket, cleaning up a socket a previous run left behind.
func (e *ExternalServer) Start() error {
	e.lifecycle.Lock()
	defer e.lifecycle.Unlock()
	e.mu.Lock()
	if e.ln != nil {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	if e.deps.Workspace == nil {
		return errors.New("the workspace bridge is unavailable")
	}
	dir := e.deps.Workspace.UserDataDir()
	if dir == "" {
		return errors.New("the app has no user data folder for its socket")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// The socket's parent folder holds only the user's own files.
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	path := ipc.SocketPath(dir)
	socketFiles.Lock()
	defer socketFiles.Unlock()
	if err := clearStaleSocket(path); err != nil {
		return err
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		ln.Close()
		return err
	}
	ln.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return errors.Join(err, removeOwnedSocket(path, info))
	}

	e.mu.Lock()
	e.ln = ln
	e.path = path
	e.socketInfo = info
	e.mu.Unlock()

	e.wg.Add(1)
	go e.accept(ln)
	return nil
}

// clearStaleSocket removes a socket file that nothing answers on, and refuses
// to take over one that another instance is serving.
func clearStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSocket == 0 {
		return fmt.Errorf("the socket path %s is occupied by another file", path)
	}
	conn, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		return fmt.Errorf("another Mark Go Go instance is serving %s", path)
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return dialErr
	}
	return removeOwnedSocket(path, info)
}

func removeOwnedSocket(path string, owned fs.FileInfo) error {
	current, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Mode()&fs.ModeSocket == 0 || !os.SameFile(current, owned) {
		return nil
	}
	return os.Remove(path)
}

func (e *ExternalServer) accept(ln net.Listener) {
	defer e.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		e.mu.Lock()
		if e.ln != ln {
			e.mu.Unlock()
			conn.Close()
			return
		}
		e.conns[conn] = struct{}{}
		e.mu.Unlock()

		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			defer func() {
				e.mu.Lock()
				delete(e.conns, conn)
				e.mu.Unlock()
				conn.Close()
			}()
			e.serve(conn)
		}()
	}
}

func (e *ExternalServer) serve(conn net.Conn) {
	reader := bufio.NewReader(conn)
	if err := conn.SetDeadline(time.Now().Add(helloTimeout)); err != nil {
		return
	}
	line, err := ipc.ReadMessage(reader)
	if err != nil {
		return
	}
	var hello ipc.Hello
	if err := json.Unmarshal(line, &hello); err != nil {
		_ = ipc.WriteMessage(conn, ipc.HelloReply{OK: false, Error: &ipc.AppError{
			Code:    ipc.CodeInvalidArgs,
			Message: "The first frame must be a JSON object with a cwd.",
			Hint:    "Send {\"cwd\": \"<absolute path>\"} before any JSON-RPC message.",
		}})
		return
	}
	root, rootErr := e.rootFor(hello.CWD)
	var rootOpen func() bool
	if rootErr == nil {
		if workspace, ok := e.deps.Workspace.(interface{ MCPBindRoot(string) func() bool }); ok {
			rootOpen = workspace.MCPBindRoot(root)
		}
		if rootOpen == nil {
			failure := newAppError(ipc.CodeWorkspaceNotOpen, "That workspace is no longer open.")
			rootErr = &ipc.AppError{Code: failure.Code, Message: failure.Message, Hint: failure.Hint}
		}
	}
	if rootErr != nil {
		_ = ipc.WriteMessage(conn, ipc.HelloReply{OK: false, Error: rootErr})
		return
	}
	if err := ipc.WriteMessage(conn, ipc.HelloReply{
		OK:               true,
		Root:             root,
		AppVersion:       appversion.Version,
		ProtocolVersions: []string{modernProtocolVersion, latestLegacyProtocolVersion},
	}); err != nil {
		return
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return
	}

	binding := &sessionBinding{root: root, channel: channelExternal, rootOpen: rootOpen}
	var legacyClient string
	for {
		line, err := ipc.ReadMessage(reader)
		if err != nil {
			return
		}
		if len(line) > 0 && line[0] == '[' {
			_ = ipc.WriteMessage(conn, rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: errCodeInvalidRequest, Message: "this channel carries one JSON-RPC message per line"},
			})
			continue
		}
		e.note(line, &legacyClient)
		resp, _, hasResp := e.server.dispatchMessage(context.Background(), binding, line)
		if !hasResp {
			continue
		}
		if err := ipc.WriteMessage(conn, resp); err != nil {
			return
		}
	}
}

// rootFor picks the deepest open workspace that contains the caller's working
// directory.
func (e *ExternalServer) rootFor(cwd string) (string, *ipc.AppError) {
	if e.deps.Workspace == nil {
		return "", &ipc.AppError{Code: ipc.CodeWorkspaceNotOpen, Message: "The workspace bridge is unavailable."}
	}
	canonCwd := fsutil.CanonicalPath(strings.TrimSpace(cwd))
	if canonCwd == "" {
		return "", &ipc.AppError{
			Code:    ipc.CodeInvalidArgs,
			Message: "The first frame needs a cwd.",
			Hint:    "Send {\"cwd\": \"<absolute path>\"} before any JSON-RPC message.",
		}
	}
	best, bestDepth := "", -1
	for _, root := range e.deps.Workspace.MCPOpenRoots() {
		canonRoot := fsutil.CanonicalPath(root)
		if canonRoot == "" || !fsutil.FolderContains(canonRoot, canonCwd) {
			continue
		}
		depth := strings.Count(canonRoot, string(filepath.Separator))
		if depth > bestDepth || (depth == bestDepth && len(canonRoot) > len(best)) {
			best, bestDepth = canonRoot, depth
		}
	}
	if best == "" {
		return "", &ipc.AppError{
			Code:    ipc.CodeWorkspaceNotOpen,
			Message: fmt.Sprintf("No open workspace contains %s.", canonCwd),
			Hint:    newAppError(ipc.CodeWorkspaceNotOpen, "That workspace is no longer open.").Hint,
		}
	}
	return best, nil
}

// Close stops listening, closes the open connections, and removes the socket.
func (e *ExternalServer) Close() error {
	e.lifecycle.Lock()
	defer e.lifecycle.Unlock()

	e.mu.Lock()
	ln := e.ln
	if ln == nil {
		e.mu.Unlock()
		return nil
	}
	path, info := e.path, e.socketInfo
	e.ln, e.socketInfo = nil, nil
	for conn := range e.conns {
		conn.Close()
	}
	e.mu.Unlock()

	socketFiles.Lock()
	err := errors.Join(ln.Close(), removeOwnedSocket(path, info))
	socketFiles.Unlock()
	e.wg.Wait()
	return err
}
