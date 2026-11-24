package nativefeynman

import (
	"strings"
	"testing"
	"time"

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

func TestResearchPickerPaintsProviderRows(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.SetScale(2)
	app.Frame()
	if err := app.Click("Model"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	app.Frame()
	row, ok := app.AccessByRole(ui.RoleButton, "Show Anthropic models")
	if !ok {
		t.Fatal("provider row is missing")
	}
	ink := 0
	img := app.Image()
	for y := int(row.Bounds.Y * 2); y < int((row.Bounds.Y+row.Bounds.H)*2); y++ {
		for x := int(row.Bounds.X * 2); x < int((row.Bounds.X+row.Bounds.W)*2); x++ {
			px := img.RGBAAt(x, y)
			if int(px.R)+int(px.G)+int(px.B) < 450 {
				ink++
			}
		}
	}
	if ink < 10 {
		t.Fatalf("provider row has only %d dark pixels; bounds=%+v", ink, row.Bounds)
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
		{ui.KeyDown, "Show Local models"},
		{ui.KeyDown, "Show OpenAI models"},
		{ui.KeyDown, "Show Anthropic models"},
		{ui.KeyUp, "Show OpenAI models"},
		{ui.KeyUp, "Show Local models"},
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
	if _, ok := app.AccessByRole(ui.RoleButton, "Show OpenAI models"); !ok {
		t.Fatal("reopening the picker retained the previous filter")
	}
	if filter, ok := app.AccessByRole(ui.RoleTextField, "Find Model"); !ok || filter.Value != "" {
		t.Fatalf("reopened filter = %+v, present=%t; want an empty query", filter, ok)
	}
}

func TestModelPickerGroupsByProviderWithPinnedAndRecent(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	p.toggleModelPinned("local/hold")
	p.rememberModel("openai/gpt")
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.Frame()
	if err := app.Click("Model"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	for _, want := range []string{"Pinned", "Recent", "Providers"} {
		if _, ok := app.AccessByRole(ui.RoleHeading, want); !ok {
			t.Errorf("empty query has no %q section", want)
		}
	}
	for _, name := range []string{"Use local model hold", "Use openai model gpt", "Show Anthropic models", "Show OpenAI models", "Show Local models"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("missing %q", name)
		}
	}
	for _, noise := range []string{"◇", "◉", "$", "200000", "tokens"} {
		for _, text := range app.Texts() {
			if strings.Contains(text, noise) {
				t.Errorf("picker shows %q in %q", noise, text)
			}
		}
	}
	if err := app.Click("Show Anthropic models"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Back to providers"); !ok || !app.HasText("Back") {
		t.Fatal("drilling into a provider has no Back row")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Use openai model gpt"); ok {
		t.Fatal("drilling into Anthropic still lists OpenAI models")
	}
	if claude, ok := app.AccessByRole(ui.RoleButton, "Use anthropic model claude"); !ok || !claude.Checked {
		t.Fatalf("drilled list lost the current model or its check: %+v", claude)
	}
	if err := app.Click("Back to providers"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("o")
	app.Frame()
	if !app.HasText("OpenAI") || !app.HasText("Local") {
		t.Errorf("cross-provider search does not name each match's provider: %v", app.Texts())
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Show OpenAI models"); ok {
		t.Error("search still shows provider rows instead of flat matches")
	}
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if selected == "" || p.modelList(recentModelsPreference)[0] != selected {
		t.Fatalf("selected %q, recent %v; want the choice remembered first", selected, p.modelList(recentModelsPreference))
	}
}

func TestHiddenModelsLeaveThePickerButNotTheCurrentOne(t *testing.T) {
	selected := ""
	p := &panel{collapsed: make(map[string]*bool)}
	p.setModelShown("openai/gpt", false)
	p.setModelShown("anthropic/claude", false)
	app := newResearchPickerTester(p, fixtureResearchPicker(&selected))
	app.Frame()
	_ = app.Click("Model")
	app.Frame()
	app.Type("a")
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Use local model hold"); !ok {
		t.Error("search lost a shown model")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Use openai model gpt"); ok {
		t.Error("a model hidden from the picker is still listed")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Use anthropic model claude"); !ok {
		t.Error("the current model disappeared because it was hidden")
	}
}
