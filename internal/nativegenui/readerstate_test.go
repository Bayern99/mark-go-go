package nativegenui_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func viewFile(t *testing.T, created []string, updates ...[]string) string {
	t.Helper()
	list := func(statements []string) string {
		components, err := json.Marshal(statements)
		if err != nil {
			t.Fatal(err)
		}
		return string(components)
	}
	messages := []string{`{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":` + list(created) + `}}`}
	for _, update := range updates {
		messages = append(messages, `{"version":"1.0","updateComponents":{"surfaceId":"view","components":`+list(update)+`}}`)
	}
	return `{"version":3,"language":"openui-lang/1.0","sources":{"s":{"heading":"Report"}},"a2ui":[` + strings.Join(messages, ",") + `]}`
}

func opened(t *testing.T, file string) *nativegenui.Surface {
	t.Helper()
	view, err := nativegenui.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func drag(t *testing.T, tt *ui.Tester, name string, from, to float32) {
	t.Helper()
	tt.Frame()
	slider, ok := tt.Access(name)
	if !ok {
		t.Fatalf("no slider named %s", name)
	}
	box := slider.Bounds
	tt.Press(box.X+box.W*from, box.Y+box.H/2)
	tt.Move(box.X+box.W*to, box.Y+box.H/2)
	tt.Release(box.X+box.W*to, box.Y+box.H/2)
}

var budgetPlan = []string{
	`$months = 2`,
	`root = Col([plan, risks])`,
	`plan = Card([Slider($months, 0, 10, 1, "Months", "mo"), Text("Total " + @Format($months * 150, "currency", 0)), Checkbox(false, "Reviewed")])`,
	`risks = Card([Text("Old risks")])`,
}

func TestAnUpdateOfOneStatementKeepsWhatTheReaderChangedInTheOthers(t *testing.T) {
	updated := viewFile(t, budgetPlan, []string{`risks = Card([Text("New risks")])`})
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, budgetPlan)))
	tt := ui.NewTester(reader.View, 600, 600)
	drag(t, tt, "Months", 0.2, 0.8)
	if err := tt.Click("Reviewed"); err != nil {
		t.Fatal(err)
	}
	if got := texts(tt); !strings.Contains(got, "Total $1,200") || !strings.Contains(got, "Old risks") {
		t.Fatalf("dragging to 8 months did not update the total:\n%s", got)
	}

	reader.Load(opened(t, updated))
	got := texts(tt)
	if !strings.Contains(got, "New risks") || strings.Contains(got, "Old risks") {
		t.Fatalf("the update is not shown:\n%s", got)
	}
	if slider, _ := tt.Access("Months"); slider.Now != 8 || !strings.Contains(got, "Total $1,200") {
		t.Errorf("the update reset the months the reader chose: the slider reads %v in\n%s", slider.Now, got)
	}
	if box, _ := tt.Access("Reviewed"); !box.Checked {
		t.Error("the update cleared the box the reader ticked")
	}

	if _, elsewhere := shown(t, opened(t, updated), 600, nil); !strings.Contains(elsewhere, "Total $300") {
		t.Errorf("the reader's months left this reader:\n%s", elsewhere)
	}
}

func TestGoingBackToTheEarlierViewKeepsWhatTheReaderChangedInWhatItStillDeclares(t *testing.T) {
	update := []string{`$rate = 5`, `risks = Card([Slider($rate, 0, 20, 1, "Rate", "%"), Text("Rate " + $rate)])`}
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, budgetPlan, update)))
	tt := ui.NewTester(reader.View, 600, 600)
	drag(t, tt, "Months", 0.2, 0.8)
	drag(t, tt, "Rate", 0.25, 0.5)
	if got := texts(tt); !strings.Contains(got, "Total $1,200") || !strings.Contains(got, "Rate 10") {
		t.Fatalf("the sliders did not change the page:\n%s", got)
	}

	reader.Load(opened(t, viewFile(t, budgetPlan)))
	got := texts(tt)
	if !strings.Contains(got, "Old risks") || strings.Contains(got, "Rate") || strings.Contains(got, "never defined") {
		t.Fatalf("the earlier view is not shown as it was written:\n%s", got)
	}
	if slider, _ := tt.Access("Months"); slider.Now != 8 || !strings.Contains(got, "Total $1,200") {
		t.Errorf("going back reset the months the reader chose: the slider reads %v in\n%s", slider.Now, got)
	}
}

