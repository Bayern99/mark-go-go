package nativegenui_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/egoist/mygo/ui"
)

func texts(tt *ui.Tester) string {
	tt.Frame()
	tt.Frame()
	var names []string
	for _, el := range tt.AccessAll() {
		names = append(names, el.Name, el.Description)
	}
	return strings.Join(names, "\n")
}

func TestDraggingASliderUpdatesTheTextAndChartThatDependOnIt(t *testing.T) {
	view := statementView(t,
		`$months = 2`,
		`root = Col([Slider($months, 0, 10, 1, "Months", "mo"), Text("Total " + @Format($months * 150, "currency", 0)), Chart("bar", @Each(@Range(0, $months), "m", {month: "M" + m, cost: 150}), "month", ["cost"])])`)
	tt, before := shown(t, view, 600, nil)
	if !strings.Contains(before, "Total $300") {
		t.Fatalf("the starting total is missing in\n%s", before)
	}
	slider, ok := tt.Access("Months")
	if !ok {
		t.Fatalf("no slider named Months in\n%s", before)
	}
	box := slider.Bounds
	tt.Press(box.X+box.W*0.2, box.Y+box.H/2)
	tt.Move(box.X+box.W*0.8, box.Y+box.H/2)
	tt.Release(box.X+box.W*0.8, box.Y+box.H/2)
	after := texts(tt)
	if !strings.Contains(after, "Total $1,200") || !strings.Contains(after, "M7") {
		t.Errorf("dragging to 8 months did not update the total and the chart:\n%s", after)
	}
}

func TestAFormChecksItsRulesBeforeAnActionThatSends(t *testing.T) {
	view := statementView(t,
		`$email = ""`, `$sent = 0`,
		`root = Col([Form([Input($email, "Email", "text", "you@example.com", {required: true, email: true}), Button("Subscribe", Action([@Set($sent, $sent + 1), @ToAssistant("Subscribe me")]))]), Text("Sent " + $sent)])`)
	tt, _ := shown(t, view, 600, nil)
	if err := tt.Click("Subscribe"); err != nil {
		t.Fatal(err)
	}
	if got := texts(tt); !strings.Contains(got, "This field is required") || !strings.Contains(got, "Sent 0") {
		t.Fatalf("an empty required field did not stop the action:\n%s", got)
	}
	if err := tt.Click("Email, required"); err != nil {
		t.Fatal(err)
	}
	tt.Type("not an address")
	tt.Click("Subscribe")
	if got := texts(tt); !strings.Contains(got, "Please enter a valid email") || !strings.Contains(got, "Sent 0") {
		t.Fatalf("a malformed address did not stop the action:\n%s", got)
	}
	tt.Click("Email, required")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("ada@example.com")
	tt.Click("Subscribe")
	got := texts(tt)
	if strings.Contains(got, "Please enter") || !strings.Contains(got, "Sent 1") {
		t.Errorf("a valid address did not run the action:\n%s", got)
	}
	if len(tt.OpenedURLs()) != 0 {
		t.Errorf("sending to the assistant opened %v", tt.OpenedURLs())
	}
}

