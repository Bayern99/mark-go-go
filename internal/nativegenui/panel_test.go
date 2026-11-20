package nativegenui

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/workspace"
)

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
