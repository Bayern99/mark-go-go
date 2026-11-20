package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

const (
	acpMaxRecordBytes = 16 << 20
	acpPipeDrainGrace = 250 * time.Millisecond
	acpPipeCloseGrace = 100 * time.Millisecond

	Sent     = "sent"
	Received = "received"

	ProtocolVersion = 1
	ClientName      = "MarkGoGo"
	ClientVersion   = "0.1.0"
)

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

type Location struct {
	Path string `json:"path"`
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type acpResponse struct {
	result json.RawMessage
	err    *RPCError
}

type Agent struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderrPipe io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan acpResponse

	handlerMu          sync.RWMutex
	handleRequest      func(method string, params json.RawMessage) (any, *RPCError)
	handleNotification func(method string, params json.RawMessage)

	observe func(direction string, line []byte)

	exited     chan struct{}
	readDone   chan struct{}
	stderrDone chan struct{}
	killOnce   sync.Once

	stderr *fsutil.LimitBuffer
}

func NewAgent(cmd *exec.Cmd, stderr *fsutil.LimitBuffer) (*Agent, error) {
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
	return &Agent{
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

func (a *Agent) Command() *exec.Cmd {
	if a == nil {
		return nil
	}
	return a.cmd
}

func (a *Agent) PID() int {
	if a == nil || a.cmd == nil || a.cmd.Process == nil {
		return 0
	}
	return a.cmd.Process.Pid
}

func (a *Agent) Exited() <-chan struct{} {
	return a.exited
}

func (a *Agent) StderrTail() string {
	if a == nil || a.stderr == nil {
		return ""
	}
	return a.stderr.Tail()
}

func (a *Agent) SetHandlers(request func(method string, params json.RawMessage) (any, *RPCError), notification func(method string, params json.RawMessage), observe func(direction string, line []byte)) {
	a.handlerMu.Lock()
	a.handleRequest = request
	a.handleNotification = notification
	a.observe = observe
	a.handlerMu.Unlock()
}

func (a *Agent) notifyHandler() func(method string, params json.RawMessage) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.handleNotification
}

func (a *Agent) requestHandler() func(method string, params json.RawMessage) (any, *RPCError) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.handleRequest
}

func (a *Agent) observeLine() func(direction string, line []byte) {
	a.handlerMu.RLock()
	defer a.handlerMu.RUnlock()
	return a.observe
}

func (a *Agent) Start() {
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

func (a *Agent) waitForPipes(timeout time.Duration) bool {
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

func (a *Agent) readLoop() {
	defer close(a.readDone)
	scanner := bufio.NewScanner(a.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), acpMaxRecordBytes)
	for scanner.Scan() {
		if line := bytes.TrimSpace(scanner.Bytes()); len(line) > 0 {
			if observe := a.observeLine(); observe != nil {
				observe(Received, append([]byte(nil), line...))
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
		go a.Kill()
	}
}

func (a *Agent) dispatch(line []byte) {
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error"`
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

func (a *Agent) answer(id int64, method string, params json.RawMessage) {
	var result any
	var rpcErr *RPCError
	if handle := a.requestHandler(); handle == nil {
		rpcErr = &RPCError{Code: -32601, Message: "method not found"}
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

func (a *Agent) Call(method string, params any, result any) error {
	return a.CallCapturing(method, params, result, nil)
}

func (a *Agent) CallCapturing(method string, params any, result any, raw *json.RawMessage) error {
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	ch := make(chan acpResponse, 1)
	a.pending[id] = ch
	a.mu.Unlock()

	if err := a.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		a.mu.Lock()
		delete(a.pending, id)
		a.mu.Unlock()
		return fmt.Errorf("could not send %s to the agent: %v", method, err)
	}
	var resp acpResponse
	select {
	case resp = <-ch:
	case <-a.exited:
		select {
		case resp = <-ch:
		default:
			a.mu.Lock()
			delete(a.pending, id)
			a.mu.Unlock()
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

func (a *Agent) Notify(method string, params any) error {
	return a.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (a *Agent) write(v any) error {
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
		observe(Sent, data)
	}
	return nil
}

func (a *Agent) goneMessage(method string) string {
	msg := fmt.Sprintf("The agent exited before answering %s.", method)
	if tail := a.stderr.Tail(); tail != "" {
		msg += " Its error output: " + tail
	}
	return msg
}

func (a *Agent) CancelTurn(sessionID string) {
	if sessionID == "" {
		return
	}
	go func() {
		_ = a.Notify("session/cancel", map[string]any{"sessionId": sessionID})
	}()
}

func (a *Agent) Kill() {
	a.killOnce.Do(func() {
		_ = a.stdin.Close()
		if a.cmd != nil && a.cmd.Process != nil {
			_ = a.cmd.Process.Kill()
		}
	})
	<-a.exited
}

func (a *Agent) HasExited() bool {
	select {
	case <-a.exited:
		return true
	default:
		return false
	}
}
