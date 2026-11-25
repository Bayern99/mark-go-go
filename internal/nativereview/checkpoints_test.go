package nativereview

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func checkpointLayoutTester(t *testing.T, withError bool) *ui.Tester {
	t.Helper()
	const root = "/workspace"
	path := root + "/Quiet Software.md"
	proposal := review.ChangeProposal{ID: "p1", Revision: 1, WorkspaceRoot: root, DocumentID: "a", Path: path}
	win := nativeui.SceneWindow(workspace.WorkspaceSnapshot{
		Root: root, ActiveID: "a", Documents: []workspace.DocumentInfo{{WorkspaceRoot: root, ID: "a", Path: path}},
	}, workspace.NavSnapshot{}, nil, "current text\n", portrait.DefaultSpace, false)
	scene := NewScene(win, SceneState{
		Compared: proposal, Hunks: fixtureHunks(), Checkpoints: []review.Checkpoint{{ID: "c1", Label: "Before edits"}},
	})
	if withError {
		scene.p.checkpointDocs["a"].errorText = "Could not refresh checkpoints"
	}
	app := ui.NewTester(func(c *ui.Context) { scene.Checkpoints(c) }, 520, 720)
	app.Frame()
	return app
}

func TestCheckpointComposerAndActionsUseCompactAlignedLayout(t *testing.T) {
	app := checkpointLayoutTester(t, true)
	field, _ := app.AccessByRole(ui.RoleTextField, "Checkpoint label")
	create, _ := app.AccessByRole(ui.RoleButton, "Create checkpoint")
	if create.Bounds.H != portrait.Form.FieldCompact {
		t.Fatalf("Create checkpoint height = %v, want field height %v", create.Bounds.H, portrait.Form.FieldCompact)
	}
	if !near(field.Bounds.Y+field.Bounds.H/2, create.Bounds.Y+create.Bounds.H/2) {
		t.Fatalf("checkpoint field %v and Create button %v are not centered together", field.Bounds, create.Bounds)
	}

	compare, _ := app.AccessByRole(ui.RoleButton, "Compare the selected checkpoint")
	restore, _ := app.AccessByRole(ui.RoleButton, "Restore shown version")
	refresh, _ := app.AccessByRole(ui.RoleButton, "Refresh checkpoints")
	for name, info := range map[string]ui.AccessInfo{"Compare": compare, "Restore": restore, "Refresh": refresh} {
		if info.Bounds.W > portrait.Form.PopoverWide/2 {
			t.Errorf("%s stretches to %v instead of using its intrinsic width", name, info.Bounds.W)
		}
		if !near(info.Bounds.X, compare.Bounds.X) {
			t.Errorf("%s starts at %v, want shared leading edge %v", name, info.Bounds.X, compare.Bounds.X)
		}
	}
}

func TestCheckpointEmptyUsesPortraitEmpty(t *testing.T) {
	const (
		title       = "这份文档还没有任何历史检查点"
		description = "创建检查点后，可以在这里比较或恢复先前保存的版本。"
	)
	for _, preference := range []ui.Preferences{{}, {HighContrast: true}, {ReduceTransparency: true}} {
		tt := ui.NewTesterWith(func(c *ui.Context) {
			checkpointEmpty(c, title, description).Width(220)
		}, 260, 180, ui.TesterOptions{Preferences: preference})
		tt.Frame()
		info, ok := tt.AccessByRole(ui.RoleGroup, title)
		if !ok || info.Description != description || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
			t.Fatalf("checkpoint empty state with %+v: %+v, found %v", preference, info, ok)
		}
		if bounds, found := tt.Find(description); !found || bounds.W > 220 || bounds.H <= 16 {
			t.Fatalf("checkpoint empty description did not wrap with %+v: %+v, found %v", preference, bounds, found)
		}
	}
}

func TestCheckpointRowsKeepTheirFocusRingInsideTheList(t *testing.T) {
	app := checkpointLayoutTester(t, false)
	list, ok := app.Access("Saved checkpoints")
	if !ok {
		t.Fatalf("the checkpoint list has no accessible name: %q", app.Texts())
	}
	name := find(t, app, "Before edits")
	if inset := name.X - list.Bounds.X; inset < 2.5 {
		t.Fatalf("the first checkpoint row starts %.1f inside its list %v; a focused row's ring would clip", inset, list.Bounds)
	}
	if inset := name.Y - list.Bounds.Y; inset < 2.5 {
		t.Fatalf("the first checkpoint row starts %.1f from the list top %v; a focused row's ring would clip", inset, list.Bounds)
	}
}
