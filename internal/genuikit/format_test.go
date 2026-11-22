package genuikit

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

func TestTheFormatListsEveryComponentAndFunctionOfTheCatalog(t *testing.T) {
	text := Format()
	for _, want := range []string{"- Col(children: Node[]", "- Slider(value: $binding<number>", "- Source(sources: string[], children: Node[])", "- @Format(", "- @Each("} {
		if !strings.Contains(text, want) {
			t.Errorf("the format lacks %q", want)
		}
	}
	if strings.Contains(text, "{{") {
		t.Error("a placeholder is left in the format")
	}
}

func TestTheFormatsExampleIsAValidView(t *testing.T) {
	_, example, _ := strings.Cut(Format(), "## Example")
	_, program, _ := strings.Cut(example, "\n\n")
	_, program, _ = strings.Cut(program, "\n\n")
	library, err := Library()
	if err != nil {
		t.Fatal(err)
	}
	parsed := library.Parse(program)
	session := openuilang.NewSession()
	session.Load(parsed)
	root, problems := session.Render()
	if root == nil || len(parsed.Errors)+len(problems)+len(parsed.Unresolved) > 0 {
		t.Errorf("root %v, errors %v %v, unresolved %v", root != nil, parsed.Errors, problems, parsed.Unresolved)
	}
}

func TestTheLayoutRulesCapTheMinimumWidthOfThreeCardsSideBySide(t *testing.T) {
	if rules := layoutRules(t); !strings.Contains(rules, "Three cards sit side by side only when the Grid's minChildWidth is 240 or less") {
		t.Errorf("the layout rules do not cap the minimum width of three cards side by side:\n%s", rules)
	}
}

func layoutRules(t *testing.T) string {
	t.Helper()
	_, rest, found := strings.Cut(Format(), "## Layout")
	rules, _, ended := strings.Cut(rest, "## Example")
	if !found || !ended {
		t.Fatal("the format has no Layout section before its Example")
	}
	return rules
}

func TestTheLayoutRulesShowOneSetOfFiguresInOneComponent(t *testing.T) {
	rules := layoutRules(t)
	for _, want := range []string{"\n- Show one set of figures in one component", "only when it adds columns"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}

func exampleOfTheFormat(t *testing.T) (description string, session *openuilang.Session, root *openuilang.Element) {
	t.Helper()
	_, example, _ := strings.Cut(Format(), "## Example")
	_, example, _ = strings.Cut(example, "\n\n")
	description, program, _ := strings.Cut(example, "\n\n")
	library, err := Library()
	if err != nil {
		t.Fatal(err)
	}
	session = openuilang.NewSession()
	session.Load(library.Parse(program))
	root, _ = session.Render()
	if root == nil {
		t.Fatal("the example renders nothing")
	}
	return description, session, root
}

var figure = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)

func figuresIn(text string) []float64 {
	var out []float64
	for _, match := range figure.FindAllString(text, -1) {
		if number, err := strconv.ParseFloat(strings.ReplaceAll(match, ",", ""), 64); err == nil {
			out = append(out, number)
		}
	}
	return out
}

type shownFigures struct {
	plotted map[float64]bool
	written map[float64]string
}

func (shown *shownFigures) collect(value any, inChart bool) {
	switch value := value.(type) {
	case *openuilang.Element:
		for _, key := range value.Props.Keys {
			shown.collect(value.Props.Values[key], inChart || value.Type == "Chart" && key == "data")
		}
	case *openuilang.Object:
		for _, key := range value.Keys {
			shown.collect(value.Values[key], inChart)
		}
	case []any:
		for _, item := range value {
			shown.collect(item, inChart)
		}
	case float64:
		if inChart {
			shown.plotted[value] = true
		}
	case string:
		if !inChart {
			for _, number := range figuresIn(value) {
				shown.written[number] = value
			}
		}
	}
}

func TestTheFormatsExampleWritesOutNoFigureItsChartPlots(t *testing.T) {
	_, _, root := exampleOfTheFormat(t)
	shown := shownFigures{plotted: map[float64]bool{}, written: map[float64]string{}}
	shown.collect(root, false)
	if len(shown.plotted) == 0 {
		t.Fatal("the example plots no figures")
	}
	for number := range shown.plotted {
		if text, repeated := shown.written[number]; repeated {
			t.Errorf("the example plots %v and also writes it in %q", number, text)
		}
	}
}

