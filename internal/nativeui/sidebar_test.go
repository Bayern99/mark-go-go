package nativeui_test

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func libraryModel() nativeui.Model {
	root := "/library"
	return nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Kind: "folder", Name: "Library", Root: root, ActiveID: "a",
		Workspaces: []workspace.WorkspaceSummary{{ID: "w1", Root: root, Name: "Library", Kind: "folder"}, {ID: "w2", Root: "/lab", Name: "Lab", Kind: "folder"}},
		Documents: []workspace.DocumentInfo{
			{ID: "a", Name: "One.md", Path: root + "/One.md", ViewMode: "preview"},
			{ID: "b", Name: "Two.md", Path: root + "/Two.md", ViewMode: "preview"},
			{ID: "c", Name: "Three.md", Path: root + "/Three.md", ViewMode: "preview"},
		},
	}}
}

func mustAccess(t *testing.T, app *ui.Tester, role ui.Role, name string) ui.AccessInfo {
	t.Helper()
	info, ok := app.AccessByRole(role, name)
	if !ok {
		t.Fatalf("missing %q", name)
	}
	return info
}

func TestOpenDocumentsAreVerticalTabsInTheSidebar(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{Activate: func(id string) { model.Workspace.ActiveID = id }}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	side := mustAccess(t, app, ui.RoleNavigation, "Workspace navigation").Bounds
	page := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	list := mustAccess(t, app, ui.RoleTabList, "Documents").Bounds
	if list.X < side.X || list.X+list.W > side.X+side.W || list.X+list.W > page.X {
		t.Fatalf("document tabs %v are not inside the sidebar %v beside the page %v", list, side, page)
	}
	var last ui.Rect
	for i, name := range []string{"One.md", "Two.md", "Three.md"} {
		tab := mustAccess(t, app, ui.RoleTab, name)
		if tab.Selected != (i == 0) {
			t.Errorf("%s selected = %v", name, tab.Selected)
		}
		if tab.Bounds.H != portrait.Shell.Row {
			t.Errorf("%s is %v tall, want the shell row %v", name, tab.Bounds.H, portrait.Shell.Row)
		}
		if i > 0 && (tab.Bounds.X != last.X || tab.Bounds.Y < last.Y+last.H) {
			t.Errorf("%s at %v is not stacked under the tab at %v", name, tab.Bounds, last)
		}
		last = tab.Bounds
	}
	if err := app.Click("Two.md"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if model.Workspace.ActiveID != "b" {
		t.Fatalf("clicking a tab activated %q", model.Workspace.ActiveID)
	}
	for _, step := range []struct {
		key  ui.Key
		want string
	}{{ui.KeyDown, "c"}, {ui.KeyDown, "a"}, {ui.KeyUp, "c"}, {ui.KeyHome, "a"}, {ui.KeyEnd, "c"}} {
		focus(t, app, map[string]string{"a": "One.md", "b": "Two.md", "c": "Three.md"}[model.Workspace.ActiveID])
		app.Key(0, step.key)
		app.Frame()
		if model.Workspace.ActiveID != step.want {
			t.Fatalf("key %v moved to %q, want %q", step.key, model.Workspace.ActiveID, step.want)
		}
	}
}

func TestCommandZeroHidesTheDockedSidebarAndTheToggleBringsItBack(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	docked := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	if _, ok := app.Access("Show navigation"); ok {
		t.Fatal("the page offers to show a sidebar that is already docked")
	}
	app.Key(ui.Cmd, ui.Key0)
	app.Frame()
	if _, ok := app.Access("Workspace files"); ok {
		t.Fatal("Command 0 left the sidebar docked")
	}
	alone := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	if alone.X != portrait.Shell.PageInset || alone.W <= docked.W || alone.X+alone.W != 1280-portrait.Shell.PageInset {
		t.Fatalf("page alone is %v, want it inset %v from both window edges and wider than %v", alone, portrait.Shell.PageInset, docked)
	}
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Navigation"); ok {
		t.Fatal("a wide window opened the sidebar as a drawer")
	}
	if _, ok := app.Access("Workspace files"); !ok {
		t.Fatal("the toggle did not dock the sidebar again")
	}
	if err := app.Click("Hide sidebar"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.Access("Workspace files"); ok {
		t.Fatal("Hide sidebar left the sidebar docked")
	}
}

func TestSidebarKeepsClearOfTheWindowControls(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	bar := ui.TitleBar{Height: 56, Left: 78, Bottom: 35}
	app.SetTitleBar(bar)
	app.Frame()
	if field := mustAccess(t, app, ui.RoleButton, "Search or run a command").Bounds; field.Y < bar.Bottom {
		t.Fatalf("command field at %v sits under the window controls, which end at %v", field, bar.Bottom)
	}
	hide := mustAccess(t, app, ui.RoleButton, "Hide sidebar").Bounds
	if hide.X < bar.Left {
		t.Fatalf("Hide sidebar at %v overlaps the window controls, %v wide", hide, bar.Left)
	}
	views := mustAccess(t, app, ui.RoleTab, "Preview").Bounds
	if line := bar.Height / 2; hide.Y+hide.H/2 != line || views.Y+views.H/2 != line {
		t.Fatalf("Hide sidebar is centred at %v and the view switch at %v, want both on the window controls' line %v", hide.Y+hide.H/2, views.Y+views.H/2, line)
	}
	app.Key(ui.Cmd, ui.Key0)
	app.Frame()
	if show := mustAccess(t, app, ui.RoleButton, "Show navigation").Bounds; show.X < bar.Left || show.Y+show.H/2 != bar.Height/2 {
		t.Fatalf("Show navigation at %v is not beside the window controls, %v wide on line %v", show, bar.Left, bar.Height/2)
	}
}

func TestWorkspaceMarksSwitchWorkspaces(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	var activated []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{ActivateWorkspace: func(id string) { activated = append(activated, id) }}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	if current := mustAccess(t, app, ui.RoleTab, "Show Library"); !current.Selected {
		t.Fatal("the current workspace's mark is not selected")
	}
	if err := app.Click("Show Library"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Show Lab"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(activated, []string{"w2"}) {
		t.Fatalf("workspace marks activated %v, want only w2", activated)
	}
}

func TestSceneryMenuStoresTheChosenSpace(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	var chosen []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{SetSpace: func(name string) { chosen = append(chosen, name) }}, nativeui.Slots{})
	}, 1280, 820)
	if err := app.Click("Switch workspace"); err != nil {
		t.Fatal(err)
	}
	if err := app.ChooseMenuItem("Space", "Wood–Fire"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(chosen, []string{"WoodFire"}) {
		t.Fatalf("space menu stored %v, want WoodFire", chosen)
	}
}

