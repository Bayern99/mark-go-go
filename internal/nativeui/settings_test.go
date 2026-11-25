package nativeui_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestSettingsSearchClearCloseAndDeepLink(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Feynman = true
	model.Preferences = map[string]string{}
	slots := nativeui.Slots{Settings: []nativeui.SettingsPage{{
		ID: "mcp", Section: "Research", Label: "MCP Servers", Icon: portrait.IconPlugs, Order: 120,
		Keywords: []string{"mcp server"},
		Draw: func(c *ui.Context, q nativeui.SettingsQuery) {
			portrait.SettingsGroup(c, "Servers", func(rows *portrait.Rows) {
				rows.Row(portrait.RowSpec{Title: "MCP server", Description: "Research tools"}, func() {})
			})
		},
	}}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 1280, 820)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyComma)
	app.Frame()
	app.Key(0, ui.KeySlash)
	app.Frame()
	if !app.Focused("Search settings") {
		t.Fatal("/ did not focus Settings search")
	}
	app.Type("dark")
	app.Frame()
	if !app.HasText("Choose Light, Dark, or follow macOS automatically.") {
		t.Fatal("search did not keep the working Appearance row")
	}
	if app.HasText("Reset folded headings and table widths") {
		t.Fatal("search left unrelated Editor rows visible")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if field, ok := app.AccessByRole(ui.RoleTextField, "Search settings"); !ok || field.Value != "" {
		t.Fatalf("first Escape did not clear search: %+v", field)
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Settings"); ok {
		t.Fatal("second Escape did not close Settings")
	}
	shell.OpenSettings("tools")
	app.Frame()
	if !app.HasText("MCP server") {
		t.Fatal("OpenSettings(tools) did not land on MCP Servers")
	}
}

func TestSettingsPreferencesRoundTripThroughActions(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Feynman = true
	model.Preferences = map[string]string{}
	actions := nativeui.Actions{SetPreference: func(key, value string) { model.Preferences[key] = value }}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("editor")
	app.Frame()
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleButton && info.Name == "Increase value" {
			app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
			break
		}
	}
	app.Frame()
	if got := model.Preferences["mgo.editor.readingSize"]; got != "17" {
		t.Fatalf("reading size = %q", got)
	}
	if err := app.Click("Wide"); err != nil {
		t.Fatal(err)
	}
	if info, ok := app.AccessByRole(ui.RoleSwitch, "Line numbers"); ok {
		app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	}
	app.Frame()
	if got := model.Preferences["mgo.editor.readingWidth"]; got != "720" || model.Preferences["mgo.editor.lineNumbers"] != "off" {
		t.Fatalf("editor preferences = %v", model.Preferences)
	}
	if err := app.Click("General"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Markdown"); err != nil {
		t.Fatal(err)
	}
	if info, ok := app.AccessByRole(ui.RoleSwitch, "Show sidebar"); ok {
		app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	}
	if info, ok := app.AccessByRole(ui.RoleSwitch, "Show research panel"); ok {
		app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	}
	app.Frame()
	if model.Preferences["mgo.viewMode.default"] != "markdown" || model.Preferences["mgo.sidebar.hidden"] != "on" || model.Preferences["mgo.research.hidden"] != "on" {
		t.Fatalf("general preferences = %v", model.Preferences)
	}
	if info, ok := app.AccessByRole(ui.RoleMenuButton, "Default navigation view"); ok {
		app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		if err := app.Click("Outline"); err != nil {
			t.Fatal(err)
		}
	}
	app.Frame()
	if got := model.Preferences["mgo.sidebar.view"]; got != "outline" {
		t.Fatalf("sidebar view = %q", got)
	}
}

