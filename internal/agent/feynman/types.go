package feynman

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unicode/utf8"
)

type ResearchCommandSourceInfo struct {
	Path          string `json:"path,omitempty"`
	Source        string `json:"source,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Origin        string `json:"origin,omitempty"`
	BaseDir       string `json:"baseDir,omitempty"`
	ChangelogPath string `json:"changelogPath,omitempty"`
}

type ResearchCommandInput struct {
	Hint string `json:"hint"`
}

type ResearchCommand struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Source      string                     `json:"source,omitempty"`
	SourceInfo  *ResearchCommandSourceInfo `json:"sourceInfo,omitempty"`
	Input       *ResearchCommandInput      `json:"input,omitempty"`
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
	DurationMs    int64                  `json:"durationMs,omitempty"`
	Images        []string               `json:"images,omitempty"`
	Diffs         []ResearchToolDiff     `json:"diffs,omitempty"`
}

type ResearchPlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

type ResearchOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

type ResearchModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total,omitempty"`
}

type ResearchModel struct {
	ID            string             `json:"id"`
	Name          string             `json:"name,omitempty"`
	API           string             `json:"api,omitempty"`
	Provider      string             `json:"provider"`
	BaseURL       string             `json:"baseUrl,omitempty"`
	Reasoning     bool               `json:"reasoning,omitempty"`
	Input         []string           `json:"input,omitempty"`
	ContextWindow int64              `json:"contextWindow,omitempty"`
	MaxTokens     int64              `json:"maxTokens,omitempty"`
	Cost          *ResearchModelCost `json:"cost,omitempty"`
}

type ResearchSessionState struct {
	Model                 *ResearchModel `json:"model,omitempty"`
	ThinkingLevel         string         `json:"thinkingLevel"`
	IsStreaming           bool           `json:"isStreaming"`
	IsCompacting          bool           `json:"isCompacting"`
	SteeringMode          string         `json:"steeringMode,omitempty"`
	FollowUpMode          string         `json:"followUpMode,omitempty"`
	SessionFile           string         `json:"sessionFile,omitempty"`
	SessionID             string         `json:"sessionId"`
	SessionName           string         `json:"sessionName,omitempty"`
	AutoCompactionEnabled bool           `json:"autoCompactionEnabled"`
	MessageCount          int            `json:"messageCount"`
	PendingMessageCount   int            `json:"pendingMessageCount"`
}

type ResearchQueueState struct {
	Steering []string `json:"steering"`
	FollowUp []string `json:"followUp"`
}

type ResearchTokenStats struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
	Total      int64 `json:"total"`
}

type ResearchContextUsage struct {
	Tokens        int64   `json:"tokens"`
	ContextWindow int64   `json:"contextWindow"`
	Percent       float64 `json:"percent"`
}

type ResearchSessionStats struct {
	SessionFile       string                `json:"sessionFile,omitempty"`
	SessionID         string                `json:"sessionId"`
	UserMessages      int                   `json:"userMessages"`
	AssistantMessages int                   `json:"assistantMessages"`
	ToolCalls         int                   `json:"toolCalls"`
	ToolResults       int                   `json:"toolResults"`
	TotalMessages     int                   `json:"totalMessages"`
	Tokens            ResearchTokenStats    `json:"tokens"`
	Cost              float64               `json:"cost"`
	ContextUsage      *ResearchContextUsage `json:"contextUsage,omitempty"`
}

type ResearchCompactionResult struct {
	Summary              string `json:"summary"`
	FirstKeptEntryID     string `json:"firstKeptEntryId,omitempty"`
	TokensBefore         int64  `json:"tokensBefore"`
	EstimatedTokensAfter int64  `json:"estimatedTokensAfter,omitempty"`
}

type ResearchForkMessage struct {
	EntryID string `json:"entryId"`
	Text    string `json:"text"`
}

type ResearchForkResult struct {
	Text      string `json:"text,omitempty"`
	Cancelled bool   `json:"cancelled"`
	SessionID string `json:"sessionId,omitempty"`
}

