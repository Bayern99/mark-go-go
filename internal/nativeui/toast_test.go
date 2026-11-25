package nativeui

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestToastActionIsAboveDrawerAndPopover(t *testing.T) {
	var shell Shell
	model := Model{Git: true, Workspace: workspace.WorkspaceSnapshot{
		ActiveID:  "a",
		Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview"}},
	}}
	var pending *portrait.ToastSpec
	app := ui.NewTester(func(ctx *ui.Context) {
		if pending != nil {
			portrait.Notify(ctx, *pending)
			pending = nil
		}
		shell.Draw(ctx, model, Actions{}, Slots{Checkpoints: func(c *ui.Context) {
			ui.Text(c, "Checkpoint history").Padding(8)
		}})
	}, 480, 360)
	shell.leftOpen = true
	shell.checkpoints = true
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleDialog, "Navigation"); !ok {
		t.Fatal("navigation drawer is not open")
	}
	if !app.HasText("Checkpoint history") {
		t.Fatal("checkpoint popover is not open")
	}
	called := 0
	pending = &portrait.ToastSpec{
		ID: "restore:a", Tone: portrait.Tones.Success, Title: "Restored", Action: "Undo", Timeout: -1,
		OnAction: func() { called++ },
	}
	app.Frame()
	app.Advance(portrait.Motion.Enter)
	if err := app.Click("Undo"); err != nil {
		t.Fatalf("toast action was not clickable above drawer and popover: %v", err)
	}
	if called != 1 {
		t.Fatalf("toast action called %d times, want once", called)
	}
}

func TestAllSevenAlertsUseWarningForDiskState(t *testing.T) {
	const conflict = "The file changed on disk."
	const deleted = "The file was deleted on disk. Edits are still available in this window."
	var shell Shell
	model := Model{
		Message: "Window action failed",
		Workspace: workspace.WorkspaceSnapshot{
			Kind: "folder", Root: "/notes", Name: "Notes", ActiveID: "a", RestoreError: "Session restore failed",
			Documents: []workspace.DocumentInfo{{
				ID: "a", Name: "Paper.md", Path: "/notes/Paper.md", ViewMode: "preview",
				ActionError: "Document action failed", RestoreError: "Document restore failed", Conflict: true, Deleted: true,
			}},
		},
		Navigation: workspace.NavSnapshot{Error: "Index failed"},
	}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, Actions{}, Slots{}) }, 1280, 820)
	app.Frame()
	alerts := 0
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleAlert {
			alerts++
		}
	}
	if alerts != 7 {
		t.Fatalf("found %d alerts, want all seven banner replacements", alerts)
	}
	warning, ok := app.AccessByRole(ui.RoleAlert, conflict)
	if !ok {
		t.Fatal("disk-conflict alert is missing")
	}
	if _, ok := app.AccessByRole(ui.RoleAlert, deleted); !ok {
		t.Fatal("disk-deletion alert is missing")
	}
	danger, ok := app.AccessByRole(ui.RoleAlert, "Document action failed")
	if !ok {
		t.Fatal("action-error alert is missing")
	}
	img := app.Image()
	warningBorder := img.RGBAAt(int(warning.Bounds.X+warning.Bounds.W/2), int(warning.Bounds.Y))
	dangerBorder := img.RGBAAt(int(danger.Bounds.X+danger.Bounds.W/2), int(danger.Bounds.Y))
	if warningBorder == dangerBorder {
		t.Fatal("disk warning and true error still use the same tone")
	}
}

func TestGenerationAndCopyFeedback(t *testing.T) {
	var shell Shell
	model := Model{Workspace: workspace.WorkspaceSnapshot{
		ActiveID:  "a",
		Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview", Generating: true}},
	}}
	text := "selected"
	var pending *portrait.ToastSpec
	app := ui.NewTester(func(c *ui.Context) {
		if pending != nil {
			portrait.Notify(c, *pending)
			pending = nil
		}
		shell.Draw(c, model, Actions{}, Slots{Document: func(c *ui.Context) {
			ui.TextInputBase(c, &text).Label("Document source").AutoFocus()
		}})
	}, 1280, 820)
	pending = &portrait.ToastSpec{ID: "generation:a", Tone: portrait.Tones.Info, Title: "Generating…", Timeout: -1}
	app.Frame()
	if !app.HasText("Generating…") {
		t.Fatal("generation did not show its persistent progress toast")
	}
	// Clearing Generating without an explicit completion notification must not fabricate success.
	model.Workspace.Documents[0].Generating = false
	app.Frame()
	if app.HasText("Generated") {
		t.Fatalf("clearing Generating blindly reported Generated: %q", app.Texts())
	}
	pending = &portrait.ToastSpec{ID: "generation:a", Tone: portrait.Tones.Danger, Title: "Generation failed"}
	app.Frame()
	if app.HasText("Generating…") || !app.HasText("Generation failed") {
		t.Fatalf("generation toast did not update in place on failure: %q", app.Texts())
	}
	// Empty-selection copy command must not fabricate a Copied toast.
	if err := app.Click("Document source"); err != nil {
		t.Fatal(err)
	}
	app.Command("copy")
	app.Frame()
	app.Advance(portrait.Motion.Enter)
	if app.HasText("Copied") {
		t.Fatal("copy command with no selection fabricated Copied toast")
	}
}
