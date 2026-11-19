package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/egoist/mygo"
)

const (
	researchConsoleMaxInput   = 256 << 10
	researchConsoleMaxBacklog = 2 << 20
	researchConsoleMaxArgs    = 256
	researchConsoleMaxArgSize = 64 << 10
)

type ResearchConsoleUpdate struct {
	Root     string `json:"root"`
	Kind     string `json:"kind"`
	Text     string `json:"text,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	Seq      uint64 `json:"seq,omitempty"`
}

type ResearchConsoleSnapshot struct {
	Text     string `json:"text"`
	Seq      uint64 `json:"seq"`
	Running  bool   `json:"running"`
	Complete bool   `json:"complete"`
	ExitCode int    `json:"exitCode,omitempty"`
}

var researchConsoleUpdate = mygo.NewEvent[ResearchConsoleUpdate]("researchConsoleUpdate")

type researchConsoleSession struct {
	root     string
	work     string
	win      *mygo.Window
	cmd      *exec.Cmd
	pty      *os.File
	baseline researchBaseline

	mu        sync.Mutex
	backlog   bytes.Buffer
	seq       uint64
	utf8Tail  []byte
	stopOnce  sync.Once
	done      chan struct{}
	readDone  chan struct{}
	stopping  bool
	offClosed func()
}

type researchConsoleHost struct {
	documents *documentHost
	research  *researchHost
	dataDir   string
	command   func(root, work string, args []string) (*exec.Cmd, error)

	mu             sync.Mutex
	sessions       map[string]*researchConsoleSession
	completed      map[string]ResearchConsoleSnapshot
	completedOwner map[string]*mygo.Window
	closed         bool
}

func newResearchConsoleHost(documents *documentHost, research *researchHost) *researchConsoleHost {
	return &researchConsoleHost{
		documents:      documents,
		research:       research,
		dataDir:        documents.settings.dir,
		command:        researchConsoleCommand,
		sessions:       make(map[string]*researchConsoleSession),
		completed:      make(map[string]ResearchConsoleSnapshot),
		completedOwner: make(map[string]*mygo.Window),
	}
}

func (h *researchConsoleHost) workspace(ctx context.Context, root string) (*workspace, error) {
	return h.research.workspace(ctx, root)
}

func (h *researchConsoleHost) Connect(ctx context.Context, root string, args []string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	root = canonicalPath(w.root)

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return errors.New("The Feynman console is closing.")
	}
	if session := h.sessions[root]; session != nil && session.win == w.win {
		h.mu.Unlock()
		return errors.New("The Feynman console is already open for this workspace. Attach to the existing console.")
	}
	h.mu.Unlock()
	if err := validateResearchConsoleArgs(args); err != nil {
		return err
	}
	if err := h.research.reserveConsole(root); err != nil {
		return err
	}
	releaseConsole := true
	defer func() {
		if releaseConsole {
			h.research.releaseConsole(root)
		}
	}()
	h.mu.Lock()
	delete(h.completed, root)
	delete(h.completedOwner, root)
	h.mu.Unlock()

	work := researchWorkDir(h.dataDir, root)
	baseline, err := prepareResearchArtifacts(work, root)
	if err != nil {
		return err
	}
	cmd, err := h.command(root, work, args)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = nil
	term, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return fmt.Errorf("could not start Feynman console: %w", err)
	}
	session := &researchConsoleSession{
		root:     root,
		work:     work,
		win:      w.win,
		cmd:      cmd,
		pty:      term,
		baseline: baseline,
		done:     make(chan struct{}),
		readDone: make(chan struct{}),
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = term.Close()
		killResearchConsoleProcess(cmd)
		return errors.New("The Feynman console is closing.")
	}
	if previous := h.sessions[root]; previous != nil {
		h.mu.Unlock()
		_ = term.Close()
		killResearchConsoleProcess(cmd)
		return errors.New("The Feynman console is already open for this workspace.")
	}
	h.sessions[root] = session
	h.mu.Unlock()
	releaseConsole = false

	if w.win != nil {
		offClosed := w.win.OnClosed(func() { _ = h.disconnect(root, session) })
		session.mu.Lock()
		session.offClosed = offClosed
		session.mu.Unlock()
	}
	_ = researchConsoleUpdate.Emit(w.win, ResearchConsoleUpdate{Root: root, Kind: "reset"})
	go h.read(session)
	go h.wait(session)
	return nil
}

func (h *researchConsoleHost) read(session *researchConsoleSession) {
	defer close(session.readDone)
	buf := make([]byte, 32<<10)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 {
			session.mu.Lock()
			data := append([]byte(nil), session.utf8Tail...)
			data = append(data, buf[:n]...)
			complete, tail := completeUTF8(data)
			session.utf8Tail = append(session.utf8Tail[:0], tail...)
			text := string(complete)
			if text == "" {
				session.mu.Unlock()
				if err != nil {
					return
				}
				continue
			}
			session.seq++
			seq := session.seq
			_, _ = session.backlog.Write([]byte(text))
			if session.backlog.Len() > researchConsoleMaxBacklog {
				data := append([]byte(nil), session.backlog.Bytes()...)
				data = data[len(data)-researchConsoleMaxBacklog:]
				session.backlog.Reset()
				_, _ = session.backlog.Write(data)
			}
			session.mu.Unlock()
			_ = researchConsoleUpdate.Emit(session.win, ResearchConsoleUpdate{Root: session.root, Kind: "output", Text: text, Seq: seq})
		}
		if err != nil {
			session.mu.Lock()
			text := string(session.utf8Tail)
			session.utf8Tail = nil
			session.mu.Unlock()
			if text != "" {
				session.mu.Lock()
				session.seq++
				seq := session.seq
				_, _ = session.backlog.Write([]byte(text))
				session.mu.Unlock()
				_ = researchConsoleUpdate.Emit(session.win, ResearchConsoleUpdate{Root: session.root, Kind: "output", Text: text, Seq: seq})
			}
			return
		}
	}
}

func completeUTF8(data []byte) (complete, tail []byte) {
	for i := len(data); i > 0 && len(data)-i <= 4; i-- {
		if data[i-1]&0xc0 == 0x80 {
			continue
		}
		if !utf8.FullRune(data[i-1:]) {
			return data[:i-1], data[i-1:]
		}
		break
	}
	return data, nil
}

func (h *researchConsoleHost) wait(session *researchConsoleSession) {
	err := session.cmd.Wait()
	select {
	case <-session.readDone:
	case <-time.After(250 * time.Millisecond):
		_ = session.pty.Close()
		<-session.readDone
	}
	_ = session.pty.Close()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	session.mu.Lock()
	stopping := session.stopping
	session.mu.Unlock()
	finishErr := error(nil)
	if !stopping && exitCode == 0 {
		finishErr = finishResearchArtifacts(session.work, session.root, session.baseline)
	}
	session.mu.Lock()
	offClosed := session.offClosed
	session.offClosed = nil
	snapshot := ResearchConsoleSnapshot{Text: session.backlog.String(), Seq: session.seq, Complete: true, ExitCode: exitCode}
	session.mu.Unlock()
	if offClosed != nil {
		offClosed()
	}
	h.mu.Lock()
	if h.sessions[session.root] == session {
		delete(h.sessions, session.root)
	}
	h.completed[session.root] = snapshot
	h.completedOwner[session.root] = session.win
	h.mu.Unlock()
	if finishErr != nil {
		_ = researchConsoleUpdate.Emit(session.win, ResearchConsoleUpdate{Root: session.root, Kind: "error", Text: finishErr.Error()})
	}
	_ = researchConsoleUpdate.Emit(session.win, ResearchConsoleUpdate{Root: session.root, Kind: "exit", ExitCode: exitCode})
	h.research.releaseConsole(session.root)
	close(session.done)
}

func (h *researchConsoleHost) session(ctx context.Context, root string) (*researchConsoleSession, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	session := h.sessions[canonicalPath(w.root)]
	h.mu.Unlock()
	if session == nil || session.win != mygo.CallerWindow(ctx) {
		return nil, errors.New("Open the Feynman console for this workspace first.")
	}
	return session, nil
}

func (h *researchConsoleHost) sessionForWindow(ctx context.Context, root string) (*researchConsoleSession, error) {
	canonical := canonicalPath(root)
	caller := mygo.CallerWindow(ctx)
	h.mu.Lock()
	session := h.sessions[canonical]
	h.mu.Unlock()
	if session == nil || session.win != caller {
		return nil, errors.New("Open the Feynman console for this workspace first.")
	}
	return session, nil
}

func (h *researchConsoleHost) Input(ctx context.Context, root, data string) error {
	session, err := h.session(ctx, root)
	if err != nil {
		return err
	}
	if len(data) > researchConsoleMaxInput {
		return errors.New("The Feynman console input is too large.")
	}
	_, err = session.pty.Write([]byte(data))
	return err
}

func (h *researchConsoleHost) Resize(ctx context.Context, root string, cols, rows int) error {
	session, err := h.session(ctx, root)
	if err != nil {
		return err
	}
	if cols < 20 || cols > 500 || rows < 4 || rows > 200 {
		return errors.New("The Feynman console size is outside the supported range.")
	}
	return pty.Setsize(session.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (h *researchConsoleHost) Snapshot(ctx context.Context, root string) (ResearchConsoleSnapshot, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return ResearchConsoleSnapshot{}, err
	}
	h.mu.Lock()
	session := h.sessions[canonicalPath(w.root)]
	completed, ok := h.completed[canonicalPath(w.root)]
	h.mu.Unlock()
	if session == nil || session.win != w.win {
		if ok {
			return completed, nil
		}
		return ResearchConsoleSnapshot{}, errors.New("Open the Feynman console for this workspace first.")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return ResearchConsoleSnapshot{Text: session.backlog.String(), Seq: session.seq, Running: true}, nil
}

func (h *researchConsoleHost) Disconnect(ctx context.Context, root string) error {
	session, err := h.sessionForWindow(ctx, root)
	if err != nil {
		canonical := canonicalPath(root)
		caller := mygo.CallerWindow(ctx)
		h.mu.Lock()
		if h.completedOwner[canonical] == caller {
			delete(h.completed, canonical)
			delete(h.completedOwner, canonical)
		}
		h.mu.Unlock()
		return nil
	}
	return h.disconnect(canonicalPath(root), session)
}

func (h *researchConsoleHost) disconnect(root string, session *researchConsoleSession) error {
	session.stopOnce.Do(func() {
		session.mu.Lock()
		session.stopping = true
		session.mu.Unlock()
		_, _ = session.pty.Write([]byte{3})
		go func() {
			select {
			case <-session.done:
			case <-time.After(2 * time.Second):
				killResearchConsoleProcess(session.cmd)
			}
		}()
	})
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		killResearchConsoleProcess(session.cmd)
		<-session.done
	}
	return nil
}

func (h *researchConsoleHost) shutdown() {
	h.mu.Lock()
	h.closed = true
	sessions := make([]*researchConsoleSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		sessions = append(sessions, session)
	}
	h.mu.Unlock()
	for _, session := range sessions {
		_ = h.disconnect(session.root, session)
	}
	h.mu.Lock()
	h.completed = make(map[string]ResearchConsoleSnapshot)
	h.completedOwner = make(map[string]*mygo.Window)
	h.mu.Unlock()
}

func validateResearchConsoleArgs(args []string) error {
	if len(args) > researchConsoleMaxArgs {
		return fmt.Errorf("Choose a Feynman command with at most %d arguments.", researchConsoleMaxArgs)
	}
	literal := false
	for _, arg := range args {
		if len(arg) > researchConsoleMaxArgSize {
			return errors.New("A Feynman console argument is too long.")
		}
		if strings.IndexByte(arg, 0) >= 0 {
			return errors.New("A Feynman console argument contains a NUL byte.")
		}
		if literal || arg == "--" {
			literal = true
			continue
		}
		for _, name := range []string{"--cwd", "--session-dir", "--mode"} {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return errors.New("The console controls its own Feynman working directory and mode.")
			}
		}
	}
	return nil
}

func findResearchExecutable(env []string, name string) string {
	pathValue := ""
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			pathValue = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	for _, dir := range filepath.SplitList(pathValue) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return candidate
		}
	}
	return ""
}
