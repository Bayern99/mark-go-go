package feynman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

func (h *ResearchManager) sessionsDirFor(s *researchSession) string {
	if s != nil && s.sessionsDir != "" {
		return s.sessionsDir
	}
	_, dir, err := resolveFeynmanDirs()
	if err != nil {
		return ""
	}
	return dir
}

func collectSessionJSONLFiles(sessionsDir string) []string {
	if sessionsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		full := filepath.Join(sessionsDir, name)
		if !entry.IsDir() && strings.HasSuffix(name, ".jsonl") {
			files = append(files, full)
			continue
		}
		if entry.IsDir() && strings.HasPrefix(name, "--") {
			subEntries, err := os.ReadDir(full)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if !sub.IsDir() && strings.HasSuffix(sub.Name(), ".jsonl") {
					files = append(files, filepath.Join(full, sub.Name()))
				}
			}
		}
	}
	return files
}

func parseWorkspaceSessionJSONL(filePath, work string) (ResearchConversation, bool) {
	info, err := os.Stat(filePath)
	if err != nil || !info.Mode().IsRegular() {
		return ResearchConversation{}, false
	}
	f, err := os.Open(filePath)
	if err != nil {
		return ResearchConversation{}, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	sc.Split(splitJSONLLines)

	var (
		gotHeader       bool
		id              string
		cwd             string
		headerTimestamp string
		parentSession   string
		latestName      string
		firstPreview    string
		latestActivity  string
	)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		if !gotHeader {
			var hdr struct {
				Type          string `json:"type"`
				ID            string `json:"id"`
				Cwd           string `json:"cwd"`
				Timestamp     string `json:"timestamp"`
				ParentSession string `json:"parentSession"`
			}
			if json.Unmarshal(line, &hdr) != nil {
				continue
			}
			if hdr.Type != "session" || hdr.ID == "" {
				return ResearchConversation{}, false
			}
			if hdr.Cwd == "" || !filepath.IsAbs(hdr.Cwd) || !fsutil.SameRoot(hdr.Cwd, work) {
				return ResearchConversation{}, false
			}
			gotHeader = true
			id = hdr.ID
			cwd = hdr.Cwd
			headerTimestamp = hdr.Timestamp
			parentSession = hdr.ParentSession
			continue
		}

		var entry struct {
			Type      string          `json:"type"`
			Timestamp string          `json:"timestamp"`
			Name      string          `json:"name"`
			Message   json.RawMessage `json:"message"`
		}
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		switch entry.Type {
		case "session_info":
			latestName = strings.TrimSpace(entry.Name)
			if entry.Timestamp != "" {
				latestActivity = entry.Timestamp
			}
		case "message":
			if entry.Timestamp != "" {
				latestActivity = entry.Timestamp
			}
			if len(entry.Message) > 0 {
				var msg struct {
					Role      string          `json:"role"`
					Content   json.RawMessage `json:"content"`
					Timestamp int64           `json:"timestamp"`
				}
				if json.Unmarshal(entry.Message, &msg) == nil {
					if latestActivity == "" && msg.Timestamp > 0 {
						latestActivity = time.UnixMilli(msg.Timestamp).UTC().Format(time.RFC3339Nano)
					}
					if firstPreview == "" && msg.Role == "user" {
						firstPreview = cleanMessagePreview(extractRawContentText(msg.Content))
					}
				}
			}
		}
	}

	if !gotHeader {
		return ResearchConversation{}, false
	}

	updatedAt := latestActivity
	if updatedAt == "" {
		updatedAt = headerTimestamp
	}
	if updatedAt == "" {
		updatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}

	title := latestName
	if title == "" {
		title = firstPreview
	}

	return ResearchConversation{
		ID:            id,
		Title:         title,
		Name:          latestName,
		Preview:       firstPreview,
		Cwd:           cwd,
		Timestamp:     headerTimestamp,
		UpdatedAt:     updatedAt,
		ParentSession: parentSession,
		SessionFile:   fsutil.CanonicalPath(filePath),
	}, true
}

func extractRawContentText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	if raw[0] == '[' {
		var blocks []researchContentBlock
		if json.Unmarshal(raw, &blocks) == nil {
			var parts []string
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					parts = append(parts, b.Text)
				}
			}
			return strings.Join(parts, " ")
		}
	}
	return ""
}

func cleanMessagePreview(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	joined := strings.Join(fields, " ")
	const maxRunes = 160
	if utf8.RuneCountInString(joined) <= maxRunes {
		return joined
	}
	runes := []rune(joined)
	return string(runes[:maxRunes]) + "…"
}

