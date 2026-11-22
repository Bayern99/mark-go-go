package genui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func statementCandidate(t *testing.T, sources string, batches ...[]string) []byte {
	t.Helper()
	var messages []map[string]any
	for i, batch := range batches {
		kind := "updateComponents"
		body := map[string]any{"surfaceId": "view", "components": batch}
		if i == 0 {
			kind, body["catalogId"] = "createSurface", StatementCatalogID
		}
		messages = append(messages, map[string]any{"version": "1.0", kind: body})
	}
	raw, err := json.Marshal(map[string]any{"version": 3, "language": StatementLanguage, "sources": json.RawMessage(sources), "a2ui": messages})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func doubling(n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, "s"+strconv.Itoa(i)+" = Col([s"+strconv.Itoa(i-1)+", s"+strconv.Itoa(i-1)+"])")
	}
	return out
}

const oneSource = `{"overview": {"heading": "Overview", "occurrence": 1}}`

func TestAStatementCandidateIsAcceptedWhenItRendersWithFewFaults(t *testing.T) {
	for name, test := range map[string]struct {
		statements []string
		want       string
	}{
		"a valid view":                {[]string{`root = Col([lead, Slider($n, 0, 10)])`, `$n = 3`, `lead = Source(["overview"], [Text("n is " + $n)])`}, ""},
		"one faulty statement":        {[]string{`root = Col([lead, extra])`, `lead = Text("Kept")`, `extra = Sparkle("unknown")`}, ""},
		"one statement, three faults": {[]string{`root = Col([lead, Title()])`, `lead = Chart("bogus", "none", 3, 4)`}, ""},
		"a subtree shared many times": {append([]string{`root = Col([s30])`, `s0 = Source(["overview"], [Text("x")])`}, doubling(30)...), ""},
		"no root":                     {[]string{`lead = Text("Orphan")`}, "no root"},
		"too many faults":             {[]string{`root = Col([a, b, c, d])`, `a = Sparkle()`, `b = Glitter()`, `c = Shimmer()`}, "faulty statements"},
		"a source it does not own":    {[]string{`root = Col([Source(["budget"], [Text("Cost")])])`}, `"budget"`},
	} {
		err := ValidateCandidate(context.Background(), statementCandidate(t, oneSource, test.statements))
		switch {
		case test.want == "" && err != nil:
			t.Errorf("%s: rejected with %v", name, err)
		case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
			t.Errorf("%s: got %v, want an error with %q", name, err, test.want)
		}
	}
}

func TestAStatementUpdateKeepsTheStatementsItDoesNotName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md.genui.json")
	saved := statementCandidate(t, oneSource, []string{`root = Col([lead, tail])`, `lead = Source(["overview"], [Text("Old")])`, `tail = Text("Stays")`})
	if err := os.WriteFile(path, saved, 0o644); err != nil {
		t.Fatal(err)
	}
	base, err := ReadBaseView(path, []string{"lead"})
	if err != nil {
		t.Fatal(err)
	}
	update := func(statements ...string) json.RawMessage {
		raw, _ := json.Marshal([]any{map[string]any{"version": "1.0", "updateComponents": map[string]any{"surfaceId": "view", "components": statements}}})
		return raw
	}
	if _, _, err := base.Merge(update(`tail = Text("Changed")`), nil); err == nil || !strings.Contains(err.Error(), `"tail"`) {
		t.Errorf("an update of a statement it was not asked to replace gave %v", err)
	}
	if _, _, err := base.Merge(update("lead = Text(\"New\")\ntail = Text(\"Changed\")"), nil); err == nil {
		t.Error("a component holding two statements was accepted")
	}
	sources, merged, err := base.Merge(update(`lead = Source(["overview"], [Text("New"), more])`, `more = Caption("Added")`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !WritesStatements(merged) || len(sources) != 1 {
		t.Fatalf("merged %s with sources %v", merged, sources)
	}
	view := statementCandidate(t, oneSource)
	var file map[string]json.RawMessage
	_ = json.Unmarshal(view, &file)
	file["a2ui"] = merged
	whole, _ := json.Marshal(file)
	if err := ValidateCandidate(context.Background(), whole); err != nil {
		t.Fatal(err)
	}
	read, err := ReadStatementView(whole)
	if err != nil || len(read.Statements) != 2 || read.Statements[0][2] != `tail = Text("Stays")` {
		t.Errorf("the merged view lost a statement: %v %v", read, err)
	}
}

func TestThePromptForStatementsAsksForTheStatementFormat(t *testing.T) {
	full := GenerationPrompt(GenerateRequest{Text: "# Doc", Mode: "full", Format: "FORMAT", Language: StatementLanguage}, nil, "/tmp/x")
	for _, want := range []string{StatementCatalogID, "openui-lang statement", "Do not use the network", "Preserve facts exactly", "use a Table"} {
		if !strings.Contains(full, want) {
			t.Errorf("the prompt lacks %q", want)
		}
	}
	if old := GenerationPrompt(GenerateRequest{Text: "# Doc", Mode: "full", Format: "FORMAT"}, nil, "/tmp/x"); strings.Contains(old, StatementCatalogID) || !strings.Contains(old, "ComparisonTable") {
		t.Error("a request without the statement language changed its prompt")
	}
}

func TestThePromptForStatementsKeepsNumberedStepsOutOfTheTimeline(t *testing.T) {
	full := GenerationPrompt(GenerateRequest{Text: "# Doc", Mode: "full", Format: "FORMAT", Language: StatementLanguage}, nil, "/tmp/x")
	for _, want := range []string{"use a timeline List", "time or date as its marker", "use a numbered List"} {
		if !strings.Contains(full, want) {
			t.Errorf("the prompt lacks %q", want)
		}
	}
	if strings.Contains(full, "route steps, use a timeline List") {
		t.Error("the prompt still sends every ordered sequence to a timeline")
	}
	if old := GenerationPrompt(GenerateRequest{Text: "# Doc", Mode: "full", Format: "FORMAT"}, nil, "/tmp/x"); !strings.Contains(old, "route steps, use Timeline") || strings.Contains(old, "numbered List") {
		t.Error("a request without the statement language changed its prompt")
	}
}
