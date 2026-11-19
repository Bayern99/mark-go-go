package feynman

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maxResearchUIBytes = 1 << 20

type researchUI struct {
	listener net.Listener
	dir      string
	token    string
	context  context.Context
	cancel   context.CancelFunc
}

func newResearchUI(cmd *exec.Cmd, relayScript []byte) (*researchUI, error) {
	dir, err := os.MkdirTemp("/tmp", "mgo-pi-")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "ui.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		os.Remove(dir)
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ui := &researchUI{listener: listener, dir: dir, token: rand.Text(), context: ctx, cancel: cancel}
	if err := os.Chmod(path, 0600); err != nil {
		ui.close()
		return nil, err
	}
	cmd.Env = append(cmd.Environ(), "MGO_FEYNMAN_UI_SOCKET="+path, "MGO_FEYNMAN_UI_TOKEN="+ui.token)
	if err := prepareResearchRelay(cmd, dir, relayScript); err != nil {
		ui.close()
		return nil, err
	}
	return ui, nil
}

func (ui *researchUI) close() {
	if ui != nil {
		ui.cancel()
		ui.listener.Close()
		os.RemoveAll(ui.dir)
	}
}

func (ui *researchUI) serve(h *ResearchManager, s *researchSession) {
	for {
		conn, err := ui.listener.Accept()
		if err != nil {
			return
		}
		go ui.handle(conn, h, s)
	}
}

type researchUIRequest struct {
	Token, Method, Title, Placeholder, Prefill, Text, Key string
	Commands                                              []ResearchCommand
}

func (ui *researchUI) handle(conn net.Conn, h *ResearchManager, s *researchSession) {
	defer conn.Close()
	stop := context.AfterFunc(ui.context, func() { conn.Close() })
	defer stop()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var req researchUIRequest
	if err := json.NewDecoder(io.LimitReader(conn, maxResearchUIBytes)).Decode(&req); err != nil || req.Token != ui.token {
		return
	}
	conn.SetReadDeadline(time.Time{})
	var response any = struct{}{}
	switch req.Method {
	case "commands":
		s.updateMu.Lock()
		s.extensions = req.Commands
		for i := range s.extensions {
			s.extensions[i].Source = "extension"
		}
		var upd *ResearchUpdate
		if s.replay == nil && s.commands != nil && ui.context.Err() == nil {
			u := s.commandsUpdateLocked()
			upd = &u
		}
		s.updateMu.Unlock()
		if upd != nil {
			s.emit(*upd)
		}
	case "input", "editor":
		ctx, cancel := context.WithCancel(ui.context)
		defer cancel()
		go func() {
			var b [1]byte
			conn.Read(b[:])
			cancel()
		}()
		answer := h.askResearchQuestion(ctx, s, "", ResearchUpdate{Kind: req.Method, Text: req.Title, Placeholder: req.Placeholder, Prefill: req.Prefill})
		if answer != nil {
			response = map[string]string{"value": *answer}
		} else {
			response = map[string]bool{"cancelled": true}
		}
	case "set_editor_text":
		h.mu.Lock()
		live := s.busy && !s.cancelled
		h.mu.Unlock()
		if live {
			s.emit(ResearchUpdate{Kind: "editorText", Text: req.Text})
		}
	case "activity":
		if req.Key != "" && ui.context.Err() == nil {
			s.emit(ResearchUpdate{Kind: "activity", Key: req.Key, Text: req.Text})
		}
	case "message":
		if req.Text != "" && ui.context.Err() == nil {
			s.emit(ResearchUpdate{Kind: "assistant", Text: req.Text, MessageID: newRandomID()})
		}
	default:
		response = map[string]bool{"cancelled": true}
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	json.NewEncoder(conn).Encode(response)
}

func (s *researchSession) commandsUpdateLocked() ResearchUpdate {
	commands := slices.Clone(s.commands)
	for _, ext := range s.extensions {
		switch ext.Name {
		case "", "new", "quit", "exit":
			continue
		}
		if !strings.ContainsAny(ext.Name, " \t\r\n/") && !slices.ContainsFunc(commands, func(c ResearchCommand) bool { return c.Name == ext.Name }) {
			commands = append(commands, ext)
		}
	}
	return ResearchUpdate{Kind: "commands", Commands: commands}
}

func (h *ResearchManager) askResearchQuestion(ctx context.Context, s *researchSession, sessionID string, event ResearchUpdate) *string {
	h.mu.Lock()
	turn := s.turn
	agent := s.agent
	h.mu.Unlock()
	select {
	case s.questionSlot <- struct{}{}:
		defer func() { <-s.questionSlot }()
	case <-ctx.Done():
		return nil
	case <-s.stopped:
		return nil
	case <-turn:
		return nil
	}
	h.mu.Lock()
	if !s.busy || s.cancelled || s.turn != turn || (sessionID != "" && sessionID != s.id) || ctx.Err() != nil {
		h.mu.Unlock()
		return nil
	}
	q := &researchQuestion{id: newRandomID(), options: event.Options, answer: make(chan *string, 1)}
	s.pending = q
	h.mu.Unlock()
	event.RequestID = q.id
	s.emit(event)
	var exited <-chan struct{}
	if agent != nil {
		exited = agent.Exited()
	}
	var answer *string
	select {
	case answer = <-q.answer:
	case <-ctx.Done():
	case <-s.stopped:
	case <-exited:
	case <-turn:
	}
	h.mu.Lock()
	if s.pending == q {
		s.pending = nil
	}
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "permissionDone", RequestID: q.id})
	return answer
}

func (h *ResearchManager) AnswerText(ctx context.Context, root, requestID, text string, cancelled bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return err
	}
	q := s.pending
	if q == nil || q.id != requestID || len(q.options) != 0 || s.cancelled {
		return errors.New("This Feynman input request is no longer available.")
	}
	if !cancelled && len(text) > maxResearchUIBytes {
		return errors.New("The response exceeds the 1 MiB limit.")
	}
	s.pending = nil
	if cancelled {
		q.answer <- nil
	} else {
		q.answer <- &text
	}
	return nil
}
