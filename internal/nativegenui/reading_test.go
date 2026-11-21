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
	bestY, bestN := 0, 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		n := 0
		for x := 0; x < img.Bounds().Dx(); x++ {
			if tinted(x, y) {
				n++
			}
		}
		if n > bestN {
			bestY, bestN = y, n
		}
	}
	x0, x1, run := -1, -1, -1
	for x := 0; x < img.Bounds().Dx(); x++ {
		if tinted(x, bestY) {
			if run < 0 {
				run = x
			}
			continue
		}
		if run >= 0 && x-run > x1-x0 {
			x0, x1 = run, x-1
		}
		if run >= 0 {
			run = -1
		}
	}
	if run >= 0 && img.Bounds().Dx()-run > x1-x0 {
		x0, x1 = run, img.Bounds().Dx()-1
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
