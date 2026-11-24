package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

type connectionStatus string

const (
	statusOffline    connectionStatus = "offline"
	statusConnecting connectionStatus = "connecting"
	statusReady      connectionStatus = "ready"
	statusWorking    connectionStatus = "working"
)

type message struct {
	kind      string
	text      string
	messageID string
	image     string
	notify    string
	tool      feynman.ResearchTool
	steps     []feynman.ResearchPlanEntry
}

type paneState struct {
	root          string
	status        connectionStatus
	error         string
	progress      string
	title         string
	messages      []message
	commands      []feynman.ResearchCommand
	configs       []feynman.ResearchConfig
	models        []feynman.ResearchModel
	thinking      []string
	thinkingLevel string
	session       feynman.ResearchSessionState
	stats         feynman.ResearchSessionStats
	hasStats      bool
	queue         feynman.ResearchQueueState
	permission    *feynman.ResearchUpdate
	suggested     string
	statuses      map[string]string
	widgets       map[string]feynman.ResearchUpdate
	artifacts     []review.ResearchArtifact
	filesLoading  bool
	filesError    string
	filesRequest  uint64
	sources       []string
	draft         string
	failedDraft   string
	failedSources []string
	includeDoc    bool
	selectedTab   string
	actionsOpen   bool
	sessionsOpen  bool
	sessionRows   []feynman.ResearchConversation
	sessionFilter string
	renameDraft   string
	selectedPaper string
	searchQuery   string
	searchMode    string
	searchResult  *feynman.CLIAlphaSearchResult
	paperResult   *feynman.CLIAlphaPaperResult
	cliResult     string
	cliError      string
	cliBusy       bool
	cliStatus     *feynman.CLIStatusResult
	doctorResult  *feynman.CLIDoctorResult
	setupPreview  *feynman.CLICommandResult
	alphaStatus   *feynman.CLIAlphaStatusResult
	searchStatus  *feynman.CLISearchStatusResult
	modelRows     []feynman.ResearchModel
	cliModels     *feynman.CLIModelListResult
	packages      *feynman.CLIPackageListResult
	mcpServers    *feynman.CLIMCPListResult
	inputValues   map[string]string
	alphaAsk      string
	alphaNote     string
	alphaRepo     string
	alphaPath     string
	alphaFullText bool
	alphaOutput   string
}

func newPaneState(root string) *paneState {
	return &paneState{
		root:        fsutil.CanonicalPath(root),
		status:      statusOffline,
		queue:       feynman.ResearchQueueState{},
		statuses:    make(map[string]string),
		widgets:     make(map[string]feynman.ResearchUpdate),
		inputValues: make(map[string]string),
		selectedTab: "chat",
		searchMode:  "relevance",
	}
}

// restoreDraft returns submitted input to the composer after a failure without
// overwriting anything the user typed while the request was in flight. When
// the user already started a new draft, the failed submission and its sources
// stay separately available for recovery instead of being dropped or merged.
func (s *paneState) restoreDraft(rawDraft string, sources []string) {
	if strings.TrimSpace(s.draft) == "" {
		s.draft = rawDraft
		s.sources = mergeSources(sources, s.sources)
		s.failedDraft = ""
		s.failedSources = nil
		return
	}
	s.failedDraft = rawDraft
	s.failedSources = append([]string(nil), sources...)
}

func (s *paneState) resetConversation() {
	s.messages = nil
	s.commands = nil
	s.configs = nil
	s.models = nil
	s.thinking = nil
	s.thinkingLevel = ""
	s.session = feynman.ResearchSessionState{}
	s.stats = feynman.ResearchSessionStats{}
	s.hasStats = false
	s.queue = feynman.ResearchQueueState{}
	s.title = ""
	s.permission = nil
	s.suggested = ""
	s.statuses = make(map[string]string)
	s.widgets = make(map[string]feynman.ResearchUpdate)
	s.progress = ""
	s.sources = nil
	s.failedDraft = ""
	s.failedSources = nil
}

