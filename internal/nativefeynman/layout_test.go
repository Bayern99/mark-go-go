package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
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
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Research model and thinking settings"); !ok {
		t.Fatal("compact model and thinking controls are unavailable")
	}
	if bounds, ok := app.Find("Feynman conversation"); !ok || bounds.H <= 0 {
		t.Fatalf("conversation scroll bounds = %+v, present=%t; want positive height", bounds, ok)
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
	if err := app.Click("Research model and thinking settings"); err != nil {
		t.Fatalf("open compact model settings: %v", err)
	}
	app.Frame()
	for _, name := range []string{"Use local model hold", "Set thinking level to medium"} {
		info, ok := app.AccessByRole(ui.RoleButton, name)
		if !ok {
			t.Errorf("compact model popover has no accessible %q control", name)
		} else if info.Offscreen || info.Bounds.X < 0 || info.Bounds.Y < 0 || info.Bounds.X+info.Bounds.W > 480 || info.Bounds.Y+info.Bounds.H > 360 {
			t.Errorf("compact model control %q is outside the window: %+v offscreen=%t", name, info.Bounds, info.Offscreen)
		}
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Research model and thinking settings"); !ok {
		t.Fatal("closing compact model settings removed its trigger")
	}
	if !app.Focused("Research model and thinking settings") {
		t.Fatal("Escape did not return focus to the compact model settings trigger")
	}
}

func TestResearchHistoryPopoverFitsNarrowWindowWithoutDisplacingComposer(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.sessionsOpen = true
	p.state.sessionRows = []feynman.ResearchConversation{{ID: "fixture", Title: "Fixture session", Current: true}}
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 376, 272) }, 480, 360)
	app.Frame()

	close, ok := app.AccessByRole(ui.RoleButton, "Close conversation history")
	if !ok {
		t.Fatal("history popover has no reachable Close action")
	}
	if close.Offscreen || close.Bounds.X < 0 || close.Bounds.Y < 0 || close.Bounds.X+close.Bounds.W > 480 || close.Bounds.Y+close.Bounds.H > 360 {
		t.Fatalf("history Close bounds %+v offscreen=%t, want inside the window", close.Bounds, close.Offscreen)
	}
	if bounds, ok := app.Find("Feynman conversation"); !ok || bounds.H <= 0 {
		t.Fatalf("history removed the conversation scroll: bounds=%+v, present=%t", bounds, ok)
	}
	if info, ok := app.AccessByRole(ui.RoleButton, "Send research prompt"); !ok || info.Offscreen || info.Bounds.Y+info.Bounds.H > 320 {
		t.Fatalf("history obscured the composer Send action: info=%+v, present=%t", info, ok)
	}
	if err := app.Click("Close conversation history"); err != nil {
		t.Fatalf("close History: %v", err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Close conversation history"); ok {
		t.Fatal("History popover stayed open after Close")
	}
}

func TestResearchLayoutKeepsExpandedModelControlsAtStandardWindow(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 320, 820) }, 1280, 820)
	app.Frame()
	for _, name := range []string{"Use anthropic model claude-sonnet-4-5", "Set thinking level to medium"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("standard layout lost expanded control %q", name)
		}
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Research model and thinking settings"); ok {
		t.Fatal("standard layout unexpectedly switched to compact controls")
	}
}
