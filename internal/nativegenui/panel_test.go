package nativegenui

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestReaderNoteShortcutsFollowNoteFocus(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	card := v.reading.surface.Nodes["card"]
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Button(c, "Elsewhere")
			p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, card, false)
		})
	}, 620, 420)
	app.Frame()

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("Draft note")
	app.Frame()
	if v.note != "Draft note" || v.noteTarget != card.ID {
		t.Fatalf("note draft = %q for %q, want %q for %q", v.note, v.noteTarget, "Draft note", card.ID)
	}

	if err := app.Click("Elsewhere"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != card.ID || v.note != "Draft note" || v.message != "" {
		t.Fatalf("shortcuts away from the note changed it: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.Focused("Reader note") {
		t.Fatal("clicking the note did not focus it")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != "" || v.note != "" {
		t.Fatalf("Escape focused on the note did not cancel it: target=%q note=%q", v.noteTarget, v.note)
	}

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("Second draft")
	app.Frame()
	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	// The window has no open document, so reaching addNote reports it.
	if v.message != "Document closed" || v.noteTarget != card.ID || v.note != "Second draft" {
		t.Fatalf("Cmd+Enter focused on the note did not save it: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}
}

func TestReaderNoteShortcutsIgnoreActiveIMEComposition(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	card := v.reading.surface.Nodes["card"]
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, card, false)
	}, 620, 420)
	app.Frame()

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("keep ")
	app.Frame()

	// Start IME composition with active marked text.
	app.Compose("ceshi", 5)
	app.Frame()

	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != card.ID || v.note != "keep " || v.message != "" {
		t.Fatalf("Escape during active IME composition changed note: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	if v.noteTarget != card.ID || v.note != "keep " || v.message != "" {
		t.Fatalf("Cmd+Enter during active IME composition prematurely submitted note: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	// End composition and press Cmd+Enter; now it submits cleanly.
	app.Compose("", 0)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	if v.message != "Document closed" || v.note != "keep " {
		t.Fatalf("Cmd+Enter after IME composition ended did not reach save: note=%q message=%q", v.note, v.message)
	}
}

func TestChangedIDsUsesStoredIDsForRepeatedInstances(t *testing.T) {
	view := &documentView{
		reading: Reading{surface: &Surface{
			Sources: map[string]Reference{
				"changed": {Heading: "Changed", Hash: "old"},
				"current": {Heading: "Current"},
			},
			Nodes: map[string]Node{
				"row-[/items/0]":   {ID: "row-[/items/0]", ComponentID: "rowTemplate", Sources: []string{"changed"}},
				"row-[/items/1]":   {ID: "row-[/items/1]", ComponentID: "rowTemplate", Sources: []string{"changed"}},
				"current-instance": {ID: "current-instance", ComponentID: "currentCard", Sources: []string{"current"}},
				"stale-instance":   {ID: "stale-instance", ComponentID: "staleCard", Sources: []string{"changed"}},
				"missing-instance": {ID: "missing-instance", ComponentID: "missingCard", Sources: []string{"missing"}},
				"legacy":           {ID: "legacy", Sources: []string{"changed"}},
			},
		}},
	}
	view.index("# Changed\n\nNew section text.\n\n# Current\n\nUnchanged section text.\n")
	currentReference := view.reading.surface.Sources["current"]
	currentReference.Hash = view.sections[headingKey{name: workspace.NormalizeHeadingText("Current"), occurrence: 1}].Hash
	view.reading.surface.Sources["current"] = currentReference

	got := view.changedIDs()
	want := []string{"legacy", "missingCard", "rowTemplate", "staleCard"}
	if !slices.Equal(got, want) {
		t.Fatalf("changed IDs = %v, want stored component IDs %v", got, want)
	}
	for _, id := range got {
		if id == "row-[/items/0]" || id == "row-[/items/1]" {
			t.Errorf("changed IDs included a runtime row ID: %q", id)
		}
	}
}

func TestLoadingShowsAShimmeringSkeletonUnderReadableStatus(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		app := ui.NewTesterWith(func(c *ui.Context) {
			loading(c, "Loading the GenUI view", "Reading the view file for this document.")
		}, 640, 420, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		app.Frame()
		status, ok := app.AccessByRole(ui.RoleStatus, "Loading the GenUI view")
		if !ok || !status.Busy || !app.HasText("Reading the view file for this document.") {
			t.Fatalf("reduce %v: status %+v, texts %v", reduce, status, app.Texts())
		}
		before := app.Image()
		app.Advance(400 * time.Millisecond)
		moved := changedPixels(before, app.Image()) != 0
		if moved == reduce || app.FramePending() == reduce {
			t.Fatalf("reduce %v: skeleton shimmer moved %v, frames pending %v", reduce, moved, app.FramePending())
		}
	}
}

func TestSourceStatusSaysWhenTheSourceIsMissing(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"gone": {Heading: "Gone"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"gone"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, v.reading.surface.Nodes["card"], false)
	}, 620, 200)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleStatus, "Source missing"); !ok {
		t.Fatal("a missing source is not announced")
	}
	if heading, ok := app.Access("Gone"); !ok || !heading.Disabled {
		t.Fatalf("the missing source heading %+v, want disabled", heading)
	}
	if add, ok := app.Access("Add note"); !ok || !add.Disabled {
		t.Fatalf("Add note %+v, want disabled without a source", add)
	}
}

func changedPixels(a, b *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
			n++
		}
	}
	return n
}
