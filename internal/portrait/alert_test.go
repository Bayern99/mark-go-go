package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestAlertTonesHaveAccessibleStructureAndDistinctPaint(t *testing.T) {
	for _, tc := range []struct {
		name string
		tone portrait.Tone
	}{
		{"neutral", portrait.Tones.Neutral},
		{"accent", portrait.Tones.Accent},
		{"info", portrait.Tones.Info},
		{"warning", portrait.Tones.Warning},
		{"danger", portrait.Tones.Danger},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := ui.NewTester(onChrome(func(c *ui.Context) {
				portrait.Alert(c, tc.tone, "File changed", "Choose which version to keep.")
			}), 420, 180)
			tt.Frame()
			alert, ok := tt.AccessByRole(ui.RoleAlert, "File changed")
			if !ok {
				t.Fatal("alert is missing its role or accessible name")
			}
			if !tt.HasText("Choose which version to keep.") {
				t.Fatal("alert body is not visible")
			}
			b := alert.Bounds
			img := tt.Image()
			chrome := img.RGBAAt(1, 1)
			wash := img.RGBAAt(int(b.X+b.W)-5, int(b.Y+b.H/2))
			border := img.RGBAAt(int(b.X+b.W/2), int(b.Y))
			if wash == chrome {
				t.Fatal("alert does not paint its tone wash")
			}
			if border == chrome || border == wash {
				t.Fatalf("alert border is not distinct: border %v, wash %v, chrome %v", border, wash, chrome)
			}
		})
	}
}

func TestAlertAction(t *testing.T) {
	called := 0
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Alert(c, portrait.Tones.Danger, "Save failed", "The disk is full.", portrait.AlertAction{
			Label: "Retry",
			OnAction: func() {
				called++
			},
		})
	}), 420, 180)
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("action called %d times, want once", called)
	}
	if _, ok := tt.AccessByRole(ui.RoleAlert, "Save failed"); !ok {
		t.Fatal("adding an action removed the alert semantics")
	}
}

func TestAlertKeepsTitleAndBodyInProportion(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Alert(c, portrait.Tones.Neutral, "File changed", "Choose which version to keep.")
	}), 420, 180)
	tt.Frame()
	body, ok := tt.Find("Choose which version to keep.")
	if !ok {
		t.Fatal("body is missing")
	}
	// A 14/22 body under a 13/18 title read upside down: both lines belong to
	// the UI style so the title's weight, not its size, leads.
	if body.H != portrait.Type.UI.Leading {
		t.Fatalf("body leading %v, want the title's %v", body.H, portrait.Type.UI.Leading)
	}
	alert, _ := tt.AccessByRole(ui.RoleAlert, "File changed")
	if body.Y <= alert.Bounds.Y+alert.Bounds.H/2 {
		t.Fatalf("body at %v is not below the title inside %+v", body.Y, alert.Bounds)
	}
}
