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
