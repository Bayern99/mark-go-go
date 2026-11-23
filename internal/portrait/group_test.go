package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestGroupConnectsActionButtonsWithoutSegmentedSemantics(t *testing.T) {
	clicked := ""
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Group(c, "Hunk actions", func(g *portrait.GroupBuilder) {
			for _, action := range []string{"Accept", "Reject", "Clear"} {
				if g.Add(portrait.QuietButton(c, action, action+" hunk")).Clicked() {
					clicked = action
				}
			}
		})
	}), 320, 160)
	tt.Frame()
	group, ok := tt.AccessByRole(ui.RoleGroup, "Hunk actions")
	if !ok {
		t.Fatal("action group missing")
	}
	for _, access := range tt.AccessAll() {
		if access.Role == ui.RoleTabList {
			t.Fatalf("action group acquired segmented-control semantics: %+v", access)
		}
	}
	accept, _ := tt.AccessByRole(ui.RoleButton, "Accept hunk")
	reject, _ := tt.AccessByRole(ui.RoleButton, "Reject hunk")
	clear, _ := tt.AccessByRole(ui.RoleButton, "Clear hunk")
	if gap := reject.Bounds.X - (accept.Bounds.X + accept.Bounds.W); gap < -0.001 || gap > 0.001 {
		t.Fatalf("gap between Accept and Reject = %v, want a connected group", gap)
	}
	if gap := clear.Bounds.X - (reject.Bounds.X + reject.Bounds.W); gap < -0.001 || gap > 0.001 {
		t.Fatalf("gap between Reject and Clear = %v, want a connected group", gap)
	}
	chrome := luma(tt, 10, 10)
	if corner := luma(tt, int(group.Bounds.X), int(group.Bounds.Y)); corner != chrome {
		t.Fatalf("outer corner %d is not clipped to the chrome %d", corner, chrome)
	}
	if edge := luma(tt, int(group.Bounds.X+group.Bounds.W/2), int(group.Bounds.Y)); edge >= chrome {
		t.Fatalf("group has no shared top border: edge %d, chrome %d", edge, chrome)
	}
	if err := tt.Click("Reject hunk"); err != nil {
		t.Fatal(err)
	}
	if clicked != "Reject" {
		t.Fatalf("clicked %q, want Reject", clicked)
	}
}

func TestGroupSupportsASingleAction(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Group(c, "Document actions", func(g *portrait.GroupBuilder) {
			g.Add(portrait.Button(c, "Save", "Save document"))
		})
	}), 240, 160)
	tt.Frame()
	group, groupOK := tt.AccessByRole(ui.RoleGroup, "Document actions")
	button, buttonOK := tt.AccessByRole(ui.RoleButton, "Save document")
	if !groupOK || !buttonOK || button.Bounds.H != portrait.Sizes.Regular {
		t.Fatalf("single-action group %+v, button %+v", group, button)
	}
}