func TestAWholeNewViewStartsFromItsOwnDefaultsWhoeverWroteIt(t *testing.T) {
	again := []string{`$months = 4`, `root = Col([Slider($months, 0, 10, 1, "Months", "mo"), Text("Total " + @Format($months * 150, "currency", 0)), Checkbox(false, "Reviewed")])`}
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, budgetPlan)))
	tt := ui.NewTester(reader.View, 600, 600)
	chosen := func(view string, months float64, total string, reviewed bool) {
		t.Helper()
		got := texts(tt)
		if slider, _ := tt.Access("Months"); slider.Now != months || !strings.Contains(got, total) {
			t.Errorf("%s: the slider reads %v, want %v and %s in\n%s", view, slider.Now, months, total, got)
		}
		if box, _ := tt.Access("Reviewed"); box.Checked != reviewed {
			t.Errorf("%s: the box is ticked: %t, want %t", view, box.Checked, reviewed)
		}
	}
	drag(t, tt, "Months", 0.2, 0.8)
	if err := tt.Click("Reviewed"); err != nil {
		t.Fatal(err)
	}
	chosen("the first view after the reader's changes", 8, "Total $1,200", true)

	reader.Load(opened(t, viewFile(t, again)))
	chosen("a whole new view", 4, "Total $600", false)

	drag(t, tt, "Months", 0.4, 0.6)
	chosen("the new view after the reader's changes", 6, "Total $900", false)

	reader.Load(opened(t, viewFile(t, budgetPlan)))
	chosen("the first view put back", 2, "Total $300", false)
}

func moved(from, to ui.Rect) bool {
	for _, d := range []float32{to.X - from.X, to.Y - from.Y, to.W - from.W, to.H - from.H} {
		if d > 0.5 || d < -0.5 {
			return true
		}
	}
	return false
}

func busy(tt *ui.Tester) []ui.AccessInfo {
	var found []ui.AccessInfo
	for _, el := range tt.AccessAll() {
		if el.Busy {
			found = append(found, el)
		}
	}
	return found
}

func TestTheCardBeingUpdatedShowsAPlaceholderInItsPlaceWhileTheOthersStayReadable(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, []string{
		`$months = 2`,
		`root = Col([plan, risks, after])`,
		`plan = Source(["s"], [Card([Slider($months, 0, 10, 1, "Months", "mo"), Text("Total " + @Format($months * 150, "currency", 0))])])`,
		`risks = Source(["s"], [Card([Title("Risks", "md"), Text("Lead costs may rise."), Text("The venue may slip."), Text("The budget lapses in December.")])])`,
		`after = Card([Text("Next quarter")])`,
	})))
	reader.Sources = func(c *ui.Context, n nativegenui.Node) { ui.Button(c, "Update "+n.ComponentID) }
	for _, width := range []int{760, 360} {
		reader.Replacing = nil
		tt := ui.NewTester(reader.View, width, 900)
		before := texts(tt)
		risk, _ := tt.Find("The venue may slip.")
		next, _ := tt.Find("Next quarter")
		if len(busy(tt)) != 0 || risk.H == 0 || next.H == 0 {
			t.Fatalf("width %v: before the update %v is busy, in\n%s", width, busy(tt), before)
		}

		reader.Replacing = []string{"risks"}
		during := texts(tt)
		if !strings.Contains(during, "Updating card…") || strings.Contains(during, "The venue may slip.") || strings.Contains(during, "Update risks") {
			t.Errorf("width %v: the card being updated does not show a placeholder in its place:\n%s", width, during)
		}
		marked := busy(tt)
		if len(marked) != 1 {
			t.Fatalf("width %v: %v parts are busy, want the card being updated alone", width, len(marked))
		}
		if box := marked[0].Bounds; risk.Y < box.Y || risk.Y+risk.H > box.Y+box.H || risk.X < box.X || risk.X+risk.W > box.X+box.W {
			t.Errorf("width %v: the placeholder at %+v does not cover where the card's text was, %+v", width, box, risk)
		}
		if still, _ := tt.Find("Next quarter"); moved(next, still) {
			t.Errorf("width %v: the card below moved from %+v to %+v", width, next, still)
		}
		drag(t, tt, "Months", 0.2, 0.8)
		if got := texts(tt); !strings.Contains(got, "Total $1,200") || !strings.Contains(got, "Update plan") || !strings.Contains(got, "Next quarter") {
			t.Errorf("width %v: the other cards cannot be read and used during the update:\n%s", width, got)
		}
		if still, _ := tt.Find("Next quarter"); moved(next, still) {
			t.Errorf("width %v: the card below drifted from %+v to %+v as the update went on", width, next, still)
		}

		reader.Replacing = nil
		drag(t, tt, "Months", 0.8, 0.2)
		after := texts(tt)
		back, _ := tt.Find("The venue may slip.")
		if after != before || moved(risk, back) || len(busy(tt)) != 0 {
			t.Errorf("width %v: after the update stopped the card is at %+v, was %+v, and the page reads\n%s\nwant\n%s", width, back, risk, after, before)
		}
	}
}

