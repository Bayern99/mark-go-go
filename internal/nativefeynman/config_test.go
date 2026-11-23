package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestResearchConfigPickersUseAgentFallbackAndKeepExtraOrder(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.models = nil
	p.state.thinking = nil
	p.state.session.Model = nil
	p.state.thinkingLevel = ""
	p.state.session.ThinkingLevel = ""
	p.state.configs = []feynman.ResearchConfig{
		{ID: "steering_mode", Name: "Steering", CurrentValue: "queue", Options: []feynman.ResearchConfigValue{{Value: "queue", Name: "Queue"}, {Value: "interrupt", Name: "Interrupt"}}},
		{ID: "model", Category: "model", CurrentValue: "local/terra", Options: []feynman.ResearchConfigValue{{Value: "local/terra", Name: "Terra", Description: "Local model"}}},
		{ID: "thinking_level", Category: "thinking", CurrentValue: "high", Options: []feynman.ResearchConfigValue{{Value: "high", Name: "High"}}},
		{ID: "follow_up_mode", Name: "Follow up", CurrentValue: "queue", Options: []feynman.ResearchConfigValue{{Value: "queue", Name: "Queue"}}},
	}

	pickers := p.researchConfigPickers()
	if len(pickers) != 4 {
		t.Fatalf("picker count = %d, want model, thinking and two agent settings", len(pickers))
	}
	if pickers[0].key != "model" || pickers[0].icon != portrait.IconCube || currentResearchPickerLabel(pickers[0]) != "Terra" {
		t.Errorf("model picker = %+v, current label %q", pickers[0], currentResearchPickerLabel(pickers[0]))
	}
	if pickers[1].key != "thinking" || pickers[1].icon != portrait.IconSlidersHorizontal || currentResearchPickerLabel(pickers[1]) != "High" {
		t.Errorf("thinking picker = %+v, current label %q", pickers[1], currentResearchPickerLabel(pickers[1]))
	}
	if pickers[2].key != "config-steering_mode" || pickers[3].key != "config-follow_up_mode" {
		t.Errorf("extra agent settings changed order: %q, %q", pickers[2].key, pickers[3].key)
	}
}

func fixtureResearchPicker(selected *string) researchPicker {
	return researchPicker{
		key: "model", label: "Model", current: "anthropic/claude", icon: portrait.IconCube,
		items: []researchPickerItem{
			{value: "anthropic/claude", label: "Claude Sonnet", detail: "anthropic", description: "Use anthropic model claude", current: true},
			{value: "openai/gpt", label: "GPT Terra", detail: "openai", description: "Use openai model gpt"},
			{value: "local/hold", label: "Slow Model", detail: "local", description: "Use local model hold"},
		},
		onSelect: func(value string) { *selected = value },
	}
}

func newResearchPickerTester(p *panel, picker researchPicker) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Justify(ui.End).Padding(20).Children(func() {
			p.configPicker(c, picker)
		})
	}, 420, 500)
}

func TestResearchPickerOpensAboveAndFiltersThenSelects(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.Frame()
	trigger, ok := app.AccessByRole(ui.RoleButton, "Model")
	if !ok {
		t.Fatal("missing compact Model trigger")
	}
	if trigger.Value != "Claude Sonnet" {
		t.Fatalf("Model trigger value = %q, want the current model label", trigger.Value)
	}
	if err := app.Click("Model"); err != nil {
		t.Fatalf("open Model picker: %v", err)
	}
	app.Frame()
	if !app.Focused("Find Model") {
		t.Fatal("opening the picker did not focus its filter")
	}
	popup, ok := app.AccessByRole(ui.RolePopup, "Model options")
	if !ok {
		t.Fatal("Model picker has no named popup")
	}
	trigger, _ = app.AccessByRole(ui.RoleButton, "Model")
	if popup.Bounds.Y+popup.Bounds.H > trigger.Bounds.Y {
		t.Fatalf("popup bounds %+v are not above trigger %+v", popup.Bounds, trigger.Bounds)
	}
	if popup.Bounds.X != trigger.Bounds.X {
		t.Errorf("popup starts at x=%v, trigger starts at x=%v; want start alignment", popup.Bounds.X, trigger.Bounds.X)
	}

	app.Type("gpt")
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Use openai model gpt"); !ok {
		t.Fatal("filter did not retain the matching GPT model")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Use anthropic model claude"); ok {
		t.Fatal("filter left a non-matching Claude option")
	}
	app.Key(0, ui.KeyDown)
	app.Frame()
	if !app.Focused("Use openai model gpt") {
		t.Fatal("ArrowDown did not move focus from the filter to the matching option")
	}
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if selected != "openai/gpt" {
		t.Fatalf("selected model = %q, want openai/gpt", selected)
	}
	if _, ok := app.AccessByRole(ui.RolePopup, "Model options"); ok {
		t.Fatal("selecting a model did not close the popup")
	}
	if !app.Focused("Model") {
		t.Fatal("selecting a model did not restore focus to its trigger")
	}
}

func TestResearchPickerArrowKeysWrapThroughOptions(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.Frame()
	if err := app.Click("Model"); err != nil {
		t.Fatalf("open Model picker: %v", err)
	}
	app.Frame()
	for _, step := range []struct {
		key  ui.Key
		want string
	}{
		{ui.KeyDown, "Use openai model gpt"},
		{ui.KeyDown, "Use local model hold"},
		{ui.KeyDown, "Use anthropic model claude"},
		{ui.KeyUp, "Use local model hold"},
		{ui.KeyUp, "Use openai model gpt"},
	} {
		app.Key(0, step.key)
		app.Frame()
		if !app.Focused(step.want) {
			t.Fatalf("after %v, focused option is not %q", step.key, step.want)
		}
	}
}

func TestResearchPickerEscapeRestoresTriggerAndResetsFilter(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.Frame()
	if err := app.Click("Model"); err != nil {
		t.Fatalf("open Model picker: %v", err)
	}
	app.Frame()
	app.Type("gpt")
	app.Frame()
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Model options"); ok {
		t.Fatal("Escape did not close the picker")
	}
	if !app.Focused("Model") {
		t.Fatal("Escape did not restore focus to the Model trigger")
	}
	if selected != "" {
		t.Fatalf("Escape unexpectedly selected %q", selected)
	}
	if err := app.Click("Model"); err != nil {
		t.Fatalf("reopen Model picker: %v", err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Use anthropic model claude"); !ok {
		t.Fatal("reopening the picker retained the previous filter")
	}
	if filter, ok := app.AccessByRole(ui.RoleTextField, "Find Model"); !ok || filter.Value != "" {
		t.Fatalf("reopened filter = %+v, present=%t; want an empty query", filter, ok)
	}
}
