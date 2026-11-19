package feynman

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"slices"
	"strings"
	"unicode/utf8"
)

type ResearchCommandInput struct {
	Hint string `json:"hint"`
}

type ResearchCommand struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Source      string                `json:"source,omitempty"`
	Input       *ResearchCommandInput `json:"input,omitempty"`
}

type ResearchConfigValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ResearchConfig struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Type         string                `json:"type"`
	Category     string                `json:"category,omitempty"`
	Description  string                `json:"description,omitempty"`
	CurrentValue string                `json:"currentValue"`
	Options      []ResearchConfigValue `json:"options"`
}

type ResearchToolLocation struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

type ResearchToolDiff struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

type ResearchTool struct {
	ToolCallID    string                 `json:"toolCallId"`
	Title         string                 `json:"title,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Locations     []ResearchToolLocation `json:"locations,omitempty"`
	Input         string                 `json:"input,omitempty"`
	Output        string                 `json:"output,omitempty"`
	TerminalDelta string                 `json:"terminalDelta,omitempty"`
	ExitCode      *int                   `json:"exitCode,omitempty"`
	Images        []string               `json:"images,omitempty"`
	Diffs         []ResearchToolDiff     `json:"diffs,omitempty"`
}

type ResearchPlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

func (h *ResearchManager) setResearchConfig(s *researchSession, config []ResearchConfig) {
	h.mu.Lock()
	s.config = config
	h.mu.Unlock()
	s.emit(ResearchUpdate{Kind: "config", Config: config})
}

func (h *ResearchManager) SetConfig(ctx context.Context, root, id, value string) error {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return s.startErr
		}
	default:
		h.mu.Unlock()
		return errors.New("Feynman is connecting.")
	}
	if s.busy || s.configuring || s.switching || s.id == "" {
		h.mu.Unlock()
		return errors.New("Wait for the current research operation to finish.")
	}
	if !allowedResearchSetting(s.config, id, value) {
		h.mu.Unlock()
		return errors.New("Choose an available Feynman setting.")
	}
	s.configuring = true
	s.turn = make(chan struct{})
	sessionID := s.id
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		s.configuring = false
		close(s.turn)
		h.mu.Unlock()
	}()
	var result struct{ ConfigOptions []ResearchConfig }
	if err := s.agent.Call("session/set_config_option", map[string]string{"sessionId": sessionID, "configId": id, "value": value}, &result); err != nil {
		return err
	}
	if result.ConfigOptions != nil {
		h.setResearchConfig(s, result.ConfigOptions)
	}
	return nil
}

func allowedResearchSetting(options []ResearchConfig, id, value string) bool {
	for _, option := range options {
		if option.ID == id && option.Type == "select" {
			for _, choice := range option.Options {
				if choice.Value == value {
					return true
				}
			}
		}
	}
	return false
}

type researchContentBlock struct {
	Type, Text, Name, URI, Data, MimeType string
	Resource                              struct{ URI, MimeType, Text string }
}

func researchRasterMIME(data []byte) string {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width*cfg.Height > 16<<20 {
		return ""
	}
	switch format {
	case "png", "jpeg", "gif":
		return "image/" + format
	}
	return ""
}

func (b researchContentBlock) display() (string, string) {
	switch b.Type {
	case "text":
		return b.Text, ""
	case "image":
		const maxImageBytes = 1 << 20
		if len(b.Data) <= base64.StdEncoding.EncodedLen(maxImageBytes) {
			data, err := base64.StdEncoding.DecodeString(b.Data)
			if err == nil && len(data) <= maxImageBytes {
				if mime := researchRasterMIME(data); mime != "" && mime == b.MimeType {
					return "Image (" + mime + ")", "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
				}
			}
		}
		return "Image preview unavailable (" + researchPreview(b.MimeType) + ")", ""
	case "resource_link":
		return strings.TrimSpace(b.Name + " " + researchPreview(b.URI)), ""
	case "resource":
		return strings.TrimSpace(researchPreview(b.Resource.URI) + "\n" + researchPreview(b.Resource.Text)), ""
	case "audio":
		return "Audio attachment (" + researchPreview(b.MimeType) + ")", ""
	default:
		return "", ""
	}
}

func decodeResearchUpdate(raw json.RawMessage) ResearchUpdate {
	var u struct {
		SessionUpdate, MessageID, Title string
		Content                         json.RawMessage
		AvailableCommands               []ResearchCommand
		ConfigOptions                   []ResearchConfig
		Entries                         []ResearchPlanEntry
		Used, Size                      int64
	}
	if json.Unmarshal(raw, &u) != nil {
		return ResearchUpdate{}
	}
	switch u.SessionUpdate {
	case "agent_message_chunk", "user_message_chunk", "agent_thought_chunk":
		var content researchContentBlock
		if json.Unmarshal(u.Content, &content) == nil {
			text, preview := content.display()
			if text == "" && preview == "" {
				return ResearchUpdate{}
			}
			role := "assistant"
			if u.SessionUpdate == "user_message_chunk" {
				role = "user"
			} else if u.SessionUpdate == "agent_thought_chunk" {
				role = "thought"
			}
			return ResearchUpdate{Kind: role, Text: text, Image: preview, MessageID: u.MessageID}
		}
	case "available_commands_update":
		return ResearchUpdate{Kind: "commands", Commands: u.AvailableCommands}
	case "config_option_update":
		return ResearchUpdate{Kind: "config", Config: u.ConfigOptions}
	case "tool_call", "tool_call_update":
		if tool := decodeResearchTool(raw); tool != nil {
			return ResearchUpdate{Kind: "tool", Tool: tool}
		}
	case "plan":
		return ResearchUpdate{Kind: "plan", Plan: u.Entries}
	case "session_info_update":
		if u.Title != "" {
			return ResearchUpdate{Kind: "session", Text: u.Title}
		}
	case "usage_update":
		if u.Size > 0 && u.Used >= 0 {
			return ResearchUpdate{Kind: "usage", Used: u.Used, Size: u.Size}
		}
	}
	return ResearchUpdate{}
}

func decodeResearchTool(raw json.RawMessage) *ResearchTool {
	var wire struct {
		ResearchTool
		RawInput, RawOutput json.RawMessage
		Content             []struct {
			Type string
			ResearchToolDiff
			Content researchContentBlock
		}
		Meta struct {
			TerminalOutput struct{ Data string } `json:"terminal_output"`
			TerminalExit   struct {
				Code *int `json:"exit_code"`
			} `json:"terminal_exit"`
		} `json:"_meta"`
	}
	if json.Unmarshal(raw, &wire) != nil || wire.ToolCallID == "" {
		return nil
	}
	t := &wire.ResearchTool
	t.Input = researchJSONPreview(wire.RawInput)
	var content []researchContentBlock
	for _, block := range wire.Content {
		switch block.Type {
		case "diff":
			diff := block.ResearchToolDiff
			diff.OldText = researchPreview(diff.OldText)
			diff.NewText = researchPreview(diff.NewText)
			t.Diffs = append(t.Diffs, diff)
		case "content":
			content = append(content, block.Content)
		}
	}
	if len(wire.RawOutput) > 0 && bytes.Contains(wire.RawOutput, []byte(`"image"`)) {
		var result struct{ Content []researchContentBlock }
		if json.Unmarshal(wire.RawOutput, &result) == nil && slices.ContainsFunc(result.Content, func(b researchContentBlock) bool { return b.Type == "image" }) {
			content = result.Content
		}
	}
	var parts []string
	for _, block := range content {
		output, preview := block.display()
		if output != "" {
			parts = append(parts, output)
		}
		if preview != "" {
			t.Images = append(t.Images, preview)
		}
	}
	t.Output = researchPreview(strings.Join(parts, "\n"))
	if t.Output == "" {
		t.Output = researchJSONPreview(wire.RawOutput)
	}
	t.TerminalDelta = researchPreview(wire.Meta.TerminalOutput.Data)
	t.ExitCode = wire.Meta.TerminalExit.Code
	return t
}

func researchJSONPreview(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return researchPreview(text)
		}
	}
	return researchPreview(string(raw))
}

func researchPreview(s string) string {
	const limit = 16 << 10
	if len(s) <= limit {
		return s
	}
	end := limit
	for !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "\n…"
}
