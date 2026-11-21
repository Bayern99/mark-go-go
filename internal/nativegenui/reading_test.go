package nativegenui_test

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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

func threeCards(t *testing.T) *nativegenui.Surface {
	t.Helper()
	view, err := nativegenui.Open(`{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[` +
		`{"id":"root","component":"Column","children":["a","b","c"]},` +
		`{"id":"a","component":"KeyPoints","title":"First","items":["Alpha point"]},` +
		`{"id":"b","component":"KeyPoints","title":"Second","items":["Beta point"]},` +
		`{"id":"c","component":"KeyPoints","title":"Third","items":["Gamma point"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func cardTops(t *testing.T, tt *ui.Tester) [3]float32 {
	t.Helper()
	var ys [3]float32
	for i, text := range []string{"Alpha point", "Beta point", "Gamma point"} {
		r, ok := tt.Find(text)
		if !ok {
			t.Fatalf("%q is not shown", text)
		}
		ys[i] = r.Y
	}
	return ys
}

func TestCardsEnterOneAfterAnother(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(threeCards(t))
	tt := ui.NewTester(reader.View, 520, 600)
	start := cardTops(t, tt)
	tt.Advance(30 * time.Millisecond)
	early := cardTops(t, tt)
	tt.Advance(40 * time.Millisecond)
	later := cardTops(t, tt)
	tt.Advance(time.Second)
	end := cardTops(t, tt)
	if !(early[0] < start[0]) || early[1] != start[1] || early[2] != start[2] {
		t.Fatalf("at 30ms only the first card should rise: start %v, then %v", start, early)
	}
	if !(later[1] < start[1]) || later[2] != start[2] {
		t.Fatalf("at 70ms the second card should rise and the third wait: start %v, then %v", start, later)
	}
	for i := range end {
		if !(end[i] < start[i]) || start[i]-end[i] > portrait.GenUI.Enter.Rise+0.01 {
			t.Fatalf("card %d settled at %v from %v, want %v DIPs higher", i, end[i], start[i], portrait.GenUI.Enter.Rise)
		}
	}
	if tt.FramePending() {
		t.Fatal("cards kept animating after they came in")
	}
}

func TestCardsLandAtOnceWithReducedMotion(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(threeCards(t))
	tt := ui.NewTesterWith(reader.View, 520, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	start := cardTops(t, tt)
	shown := tt.Image()
	if tt.FramePending() {
		t.Fatal("cards animate with reduced motion")
	}
	tt.Advance(time.Second)
	if cardTops(t, tt) != start || changedPixels(shown, tt.Image()) != 0 {
		t.Fatal("cards moved after the first frame with reduced motion")
	}
}

func TestHaloAndRimSweepOnceWhenAViewIsGenerated(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(threeCards(t))
	tt := ui.NewTester(reader.View, 520, 600)
	tt.Advance(time.Second)
	still := tt.Image()
	reader.Halo()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if changedPixels(still, tt.Image()) == 0 {
		t.Fatal("no halo 300ms after the view was generated")
	}
	tt.Advance(time.Second)
	if n := changedPixels(still, tt.Image()); n != 0 || tt.FramePending() {
		t.Fatalf("the halo stayed: %d pixels differ, frames pending %v", n, tt.FramePending())
	}
}

func TestNoHaloWithReducedMotion(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(threeCards(t))
	tt := ui.NewTesterWith(reader.View, 520, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	still := tt.Image()
	reader.Halo()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if changedPixels(still, tt.Image()) != 0 || tt.FramePending() {
		t.Fatal("the halo played with reduced motion")
	}
}

func TestReplacedCardShowsAShimmeringSkeletonAndSaysSo(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		reader := nativegenui.Reading{Replacing: []string{"b"}}
		reader.Load(threeCards(t))
		tt := ui.NewTesterWith(reader.View, 520, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		tt.Advance(time.Second)
		if !tt.HasText("Updating card…") || tt.HasText("Beta point") {
			t.Fatalf("reduce %v: texts %v, want the card's status in place of its content", reduce, tt.Texts())
		}
		before := tt.Image()
		tt.Advance(400 * time.Millisecond)
		moved := changedPixels(before, tt.Image()) != 0
		if moved == reduce || tt.FramePending() == reduce {
			t.Fatalf("reduce %v: shimmer moved %v, frames pending %v", reduce, moved, tt.FramePending())
		}
	}
}

func changedPixels(a, b *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
			n++
		}
	}
	return n
}
