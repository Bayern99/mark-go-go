package feynman

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

const (
	rpcMaxRecordBytes = 32 << 20
	rpcPipeDrainGrace = 250 * time.Millisecond
	rpcPipeCloseGrace = 100 * time.Millisecond
)

type rpcCallResponse struct {
	data json.RawMessage
	err  error
}

type rpcClient struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderrPipe io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[string]chan rpcCallResponse

	handlerMu sync.RWMutex
	onEvent   func(eventType string, raw json.RawMessage)
	onUIReq   func(raw json.RawMessage)

	exited     chan struct{}
	readDone   chan struct{}
	stderrDone chan struct{}
	killOnce   sync.Once

	stderr *fsutil.LimitBuffer
}

func newRPCClient(cmd *exec.Cmd) (*rpcClient, error) {
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
	return &rpcClient{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderrPipe: stderrPipe,
		pending:    make(map[string]chan rpcCallResponse),
		exited:     make(chan struct{}),
		readDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		stderr:     fsutil.NewLimitBuffer(8192),
	}, nil
}

func (c *rpcClient) Command() *exec.Cmd {
	if c == nil {
		return nil
	}
	return c.cmd
}

func (c *rpcClient) Exited() <-chan struct{} {
	return c.exited
}

func (c *rpcClient) SetHandlers(onEvent func(eventType string, raw json.RawMessage), onUIReq func(raw json.RawMessage)) {
	c.handlerMu.Lock()
	c.onEvent = onEvent
	c.onUIReq = onUIReq
	c.handlerMu.Unlock()
}

func (c *rpcClient) Start() {
	go c.readLoop()
	go func() {
		_ = c.cmd.Wait()
		if !c.waitForPipes(rpcPipeDrainGrace) {
			_ = c.stdout.Close()
			_ = c.stderrPipe.Close()
			c.waitForPipes(rpcPipeCloseGrace)
		}
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		_ = c.stderrPipe.Close()
		close(c.exited)
	}()
	go func() {
		defer close(c.stderrDone)
		_, _ = io.Copy(c.stderr, c.stderrPipe)
	}()
}

func (c *rpcClient) waitForPipes(timeout time.Duration) bool {
	stdoutDone, stderrDone := c.readDone, c.stderrDone
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

func splitJSONLLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line := data[:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		return i + 1, line, nil
	}
	if atEOF {
		line := data
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		return len(data), line, nil
	}
	return 0, nil, nil
}

func (c *rpcClient) readLoop() {
	defer close(c.readDone)
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), rpcMaxRecordBytes)
	scanner.Split(splitJSONLLines)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			// Ignore blank lines and non-JSON stdout banner lines.
			continue
		}
		c.dispatch(append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		reason := err.Error()
		if errors.Is(err, bufio.ErrTooLong) {
			reason = fmt.Sprintf("a message exceeded the %d MiB limit", rpcMaxRecordBytes>>20)
		}
		_, _ = fmt.Fprintf(c.stderr, "\nCould not read Feynman RPC output: %s", reason)
		go c.Kill()
	}
}

func (c *rpcClient) dispatch(line []byte) {
	var envelope struct {
		ID      string          `json:"id"`
		Type    string          `json:"type"`
		Command string          `json:"command"`
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil || envelope.Type == "" {
		return
	}
	switch envelope.Type {
	case "response":
		if envelope.ID == "" {
			return
		}
		var resp rpcCallResponse
		if !envelope.Success {
			msg := strings.TrimSpace(envelope.Error)
			if msg == "" {
				msg = fmt.Sprintf("Feynman %s command failed.", envelope.Command)
			}
			resp.err = errors.New(msg)
		} else if len(envelope.Data) > 0 {
			resp.data = append([]byte(nil), envelope.Data...)
		}
		c.mu.Lock()
		ch := c.pending[envelope.ID]
		delete(c.pending, envelope.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- resp
		}

	case "extension_ui_request":
		c.handlerMu.RLock()
		onUIReq := c.onUIReq
		c.handlerMu.RUnlock()
		if onUIReq != nil {
			onUIReq(line)
		}

	default:
		c.handlerMu.RLock()
		onEvent := c.onEvent
		c.handlerMu.RUnlock()
		if onEvent != nil {
			onEvent(envelope.Type, line)
		}
	}
}

func (c *rpcClient) Call(commandType string, fields map[string]any, result any) error {
	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("req-%d", c.nextID)
	ch := make(chan rpcCallResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	payload := make(map[string]any, len(fields)+2)
	maps.Copy(payload, fields)
	payload["id"] = id
	payload["type"] = commandType

	if err := c.write(payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("could not send %s to Feynman: %w", commandType, err)
	}

	var resp rpcCallResponse
	select {
	case resp = <-ch:
	case <-c.exited:
		select {
		case resp = <-ch:
		default:
			c.mu.Lock()
			delete(c.pending, id)
			c.mu.Unlock()
			return errors.New(c.goneMessage(commandType))
		}
	}
	if resp.err != nil {
		return resp.err
	}
	if result == nil || len(resp.data) == 0 || string(resp.data) == "null" {
		return nil
	}
	if err := json.Unmarshal(resp.data, result); err != nil {
		return fmt.Errorf("malformed response for %s: %w", commandType, err)
	}
	return nil
}

func (c *rpcClient) SendUIResponse(id string, fields map[string]any) error {
	payload := make(map[string]any, len(fields)+2)
	maps.Copy(payload, fields)
	payload["type"] = "extension_ui_response"
	payload["id"] = id
	return c.write(payload)
}

func (c *rpcClient) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *rpcClient) goneMessage(commandType string) string {
	msg := fmt.Sprintf("Feynman exited before answering %s.", commandType)
	if tail := c.stderr.Tail(); tail != "" {
		msg += " Error output: " + tail
	}
	return msg
}

func (c *rpcClient) Kill() {
	c.killOnce.Do(func() {
		_ = c.stdin.Close()
		stopResearchProcess(c.cmd)
	})
	<-c.exited
}
