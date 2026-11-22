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
		"other surface":     `{"version":3,"language":"openui-lang/1.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v3","components":["root = Text(\"x\")"]}},{"version":"1.0","updateComponents":{"surfaceId":"w","components":["root = Text(\"y\")"]}}]}`,
		"no root":           `{"version":3,"language":"openui-lang/1.0","a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v3","components":["$count = 1"]}}]}`,
	} {
		if _, err := nativegenui.Open(file); err == nil {
			t.Errorf("%s: opened", name)
		}
	}
}

func TestStatementViewKeepsStatementsALaterUpdateUsesAgain(t *testing.T) {
	view, err := nativegenui.Open(`{"version":3,"language":"openui-lang/1.0","a2ui":[
		{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Col([lead, tail])","lead = Text(\"Lead\")","tail = Text(\"Kept\")"]}},
		{"version":"1.0","updateComponents":{"surfaceId":"view","components":["root = Col([lead])"]}},
		{"version":"1.0","updateComponents":{"surfaceId":"view","components":["root = Col([lead, tail])"]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	_, text := shown(t, view, 520, nil)
	if !strings.Contains(text, "Kept") || strings.Contains(text, "never defined") {
		t.Fatalf("shown:\n%s", text)
	}
}

func TestStatementTextKeepsParenthesesInLinkAddresses(t *testing.T) {
	view := statementView(t, `root = Col([Text("See [Go](https://en.wikipedia.org/wiki/Go_(programming_language)) for more."), ListItem("Alone", "Detail", "March")])`)
	tt, text := shown(t, view, 520, nil)
	if !strings.Contains(text, "March") {
		t.Errorf("a list entry outside a list lost its marker:\n%s", text)
	}
	if strings.Contains(text, ") for more") {
		t.Errorf("the link address was cut at its first parenthesis:\n%s", text)
	}
	for _, el := range tt.AccessAll() {
		if el.Name == "Go" && el.Role == ui.RoleLink {
			return
		}
	}
	t.Fatalf("no link named Go in\n%s", text)
}

func named(tt *ui.Tester, role ui.Role, name string) []ui.Rect {
	var boxes []ui.Rect
	for _, el := range tt.AccessAll() {
		if el.Role == role && el.Name == name {
			boxes = append(boxes, el.Bounds)
		}
	}
	return boxes
}

func TestCardTitleIsSetNoSmallerThanTheCardsContent(t *testing.T) {
	const words = "Speed against operations"
	for _, size := range []string{"sm", "md"} {
		view := statementView(t, `root = Col([Title("`+words+`", "`+size+`"), Card([Title("`+words+`", "`+size+`")]), Card([Title("Trade-offs", "`+size+`"), List([ListItem("`+words+`", "Detail")])]), Card([Title("Context", "`+size+`"), Divider(), Text("`+words+`")])], null, "start")`)
		tt, text := shown(t, view, 700, nil)
		titles, plain := named(tt, ui.RoleHeading, words), named(tt, ui.RoleText, words)
		if len(titles) != 2 || len(plain) != 2 {
			t.Fatalf("%s: %d titles and %d texts read %q in\n%s", size, len(titles), len(plain), words, text)
		}
		page, card, item, body := titles[0].W, titles[1].W, plain[0].W, plain[1].W
		if card != page {
			t.Errorf("%s: the same words are %v wide as a card's title and %v wide as a title outside a card", size, card, page)
		}
		if card < item || card < body {
			t.Errorf("%s: the same words are %v wide as the card's title, %v as a list entry's title and %v as body text", size, card, item, body)
		}
	}
}

func afterTitleAndAsBody(t *testing.T, tt *ui.Tester, words string) (afterTitle, body ui.Rect) {
	t.Helper()
	boxes := named(tt, ui.RoleText, words)
	if len(boxes) != 2 {
		t.Fatalf("%d texts read %q", len(boxes), words)
	}
	return boxes[0], boxes[1]
}

func TestParagraphAfterACardTitleIsSetAsBodyText(t *testing.T) {
	const paragraph = "The search team needs a vector store for 40 million 768-dimension embeddings, growing by about 2 million a month. The target is p95 query latency under 50 ms at 200 queries per second with recall@10 of at least 0.95."
	for _, size := range []string{``, `, "sm"`, `, "xs"`} {
		text := `Text("` + paragraph + `"` + size + `)`
		view := statementView(t, `root = Col([Card([Title("Context", "md"), `+text+`]), Card([Title("Context", "md"), Divider(), `+text+`])])`)
		for _, width := range []int{700, 320} {
			tt, _ := shown(t, view, width, nil)
			afterTitle, body := afterTitleAndAsBody(t, tt, paragraph)
			if afterTitle.H != body.H || afterTitle.W != body.W {
				t.Errorf("Text(…%s) at width %d: the paragraph takes %vx%v after the card's title and %vx%v as body text", size, width, afterTitle.W, afterTitle.H, body.W, body.H)
			}
		}
	}
}

func TestOneLineAfterACardTitleIsItsSubtitleUntilItWraps(t *testing.T) {
	const phrase = "Three phases, finished by the middle of March"
	for _, size := range []string{``, `, "sm"`} {
		text := `Text("` + phrase + `"` + size + `)`
		view := statementView(t, `root = Col([Card([Title("Plan", "md"), `+text+`]), Card([Title("Plan", "md"), Divider(), `+text+`])])`)
		tt, _ := shown(t, view, 700, nil)
		for _, step := range []struct {
			width    int
			subtitle bool
		}{{700, true}, {220, false}, {700, true}} {
			tt.SetSize(step.width, 600)
			afterTitle, body := afterTitleAndAsBody(t, tt, phrase)
			if smaller := afterTitle.H < body.H; smaller != step.subtitle {
				t.Errorf("Text(…%s) at width %d: the phrase is %v high after the card's title and %v high as body text", size, step.width, afterTitle.H, body.H)
			}
		}
	}
}
