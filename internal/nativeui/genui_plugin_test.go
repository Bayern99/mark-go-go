package nativeui_test

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func genuiPage() nativeui.SettingsPage {
	return nativeui.SettingsPage{
		ID: "genui", Section: "Workspace", Label: "GenUI", Icon: portrait.IconSparkle, Order: 210,
		Draw: func(c *ui.Context, q nativeui.SettingsQuery) { ui.Text(c, "GenUI generation settings") },
	}
}

func genuiSlots() nativeui.Slots {
	return nativeui.Slots{
		Document: func(c *ui.Context) { ui.Text(c, "Markdown source") },
		GenUI:    func(c *ui.Context) { ui.Text(c, "GenUI reader") },
		Settings: []nativeui.SettingsPage{genuiPage()},
	}
}

func TestGenUIOffOffersNoGenUIViewOrBackLink(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{}
	model.Workspace.Documents[0].ViewMode = "genui"
	model.ReturnToGenUI = true
	slots := genuiSlots()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 1280, 820)
	app.Frame()

	if _, ok := app.AccessByRole(ui.RoleTab, "GenUI"); ok {
		t.Fatal("GenUI stayed in the view switch while the plugin is off")
	}
	if app.HasText("Back to GenUI") {
		t.Fatal("the Back to GenUI prompt stayed while the plugin is off")
	}
	if !mustAccess(t, app, ui.RoleTab, "Preview").Selected {
		t.Fatal("a document left on GenUI did not fall back to Preview")
	}
	if !app.HasText("Markdown source") || app.HasText("GenUI reader") {
		t.Fatal("the page did not fall back from GenUI to the Markdown view")
	}

	model.GenUI = true
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleTab, "GenUI"); !ok {
		t.Fatal("turning GenUI on did not bring back its view switch")
	}
	if !mustAccess(t, app, ui.RoleTab, "GenUI").Selected || !app.HasText("GenUI reader") {
		t.Fatal("the document did not return to GenUI")
	}
	if !app.HasText("Back to GenUI") {
		t.Fatal("turning GenUI on did not bring back the Back to GenUI prompt")
	}
}

func TestCommandBarOffersGenUIOnlyWhileItsPluginIsOn(t *testing.T) {
	for _, on := range []bool{true, false} {
		var shell nativeui.Shell
		model := libraryModel()
		model.Preferences = map[string]string{}
		model.GenUI = on
		actions := nativeui.Actions{SetMode: func(string) {}}
		app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, genuiSlots()) }, 1280, 820)
		app.Frame()
		app.Key(ui.Cmd, ui.KeyK)
		app.Type("genui")
		app.Frame()
		_, offered := app.AccessByRole(ui.RoleOption, "Show GenUI")
		if offered != on {
			t.Fatalf("With GenUI %v the command bar offered Show GenUI = %v", on, offered)
		}
	}
}

func TestPluginsPageTogglesGenUIAndOpensItsSettings(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{}
	model.GenUI = true
	var genui []bool
	actions := nativeui.Actions{SetGenUI: func(on bool) { genui = append(genui, on); model.GenUI = on }}
	slots := genuiSlots()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, slots) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("plugins")
	app.Frame()

	if !app.HasText("WORKSPACE") {
		t.Fatal("the Plugins page is not in the Workspace group")
	}
	if on := mustAccess(t, app, ui.RoleSwitch, "Enable GenUI"); !on.Checked {
		t.Fatal("GenUI reads as off in Plugins while its plugin is on")
	}
	if _, ok := app.AccessByRole(ui.RoleSwitch, "Enable Feynman"); !ok {
		t.Fatal("Plugins lost the Feynman switch")
	}
	if _, ok := app.AccessByRole(ui.RoleSwitch, "Enable Git"); !ok {
		t.Fatal("Plugins lost the Git switch")
	}
	if err := app.Click("Configure GenUI"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("GenUI generation settings") {
		t.Fatal("Configure GenUI did not open the GenUI page")
	}

	shell.OpenSettings("plugins")
	app.Frame()
	if err := app.Click("Enable GenUI"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !slices.Equal(genui, []bool{false}) || model.GenUI {
		t.Fatalf("Enable GenUI sent %v, want one off", genui)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Configure GenUI"); ok {
		t.Fatal("a disabled plugin still offered its settings link")
	}
	if err := app.Click("Enable GenUI"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !slices.Equal(genui, []bool{false, true}) || !model.GenUI {
		t.Fatalf("Enable GenUI sent %v, want off then on", genui)
	}
}

func TestCommandBarTogglesGenUIAndReachesItsSettings(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{}
	model.GenUI = true
	var genui []bool
	actions := nativeui.Actions{SetGenUI: func(on bool) { genui = append(genui, on); model.GenUI = on }}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, genuiSlots()) }, 1280, 820)
	app.Frame()

	command := func(query, title string) {
		t.Helper()
		app.Key(ui.Cmd, ui.KeyK)
		app.Type(query)
		app.Frame()
		if err := app.Click(title); err != nil {
			t.Fatal(err)
		}
		app.Frame()
	}
	command("genui", "Disable GenUI")
	if !slices.Equal(genui, []bool{false}) || model.GenUI {
		t.Fatalf("Disable GenUI sent %v", genui)
	}
	command("genui", "Enable GenUI")
	if !slices.Equal(genui, []bool{false, true}) || !model.GenUI {
		t.Fatalf("Enable GenUI sent %v", genui)
	}
	command("plugins", "Settings: Plugins")
	if _, ok := app.AccessByRole(ui.RoleSwitch, "Enable GenUI"); !ok {
		t.Fatal("Settings: Plugins did not open the Plugins page")
	}
}
