package nativeui_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestCommandBarRestoresEditorFocusOnEscapeDismissAndRun(t *testing.T) {
	for _, close := range []string{"escape", "dismiss", "run"} {
		t.Run(close, func(t *testing.T) {
			model := libraryModel()
			model.Workspace.Documents = model.Workspace.Documents[:1]
			var shell nativeui.Shell
			var editor ui.Handle
			text := ""
			created := 0
			actions := nativeui.Actions{
				CommandFocus: func() *ui.Handle { return &editor },
				NewDocument:  func() { created++ },
			}
			slots := nativeui.Slots{Document: func(c *ui.Context) {
				ui.TextInputBase(c.Key("editor"), &text).Label("Document source").Bind(&editor)
			}}
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, slots) }, 1000, 720)
			app.Frame()
			if err := app.Click("Document source"); err != nil {
				t.Fatal(err)
			}
			app.Key(ui.Cmd, ui.KeyK)
			if !app.Focused("Search documents, headings and commands") {
				t.Fatal("Command Bar did not focus its query")
			}
			switch close {
			case "escape":
				app.Key(0, ui.KeyEscape)
			case "dismiss":
				app.ClickAt(1, 1)
			case "run":
				app.Type("new doc")
				app.Key(0, ui.KeyEnter)
			}
			app.Frame()
			if !app.Focused("Document source") {
				t.Fatalf("%s did not restore editor focus", close)
			}
			if close == "run" && created != 1 {
				t.Fatalf("running a command created %d documents", created)
			}
		})
	}
}

func TestCommandBarRestoresSidebarFilterFocus(t *testing.T) {
	model := libraryModel()
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{Filter: func(query string) { model.Search.Query = query }}, nativeui.Slots{})
	}, 1000, 720)
	app.Frame()
	if err := app.Click("Filter files"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Filter files and headings"); err != nil {
		t.Fatal(err)
	}
	app.Type("notes")
	app.Key(ui.Cmd, ui.KeyK)
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if !app.Focused("Filter files and headings") {
		t.Fatal("closing Command Bar did not return focus to the sidebar filter")
	}
}

func TestCommandBarDocumentCommandsAndSynonymsUseWindowActions(t *testing.T) {
	for _, tc := range []struct {
		query, option, want string
	}{
		{"save", "Save", "save:a"},
		{"save as", "Save As…", "saveAs:a"},
		{"rename", "Rename…", "rename:a"},
		{"move", "Move…", "move:a"},
		{"revert", "Revert", "revert:a"},
		{"open file", "Open…", "openFiles"},
		{"find", "Find", "find"},
		{"undo", "Undo", "undo"},
		{"colour", "Toggle Dark Mode", "appearance"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			model := libraryModel()
			model.Workspace.Documents = model.Workspace.Documents[:1]
			var events []string
			actions := nativeui.Actions{
				OpenFiles: func() { events = append(events, "openFiles") },
				Find:      func() { events = append(events, "find") },
				Undo:      func() { events = append(events, "undo") },
				Save:      func(id string) { events = append(events, "save:"+id) },
				SaveAs:    func(id string) { events = append(events, "saveAs:"+id) },
				Rename:    func(id string) { events = append(events, "rename:"+id) },
				Move:      func(id string) { events = append(events, "move:"+id) },
				Revert:    func(id string) { events = append(events, "revert:"+id) },
				SetAppearance: func(portrait.Appearance) {
					events = append(events, "appearance")
				},
			}
			var shell nativeui.Shell
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1100, 720)
			app.Frame()
			app.Key(ui.Cmd, ui.KeyK)
			app.Type(tc.query)
			if _, ok := app.AccessByRole(ui.RoleOption, tc.option); !ok {
				t.Fatalf("query %q has no %q option: %v", tc.query, tc.option, app.Texts())
			}
			app.Key(0, ui.KeyEnter)
			app.Frame()
			if !slices.Contains(events, tc.want) {
				t.Fatalf("query %q ran %v, want %q", tc.query, events, tc.want)
			}
		})
	}
}

func TestCommandBarSettingsKeywordOpensSettings(t *testing.T) {
	model := libraryModel()
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1100, 720)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("keyboard")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Settings"); !ok {
		t.Fatal("keyboard synonym did not open Settings")
	}
}

