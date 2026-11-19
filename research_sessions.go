package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

type ResearchConversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
	Current   bool   `json:"current"`
}

func (h *researchHost) routeSessionUpdate(s *researchSession, method string, raw json.RawMessage) {
	if method != "session/update" {
		return
	}
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	if s.replay != nil {
		s.replay = append(s.replay, raw)
		return
	}
	h.update(s, method, raw)
}

func workspaceConversations(s *researchSession, work, current string) ([]ResearchConversation, error) {
	var result []ResearchConversation
	cursor := ""
	seen := map[string]bool{}
	for {
		params := map[string]string{"cwd": work}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var page struct {
			Sessions   []struct{ SessionID, Cwd, Title, UpdatedAt string }
			NextCursor string
		}
		if err := s.agent.call("session/list", params, &page); err != nil {
			return nil, err
		}
		for _, entry := range page.Sessions {
			if entry.SessionID != "" && filepath.IsAbs(entry.Cwd) && sameRoot(entry.Cwd, work) {
				result = append(result, ResearchConversation{ID: entry.SessionID, Title: entry.Title, UpdatedAt: entry.UpdatedAt, Current: entry.SessionID == current})
			}
		}
		if page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return nil, errors.New("Feynman returned an invalid conversation history cursor.")
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
	if current != "" && !slices.ContainsFunc(result, func(c ResearchConversation) bool { return c.Current }) {
		result = append([]ResearchConversation{{ID: current, Current: true}}, result...)
	}
	return result, nil
}

func (h *researchHost) Sessions(ctx context.Context, root string) ([]ResearchConversation, error) {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	id := s.id
	h.mu.Unlock()
	return workspaceConversations(s, researchWorkDir(h.dataDir, root), id)
}

func (h *researchHost) beginConversationChange(ctx context.Context, root string) (*researchSession, string, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		return nil, "", nil, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			return nil, "", nil, s.startErr
		}
	default:
		return nil, "", nil, errors.New("Feynman is connecting.")
	}
	if s.busy || s.switching || s.configuring {
		return nil, "", nil, errors.New("Wait for the current research operation to finish.")
	}
	s.switching = true
	s.turn = make(chan struct{})
	currentID := s.id
	return s, currentID, func() {
		h.mu.Lock()
		s.switching = false
		close(s.turn)
		h.mu.Unlock()
	}, nil
}

func checkWorkspaceConversation(s *researchSession, work, id string) error {
	list, err := workspaceConversations(s, work, "")
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(list, func(c ResearchConversation) bool { return c.ID == id }) {
		return errors.New("Choose a conversation from this workspace.")
	}
	return nil
}

func (h *researchHost) SelectSession(ctx context.Context, root, id string) error {
	s, currentID, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	work := researchWorkDir(h.dataDir, root)
	if id == currentID {
		release()
		return nil
	}
	if id != "" {
		if err := checkWorkspaceConversation(s, work, id); err != nil {
			release()
			return err
		}
	}
	err = h.openConversation(s, work, id, "")
	if err != nil {
		h.mu.Lock()
		s.startErr = err
		h.mu.Unlock()
	}
	release()
	if err != nil {
		h.stop(canonicalPath(root), s, "Reconnect to return to the previous conversation.")
	}
	return err
}

func (h *researchHost) openConversation(s *researchSession, work, id, notice string) error {
	params := map[string]any{"cwd": work, "mcpServers": []any{}}
	method := "session/new"
	if id != "" {
		method = "session/load"
		params["sessionId"] = id
	}

	s.updateMu.Lock()
	s.replay = []json.RawMessage{}
	s.commands = nil
	s.extensions = nil
	s.updateMu.Unlock()
	var opened struct {
		SessionID     string
		ConfigOptions []ResearchConfig
	}
	err := s.agent.call(method, params, &opened)
	clearReplay := func() {
		s.updateMu.Lock()
		s.replay = nil
		s.updateMu.Unlock()
	}
	if err != nil {
		clearReplay()
		return err
	}
	if id == "" {
		id = opened.SessionID
		if id == "" {
			clearReplay()
			return errors.New("Feynman returned an empty session ID.")
		}
	}
	if err := h.rememberConversation(s, work, id); err != nil {
		clearReplay()
		return err
	}
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	replay := s.replay
	s.replay = nil
	s.emit(ResearchUpdate{Kind: "reset"})
	if notice != "" {
		s.emit(ResearchUpdate{Kind: "notice", Text: notice})
	}
	h.setResearchConfig(s, opened.ConfigOptions)
	for _, raw := range replay {
		h.update(s, "session/update", raw)
	}
	return nil
}

func (h *researchHost) rememberConversation(s *researchSession, work, id string) error {
	h.mu.Lock()
	select {
	case <-s.stopped:
		h.mu.Unlock()
		return errors.New("Feynman disconnected.")
	default:
	}
	h.mu.Unlock()

	path := filepath.Join(filepath.Dir(work), "session")
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if string(current) != id {
		if err := writeFileAtomicMode(path, []byte(id), 0600); err != nil {
			return err
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-s.stopped:
		return errors.New("Feynman disconnected.")
	default:
	}
	s.id = id
	return nil
}

func (h *researchHost) DeleteSession(ctx context.Context, root, id string) error {
	s, currentID, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	defer release()
	if id == "" || id == currentID {
		return errors.New("Switch to another conversation before deleting this one.")
	}
	if err := checkWorkspaceConversation(s, researchWorkDir(h.dataDir, root), id); err != nil {
		return err
	}
	if err := s.agent.call("session/delete", map[string]string{"sessionId": id}, nil); err != nil {
		return err
	}
	remaining, err := workspaceConversations(s, researchWorkDir(h.dataDir, root), "")
	if err == nil && slices.ContainsFunc(remaining, func(c ResearchConversation) bool { return c.ID == id }) {
		return errors.New("Feynman could not delete this conversation.")
	}
	return err
}
