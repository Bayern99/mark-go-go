package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestFilesPanelWithoutArtifactsDrawsNoEmptyCard(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.artifacts = nil
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.files(c) })
	}, 640, 480)
	app.Frame()
	if !app.HasText("No research files yet") {
		t.Fatal("an empty Files panel does not explain itself")
	}
	description, ok := app.Find("Completed research files appear here.")
	if !ok {
		t.Fatal("the empty Files panel shows no description")
	}
	blank := ui.NewTester(func(c *ui.Context) {}, 640, 480)
	blank.Frame()
	top := int(description.Y + description.H)
	background := blank.Image().RGBAAt(int(description.X+description.W/2), top+10)
	img := app.Image()
	for y := top; y < top+24; y++ {
		for x := int(description.X); x < int(description.X+description.W); x++ {
			if got := img.RGBAAt(x, y); got != background {
				t.Fatalf("empty Files panel paints a card surface at (%d, %d): %v, want the plain background %v", x, y, got, background)
			}
		}
	}
}

func TestFilesErrorReplacesEmptyState(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.artifacts = nil
	p.state.filesError = "Could not load research files"
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.files(c) })
	}, 640, 480)
	app.Frame()
	if !app.HasText("Could not load research files") {
		t.Fatal("Files error is not visible")
	}
	if app.HasText("No research files yet") {
		t.Fatal("Files error is accompanied by a contradictory empty state")
	}
}

func TestAlphaXivFullTextCheckboxKeepsCompactHeight(t *testing.T) {
	p := fixtureAlphaPanel()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { p.alphaXiv(c) })
	}, 720, 820)
	app.Frame()
	info, ok := app.AccessByRole(ui.RoleCheckBox, "Full text")
	if !ok {
		t.Fatal("the paper tools show no Full text check box")
	}
	if info.Bounds.H > portrait.Type.UI.Leading {
		t.Errorf("Full text check box height = %v, want the compact %v UI label height beside its button", info.Bounds.H, portrait.Type.UI.Leading)
	}
	inspect, ok := app.AccessByRole(ui.RoleButton, "Get alphaXiv paper details")
	if !ok {
		t.Fatal("the paper tools show no Inspect button")
	}
	if inspect.Bounds.H != portrait.Sizes.Regular {
		t.Errorf("Inspect button height = %v, want Regular %v", inspect.Bounds.H, portrait.Sizes.Regular)
	}
}

func TestMCPSettingsHidesAddTriggerWhileItsFormIsOpen(t *testing.T) {
	p := fixtureSettingsPanel()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.mcpSettings(c) })
	}, 1000, 1400)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Open the add MCP server form"); !ok {
		t.Fatal("the closed MCP form offers no Add server button")
	}
	if _, ok := app.AccessByRole(ui.RoleTextField, "Server name"); ok {
		t.Fatal("the add server form is open before its button is pressed")
	}
	if err := app.Click("Open the add MCP server form"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Open the add MCP server form"); ok {
		t.Error("the Add server trigger still shows over the open form")
	}
	submit, ok := app.AccessByRole(ui.RoleButton, "Add MCP server to Feynman")
	if !ok {
		t.Fatal("the open MCP form offers no submit button")
	}
	if submit.Bounds.H != portrait.Sizes.Regular {
		t.Errorf("Add server button height = %v, want Regular %v", submit.Bounds.H, portrait.Sizes.Regular)
	}
	if _, ok := app.AccessByRole(ui.RoleTextField, "Server name"); !ok {
		t.Fatal("the open MCP form shows no server name field")
	}
}

func TestSettingsActionButtonsUseRegularHeight(t *testing.T) {
	for _, test := range []struct {
		page  string
		draw  func(*panel) func(*ui.Context)
		names []string
	}{
		{page: "models", draw: func(p *panel) func(*ui.Context) { return p.modelSettings }, names: []string{"Set the Feynman service tier"}},
		{page: "search", draw: func(p *panel) func(*ui.Context) { return p.searchSettings }, names: []string{"Set the selected web search provider and API key", "Reset search routing to automatic"}},
		{page: "packages", draw: func(p *panel) func(*ui.Context) { return p.packageSettings }, names: []string{"Update all installed Feynman packages"}},
		{page: "diagnostics", draw: func(p *panel) func(*ui.Context) { return p.statusSettings }, names: []string{"Run feynman doctor", "Run feynman setup preview", "Launch the Feynman setup wizard in Terminal", "Refresh Feynman CLI settings"}},
	} {
		t.Run(test.page, func(t *testing.T) {
			p := fixtureSettingsPanel()
			p.searchProvider = "auto"
			draw := test.draw(p)
			app := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Padding(12).Children(func() { draw(c) })
			}, 1000, 1600)
			app.Frame()
			for _, name := range test.names {
				info, ok := app.AccessByRole(ui.RoleButton, name)
				if !ok {
					t.Errorf("missing settings action %q", name)
					continue
				}
				if info.Bounds.H != portrait.Sizes.Regular {
					t.Errorf("%q height = %v, want the shared Regular %v", name, info.Bounds.H, portrait.Sizes.Regular)
				}
			}
		})
	}
}

func TestModelRowLabelsPinAndPickerAffordances(t *testing.T) {
	p := fixtureSettingsPanel()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() { p.modelSettings(c) })
	}, 1280, 1600)
	app.Frame()
	for _, name := range []string{"Pin Claude Sonnet 4.5", "Pin Claude Opus 4.1", "Show Claude Opus 4.1 in Picker"} {
		info, ok := app.Access(name)
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if info.Bounds.H > portrait.Sizes.Regular {
			t.Errorf("%q height = %v, want at most Regular %v", name, info.Bounds.H, portrait.Sizes.Regular)
		}
	}
	models := 0
	for _, group := range p.providerGroups() {
		models += len(group.models)
	}
	pickerLabels := 0
	for _, value := range app.Texts() {
		if value == "In picker" {
			pickerLabels++
		}
	}
	if pickerLabels != models {
		t.Errorf("%d visible In picker labels, want one for each of the %d models", pickerLabels, models)
	}
}
