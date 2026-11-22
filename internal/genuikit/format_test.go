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

func TestTheFormatSizesATableForThePage(t *testing.T) {
	_, layout, _ := strings.Cut(Format(), "## Layout")
	layout, _, _ = strings.Cut(layout, "## Example")
	for _, want := range []string{"about 760 points wide", "about five columns", "more attributes than subjects, put the attributes in rows and the subjects in columns"} {
		if !strings.Contains(layout, want) {
			t.Errorf("the layout rules lack %q", want)
		}
	}
}
