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

const fixedMarkerColumnStepStart = 107

func timeline(t *testing.T, width int, steps string) (*ui.Tester, float32) {
	t.Helper()
	tt, _ := shown(t, statementView(t, `root = Col([Text("Before the steps"), List([`+steps+`], null, true)])`), width, nil)
	edge, ok := tt.Find("Before the steps")
	if !ok {
		t.Fatal("the text before the steps is not shown")
	}
	return tt, edge.X
}

func stepStart(t *testing.T, tt *ui.Tester, edge float32, title string) float32 {
	t.Helper()
	box, ok := tt.Find(title)
	if !ok {
		t.Fatalf("%q is not shown", title)
	}
	return box.X - edge
}

func oneLineMarker(t *testing.T, width int) float32 {
	t.Helper()
	tt, _ := timeline(t, width, `ListItem("Only", "One step.", "1")`)
	box, ok := tt.Find("1")
	if !ok || box.H == 0 {
		t.Fatalf("the marker is %v", box)
	}
	return box.H
}

func TestTimelineMarkerColumnIsAsWideAsItsMarkers(t *testing.T) {
	for _, width := range []int{900, 320} {
		tt, edge := timeline(t, width, `ListItem("Copy", "Mirror every bucket.", "1"), ListItem("Switch reads", "Serve reads from object storage.", "2"), ListItem("Retire disks", "After 30 days without errors.", "3")`)
		if marker, ok := tt.Find("1"); !ok || marker.X != edge {
			t.Errorf("width %v: the marker is at %v, the list starts at %v", width, marker, edge)
		}
		digits := stepStart(t, tt, edge, "Copy")
		if digits > fixedMarkerColumnStepStart/2 {
			t.Errorf("width %v: steps with one-digit markers start %v points into the list, %v with the fixed column", width, digits, fixedMarkerColumnStepStart)
		}
		for _, title := range []string{"Switch reads", "Retire disks"} {
			if at := stepStart(t, tt, edge, title); at != digits {
				t.Errorf("width %v: %q starts %v points in, the first step %v", width, title, at, digits)
			}
		}
	}
}

func TestTimelineDateMarkersStayWholeOnOneLine(t *testing.T) {
	dates := []string{"May 2", "Mar 14", "3月14日"}
	for _, width := range []int{900, 320} {
		line := oneLineMarker(t, width)
		digitTimeline, digitEdge := timeline(t, width, `ListItem("Only", "One step.", "1")`)
		digits := stepStart(t, digitTimeline, digitEdge, "Only")
		tt, edge := timeline(t, width, `ListItem("Kickoff", "The team meets.", "May 2"), ListItem("Freeze", "No new work lands.", "Mar 14"), ListItem("发布", "对外公布。", "3月14日")`)
		steps := stepStart(t, tt, edge, "Kickoff")
		for _, title := range []string{"Freeze", "发布"} {
			if at := stepStart(t, tt, edge, title); at != steps {
				t.Errorf("width %v: %q starts %v points in, the first step %v", width, title, at, steps)
			}
		}
		for _, date := range dates {
			box, ok := tt.Find(date)
			if !ok || box.W == 0 {
				t.Fatalf("width %v: %q is not shown", width, date)
			}
			if box.H != line {
				t.Errorf("width %v: %q is %v tall, a one-line marker %v", width, date, box.H, line)
			}
			if box.X != edge || box.X+box.W > edge+steps {
				t.Errorf("width %v: %q spans %v to %v, the list starts at %v and its steps at %v", width, date, box.X, box.X+box.W, edge, edge+steps)
			}
		}
		if steps <= digits || steps >= fixedMarkerColumnStepStart {
			t.Errorf("width %v: steps with dates start %v points in, %v with one digit and %v with the fixed column", width, steps, digits, fixedMarkerColumnStepStart)
		}
	}
}

func TestTimelineMarkerWiderThanTheFixedColumnWrapsInsideIt(t *testing.T) {
	for _, width := range []int{900, 320} {
		line := oneLineMarker(t, width)
		tt, edge := timeline(t, width, `ListItem("Freeze", "No new work lands.", "September 30, 2026 at 02:00"), ListItem("Ship", "Released to everyone.", "1")`)
		box, ok := tt.Find("September 30, 2026 at 02:00")
		if !ok || box.H <= line {
			t.Errorf("width %v: the long marker is %v, one line is %v tall", width, box, line)
		}
		for _, title := range []string{"Freeze", "Ship"} {
			if at := stepStart(t, tt, edge, title); at != fixedMarkerColumnStepStart {
				t.Errorf("width %v: %q starts %v points in, %v with the fixed column", width, title, at, fixedMarkerColumnStepStart)
			}
		}
		if box.X+box.W > edge+stepStart(t, tt, edge, "Freeze") {
			t.Errorf("width %v: the long marker ends at %v, its step starts at %v", width, box.X+box.W, edge+stepStart(t, tt, edge, "Freeze"))
		}
	}
}

func TestTimelineWithoutMarkersHasNoMarkerColumn(t *testing.T) {
	for _, width := range []int{900, 320} {
		tt, edge := timeline(t, width, `ListItem("Copy", "Mirror every bucket.", "1"), ListItem("Switch reads", "Serve reads from object storage.", "2")`)
		marked := stepStart(t, tt, edge, "Copy")
		for name, steps := range map[string]string{
			"entries without a marker": `ListItem("Copy", "Mirror every bucket."), ListItem("Switch reads", "Serve reads from object storage.", "")`,
			"plain strings":            `"Copy", "Switch reads"`,
		} {
			tt, edge := timeline(t, width, steps)
			for _, title := range []string{"Copy", "Switch reads"} {
				if at := stepStart(t, tt, edge, title); at >= marked || at > 24 {
					t.Errorf("width %v, %s: %q starts %v points in, %v with one-digit markers", width, name, title, at, marked)
				}
			}
		}
	}
}
