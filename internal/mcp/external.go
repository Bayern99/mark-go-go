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
	"time"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/ipc"
)

const helloTimeout = 5 * time.Second

// ExternalServer is the opt-in local socket channel: newline-delimited
// JSON-RPC at <UserData>/ipc.sock, opened by a first frame that names the
// caller's working directory.
type ExternalServer struct {
	server *Server
	deps   Dependencies

	mu    sync.Mutex
	ln    net.Listener
	path  string
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
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

// Start listens on the socket, cleaning up a socket a previous run left behind.
func (e *ExternalServer) Start() error {
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
	if err := clearStaleSocket(path); err != nil {
		return err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return err
	}

	e.mu.Lock()
	e.ln = ln
	e.path = path
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
	if info.Mode()&fs.ModeSocket != 0 {
		conn, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return fmt.Errorf("another Mark Go Go instance is serving %s", path)
		}
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

	binding := &sessionBinding{root: root, channel: channelExternal}
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
			Hint:    newAppError(ipc.CodeWorkspaceNotOpen, "").Hint,
		}
	}
	return best, nil
}

// Close stops listening, closes the open connections, and removes the socket.
func (e *ExternalServer) Close() error {
	e.mu.Lock()
	ln := e.ln
	path := e.path
	e.ln = nil
	conns := make([]net.Conn, 0, len(e.conns))
	for conn := range e.conns {
		conns = append(conns, conn)
	}
	e.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}
	var err error
	if ln != nil {
		err = ln.Close()
	}
	e.wg.Wait()
	if path != "" {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}
