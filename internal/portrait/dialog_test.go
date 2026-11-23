package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const confirmTriggerLabel = "Delete conversation"

type confirmFixture struct {
	open        bool
	confirmed   int
	returnFocus ui.Handle
}

func (f *confirmFixture) view(c *ui.Context) {
	ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(24).Children(func() {
		trigger := portrait.Button(c, confirmTriggerLabel, confirmTriggerLabel).Bind(&f.returnFocus)
		if trigger.Clicked() {
			f.open = true
		}
	})
	portrait.Confirm(c, &f.open, portrait.ConfirmSpec{
		Title:        "Delete this conversation?",
		Body:         "This action cannot be undone.",
		ConfirmLabel: "Delete",
		CancelLabel:  "Cancel",
		Return:       &f.returnFocus,
		OnConfirm:    func() { f.confirmed++ },
	})
}

func openConfirm(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(confirmTriggerLabel); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("Cancel") {
		t.Fatal("opening Confirm did not focus Cancel")
	}
}

func assertConfirmClosed(t *testing.T, tt *ui.Tester, f *confirmFixture, confirmed int) {
	t.Helper()
	if f.open {
		t.Fatal("dialog is still open")
	}
	if f.confirmed != confirmed {
		t.Fatalf("OnConfirm called %d times, want %d", f.confirmed, confirmed)
	}
	if _, ok := tt.AccessByRole(ui.RoleAlertDialog, "Delete this conversation?"); ok {
		t.Fatal("closed dialog remains in the accessibility tree")
	}
	if !tt.Focused(confirmTriggerLabel) {
		t.Fatal("focus was not returned to the dialog trigger")
	}
}

func TestConfirmDefaultsToCancelAndEnterCancels(t *testing.T) {
	f := &confirmFixture{}
	tt := ui.NewTester(f.view, 600, 400)
	tt.Frame()
	openConfirm(t, tt)
	cancel, _ := tt.AccessByRole(ui.RoleButton, "Cancel")
	confirm, _ := tt.AccessByRole(ui.RoleButton, "Delete")
	if !cancel.Focused || confirm.Focused {
		t.Fatalf("Cancel focused = %v, Delete focused = %v", cancel.Focused, confirm.Focused)
	}
	tt.Key(0, ui.KeyEnter)
	assertConfirmClosed(t, tt, f, 0)
}

func TestConfirmEscapeAndBackdropAreCancel(t *testing.T) {
	for _, close := range []string{"escape", "backdrop"} {
		t.Run(close, func(t *testing.T) {
			f := &confirmFixture{}
			tt := ui.NewTester(f.view, 600, 400)
			tt.Frame()
			openConfirm(t, tt)
			if close == "escape" {
				tt.Key(0, ui.KeyEscape)
			} else {
				tt.ClickAt(5, 5)
			}
			assertConfirmClosed(t, tt, f, 0)
		})
	}
}

func TestConfirmButtonsCloseAndOnlyConfirmRunsTheAction(t *testing.T) {
	for _, button := range []string{"Cancel", "Delete"} {
		t.Run(button, func(t *testing.T) {
			f := &confirmFixture{}
			tt := ui.NewTester(f.view, 600, 400)
			tt.Frame()
			openConfirm(t, tt)
			if err := tt.Click(button); err != nil {
				t.Fatal(err)
			}
			want := 0
			if button == "Delete" {
				want = 1
			}
			assertConfirmClosed(t, tt, f, want)
		})
	}
}

func TestConfirmReturnsFocusWhenCallerClosesIt(t *testing.T) {
	f := &confirmFixture{}
	tt := ui.NewTester(f.view, 600, 400)
	tt.Frame()
	openConfirm(t, tt)
	f.open = false
	tt.Frame()
	assertConfirmClosed(t, tt, f, 0)
}

func TestConfirmAccessibilityGeometryAndScrim(t *testing.T) {
	for _, tc := range []struct {
		width     int
		wantPanel float32
	}{{600, 420}, {300, 252}} {
		f := &confirmFixture{}
		tt := ui.NewTester(f.view, tc.width, 400)
		tt.Frame()
		before := tt.Image().RGBAAt(5, 5)
		openConfirm(t, tt)
		dialog, ok := tt.AccessByRole(ui.RoleAlertDialog, "Delete this conversation?")
		if !ok || dialog.Description != "This action cannot be undone." {
			t.Fatalf("dialog accessibility = %+v, %v", dialog, ok)
		}
		if dialog.Bounds.W != tc.wantPanel {
			t.Fatalf("window %d: panel width %v, want %v", tc.width, dialog.Bounds.W, tc.wantPanel)
		}
		after := tt.Image().RGBAAt(5, 5)
		if before.R == after.R || after.R < 220 || after.R > 235 {
			t.Fatalf("scrim changed the paper from %v to %v, want an unblurred 12%% dim", before, after)
		}
	}
}

func TestConfirmUsesRightAlignedCancelThenDangerButton(t *testing.T) {
	f := &confirmFixture{}
	tt := ui.NewTester(f.view, 600, 400)
	tt.Frame()
	openConfirm(t, tt)
	tt.Advance(time.Second)
	dialog, _ := tt.AccessByRole(ui.RoleAlertDialog, "Delete this conversation?")
	cancel, _ := tt.AccessByRole(ui.RoleButton, "Cancel")
	confirm, _ := tt.AccessByRole(ui.RoleButton, "Delete")
	if cancel.Bounds.X >= confirm.Bounds.X {
		t.Fatalf("Cancel at %+v is not before Delete at %+v", cancel.Bounds, confirm.Bounds)
	}
	if got, want := confirm.Bounds.X+confirm.Bounds.W, dialog.Bounds.X+dialog.Bounds.W-20; got != want {
		t.Fatalf("Delete right edge %v, want panel content edge %v", got, want)
	}
	px := tt.Image().RGBAAt(int(confirm.Bounds.X+4), int(confirm.Bounds.Y+confirm.Bounds.H/2))
	want := portrait.Tones.Danger.Solid
	if px.R != want.R || px.G != want.G || px.B != want.B {
		t.Fatalf("Delete fill = #%02x%02x%02x, want Danger #%02x%02x%02x", px.R, px.G, px.B, want.R, want.G, want.B)
	}
}

func TestConfirmKeyboardOpeningSkipsPopupTransition(t *testing.T) {
	f := &confirmFixture{}
	tt := ui.NewTester(f.view, 600, 400)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	if !tt.Focused(confirmTriggerLabel) {
		t.Fatal("Tab did not focus the trigger")
	}
	tt.Key(0, ui.KeyEnter)
	if !f.open || !tt.Focused("Cancel") {
		t.Fatal("Enter did not open Confirm with Cancel focused")
	}
	if tt.FramePending() {
		t.Fatal("keyboard-triggered Confirm scheduled a popup transition")
	}
}