func (s *paneState) receive(update feynman.ResearchUpdate) {
	if update.Root == "" || fsutil.CanonicalPath(update.Root) != s.root {
		return
	}
	text := update.Text
	switch update.Kind {
	case "reset":
		s.resetConversation()
	case "assistant", "user", "thought":
		n := len(s.messages)
		if n > 0 && update.Kind != "user" {
			last := &s.messages[n-1]
			if last.kind == update.Kind && last.messageID == update.MessageID && last.image == "" && update.Image == "" {
				last.text += text
				break
			}
		}
		s.messages = append(s.messages, message{kind: update.Kind, text: text, messageID: update.MessageID, image: update.Image})
	case "notice":
		if text != "" {
			s.messages = append(s.messages, message{kind: "notice", text: text, notify: update.NotifyType})
		}
	case "notify":
		body := update.Message
		if body == "" {
			body = text
		}
		if body != "" {
			s.messages = append(s.messages, message{kind: "notice", text: body, notify: update.NotifyType})
		}
	case "tool":
		if update.Tool != nil {
			s.mergeTool(*update.Tool)
		}
	case "plan":
		if n := len(s.messages); n > 0 && s.messages[n-1].kind == "plan" {
			s.messages[n-1].steps = append([]feynman.ResearchPlanEntry(nil), update.Plan...)
		} else {
			s.messages = append(s.messages, message{kind: "plan", steps: append([]feynman.ResearchPlanEntry(nil), update.Plan...)})
		}
	case "session", "setTitle":
		s.title = update.Title
		if s.title == "" {
			s.title = text
		}
	case "config":
		if update.Config != nil {
			s.configs = append([]feynman.ResearchConfig(nil), update.Config...)
		}
		if update.Models != nil {
			s.models = append([]feynman.ResearchModel(nil), update.Models...)
		}
		if update.ThinkingLevels != nil {
			s.thinking = append([]string(nil), update.ThinkingLevels...)
		}
		if update.ThinkingLevel != "" {
			s.thinkingLevel = update.ThinkingLevel
		}
		if update.State != nil {
			s.session = *update.State
			if s.session.ThinkingLevel != "" {
				s.thinkingLevel = s.session.ThinkingLevel
			}
		}
	case "usage":
		if update.Stats != nil {
			s.stats, s.hasStats = *update.Stats, true
		}
	case "queue":
		if update.Queue != nil {
			s.queue = *update.Queue
		}
	case "progress":
		s.progress = text
		if update.EventType == "agent_start" || update.EventType == "turn_start" {
			s.status = statusWorking
		}
	case "compactionStart":
		s.progress = text
		if s.progress == "" {
			s.progress = "Compacting conversation context…"
		}
	case "compactionEnd":
		s.progress = ""
		if text != "" {
			tone := "error"
			if update.Aborted {
				tone = "warning"
			}
			s.messages = append(s.messages, message{kind: "notice", text: text, notify: tone})
		} else if update.Compaction != nil && update.Compaction.Summary != "" {
			s.messages = append(s.messages, message{kind: "notice", text: "Conversation context compacted."})
		}
	case "retryStart":
		s.progress = text
		if update.RetryAttempt > 0 {
			s.progress = "Retrying (" + itoa(update.RetryAttempt)
			if update.RetryMaxAttempts > 0 {
				s.progress += "/" + itoa(update.RetryMaxAttempts)
			}
			s.progress += ")…"
		} else if s.progress == "" {
			s.progress = "Retrying…"
		}
	case "retryEnd":
		s.progress = ""
		if !update.RetrySuccess && text != "" {
			s.messages = append(s.messages, message{kind: "notice", text: text, notify: "error"})
		}
	case "agentSettled":
		s.progress = ""
		s.status = statusReady
		status := "interrupted"
		if update.Aborted {
			status = "cancelled"
		}
		s.reconcilePendingSteps(status)
	case "commands":
		s.commands = append([]feynman.ResearchCommand(nil), update.Commands...)
	case "setStatus":
		key := update.StatusKey
		if key == "" {
			key = update.Key
		}
		if key != "" {
			if text == "" {
				delete(s.statuses, key)
			} else {
				s.statuses[key] = text
			}
		}
	case "setWidget":
		key := update.WidgetKey
		if key == "" {
			key = update.Key
		}
		if key != "" {
			lines := update.WidgetLines
			if lines == nil && text != "" {
				lines = strings.Split(text, "\n")
			}
			if len(lines) == 0 {
				delete(s.widgets, key)
			} else {
				copy := update
				copy.WidgetLines = append([]string(nil), lines...)
				s.widgets[key] = copy
			}
		}
	case "permission", "select", "confirm", "input", "editor":
		copy := update
		copy.Options = append([]feynman.ResearchOption(nil), update.Options...)
		s.permission = &copy
	case "editorText", "set_editor_text":
		if strings.TrimSpace(s.draft) == "" {
			s.draft = text
		} else if text != "" && text != s.draft {
			s.suggested = text
		}
	case "permissionDone":
		if s.permission != nil && s.permission.RequestID == update.RequestID {
			s.permission = nil
		}
	case "error":
		s.status = statusOffline
		s.progress = ""
		s.permission = nil
		s.statuses = make(map[string]string)
		s.error = text
		s.reconcilePendingSteps("interrupted")
	}
}

func (s *paneState) reconcilePendingSteps(status string) {
	for i := range s.messages {
		item := &s.messages[i]
		switch item.kind {
		case "tool":
			if !terminalResearchStep(item.tool.Status) {
				item.tool.Status = status
			}
		case "plan":
			for j := range item.steps {
				if !terminalResearchStep(item.steps[j].Status) {
					item.steps[j].Status = status
				}
			}
		}
	}
}

func terminalResearchStep(status string) bool {
	switch status {
	case "completed", "failed", "error", "cancelled", "canceled", "interrupted":
		return true
	default:
		return false
	}
}

func (s *paneState) mergeTool(patch feynman.ResearchTool) {
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].kind != "tool" || s.messages[i].tool.ToolCallID != patch.ToolCallID {
			continue
		}
		current := &s.messages[i].tool
		if patch.Title != "" {
			current.Title = patch.Title
		}
		if patch.Kind != "" {
			current.Kind = patch.Kind
		}
		if patch.Status != "" {
			current.Status = patch.Status
		}
		if patch.Input != "" {
			current.Input = patch.Input
		}
		if patch.Output != "" {
			current.Output = patch.Output
		}
		if patch.ExitCode != nil {
			current.ExitCode = patch.ExitCode
		}
		if patch.DurationMs != 0 {
			current.DurationMs = patch.DurationMs
		}
		if patch.Locations != nil {
			current.Locations = append([]feynman.ResearchToolLocation(nil), patch.Locations...)
		}
		if patch.Images != nil {
			current.Images = append([]string(nil), patch.Images...)
		}
		if patch.Diffs != nil {
			current.Diffs = append([]feynman.ResearchToolDiff(nil), patch.Diffs...)
		}
		current.TerminalDelta += patch.TerminalDelta
		if len(current.TerminalDelta) > 65536 {
			current.TerminalDelta = current.TerminalDelta[len(current.TerminalDelta)-65536:]
		}
		return
	}
	s.messages = append(s.messages, message{kind: "tool", tool: patch})
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
