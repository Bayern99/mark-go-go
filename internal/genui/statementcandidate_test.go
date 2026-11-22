package genui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

var threeOversizedTexts = []string{
	`root = Col([latency, recall, cost])`,
	`latency = Card([Title("p95 latency", "md"), Text("23 ms", "xl", "success", "semibold")])`,
	`recall = Card([Title("Recall@10", "md"), Text("0.97", "xl")])`,
	`cost = Source(["overview"], [Text("$2,850 a month", "xl")])`,
}

func TestAStatementCandidateIsAcceptedWhenOptionalPropertiesHoldValuesTheCatalogDoesNotAllow(t *testing.T) {
	if err := ValidateCandidate(context.Background(), statementCandidate(t, oneSource, threeOversizedTexts)); err != nil {
		t.Fatalf("rejected with %v", err)
	}
}

func TestTheCandidateReportListsEachDegradedPropertyWithItsStatement(t *testing.T) {
	const oversized = `field "/size" expects one of ["xs", "sm", "md", "lg"] but got "xl"`
	for name, test := range map[string]struct {
		statements []string
		want       []DegradedProperty
	}{
		"one in each of three statements": {threeOversizedTexts, []DegradedProperty{
			{Statement: "latency", Component: "Text", Property: "size", Message: oversized},
			{Statement: "recall", Component: "Text", Property: "size", Message: oversized},
			{Statement: "cost", Component: "Text", Property: "size", Message: oversized},
		}},
		"the same property twice in a statement": {[]string{`root = Col([pair])`, `pair = Row([Text("23 ms", "xl"), Text("0.97", "xl")])`}, []DegradedProperty{
			{Statement: "pair", Component: "Text", Property: "size", Message: oversized},
		}},
		"a property of a component that is dropped": {[]string{`root = Col([chart])`, `chart = Chart("spiral", [{month: "Jan", cost: 1}], "month", ["cost"], "money")`}, nil},
		"properties of several components": {[]string{`root = Col([Badge("Late", "loud"), Row([Title("Costs", "huge")], 2, "middle"), Text("Struck", "md", "muted", "bold", "yes")])`}, []DegradedProperty{
			{Statement: "root", Component: "Badge", Property: "color", Message: `field "/color" expects one of ["neutral", "info", "success", "warning", "danger", "purple", "pink"] but got "loud"`},
			{Statement: "root", Component: "Title", Property: "size", Message: `field "/size" expects one of ["sm", "md", "lg", "xl", "2xl", "3xl"] but got "huge"`},
			{Statement: "root", Component: "Row", Property: "align", Message: `field "/align" expects one of ["start", "center", "end", "stretch", "baseline"] but got "middle"`},
			{Statement: "root", Component: "Text", Property: "weight", Message: `field "/weight" expects one of ["normal", "medium", "semibold"] but got "bold"`},
			{Statement: "root", Component: "Text", Property: "italic", Message: `field "/italic" expects boolean but got string`},
		}},
	} {
		report, err := CheckCandidate(context.Background(), statementCandidate(t, oneSource, test.statements))
		if err != nil {
			t.Errorf("%s: rejected with %v", name, err)
		}
		if !slices.Equal(report.Degraded, test.want) {
			t.Errorf("%s: degraded %+v, want %+v", name, report.Degraded, test.want)
		}
	}
}

func TestAStatementCandidateStillCountsTheStatementsItCannotDraw(t *testing.T) {
	oversized := threeOversizedTexts[1:]
	for name, test := range map[string]struct {
		statements []string
		want       string
	}{
		"unknown components":                         {[]string{`root = Col([a, b, c])`, `a = Sparkle("x")`, `b = Glitter("y")`, `c = Shimmer("z")`}, "3 faulty statements"},
		"missing required arguments":                 {[]string{`root = Col([a, b, c])`, `a = Text()`, `b = Badge()`, `c = Link("Docs")`}, "3 faulty statements"},
		"undefined references":                       {[]string{`root = Col([a, b, c])`}, "3 faulty statements"},
		"required properties of the wrong type":      {[]string{`root = Col([a, b, c])`, `a = Chart("spiral", [{month: "Jan", cost: 1}])`, `b = Title(12)`, `c = Slider($n, "low", 10)`, `$n = 3`}, "3 faulty statements"},
		"two faults beside degraded properties":      {append([]string{`root = Col([latency, recall, cost, a, b])`, `a = Sparkle("x")`, `b = Title(12)`}, oversized...), ""},
		"three faults beside degraded properties":    {append([]string{`root = Col([latency, recall, cost, a, b, c])`, `a = Sparkle("x")`, `b = Title(12)`, `c = Text()`}, oversized...), "3 faulty statements"},
		"a degraded property on a dropped component": {[]string{`root = Col([a, b, c])`, `a = Chart("spiral", [{month: "Jan", cost: 1}], "month", ["cost"], "money")`, `b = Glitter("y")`, `c = Shimmer("z")`}, "3 faulty statements"},
	} {
		report, err := CheckCandidate(context.Background(), statementCandidate(t, oneSource, test.statements))
		switch {
		case test.want == "" && err != nil:
			t.Errorf("%s: rejected with %v", name, err)
		case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
			t.Errorf("%s: got %v, want an error with %q", name, err, test.want)
		case test.want != "" && len(report.Degraded) > 0:
			t.Errorf("%s: a rejected candidate reports %+v", name, report.Degraded)
		}
	}
}
