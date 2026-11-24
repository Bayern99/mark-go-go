package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func fixtureReadyPanel() *panel {
	state := newPaneState("/workspace/notes")
	state.status = statusReady
	state.title = "Acoustic Study"
	state.draft = "Summarize this workspace"
	state.session.Model = &feynman.ResearchModel{
		ID:       "claude-sonnet-4-5",
		Name:     "Claude Sonnet 4.5",
		Provider: "anthropic",
	}
	state.models = []feynman.ResearchModel{
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "anthropic"},
		{ID: "gpt-5.4", Name: "GPT-5.4", Provider: "openai"},
		{ID: "hold", Name: "Slow Model", Provider: "local"},
	}
	state.thinking = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}
	state.thinkingLevel = "medium"
	state.hasStats = true
	state.stats.Tokens.Total = 1500
	state.stats.Cost = 0.0125
	state.stats.ContextUsage = &feynman.ResearchContextUsage{Tokens: 1500, ContextWindow: 200000, Percent: 0.0075}
	state.messages = []message{{kind: "assistant", messageID: "fixture-reply", text: "Created first"}}
	return &panel{window: &nativeui.Window{}, state: state, collapsed: make(map[string]*bool)}
}

func drawFixturePane(c *ui.Context, p *panel, width, height float32) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c.Key("fixture-research-drawer")).Absolute().Left(96).Top(48).Width(width).Height(height).Children(func() {
			p.header(c)
			p.tabs(c)
			p.chat(c)
		})
	})
}

func TestResearchDrawerAt480By360KeepsComposerVisibleAndFocusable(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 376, 272) }, 480, 360)
	app.Frame()

	for _, target := range []struct {
		role ui.Role
		name string
	}{
		{ui.RoleTextField, "Research prompt"},
		{ui.RoleButton, "Attach source files"},
		{ui.RoleButton, "Send research prompt"},
	} {
		info, ok := app.AccessByRole(target.role, target.name)
		if !ok {
			t.Fatalf("missing accessible %v %q", target.role, target.name)
		}
		if info.Offscreen || info.Bounds.X < 96 || info.Bounds.Y < 48 || info.Bounds.X+info.Bounds.W > 472 || info.Bounds.Y+info.Bounds.H > 320 {
			t.Errorf("%s bounds %+v offscreen=%t, want inside the 376×272 research drawer", target.name, info.Bounds, info.Offscreen)
		}
		if target.role == ui.RoleButton && info.Disabled {
			t.Errorf("%s is disabled with a ready fixture pane", target.name)
		}
		if target.role == ui.RoleTextField && info.Bounds.H > 50 {
			t.Errorf("research prompt takes %v DIP, leaving too little room for the conversation", info.Bounds.H)
		}
	}
	for _, name := range []string{"Model", "Thinking level"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Fatalf("compact composer has no %s picker", name)
		}
	}
	if bounds, ok := app.Find("Feynman conversation"); !ok || bounds.H < 40 {
		t.Fatalf("conversation scroll bounds = %+v, present=%t; want at least 40 DIP", bounds, ok)
	}
	transcript, ok := app.Access("Created first")
	if !ok || transcript.Offscreen {
		t.Fatalf("the ready transcript is not visible: %+v, present=%t", transcript, ok)
	}
	if err := app.Click("Research prompt"); err != nil {
		t.Fatalf("focus research prompt: %v", err)
	}
	app.Frame()
	if !app.Focused("Research prompt") {
		t.Fatal("research prompt did not take keyboard focus")
	}
	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Attach source files") {
		t.Fatal("Tab from the research prompt did not reach Attach source files")
	}
	if err := app.Click("Model"); err != nil {
		t.Fatalf("open compact model picker: %v", err)
	}
	app.Frame()
	for _, name := range []string{"Show Local models", "Show OpenAI models"} {
		info, ok := app.AccessByRole(ui.RoleButton, name)
		if !ok {
			t.Errorf("compact model popover has no accessible %q control", name)
		} else if info.Offscreen || info.Bounds.X < 0 || info.Bounds.Y < 0 || info.Bounds.X+info.Bounds.W > 480 || info.Bounds.Y+info.Bounds.H > 360 {
			t.Errorf("compact model control %q is outside the window: %+v offscreen=%t", name, info.Bounds, info.Offscreen)
		}
	}
	if app.HasText("1500 / 200000 tokens") || app.HasText("$0.0125") {
		t.Error("composer still prints token and cost statistics")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Model"); !ok {
		t.Fatal("closing the compact model picker removed its trigger")
	}
	if !app.Focused("Model") {
		t.Fatal("Escape did not return focus to the Model trigger")
	}
}

func TestResearchComposerWithAttachedSourceFitsNarrowDrawer(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.sources = []string{"/workspace/Attachment.md"}
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 376, 272) }, 480, 360)
	app.Frame()

	for _, target := range []struct {
		role ui.Role
		name string
	}{
		{ui.RoleTextField, "Research prompt"},
		{ui.RoleButton, "Remove attached source /workspace/Attachment.md"},
		{ui.RoleButton, "Attach source files"},
		{ui.RoleButton, "Send research prompt"},
	} {
		info, ok := app.AccessByRole(target.role, target.name)
		if !ok {
			t.Fatalf("attached-source composer has no accessible %q", target.name)
		}
		if info.Offscreen || info.Bounds.X < 96 || info.Bounds.Y < 48 || info.Bounds.X+info.Bounds.W > 472 || info.Bounds.Y+info.Bounds.H > 320 {
			t.Errorf("attached-source composer control %q is outside the drawer: %+v", target.name, info.Bounds)
		}
	}
	bounds, present := app.Find("Feynman conversation")
	if !present || bounds.H <= 0 {
		t.Fatalf("attached source left no conversation scroll: bounds=%+v, present=%t", bounds, present)
	}
	if transcript, ok := app.Access("Created first"); !ok || transcript.Offscreen {
		t.Fatalf("the attached-source transcript is not visible: %+v, present=%t", transcript, ok)
	}
}

