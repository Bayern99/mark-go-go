package genuikit

import (
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

func TestTheFormatShowsHowToNameAChartsSeries(t *testing.T) {
	text := Format()
	if !strings.Contains(text, `height?: number, seriesLabels?: {[key: string]: string})`) {
		t.Error("seriesLabels is not the last property of Chart's signature")
	}
	_, layout, _ := strings.Cut(text, "## Layout")
	layout, _, _ = strings.Cut(layout, "## Example")
	_, usage, found := strings.Cut(layout, "seriesLabels")
	start, end := strings.Index(usage, "Chart("), strings.Index(usage, "})")
	if !found || start < 0 || end < start {
		t.Fatalf("the layout rules show no Chart with seriesLabels:\n%s", layout)
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
