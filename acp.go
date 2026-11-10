package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	acpPipeDrainGrace = 250 * time.Millisecond
	acpPipeCloseGrace = 100 * time.Millisecond

	// acpSent and acpReceived are the directions observe reports.
	acpSent     = "sent"
	acpReceived = "received"

	// acpProtocolVersion is the ACP version this client speaks; the
	// initialize answer is checked against it. The client identity is
	// what the agent sees in the handshake.
	acpProtocolVersion = 1
	acpClientName      = "MarkGoGo"
	acpClientVersion   = "0.1.0"
)

// acpError is a JSON-RPC error object, as an error.
type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *acpError) Error() string { return e.Message }

// acpLocation is one file location an agent names for a tool call.
type acpLocation struct {
	Path string `json:"path"`
}

// acpPermissionOption is one option of a permission request of the
// agent.
type acpPermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// limitBuffer keeps the last max bytes written to it: the tail of an
// agent's error output, for error messages. Safe for concurrent use.
type limitBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newLimitBuffer(max int) *limitBuffer { return &limitBuffer{max: max} }

func (b *limitBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = append(b.buf[:0], b.buf[len(b.buf)-b.max:]...)
	}
	return len(p), nil
}

// Tail returns what the buffer keeps, without its trailing newlines.
func (b *limitBuffer) Tail() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimRight(string(b.buf), "\n")
}

// acpResponse is one answer of the agent.
type acpResponse struct {
	result json.RawMessage
	err    *acpError
}

// acpAgent is a minimal client for an ACP agent: JSON-RPC 2.0 over
// newline-delimited JSON on the pipes of a running command. Its
// goroutine reads the agent's messages, answering the agent's requests
// through handleRequest and handing its notifications to
// handleNotification; call sends a request and waits for the answer.
type acpAgent struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderrPipe io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan acpResponse

	handleRequest      func(method string, params json.RawMessage) (any, *acpError)
	handleNotification func(method string, params json.RawMessage)

	// observe, when set, gets every JSON-RPC message that crossed the
	// pipes, with its direction, for tracing. It is called for an
	// inbound message before dispatch, with bytes that are already
	// copied (the scanner reuses its buffer), and for an outbound
	// message only after the write to stdin succeeded, with the exact
	// marshaled bytes.
	observe func(direction string, line []byte)

	exited     chan struct{} // closed once the process has been waited for
	readDone   chan struct{} // closed when readLoop has ended
	stderrDone chan struct{} // closed when stderr has been drained
	killOnce   sync.Once

	stderr *limitBuffer
}

// newACPAgent starts cmd (its arguments, working directory and
// environment are already set) and prepares to speak ACP over its
// pipes. Call start once the agent's handlers are set.
func newACPAgent(cmd *exec.Cmd, stderr *limitBuffer) (*acpAgent, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// Direct file pipes let Wait reap the process without waiting for
	// descendants that inherited its output descriptors.
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderrPipe, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stdoutWriter.Close()
		return nil, err
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stdoutWriter.Close()
		_ = stderrPipe.Close()
		_ = stderrWriter.Close()
		return nil, err
	}
	// The child has its own copies. Leaving the parent's writers open
	// would prevent the readers from seeing EOF when the agent exits.
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	return &acpAgent{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderrPipe: stderrPipe,
		pending:    make(map[int64]chan acpResponse),
		exited:     make(chan struct{}),
		readDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		stderr:     stderr,
	}, nil
}

// start reads protocol messages, drains stderr and reaps the process on
// independent goroutines.
func (a *acpAgent) start() {
	go a.readLoop()
	go func() {
		_ = a.cmd.Wait()
		if !a.waitForPipes(acpPipeDrainGrace) {
			// shortcut: agent descendants can outlive pipe cleanup, add process-tree control if they must exit too.
			_ = a.stdout.Close()
			_ = a.stderrPipe.Close()
			a.waitForPipes(acpPipeCloseGrace)
		}
		_ = a.stdin.Close()
		_ = a.stdout.Close()
		_ = a.stderrPipe.Close()
		close(a.exited)
	}()
	go func() {
		defer close(a.stderrDone)
		_, _ = io.Copy(a.stderr, a.stderrPipe)
	}()
}

// waitForPipes lets completed output readers finish, but never waits
// beyond the supplied duration for inherited descriptors to close.
func (a *acpAgent) waitForPipes(timeout time.Duration) bool {
	stdoutDone, stderrDone := a.readDone, a.stderrDone
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for stdoutDone != nil || stderrDone != nil {
		select {
		case <-stdoutDone:
			stdoutDone = nil
		case <-stderrDone:
			stderrDone = nil
		case <-timer.C:
			return false
		}
	}
	return true
}