func sceneryAt(app *ui.Tester, x, y int) [3]int {
	r, g, b, _ := app.Image().At(x, y).RGBA()
	return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
}

func within(pixel [3]int, space portrait.Space) bool {
	s := space.Light
	from, to := [3]int{int(s.From.R), int(s.From.G), int(s.From.B)}, [3]int{int(s.To.R), int(s.To.G), int(s.To.B)}
	for i := range pixel {
		if pixel[i] < min(from[i], to[i])-2 || pixel[i] > max(from[i], to[i])+2 {
			return false
		}
	}
	return true
}

func TestWindowFieldIsTheWorkspaceSceneryAndThePageIsPaper(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	space := portrait.SpaceFor("/library")
	side := mustAccess(t, app, ui.RoleNavigation, "Workspace navigation").Bounds
	field := [2]int{int(side.X + 3), int(side.Y + side.H - 2)}
	if pixel := sceneryAt(app, field[0], field[1]); !within(pixel, space) {
		t.Fatalf("sidebar field is %v, want the %s scenery between %+v and %+v", pixel, space.Name, space.Light.From, space.Light.To)
	}
	page := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	if pixel := sceneryAt(app, int(page.X+page.W/2), int(page.Y+page.H/2)); pixel != [3]int{255, 255, 255} {
		t.Fatalf("page is %v, want opaque paper", pixel)
	}
	if gap := sceneryAt(app, int(page.X+page.W/2), int(page.Y-portrait.Shell.PageInset/2)); !within(gap, space) {
		t.Fatalf("the inset above the page is %v, want the scenery", gap)
	}
	for _, other := range portrait.Spaces {
		if other == space {
			continue
		}
		model.Spaces = map[string]string{"/library": other.Name}
		app.Frame()
		if pixel := sceneryAt(app, field[0], field[1]); !within(pixel, other) {
			t.Fatalf("with %s chosen the field is %v, want between %+v and %+v", other.Name, pixel, other.Light.From, other.Light.To)
		}
		break
	}
	model.Spaces = map[string]string{"/library": "Nowhere"}
	app.Frame()
	if pixel := sceneryAt(app, field[0], field[1]); !within(pixel, space) {
		t.Fatalf("an unknown scenery name painted %v, want the workspace's own %s", pixel, space.Name)
	}
}

