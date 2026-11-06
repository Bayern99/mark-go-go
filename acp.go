package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
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
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan acpResponse

	handleRequest      func(method string, params json.RawMessage) (any, *acpError)
	handleNotification func(method string, params json.RawMessage)

	exited     chan struct{} // closed once the process has been waited for
	exitedOnce sync.Once
	readDone   chan struct{} // closed when readLoop has ended
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
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &acpAgent{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		pending:  make(map[int64]chan acpResponse),
		exited:   make(chan struct{}),
		readDone: make(chan struct{}),
		stderr:   stderr,
	}, nil
}

// start reads the agent's messages and reaps the process, each in a
// goroutine of its own. The reaper waits for the reading to be over:
// Wait closes the pipes, so it is only correct once the read loop is
// done; killing the process ends the read loop.
func (a *acpAgent) start() {
	go a.readLoop()
	go func() {
		<-a.readDone
		_ = a.cmd.Wait()
		a.exitedOnce.Do(func() { close(a.exited) })
	}()
}

// readLoop delivers every message of the agent until its output ends.
func (a *acpAgent) readLoop() {
	defer close(a.readDone)
	scanner := bufio.NewScanner(a.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		if line := bytes.TrimSpace(scanner.Bytes()); len(line) > 0 {
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
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	ch := make(chan acpResponse, 1)
	a.pending[id] = ch
	a.mu.Unlock()

	if err := a.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return fmt.Errorf("could not send %s to the agent: %v", method, err)
	}
	select {
	case resp := <-ch:
		if resp.err != nil {
			return resp.err
		}
		if result == nil || resp.result == nil {
			return nil
		}
		if err := json.Unmarshal(resp.result, result); err != nil {
			return fmt.Errorf("the answer to %s is malformed: %v", method, err)
		}
		return nil
	case <-a.exited:
		return errors.New(a.goneMessage(method))
	}
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
	_, err = a.stdin.Write(append(data, '\n'))
	return err
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
	if sessionID != "" {
		_ = a.notify("session/cancel", map[string]any{"sessionId": sessionID})
	}
	select {
	case <-a.exited:
		return
	case <-time.After(grace):
	}
	a.kill()
}
