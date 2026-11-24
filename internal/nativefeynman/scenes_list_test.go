package nativefeynman

import (
	"os"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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
	act           func(*ui.Tester)
	research      func(*panel)
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
	return append(scenes,
		shellScene{name: "command_bar_dark", width: 1280, height: 820, mode: "preview", space: "WoodFire", dark: true, act: func(app *ui.Tester) {
			app.Key(ui.Cmd, ui.KeyK)
			app.Type("space")
		}},
		shellScene{name: "markdown_dark", width: 1280, height: 820, mode: "markdown", space: "FiveElements", dark: true},
		shellScene{name: "feynman_picker_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", research: sceneModels, act: openModelPicker},
		shellScene{name: "feynman_picker_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "Wood", dark: true, research: sceneModels, act: openModelPicker},
		shellScene{name: "settings_appearance_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", research: sceneSettingsModal, act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.KeyComma) }},
		shellScene{name: "settings_appearance_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "WoodFire", dark: true, research: sceneSettingsModal, act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.KeyComma) }},
		shellScene{name: "settings_models_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "MetalWater", research: sceneSettingsModal, act: func(app *ui.Tester) {
			app.Key(ui.Cmd, ui.KeyComma)
			app.Frame()
			_ = app.Click("Models & Auth")
		}},
		shellScene{name: "settings_models_dark", width: 1440, height: 900, feynman: true, mode: "preview", space: "FiveElements", dark: true, research: sceneSettingsModal, act: func(app *ui.Tester) {
			app.Key(ui.Cmd, ui.KeyComma)
			app.Frame()
			_ = app.Click("Models & Auth")
		}},
		shellScene{name: "sidebar_drilled_light", width: 1440, height: 900, feynman: true, mode: "preview", space: "FiveElements", act: func(app *ui.Tester) {
			_ = app.Click("Essays")
		}},
	)
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
	s.selectedTab = "settings"
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