// readLoop delivers every message of the agent until its output ends.
func (a *acpAgent) readLoop() {
	defer close(a.readDone)
	scanner := bufio.NewScanner(a.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		if line := bytes.TrimSpace(scanner.Bytes()); len(line) > 0 {
			if a.observe != nil {
				// The scanner reuses its buffer: the observer gets its
				// own copy of the line.
				a.observe(acpReceived, append([]byte(nil), line...))
			}
			a.dispatch(line)
		}
	}
}

// dispatch reads one JSON-RPC message and routes it: a request of the
// agent to handleRequest, an answer to whoever called, a notification
// to handleNotification.
func (a *acpAgent) dispatch(line []byte) {
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *acpError       `json:"error"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return // not a JSON-RPC message; ignore it
	}
	switch {
	case msg.ID != nil && msg.Method != "":
		a.answer(*msg.ID, msg.Method, msg.Params)
	case msg.ID != nil:
		resp := acpResponse{err: msg.Error}
		if msg.Result != nil {
			resp.result = append([]byte(nil), msg.Result...)
		}
		a.mu.Lock()
		ch := a.pending[*msg.ID]
		delete(a.pending, *msg.ID)
		a.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	case msg.Method != "":
		if a.handleNotification != nil {
			a.handleNotification(msg.Method, msg.Params)
		}
	}
}

// answer replies to a request of the agent.
func (a *acpAgent) answer(id int64, method string, params json.RawMessage) {
	var result any
	var rpcErr *acpError
	if a.handleRequest == nil {
		rpcErr = &acpError{Code: -32601, Message: "method not found"}
	} else {
		result, rpcErr = a.handleRequest(method, params)
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	_ = a.write(resp)
}

// call sends a JSON-RPC request and waits for its answer or the end of
// the agent, and unmarshals the answer into result when one is given.
func (a *acpAgent) call(method string, params any, result any) error {
	return a.callCapturing(method, params, result, nil)
}

// callCapturing is call with an optional copy of the raw result for
// tracing. raw stays nil, with no copy at all, when the caller does
// not ask for it.
func (a *acpAgent) callCapturing(method string, params any, result any, raw *json.RawMessage) error {
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	ch := make(chan acpResponse, 1)
	a.pending[id] = ch
	a.mu.Unlock()

	if err := a.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return fmt.Errorf("could not send %s to the agent: %v", method, err)
	}
	var resp acpResponse
	select {
	case resp = <-ch:
	case <-a.exited:
		// readLoop may have buffered the final response just before exit.
		select {
		case resp = <-ch:
		default:
			return errors.New(a.goneMessage(method))
		}
	}
	if resp.err != nil {
		return resp.err
	}
	if raw != nil {
		*raw = resp.result
	}
	if result == nil || resp.result == nil {
		return nil
	}
	if err := json.Unmarshal(resp.result, result); err != nil {
		return fmt.Errorf("the answer to %s is malformed: %v", method, err)
	}
	return nil
}

// notify sends a JSON-RPC notification, which no answer follows.
func (a *acpAgent) notify(method string, params any) error {
	return a.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// write sends one message as a line on the agent's stdin.
func (a *acpAgent) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if _, err := a.stdin.Write(append(data, '\n')); err != nil {
		return err
	}
	if a.observe != nil {
		// The write succeeded: the line is what the agent got. data is
		// not reused after this point, so it needs no copy.
		a.observe(acpSent, data)
	}
	return nil
}

// goneMessage says why the agent's answer to method never came.
func (a *acpAgent) goneMessage(method string) string {
	msg := fmt.Sprintf("The agent exited before answering %s.", method)
	if tail := a.stderr.Tail(); tail != "" {
		msg += " Its error output: " + tail
	}
	return msg
}

// kill stops the agent: it closes its stdin, kills the process, and
// waits for it to exit. Safe to call more than once, and from any
// goroutine.
func (a *acpAgent) kill() {
	a.killOnce.Do(func() {
		_ = a.stdin.Close()
		_ = a.cmd.Process.Kill()
	})
	<-a.exited
}

// cancelGracefully asks the agent to cancel its turn, gives it grace
// to stop on its own, then kills it and waits for it to exit.
func (a *acpAgent) cancelGracefully(sessionID string, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()
	if sessionID != "" {
		// A child that stopped reading can block this write; do not delay the timer.
		go func() {
			_ = a.notify("session/cancel", map[string]any{"sessionId": sessionID})
		}()
	}
	select {
	case <-a.exited:
		return
	case <-timer.C:
	}
	a.kill()
}
