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
	acpMaxRecordBytes = 16 << 20
	acpPipeDrainGrace = 250 * time.Millisecond
	acpPipeCloseGrace = 100 * time.Millisecond

	acpSent     = "sent"
	acpReceived = "received"

	acpProtocolVersion = 1
	acpClientName      = "MarkGoGo"
	acpClientVersion   = "0.1.0"
)

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *acpError) Error() string { return e.Message }

type acpLocation struct {
	Path string `json:"path"`
}

type acpPermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

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

func (b *limitBuffer) Tail() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimRight(string(b.buf), "\n")
}

type acpResponse struct {
	result json.RawMessage
	err    *acpError
}

type acpAgent struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderrPipe io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan acpResponse

	handlerMu          sync.RWMutex
	handleRequest      func(method string, params json.RawMessage) (any, *acpError)
	handleNotification func(method string, params json.RawMessage)

	observe func(direction string, line []byte)

	exited     chan struct{}
	readDone   chan struct{}
	stderrDone chan struct{}
	killOnce   sync.Once

	stderr *limitBuffer
}

func newACPAgent(cmd *exec.Cmd, stderr *limitBuffer) (*acpAgent, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
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

func (a *acpAgent) setHandlers(request func(method string, params json.RawMessage) (any, *acpError), notification func(method string, params json.RawMessage), observe func(direction string, line []byte)) {
	a.handlerMu.Lock()
	a.handleRequest = request
	a.handleNotification = notification
	a.observe = observe
	a.handlerMu.Unlock()
}

func (a *acpAgent) notifyHandler() func(method string, params json.RawMessage) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.handleNotification
}

func (a *acpAgent) requestHandler() func(method string, params json.RawMessage) (any, *acpError) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.handleRequest
}

func (a *acpAgent) observeLine() func(direction string, line []byte) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.observe
}

func (a *acpAgent) start() {
	go a.readLoop()
	go func() {
		_ = a.cmd.Wait()
		if !a.waitForPipes(acpPipeDrainGrace) {
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

func (a *acpAgent) readLoop() {
	defer close(a.readDone)
	scanner := bufio.NewScanner(a.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), acpMaxRecordBytes)
	for scanner.Scan() {
		if line := bytes.TrimSpace(scanner.Bytes()); len(line) > 0 {
			if observe := a.observeLine(); observe != nil {
				observe(acpReceived, append([]byte(nil), line...))
			}
			a.dispatch(line)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		reason := err.Error()
		if errors.Is(err, bufio.ErrTooLong) {
			reason = fmt.Sprintf("a message exceeded the %d MiB limit", acpMaxRecordBytes>>20)
		}
		_, _ = fmt.Fprintf(a.stderr, "\nCould not read the agent's ACP output: %s", reason)
		go a.kill()
	}
}

func (a *acpAgent) dispatch(line []byte) {
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *acpError       `json:"error"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return
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
		if handle := a.notifyHandler(); handle != nil {
			handle(msg.Method, msg.Params)
		}
	}
}

func (a *acpAgent) answer(id int64, method string, params json.RawMessage) {
	var result any
	var rpcErr *acpError
	if handle := a.requestHandler(); handle == nil {
		rpcErr = &acpError{Code: -32601, Message: "method not found"}
	} else {
		result, rpcErr = handle(method, params)
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	_ = a.write(resp)
}

func (a *acpAgent) call(method string, params any, result any) error {
	return a.callCapturing(method, params, result, nil)
}

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

func (a *acpAgent) notify(method string, params any) error {
	return a.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

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
	if observe := a.observeLine(); observe != nil {
		observe(acpSent, data)
	}
	return nil
}

func (a *acpAgent) goneMessage(method string) string {
	msg := fmt.Sprintf("The agent exited before answering %s.", method)
	if tail := a.stderr.Tail(); tail != "" {
		msg += " Its error output: " + tail
	}
	return msg
}

func (a *acpAgent) cancelTurn(sessionID string) {
	if sessionID == "" {
		return
	}
	go func() {
		_ = a.notify("session/cancel", map[string]any{"sessionId": sessionID})
	}()
}

func (a *acpAgent) kill() {
	a.killOnce.Do(func() {
		_ = a.stdin.Close()
		_ = a.cmd.Process.Kill()
	})
	<-a.exited
}

func (a *acpAgent) hasExited() bool {
	select {
	case <-a.exited:
		return true
	default:
		return false
	}
}
