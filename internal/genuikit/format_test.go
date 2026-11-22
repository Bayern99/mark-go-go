package genuikit

import (
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

func slidersIn(value any, into []*openuilang.Element) []*openuilang.Element {
	switch value := value.(type) {
	case *openuilang.Element:
		if value.Type == "Slider" {
			into = append(into, value)
		}
		for _, key := range value.Props.Keys {
			into = slidersIn(value.Props.Values[key], into)
		}
	case []any:
		for _, item := range value {
			into = slidersIn(item, into)
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
	sliders := slidersIn(root, nil)
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