func TestResearchHistoryPopoverFitsNarrowWindowWithoutDisplacingComposer(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.sessionsOpen = true
	p.state.sessionRows = []feynman.ResearchConversation{{ID: "fixture", Title: "Fixture session", Current: true}}
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 376, 272) }, 480, 360)
	app.Frame()

	actions, ok := app.AccessByRole(ui.RoleMenuButton, "Conversation actions")
	if !ok {
		t.Fatal("history popover has no reachable Conversation actions trigger")
	}
	if actions.Offscreen || actions.Bounds.X < 0 || actions.Bounds.Y < 0 || actions.Bounds.X+actions.Bounds.W > 480 || actions.Bounds.Y+actions.Bounds.H > 360 {
		t.Fatalf("history actions bounds %+v offscreen=%t, want inside the window", actions.Bounds, actions.Offscreen)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Close conversation history"); ok {
		t.Fatal("history popover should not render a redundant Close button")
	}
	if bounds, ok := app.Find("Feynman conversation"); !ok || bounds.H < 40 {
		t.Fatalf("history removed the conversation scroll: bounds=%+v, present=%t", bounds, ok)
	}
	if info, ok := app.AccessByRole(ui.RoleButton, "Send research prompt"); !ok || info.Offscreen || info.Bounds.Y+info.Bounds.H > 320 {
		t.Fatalf("history obscured the composer Send action: info=%+v, present=%t", info, ok)
	}
	if err := app.Click("Workspace conversation history"); err != nil {
		t.Fatalf("toggle History closed: %v", err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleMenuButton, "Conversation actions"); ok {
		t.Fatal("History popover stayed open after toggling history")
	}
}

func TestResearchComposerConfigRowWrapsWithoutShrinkingPrompt(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.configs = []feynman.ResearchConfig{{
		ID: "steering_mode", Name: "Steering mode", CurrentValue: "queue",
		Options: []feynman.ResearchConfigValue{{Value: "queue", Name: "Queue for later"}, {Value: "interrupt", Name: "Interrupt"}},
	}}
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 220, 500) }, 480, 640)
	app.Frame()

	model, modelOK := app.AccessByRole(ui.RoleButton, "Model")
	thinking, thinkingOK := app.AccessByRole(ui.RoleButton, "Thinking level")
	extra, extraOK := app.AccessByRole(ui.RoleButton, "Steering mode")
	prompt, promptOK := app.AccessByRole(ui.RoleTextField, "Research prompt")
	if !modelOK || !thinkingOK || !extraOK || !promptOK {
		t.Fatalf("narrow composer lost a control: model=%t thinking=%t extra=%t prompt=%t", modelOK, thinkingOK, extraOK, promptOK)
	}
	if thinking.Bounds.Y <= model.Bounds.Y && extra.Bounds.Y <= model.Bounds.Y {
		t.Fatalf("configuration controls did not wrap at 220 DIP: model=%+v thinking=%+v extra=%+v", model.Bounds, thinking.Bounds, extra.Bounds)
	}
	if prompt.Bounds.H != portrait.Form.AreaMin {
		t.Errorf("wrapped configuration row changed the prompt to %v DIP, want %v", prompt.Bounds.H, portrait.Form.AreaMin)
	}
	for name, info := range map[string]ui.AccessInfo{"Model": model, "Thinking level": thinking, "Steering mode": extra, "Research prompt": prompt} {
		if info.Offscreen || info.Bounds.X < 96 || info.Bounds.X+info.Bounds.W > 316 || info.Bounds.Y+info.Bounds.H > 548 {
			t.Errorf("%s is outside the narrow drawer: %+v offscreen=%t", name, info.Bounds, info.Offscreen)
		}
	}
	if bounds, ok := app.Find("Feynman conversation"); !ok || bounds.H < 40 {
		t.Fatalf("wrapped configuration row removed the conversation: bounds=%+v present=%t", bounds, ok)
	}
}

func TestResearchLayoutUsesComposerPickersAtStandardWindow(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 320, 772) }, 1280, 820)
	app.Frame()
	for _, name := range []string{"Model", "Thinking level"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("standard composer lost compact picker %q", name)
		}
	}
	for _, name := range []string{"Use anthropic model claude-sonnet-4-5", "Set thinking level to medium"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); ok {
			t.Errorf("Chat top still exposes expanded control %q", name)
		}
	}
	prompt, _ := app.AccessByRole(ui.RoleTextField, "Research prompt")
	model, _ := app.AccessByRole(ui.RoleButton, "Model")
	if model.Bounds.Y < prompt.Bounds.Y+prompt.Bounds.H {
		t.Errorf("configuration controls are not below the prompt: prompt=%+v model=%+v", prompt.Bounds, model.Bounds)
	}
	if err := app.Click("Thinking level"); err != nil {
		t.Fatalf("open Thinking level picker: %v", err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Set thinking level to medium"); !ok {
		t.Fatal("Thinking picker did not expose the current thinking level")
	}
}
