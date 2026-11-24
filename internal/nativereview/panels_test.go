package nativereview

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
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
	popup, _ := app.AccessByRole(ui.RolePopup, "")
	open, _ := app.AccessByRole(ui.RoleButton, "Open Quiet.md")
	if open.Bounds.W > popup.Bounds.W/2 {
		t.Fatalf("Open action stretches to %v in a %v-wide popover", open.Bounds.W, popup.Bounds.W)
	}
	if got, want := open.Bounds.X, popup.Bounds.X+portrait.Form.PopoverPad+portrait.Form.PadX; !near(got, want) {
		t.Fatalf("Open action starts at %v, want content start %v", got, want)
	}
}

func proposalTester(t *testing.T, width, height int, compared bool, summary string) *ui.Tester {
	t.Helper()
	const root = "/workspace"
	path := root + "/Quiet Software.md"
	proposal := review.ChangeProposal{
		ID: "p1", Revision: 1, WorkspaceRoot: root, DocumentID: "a", Path: path,
		Summary: summary, Source: "Feynman Research", Hunks: fixtureHunks(),
	}
	proposals := []review.ChangeProposal{proposal, {
		ID: "p2", Revision: 1, WorkspaceRoot: root, Path: root + "/Reading List.md",
	}}
	win := nativeui.SceneWindow(workspace.WorkspaceSnapshot{
		Root: root, ActiveID: "a", Documents: []workspace.DocumentInfo{{WorkspaceRoot: root, ID: "a", Path: path}},
	}, workspace.NavSnapshot{}, nil, "current text\n", portrait.DefaultSpace, false)
	state := SceneState{Proposals: review.ReviewUpdateEvent{WorkspaceRoot: root, Proposals: proposals}}
	if compared {
		state.Compared = proposal
	}
	scene := NewScene(win, state)
	scene.OpenProposals()
	app := ui.NewTester(func(c *ui.Context) { scene.Proposals(c) }, width, height)
	app.Frame()
	return app
}

func near(a, b float32) bool {
	delta := a - b
	return delta >= -1 && delta <= 1
}

func TestProposalPopoverUsesResponsiveWidthAndMaxHeight(t *testing.T) {
	for _, width := range []int{400, 800} {
		app := proposalTester(t, width, 640, false, "Tighten the introduction")
		popup, ok := app.AccessByRole(ui.RolePopup, "")
		if !ok {
			t.Fatal("proposal popover missing")
		}
		want := min(portrait.Form.PopoverWide, max(portrait.Form.MenuMin, float32(width)-2*portrait.Form.DialogPad))
		if !near(popup.Bounds.W, want) {
			t.Errorf("window %d: popover width = %v, want %v", width, popup.Bounds.W, want)
		}
		trigger, _ := app.AccessByRole(ui.RoleButton, "Change proposals")
		if popup.Bounds.Y < trigger.Bounds.Y+trigger.Bounds.H {
			t.Errorf("popover %v covers its trigger %v", popup.Bounds, trigger.Bounds)
		}
	}

	const height = 220
	app := proposalTester(t, 800, height, true, strings.Repeat("A detailed proposal summary that needs scrolling. ", 20))
	popup, _ := app.AccessByRole(ui.RolePopup, "")
	maxHeight := max(portrait.Form.MenuMin, float32(height)-2*portrait.Form.DialogPad)
	if popup.Bounds.H <= 0 || popup.Bounds.H > maxHeight+2*portrait.Form.PopoverPad+0.5 {
		t.Fatalf("long proposal popover height = %v, want at most %v", popup.Bounds.H, maxHeight+2*portrait.Form.PopoverPad)
	}
}

func TestProposalSegmentsShareWidthAndFooterUsesDecisionLayout(t *testing.T) {
	app := proposalTester(t, 800, 640, true, "Tighten the introduction")
	first, _ := app.AccessByRole(ui.RoleButton, "Quiet Software.md")
	second, _ := app.AccessByRole(ui.RoleButton, "Reading List.md")
	if !near(first.Bounds.W, second.Bounds.W) {
		t.Fatalf("file segments have widths %v and %v, want equal flexible tracks", first.Bounds.W, second.Bounds.W)
	}
	if _, ok := app.Find("Reading List.md"); !ok {
		t.Fatal("Reading List.md is clipped instead of fitting its flexible segment")
	}

	dismiss, _ := app.AccessByRole(ui.RoleButton, "Dismiss proposal")
	accept, _ := app.AccessByRole(ui.RoleButton, "Accept all clean hunks")
	apply, _ := app.AccessByRole(ui.RoleButton, "Apply decisions")
	if !near(dismiss.Bounds.Y, accept.Bounds.Y) || !near(accept.Bounds.Y, apply.Bounds.Y) {
		t.Fatalf("footer actions are scattered across rows: dismiss %v, accept %v, apply %v", dismiss.Bounds, accept.Bounds, apply.Bounds)
	}
	if !(dismiss.Bounds.X < accept.Bounds.X && accept.Bounds.X < apply.Bounds.X) {
		t.Fatalf("footer action order is dismiss %v, accept %v, apply %v", dismiss.Bounds, accept.Bounds, apply.Bounds)
	}
	popup, _ := app.AccessByRole(ui.RolePopup, "")
	if got, want := apply.Bounds.X+apply.Bounds.W, popup.Bounds.X+popup.Bounds.W-portrait.Form.PopoverPad-portrait.Form.PadX; !near(got, want) {
		t.Fatalf("Apply decisions ends at %v, want footer trailing edge %v", got, want)
	}
}

func TestProposalCompareActionUsesIntrinsicWidth(t *testing.T) {
	app := proposalTester(t, 800, 640, false, "")
	popup, _ := app.AccessByRole(ui.RolePopup, "")
	compare, _ := app.AccessByRole(ui.RoleButton, "Compare current text")
	if compare.Bounds.W > popup.Bounds.W/2 {
		t.Fatalf("Compare action stretches to %v in a %v-wide popover", compare.Bounds.W, popup.Bounds.W)
	}
	if got, want := compare.Bounds.X, popup.Bounds.X+portrait.Form.PopoverPad+portrait.Form.PadX; !near(got, want) {
		t.Fatalf("Compare action starts at %v, want content start %v", got, want)
	}
}