func workspaceConversations(sessionsDir, work, currentID, currentName string) ([]ResearchConversation, error) {
	files := collectSessionJSONLFiles(sessionsDir)
	byID := make(map[string]ResearchConversation, len(files))
	for _, file := range files {
		conv, ok := parseWorkspaceSessionJSONL(file, work)
		if !ok {
			continue
		}
		conv.Current = conv.ID == currentID
		if existing, exists := byID[conv.ID]; exists && existing.UpdatedAt > conv.UpdatedAt {
			continue
		}
		byID[conv.ID] = conv
	}

	result := make([]ResearchConversation, 0, len(byID)+1)
	for _, conv := range byID {
		result = append(result, conv)
	}
	slices.SortFunc(result, func(a, b ResearchConversation) int {
		if a.Current != b.Current {
			if a.Current {
				return -1
			}
			return 1
		}
		if a.UpdatedAt != b.UpdatedAt {
			return strings.Compare(b.UpdatedAt, a.UpdatedAt)
		}
		return strings.Compare(a.ID, b.ID)
	})

	if currentID != "" && !slices.ContainsFunc(result, func(c ResearchConversation) bool { return c.Current }) {
		result = append([]ResearchConversation{{
			ID:      currentID,
			Title:   currentName,
			Name:    currentName,
			Cwd:     work,
			Current: true,
		}}, result...)
	}
	return result, nil
}

func findWorkspaceConversation(sessionsDir, work, target string) (ResearchConversation, bool, error) {
	list, err := workspaceConversations(sessionsDir, work, "", "")
	if err != nil {
		return ResearchConversation{}, false, err
	}
	for _, c := range list {
		if c.ID == target || (filepath.IsAbs(target) && c.SessionFile != "" && fsutil.SameFile(c.SessionFile, target)) {
			return c, true, nil
		}
	}
	return ResearchConversation{}, false, nil
}

func (h *ResearchManager) Sessions(ctx context.Context, root string) ([]ResearchConversation, error) {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	id := s.id
	name := s.sessionName
	sessionsDir := h.sessionsDirFor(s)
	h.mu.Unlock()
	return workspaceConversations(sessionsDir, review.WorkDir(h.dataDir, root), id, name)
}