func TestSceneryLetsTheWindowMaterialThroughUnlessTransparencyIsReduced(t *testing.T) {
	render := func(vibrancy bool, prefs ui.Preferences) [3]int {
		var shell nativeui.Shell
		model := libraryModel()
		app := ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820, ui.TesterOptions{Preferences: prefs})
		app.SetVibrancy(vibrancy)
		app.Frame()
		return sceneryAt(app, 3, 800)
	}
	opaque := render(false, ui.Preferences{})
	if through := render(true, ui.Preferences{}); through == opaque {
		t.Fatalf("over a window material the field is still the opaque %v", opaque)
	}
	if reduced := render(true, ui.Preferences{ReduceTransparency: true}); reduced != opaque {
		t.Fatalf("with reduced transparency the field is %v, want the opaque %v", reduced, opaque)
	}
}

func TestZeroIndentDrillDownAndAncestors(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Navigation = workspace.NavSnapshot{
		Root: "/library",
		Entries: []workspace.NavEntry{
			{ID: "folder:Essays", Kind: "folder", Name: "Essays", Path: "/library/Essays", Depth: 0, MarkdownCount: 2, FolderState: workspace.FolderOpen},
			{ID: "file:Essays/A.md", Kind: "file", Name: "A.md", Path: "/library/Essays/A.md", Depth: 1},
			{ID: "folder:Essays/Deep", Kind: "folder", Name: "Deep", Path: "/library/Essays/Deep", Depth: 1, MarkdownCount: 1, FolderState: workspace.FolderOpen},
			{ID: "file:Essays/Deep/B.md", Kind: "file", Name: "B.md", Path: "/library/Essays/Deep/B.md", Depth: 2},
			{ID: "file:Root.md", Kind: "file", Name: "Root.md", Path: "/library/Root.md", Depth: 0},
		},
	}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	rootFile := mustAccess(t, app, ui.RoleTreeItem, "Root.md")
	if _, ok := app.AccessByRole(ui.RoleTreeItem, "A.md"); ok {
		t.Fatal("nested file A.md should stay hidden until Essays is drilled into")
	}
	if err := app.Click("Essays"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	nestedFile := mustAccess(t, app, ui.RoleTreeItem, "A.md")
	if nestedFile.Bounds.X != rootFile.Bounds.X {
		t.Fatalf("drilled file A.md X = %v, want zero-indent root X = %v", nestedFile.Bounds.X, rootFile.Bounds.X)
	}
	if err := app.Click("Go to Library"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleTreeItem, "Root.md"); !ok {
		t.Fatal("clicking root ancestor did not return to workspace root")
	}
}

func TestSidebarFooterSwitchesWuXingSpaceAndAppearance(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	var shell nativeui.Shell
	model := libraryModel()
	var spaces []string
	var appearances []portrait.Appearance
	actions := nativeui.Actions{
		SetSpace: func(name string) {
			spaces = append(spaces, name)
			model.Spaces = map[string]string{model.Workspace.Root: name}
		},
		SetAppearance: func(a portrait.Appearance) { appearances = append(appearances, a); model.Appearance = a },
	}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, actions, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	if err := app.Click("Space: Wood–Fire"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Switch to Dark Mode"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !slices.Equal(spaces, []string{"WoodFire"}) {
		t.Fatalf("footer palette stored %v, want [WoodFire]", spaces)
	}
	if !slices.Equal(appearances, []portrait.Appearance{portrait.Dark}) {
		t.Fatalf("footer appearance toggle stored %v, want [Dark]", appearances)
	}
}

func TestSettingsDialogSwitchesSpaceAppearanceAndFeynmanTabs(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	var shell nativeui.Shell
	model := libraryModel()
	var seenTabs []string
	actions := nativeui.Actions{
		SetSpace:      func(name string) { model.Spaces = map[string]string{model.Workspace.Root: name} },
		SetAppearance: func(a portrait.Appearance) { model.Appearance = a },
	}
	slots := nativeui.Slots{
		FeynmanSettings: func(c *ui.Context, tab string) {
			seenTabs = append(seenTabs, tab)
			portrait.Type.UI.Apply(ui.Text(c, "Feynman tab: "+tab))
		},
	}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, actions, slots)
	}, 1280, 820)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyComma)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Settings"); !ok {
		t.Fatal("Command-, did not open the dedicated Settings modal")
	}
	if err := app.Click("Models & Auth"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Feynman tab: models") {
		t.Fatal("Models & Auth did not render Feynman settings slot")
	}
	if err := app.Click("Tools & MCP"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Feynman tab: tools") {
		t.Fatal("Tools & MCP did not render Feynman settings slot")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Settings"); ok {
		t.Fatal("Escape left Settings modal open")
	}
}