func TestSettingsResetAndNarrowNavigation(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{"mgo.editor.readingSize": "18"}
	var reset []string
	actions := nativeui.Actions{ResetPreferences: func(keys ...string) {
		reset = append(reset, keys...)
		for _, key := range keys {
			delete(model.Preferences, key)
		}
	}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("editor")
	app.Frame()
	if _, ok := app.Access("Reset Editor to defaults"); !ok {
		t.Fatal("dirty category hides its reset action")
	}
	if err := app.Click("Reset Editor to defaults"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reset to defaults"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if len(reset) == 0 || len(model.Preferences) != 0 {
		t.Fatalf("reset keys %v, preferences %v", reset, model.Preferences)
	}
	app.Frame()
	if _, ok := app.Access("Reset Editor to defaults"); ok {
		t.Fatal("clean category still shows reset")
	}

	narrowShell := new(nativeui.Shell)
	narrow := ui.NewTester(func(c *ui.Context) { narrowShell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 600, 720)
	narrow.Frame()
	narrowShell.OpenSettings("general")
	narrow.Frame()
	if _, ok := narrow.AccessByRole(ui.RoleMenuButton, "Settings category"); !ok {
		t.Fatal("narrow Settings did not collapse its rail to a Select")
	}
}

// TestSettingsSearchDropsAHeadingWithNoMatchingRows keeps a page's link with
// its rows: a keyword that names a page whose settings no longer match leaves
// no heading over nothing, only the empty state.
func TestSettingsSearchDropsAHeadingWithNoMatchingRows(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{}
	slots := nativeui.Slots{Settings: []nativeui.SettingsPage{{
		ID: "zephyr", Section: "System", Label: "Zephyr Deck", Icon: portrait.IconKeyboard, Order: 900,
		Keywords: []string{"deck", "motion"},
		Draw: func(c *ui.Context, q nativeui.SettingsQuery) {
			if !q.Match("Motion speed", "How fast the panel moves") {
				return
			}
			portrait.SettingsGroup(c, "Deck", func(rows *portrait.Rows) {
				rows.Row(portrait.RowSpec{Title: "Motion speed", Description: "How fast the panel moves"}, func() {})
			})
		},
	}}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 1280, 820)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyComma)
	app.Frame()
	app.Key(0, ui.KeySlash)
	app.Frame()
	app.Type("deck")
	app.Frame()
	if _, ok := app.Access("Open Zephyr Deck"); ok {
		t.Fatal("search left a page heading over a page with no matching settings")
	}
	if !app.HasText("No settings match “deck”") {
		t.Fatal("search with no matching settings did not say so")
	}
	// A row that does match keeps its page's heading, above the row.
	app.Key(0, ui.KeyEscape)
	app.Frame()
	app.Type("motion")
	app.Frame()
	if _, ok := app.Access("Open Zephyr Deck"); !ok {
		t.Fatal("search hid the heading of a page whose row matched")
	}
	if !app.HasText("How fast the panel moves") {
		t.Fatal("search did not draw the matching setting")
	}
}

func TestSettingsSearchEnterOpensFirstDisplayedMatchWithoutPhantomGap(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Preferences = map[string]string{}
	slots := nativeui.Slots{Settings: []nativeui.SettingsPage{{
		ID: "search-papers", Section: "Research", Label: "Search & Papers", Icon: portrait.IconMagnifyingGlass, Order: 110,
		Keywords: []string{"api key", "search"},
		Draw: func(c *ui.Context, q nativeui.SettingsQuery) {
			if !q.Match("API key", "Saved provider authentication", "key") {
				return
			}
			portrait.SettingsGroup(c, "Web search", func(rows *portrait.Rows) {
				rows.Row(portrait.RowSpec{Title: "API key", Description: "Saved provider authentication"}, func() {})
			})
		},
	}}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 1280, 820)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyComma)
	app.Frame()
	app.Key(0, ui.KeySlash)
	app.Frame()
	app.Type("key")
	app.Frame()
	if _, ok := app.Access("Open Shortcuts"); ok {
		t.Fatal("search drew an orphan Shortcuts heading")
	}
	link, ok := app.Access("Open Search & Papers")
	if !ok {
		t.Fatal("search did not draw the matching Search & Papers heading")
	}
	content, _ := app.Access("Settings content")
	if delta := link.Bounds.Y - content.Bounds.Y; delta > portrait.Settings.ColumnPadTop+4 {
		t.Fatalf("first matched heading has a phantom gap above it: delta=%v want <= %v", delta, portrait.Settings.ColumnPadTop+4)
	}
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if !app.HasText("API key") || app.HasText("Keyboard shortcuts") {
		t.Fatal("pressing Enter opened a rowless page instead of Search & Papers")
	}
}

func TestClosingSettingsWhilePluginSwitchAnimates(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.GenUI = true
	actions := nativeui.Actions{SetGenUI: func(on bool) { model.GenUI = on }}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	shell.OpenSettings("plugins")
	app.Frame()
	if err := app.Click("Enable GenUI"); err != nil {
		t.Fatal(err)
	}
	if model.GenUI || !app.FramePending() {
		t.Fatal("switch did not start its off animation")
	}
	app.Advance(portrait.Motion.Switch / 4)
	if err := app.Click("Close settings"); err != nil {
		t.Fatal(err)
	}
	app.Advance(portrait.Motion.Enter / 4)
	app.Image()
	app.Advance(portrait.Motion.Enter)
	if _, ok := app.AccessByRole(ui.RoleDialog, "Settings"); ok {
		t.Fatal("Settings did not close")
	}
}