func (h *ResearchManager) beginConversationChange(ctx context.Context, root string) (*researchSession, string, func(), error) {
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

func (h *ResearchManager) SelectSession(ctx context.Context, root, id string) error {
	if id == "" {
		return h.NewSession(ctx, root, "")
	}
	return h.SwitchSession(ctx, root, id)
}

func (h *ResearchManager) NewSession(ctx context.Context, root, parentSession string) error {
	s, _, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	work := review.WorkDir(h.dataDir, root)
	err = h.openNewConversation(s, work, parentSession, "")
	if err != nil {
		h.mu.Lock()
		s.startErr = err
		h.mu.Unlock()
	}
	release()
	if err != nil {
		h.stop(fsutil.CanonicalPath(root), s, "Reconnect to return to the previous conversation.")
	}
	return err
}

func (h *ResearchManager) SwitchSession(ctx context.Context, root, target string) error {
	if target == "" {
		return h.NewSession(ctx, root, "")
	}
	s, currentID, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	work := review.WorkDir(h.dataDir, root)
	if target == currentID {
		release()
		return nil
	}
	conv, ok, err := findWorkspaceConversation(h.sessionsDirFor(s), work, target)
	if err != nil {
		release()
		return err
	}
	if !ok || conv.SessionFile == "" {
		release()
		return errors.New("Choose a conversation from this workspace.")
	}
	err = h.switchConversationFile(s, work, conv.SessionFile, conv.ID, "")
	if err != nil {
		h.mu.Lock()
		s.startErr = err
		h.mu.Unlock()
	}
	release()
	if err != nil {
		h.stop(fsutil.CanonicalPath(root), s, "Reconnect to return to the previous conversation.")
	}
	return err
}

func (h *ResearchManager) openNewConversation(s *researchSession, work, parentSession, notice string) error {
	fields := map[string]any{}
	if parentSession != "" {
		fields["parentSession"] = parentSession
	}
	var res struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := s.rpc.Call("new_session", fields, &res); err != nil {
		return err
	}
	if res.Cancelled {
		return errors.New("Feynman cancelled starting a new conversation.")
	}
	return h.syncConversationState(s, work, "", notice)
}

func (h *ResearchManager) switchConversationFile(s *researchSession, work, sessionFile, expectedID, notice string) error {
	var res struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := s.rpc.Call("switch_session", map[string]any{"sessionPath": sessionFile}, &res); err != nil {
		return err
	}
	if res.Cancelled {
		return errors.New("Feynman cancelled switching conversations.")
	}
	return h.syncConversationState(s, work, expectedID, notice)
}

func (h *ResearchManager) syncConversationState(s *researchSession, work, fallbackID, notice string) error {
	var state ResearchSessionState
	if err := s.rpc.Call("get_state", nil, &state); err != nil {
		return err
	}
	id := state.SessionID
	if id == "" {
		id = fallbackID
	}
	if id == "" {
		return errors.New("Feynman returned an empty session ID.")
	}
	if err := h.rememberConversation(s, work, id, state.SessionFile, state.SessionName); err != nil {
		return err
	}

	var cmdsResp struct {
		Commands []ResearchCommand `json:"commands"`
	}
	_ = s.rpc.Call("get_commands", nil, &cmdsResp)
	filteredCmds := filterResearchCommands(cmdsResp.Commands)

	var modelsResp struct {
		Models []ResearchModel `json:"models"`
	}
	_ = s.rpc.Call("get_available_models", nil, &modelsResp)

	var levelsResp struct {
		Levels []string `json:"levels"`
	}
	_ = s.rpc.Call("get_available_thinking_levels", nil, &levelsResp)

	var msgsResp struct {
		Messages []json.RawMessage `json:"messages"`
	}
	_ = s.rpc.Call("get_messages", nil, &msgsResp)

	var stats ResearchSessionStats
	hasStats := s.rpc.Call("get_session_stats", nil, &stats) == nil

	h.mu.Lock()
	s.state = state
	s.models = modelsResp.Models
	s.thinkingLevels = levelsResp.Levels
	s.thinkingLevel = state.ThinkingLevel
	s.commands = filteredCmds
	s.imagePrompts = modelSupportsImages(state.Model, modelsResp.Models)
	cfg := buildResearchConfig(state, modelsResp.Models, levelsResp.Levels)
	s.config = cfg
	h.mu.Unlock()

	s.emit(ResearchUpdate{Kind: "reset"})
	if notice != "" {
		s.emit(ResearchUpdate{Kind: "notice", Text: notice})
	}
	if state.SessionName != "" {
		s.emit(ResearchUpdate{Kind: "session", Text: state.SessionName})
	}
	if len(cfg) > 0 {
		s.emit(ResearchUpdate{
			Kind:           "config",
			Config:         cfg,
			Models:         modelsResp.Models,
			ThinkingLevels: levelsResp.Levels,
			ThinkingLevel:  state.ThinkingLevel,
			State:          &state,
		})
	}
	s.emit(ResearchUpdate{Kind: "commands", Commands: filteredCmds})
	if hasStats {
		s.emitStatsUpdate(stats)
	}
	for _, rawMsg := range msgsResp.Messages {
		s.replayMessage(rawMsg)
	}
	return nil
}

func (h *ResearchManager) rememberConversation(s *researchSession, work, id, sessionFile, sessionName string) error {
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
		if err := fsutil.WriteFileAtomicMode(path, []byte(id), 0600); err != nil {
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
	if sessionFile != "" {
		s.sessionFile = sessionFile
	}
	s.sessionName = sessionName
	return nil
}

func (h *ResearchManager) DeleteSession(ctx context.Context, root, id string) error {
	s, currentID, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	defer release()
	if id == "" || id == currentID {
		return errors.New("Switch to another conversation before deleting this one.")
	}
	work := review.WorkDir(h.dataDir, root)
	sessionsDir := h.sessionsDirFor(s)
	conv, ok, err := findWorkspaceConversation(sessionsDir, work, id)
	if err != nil {
		return err
	}
	if !ok || conv.SessionFile == "" {
		return errors.New("Choose a conversation from this workspace.")
	}
	if err := os.Remove(conv.SessionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	remaining, err := workspaceConversations(sessionsDir, work, "", "")
	if err == nil && slices.ContainsFunc(remaining, func(c ResearchConversation) bool { return c.ID == id }) {
		return errors.New("Feynman could not delete this conversation.")
	}
	return err
}

func (h *ResearchManager) SetSessionName(ctx context.Context, root, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("Enter a conversation name.")
	}
	s, _, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return err
	}
	defer release()
	if err := s.rpc.Call("set_session_name", map[string]any{"name": name}, nil); err != nil {
		return err
	}
	h.mu.Lock()
	s.sessionName = name
	s.state.SessionName = name
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "session", EventType: "set_session_name", Text: name})
	return nil
}

