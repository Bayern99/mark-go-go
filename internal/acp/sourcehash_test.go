package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const staleHash = "fnv1a32:00000000"

var currentHash = map[string]string{
	"Report":   "fnv1a32:d04be74a",
	"Detail":   "fnv1a32:99d21f4d",
	"Appendix": "fnv1a32:c6896bcf",
}

const staleSources = `{
    "report": {"heading": "Report", "occurrence": 1, "hash": "fnv1a32:00000000"},
    "detail": {"heading": "Detail", "occurrence": 1, "hash": "fnv1a32:00000000"},
    "appendix": {"heading": "Appendix", "occurrence": 1, "hash": "fnv1a32:00000000"}
  }`

func cardsView(components ...string) string {
	return `{"version": 2, "generatedAt": "2026-11-06T09:00:00Z", "sources": ` + staleSources + `, "a2ui": [{"version": "1.0", "createSurface": {"surfaceId": "view", "catalogId": "markgogo.dev:genui/v1", "components": [` + strings.Join(components, ",") + `]}}]}`
}

func cardsUpdate(components ...string) string {
	return `{"sources":{},"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":[` + strings.Join(components, ",") + `]}}]}`
}

func statementList(statements []string) string {
	list, _ := json.Marshal(statements)
	return string(list)
}

func statementsView(statements ...string) string {
	return `{"version": 3, "language": "openui-lang/1.0", "generatedAt": "2026-11-06T09:00:00Z", "sources": ` + staleSources + `, "a2ui": [{"version": "1.0", "createSurface": {"surfaceId": "view", "catalogId": "markgogo.dev:genui/v3", "components": ` + statementList(statements) + `}}]}`
}

func statementsUpdate(statements ...string) string {
	return `{"sources":{},"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":` + statementList(statements) + `}}]}`
}

var (
	fourCards = cardsView(
		`{"id": "root", "component": "Column", "children": ["report", "detail", "shared", "alsoShared"]}`,
		`{"id": "report", "component": "KeyPoints", "items": ["Old report"], "sources": ["report"]}`,
		`{"id": "detail", "component": "KeyPoints", "items": ["Old detail"], "sources": ["detail"]}`,
		`{"id": "shared", "component": "KeyPoints", "items": ["Old appendix"], "sources": ["appendix"]}`,
		`{"id": "alsoShared", "component": "KeyPoints", "items": ["Old appendix again"], "sources": ["appendix"]}`,
	)
	fourStatements = statementsView(
		`root = Col([report, detail, shared, alsoShared])`,
		`report = Source(["report"], [Text("Old report")])`,
		`detail = Source(["detail"], [Text("Old detail")])`,
		`shared = Card([Source(["appendix"], [Text("Old appendix")])])`,
		`alsoShared = Source(["appendix"], [Text("Old appendix again")])`,
	)
)

func assertHashes(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
	for id, hash := range want {
		if got[id] != hash {
			t.Errorf("source %q has the hash %q, want %q", id, got[id], hash)
		}
	}
}

