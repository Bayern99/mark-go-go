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
	const view = `{"version":2,"sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"items":[{"name":"Alpha"},{"name":"Beta"}]},"components":[{"id":"root","component":"Column","children":["rows"]},{"id":"rows","component":"Column","children":{"componentId":"row","path":"/items"}},{"id":"row","component":"Column","children":["label","points","functionText"]},{"id":"label","component":"Text","text":{"@path":"name"}},{"id":"points","component":"KeyPoints","title":"Source details","items":["A source-bound point."],"sources":["overview"]},{"id":"functionText","component":"Text","text":{"@call":"openUrl","args":{"url":"https://example.com"}}}]}}]}`
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

func TestOpenResolvesLegacyPathAndCallAliases(t *testing.T) {
	const view = `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"title":"Alias title","body":"Alias body","description":"Alias description","hidden":true},"components":[{"id":"root","component":"Column","children":["body","tabs","prompt","aria"]},{"id":"body","component":"Text","text":{"path":"/body"}},{"id":"tabs","component":"Tabs","tabs":[{"title":{"path":"/title"},"child":"tabBody"}]},{"id":"tabBody","component":"Text","text":"Tab body"},{"id":"prompt","component":"Text","text":{"call":"formatString","args":{"value":{"path":"/title"}}}},{"id":"aria","component":"Column","children":["ariaBody"],"accessibility":{"label":{"path":"/title"},"description":{"call":"formatString","args":{"value":{"path":"/description"}}},"hidden":{"path":"/hidden"}}},{"id":"ariaBody","component":"Text","text":"Accessible content"}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	if got := surface.Nodes["body"].Text.String(); got != "Alias body" {
		t.Errorf("path alias Text = %q, want Alias body", got)
	}
	if got := surface.Nodes["tabs"].Tabs[0].Title.String(); got != "Alias title" {
		t.Errorf("path alias tab title = %q, want Alias title", got)
	}
	if got := surface.Nodes["prompt"].Text.String(); got != "Alias title" {
		t.Errorf("nested path/call aliases in prompt text = %q, want Alias title", got)
	}
	accessibility := surface.Nodes["aria"].Accessibility
	if accessibility.Label.String() != "Alias title" || accessibility.Description.String() != "Alias description" || string(accessibility.Hidden) != "true" {
		t.Errorf("accessibility aliases resolved to label=%q description=%q hidden=%s", accessibility.Label.String(), accessibility.Description.String(), accessibility.Hidden)
	}
}

func TestOpenResolvesPureBasicFunctionCallsAndReadingShowsTheResult(t *testing.T) {
	const view = `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"user":{"name":"Mira","suffix":"!"},"display/name":"Ada","ready":true,"approved":false,"email":"mira@example.com","empty":""},"components":[{"id":"root","component":"Column","children":["formatted","and","or","not","required","length","email","emptyRequired","unknown","effectful","hidden"]},{"id":"formatted","component":"Text","text":{"@call":"formatString","args":{"value":"Hello ${/user/name}${user/suffix} ${/display~1name} \\${literal}"}}},{"id":"and","component":"Text","text":{"@call":"and","args":{"values":[{"@call":"required","args":{"value":{"@path":"/user/name"}}},{"@call":"not","args":{"value":{"@path":"/approved"}}}]}}},{"id":"or","component":"Text","text":{"@call":"or","args":{"values":[{"@path":"/approved"},{"@path":"/ready"}]}}},{"id":"not","component":"Text","text":{"@call":"not","args":{"value":{"@path":"/approved"}}}},{"id":"required","component":"Text","text":{"@call":"required","args":{"value":{"@path":"/user/name"}}}},{"id":"length","component":"Text","text":{"@call":"length","args":{"value":{"@path":"/user/name"},"min":4}}},{"id":"email","component":"Text","text":{"@call":"email","args":{"value":{"@path":"/email"}}}},{"id":"emptyRequired","component":"Text","text":{"@call":"required","args":{"value":{"@path":"/empty"}}}},{"id":"unknown","component":"Text","text":{"@call":"customRenderer","args":{"value":"This stays inert."}}},{"id":"effectful","component":"Text","text":{"@call":"openUrl","args":{"url":"https://example.com"}}},{"id":"hidden","component":"Column","children":[],"accessibility":{"hidden":{"@call":"and","args":{"values":[{"@path":"/ready"},{"@call":"not","args":{"value":{"@path":"/approved"}}}]}}}}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"formatted":     "Hello Mira! Ada ${literal}",
		"and":           "true",
		"or":            "true",
		"not":           "true",
		"required":      `{"valid":true}`,
		"length":        `{"valid":true}`,
		"email":         `{"valid":true}`,
		"emptyRequired": `{"valid":false,"message":"This field is required."}`,
		"unknown":       "",
		"effectful":     "",
	} {
		if got := surface.Nodes[id].Text.String(); got != want {
			t.Errorf("%s text = %q, want %q", id, got, want)
		}
	}
	if string(surface.Nodes["hidden"].Accessibility.Hidden) != "true" {
		t.Errorf("pure boolean call hidden = %s, want true", surface.Nodes["hidden"].Accessibility.Hidden)
	}
	var reader Reading
	reader.Load(surface)
	tt := ui.NewTester(reader.View, 600, 360)
	if _, ok := tt.Access("Hello Mira! Ada ${literal}"); !ok {
		t.Fatal("formatted text was not visible in the native reader")
	}
}

