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

func TestWorkspaceMenuDiscardsCloseWhenSelectionChanges(t *testing.T) {
	a := workspace.WorkspaceSummary{ID: "workspace-a", Root: "/a", Name: "A", Kind: "folder"}
	b := workspace.WorkspaceSummary{ID: "workspace-b", Root: "/b", Name: "B", Kind: "folder"}
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Root: b.Root, Name: b.Name, Kind: b.Kind, Workspaces: []workspace.WorkspaceSummary{a, b},
	}}
	var shell nativeui.Shell
	var closed []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{CloseWorkspace: func(id string) { closed = append(closed, id) }}, nativeui.Slots{})
	}, 1280, 820)
	if err := app.Click("Switch workspace"); err != nil {
		t.Fatal(err)
	}
	model.Workspace.Root, model.Workspace.Name = a.Root, a.Name
	app.Frame()
	if err := app.ChooseMenuItem("Close Workspace"); err != nil {
		t.Fatal(err)
	}
	if len(closed) > 0 {
		t.Fatalf("a choice from B's menu closed %v after A became selected", closed)
	}
	if err := app.Click("Switch workspace"); err != nil {
		t.Fatal(err)
	}
	if err := app.ChooseMenuItem("Close Workspace"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(closed, []string{a.ID}) {
		t.Fatalf("closing A from its own menu closed %v", closed)
	}
}

func TestWorkspaceMenuKeepsSwitchTargetWhenEarlierRootDisappears(t *testing.T) {
	projects := workspace.WorkspaceSummary{ID: "projects", Root: "/Projects", Name: "Projects", Kind: "folder"}
	firstNotes := workspace.WorkspaceSummary{ID: "first-notes", Root: "/a/Notes", Name: "Notes", Kind: "folder"}
	secondNotes := workspace.WorkspaceSummary{ID: "second-notes", Root: "/b/Notes", Name: "Notes", Kind: "folder"}
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Root: secondNotes.Root, Name: secondNotes.Name, Kind: secondNotes.Kind,
		Workspaces: []workspace.WorkspaceSummary{projects, firstNotes, secondNotes},
	}}
	var shell nativeui.Shell
	var activated []string
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{ActivateWorkspace: func(id string) { activated = append(activated, id) }}, nativeui.Slots{})
	}, 1280, 820)
	if err := app.Click("Switch workspace"); err != nil {
		t.Fatal(err)
	}
	model.Workspace.Workspaces = model.Workspace.Workspaces[1:]
	app.Frame()
	if err := app.ChooseMenuItem("Notes"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(activated, []string{firstNotes.ID}) {
		t.Fatalf("choosing the first Notes workspace activated %v", activated)
	}
}

func fileTreeMotionShell(prefs ui.Preferences) (*ui.Tester, *nativeui.Model) {
	m := &nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Kind: "folder", Root: "/notes", ActiveID: "one",
		Documents: []workspace.DocumentInfo{
			{ID: "one", Name: "One.md", Path: "/notes/One.md"},
			{ID: "two", Name: "Two.md", Path: "/notes/Two.md"},
		},
	}, Navigation: workspace.NavSnapshot{Entries: []workspace.NavEntry{
		{ID: "file:One.md", Kind: "file", Name: "One.md", Path: "/notes/One.md", DocumentID: "one"},
		{ID: "file:Two.md", Kind: "file", Name: "Two.md", Path: "/notes/Two.md", DocumentID: "two"},
	}}}
	var shell nativeui.Shell
	app := ui.NewTesterWith(func(c *ui.Context) {
		shell.Draw(c, *m, nativeui.Actions{Activate: func(id string) { m.Workspace.ActiveID = id }}, nativeui.Slots{})
	}, 1280, 820, ui.TesterOptions{Preferences: prefs})
	app.Frame()
	return app, m
}

func fileTreePixels(t *testing.T, app *ui.Tester) []uint32 {
	t.Helper()
	tree, ok := app.AccessByRole(ui.RoleTree, "Workspace files")
	if !ok {
		t.Fatal("workspace tree is missing")
	}
	return pixels(app.Image(), tree.Bounds)
}

func checkFileTreeSelection(t *testing.T, app *ui.Tester, name string) {
	t.Helper()
	row, ok := app.AccessByRole(ui.RoleTreeItem, name)
	if !ok || !row.Selected || row.Name != name {
		t.Fatalf("selected tree row = %+v, %v; want %q selected with its name intact", row, ok, name)
	}
}