func TestAConditionalSourceBelongsToItsOwnStatementDuringAnUpdate(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, []string{
		`$option = "qdrant"`,
		`root = Col([option_card])`,
		`option_card = Card([Title("Options", "md"), option_notes])`,
		`option_notes = $option == "pgvector" ? pgvector_details : $option == "qdrant" ? qdrant_details : milvus_details`,
		`pgvector_details = Source(["s"], [Text("pgvector details")])`,
		`qdrant_details = Source(["s"], [Text("Qdrant details")])`,
		`milvus_details = Source(["s"], [Text("Milvus details")])`,
	})))
	reader.Sources = func(c *ui.Context, n nativegenui.Node) { ui.Button(c, "Update "+n.ComponentID) }
	tt := ui.NewTester(reader.View, 640, 600)
	before := texts(tt)
	if !strings.Contains(before, "Update qdrant_details") || strings.Contains(before, "Update option_card") || strings.Contains(before, "Update option_notes") {
		t.Fatalf("the visible source offers the wrong statement for Update card:\n%s", before)
	}

	reader.Replacing = []string{"qdrant_details"}
	during := texts(tt)
	if !strings.Contains(during, "Updating card…") || strings.Contains(during, "Qdrant details") {
		t.Errorf("the conditional statement has no update placeholder:\n%s", during)
	}
	if !strings.Contains(during, "Options") || len(busy(tt)) != 1 {
		t.Errorf("the placeholder covers the outer option card instead of only qdrant_details:\n%s", during)
	}
}

func TestAPlaceholderInARowLeavesItsNeighboursWhereTheyWere(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(opened(t, viewFile(t, []string{
		`root = Col([Row([left, right, wide]), Row([first, second], null, null, null, true)])`,
		`left = Source(["s"], [Card([Text("Left")])])`,
		`right = Source(["s"], [Card([Text("Right")])])`,
		`wide = Card([Text("Wide")])`,
		`first = Card([Source(["s"], [Text("First")])])`,
		`second = Card([Text("Second")])`,
	})))
	reader.Sources = func(c *ui.Context, n nativegenui.Node) { ui.Text(c, "From "+n.ComponentID) }
	tt := ui.NewTester(reader.View, 760, 600)
	texts(tt)
	for _, replaced := range []string{"left", "first"} {
		before := map[string]ui.Rect{}
		for _, name := range []string{"Left", "Right", "Wide", "First", "Second"} {
			before[name], _ = tt.Find(name)
		}
		reader.Replacing = []string{replaced}
		texts(tt)
		texts(tt)
		for name, was := range before {
			now, shown := tt.Find(name)
			if strings.EqualFold(name, replaced) {
				if shown {
					t.Errorf("%s is still shown while it is updated", name)
				}
				continue
			}
			if !shown || moved(was, now) {
				t.Errorf("while %s is updated, %s moved from %+v to %+v", replaced, name, was, now)
			}
		}
		reader.Replacing = nil
		texts(tt)
	}
}
