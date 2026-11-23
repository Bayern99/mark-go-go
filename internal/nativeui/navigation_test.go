package nativeui_test

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
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
