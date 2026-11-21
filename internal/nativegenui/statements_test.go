package nativegenui_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func statementView(t *testing.T, statements ...string) *nativegenui.Surface {
	t.Helper()
	components, _ := json.Marshal(statements)
	view, err := nativegenui.Open(`{"version":3,"language":"openui-lang/1.0","sources":{"s":{"heading":"Report"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":` + string(components) + `}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func shown(t *testing.T, view *nativegenui.Surface, width int, prepare func(*nativegenui.Reading)) (*ui.Tester, string) {
	t.Helper()
	var reader nativegenui.Reading
	reader.Load(view)
	if prepare != nil {
		prepare(&reader)
	}
	tt := ui.NewTester(reader.View, width, 600)
	tt.Frame()
	tt.Frame()
	var names []string
	for _, el := range tt.AccessAll() {
		names = append(names, el.Name)
	}
	return tt, strings.Join(names, "\n")
}

func TestPublishedStatementExampleShowsEveryLayoutAndTextPrimitive(t *testing.T) {
	doc, err := os.ReadFile("../../docs/genui/view-format.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```json\n(.*?)```").FindAllStringSubmatch(string(doc), -1)
	example := blocks[len(blocks)-1][1]
	view, err := nativegenui.Open(example)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{900, 320} {
		_, text := shown(t, view, width, nil)
		for _, want := range []string{"Block-preserving edits", "Edits keep untouched blocks byte for byte.", "In short", "Stable", "Only the edited block is written again.", "First prototype", "Design notes", "What the reader sees is what is saved.", "ok := bytes.Equal(before, after)", "Figures are from the March run."} {
			if !strings.Contains(text, want) {
				t.Errorf("width %v: %q is not shown in\n%s", width, want, text)
			}
		}
		if strings.Contains(text, "not a component") {
			t.Errorf("width %v: the example reports an unknown component:\n%s", width, text)
		}
	}
	for _, name := range []string{"Col", "Row", "Box", "Grid", "Card", "Divider", "Spacer", "Title", "Text", "Caption", "Label", "Badge", "Link", "List", "ListItem", "Markdown", "Quote", "Code", "Source"} {
		if !regexp.MustCompile(`\b` + name + `\(`).MatchString(example) {
			t.Errorf("the published example does not use %s", name)
		}
	}
}

func TestStatementViewAppliesLaterUpdates(t *testing.T) {
	view, err := nativegenui.Open(`{"version":3,"language":"openui-lang/1.0","a2ui":[
		{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Col([lead, tail])","lead = Text(\"Draft\")","tail = Text(\"Kept\")"]}},
		{"version":"1.0","updateComponents":{"surfaceId":"view","components":["lead = Text(\"Final\")"]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	_, text := shown(t, view, 520, nil)
	if strings.Contains(text, "Draft") || !strings.Contains(text, "Final") || !strings.Contains(text, "Kept") {
		t.Fatalf("shown:\n%s", text)
	}
}

func TestStatementSourceWrapperOffersItsSources(t *testing.T) {
	view := statementView(t, `root = Col([Source(["s"], [Text("Figures")])])`)
	var offered []string
	_, text := shown(t, view, 520, func(r *nativegenui.Reading) {
		r.Sources = func(c *ui.Context, n nativegenui.Node) {
			offered = n.Sources
			ui.Text(c, "From Report")
		}
	})
	if len(offered) != 1 || offered[0] != "s" || !strings.Contains(text, "Figures") || !strings.Contains(text, "From Report") {
		t.Fatalf("offered %v, shown:\n%s", offered, text)
	}
}

func TestStatementViewNamesUnknownComponentsAndKeepsTheRest(t *testing.T) {
	view := statementView(t, `root = Col([Text("Still here"), Sparkle("x")])`)
	_, text := shown(t, view, 520, nil)
	if !strings.Contains(text, "Still here") || !strings.Contains(text, "“Sparkle” is not a component this app can show.") {
		t.Fatalf("shown:\n%s", text)
	}
}

func TestStatementViewRejectsWhatItCannotRead(t *testing.T) {
	for name, file := range map[string]string{
		"object components": `{"version":3,"language":"openui-lang/1.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v3","components":[{"id":"root","component":"Text","text":"x"}]}}]}`,
		"other language":    `{"version":3,"language":"openui-lang/2.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v3","components":["root = Text(\"x\")"]}}]}`,
		"other catalog":     `{"version":3,"language":"openui-lang/1.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":["root = Text(\"x\")"]}}]}`,
		"no root":           `{"version":3,"language":"openui-lang/1.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v3","components":["$count = 1"]}}]}`,
	} {
		if _, err := nativegenui.Open(file); err == nil {
			t.Errorf("%s: opened", name)
		}
	}
}
