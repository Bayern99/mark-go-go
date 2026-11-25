package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// RPCTransportError reports that the channel to the app failed, so the result
// of the message is unknown.
const RPCTransportError = -32001

// ShimOptions configures the stdio shim: `markgogo mcp` runs one of these
// between an MCP client's standard streams and the app's socket.
type ShimOptions struct {
	SocketPath  string
	CWD         string
	Name        string
	Version     string
	StdIn       io.Reader
	StdOut      io.Writer
	StdErr      io.Writer
	DialTimeout time.Duration
}

// RunShim copies newline-delimited MCP messages from stdin to the app socket
// and back, dialing again after the app restarts. It never replays a message
// whose result is unknown.
func RunShim(opts ShimOptions) error {
	s := &Shim{opts: opts, pending: map[string]bool{}}
	if s.opts.StdOut == nil {
		return errors.New("the shim needs an output stream")
	}
	if s.opts.StdErr == nil {
		s.opts.StdErr = io.Discard
	}
	return s.run()
}

// Shim is the stdio to socket pipe.
type Shim struct {
	opts ShimOptions

	mu      sync.Mutex
	conn    net.Conn
	gen     int
	legacy  string
	pending map[string]bool

	outMu sync.Mutex
}

func (s *Shim) run() error {
	scanner := bufio.NewScanner(s.opts.StdIn)
	scanner.Buffer(make([]byte, 0, 64<<10), 32<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		s.forward(line)
	}
	s.close()
	return scanner.Err()
}

func (s *Shim) forward(line []byte) {
	id, isRequest := messageID(line)
	s.mu.Lock()
	err := s.connectLocked()
	conn, gen := s.conn, s.gen
	if isRequest && conn != nil {
		s.pending[string(id)] = true
	}
	s.mu.Unlock()

	if conn == nil {
		if isRequest {
			s.refuse(id, err)
		}
		return
	}
	if _, err := conn.Write(append(append([]byte{}, line...), '\n')); err != nil {
		s.drop(conn, gen, err)
	}
}

// connectLocked dials the app when no connection is open and restores the
// legacy handshake a reconnect would otherwise lose. It returns the refusal so
// the caller can report it, and keeps the connection closed.
func (s *Shim) connectLocked() error {
	if s.conn != nil {
		return nil
	}
	client := Client{SocketPath: s.opts.SocketPath, CWD: s.opts.CWD, Name: s.opts.Name, Version: s.opts.Version, Timeout: s.opts.DialTimeout}
	conn, _, err := client.Connect()
	if err != nil {
		var appErr *AppError
		if !errors.As(err, &appErr) {
			appErr = &AppError{Code: CodeAppNotRunning, Message: err.Error()}
			err = appErr
		}
		return err
	}
	if legacy := s.legacy; legacy != "" {
		echo, err := negotiateLegacy(conn, legacy, s.opts)
		if err != nil {
			conn.Close()
			return NotRunning(s.opts.SocketPath, err)
		}
		if echo != "" {
			s.legacy = echo
		}
	}
	s.gen++
	s.conn = conn
	go s.readLoop(conn, s.gen)
	return nil
}

// negotiateLegacy repeats the initialize handshake on a new connection so an
// old client keeps the protocol revision it already agreed to.
func negotiateLegacy(conn net.Conn, version string, opts ShimOptions) (string, error) {
	params := map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": opts.Name, "version": opts.Version},
	}
	if err := WriteMessage(conn, request{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: params}); err != nil {
		return "", err
	}
	line, err := ReadMessage(bufio.NewReader(conn))
	if err != nil {
		return "", err
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", resp.Error
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", err
	}
	return result.ProtocolVersion, nil
}

func (s *Shim) readLoop(conn net.Conn, gen int) {
	reader := bufio.NewReader(conn)
	for {
		line, err := ReadMessage(reader)
		if err != nil {
			s.drop(conn, gen, err)
			return
		}
		id, isRequest := messageID(line)
		s.mu.Lock()
		live := s.conn == conn && s.gen == gen
		if live && !isRequest && len(id) > 0 {
			delete(s.pending, string(id))
		}
		if live {
			if version := legacyVersionOf(line); version != "" {
				s.legacy = version
			}
		}
		s.mu.Unlock()
		if !live {
			return
		}
		s.writeOut(line)
	}
}

// drop closes a broken connection and tells the client that every request it
// left unanswered has an unknown result, without replaying any of them.
func (s *Shim) drop(conn net.Conn, gen int, cause error) {
	s.mu.Lock()
	if s.conn != conn || s.gen != gen {
		s.mu.Unlock()
		return
	}
	s.conn = nil
	conn.Close()
	ids := make([]string, 0, len(s.pending))
	for id := range s.pending {
		ids = append(ids, id)
	}
	clear(s.pending)
	s.mu.Unlock()

	fmt.Fprintf(s.opts.StdErr, "markgogo mcp: the app connection closed (%v)\n", cause)
	for _, id := range ids {
		s.writeError(json.RawMessage(id), &AppError{
			Code:    CodeAppNotRunning,
			Message: "The connection to Mark Go Go closed before this request finished, so its result is unknown.",
			Hint:    "Start the app again and re-read the workspace before retrying a write.",
		})
	}
}

func (s *Shim) refuse(id json.RawMessage, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = &AppError{Code: CodeAppNotRunning, Message: fmt.Sprint(err)}
	}
	s.writeError(id, appErr)
}

func (s *Shim) close() {
	s.mu.Lock()
	conn := s.conn
	s.conn = nil
	s.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func (s *Shim) writeOut(line []byte) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_, _ = s.opts.StdOut.Write(append(append([]byte{}, line...), '\n'))
}

func (s *Shim) writeError(id json.RawMessage, appErr *AppError) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   &RPCError{Code: RPCTransportError, Message: appErr.Message, Data: marshal(appErr)},
	}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_ = WriteMessage(s.opts.StdOut, payload)
}

func marshal(payload any) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return data
}

// messageID reports a message's id and whether it is a request rather than a
// response or a notification.
func messageID(line []byte) (json.RawMessage, bool) {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return json.RawMessage("null"), true
	}
	if msg.Method == "" {
		return msg.ID, false
	}
	if len(msg.ID) == 0 || string(msg.ID) == "null" {
		return nil, false
	}
	return msg.ID, true
}

// legacyVersionOf reports the revision a legacy initialize result negotiated.
func legacyVersionOf(line []byte) string {
	var msg struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return ""
	}
	return msg.Result.ProtocolVersion
}
