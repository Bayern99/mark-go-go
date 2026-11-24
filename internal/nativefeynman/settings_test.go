package nativefeynman

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/egoist/mygo/ui"
)

func fixtureSettingsPanel() *panel {
	p := fixtureReadyPanel()
	s := p.state
	s.selectedTab = "chat"
	s.cliModels = &feynman.CLIModelListResult{Parsed: true, Tier: "priority", Models: []feynman.CLIModelEntry{
		{ID: "claude-sonnet-4-5", Provider: "anthropic", Name: "Claude Sonnet 4.5", Current: true},
		{ID: "claude-opus-4-1", Provider: "anthropic", Name: "Claude Opus 4.1"},
		{ID: "gpt-5.4", Provider: "openai", Name: "GPT-5.4"},
	}}
	s.cliStatus = &feynman.CLIStatusResult{Parsed: true, ServiceTier: "priority", Sections: []feynman.CLISection{{Title: "Runtime", Entries: []feynman.CLIKeyValue{{Key: "Pi runtime", Value: "/opt/pi"}}}}}
	return p
}

func TestSettingsGroupModelsByProviderWithOneSignOut(t *testing.T) {
	p := fixtureSettingsPanel()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.modelSettings(c) })
	}, 1280, 1600)
	app.Frame()
	for _, group := range []string{"Anthropic models", "OpenAI models", "Local models"} {
		if _, ok := app.AccessByRole(ui.RoleGroup, group); !ok {
			t.Errorf("missing provider group %q", group)
		}
	}
	logouts := 0
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleButton && strings.HasPrefix(info.Name, "Log out of ") {
			logouts++
		}
	}
	if logouts != 2 {
		t.Errorf("%d Log out buttons, want one per signed-in provider (2)", logouts)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Sign in to Local in Terminal"); !ok {
		t.Error("a provider without sign-in offers no Sign in in Terminal")
	}
	if !app.HasText("Default") {
		t.Error("the default model is not labelled Default")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Set Claude Opus 4.1 as the default model"); !ok {
		t.Error("other models offer no Set default")
	}
	for _, name := range []string{"Pin Claude Opus 4.1", "Show GPT-5.4 in Picker"} {
		if _, ok := app.Access(name); !ok {
			t.Errorf("missing %q", name)
		}
	}
	for _, text := range app.Texts() {
		for _, noise := range []string{"◇", "◉", "$"} {
			if strings.Contains(text, noise) {
				t.Errorf("settings show %q in %q", noise, text)
			}
		}
	}
	if err := app.Click("Pin Claude Opus 4.1"); err != nil {
		t.Fatal(err)
	}
	if !p.modelPinned("anthropic/claude-opus-4-1") {
		t.Error("the star did not pin the model")
	}
}

func TestSettingsPrefillServiceTierAndCollapseStatusSections(t *testing.T) {
	p := fixtureSettingsPanel()
	models := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.modelSettings(c) })
	}, 1280, 1600)
	models.Frame()
	if tier, ok := models.AccessByRole(ui.RoleTextField, "Service tier"); !ok || tier.Value != "priority" {
		t.Fatalf("service tier field = %+v, want the CLI's priority", tier)
	}
	p = fixtureSettingsPanel()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.statusSettings(c) })
	}, 1280, 1600)
	app.Frame()
	if app.HasText("Pi runtime") {
		t.Fatal("status section is expanded by default")
	}
	if !app.HasText("Run Doctor") {
		t.Fatal("diagnostics actions are missing")
	}
	if err := app.Click("Runtime"); err != nil {
		t.Fatal(err)
	}
	app.Advance(1e9)
	app.Frame()
	if !app.HasText("Pi runtime") {
		t.Fatal("opening Runtime did not reveal its status entries")
	}
}