type ResearchTreeEntry struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	ParentID      string `json:"parentId,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Role          string `json:"role,omitempty"`
	Text          string `json:"text,omitempty"`
	Name          string `json:"name,omitempty"`
	Summary       string `json:"summary,omitempty"`
	CustomType    string `json:"customType,omitempty"`
	Provider      string `json:"provider,omitempty"`
	ModelID       string `json:"modelId,omitempty"`
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

type ResearchTreeNode struct {
	Entry          ResearchTreeEntry  `json:"entry"`
	Children       []ResearchTreeNode `json:"children"`
	Label          string             `json:"label,omitempty"`
	LabelTimestamp string             `json:"labelTimestamp,omitempty"`
}

type ResearchTree struct {
	Tree   []ResearchTreeNode `json:"tree"`
	LeafID string             `json:"leafId,omitempty"`
}

type ResearchConversation struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Name          string `json:"name,omitempty"`
	Preview       string `json:"preview,omitempty"`
	Cwd           string `json:"cwd,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
	ParentSession string `json:"parentSession,omitempty"`
	SessionFile   string `json:"sessionFile,omitempty"`
	Current       bool   `json:"current"`
}

type ResearchUpdate struct {
	Root             string                    `json:"root"`
	Kind             string                    `json:"kind"`
	EventType        string                    `json:"eventType,omitempty"`
	UIMethod         string                    `json:"uiMethod,omitempty"`
	Title            string                    `json:"title,omitempty"`
	Message          string                    `json:"message,omitempty"`
	Text             string                    `json:"text,omitempty"`
	Placeholder      string                    `json:"placeholder,omitempty"`
	Prefill          string                    `json:"prefill,omitempty"`
	NotifyType       string                    `json:"notifyType,omitempty"`
	StatusKey        string                    `json:"statusKey,omitempty"`
	WidgetKey        string                    `json:"widgetKey,omitempty"`
	WidgetLines      []string                  `json:"widgetLines,omitempty"`
	WidgetPlacement  string                    `json:"widgetPlacement,omitempty"`
	Timeout          int64                     `json:"timeout,omitempty"`
	Commands         []ResearchCommand         `json:"commands,omitempty"`
	Config           []ResearchConfig          `json:"config,omitempty"`
	Models           []ResearchModel           `json:"models,omitempty"`
	ThinkingLevels   []string                  `json:"thinkingLevels,omitempty"`
	ThinkingLevel    string                    `json:"thinkingLevel,omitempty"`
	State            *ResearchSessionState     `json:"state,omitempty"`
	Queue            *ResearchQueueState       `json:"queue,omitempty"`
	Stats            *ResearchSessionStats     `json:"stats,omitempty"`
	Compaction       *ResearchCompactionResult `json:"compaction,omitempty"`
	Reason           string                    `json:"reason,omitempty"`
	WillRetry        bool                      `json:"willRetry,omitempty"`
	Aborted          bool                      `json:"aborted,omitempty"`
	RetryAttempt     int                       `json:"retryAttempt,omitempty"`
	RetryMaxAttempts int                       `json:"retryMaxAttempts,omitempty"`
	RetryDelayMs     int64                     `json:"retryDelayMs,omitempty"`
	RetrySuccess     bool                      `json:"retrySuccess,omitempty"`
	Tool             *ResearchTool             `json:"tool,omitempty"`
	Plan             []ResearchPlanEntry       `json:"plan,omitempty"`
	Used             int64                     `json:"used,omitempty"`
	Size             int64                     `json:"size,omitempty"`
	Image            string                    `json:"image,omitempty"`
	MessageID        string                    `json:"messageId,omitempty"`
	Key              string                    `json:"key,omitempty"`
	RequestID        string                    `json:"requestId,omitempty"`
	Options          []ResearchOption          `json:"options,omitempty"`
}

type researchContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Name      string          `json:"name,omitempty"`
	URI       string          `json:"uri,omitempty"`
	Data      string          `json:"data,omitempty"`
	MimeType  string          `json:"mimeType,omitempty"`
	ID        string          `json:"id,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Resource  struct {
		URI      string `json:"uri,omitempty"`
		MimeType string `json:"mimeType,omitempty"`
		Text     string `json:"text,omitempty"`
	} `json:"resource,omitempty"`
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
	case "thinking":
		return b.Thinking, ""
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