func TestOpenResolvesBasicNumberDateAndPluralFunctions(t *testing.T) {
	const view = `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"amount":1234.5678,"currencyAmount":1234.5,"timestamp":"2024-02-29T09:05:06+08:00","count":2,"fraction":1.5},"components":[{"id":"root","component":"Column","children":["number","plainNumber","currency","date","two","other"]},{"id":"number","component":"Text","text":{"@call":"formatNumber","args":{"value":{"@path":"/amount"},"decimals":2}}},{"id":"plainNumber","component":"Text","text":{"@call":"formatNumber","args":{"value":{"@path":"/currencyAmount"},"decimals":2,"grouping":false}}},{"id":"currency","component":"Text","text":{"@call":"formatCurrency","args":{"value":{"@path":"/currencyAmount"},"currency":"eur"}}},{"id":"date","component":"Text","text":{"@call":"formatDate","args":{"value":{"@path":"/timestamp"},"format":"EEEE, MMM d yyyy HH:mm"}}},{"id":"two","component":"Text","text":{"@call":"pluralize","args":{"value":{"@path":"/count"},"one":"one item","two":"two items","other":"many items"}}},{"id":"other","component":"Text","text":{"@call":"pluralize","args":{"value":{"@path":"/fraction"},"zero":"no items","one":"one item","other":"many items"}}}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"number":      "1,234.57",
		"plainNumber": "1234.50",
		"currency":    "€1,234.50",
		"date":        "Thursday, Feb 29 2024 09:05",
		"two":         "two items",
		"other":       "many items",
	} {
		if got := surface.Nodes[id].Text.String(); got != want {
			t.Errorf("%s text = %q, want %q", id, got, want)
		}
	}
	var reader Reading
	reader.Load(surface)
	tt := ui.NewTester(reader.View, 600, 360)
	if _, ok := tt.Access("1,234.57"); !ok {
		t.Fatal("formatted number was not visible in the native reader")
	}
}

func TestOpenMatchesBasicIntlFormattingEdgeCases(t *testing.T) {
	const view = `{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","dataModel":{"oldYear":"1901-01-02","negativeCount":-1,"roundedCount":1.0001,"half":1.005,"cadAmount":1234.5,"dateTime":"2026-11-20T08:45:00Z"},"components":[{"id":"root","component":"Column","children":["year","negative","rounded","half","cad","currentDate"]},{"id":"year","component":"Text","text":{"@call":"formatDate","args":{"value":{"@path":"/oldYear"},"format":"yy"}}},{"id":"negative","component":"Text","text":{"@call":"pluralize","args":{"value":{"@path":"/negativeCount"},"one":"one item","other":"many items"}}},{"id":"rounded","component":"Text","text":{"@call":"pluralize","args":{"value":{"@path":"/roundedCount"},"one":"one item","other":"many items"}}},{"id":"half","component":"Text","text":{"@call":"formatNumber","args":{"value":{"@path":"/half"},"decimals":2}}},{"id":"cad","component":"Text","text":{"@call":"formatCurrency","args":{"value":{"@path":"/cadAmount"},"currency":"CAD"}}},{"id":"currentDate","component":"Text","text":{"@call":"formatDate","args":{"value":{"@path":"/dateTime"},"format":"EEEE, MMMM d yyyy hh:mm a"}}}]}}]}`
	surface, err := Open(view)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"year":        "01",
		"negative":    "one item",
		"rounded":     "one item",
		"half":        "1.01",
		"cad":         "CA$1,234.50",
		"currentDate": "Friday, November 20 2026 08:45 AM",
	} {
		if got := surface.Nodes[id].Text.String(); got != want {
			t.Errorf("%s text = %q, want %q", id, got, want)
		}
	}
}

func TestEnglishCurrencyPrefixMatchesIntlSpacingExceptions(t *testing.T) {
	for code, want := range map[string]string{
		"PHP": "₱",
		"SAR": "\u200e\u20c1\u00a0",
		"XAF": "FCFA\u00a0",
		"XCG": "Cg.\u00a0",
		"XOF": "F\u202fCFA\u00a0",
		"XPF": "CFPF\u00a0",
		"XXX": "¤",
	} {
		if got := englishCurrencyPrefix(code); got != want {
			t.Errorf("%s prefix = %q, want %q", code, got, want)
		}
	}
}
