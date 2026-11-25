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
	if _, ok := app.Access("Workspace files"); !ok {
		t.Fatal("workspace tree is not docked at the default size")
	}
}

func TestResearchDrawerReturnsKeyboardFocusAfterResize(t *testing.T) {
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, nativeui.Model{Feynman: true}, nativeui.Actions{}, nativeui.Slots{})
	}, 480, 360)
	app.Key(ui.Cmd|ui.Alt, ui.Key0)
	if _, ok := app.AccessByRole(ui.RoleDialog, "Research"); !ok {
		t.Fatal("the research shortcut did not open the drawer")
	}
	app.SetSize(900, 600)
	if _, ok := app.AccessByRole(ui.RoleDialog, "Research"); !ok {
		t.Fatal("resizing to a docked navigation layout closed the research drawer")
	}
	app.Key(0, ui.KeyEscape)
	if _, ok := app.AccessByRole(ui.RoleDialog, "Research"); ok {
		t.Fatal("Escape did not close the research drawer")
	}
	if !app.Focused("Toggle research") {
		t.Fatal("closing the drawer should focus the research button in the resized layout")
	}
}

func TestCommandBarOpensADocumentBySearchAndControlTabReturns(t *testing.T) {
	var shell nativeui.Shell
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "One.md"}, {ID: "b", Name: "Two.md"}, {ID: "c", Name: "Three.md"}, {ID: "d", Name: "Four.md"}, {ID: "e", Name: "Five.md"}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{Activate: func(id string) { model.Workspace.ActiveID = id }}, nativeui.Slots{})
	}, 480, 360)
	app.Key(ui.Cmd, ui.KeyK)
	if _, ok := app.AccessByRole(ui.RoleDialog, "Command Bar"); !ok {
		t.Fatal("Command K did not open the command bar")
	}
	app.Type("Four")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if model.Workspace.ActiveID != "d" {
		t.Fatal("Enter did not open the filtered document")
	}
	if _, ok := app.AccessByRole(ui.RoleDialog, "Command Bar"); ok {
		t.Fatal("the command bar stayed open after running its first result")
	}
	app.Key(ui.Ctrl, ui.KeyTab)
	app.Frame()
	if model.Workspace.ActiveID != "a" {
		t.Fatal("Control Tab did not return to the last active document")
	}
}

func TestSelectedViewLabelIsPaintedAboveItsCapsule(t *testing.T) {
	var shell nativeui.Shell
	m := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview"}}}}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, m, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	tab, ok := app.AccessByRole(ui.RoleTab, "Preview")
	if !ok {
		t.Fatal("missing Preview tab")
	}
	img := app.Image()
	box := tab.Bounds
	dark := 0
	for y := int(box.Y) + 4; y < int(box.Y+box.H)-4; y++ {
		for x := int(box.X) + 4; x < int(box.X+box.W)-4; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 0x7000 && g < 0x7000 && b < 0x7000 {
				dark++
			}
		}
	}
	if dark < 10 {
		t.Fatalf("selected Preview label was obscured (%d dark pixels)", dark)
	}
}

func TestDraggedPathsReachTheWorkspaceInOrder(t *testing.T) {
	var shell nativeui.Shell
	var paths []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, nativeui.Model{}, nativeui.Actions{Drop: func(files []string) { paths = append(paths, files...) }}, nativeui.Slots{})
	}, 480, 360)
	if !app.DropFiles(200, 120, "/tmp/library", "/tmp/library/notes.md") || len(paths) != 2 || paths[0] != "/tmp/library" || paths[1] != "/tmp/library/notes.md" {
		t.Fatalf("dropped group was lost or reordered: %v", paths)
	}
}

func TestRecentWorkspacesWithSameNameOpenOnlySelectedRoot(t *testing.T) {
	var shell nativeui.Shell
	var opened []string
	model := nativeui.Model{Recents: []workspace.RecentWorkspace{
		{Name: "notes", Root: "/work/notes"},
		{Name: "notes", Root: "/personal/notes"},
	}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{OpenRecent: func(root string) { opened = append(opened, root) }}, nativeui.Slots{})
	}, 480, 360)
	app.Frame()
	var buttons []ui.AccessInfo
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleButton && info.Name == "notes" {
			buttons = append(buttons, info)
		}
	}
	if len(buttons) != 2 {
		t.Fatalf("recent workspace buttons = %d, want 2", len(buttons))
	}
	second := buttons[1].Bounds
	app.ClickAt(second.X+second.W/2, second.Y+second.H/2)
	app.Frame()
	if len(opened) != 1 || opened[0] != "/personal/notes" {
		t.Fatalf("opened roots = %v, want only /personal/notes", opened)
	}
}