func darkestCategoryPixel(t *testing.T, app *ui.Tester, label string) uint32 {
	t.Helper()
	r, ok := app.Find(label)
	if !ok {
		t.Fatalf("category %q is missing", label)
	}
	darkest := uint32(3 * 255)
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			r, g, b, _ := app.Image().At(x, y).RGBA()
			darkest = min(darkest, uint32(r>>8)+uint32(g>>8)+uint32(b>>8))
		}
	}
	return darkest
}

func TestSettingsUnmatchedCategoriesStayReadableAndClickable(t *testing.T) {
	model := libraryModel()
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("general")
	app.Frame()
	before := darkestCategoryPixel(t, app, "General")
	app.Key(0, ui.KeySlash)
	app.Type("dark")
	app.Frame()
	after := darkestCategoryPixel(t, app, "General")
	if after > before+3 {
		t.Fatalf("unmatched category ink faded from channel sum %v to %v", before, after)
	}
	if err := app.Click("General"); err != nil {
		t.Fatalf("unmatched category is not clickable: %v", err)
	}
	app.Frame()
	if !app.HasText("Keep the workspace navigation visible.") {
		t.Fatal("clicking an unmatched category did not open General")
	}
}

func TestPreferenceFailureAlertRemainsVisibleInsideSettings(t *testing.T) {
	model := libraryModel()
	model.Message = "Could not save preferences"
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("diagnostics")
	app.Frame()
	dialog := mustAccess(t, app, ui.RoleDialog, "Settings").Bounds
	found := false
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleAlert && info.Name == model.Message && info.Bounds.X >= dialog.X && info.Bounds.Y >= dialog.Y && info.Bounds.X+info.Bounds.W <= dialog.X+dialog.W && info.Bounds.Y+info.Bounds.H <= dialog.Y+dialog.H {
			found = true
		}
	}
	if !found {
		t.Fatal("preference failure is only behind the Settings sheet")
	}
}

func TestSettingsResetAllCancelReturnsFocusToItsTrigger(t *testing.T) {
	model := libraryModel()
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("diagnostics")
	app.Frame()
	reset := mustAccess(t, app, ui.RoleButton, "Reset all settings")
	app.ClickAt(reset.Bounds.X+reset.Bounds.W/2, reset.Bounds.Y+reset.Bounds.H/2)
	app.Frame()
	if err := app.Click("Cancel"); err != nil {
		t.Fatalf("%v; visible text: %v", err, app.Texts())
	}
	app.Frame()
	if !app.Focused("Reset all settings") {
		t.Fatal("canceling reset did not return focus to Reset all settings")
	}
}

