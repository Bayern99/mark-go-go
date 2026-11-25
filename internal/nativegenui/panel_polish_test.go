package nativegenui

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// polishSize is the test window's width, and polishWidth the same width as
// the float32 layout boxes report.
const polishSize = 640

const polishWidth = float32(polishSize)

// drawPolishedPanel draws the status and the view the way panel.draw nests
// them, so a test can render both without a window or an open document.
func drawPolishedPanel(c *ui.Context, p *panel, v *documentView, d workspace.DocumentInfo, busy bool) {
	ui.Column(c).Fill().Gap(8).Padding(8, 12, 12, 12).Children(func() {
		p.drawGenerationStatus(c, v, d, busy)
		p.drawView(c, v, d, "", busy)
	})
}

func TestAFailedGenerationDrawsTheFailureAlone(t *testing.T) {
	p := &panel{win: &nativeui.Window{}}
	d := workspace.DocumentInfo{Path: "doc.md"}
	v := &documentView{loaded: true, failure: &genui.GenerateFailure{
		Message: "The agent exited", Hint: "Check the agent command.", Detail: "stderr: unknown flag",
	}}
	app := ui.NewTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	if !app.HasText("Generation failed") || !app.HasText("Generation details") {
		t.Fatalf("the failure is not reported: %v", app.Texts())
	}
	if app.HasText("No view yet") {
		t.Error("a failed generation also draws the empty state, whose next step contradicts it")
	}

	v = &documentView{loaded: true}
	app.Frame()
	if !app.HasText("No view yet") {
		t.Fatalf("a document without a view lost its empty state: %v", app.Texts())
	}
}

func TestGenerationActionsNeverStretchAcrossThePanel(t *testing.T) {
	p := &panel{win: &nativeui.Window{}}
	d := workspace.DocumentInfo{Path: "doc.md"}
	v := &documentView{loaded: true, cancel: func() {}, failure: &genui.GenerateFailure{
		Message: "The agent exited", Hint: "Check the agent command.",
	}}
	app := ui.NewTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()

	retry, ok := app.Access("Retry")
	if !ok {
		t.Fatalf("a failed generation offers no retry: %v", app.Texts())
	}
	if retry.Bounds.W > polishWidth/2 {
		t.Errorf("Retry is %.0f wide in a %.0f panel, want a button that fits its label", retry.Bounds.W, polishWidth)
	}

	cancel, ok := app.Access("Cancel generation")
	if !ok {
		t.Fatal("a running generation offers no cancel")
	}
	if cancel.Bounds.W > polishWidth/2 {
		t.Errorf("Cancel generation is %.0f wide in a %.0f panel, want a button that fits its label", cancel.Bounds.W, polishWidth)
	}
	if cancel.Bounds.X > polishWidth/4 {
		t.Errorf("Cancel generation starts at %.0f, want it beside the view's left edge", cancel.Bounds.X)
	}
}

func TestTheRequestButtonSaysWhetherItsFormIsOpen(t *testing.T) {
	p := &panel{win: &nativeui.Window{}}
	v := &documentView{}
	app := ui.NewTester(func(c *ui.Context) { p.drawRequestToggle(c, v) }, polishSize, 120)

	state := func() (ui.AccessInfo, bool) { return app.Access("Generation request") }
	app.Frame()
	closed, ok := state()
	if !ok {
		t.Fatalf("the panel has no request toggle: %v", app.Texts())
	}
	if closed.Expanded || closed.Checked {
		t.Errorf("the closed request toggle reports expanded=%t pressed=%t, want neither", closed.Expanded, closed.Checked)
	}

	if err := app.Click("Generation request"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	open, _ := state()
	if !v.settings {
		t.Fatal("clicking the request toggle did not open the form")
	}
	if !open.Expanded || !open.Checked {
		t.Errorf("the open request toggle reports expanded=%t pressed=%t, want both", open.Expanded, open.Checked)
	}

	if err := app.Click("Generation request"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if v.settings {
		t.Error("clicking the open request toggle did not close the form")
	}
}

func TestTheEmptyStateAsksForARequestInsteadOfOpeningSettings(t *testing.T) {
	p := &panel{win: &nativeui.Window{}}
	v := &documentView{loaded: true}
	app := ui.NewTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, workspace.DocumentInfo{Path: "doc.md"}, false) }, polishSize, 480)
	app.Frame()

	if err := app.Click("Write a request"); err != nil {
		t.Fatalf("the empty state offers no request action: %v", app.Texts())
	}
	app.Frame()
	if !v.settings {
		t.Error("the empty state's action did not open the request form")
	}
	if _, ok := app.Access("Write a request"); ok {
		t.Error("the empty state still offers its action beside the open request form")
	}
}

func TestReaderNoteActionsRunCancelThenSaveFromTheRight(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	card := v.reading.surface.Nodes["card"]
	v.noteTarget, v.note = card.ID, "Draft note"
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, card, false)
	}, polishSize, 420)
	app.Frame()

	save, ok := app.Access("Save note")
	if !ok {
		t.Fatalf("the note composer offers no save: %v", app.Texts())
	}
	cancel, ok := app.Access("Cancel note")
	if !ok {
		t.Fatalf("the note composer offers no cancel: %v", app.Texts())
	}
	if cancel.Bounds.Y > save.Bounds.Y+save.Bounds.H || save.Bounds.Y > cancel.Bounds.Y+cancel.Bounds.H {
		t.Errorf("cancel sits at y %.0f and save at y %.0f, want them on one row", cancel.Bounds.Y, save.Bounds.Y)
	}
	if cancel.Bounds.X >= save.Bounds.X {
		t.Errorf("cancel starts at x %.0f and save at x %.0f, want cancel on the left", cancel.Bounds.X, save.Bounds.X)
	}
	if right := save.Bounds.X + save.Bounds.W; right < polishWidth*0.75 {
		t.Errorf("save ends at x %.0f in a %.0f panel, want the actions along the right edge", right, polishWidth)
	}
}

func TestShowAllKeepsItsOwnWidthInsideTheCard(t *testing.T) {
	view, err := Open(`{"version":2,"sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Report","items":["One","Two","Three","Four","Five","Six"],"sources":["overview"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	reader := &Reading{}
	reader.Load(view)
	app := ui.NewTester(reader.View, polishSize, 480)
	app.Frame()

	show, ok := app.Access("Show all 6")
	if !ok {
		t.Fatalf("a long list offers no way to show every point: %v", app.Texts())
	}
	if show.Bounds.W > polishWidth/2 || show.Bounds.X > polishWidth/4 {
		t.Errorf("Show all 6 is %.0f wide at x %.0f, want a button at the card's left edge", show.Bounds.W, show.Bounds.X)
	}
	if err := app.Click("Show all 6"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Six") || !app.HasText("Show fewer") {
		t.Errorf("expanding the list shows %v, want every point and a way back", app.Texts())
	}
}