func TestFileTreeCapsuleMovesVerticallyThenSettles(t *testing.T) {
	app, m := fileTreeMotionShell(ui.Preferences{})
	from := fileTreePixels(t, app)
	if err := app.Click("Two.md"); err != nil {
		t.Fatal(err)
	}
	if m.Workspace.ActiveID != "two" {
		t.Fatalf("click activated %q, want two", m.Workspace.ActiveID)
	}
	app.Advance(portrait.Motion.Switch / 3)
	mid := fileTreePixels(t, app)
	app.Advance(time.Second)
	to := fileTreePixels(t, app)
	if changed(from, mid) == 0 || changed(mid, to) == 0 {
		t.Fatal("selection capsule did not pass through an intermediate vertical position")
	}
	app.Advance(time.Second)
	if n := changed(to, fileTreePixels(t, app)); n != 0 {
		t.Fatalf("selection capsule kept moving after settling: %d pixels changed", n)
	}
	checkFileTreeSelection(t, app, "Two.md")
}

func TestFileTreeCapsuleLandsAtOnceForKeyboardOrReducedMotion(t *testing.T) {
	t.Run("keyboard", func(t *testing.T) {
		app, m := fileTreeMotionShell(ui.Preferences{})
		focus(t, app, "One.md")
		app.Key(0, ui.KeyDown)
		if !app.Focused("Two.md") {
			t.Fatal("Down did not move through the workspace tree")
		}
		app.Key(0, ui.KeyEnter)
		app.Frame()
		if m.Workspace.ActiveID != "two" {
			t.Fatalf("Enter activated %q, want two", m.Workspace.ActiveID)
		}
		landed := fileTreePixels(t, app)
		app.Advance(time.Second)
		if n := changed(landed, fileTreePixels(t, app)); n != 0 {
			t.Fatalf("keyboard selection capsule still moving: %d pixels changed", n)
		}
		checkFileTreeSelection(t, app, "Two.md")
	})
	t.Run("reduced motion", func(t *testing.T) {
		app, m := fileTreeMotionShell(ui.Preferences{ReduceMotion: true})
		if err := app.Click("Two.md"); err != nil {
			t.Fatal(err)
		}
		if m.Workspace.ActiveID != "two" {
			t.Fatalf("click activated %q, want two", m.Workspace.ActiveID)
		}
		landed := fileTreePixels(t, app)
		app.Advance(time.Second)
		if n := changed(landed, fileTreePixels(t, app)); n != 0 {
			t.Fatalf("reduced-motion selection capsule still moving: %d pixels changed", n)
		}
		checkFileTreeSelection(t, app, "Two.md")
	})
}

func TestFileTreeReopensPathAfterDocumentCloses(t *testing.T) {
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Kind:     "folder",
		Root:     "/notes",
		ActiveID: "welcome",
		Documents: []workspace.DocumentInfo{
			{ID: "welcome", Name: "Welcome.md", Path: "/notes/Welcome.md"},
		},
	}, Navigation: workspace.NavSnapshot{Entries: []workspace.NavEntry{
		{ID: "file:Discussion.md", Kind: "file", Name: "Discussion.md", Path: "/notes/Discussion.md", DocumentID: "closed"},
		{ID: "file:Welcome.md", Kind: "file", Name: "Welcome.md", Path: "/notes/Welcome.md", DocumentID: "welcome"},
	}}}
	var activated, opened string
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{
			Activate: func(id string) { activated = id },
			OpenPath: func(path string, _ int) { opened = path },
		}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	if err := app.Click("Discussion.md"); err != nil {
		t.Fatal(err)
	}
	if activated != "" {
		t.Fatalf("activated closed document %q", activated)
	}
	if opened != "/notes/Discussion.md" {
		t.Fatalf("opened path = %q, want Discussion.md", opened)
	}
}

func TestFileTreeActivatesOpenDocumentDeletedFromDisk(t *testing.T) {
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{
		Kind:     "folder",
		Root:     "/notes",
		ActiveID: "welcome",
		Documents: []workspace.DocumentInfo{
			{ID: "welcome", Name: "Welcome.md", Path: "/notes/Welcome.md"},
			{ID: "deleted", Name: "Deleted.md", Path: "/notes/Deleted.md", Deleted: true},
		},
	}, Navigation: workspace.NavSnapshot{Entries: []workspace.NavEntry{
		{ID: "file:Deleted.md", Kind: "file", Name: "Deleted.md", Path: "/notes/Deleted.md", DocumentID: "deleted", Deleted: true},
	}}}
	var activated, opened string
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{
			Activate: func(id string) { activated = id },
			OpenPath: func(path string, _ int) { opened = path },
		}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	if err := app.Click("Deleted.md"); err != nil {
		t.Fatal(err)
	}
	if activated != "deleted" || opened != "" {
		t.Fatalf("activated = %q, opened = %q; want the existing deleted document", activated, opened)
	}
}