func TestControlsKeepTheirStateOutOfTheViewFile(t *testing.T) {
	path := "testdata/trip.report.md.genui.json"
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	view := statementView(t,
		`$done = false`, `$plan = "a"`, `$n = 1`,
		`root = Col([Checkbox($done, "Booked", true), SegmentedControl($plan, [{value: "a", label: "Basic"}, {value: "b", label: "Plus"}]), RadioGroup($plan, ["a", "b"], "Plan", "pills"), Button("More", @Set($n, $n + 1)), Button("Ask", @ToAssistant("more")), Button("Docs", @OpenUrl("https://example.com/docs")), Button("Script", @OpenUrl("javascript:alert(1)")), Text(($done ? "Booked" : "Open") + " " + $plan + " " + $n)])`)
	tt, got := shown(t, view, 600, nil)
	if !strings.Contains(got, "Open a 1") {
		t.Fatalf("starting state is missing in\n%s", got)
	}
	for _, name := range []string{"Booked", "Plus", "More", "Ask", "Docs", "Script"} {
		if err := tt.Click(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tt.Frame()
	}
	if got := texts(tt); !strings.Contains(got, "Booked b 2") {
		t.Errorf("the controls did not change the page:\n%s", got)
	}
	if opened := tt.OpenedURLs(); len(opened) != 1 || opened[0] != "https://example.com/docs" {
		t.Errorf("opened %v, want only the web address", opened)
	}
	reopened := statementView(t, `$n = 1`, `root = Col([Text("n " + $n)])`)
	if _, got := shown(t, reopened, 600, nil); !strings.Contains(got, "n 1") {
		t.Errorf("a reopened view did not start from its defaults:\n%s", got)
	}
	if now, _ := os.ReadFile(path); string(now) != string(saved) {
		t.Error("interaction wrote to a view file")
	}
}

func TestATimerCountsItsSecondsDownAndACelebrationStaysStillUnderReducedMotion(t *testing.T) {
	view := statementView(t,
		`$left = 3`,
		`root = Col([Timer($left, "Rest"), Text($left == 0 ? "Time is up" : "Left " + $left), Celebration($left == 0), Flashcard("Bonjour", "Hello", "French")])`)
	var reader nativegenui.Reading
	reader.Load(view)
	tt := ui.NewTester(reader.View, 600, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if got := texts(tt); !strings.Contains(got, "00:03") || !strings.Contains(got, "Bonjour") {
		t.Fatalf("the timer and card are missing in\n%s", got)
	}
	tt.Click("Start")
	for range 3 {
		tt.Advance(time.Second)
		tt.Frame()
	}
	if got := texts(tt); !strings.Contains(got, "Time is up") || !strings.Contains(got, "00:00") {
		t.Errorf("the timer did not reach zero:\n%s", got)
	}
	if tt.FramePending() {
		t.Error("a frame is still pending under reduced motion")
	}
	tt.Click("Bonjour")
	if got := texts(tt); !strings.Contains(got, "Hello") {
		t.Errorf("the card did not flip:\n%s", got)
	}
}

func TestPressingTheTextInsideAPressableOrAFlashcardCounts(t *testing.T) {
	view := statementView(t, `$n = 0`,
		`root = Col([Pressable([Text("Tap this tile")], @Set($n, $n + 1)), Text("Taps " + $n), Flashcard([Text("Front words")], [Text("Back words")])])`)
	tt, _ := shown(t, view, 600, nil)
	for _, name := range []string{"Tap this tile", "Front words"} {
		box, ok := tt.Find(name)
		if !ok {
			t.Fatalf("%q is not shown", name)
		}
		tt.Press(box.X+box.W/2, box.Y+box.H/2)
		tt.Release(box.X+box.W/2, box.Y+box.H/2)
		tt.Frame()
	}
	if got := texts(tt); !strings.Contains(got, "Taps 1") || !strings.Contains(got, "back side") {
		t.Errorf("pressing the text did nothing:\n%s", got)
	}
}

func TestANumberFieldWithoutAVariableStaysEmptyOnceCleared(t *testing.T) {
	view := statementView(t, `root = Col([Form([Input(5, "Seats", "number", null, {required: true}), Button("Book", @ToAssistant("book"))])])`)
	tt, _ := shown(t, view, 600, nil)
	tt.Click("Seats, required")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Click("Book")
	if got := texts(tt); !strings.Contains(got, "This field is required") {
		t.Errorf("the cleared field passed its required rule:\n%s", got)
	}
}

func TestLoadingAViewAgainStopsItsRunningTimer(t *testing.T) {
	lines := []string{`$left = 30`, `root = Col([Timer($left, "Rest")])`}
	var reader nativegenui.Reading
	reader.Load(statementView(t, lines...))
	tt := ui.NewTester(reader.View, 600, 400)
	tt.Frame()
	tt.Click("Start")
	tt.Advance(2 * time.Second)
	tt.Frame()
	reader.Load(statementView(t, lines...))
	tt.Advance(5 * time.Second)
	if got := texts(tt); !strings.Contains(got, "00:30") || !strings.Contains(got, "Start") {
		t.Errorf("the reloaded timer kept running:\n%s", got)
	}
}
