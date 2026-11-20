package nativegenui_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func TestReaderSortsNumbersByKeyboardAndRestoresOriginalOrder(t *testing.T) {
	view, err := nativegenui.Open(`{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"ComparisonTable","title":"Fees","columns":["Plan","Price"],"rows":[["Basic","$2.15"],["Pro","-$20"],["Small","$2.5"]]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	var reader nativegenui.Reading
	reader.Load(view)
	tt := ui.NewTester(reader.View, 520, 400)
	rows := func() []string {
		var names []string
		for _, el := range tt.AccessAll() {
			if el.Role == ui.RoleRowHeader {
				names = append(names, el.Name)
			}
		}
		return names
	}
	if err := tt.Click("Sort by Price"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		order ui.SortDirection
		names []string
	}{
		{ui.SortAscending, []string{"Pro", "Basic", "Small"}},
		{ui.SortDescending, []string{"Small", "Basic", "Pro"}},
		{ui.SortNone, []string{"Basic", "Pro", "Small"}},
	} {
		tt.Frame()
		header, ok := tt.AccessByRole(ui.RoleColumnHeader, "Price")
		if !ok || header.Sort != step.order || !slices.Equal(rows(), step.names) {
			t.Fatalf("header %+v rows %v, want %v", header, rows(), step.names)
		}
		tt.Key(0, ui.KeySpace)
	}
}

func TestChecklistRevealsQuotedSourceWithoutChangingItsDoneState(t *testing.T) {
	view, err := nativegenui.Open(`{"version":2,"sources":{"s":{"heading":"Report"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Checklist","items":[{"text":"Review figures","quote":"actual figures","done":true}],"sources":["s"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	available := false
	selected := ""
	reader := nativegenui.Reading{
		CanSelect:  func(n nativegenui.Node) bool { return available && slices.Contains(n.Sources, "s") },
		SelectItem: func(_ nativegenui.Node, quote string) { selected = quote },
	}
	reader.Load(view)
	tt := ui.NewTester(reader.View, 400, 300)
	control, ok := tt.Access("Review figures")
	if !ok || !control.Disabled || !strings.Contains(control.Description, "Completed") {
		t.Fatalf("missing source should disable the completed item: %+v", control)
	}
	available = true
	tt.Frame()
	if err := tt.Click("Review figures"); err != nil {
		t.Fatal(err)
	}
	if selected != "actual figures" || !strings.Contains(string(view.Nodes["root"].Items[0]), `"done":true`) {
		t.Fatalf("selection %q, item %s", selected, view.Nodes["root"].Items[0])
	}
}
