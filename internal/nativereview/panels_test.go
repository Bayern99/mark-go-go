package nativereview

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func TestProposalControlsKeepAccessibilityNames(t *testing.T) {
	p := &panel{win: &nativeui.Window{}, latest: review.ReviewUpdateEvent{Proposals: []review.ChangeProposal{{ID: "p1", Path: "Quiet.md", Summary: "Tighten the introduction"}}}}
	app := ui.NewTester(func(c *ui.Context) { p.drawProposals(c) }, 800, 600)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Change proposals"); !ok {
		t.Fatal("proposal trigger lost its accessible name")
	}
	if err := app.Click("Change proposals"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	for _, name := range []string{"Quiet.md", "Open Quiet.md"} {
		if _, ok := app.Access(name); !ok {
			t.Errorf("missing proposal control %q", name)
		}
	}
}