func TestSettingsValueChoicesAreRadioGroups(t *testing.T) {
	model := libraryModel()
	model.Preferences = map[string]string{}
	actions := nativeui.Actions{
		SetPreference: func(key, value string) { model.Preferences[key] = value },
		SetAppearance: func(appearance portrait.Appearance) { model.Appearance = appearance },
	}
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("editor")
	app.Frame()
	if err := app.Click("Wide"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if wide := mustAccess(t, app, ui.RoleRadio, "Wide"); !wide.Checked {
		t.Fatalf("Reading line width radio = %+v", wide)
	}
	shell.OpenSettings("appearance")
	app.Frame()
	if err := app.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if dark := mustAccess(t, app, ui.RoleRadio, "Dark"); !dark.Checked {
		t.Fatal("Appearance is not a selected radio")
	}
}

func TestNavigationControlsHavePressedFeedback(t *testing.T) {
	for _, target := range []string{"Search or run a command", "One.md", "Two.md", "Space: Wood–Fire", "Show Library"} {
		t.Run(target, func(t *testing.T) {
			model := libraryModel()
			model.Navigation.Entries = []workspace.NavEntry{{ID: "one", Kind: "file", Name: "One.md", Path: "/library/One.md", DocumentID: "a"}}
			var shell nativeui.Shell
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
			app.Frame()
			info, ok := app.Access(target)
			if target == "One.md" {
				info, ok = app.AccessByRole(ui.RoleTreeItem, target)
			}
			if !ok {
				t.Fatalf("missing %q", target)
			}
			before := pixels(app.Image(), info.Bounds)
			app.Press(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
			app.Advance(portrait.Motion.Press + 20*time.Millisecond)
			if changed(before, pixels(app.Image(), info.Bounds)) == 0 {
				t.Fatal("control has no visible pressed feedback")
			}
		})
	}
}

func TestNarrowResearchDrawerHasOneVisibleTitleAndKeepsPanelTools(t *testing.T) {
	model := libraryModel()
	model.Feynman = true
	var shell nativeui.Shell
	slots := nativeui.Slots{Research: func(c *ui.Context) {
		ui.Row(c).Height(portrait.Shell.PageBar).AlignItems(ui.Center).Children(func() {
			portrait.Type.UI.Apply(ui.Text(c, "Research")).TextColor(portrait.Colors.Ink)
			portrait.QuietButton(c, "Workspace conversation history", "Workspace conversation history")
		})
	}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 480, 720)
	app.Frame()
	app.Key(ui.Cmd|ui.Alt, ui.Key0)
	app.Advance(time.Second)
	visibleTitles := 0
	for _, text := range app.Texts() {
		if text == "Research" {
			visibleTitles++
		}
	}
	// One is the dialog's accessible label and one is the single visible title.
	if visibleTitles != 2 {
		t.Fatalf("Research title/label count = %d, want 2: %v", visibleTitles, app.Texts())
	}
	if _, ok := app.Access("Workspace conversation history"); !ok {
		t.Fatal("narrow drawer removed the research panel's tools")
	}
}

func TestGeneratingStatusIsAHollowRingInEarthLightAndDark(t *testing.T) {
	for _, appearance := range []portrait.Appearance{portrait.Light, portrait.Dark} {
		t.Run(string(appearance), func(t *testing.T) {
			model := libraryModel()
			model.Spaces = map[string]string{"/library": "Earth"}
			model.Appearance = appearance
			model.Workspace.Documents = model.Workspace.Documents[:1]
			model.Workspace.Documents[0].Edited = true
			model.Workspace.Documents[0].Generating = true
			var shell nativeui.Shell
			app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
			app.Frame()
			generating := documentStatusDot(t, app, "Generating").Bounds
			x, y := int(generating.X+generating.W/2), int(generating.Y+generating.H/2)
			r, g, b, _ := app.Image().At(x, y).RGBA()
			accent := portrait.Tones.Accent.Solid
			if uint8(r>>8) == accent.R && uint8(g>>8) == accent.G && uint8(b>>8) == accent.B {
				t.Fatal("Generating is still a solid accent circle")
			}
		})
	}
}

func TestHoveringActiveTabAndFileRowSettlesWithoutPerpetualRedraw(t *testing.T) {
	model := libraryModel()
	model.Navigation.Entries = []workspace.NavEntry{
		{ID: "one", Kind: "file", Name: "One.md", Path: "/library/One.md", DocumentID: "a"},
	}
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	for _, tc := range []struct {
		kind string
		name string
		role ui.Role
	}{
		{"tab", "One.md", ui.RoleTab},
		{"tree", "One.md", ui.RoleTreeItem},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			info := mustAccess(t, app, tc.role, tc.name)
			app.Move(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
			app.Advance(2 * time.Second)
			app.Frame()
			if app.FramePending() {
				t.Fatalf("hovering active %s keeps FramePending=true after settling", tc.kind)
			}
		})
	}
}

func TestSettingsSearchRapidTypeAndEnterOpensDisplayedMatch(t *testing.T) {
	model := libraryModel()
	var shell nativeui.Shell
	slots := nativeui.Slots{
		Settings: []nativeui.SettingsPage{{
			ID: "search", Label: "Search & Papers", Section: "Research",
			Keywords: []string{"web search api key"},
			Draw: func(c *ui.Context, q nativeui.SettingsQuery) {
				if q.Match("API key", "Provider & API key", "web search api key") {
					portrait.Label(c, "API key")
				}
			},
		}},
	}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("general")
	app.Frame()
	app.Key(0, ui.KeySlash)
	app.Frame()
	// Deliver text and Enter without an intermediate frame.
	app.Type("key")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleHeading, "Search & Papers"); !ok {
		t.Fatalf("rapid Enter did not open matched Search & Papers page; visible texts: %v", app.Texts())
	}
}
