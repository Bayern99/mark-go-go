package nativegenui_test

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func metricGridView(t *testing.T) *nativegenui.Surface {
	t.Helper()
	view, err := nativegenui.Open(`{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"MetricGrid","title":"Key figures","metrics":[{"label":"Verified steps","value":"10","unit":"stages"},{"label":"High-risk ports","value":"2","unit":"ports"},{"label":"Proxy targets","value":"0","unit":"active","change":"Cleared","trend":"down"},{"label":"Release baseline","value":"CLEAN","unit":"all checks"}]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestMetricGridLaysOutWideRowWithPortraitTypeAndFlatMaterial(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(metricGridView(t))
	tt := ui.NewTesterWith(reader.View, 1280, 820, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.Frame()
	nearLeading := func(got, want float32) bool { return got >= want-2 && got <= want+2 }

	values := []string{"10", "2", "0", "CLEAN"}
	var previous ui.Rect
	for i, value := range values {
		box, ok := tt.Find(value)
		if !ok || box.W == 0 {
			t.Fatalf("metric value %q is not fully shown: %+v", value, box)
		}
		if !nearLeading(box.H, portrait.Type.Title.Leading) {
			t.Errorf("metric value %q line height = %v, want Portrait title leading %v (±2px)", value, box.H, portrait.Type.Title.Leading)
		}
		if i > 0 && (box.Y != previous.Y || box.X <= previous.X) {
			t.Errorf("metric value %q at %+v does not follow %+v on the same row", value, box, previous)
		}
		previous = box
	}
	for _, unit := range []string{"stages", "ports", "active", "all checks"} {
		if !tt.HasText(unit) {
			t.Errorf("metric unit %q is missing", unit)
		}
	}
	heading, ok := tt.Find("Key figures")
	if !ok || !nearLeading(heading.H, portrait.Type.Meta.Leading) || heading.H >= previous.H {
		t.Errorf("section heading = %+v, want Portrait meta leading %v (±2px) below the title value", heading, portrait.Type.Meta.Leading)
	}
	label, ok := tt.Find("Verified steps")
	if !ok || !nearLeading(label.H, portrait.Type.Meta.Leading) || label.H >= previous.H {
		t.Errorf("metric label = %+v, want Portrait meta leading %v (±2px) below the title value", label, portrait.Type.Meta.Leading)
	}

	img := tt.Image()
	for _, name := range []string{"Verified steps", "High-risk ports", "Proxy targets", "Release baseline"} {
		box, ok := tt.Find(name)
		if !ok {
			t.Fatalf("metric label %q is not shown", name)
		}
		x, y := int(box.X-2), int(box.Y+box.H/2)
		pixel := img.RGBAAt(x, y)
		if pixel.R != portrait.Colors.Paper.R || pixel.G != portrait.Colors.Paper.G || pixel.B != portrait.Colors.Paper.B {
			t.Errorf("metric %q has an independent fill at (%d,%d): %v", name, x, y, pixel)
		}
	}
}

func TestMetricGridReducesColumnsWhenItsWidthIsNarrow(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(metricGridView(t))
	tt := ui.NewTesterWith(reader.View, 480, 820, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.Frame()
	first, firstOK := tt.Find("10")
	third, thirdOK := tt.Find("0")
	if !firstOK || !thirdOK || first.X != third.X || third.Y <= first.Y {
		t.Fatalf("narrow metrics first = %+v (%v), third = %+v (%v), want two rows in reading order", first, firstOK, third, thirdOK)
	}
}

func TestWeightedCardsStackInReadingOrderAtNarrowWidth(t *testing.T) {
	view, err := nativegenui.Open(`{"version":2,"sources":{"a":{"heading":"A"},"b":{"heading":"B"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Row","children":["firstCard","secondCard"]},{"id":"firstCard","component":"Card","child":"first","weight":1},{"id":"secondCard","component":"Card","child":"second","weight":1},{"id":"first","component":"KeyPoints","title":"First section","items":["The first card keeps a complete readable paragraph without being squeezed into a narrow column."],"sources":["a"]},{"id":"second","component":"KeyPoints","title":"Second section","items":["The second card follows the first in reading order and keeps all of its content available."],"sources":["b"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	var reader nativegenui.Reading
	reader.Load(view)
	reader.Sources = func(c *ui.Context, n nativegenui.Node) {
		ui.ButtonBase(c).Label("Show " + n.ID)
	}
	tt := ui.NewTesterWith(reader.View, 1280, 820, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	first, _ := tt.Find("First section")
	second, _ := tt.Find("Second section")
	if first.Y != second.Y || first.X >= second.X {
		t.Fatalf("wide cards first = %+v, second = %+v, want two columns", first, second)
	}

	tt.SetSize(480, 820)
	first, _ = tt.Find("First section")
	second, _ = tt.Find("Second section")
	if first.X != second.X || second.Y <= first.Y {
		t.Fatalf("narrow cards first = %+v, second = %+v, want one column", first, second)
	}
	var order []string
	for _, el := range tt.AccessAll() {
		if el.Name == "First section" || el.Name == "Second section" {
			order = append(order, el.Name)
		}
	}
	if !slices.Equal(order, []string{"First section", "Second section"}) {
		t.Errorf("narrow card reading order = %v", order)
	}
	for _, name := range []string{"Show first", "Show second"} {
		control, ok := tt.Access(name)
		if !ok || control.Bounds.X < 0 || control.Bounds.X+control.Bounds.W > 480 || control.Bounds.Y < 0 || control.Bounds.Y+control.Bounds.H > 820 {
			t.Errorf("source action %q is clipped: %+v", name, control)
		}
	}
}

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

func locatedCards(t *testing.T, version int) (*nativegenui.Surface, string) {
	t.Helper()
	if version == 2 {
		return threeCards(t), "b"
	}
	view, err := nativegenui.Open(`{"version":3,"language":"openui-lang/1.0","sources":{"s":{"heading":"Report"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":[` +
		`"root = Col([first, second])",` +
		`"first = Card([first_source])",` +
		`"first_source = Source([\"s\"], [Text(\"Alpha point\")])",` +
		`"second = Card([second_source])",` +
		`"second_source = Source([\"s\"], [Text(\"Beta point\")])"` + `]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return view, "second_source"
}

func colorDelta(a, b *image.RGBA) int {
	n := 0
	for i := 0; i+2 < len(a.Pix) && i+2 < len(b.Pix); i++ {
		n += abs(int(a.Pix[i]) - int(b.Pix[i]))
	}
	return n
}

func changedPixelsIn(a, b *image.RGBA, r ui.Rect) int {
	n := 0
	for y := max(0, int(r.Y)); y < min(a.Rect.Dy(), int(r.Y+r.H)); y++ {
		for x := max(0, int(r.X)); x < min(a.Rect.Dx(), int(r.X+r.W)); x++ {
			a, b := a.RGBAAt(x, y), b.RGBAAt(x, y)
			if a != b {
				n++
			}
		}
	}
	return n
}

func TestLocatedCardHighlightsAndFadesWithinJumpDuration(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			view, id := locatedCards(t, version)
			var reader nativegenui.Reading
			reader.Load(view)
			tt := ui.NewTester(reader.View, 520, 600)
			tt.Advance(time.Second)
			still := tt.Image()
			target, ok := tt.Find("Beta point")
			if !ok {
				t.Fatal("target card content is not shown")
			}
			other, _ := tt.Find("Alpha point")

			reader.Highlight(id)
			tt.Frame()
			start := colorDelta(still, tt.Image())
			if start == 0 || !tt.FramePending() {
				t.Fatalf("located card did not start its highlight: delta %d, frame pending %v", start, tt.FramePending())
			}
			if changedPixelsIn(still, tt.Image(), other) != 0 {
				t.Fatal("the other card was highlighted")
			}
			if after, _ := tt.Find("Beta point"); after != target {
				t.Fatalf("highlight moved the target from %+v to %+v", target, after)
			}

			tt.Advance(portrait.Motion.Jump * 35 / 100)
			peak := colorDelta(still, tt.Image())
			tt.Advance(portrait.Motion.Jump * 45 / 100)
			late := colorDelta(still, tt.Image())
			if late >= peak {
				t.Fatalf("highlight delta went %d, %d, %d; want a fade", start, peak, late)
			}
			tt.Advance(portrait.Motion.Jump)
			if end := colorDelta(still, tt.Image()); end != 0 || tt.FramePending() {
				t.Fatalf("highlight did not return to the original card within %v: delta %d, frame pending %v", portrait.Motion.Jump, end, tt.FramePending())
			}
			if !tt.HasText("Beta point") {
				t.Fatal("highlight removed the card content")
			}
		})
	}
}

func TestLocatingAnotherCardKeepsOnlyTheLatestHighlight(t *testing.T) {
	var reader nativegenui.Reading
	reader.Load(threeCards(t))
	tt := ui.NewTester(reader.View, 520, 600)
	tt.Advance(time.Second)
	still := tt.Image()
	first, _ := tt.Find("Alpha point")
	second, _ := tt.Find("Beta point")

	reader.Highlight("a")
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	reader.Highlight("b")
	tt.Frame()
	if changedPixelsIn(still, tt.Image(), first) != 0 || changedPixelsIn(still, tt.Image(), second) == 0 {
		t.Fatal("the previous card stayed highlighted or the latest target did not highlight")
	}

	// Let the first run reach its old deadline. It must not clear the new run.
	tt.Advance(portrait.Motion.Jump - 200*time.Millisecond)
	if changedPixelsIn(still, tt.Image(), second) == 0 {
		t.Fatal("the old highlight deadline cleared the latest card")
	}
	tt.Advance(201 * time.Millisecond)
	tt.Frame()
	if delta := colorDelta(still, tt.Image()); delta != 0 || tt.FramePending() {
		t.Fatalf("the latest highlight did not fade back to the original cards: delta %d, frame pending %v", delta, tt.FramePending())
	}
}

func TestLocatedCardDoesNotHighlightWithReducedMotion(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			view, id := locatedCards(t, version)
			var reader nativegenui.Reading
			reader.Load(view)
			tt := ui.NewTesterWith(reader.View, 520, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
			tt.Advance(time.Second)
			still := tt.Image()
			reader.Highlight(id)
			tt.Frame()
			if colorDelta(still, tt.Image()) != 0 || tt.FramePending() {
				t.Fatal("locating a card played a highlight with reduced motion")
			}
			tt.Advance(portrait.Motion.Jump)
			if colorDelta(still, tt.Image()) != 0 || tt.FramePending() {
				t.Fatal("reduced-motion highlight had a delayed transition")
			}
		})
	}
}

func TestLocatedCardSourceButtonStillWorksDuringHighlight(t *testing.T) {
	view, err := nativegenui.Open(`{"version":2,"sources":{"s":{"heading":"Report"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Report","items":["Alpha point"],"sources":["s"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	clicks := 0
	reader := nativegenui.Reading{Sources: func(c *ui.Context, _ nativegenui.Node) {
		if portrait.Button(c, "Show in source", "Show in source").Clicked() {
			clicks++
		}
	}}
	reader.Load(view)
	tt := ui.NewTester(reader.View, 520, 300)
	tt.Advance(time.Second)
	reader.Highlight("root")
	tt.Frame()
	if err := tt.Click("Show in source"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 || !tt.HasText("Alpha point") {
		t.Fatalf("source clicks = %d, texts = %v", clicks, tt.Texts())
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

func codeBitmap(img *image.RGBA, r ui.Rect) (cells map[[2]int]bool, w, h int) {
	cells = map[[2]int]bool{}
	x0, y0 := int(r.X+r.W), int(r.Y+r.H)
	x1, y1 := int(r.X), int(r.Y)
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 560 {
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x), max(y1, y)
			}
		}
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 560 {
				cells[[2]int{x - x0, y - y0}] = true
			}
		}
	}
	return cells, x1 - x0 + 1, y1 - y0 + 1
}

func codeSpan(t *testing.T, tt *ui.Tester, text string) (map[[2]int]bool, int) {
	t.Helper()
	img := tt.Image()
	tinted := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		r, g, b := int(c.R), int(c.G), int(c.B)
		return r > 225 && r < 250 && abs(r-g) < 3 && abs(g-b) < 3
	}
	x0, x1, bestY := -1, -1, 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		start, end, run := -1, -1, -1
		for x := 0; x < img.Bounds().Dx(); x++ {
			if tinted(x, y) {
				if run < 0 {
					run = x
				}
				continue
			}
			if run >= 0 && x-run > end-start {
				start, end = run, x-1
			}
			if run >= 0 {
				run = -1
			}
		}
		if run >= 0 && img.Bounds().Dx()-run > end-start {
			start, end = run, img.Bounds().Dx()-1
		}
		if end-start > x1-x0 {
			x0, x1, bestY = start, end, y
		}
	}
	if x0 < 0 {
		t.Fatalf("no tinted code background behind %q", text)
	}
	spanRow := func(y int) bool {
		n := 0
		for x := x0; x <= x1; x++ {
			if tinted(x, y) {
				n++
			}
		}
		return n >= 4
	}
	y0, y1 := bestY, bestY
	for y0 > 0 && spanRow(y0-1) {
		y0--
	}
	for y1+1 < img.Bounds().Dy() && spanRow(y1+1) {
		y1++
	}
	cells, _, _ := codeBitmap(img, ui.Rect{X: float32(x0), Y: float32(y0), W: float32(x1 - x0 + 1), H: float32(y1 - y0 + 1)})
	return cells, x1 - x0 + 1
}

func referenceCode(t *testing.T) map[[2]int]bool {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Children(func() {
			ui.RichText(c, ui.Span{Text: "printf", Font: portrait.FontMono, Size: 12})
		})
	}, 200, 60)
	tt.Frame()
	cells, _, _ := codeBitmap(tt.Image(), ui.Rect{W: 200, H: 60})
	return cells
}

func TestTextInlineCodeUsesPortraitMonoAtTwelvePoints(t *testing.T) {
	want := referenceCode(t)
	for _, variant := range []string{"", "caption"} {
		t.Run("variant="+variant, func(t *testing.T) {
			attr := ""
			if variant != "" {
				attr = `,"variant":"` + variant + `"`
			}
			view, err := nativegenui.Open(`{"version":2,"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Use ` + "`printf`" + ` here"` + attr + `}]}}]}`)
			if err != nil {
				t.Fatal(err)
			}
			var reader nativegenui.Reading
			reader.Load(view)
			tt := ui.NewTester(reader.View, 520, 200)
			tt.Frame()
			cells, span := codeSpan(t, tt, "printf")
			if span < 40 {
				t.Fatalf("code span is %d wide; six glyphs of Plex Mono at 12pt take about 43", span)
			}
			best := 0
			for dy := -3; dy <= 3; dy++ {
				for dx := -3; dx <= 3; dx++ {
					n := 0
					for cell := range cells {
						if want[[2]int{cell[0] + dx, cell[1] + dy}] {
							n++
						}
					}
					best = max(best, n)
				}
			}
			if best*10 < len(cells)*7 {
				t.Fatalf("only %d of %d code glyph pixels match IBM Plex Mono at 12pt", best, len(cells))
			}
		})
	}
}
