package nativegenui

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestOpenPublishedCatalogExample(t *testing.T) {
	doc, err := os.ReadFile("../../docs/genui/view-format.md")
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := strings.Cut(string(doc), "\x60\x60\x60json\n")
	data, _, _ := strings.Cut(body, "\n\x60\x60\x60")
	s, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Nodes["root"].Component != "Tabs" || len(s.Nodes["compare"].Rows) != 3 || s.Sources["overview"].Heading != "Overview" {
		t.Fatalf("lost catalog or references: %+v", s)
	}
}

func TestOpenKeepsUnknownComponentsInertAndChecksTheirSiblings(t *testing.T) {
	const view = `{"version":2,"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Column","children":["future","body"]},{"id":"future","component":"Future","items":{"unrecognised":"shape"}},{"id":"body","component":"Text","text":"Keep me"}]}}]}`
	s, err := Open(view)
	if err != nil || s.Nodes["future"].Component != "Future" || s.Nodes["body"].Text.String() != "Keep me" {
		t.Fatalf("open existing view: %v %v", s, err)
	}
	for _, bad := range []string{
		strings.Replace(view, `"text":"Keep me"`, `"text":"Keep me","bad":true`, 1),
		strings.Replace(view, `["future","body"]`, `["root","body"]`, 1),
	} {
		if _, err := Open(bad); err == nil {
			t.Fatal("invalid sibling or cycle was accepted")
		}
	}
}

func TestOpenViewWithUnboundTemplateAndLiteralSibling(t *testing.T) {
	view := `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Column","children":["list","title","saved"]},{"id":"list","component":"Column","children":{"componentId":"template","path":"/rows"}},{"id":"template","component":"Text","text":{"path":"/item"}},{"id":"title","component":"Text","text":{"path":"/title"}},{"id":"saved","component":"Text","text":"Saved report"}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	if len(surface.Nodes["list"].Children.IDs) != 0 || surface.Nodes["list"].Children.Template != nil || surface.Nodes["saved"].Text.String() != "Saved report" {
		t.Fatalf("unbound template hid the saved report: %+v", surface)
	}
}

func TestOpenExpandsReadOnlyChildListTemplates(t *testing.T) {
	const view = `{"version":2,"sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"items":[{"name":"Alpha"},{"name":"Beta"}]},"components":[{"id":"root","component":"Column","children":["rows"]},{"id":"rows","component":"Column","children":{"componentId":"row","path":"/items"}},{"id":"row","component":"Column","children":["label","points","functionText"]},{"id":"label","component":"Text","text":{"@path":"name"}},{"id":"points","component":"KeyPoints","title":"Source details","items":["A source-bound point."],"sources":["overview"]},{"id":"functionText","component":"Text","text":{"@call":"formatString","args":{"value":"This stays inert."}}}]}}]}`
	first, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	rowIDs := first.Nodes["rows"].Children.IDs
	if len(rowIDs) != 2 || rowIDs[0] != "row-[/items/0]" || rowIDs[1] != "row-[/items/1]" {
		t.Fatalf("expanded row identities = %v", rowIDs)
	}
	labelIDs := []string{"label-[/items/0]", "label-[/items/1]"}
	pointIDs := []string{"points-[/items/0]", "points-[/items/1]"}
	functionIDs := []string{"functionText-[/items/0]", "functionText-[/items/1]"}
	for i, name := range []string{"Alpha", "Beta"} {
		rowID := rowIDs[i]
		labelID := labelIDs[i]
		label, ok := first.Nodes[labelID]
		if !ok || label.Text.String() != name || label.ComponentID != "label" {
			t.Errorf("row %q label = %+v, want %q from template component label", rowID, label, name)
		}
		pointsID := pointIDs[i]
		points, ok := first.Nodes[pointsID]
		if !ok || len(points.Sources) != 1 || points.Sources[0] != "overview" || first.Sources["overview"].Heading != "Overview" {
			t.Errorf("row %q source mapping = %+v, source = %+v", rowID, points.Sources, first.Sources["overview"])
		}
		functionID := functionIDs[i]
		if got := first.Nodes[functionID].Text.String(); got != "" {
			t.Errorf("renderer function produced text %q", got)
		}
		if _, ok := second.Nodes[labelID]; !ok {
			t.Errorf("row identity %q changed when the same view was reopened", labelID)
		}
	}
	renderedSources := []string{}
	var reader Reading
	reader.Sources = func(_ *ui.Context, node Node) {
		if len(node.Sources) > 0 {
			renderedSources = append(renderedSources, node.ID)
		}
	}
	reader.Load(first)
	tt := ui.NewTester(reader.View, 600, 400)
	if _, ok := tt.Access("Alpha"); !ok {
		t.Fatal("first generated row was not visible in the native reader")
	}
	if _, ok := tt.Access("Beta"); !ok {
		t.Fatal("second generated row was not visible in the native reader")
	}
	if len(renderedSources) != 2 || renderedSources[0] == renderedSources[1] {
		t.Fatalf("source controls were not bound to distinct row instances: %v", renderedSources)
	}
}

func TestOpenResolvesReadOnlyA2UIDataBindingsAndComponentUpdates(t *testing.T) {
	const view = `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"content":{"text":"Initial","updatedText":"Updated"},"labels":{"tabs":["Summary","Sections"],"label":"Reading","updatedLabel":"Updated label","description":"Initial description","updatedDescription":"Updated description","hidden":true},"count":42},"components":[{"id":"root","component":"Column","children":["tabs","aria","count"]},{"id":"tabs","component":"Tabs","tabs":[{"title":{"@path":"/labels/tabs/0"},"child":"body"}]},{"id":"body","component":"Text","text":{"@path":"/content/text"}},{"id":"aria","component":"Column","children":["detail"],"accessibility":{"label":{"@path":"/labels/label"},"description":{"@path":"/labels/description"},"hidden":{"@path":"/labels/hidden"}}},{"id":"detail","component":"Text","text":"Details"},{"id":"count","component":"Text","text":{"@path":"/count"}}]}},{"version":"1.0","updateComponents":{"surfaceId":"v","components":[{"id":"tabs","component":"Tabs","tabs":[{"title":{"@path":"/labels/tabs/1"},"child":"body"}]},{"id":"body","component":"Text","text":{"@path":"/content/updatedText"}},{"id":"aria","component":"Column","children":["detail"],"accessibility":{"label":{"@path":"/labels/updatedLabel"},"description":{"@path":"/labels/updatedDescription"},"hidden":{"@path":"/labels/hidden"}}}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	if got := surface.Nodes["body"].Text.String(); got != "Updated" {
		t.Errorf("bound Text = %q, want updated data; nodes: %+v", got, surface.Nodes)
	}
	if got := surface.Nodes["tabs"].Tabs[0].Title.String(); got != "Sections" {
		t.Errorf("bound tab title = %q, want updated data", got)
	}
	accessibility := surface.Nodes["aria"].Accessibility
	if accessibility.Label.String() != "Updated label" || accessibility.Description.String() != "Updated description" {
		t.Errorf("bound accessibility = %+v, want updated label and description", accessibility)
	}
	if string(accessibility.Hidden) != "true" {
		t.Errorf("bound accessibility hidden = %s, want true", accessibility.Hidden)
	}
	if got := surface.Nodes["count"].Text.String(); got != "42" {
		t.Errorf("bound numeric Text = %q, want 42", got)
	}
	if got := surface.Nodes["detail"].Text.String(); got != "Details" {
		t.Errorf("literal Text = %q, want Details", got)
	}
}