func staleView(t *testing.T, text string) string {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	if err := os.WriteFile(viewPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return viewPath
}

func TestAnUpdateRehashesOnlyTheSourcesWhoseCardsItReplaced(t *testing.T) {
	onlyReport := map[string]string{"report": currentHash["Report"], "detail": staleHash, "appendix": staleHash}
	onlyAppendix := map[string]string{"report": staleHash, "detail": staleHash, "appendix": currentHash["Appendix"]}
	none := map[string]string{"report": staleHash, "detail": staleHash, "appendix": staleHash}
	for name, test := range map[string]struct {
		view, out string
		replace   []string
		want      map[string]string
	}{
		"one of two changed cards": {
			view:    fourCards,
			replace: []string{"report"},
			out:     cardsUpdate(`{"id":"report","component":"KeyPoints","items":["New report"],"sources":["report"]}`),
			want:    onlyReport,
		},
		"one of two cards that share a source": {
			view:    fourCards,
			replace: []string{"shared"},
			out:     cardsUpdate(`{"id":"shared","component":"KeyPoints","items":["New appendix"],"sources":["appendix"]}`),
			want:    none,
		},
		"both cards that share a source": {
			view:    fourCards,
			replace: []string{"shared", "alsoShared"},
			out: cardsUpdate(
				`{"id":"shared","component":"KeyPoints","items":["New appendix"],"sources":["appendix"]}`,
				`{"id":"alsoShared","component":"Card","child":"moved"}`,
				`{"id":"moved","component":"KeyPoints","items":["New appendix again"],"sources":["appendix"]}`,
			),
			want: onlyAppendix,
		},
		"a card that now names the sources a row template and a tab still use": {
			view: cardsView(
				`{"id": "root", "component": "Column", "children": ["report", "rows", "tabs"]}`,
				`{"id": "report", "component": "KeyPoints", "items": ["Old report"], "sources": ["report"]}`,
				`{"id": "rows", "component": "Column", "children": {"componentId": "row", "path": "/rows"}}`,
				`{"id": "row", "component": "KeyPoints", "items": ["Old detail"], "sources": ["detail"]}`,
				`{"id": "tabs", "component": "Tabs", "tabs": [{"title": "Appendix", "child": "tab"}]}`,
				`{"id": "tab", "component": "KeyPoints", "items": ["Old appendix"], "sources": ["appendix"]}`,
			),
			replace: []string{"report"},
			out:     cardsUpdate(`{"id":"report","component":"KeyPoints","items":["New report"],"sources":["report","detail","appendix"]}`),
			want:    onlyReport,
		},
		"a card whose earlier child is left behind": {
			view: cardsView(
				`{"id": "root", "component": "Column", "children": ["card"]}`,
				`{"id": "card", "component": "Card", "child": "left"}`,
				`{"id": "left", "component": "KeyPoints", "items": ["Old report"], "sources": ["report"]}`,
			),
			replace: []string{"card"},
			out: cardsUpdate(
				`{"id":"card","component":"Card","child":"fresh"}`,
				`{"id":"fresh","component":"KeyPoints","items":["New report"],"sources":["report"]}`,
			),
			want: onlyReport,
		},
		"one of two changed statements": {
			view:    fourStatements,
			replace: []string{"report"},
			out:     statementsUpdate(`report = Source(["report"], [Text("New report")])`),
			want:    onlyReport,
		},
		"one of two statements that share a source": {
			view:    fourStatements,
			replace: []string{"shared"},
			out:     statementsUpdate(`shared = Card([Source(["appendix"], [Text("New appendix")])])`),
			want:    none,
		},
		"both statements that share a source": {
			view:    fourStatements,
			replace: []string{"shared", "alsoShared"},
			out: statementsUpdate(
				`shared = Card([moved])`,
				`moved = Source(["appendix"], [Text("New appendix")])`,
				`alsoShared = Source(["appendix"], [Text("New appendix again")])`,
			),
			want: onlyAppendix,
		},
		"two statements asked for, one written": {
			view:    fourStatements,
			replace: []string{"shared", "alsoShared"},
			out:     statementsUpdate(`shared = Card([Source(["appendix"], [Text("New appendix")])])`),
			want:    none,
		},
		"a statement that uses the source only while a switch is on": {
			view: statementsView(
				`root = Col([Switch($open, "Open"), report, hidden])`,
				`$open = false`,
				`report = Source(["report"], [Text("Old report")])`,
				`hidden = $open ? Source(["report"], [Text("Old report, in full")]) : Text("Closed")`,
			),
			replace: []string{"report"},
			out:     statementsUpdate(`report = Source(["report"], [Text("New report")])`),
			want:    none,
		},
		"a statement that takes its sources from another statement": {
			view: statementsView(
				`root = Col([report, computed])`,
				`report = Source(["report"], [Text("Old report")])`,
				`computed = Source(cited, [Text("Old report again")])`,
				`cited = ["report"]`,
			),
			replace: []string{"report"},
			out:     statementsUpdate(`report = Source(["report"], [Text("New report")])`),
			want:    none,
		},
		"a statement an earlier update left behind": {
			view: statementsView(
				`root = Col([report])`,
				`report = Card([body])`,
				`body = Source(["report"], [Text("Old report")])`,
				`left = Source(["report"], [Text("Older report")])`,
			),
			replace: []string{"report"},
			out:     statementsUpdate(`report = Source(["report"], [Text("New report")])`),
			want:    onlyReport,
		},
	} {
		t.Run(name, func(t *testing.T) {
			viewPath := staleView(t, test.view)
			if err, _, _ := driveGeneration(t, fakeSpec{Out: test.out}, nil, updateJob(viewPath, test.replace...)); err != nil {
				t.Fatalf("RunGeneration: %v", err)
			}
			assertHashes(t, writtenView(t, viewPath).Hashes, test.want)
		})
	}
}

func TestAFullGenerationRehashesEverySource(t *testing.T) {
	const sources = `{"report":{"heading":"Report","occurrence":1},"detail":{"heading":"Detail","occurrence":1},"appendix":{"heading":"Appendix","occurrence":1}}`
	for name, test := range map[string]struct{ view, out string }{
		"cards": {
			view: fourCards,
			out:  `{"sources":` + sources + `,"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","items":["New report"],"sources":["report"]}]}}]}`,
		},
		"statements": {
			view: fourStatements,
			out:  `{"sources":` + sources + `,"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Source([\"report\"], [Text(\"New report\")])"]}}]}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			viewPath := staleView(t, test.view)
			if err, _, _ := driveGeneration(t, fakeSpec{Out: test.out}, nil, func(job *GenerationJob) { job.ViewPath = viewPath }); err != nil {
				t.Fatalf("RunGeneration: %v", err)
			}
			assertHashes(t, writtenView(t, viewPath).Hashes, map[string]string{"report": currentHash["Report"], "detail": currentHash["Detail"], "appendix": currentHash["Appendix"]})
		})
	}
}

func TestALaterUpdateCountsWhatEarlierUpdatesWrote(t *testing.T) {
	for name, test := range map[string]struct{ view, unsourced, renewed string }{
		"cards": {
			view:      fourCards,
			unsourced: cardsUpdate(`{"id":"shared","component":"KeyPoints","items":["No longer from the appendix"]}`),
			renewed:   cardsUpdate(`{"id":"alsoShared","component":"KeyPoints","items":["New appendix"],"sources":["appendix"]}`),
		},
		"statements": {
			view:      fourStatements,
			unsourced: statementsUpdate(`shared = Card([Text("No longer from the appendix")])`),
			renewed:   statementsUpdate(`alsoShared = Source(["appendix"], [Text("New appendix")])`),
		},
	} {
		t.Run(name, func(t *testing.T) {
			viewPath := staleView(t, test.view)
			if err, _, _ := driveGeneration(t, fakeSpec{Out: test.unsourced}, nil, updateJob(viewPath, "shared")); err != nil {
				t.Fatalf("first update: %v", err)
			}
			assertHashes(t, writtenView(t, viewPath).Hashes, map[string]string{"report": staleHash, "detail": staleHash, "appendix": staleHash})
			if err, _, _ := driveGeneration(t, fakeSpec{Out: test.renewed}, nil, updateJob(viewPath, "alsoShared")); err != nil {
				t.Fatalf("second update: %v", err)
			}
			assertHashes(t, writtenView(t, viewPath).Hashes, map[string]string{"report": staleHash, "detail": staleHash, "appendix": currentHash["Appendix"]})
		})
	}
}
