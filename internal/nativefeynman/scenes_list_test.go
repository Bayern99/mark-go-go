package nativefeynman

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativereview"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type shellScene struct {
	name          string
	width, height int
	feynman       bool
	empty         bool
	mode          string
	space         string
	dark          bool
	dialog        string
	act           func(*ui.Tester)
	actSource     func(*ui.Tester, *nativeeditor.Document)
	research      func(*panel)
	document      string
}

func sceneActions(model *nativeui.Model) nativeui.Actions {
	return nativeui.Actions{
		Activate:      func(id string) { model.Workspace.ActiveID = id },
		SetSpace:      func(name string) { model.Spaces = map[string]string{model.Workspace.Root: name} },
		SetAppearance: func(a portrait.Appearance) { model.Appearance = a },
		SetMode: func(mode string) {
			for i := range model.Workspace.Documents {
				if model.Workspace.Documents[i].ID == model.Workspace.ActiveID {
					model.Workspace.Documents[i].ViewMode = mode
				}
			}
		},
	}
}

func sceneList() []shellScene {
	return []shellScene{
		{name: "main", width: 1280, height: 820, mode: "preview"},
		{name: "agent", width: 1440, height: 900, feynman: true, mode: "preview"},
		{name: "markdown", width: 1280, height: 820, mode: "markdown"},
		{name: "outline", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { _ = app.Click("Outline") }},
		{name: "sidebar_hidden", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.Key0) }},
		{name: "command_bar", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.KeyK) }},
		{name: "command_search", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) {
			app.Key(ui.Cmd, ui.KeyK)
			app.Type("sh")
			app.Key(0, ui.KeyDown)
		}},
		{name: "narrow", width: 560, height: 720, mode: "preview"},
		{name: "narrow_drawer", width: 560, height: 720, mode: "preview", act: func(app *ui.Tester) { _ = app.Click("Show navigation") }},
		{name: "welcome", width: 1280, height: 820, empty: true, mode: "preview"},
	}
}

func openSettingsScene(category string) func(*ui.Tester) {
	return func(app *ui.Tester) {
		app.Key(ui.Cmd, ui.KeyComma)
		app.Frame()
		if category != "" && category != "general" {
			_ = app.Click(category)
		}
	}
}

