package nativegenui

import (
	"slices"
	"testing"

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
