// Package pirpc speaks Pi's JSONL RPC transport: one JSON record per line on
// stdin and stdout, responses correlated by id, session events by type, and the
// extension UI sub-protocol. It knows nothing about which CLI arguments make a
// program speak RPC or what its commands mean; each backend names the program,
// prepares the *exec.Cmd, and reads the events it cares about.
package pirpc

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
	// MaxRecordBytes is the longest single JSONL record a program may write.
	MaxRecordBytes = 32 << 20
	// PipeDrainGrace is how long the pipes get to deliver what the program
	// wrote before it exited.
	PipeDrainGrace = 250 * time.Millisecond
	// PipeCloseGrace is how long the pipes get to report EOF once closed.
	PipeCloseGrace = 100 * time.Millisecond

	defaultStderrLimit = 8 << 10
)

// Config is what a backend hands the transport. The command carries the
// backend's own executable, arguments, working folder and environment, so no
// CLI knowledge lives here.
type Config struct {
	Command *exec.Cmd
	// Name is how error messages refer to the program, such as "Feynman".
	Name string
	// StderrLimit caps how much error output is kept for the message that
	// explains an exit. Zero keeps the default.
	StderrLimit int
}

type CallResponse struct {
	data json.RawMessage
	err  error
}

type Client struct {
	cmd        *exec.Cmd
	name       string
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderrPipe io.ReadCloser

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[string]chan CallResponse

	handlerMu sync.RWMutex
	onEvent   func(eventType string, raw json.RawMessage)
	onUIReq   func(raw json.RawMessage)

	exited     chan struct{}
	readDone   chan struct{}
	stderrDone chan struct{}
	killOnce   sync.Once
	shutOnce   sync.Once

	stderr *fsutil.LimitBuffer
}

// New wires the program's pipes and starts it. Start must follow, once the
// caller has installed its handlers, so no record is dispatched into a client
// that is still being assembled.
func New(cfg Config) (*Client, error) {
	if cfg.Command == nil {
		return nil, errors.New("no command to start")
	}
	cmd := cfg.Command
	limit := cfg.StderrLimit
	if limit <= 0 {
		limit = defaultStderrLimit
	}
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
	return &Client{
		cmd:        cmd,
		name:       cfg.Name,
		stdin:      stdin,
		stdout:     stdout,
		stderrPipe: stderrPipe,
		pending:    make(map[string]chan CallResponse),
		exited:     make(chan struct{}),
		readDone:   make(chan struct{}),
		stderrDone: make(chan struct{}),
		stderr:     fsutil.NewLimitBuffer(limit),
	}, nil
}

func (c *Client) Command() *exec.Cmd {
	if c == nil {
		return nil
	}
	return c.cmd
}

func (c *Client) Exited() <-chan struct{} {
	return c.exited
}

// SetHandlers installs the two callbacks a backend receives records through:
// one for session events, one for extension UI requests. Passing nil stops
// delivery, which a backend does before it lets the process exit.
func (c *Client) SetHandlers(onEvent func(eventType string, raw json.RawMessage), onUIReq func(raw json.RawMessage)) {
	c.handlerMu.Lock()
	c.onEvent = onEvent
	c.onUIReq = onUIReq
	c.handlerMu.Unlock()
}

// Start begins reading the program's records.
func (c *Client) Start() {
	go c.readLoop()
	go func() {
		_ = c.cmd.Wait()
		if !c.waitForPipes(PipeDrainGrace) {
			_ = c.stdout.Close()
			_ = c.stderrPipe.Close()
			c.waitForPipes(PipeCloseGrace)
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

// StderrTail is what the program wrote to its error output, so a failure can
// quote the program's own explanation, such as an extension that failed to
// load.
func (c *Client) StderrTail() string {
	if c == nil {
		return ""
	}
	return c.stderr.Tail()
}

func (c *Client) waitForPipes(timeout time.Duration) bool {
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

func (c *Client) readLoop() {
	defer close(c.readDone)
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), MaxRecordBytes)
	scanner.Split(SplitJSONLLines)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		c.dispatch(append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		reason := err.Error()
		if errors.Is(err, bufio.ErrTooLong) {
			reason = fmt.Sprintf("a message exceeded the %d MiB limit", MaxRecordBytes>>20)
		}
		_, _ = fmt.Fprintf(c.stderr, "\nCould not read %s RPC output: %s", c.name, reason)
		go c.Kill()
	}
}

func (c *Client) dispatch(line []byte) {
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
		var resp CallResponse
		if !envelope.Success {
			msg := strings.TrimSpace(envelope.Error)
			if msg == "" {
				msg = fmt.Sprintf("%s %s command failed.", c.name, envelope.Command)
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

// Call sends one command and decodes its response data into result when result
// is not nil.
func (c *Client) Call(commandType string, fields map[string]any, result any) error {
	ch := make(chan CallResponse, 1)
	id := c.reserve(ch)

	payload := make(map[string]any, len(fields)+2)
	maps.Copy(payload, fields)
	payload["id"] = id
	payload["type"] = commandType

	if err := c.write(payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("could not send %s to %s: %w", commandType, c.name, err)
	}

	var resp CallResponse
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

// Send writes one command without waiting for its answer, for a command whose
// reply the caller does not need, such as an abort.
func (c *Client) Send(commandType string, fields map[string]any) error {
	id := c.reserve(nil)
	payload := make(map[string]any, len(fields)+2)
	maps.Copy(payload, fields)
	payload["id"] = id
	payload["type"] = commandType
	return c.write(payload)
}

// reserve hands out the next id and, when a reply is expected, the channel it
// will arrive on.
func (c *Client) reserve(reply chan CallResponse) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := fmt.Sprintf("req-%d", c.nextID)
	if reply != nil {
		c.pending[id] = reply
	}
	return id
}

// SendUIResponse answers an extension_ui_request by the id it carried.
func (c *Client) SendUIResponse(id string, fields map[string]any) error {
	payload := make(map[string]any, len(fields)+2)
	maps.Copy(payload, fields)
	payload["type"] = "extension_ui_response"
	payload["id"] = id
	return c.write(payload)
}

func (c *Client) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *Client) goneMessage(commandType string) string {
	msg := fmt.Sprintf("%s exited before answering %s.", c.name, commandType)
	if tail := c.StderrTail(); tail != "" {
		msg += " Error output: " + tail
	}
	return msg
}

// Kill stops the program and everything it started, then waits for the pipes to
// close. It is safe to call more than once.
func (c *Client) Kill() {
	c.killOnce.Do(func() {
		_ = c.stdin.Close()
		Stop(c.cmd)
	})
	<-c.exited
}

// Shutdown asks the program to exit by closing its input, gives it grace to
// finish on its own, and kills it when it does not.
func (c *Client) Shutdown(grace time.Duration) {
	c.shutOnce.Do(func() {
		_ = c.stdin.Close()
		select {
		case <-c.exited:
		case <-time.After(grace):
			c.Kill()
		}
	})
	<-c.exited
}