func (h *ResearchManager) ForkMessages(ctx context.Context, root string) ([]ResearchForkMessage, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return nil, err
	}
	var res struct {
		Messages []ResearchForkMessage `json:"messages"`
	}
	if err := s.rpc.Call("get_fork_messages", nil, &res); err != nil {
		return nil, err
	}
	return res.Messages, nil
}

func (h *ResearchManager) Fork(ctx context.Context, root, entryID string) (ResearchForkResult, error) {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return ResearchForkResult{}, errors.New("Choose a message entry to fork from.")
	}
	s, _, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return ResearchForkResult{}, err
	}
	defer release()
	var res ResearchForkResult
	if err := s.rpc.Call("fork", map[string]any{"entryId": entryID}, &res); err != nil {
		return ResearchForkResult{}, err
	}
	if !res.Cancelled {
		work := review.WorkDir(h.dataDir, root)
		if err := h.syncConversationState(s, work, "", ""); err != nil {
			return ResearchForkResult{}, err
		}
		h.mu.Lock()
		res.SessionID = s.id
		h.mu.Unlock()
		if res.Text != "" {
			s.emit(ResearchUpdate{Kind: "editorText", UIMethod: "set_editor_text", Text: res.Text})
		}
	}
	return res, nil
}

func (h *ResearchManager) Clone(ctx context.Context, root string) (bool, error) {
	s, _, release, err := h.beginConversationChange(ctx, root)
	if err != nil {
		return false, err
	}
	defer release()
	var res struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := s.rpc.Call("clone", nil, &res); err != nil {
		return false, err
	}
	if !res.Cancelled {
		work := review.WorkDir(h.dataDir, root)
		if err := h.syncConversationState(s, work, "", ""); err != nil {
			return false, err
		}
	}
	return !res.Cancelled, nil
}

type wireTreeNode struct {
	Entry struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		ParentID      *string         `json:"parentId"`
		Timestamp     string          `json:"timestamp"`
		Name          string          `json:"name"`
		Summary       string          `json:"summary"`
		CustomType    string          `json:"customType"`
		Provider      string          `json:"provider"`
		ModelID       string          `json:"modelId"`
		ThinkingLevel string          `json:"thinkingLevel"`
		Content       json.RawMessage `json:"content"`
		Message       struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"entry"`
	Children       []wireTreeNode `json:"children"`
	Label          string         `json:"label"`
	LabelTimestamp string         `json:"labelTimestamp"`
}

func convertWireTreeNode(w wireTreeNode) ResearchTreeNode {
	parentID := ""
	if w.Entry.ParentID != nil {
		parentID = *w.Entry.ParentID
	}
	text := extractRawContentText(w.Entry.Message.Content)
	if text == "" {
		text = extractRawContentText(w.Entry.Content)
	}
	children := make([]ResearchTreeNode, len(w.Children))
	for i, child := range w.Children {
		children[i] = convertWireTreeNode(child)
	}
	return ResearchTreeNode{
		Entry: ResearchTreeEntry{
			Type:          w.Entry.Type,
			ID:            w.Entry.ID,
			ParentID:      parentID,
			Timestamp:     w.Entry.Timestamp,
			Role:          w.Entry.Message.Role,
			Text:          cleanMessagePreview(text),
			Name:          w.Entry.Name,
			Summary:       w.Entry.Summary,
			CustomType:    w.Entry.CustomType,
			Provider:      w.Entry.Provider,
			ModelID:       w.Entry.ModelID,
			ThinkingLevel: w.Entry.ThinkingLevel,
		},
		Children:       children,
		Label:          w.Label,
		LabelTimestamp: w.LabelTimestamp,
	}
}

func (h *ResearchManager) Tree(ctx context.Context, root string) (ResearchTree, error) {
	s, err := h.readySession(ctx, root)
	if err != nil {
		return ResearchTree{}, err
	}
	var raw struct {
		Tree   []wireTreeNode `json:"tree"`
		LeafID *string        `json:"leafId"`
	}
	if err := s.rpc.Call("get_tree", nil, &raw); err != nil {
		return ResearchTree{}, err
	}
	nodes := make([]ResearchTreeNode, len(raw.Tree))
	for i, n := range raw.Tree {
		nodes[i] = convertWireTreeNode(n)
	}
	leafID := ""
	if raw.LeafID != nil {
		leafID = *raw.LeafID
	}
	return ResearchTree{Tree: nodes, LeafID: leafID}, nil
}
