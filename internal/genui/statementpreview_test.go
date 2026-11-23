package genui

import (
	"slices"
	"testing"
)

func TestStatementPreviewReadsOnlyCompleteStatementsAtEveryJSONPrefix(t *testing.T) {
	candidate := []byte(`{"sources":{"overview":{"heading":"Overview","occurrence":1}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Col([lead, note])","lead = Source([\"overview\"], [Text(\"Hello\", \"xl\")])","note = Text(\"A unicode escape: \u4f60\u597d and a quote: \\\"\")"]}}]}`)
	all := []string{
		`root = Col([lead, note])`,
		`lead = Source(["overview"], [Text("Hello", "xl")])`,
		`note = Text("A unicode escape: 你好 and a quote: \"")`,
	}
	seenStages := 0
	last := 0
	for at := 0; at <= len(candidate); at++ {
		preview := ReadStatementPreview(candidate[:at])
		if len(preview.Statements) > len(all) || !slices.Equal(preview.Statements, all[:len(preview.Statements)]) {
			t.Fatalf("prefix %d has statements %#v, want a prefix of %#v", at, preview.Statements, all)
		}
		if len(preview.Statements) != last {
			last = len(preview.Statements)
			seenStages++
		}
	}
	if seenStages != len(all) {
		t.Fatalf("preview changed through %d statement stages, want %d", seenStages, len(all))
	}
	preview := ReadStatementPreview(candidate)
	if got := preview.Sources["overview"]; got.Heading != "Overview" || got.Occurrence != 1 {
		t.Errorf("preview source = %+v", got)
	}
}

func TestStatementPreviewDoesNotMistakeAnUpdateForAWholeView(t *testing.T) {
	preview := ReadStatementPreview([]byte(`{"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":["root = Col([])"]}}]}`))
	if len(preview.Statements) != 0 {
		t.Fatalf("update preview statements = %v, want none", preview.Statements)
	}
}

func TestStatementPreviewCanFindSourcesWrittenAfterTheSurface(t *testing.T) {
	preview := ReadStatementPreview([]byte(`{"a2ui":[{"createSurface":{"components":["root = Col([])"]}}],"sources":{"later":{"heading":"Later"}}}`))
	if len(preview.Statements) != 1 || preview.Sources["later"].Heading != "Later" {
		t.Fatalf("preview = %+v", preview)
	}
}
