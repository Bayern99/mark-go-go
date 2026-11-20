package nativeui_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestNarrowNavigationReturnsKeyboardFocus(t *testing.T) {
	var shell nativeui.Shell
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 480, 360)
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.AccessByRole(ui.RoleDialog, "Navigation"); !ok {
		t.Fatal("navigation drawer did not open")
	}
	app.Key(0, ui.KeyEscape)
	if !app.Focused("Show navigation") {
		t.Fatal("closing the drawer should return focus to its opener")
	}
	app.SetSize(1280, 820)
	if _, ok := app.Access("Filter files and headings"); !ok {
		t.Fatal("workspace tree is not docked at the default size")
	}
}

func TestPortraitHasOfflineTypefaces(t *testing.T) {
	if err := nativeui.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, f := range ui.FontFamilies() {
		if f.Registered {
			registered[f.Name] = true
		}
	}
	for _, name := range []string{"IBM Plex Sans", "IBM Plex Sans SC", "IBM Plex Mono", "Barlow Semi Condensed", "Phosphor"} {
		if !registered[name] {
			t.Errorf("typeface absent from native text system: %s", name)
		}
	}
}

func TestHiddenTabCanBeChosenBySearchAndReturnedToByKeyboard(t *testing.T) {
	var shell nativeui.Shell
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "One.md"}, {ID: "b", Name: "Two.md"}, {ID: "c", Name: "Three.md"}, {ID: "d", Name: "Four.md"}, {ID: "e", Name: "Five.md"}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{Activate: func(id string) { model.Workspace.ActiveID = id }}, nativeui.Slots{})
	}, 480, 360)
	if err := app.Click("More documents"); err != nil {
		t.Fatal(err)
	}
	app.Type("Four")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if model.Workspace.ActiveID != "d" {
		t.Fatal("Enter did not open the filtered document")
	}
	app.Key(ui.Ctrl, ui.KeyTab)
	app.Frame()
	if model.Workspace.ActiveID != "a" {
		t.Fatal("Control Tab did not return to the last active document")
	}
}
