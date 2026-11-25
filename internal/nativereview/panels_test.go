package nativereview

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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

func duplicateBasenamePanel(proposals []review.ChangeProposal) *panel {
	return &panel{
		win:       &nativeui.Window{},
		latest:    review.ReviewUpdateEvent{Proposals: proposals},
		open:      true,
		decisions: map[string]review.HunkDecision{},
	}
}

func TestDuplicateBasenamesKeepDistinctTabsAndSelectTheirOwnProposal(t *testing.T) {
	const root = "/workspace"
	first := review.ChangeProposal{ID: "p-a", WorkspaceRoot: root, Path: root + "/a/README.md", RelPath: "a/README.md"}
	second := review.ChangeProposal{ID: "p-b", WorkspaceRoot: root, Path: root + "/b/README.md", RelPath: "b/README.md"}
	p := duplicateBasenamePanel([]review.ChangeProposal{first, second})
	app := ui.NewTester(func(c *ui.Context) { p.drawProposals(c) }, 900, 640)
	app.Frame()

	if p.selected != first.ID {
		t.Fatalf("opening the popover selects %q, want the first proposal %q", p.selected, first.ID)
	}
	a, ok := app.AccessByRole(ui.RoleTab, "a/README.md")
	if !ok {
		t.Fatalf("the first duplicate basename is not labeled by its folder: %q", app.Texts())
	}
	b, ok := app.AccessByRole(ui.RoleTab, "b/README.md")
	if !ok {
		t.Fatalf("the second duplicate basename is not labeled by its folder: %q", app.Texts())
	}
	if a.Description != first.Path || b.Description != second.Path {
		t.Fatalf("tabs describe %q and %q, want the full paths %q and %q", a.Description, b.Description, first.Path, second.Path)
	}
	if err := app.Click("b/README.md"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if p.selected != second.ID {
		t.Fatalf("clicking the second tab selects %q, want %q", p.selected, second.ID)
	}
	if b, _ := app.AccessByRole(ui.RoleTab, "b/README.md"); !b.Selected {
		t.Fatalf("the clicked tab is not shown selected: %+v", b)
	}
}

func TestManyProposalsMoveFromTabsToAReadableSelector(t *testing.T) {
	const root = "/workspace"
	proposals := make([]review.ChangeProposal, 8)
	for i := range proposals {
		name := fmt.Sprintf("design-notes-%d.md", i+1)
		proposals[i] = review.ChangeProposal{
			ID: fmt.Sprintf("p-%d", i+1), WorkspaceRoot: root,
			Path: root + "/design/" + name, RelPath: "design/" + name,
		}
	}
	p := duplicateBasenamePanel(proposals)
	app := ui.NewTester(func(c *ui.Context) { p.drawProposals(c) }, 900, 720)
	app.Frame()

	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleTab {
			t.Fatalf("eight documents still squeeze into a tab %q %.0f wide", info.Name, info.Bounds.W)
		}
	}
	trigger, ok := app.Access("Change proposal documents")
	if !ok {
		t.Fatalf("no document selector for eight proposals: %q", app.Texts())
	}
	if !strings.HasSuffix(trigger.Description, "/design/design-notes-1.md") {
		t.Fatalf("the selector describes %q, want the selected document's full path", trigger.Description)
	}
	if err := app.Click("Change proposal documents"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	last := "design-notes-8.md"
	if _, ok := app.Access(last); !ok {
		t.Fatalf("the selector does not list %q: %q", last, app.Texts())
	}
	if err := app.Click(last); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if p.selected != "p-8" {
		t.Fatalf("choosing %q selects %q, want p-8", last, p.selected)
	}
	if trigger, _ := app.Access("Change proposal documents"); !strings.HasSuffix(trigger.Description, "/design/"+last) {
		t.Fatalf("the selector describes %q, want the chosen document's full path", trigger.Description)
	}
}

func TestDismissedProposalComesBackFromItsUndoToast(t *testing.T) {
	const (
		root = "/workspace"
		path = root + "/Quiet Software.md"
		base = "first line\nsecond line\nthird line\n"
	)
	service := review.NewService()
	proposal, err := service.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: root, DocumentID: "a", Path: path, Summary: "Tighten the middle",
		BaseText: base, ProposedText: "first line\nSECOND line\nthird line\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	win := nativeui.SceneWindow(workspace.WorkspaceSnapshot{
		Root: root, ActiveID: "a", Documents: []workspace.DocumentInfo{{WorkspaceRoot: root, ID: "a", Path: path}},
	}, workspace.NavSnapshot{}, nil, base, portrait.DefaultSpace, false)
	p := &panel{
		win: win, ctx: context.Background(), proposals: service, root: root,
		latest: service.Snapshot(root), compared: proposal, open: true,
		decisions: map[string]review.HunkDecision{}, checkpointDocs: map[string]*checkpointState{},
		base: reading{document: win.Source(), id: "a", path: path, text: base, revision: win.Source().Version()},
	}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawProposals(c)
		portrait.Toasts(c)
	}, 900, 720)
	app.Frame()

	if err := app.Click("Dismiss proposal"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Advance(portrait.Motion.Enter + 20*time.Millisecond)
	if !app.HasText("Proposal dismissed") {
		t.Fatalf("dismissing shows no toast: %q", app.Texts())
	}
	if got := service.Snapshot(root).Proposals; len(got) != 0 {
		t.Fatalf("the dismissed proposal is still in the queue: %+v", got)
	}
	if err := app.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	restored := service.Snapshot(root).Proposals
	if len(restored) != 1 || restored[0].ID != proposal.ID {
		t.Fatalf("Undo restored %+v, want the dismissed proposal %q", restored, proposal.ID)
	}
	if !reflect.DeepEqual(restored[0].Hunks, proposal.Hunks) {
		t.Fatalf("Undo restored hunks:\n%+v\nwant:\n%+v", restored[0].Hunks, proposal.Hunks)
	}
	if p.selected != proposal.ID {
		t.Fatalf("Undo selects %q, want the restored proposal %q", p.selected, proposal.ID)
	}
}

