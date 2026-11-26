package genui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	md "github.com/Bayern99/mark-go-go/internal/markdown"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

func object(values ...any) *openuilang.Object {
	o := openuilang.NewObject()
	for i := 0; i+1 < len(values); i += 2 {
		o.Set(values[i].(string), values[i+1])
	}
	return o
}

func element(kind string, values ...any) *openuilang.Element {
	return &openuilang.Element{Type: kind, Props: object(values...)}
}

func TestEveryCatalogComponentHasAnExporter(t *testing.T) {
	names := genuikit.ComponentNames()
	if len(names) != 50 {
		t.Fatalf("catalog has %d components, want 50", len(names))
	}
	for _, name := range names {
		if !supportedComponents[name] {
			t.Errorf("%s has no export renderer", name)
		}
	}
	if len(supportedComponents) != len(names) {
		t.Errorf("exporter has %d components, catalog has %d", len(supportedComponents), len(names))
	}
}

func TestEveryCatalogComponentReallyRenders(t *testing.T) {
	text := func(kind string) *openuilang.Element { return element(kind, "text", "Visible") }
	for _, name := range genuikit.ComponentNames() {
		t.Run(name, func(t *testing.T) {
			var root *openuilang.Element
			want := "Visible"
			switch name {
			case "Col", "Row", "Grid", "Card", "Carousel":
				root = element(name, "children", []any{text("Text")})
			case "Box":
				root = element(name, "children", []any{text("Text")}, "direction", "col")
			case "Divider":
				root, want = element(name), "<hr>"
			case "Spacer":
				root, want = element(name), "g-spacer"
			case "Title", "Text", "Caption", "Label", "Badge":
				root = text(name)
			case "Link":
				root = element(name, "text", "Visible", "url", "https://example.com")
			case "ListItem":
				root = element(name, "title", "Visible")
			case "List":
				root, want = element(name, "items", []any{"Visible"}), "g-list"
			case "Markdown":
				root, want = element(name, "text", "**Visible**"), "<strong>Visible</strong>"
			case "Quote":
				root = element(name, "text", "Visible")
			case "Code":
				root = element(name, "code", "Visible", "language", "go")
			case "Source":
				root, want = element(name, "sources", []any{"section"}, "children", []any{text("Text")}), "g-source-link"
			case "Table":
				root = element(name, "columns", []any{"Visible"}, "rows", []any{[]any{"Cell"}})
			case "Chart":
				root, want = element(name, "type", "bar", "data", []any{object("x", "Visible", "value", 1.0)}, "x", "x", "series", []any{"value"}), "<svg"
			case "Icon":
				root, want = element(name, "name", "export"), "g-icon"
			case "Image":
				root, want = element(name, "src", "https://example.com/image.png", "alt", "Visible"), `<img src="https://example.com/image.png"`
			case "Svg":
				root, want = element(name, "viewBox", "0 0 10 10", "children", []any{element("Rect", "width", 10.0, "height", 10.0)}), "<svg"
			case "G":
				root, want = element(name, "children", []any{}), "<g>"
			case "Circle":
				root, want = element(name, "cx", 1.0, "cy", 1.0, "r", 1.0), "<circle"
			case "Ellipse":
				root, want = element(name, "cx", 1.0, "cy", 1.0, "rx", 1.0, "ry", 1.0), "<ellipse"
			case "Rect":
				root, want = element(name, "width", 1.0, "height", 1.0), "<rect"
			case "Line":
				root, want = element(name, "x1", 0.0, "y1", 0.0, "x2", 1.0, "y2", 1.0), "<line"
			case "Path":
				root, want = element(name, "d", "M 0 0 L 1 1"), "<path"
			case "Polyline":
				root, want = element(name, "points", "0,0 1,1"), "<polyline"
			case "Polygon":
				root, want = element(name, "points", "0,0 1,1 1,0"), "<polygon"
			case "SvgText":
				root, want = element(name, "text", "Visible"), "<text"
			case "Popover":
				root = element(name, "trigger", element("Button", "label", "Visible"), "content", text("Text"))
			case "Button":
				root = element(name, "label", "Visible")
			case "IconButton":
				root = element(name, "icon", "export", "label", "Visible")
			case "Pressable":
				root = element(name, "children", []any{text("Text")})
			case "Form":
				root = element(name, "children", []any{text("Text")})
			case "Input", "TextArea", "DatePicker":
				root = element(name, "value", "Visible")
			case "Select", "RadioGroup", "SegmentedControl":
				root = element(name, "value", "Visible", "options", []any{"Visible"})
			case "Checkbox":
				root = element(name, "checked", true, "label", "Visible")
			case "Slider":
				root, want = element(name, "value", 5.0, "min", 0.0, "max", 10.0), `aria-valuenow="5"`
			case "Timer":
				root, want = element(name, "seconds", 65.0), "01:05"
			case "Flashcard":
				root = element(name, "front", "Visible", "back", "Answer")
			case "Celebration":
				root = element(name, "when", true)
			default:
				t.Fatalf("no minimal render input for %s", name)
			}
			result, err := Render(root, nil, export.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if name == "Celebration" {
				return
			}
			if !strings.Contains(string(result.Data), want) {
				t.Fatalf("real Render output is missing %q: %s", want, result.Data)
			}
		})
	}
}

func TestRepresentativeComponentsAreStatic(t *testing.T) {
	chart := element("Chart", "type", "bar", "data", []any{
		object("month", "Jan", "actual", 4.0, "budget", 5.0),
		object("month", "Feb", "actual", 6.0, "budget", 7.0),
	}, "x", "month", "series", []any{"actual", "budget"}, "seriesLabels", object("actual", "Actual", "budget", "Budget"))
	root := element("Col", "children", []any{
		element("Title", "text", "Report", "size", "xl"),
		element("Markdown", "text", "**Bold** prose"),
		element("Code", "code", "package main", "language", "go"),
		chart,
		element("Table", "columns", []any{"Name", "Value"}, "rows", []any{[]any{"Alpha", 2.0}, []any{"Beta", 3.0}}),
		element("Carousel", "children", []any{element("Text", "text", "One"), element("Text", "text", "Two")}),
		element("Flashcard", "front", "Question", "back", "Answer", "hint", "Hint"),
		element("Popover", "trigger", element("Button", "label", "More"), "content", element("Text", "text", "Aside")),
		element("Celebration", "when", true),
	})
	result, err := Render(root, nil, export.Options{SpaceName: "Wood", Dark: true, PrintLight: true})
	if err != nil {
		t.Fatal(err)
	}
	html := string(result.Data)
	for _, want := range []string{
		"Report", "<strong>Bold</strong>", "tok-keyword", "<svg", "Actual", "Budget", "Alpha", "Beta", `align="right"`,
		`class="g-carousel"`, "Question", "Answer", "Popover contents", "Aside", "@media print",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	for _, unwanted := range []string{"<script", "prefers-color-scheme", "confetti"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("output contains %q", unwanted)
		}
	}
}

func TestReaderNotesAndSourcesLinkBothWays(t *testing.T) {
	root := element("Col", "children", []any{
		element("Source", "sources", []any{"s1"}, "children", []any{element("Text", "text", "First")}),
		element("Source", "sources", []any{"s2"}, "children", []any{element("Text", "text", "Second")}),
	})
	notes := []md.Note{
		{Text: "first secret", SectionID: "shared-hash", SectionHeading: "Repeated", SectionOccurrence: 1},
		{Text: "second secret", SectionID: "shared-hash", SectionHeading: "Repeated", SectionOccurrence: 2},
	}
	without, err := Render(root, notes, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if text := string(without.Data); strings.Contains(text, "secret") || strings.Contains(text, "Reader Notes") {
		t.Fatal("disabled notes leaked into the export")
	}
	sources := map[string]SectionSource{
		"s1": {Heading: "Repeated", Occurrence: 1},
		"s2": {Heading: "Repeated", Occurrence: 2},
	}
	with, err := RenderSnapshot(root, nil, sources, notes, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	html := string(with.Data)
	for _, want := range []string{"first secret", "second secret", `id="notes-repeated-1"`, `id="notes-repeated-2"`, `href="#notes-repeated-1"`, `href="#notes-repeated-2"`, "Repeated (2)", "↩ Source"} {
		if !strings.Contains(html, want) {
			t.Errorf("notes output is missing %q", want)
		}
	}
	links := regexp.MustCompile(`href="#(source-repeated-[0-9]+-[0-9]+)"`).FindAllStringSubmatch(html, -1)
	if len(links) < 2 {
		t.Fatalf("source backlinks = %v", links)
	}
	for _, link := range links {
		if !strings.Contains(html, `id="`+link[1]+`"`) {
			t.Errorf("backlink target %s does not exist", link[1])
		}
	}
}

func TestSourceAndNoteAnchorsSurviveSlugCollisions(t *testing.T) {
	root := element("Col", "children", []any{
		element("Source", "sources", []any{"bang"}, "children", []any{element("Text", "text", "First")}),
		element("Source", "sources", []any{"question"}, "children", []any{element("Text", "text", "Second")}),
	})
	sources := map[string]SectionSource{"bang": {Heading: "A!", Occurrence: 1}, "question": {Heading: "A?", Occurrence: 1}}
	notes := []md.Note{{Text: "bang", SectionHeading: "A!", SectionOccurrence: 1}, {Text: "question", SectionHeading: "A?", SectionOccurrence: 1}}
	result, err := RenderSnapshot(root, nil, sources, notes, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	ids := regexp.MustCompile(`class="g-source-link" id="([^"]+)"`).FindAllStringSubmatch(text, -1)
	groups := regexp.MustCompile(`class="g-note-group" id="([^"]+)"`).FindAllStringSubmatch(text, -1)
	if len(ids) != 2 || ids[0][1] == ids[1][1] || len(groups) != 2 || groups[0][1] == groups[1][1] {
		t.Fatalf("slug collisions produced duplicate anchors: sources=%v groups=%v", ids, groups)
	}
}

func TestSessionBindingsRenderTheirCurrentValues(t *testing.T) {
	library, err := genuikit.Library()
	if err != nil {
		t.Fatal(err)
	}
	program := library.Parse("root = Col([Input($name, \"Name\"), Slider($count, 0, 10, 1, \"Count\"), SegmentedControl($mode, [\"A\", \"B\"])])\n$name = \"Ada\"\n$count = 6\n$mode = \"B\"")
	session := openuilang.NewSession()
	session.Load(program)
	session.Set("$name", "Grace")
	session.Set("$count", 8.0)
	result, err := RenderSession(session, nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	html := string(result.Data)
	for _, want := range []string{"Grace", "80%", `aria-valuenow="8"`, `class="g-segment g-selected">B`} {
		if !strings.Contains(html, want) {
			t.Errorf("binding output is missing %q: %s", want, html)
		}
	}
}

func TestPhosphorFontIsOnlyEmbeddedWhenIconsAreUsed(t *testing.T) {
	plain, err := Render(element("Text", "text", "Plain"), nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain.Data), "@font-face") {
		t.Fatal("plain text embeds Phosphor")
	}
	icon, err := Render(element("Icon", "name", "export"), nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(icon.Data), "@font-face") || !strings.Contains(string(icon.Data), "data:font/ttf;base64,") || !strings.Contains(string(icon.Data), "\ueaf0") {
		t.Fatal("icon export did not inline its glyph and font")
	}
}

func TestAllChartKindsProduceSVG(t *testing.T) {
	for _, kind := range []string{"bar", "line", "area", "pie", "donut", "scatter"} {
		t.Run(kind, func(t *testing.T) {
			root := element("Chart", "type", kind, "data", []any{
				object("x", "A", "one", 2.0, "two", 3.0), object("x", "B", "one", 4.0, "two", 1.0),
			}, "x", "x", "series", []any{"one", "two"}, "seriesLabels", object("one", "One", "two", "Two"))
			result, err := Render(root, nil, export.Options{})
			if err != nil {
				t.Fatal(err)
			}
			html := string(result.Data)
			if !strings.Contains(html, "<svg") || !strings.Contains(html, "var(--chart-1)") {
				t.Fatalf("chart has no themed SVG: %s", html)
			}
			if kind != "pie" && kind != "donut" && (!strings.Contains(html, "One") || !strings.Contains(html, "Two")) {
				t.Fatal("chart legend is missing")
			}
		})
	}
}

func TestSVGPrimitivesAreEscaped(t *testing.T) {
	root := element("Svg", "viewBox", "0 0 100 100", "label", "Diagram", "children", []any{
		element("Circle", "cx", 20.0, "cy", 20.0, "r", 10.0, "fill", "chart1"),
		element("Path", "d", "M 0 0 L 10 10", "stroke", "accent"),
		element("SvgText", "text", "<safe>", "x", 5.0, "y", 10.0),
	})
	result, err := Render(root, nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	html := string(result.Data)
	for _, want := range []string{"<circle", "<path", "&lt;safe&gt;", `fill="var(--chart-1)"`, `stroke="var(--accent)"`} {
		if !strings.Contains(html, want) {
			t.Errorf("SVG output is missing %q", want)
		}
	}
}

func TestButtonVariantIsAllowlisted(t *testing.T) {
	result, err := Render(element("Button", "label", "Safe", "variant", `evil\" onclick=\"alert(1)`), nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	if strings.Contains(text, "g-button-evil") || strings.Contains(text, "onclick") {
		t.Fatalf("unsafe button class escaped the whitelist: %s", text)
	}
	for _, variant := range []string{"outline", "soft", "ghost"} {
		result, err := Render(element("Button", "label", "Safe", "variant", variant), nil, export.Options{})
		if err != nil || !strings.Contains(string(result.Data), "g-button-"+variant) {
			t.Errorf("variant %q was not preserved: %s, %v", variant, result.Data, err)
		}
	}
}

func TestTypedNilCollectionsDoNotPanic(t *testing.T) {
	var nilObject *openuilang.Object
	var nilElement *openuilang.Element
	root := element("Col", "children", []any{
		element("List", "items", []any{nilObject, nilElement}),
		element("Select", "value", "A", "options", []any{nilObject}),
		element("RadioGroup", "value", "A", "options", []any{nilObject}),
		element("Slider", "value", 1.0, "min", 0.0, "max", 2.0, "marks", []any{nilObject}),
	})
	result, err := Render(root, nil, export.Options{})
	if err != nil || !strings.Contains(string(result.Data), "g-slider-track") {
		t.Fatalf("typed nil render: %v, %s", err, result.Data)
	}
}

func TestStackedAreaUsesThePreviousSeriesAsBaseline(t *testing.T) {
	model := chartModel{kind: "area", stacked: true, labels: []string{"A"}, series: []chartSeries{
		{key: "one", values: []float64{2}},
		{key: "two", values: []float64{3}},
	}}
	scale := newChartScale(model)
	var out strings.Builder
	lineChart(&out, model, scale, scale.y(0))
	x := numberString(scale.left + scale.width/2)
	second := `points="` + x + `,` + numberString(scale.y(5)) + ` ` + x + `,` + numberString(scale.y(2)) + `"`
	if !strings.Contains(out.String(), second) {
		t.Fatalf("second stacked area does not start at the first series: %s", out.String())
	}
}

func TestPiePercentUsesTheSliceTotal(t *testing.T) {
	root := element("Chart", "type", "pie", "format", "percent", "data", []any{
		object("x", "A", "value", 2.0), object("x", "B", "value", 6.0),
	}, "x", "x", "series", []any{"value"})
	result, err := Render(root, nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	for _, want := range []string{"A: 25%", "B: 75%"} {
		if !strings.Contains(text, want) {
			t.Errorf("pie output is missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{"200%", "600%"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("pie output contains %q", unwanted)
		}
	}
}

func TestSVGTransformCompositionAndExplicitNoneStroke(t *testing.T) {
	root := element("Svg", "viewBox", "0 0 100 100", "children", []any{
		element("G", "transform", "translate(10 20) rotate(30 50 50) scale(2)", "children", []any{
			element("Rect", "width", 10.0, "height", 10.0, "stroke", "none", "strokeWidth", 4.0),
		}),
	})
	result, err := Render(root, nil, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	for _, want := range []string{`transform="translate(10 20) rotate(30 50 50) scale(2)"`, `stroke="none"`, `stroke-width="4"`} {
		if !strings.Contains(text, want) {
			t.Errorf("SVG output is missing %q: %s", want, text)
		}
	}
}

func TestTextGolden(t *testing.T) {
	r := &renderer{}
	var out strings.Builder
	r.value(&out, element("Text", "text", "Hello", "size", "sm", "weight", "semibold"), 0)
	want := `<p class="g-text" style="font-size:14px;font-weight:600">Hello</p>`
	if out.String() != want {
		t.Fatalf("text = %q, want %q", out.String(), want)
	}
}

func TestSourceWithoutNotesHasNoBrokenLinks(t *testing.T) {
	root := element("Col", "children", []any{
		element("Source", "sources", []any{"first", "second"}, "children", []any{element("Text", "text", "Combined")}),
	})
	sources := map[string]SectionSource{
		"first":  {Heading: "Repeat", Occurrence: 1},
		"second": {Heading: "Repeat", Occurrence: 2},
	}
	notes := []md.Note{{Text: "only first note", SectionHeading: "Repeat", SectionOccurrence: 1}}
	result, err := RenderSnapshot(root, nil, sources, notes, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	if !strings.Contains(text, `href="#notes-repeat-1"`) || !strings.Contains(text, `<span class="g-source-link">§ Repeat (2)</span>`) {
		t.Fatalf("mixed source badges = %s", text)
	}
	for _, match := range regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(text, -1) {
		if !strings.Contains(text, `id="`+match[1]+`"`) {
			t.Errorf("broken fragment href #%s in output", match[1])
		}
	}
}
