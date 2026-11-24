package nativeui_test

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func commandBarOpen(app *ui.Tester) bool {
	_, ok := app.AccessByRole(ui.RoleDialog, "Command Bar")
	return ok
}

func TestCommandFieldAndBothShortcutsOpenTheCommandBar(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	if err := app.Click("Search or run a command"); err != nil {
		t.Fatal(err)
	}
	if !commandBarOpen(app) {
		t.Fatal("the sidebar's command field did not open the command bar")
	}
	if !app.Focused("Search documents, headings and commands") {
		t.Fatal("the command bar opened without the keyboard in its field")
	}
	app.Key(0, ui.KeyEscape)
	if commandBarOpen(app) {
		t.Fatal("Escape left the command bar open")
	}
	for _, key := range []ui.Key{ui.KeyK, ui.KeyT} {
		app.Key(ui.Cmd, key)
		if !commandBarOpen(app) {
			t.Fatalf("Command %v did not open the command bar", key)
		}
		app.Key(ui.Cmd, key)
		if commandBarOpen(app) {
			t.Fatalf("Command %v did not close the open command bar", key)
		}
	}
}

func TestCommandBarRunsTheResultChosenWithTheArrows(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	var modes []string
	var created int
	actions := nativeui.Actions{SetMode: func(mode string) { modes = append(modes, mode) }, NewDocument: func() { created++ }}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Key(ui.Cmd, ui.KeyK)
	if first := mustAccess(t, app, ui.RoleOption, "Two.md"); !first.Selected {
		t.Fatal("an empty command bar does not start on the most recent other document")
	}
	if current := mustAccess(t, app, ui.RoleOption, "One.md"); current.Selected || current.Bounds.Y < mustAccess(t, app, ui.RoleOption, "Three.md").Bounds.Y {
		t.Fatalf("the current document %+v should follow the other open documents", current.Bounds)
	}
	app.Type("show")
	if _, ok := app.AccessByRole(ui.RoleOption, "New Document"); ok {
		t.Fatal("results that do not match the query stayed listed")
	}
	if !mustAccess(t, app, ui.RoleOption, "Show Markdown").Selected {
		t.Fatal("typing did not move the selection to the first match")
	}
	app.Key(0, ui.KeyDown)
	if !mustAccess(t, app, ui.RoleOption, "Show Outline").Selected {
		t.Fatal("Down did not select the next result")
	}
	app.Key(0, ui.KeyUp)
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if !slices.Equal(modes, []string{"markdown"}) || commandBarOpen(app) {
		t.Fatalf("Enter ran %v and left the bar open = %v, want Show Markdown then closed", modes, commandBarOpen(app))
	}
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("new doc")
	if err := app.Click("New Document"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if created != 1 || commandBarOpen(app) {
		t.Fatalf("clicking New Document created %d and left the bar open = %v", created, commandBarOpen(app))
	}
}

func TestCommandBarNavigationResultsMoveTheSidebar(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Sections = []workspace.Section{{Heading: "Principles", Level: 2, Start: 40}}
	var revealed []int
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{Reveal: func(offset int) { revealed = append(revealed, offset) }}, nativeui.Slots{})
	}, 1280, 820)
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("hide sidebar")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if _, ok := app.Access("Filter files and headings"); ok {
		t.Fatal("Hide Sidebar left the sidebar docked")
	}
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("show outline")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleTreeItem, "Principles"); !ok {
		t.Fatal("Show Outline did not bring back the sidebar on the document outline")
	}
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("princ")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if !slices.Equal(revealed, []int{40}) {
		t.Fatalf("choosing a heading revealed %v, want offset 40", revealed)
	}
}

func TestCommandBarSearchesTheWorkspaceAndGivesTheSidebarItsFilterBack(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	var filters []string
	type opened struct {
		path   string
		offset int
	}
	var opens []opened
	actions := nativeui.Actions{
		Filter: func(query string) {
			filters = append(filters, query)
			model.Search = workspace.NavSearchResult{Root: "/library", Query: query}
			if query == "road" {
				model.Search.Hits = []workspace.NavSearchHit{{Path: "/library/Plans/Roadmap.md", Name: "Roadmap.md", Headings: []workspace.HeadingHit{{Text: "Road ahead", Offset: 12, Level: 2}}}}
			}
		},
		OpenPath: func(path string, offset int) { opens = append(opens, opened{path, offset}) },
	}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	if err := app.Click("Filter files and headings"); err != nil {
		t.Fatal(err)
	}
	app.Type("one")
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("road")
	app.Frame()
	if filters[len(filters)-1] != "road" {
		t.Fatalf("the command bar searched %v, want road last", filters)
	}
	file := mustAccess(t, app, ui.RoleOption, "Roadmap.md")
	if file.Description != "Plans" {
		t.Errorf("file result says %q, want its folder Plans", file.Description)
	}
	if err := app.Click("Road ahead"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if len(opens) != 1 || opens[0] != (opened{"/library/Plans/Roadmap.md", 12}) {
		t.Fatalf("choosing a heading opened %v, want Roadmap.md at offset 12", opens)
	}
	if filters[len(filters)-1] != "one" {
		t.Fatalf("closing the command bar left the workspace filter at %q, want the sidebar's own %q", filters[len(filters)-1], "one")
	}
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("road")
	app.Key(0, ui.KeyEscape)
	if commandBarOpen(app) || filters[len(filters)-1] != "one" {
		t.Fatalf("Escape left the bar open = %v with the filter at %q", commandBarOpen(app), filters[len(filters)-1])
	}
}

func TestPageBarCarriesPathStateAndViewsInOneRow(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Workspace.Documents[0].Edited = true
	model.Crumbs = []workspace.Crumb{{Name: "Library", Path: "/library"}, {Name: "One.md", Path: "/library/One.md", IsFile: true}}
	var revealed []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{RevealPath: func(path string) { revealed = append(revealed, path) }}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	page := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	row := func(info ui.AccessInfo) bool {
		return info.Bounds.Y >= page.Y && info.Bounds.Y+info.Bounds.H <= page.Y+40
	}
	for _, target := range []struct {
		role ui.Role
		name string
	}{{ui.RoleButton, "Reveal Library"}, {ui.RoleStatus, "Edited"}, {ui.RoleTab, "Preview"}, {ui.RoleTab, "Markdown"}} {
		if info := mustAccess(t, app, target.role, target.name); !row(info) {
			t.Errorf("%s at %v is outside the page bar at the top of %v", target.name, info.Bounds, page)
		}
	}
	if app.HasText("/library/One.md") || app.HasText("Saved") || slices.ContainsFunc(app.Texts(), func(text string) bool { return len(text) > 1 && text[0] == 'v' && text[1] >= '0' && text[1] <= '9' }) {
		t.Errorf("the page still draws a status bar: %v", app.Texts())
	}
	if err := app.Click("Reveal Library"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(revealed, []string{"/library"}) {
		t.Fatalf("the folder crumb revealed %v", revealed)
	}
}
