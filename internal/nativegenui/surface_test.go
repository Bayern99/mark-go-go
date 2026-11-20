package nativegenui

import (
	"os"
	"strings"
	"testing"
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
	if err != nil || s.Nodes["future"].Component != "Future" || s.Nodes["body"].Text != "Keep me" {
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
	if len(surface.Nodes["list"].Children) != 0 || surface.Nodes["saved"].Text != "Saved report" {
		t.Fatalf("unbound template hid the saved report: %+v", surface)
	}
}