func TestTheLayoutRulesTakeAControlsRangeAndDefaultFromTheDocument(t *testing.T) {
	rules := layoutRules(t)
	for _, want := range []string{"\n- Take a control's range and default from the document", "leave the control out"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}

func componentsIn(value any, component string, into []*openuilang.Element) []*openuilang.Element {
	switch value := value.(type) {
	case *openuilang.Element:
		if value.Type == component {
			into = append(into, value)
		}
		for _, key := range value.Props.Keys {
			into = componentsIn(value.Props.Values[key], component, into)
		}
	case []any:
		for _, item := range value {
			into = componentsIn(item, component, into)
		}
	}
	return into
}

func TestTheFormatsExampleTakesItsControlsRangeAndDefaultFromTheDocumentItDescribes(t *testing.T) {
	description, session, root := exampleOfTheFormat(t)
	stated := map[any]bool{}
	for _, number := range figuresIn(description) {
		stated[number] = true
	}
	sliders := componentsIn(root, "Slider", nil)
	if len(sliders) == 0 {
		t.Fatal("the example has no slider")
	}
	for _, slider := range sliders {
		least, _ := slider.Props.Get("min")
		greatest, _ := slider.Props.Get("max")
		binding, _ := slider.Props.Get("value")
		variable, bound := binding.(*openuilang.Binding)
		if !bound {
			t.Fatalf("the slider's value is %T, not a variable", binding)
		}
		initial, _ := session.Get(variable.Target)
		for name, value := range map[string]any{"least value": least, "greatest value": greatest, "default": initial} {
			if !stated[value] {
				t.Errorf("the slider's %s %v is not a number of the document described as %q", name, value, description)
			}
		}
	}
}

func eachElement(value any, visit func(*openuilang.Element)) {
	switch v := value.(type) {
	case *openuilang.Element:
		if v == nil {
			return
		}
		visit(v)
		eachElement(v.Props, visit)
	case *openuilang.Object:
		if v == nil {
			return
		}
		for _, key := range v.Keys {
			eachElement(v.Values[key], visit)
		}
	case []any:
		for _, item := range v {
			eachElement(item, visit)
		}
	}
}

func TestTheFormatsExampleMarksItsTimelineWithDatesNotNumbers(t *testing.T) {
	_, example, _ := strings.Cut(Format(), "## Example")
	_, program, _ := strings.Cut(example, "\n\n")
	_, program, _ = strings.Cut(program, "\n\n")
	library, err := Library()
	if err != nil {
		t.Fatal(err)
	}
	session := openuilang.NewSession()
	session.Load(library.Parse(program))
	root, _ := session.Render()
	var markers []string
	eachElement(root, func(list *openuilang.Element) {
		if on, _ := list.Props.Get("timeline"); list.Type != "List" || on != true {
			return
		}
		eachElement(list.Props, func(item *openuilang.Element) {
			if item.Type != "ListItem" {
				return
			}
			marker, _ := item.Props.Get("marker")
			text, _ := marker.(string)
			markers = append(markers, text)
		})
	})
	if len(markers) == 0 {
		t.Fatal("the example has no timeline")
	}
	for _, marker := range markers {
		if strings.Trim(marker, "0123456789. ") == "" {
			t.Errorf("a timeline entry of the example is marked %q", marker)
		}
	}
}

func TestTheFormatTellsTimelineMarkersFromNumberedSteps(t *testing.T) {
	_, layout, _ := strings.Cut(Format(), "## Layout")
	layout, _, _ = strings.Cut(layout, "## Example")
	for _, want := range []string{"third argument of ListItem", "time or date", `List([...], "number")`} {
		if !strings.Contains(layout, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}

func TestTheFormatShowsHowToNameAChartsSeries(t *testing.T) {
	if !strings.Contains(Format(), `height?: number, seriesLabels?: {[key: string]: string})`) {
		t.Error("seriesLabels is not the last property of Chart's signature")
	}
	rules := layoutRules(t)
	_, usage, found := strings.Cut(rules, "seriesLabels")
	start, end := strings.Index(usage, "Chart("), strings.Index(usage, "})")
	if !found || start < 0 || end < start {
		t.Fatalf("the layout rules show no Chart with seriesLabels:\n%s", rules)
	}
	library, err := Library()
	if err != nil {
		t.Fatal(err)
	}
	parsed := library.Parse("root = Col([" + usage[start:end+2] + "])\nrows = [{channel: \"Search\", budget: 420, actual: 398}]")
	session := openuilang.NewSession()
	session.Load(parsed)
	root, problems := session.Render()
	if root == nil || len(parsed.Errors)+len(problems)+len(parsed.Unresolved) > 0 {
		t.Errorf("%s: root %v, errors %v %v, unresolved %v", usage[start:end+2], root != nil, parsed.Errors, problems, parsed.Unresolved)
	}
}

func TestTheFormatsExampleNamesEverySeriesOfAChartThatShowsALegend(t *testing.T) {
	_, _, root := exampleOfTheFormat(t)
	legends := 0
	for _, chart := range componentsIn(root, "Chart", nil) {
		series, _ := chart.Props.Values["series"].([]any)
		if len(series) < 2 {
			continue
		}
		legends++
		names, _ := chart.Props.Values["seriesLabels"].(*openuilang.Object)
		for _, key := range series {
			key, _ := key.(string)
			if names == nil {
				t.Errorf("the example's chart of %v has no seriesLabels", series)
				break
			}
			if name, _ := names.Values[key].(string); name == "" || name == key {
				t.Errorf("the example's chart names its series %q %q", key, name)
			}
		}
	}
	if legends == 0 {
		t.Fatal("the example has no chart with several series")
	}
}

func TestTheFormatNamesTheLargestTextSizeAndWhereBigNumbersGo(t *testing.T) {
	var top struct {
		Components map[string]struct{ Signature string }
	}
	if err := json.Unmarshal(catalog, &top); err != nil {
		t.Fatal(err)
	}
	_, sizes, _ := strings.Cut(top.Components["Text"].Signature, "size?: ")
	sizes, _, _ = strings.Cut(sizes, ", ")
	choices := strings.Split(sizes, " | ")
	largest := choices[len(choices)-1]
	rules := layoutRules(t)
	for _, want := range []string{"Text sizes stop at " + largest + ": a big number is a Title", "its subtitle: one short phrase that fits a single line"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}

func TestTheLayoutRulesSizeATableForThePage(t *testing.T) {
	rules := layoutRules(t)
	for _, want := range []string{"about 760 points wide", "about five columns", "more attributes than subjects, put the attributes in rows and the subjects in columns"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}

func sourceRules(t *testing.T) string {
	t.Helper()
	_, rest, found := strings.Cut(Format(), "## Sources")
	rules, _, ended := strings.Cut(rest, "## Syntax")
	if !found || !ended {
		t.Fatal("the format has no Sources section before its Syntax")
	}
	return rules
}

func TestTheSourceRulesKeepASourcesContentInOneStatement(t *testing.T) {
	rules := sourceRules(t)
	for _, want := range []string{"\n- Give each Source its own statement and write all of its content in that one statement", "An update then replaces that statement alone"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the source rules lack %q", want)
		}
	}
}

func TestTheSourceRulesKeepThePageTitleOffTheFirstLevelHeading(t *testing.T) {
	rules := sourceRules(t)
	for _, want := range []string{"\n- Leave the page title outside any Source", "first-level heading", "spans the whole document"} {
		if !strings.Contains(rules, want) {
			t.Errorf("the source rules lack %q", want)
		}
	}
}

func TestTheFormatsExampleWritesEachSourcesContentInTheSourcesOwnStatement(t *testing.T) {
	_, _, root := exampleOfTheFormat(t)
	sources := 0
	eachElement(root, func(source *openuilang.Element) {
		if source.Type != "Source" {
			return
		}
		sources++
		if source.StatementID == "" {
			t.Error("a Source of the example has no statement of its own")
		}
		eachElement(source.Props, func(inner *openuilang.Element) {
			if inner.StatementID != "" && inner.StatementID != source.StatementID {
				t.Errorf("the Source %q of the example keeps its %s in the statement %q", source.StatementID, inner.Type, inner.StatementID)
			}
		})
	})
	if sources == 0 {
		t.Fatal("the example has no Source")
	}
}

func TestTheFormatsExampleKeepsItsPageTitleOutOfEverySource(t *testing.T) {
	_, _, root := exampleOfTheFormat(t)
	sourced := map[*openuilang.Element]bool{}
	var titles []*openuilang.Element
	eachElement(root, func(element *openuilang.Element) {
		switch element.Type {
		case "Title":
			titles = append(titles, element)
		case "Source":
			eachElement(element.Props, func(inner *openuilang.Element) { sourced[inner] = true })
		}
	})
	if len(titles) == 0 {
		t.Fatal("the example has no title")
	}
	if text, _ := titles[0].Props.Get("text"); sourced[titles[0]] {
		t.Errorf("the example's page title %q sits inside a Source", text)
	}
}

func TestTheFormatLeavesDeletingUnreferencedStatementsToTheApp(t *testing.T) {
	_, rest, found := strings.Cut(Format(), "## Message")
	message, _, ended := strings.Cut(rest, "## Sources")
	if !found || !ended {
		t.Fatal("the format has no Message section before its Sources")
	}
	for _, want := range []string{"The app deletes the statements an update leaves unreferenced", "name = null", "never write such a statement yourself"} {
		if !strings.Contains(message, want) {
			t.Errorf("the message rules lack %q", want)
		}
	}
}