func wuxingScenes() []shellScene {
	var scenes []shellScene
	for _, space := range sceneSpaces() {
		for _, dark := range []bool{false, true} {
			name := space + "_light"
			if dark {
				name = space + "_dark"
			}
			scenes = append(scenes, shellScene{name: name, width: 1440, height: 1100, feynman: true, mode: "preview", space: space, dark: dark})
		}
	}
	scenes = append(scenes,
		shellScene{name: "command_bar_dark", width: 1280, height: 820, mode: "preview", space: "WoodFire", dark: true, act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.KeyK); app.Type("space") }},
		shellScene{name: "markdown_dark", width: 1280, height: 820, mode: "markdown", space: "FiveElements", dark: true},
		shellScene{name: "feynman_picker_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneModels, act: openModelPicker},
		shellScene{name: "feynman_picker_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", dark: true, research: sceneModels, act: openModelPicker},
		shellScene{name: "settings_general_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", act: openSettingsScene("general")},
		shellScene{name: "settings_appearance_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", act: openSettingsScene("Appearance")},
		shellScene{name: "settings_appearance_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", dark: true, act: openSettingsScene("Appearance")},
		shellScene{name: "settings_editor_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", dark: true, act: openSettingsScene("Editor")},
		shellScene{name: "settings_models_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", research: sceneSettingsModal, act: openSettingsScene("Models & Providers")},
		shellScene{name: "settings_models_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "FiveElements", dark: true, research: sceneSettingsModal, act: openSettingsScene("Models & Providers")},
		shellScene{name: "settings_mcp_add_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneSettingsModal, act: func(app *ui.Tester) {
			openSettingsScene("MCP Servers")(app)
			app.Frame()
			_ = app.Click("Open the add MCP server form")
		}},
		shellScene{name: "settings_search_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", dark: true, research: sceneSettingsModal, act: func(app *ui.Tester) { openSettingsScene("general")(app); app.Key(0, ui.KeySlash); app.Type("key") }},
		shellScene{name: "settings_narrow_light", width: 640, height: 720, feynman: true, mode: "preview", space: "WoodFire", act: openSettingsScene("general")},
		shellScene{name: "workspace_menu_light", width: 1280, height: 820, mode: "preview", space: "Wood", act: func(app *ui.Tester) { _ = app.Click("Switch workspace") }},
		shellScene{name: "workspace_menu_dark", width: 1280, height: 820, mode: "preview", space: "Wood", dark: true, act: func(app *ui.Tester) { _ = app.Click("Switch workspace") }},
		shellScene{name: "feynman_history_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", dark: true, research: sceneHistory},
		shellScene{name: "feynman_actions_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneActionCommands, act: func(app *ui.Tester) { _ = app.Click("Browse workflows, skills and extensions") }},
		shellScene{name: "feynman_question_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", research: sceneQuestion},
		shellScene{name: "feynman_files_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", dark: true, research: sceneFiles},
		shellScene{name: "alpha_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneAlpha},
		shellScene{name: "checkpoints_light", width: 1280, height: 820, mode: "preview", space: "Wood", act: func(app *ui.Tester) { _ = app.Click("Checkpoints") }},
		shellScene{name: "proposals_dark", width: 1280, height: 820, mode: "preview", space: "MetalWater", dark: true},
		shellScene{name: "find_bar_dark", width: 1280, height: 820, mode: "markdown", space: "MetalWater", dark: true, act: func(app *ui.Tester) { _ = app.Click("Markdown source"); app.Key(ui.Cmd, ui.KeyF); app.Type("Markdown") }},
		shellScene{name: "genui_request_light", width: 1280, height: 820, mode: "genui", space: "WoodFire"},
		shellScene{name: "sidebar_drilled_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "FiveElements", act: func(app *ui.Tester) { _ = app.Click("Essays") }},
		shellScene{name: "dense_sidebar_genui_failure_light", width: 1680, height: 1127, feynman: true, mode: "genui", space: "Metal", research: sceneNoticeAndIncludeDoc},
		shellScene{name: "feynman_working_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneWorkingComposer},
		shellScene{name: "feynman_files_empty_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", research: sceneFilesEmpty},
	)
	return append(scenes, pass2Scenes()...)
}

func pass2Scenes() []shellScene {
	var scenes []shellScene
	for _, dark := range []bool{false, true} {
		suffix, space := "_light", "Wood"
		if dark {
			suffix, space = "_dark", "MetalWater"
		}
		for _, name := range []string{"unsaved_single", "unsaved_multi", "conflict_decision", "readonly_decision", "rename_prompt", "move_picker", "revert_confirm", "quit_failed"} {
			scenes = append(scenes, shellScene{name: name + suffix, width: 1280, height: 820, mode: "preview", space: space, dark: dark, dialog: name})
		}
		for _, tc := range []struct {
			name string
			act  func(*ui.Tester)
		}{{"context_menu_editor", func(app *ui.Tester) { _ = app.RightClick("Markdown Preview") }}, {"context_menu_sidebar", func(app *ui.Tester) { _ = app.RightClick("Reading List.md") }}, {"text_field_edit_menu", func(app *ui.Tester) {
			openSettingsScene("general")(app)
			app.Frame()
			_ = app.RightClick("Search settings")
		}}} {
			scenes = append(scenes, shellScene{name: tc.name + suffix, width: 1280, height: 820, mode: "preview", space: space, dark: dark, act: tc.act})
		}
		scenes = append(scenes,
			shellScene{name: "link_hover" + suffix, width: 1280, height: 820, mode: "preview", space: space, dark: dark, actSource: sceneLinkHover},
			shellScene{name: "link_edit" + suffix, width: 1280, height: 820, mode: "preview", space: space, dark: dark, actSource: func(app *ui.Tester, source *nativeeditor.Document) {
				sceneLinkHover(app, source)
				_ = app.Click("Edit link")
			}},
			shellScene{name: "preview_organs" + suffix, width: 1440, height: 1800, mode: "preview", space: space, dark: dark, document: previewOrgansDocument},
			shellScene{name: "chat_markdown_organs" + suffix, width: 1440, height: 1600, feynman: true, mode: "preview", space: space, dark: dark, research: sceneChatOrgans},
			shellScene{name: "genui_controls" + suffix, width: 1280, height: 900, mode: "genui", space: space, dark: dark},
			shellScene{name: "diff" + suffix, width: 1280, height: 820, mode: "preview", space: space, dark: dark, act: func(app *ui.Tester) { _ = app.Click("Checkpoints") }},
		)
		for _, page := range []string{"Shortcuts", "Search & Papers", "Packages", "Plugins", "GenUI", "Diagnostics"} {
			name := strings.ToLower(strings.ReplaceAll(page, " & ", "_"))
			name = strings.ReplaceAll(name, " ", "_")
			scenes = append(scenes, shellScene{name: "settings_" + name + suffix, width: 1440, height: 900, feynman: true, mode: "preview", space: space, dark: dark, research: sceneSettingsModal, act: openSettingsScene(page)})
		}
	}
	return scenes
}

const previewOrgansDocument = `# Preview Organs

- [x] Completed task uses accent and muted text
- [ ] Open task has a centered box
- Ordinary bullet shares the marker column

> A three-pixel accent bar and a soft wash hold the quote together.

![Framed image](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGAAAABACAYAAADlNHIOAAAAoklEQVR42u3bsQkAIAwEQCdyIzdySXsLN9AhAhHxitQPf+WTUlvfkRtrhu73/AIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAkACjwbj4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACQAaBAixgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPAfoECLGAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAeD7/AI0y3AxAAZkoAAAAAElFTkSuQmCC)

| Organ | Treatment |
| --- | --- |
| Header | Tinted and semibold |
| Cells | Token padding |

> [!NOTE]
> Note uses the Space accent.

> [!TIP]
> Tip uses the Wu Xing success tone.

> [!IMPORTANT]
> Important has its own mixed tone.

> [!WARNING]
> Warning is amber.

> [!CAUTION]
> Caution is the danger tone.

---

` + "```go\nfunc calm() string {\n\treturn \"shared syntax\"\n}\n```" + `
`

// sceneFolders is the move picker's list: every folder of the recorded
// workspace, including one the sidebar shows collapsed.
func sceneFolders() []workspace.NavEntry {
	root := "/Users/ada/Field Notes"
	folder := func(name, rel string, depth int) workspace.NavEntry {
		return workspace.NavEntry{ID: "folder:" + rel, Kind: "folder", Name: name, Path: root + "/" + rel, Parent: ".", Depth: depth, MarkdownCount: 2, FolderState: workspace.FolderOpen}
	}
	return []workspace.NavEntry{
		folder("Essays", "Essays", 0),
		folder("Plans", "Plans", 0),
		folder("Archive", "Archive", 0),
		folder("Exports", "Archive/Exports", 1),
	}
}

// sceneReview is the recorded review state the proposal, checkpoint and diff
// scenes draw through nativereview.
func sceneReview() nativereview.SceneState {
	root := "/Users/ada/Field Notes"
	path := root + "/Essays/Quiet Software.md"
	hunks := []review.DiffHunk{{
		ID: "h1", Header: "@@ -3,3 +3,4 @@", OldStart: 3, OldLines: 3, NewStart: 3, NewLines: 4,
		Lines: []review.DiffLine{
			{Kind: review.DiffLineContext, Text: "", OldLine: 3, NewLine: 3},
			{Kind: review.DiffLineDelete, Text: "The window stays in front.", OldLine: 4},
			{Kind: review.DiffLineInsert, Text: "The page stays in front.", NewLine: 4},
			{Kind: review.DiffLineInsert, Text: "Everything else waits at the edge.", NewLine: 5},
			{Kind: review.DiffLineContext, Text: "", OldLine: 5, NewLine: 6},
		},
	}}
	proposal := review.ChangeProposal{
		ID: "p1", Revision: 1, WorkspaceRoot: root, DocumentID: "a", Path: path,
		Source: "Feynman Research", Summary: "Tighten the introduction and keep the checklist.",
		Hunks: hunks, CreatedAt: time.Now().UnixMilli(),
	}
	state := nativereview.SceneState{
		Proposals: review.ReviewUpdateEvent{WorkspaceRoot: root, Revision: 1, Proposals: []review.ChangeProposal{proposal, {
			ID: "p2", Revision: 1, WorkspaceRoot: root, Path: root + "/Reading List.md",
			Source: "Feynman Research", Summary: "Add the two books read this week.",
		}}},
		Compared: proposal,
		Hunks:    hunks,
		Checkpoints: []review.Checkpoint{
			{ID: "c1", CreatedAt: time.Date(2026, 1, 14, 9, 30, 0, 0, time.Local).UnixMilli(), Label: "Quiet beginning", Backend: "git", Source: "git:HEAD"},
			{ID: "c2", CreatedAt: time.Date(2026, 1, 14, 10, 15, 0, 0, time.Local).UnixMilli(), Label: "Before review", Backend: "local"},
		},
	}
	return state
}

// sceneStatements is a published view whose statements draw every control the
// reader renders, so the controls scene exercises the production widgets.
const sceneStatements = `{
  "version": 3,
  "language": "openui-lang/1.0",
  "sources": {},
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "view",
        "catalogId": "markgogo.dev:genui/v3",
        "components": [
          "$title = \"A calm reader\"",
          "$request = \"A multiline request with Portrait field chrome.\"",
          "$view = \"\"",
          "$width = 64",
          "$checked = true",
          "$source = \"paper\"",
          "$date = \"2026-01-14\"",
          "$section = \"summary\"",
          "form = Form([Input($title, \"Title\", null, \"Reading view title\"), TextArea($request, \"Request\", \"What should the view contain?\", 3), Select($view, [{value: \"preview\", label: \"Preview\"}, {value: \"markdown\", label: \"Markdown\"}], \"View\", \"Choose…\"), Slider($width, 0, 100, 1, \"Reading width\"), Checkbox($checked, \"Show sources\"), RadioGroup($source, [{value: \"paper\", label: \"Published paper\"}, {value: \"notes\", label: \"Local notes\"}], \"Source\"), DatePicker($date, \"Publication date\"), SegmentedControl($section, [{value: \"summary\", label: \"Summary\"}, {value: \"evidence\", label: \"Evidence\"}]), Row([Button(\"Generate\", null, \"solid\"), Button(\"Delete\", null, \"outline\", \"danger\")])])",
          "root = Col([Title(\"GenUI Reader Controls\", \"lg\"), form])"
        ]
      }
    }
  ]
}`

func sceneChatOrgans(p *panel) {
	p.state.messages = []message{{kind: "assistant", messageID: "organs", text: previewOrgansDocument}}
}

// TestPass2SceneCoverage keeps the Pass 2 acceptance set from shrinking.
func TestPass2SceneCoverage(t *testing.T) {
	scenes := pass2Scenes()
	if len(scenes) < 44 {
		t.Fatalf("Pass 2 has %d scenes, want at least 44", len(scenes))
	}
	dialogs := 0
	for _, sc := range scenes {
		if sc.dialog != "" {
			dialogs++
		}
	}
	if dialogs != 16 {
		t.Fatalf("dialog scenes = %d, want the eight dialogs in both appearances", dialogs)
	}
}

func sceneHistory(p *panel) {
	p.state.sessionsOpen = true
	p.state.sessionRows = []feynman.ResearchConversation{{ID: "current", Title: "Quiet software", Current: true, Preview: "Current research"}, {ID: "older", Title: "Reading notes", Preview: "An earlier conversation"}}
}
func sceneActionCommands(p *panel) {
	p.state.commands = []feynman.ResearchCommand{{Name: "summarize", Description: "Summarize the active document", Source: "workflow"}, {Name: "review", Description: "Review the document for clarity", Source: "skill"}}
}
func sceneQuestion(p *panel) {
	p.state.permission = &feynman.ResearchUpdate{Kind: "select", RequestID: "scene", Text: "Choose a source for the next pass", Options: []feynman.ResearchOption{{ID: "notes", Name: "Local notes"}, {ID: "paper", Name: "Published paper"}}}
}
func sceneFiles(p *panel) {
	p.state.selectedTab = "files"
	p.state.artifacts = []review.ResearchArtifact{{Path: "research/summary.md", Target: "Summary.md", Proposal: true}, {Path: "research/notes.md", Target: "Notes.md"}}
}
func sceneFilesEmpty(p *panel) {
	p.state.selectedTab = "files"
	p.state.artifacts = nil
}
func sceneNoticeAndIncludeDoc(p *panel) {
	p.state.commands = []feynman.ResearchCommand{{Name: "summarize", Description: "Summarize the active document", Source: "workflow"}}
	p.state.includeDoc = true
	p.state.messages = []message{
		{kind: "notice", messageID: "n1", text: "The previous Feynman session is no longer available. Starting a new conversation."},
	}
}
func sceneWorkingComposer(p *panel) {
	p.state.commands = []feynman.ResearchCommand{{Name: "summarize", Description: "Summarize the active document", Source: "workflow"}}
	p.state.status = statusWorking
	p.state.includeDoc = true
	p.state.draft = "Compare the two ceramic styles in section 5."
}
func sceneAlpha(p *panel) {
	p.state.selectedTab = "alpha"
	p.alphaQuery = "quiet software"
	p.state.searchResult = &feynman.CLIAlphaSearchResult{Parsed: true, Mode: "semantic", Papers: []feynman.CLIAlphaPaper{{ID: "2401.00001", Title: "Calm Interfaces for Focused Work", Authors: "Ada Lovelace", Summary: "A study of readable, receding tool surfaces."}}}
}
func sceneLinkHover(app *ui.Tester, source *nativeeditor.Document) {
	position := strings.Index(sceneDocument, "Project") + 1
	if bounds, ok := source.RectUTF16(position); ok {
		if editor, found := app.AccessByRole(ui.RoleTextField, "Markdown Preview"); found {
			bounds.X += editor.Bounds.X
			bounds.Y += editor.Bounds.Y
		}
		app.Move(bounds.X+0.1, bounds.Y+bounds.H/2)
		app.Advance(time.Second)
		app.Frame()
	}
}

func sceneSettingsModal(p *panel) {
	sceneSettings(p)
	p.state.selectedTab = "chat"
}

func openModelPicker(app *ui.Tester) {
	_ = app.Click("Model")
	app.Frame()
}

func sceneModels(p *panel) {
	s := p.state
	s.models = []feynman.ResearchModel{
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "anthropic"},
		{ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Provider: "anthropic"},
		{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: "anthropic"},
		{ID: "gpt-5.4", Name: "GPT-5.4", Provider: "openai"},
		{ID: "gpt-5.4-mini", Name: "GPT-5.4 mini", Provider: "openai"},
		{ID: "o5", Name: "o5", Provider: "openai"},
		{ID: "gemini-3-pro", Name: "Gemini 3 Pro", Provider: "google"},
		{ID: "gemini-3-flash", Name: "Gemini 3 Flash", Provider: "google"},
		{ID: "grok-5", Name: "Grok 5", Provider: "xai"},
		{ID: "deepseek-v4", Name: "DeepSeek V4", Provider: "openrouter"},
		{ID: "qwen-4-max", Name: "Qwen 4 Max", Provider: "openrouter"},
		{ID: "llama-5", Name: "Llama 5 70B", Provider: "ollama"},
	}
	p.toggleModelPinned("anthropic/claude-opus-4-1")
	p.rememberModel("openai/gpt-5.4")
	p.rememberModel("anthropic/claude-sonnet-4-5")
}

func sceneSettings(p *panel) {
	sceneModels(p)
	s := p.state
	s.selectedTab = "chat"
	s.cliModels = &feynman.CLIModelListResult{Parsed: true, Tier: "priority", Models: []feynman.CLIModelEntry{
		{ID: "claude-sonnet-4-5", Provider: "anthropic", Name: "Claude Sonnet 4.5", Current: true},
		{ID: "claude-opus-4-1", Provider: "anthropic", Name: "Claude Opus 4.1"},
		{ID: "gpt-5.4", Provider: "openai", Name: "GPT-5.4"},
		{ID: "gpt-5.4-mini", Provider: "openai", Name: "GPT-5.4 mini"},
	}}
	s.cliStatus = &feynman.CLIStatusResult{Parsed: true, ServiceTier: "priority"}
	p.setModelShown("openai/gpt-5.4-mini", false)
}

func sceneSpaces() []string {
	if only := os.Getenv("MGG_SCENES_SPACE"); only != "" {
		return []string{only}
	}
	return []string{"FiveElements", "MetalWater", "WoodFire", "Earth", "Metal", "Wood", "Water", "Fire"}
}