func TestUndoOfADismissedProposalReplaysItsHunkDecisions(t *testing.T) {
	const (
		root = "/workspace"
		path = root + "/Quiet Software.md"
		base = "first line\nsecond line\nthird line\n"
	)
	service := review.NewService()
	proposal, err := service.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: root, DocumentID: "a", Path: path,
		BaseText: base, ProposedText: "first line\nSECOND line\nthird line\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Hunks[0].Decision != review.HunkPending {
		t.Fatalf("a fresh proposal starts with hunk %q", proposal.Hunks[0].Decision)
	}
	win := nativeui.SceneWindow(workspace.WorkspaceSnapshot{
		Root: root, ActiveID: "a", Documents: []workspace.DocumentInfo{{WorkspaceRoot: root, ID: "a", Path: path}},
	}, workspace.NavSnapshot{}, nil, base, portrait.DefaultSpace, false)
	p := &panel{
		win: win, ctx: context.Background(), proposals: service, root: root,
		latest: service.Snapshot(root), compared: proposal, open: true,
		decisions: map[string]review.HunkDecision{}, checkpointDocs: map[string]*checkpointState{},
		base: reading{document: win.Source(), id: "a", path: path, text: base, revision: win.Source().Version()},
	}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawProposals(c)
		portrait.Toasts(c)
	}, 900, 720)
	app.Frame()
	if err := app.Click("Accept hunk"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Dismiss proposal"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Advance(portrait.Motion.Enter + 20*time.Millisecond)
	if err := app.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	restored := service.Snapshot(root).Proposals
	if len(restored) != 1 {
		t.Fatalf("Undo restored %+v", restored)
	}
	if got := restored[0].Hunks[0].Decision; got != review.HunkAccepted {
		t.Fatalf("Undo restored hunk decision %q, want %q", got, review.HunkAccepted)
	}
}

func TestUndoOfADismissedProposalDoesNotOverwriteANewerProposalForTheSameDocument(t *testing.T) {
	const (
		root = "/workspace"
		path = root + "/Quiet Software.md"
		base = "first line\nsecond line\n"
	)
	service := review.NewService()
	oldProposal, err := service.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: root, DocumentID: "a", Path: path,
		BaseText: base, ProposedText: "first line\nOLD proposal\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	win := nativeui.SceneWindow(workspace.WorkspaceSnapshot{
		Root: root, ActiveID: "a", Documents: []workspace.DocumentInfo{{WorkspaceRoot: root, ID: "a", Path: path}},
	}, workspace.NavSnapshot{}, nil, base, portrait.DefaultSpace, false)
	p := &panel{
		win: win, ctx: context.Background(), proposals: service, root: root,
		latest: service.Snapshot(root), compared: oldProposal, open: true,
		decisions: map[string]review.HunkDecision{}, checkpointDocs: map[string]*checkpointState{},
		base: reading{document: win.Source(), id: "a", path: path, text: base, revision: win.Source().Version()},
	}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawProposals(c)
		portrait.Toasts(c)
	}, 900, 720)
	app.Frame()
	if err := app.Click("Dismiss proposal"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	newer, err := service.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: root, DocumentID: "a", Path: path,
		BaseText: base, ProposedText: "first line\nNEWER proposal\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.latest = service.Snapshot(root)
	app.Advance(portrait.Motion.Enter + 20*time.Millisecond)
	app.Frame()
	if err := app.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	got := service.Snapshot(root).Proposals
	if len(got) != 1 || got[0].ID != newer.ID || got[0].ProposedText != newer.ProposedText {
		t.Fatalf("Undo overwrote newer proposal: got %+v, want %+v", got, newer)
	}
	if !strings.Contains(p.proposalError, "newer proposal") {
		t.Fatalf("expected conflict notice, got %q", p.proposalError)
	}
}
