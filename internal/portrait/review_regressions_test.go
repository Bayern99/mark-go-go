package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestGroupDoesNotAccumulateHandlesAcrossFrames(t *testing.T) {
	app := ui.NewTester(func(c *ui.Context) {
		portrait.Group(c, "Review actions", func(g *portrait.GroupBuilder) {
			g.Add(portrait.Button(c, "Accept", "Accept"))
			g.Add(portrait.Button(c, "Reject", "Reject"))
		})
	}, 400, 120)
	for i := 0; i < 100; i++ {
		app.Frame()
	}
	if err := app.Click("Accept"); err != nil {
		t.Fatalf("Accept not clickable after 100 frames: %v", err)
	}
	if err := app.Click("Reject"); err != nil {
		t.Fatalf("Reject not clickable after 100 frames: %v", err)
	}
}

func TestConfirmDoesNotStealFocusLaterWhenTriggerDisappeared(t *testing.T) {
	var trigger, other ui.Handle
	showTrigger := true
	open := false
	app := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Gap(8).Children(func() {
			if showTrigger {
				if portrait.Button(c, "Delete", "Delete").Bind(&trigger).Clicked() {
					open = true
				}
			}
			portrait.Button(c, "Keep", "Keep").Bind(&other)
		})
		portrait.Confirm(c, &open, portrait.ConfirmSpec{
			Title: "Delete item?", Body: "This cannot be undone.",
			Return: &trigger,
		})
	}, 520, 320)

	app.Frame()
	if err := app.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !open {
		t.Fatal("confirm dialog did not open")
	}
	// Trigger disappears while dialog is open.
	showTrigger = false
	app.Frame()
	// Dismiss via Escape.
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if open {
		t.Fatal("confirm dialog did not close on Escape")
	}
	// Move focus to another control.
	if err := app.Click("Keep"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	// Trigger reappears later: must NOT steal focus from Keep.
	showTrigger = true
	app.Frame()
	keep, ok := app.AccessByRole(ui.RoleButton, "Keep")
	if !ok || !keep.Focused {
		t.Fatalf("Keep lost focus when vanished trigger reappeared: %+v", keep)
	}
}

func TestConfirmDoesNotStealFocusLaterWhenTriggerWasDisabled(t *testing.T) {
	var trigger, other ui.Handle
	disableTrigger := false
	open := false
	app := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Gap(8).Children(func() {
			del := portrait.Button(c, "Delete", "Delete").Bind(&trigger).Disabled(disableTrigger)
			if del.Clicked() {
				open = true
			}
			portrait.Button(c, "Keep", "Keep").Bind(&other)
		})
		portrait.Confirm(c, &open, portrait.ConfirmSpec{
			Title: "Delete item?", Body: "This cannot be undone.",
			Return: &trigger,
		})
	}, 520, 320)

	app.Frame()
	if err := app.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !open {
		t.Fatal("confirm dialog did not open")
	}
	// Trigger stays bound but disabled while dialog closes.
	disableTrigger = true
	app.Frame()
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if open {
		t.Fatal("confirm dialog did not close on Escape")
	}
	// Focus another control.
	if err := app.Click("Keep"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	// Re-enabling trigger later must NOT steal focus from Keep.
	disableTrigger = false
	app.Frame()
	keep, ok := app.AccessByRole(ui.RoleButton, "Keep")
	if !ok || !keep.Focused {
		t.Fatalf("Keep lost focus when previously disabled trigger was re-enabled: %+v", keep)
	}
}
