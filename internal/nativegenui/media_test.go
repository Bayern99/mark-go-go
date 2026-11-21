package nativegenui_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativegenui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/egoist/mygo/ui"
)

func painted(img *image.RGBA, want color.RGBA) int {
	count := 0
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			got := img.RGBAAt(x, y)
			if abs(int(got.R)-int(want.R))+abs(int(got.G)-int(want.G))+abs(int(got.B)-int(want.B)) < 12 {
				count++
			}
		}
	}
	return count
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

var (
	firstSeries  = color.RGBA{0x02, 0x85, 0xff, 0xff}
	secondSeries = color.RGBA{0x00, 0xa6, 0x7d, 0xff}
)

func TestEveryChartTypeDrawsItsSeriesAndDescribesItsData(t *testing.T) {
	data := `[{month: "Jan", Sales: 120, Costs: 80}, {month: "Feb", Sales: 150, Costs: 95}, {month: "Mar", Sales: 90, Costs: 70}]`
	for kind, statement := range map[string]string{
		"bar":     `Chart("bar", ` + data + `, "month")`,
		"line":    `Chart("line", ` + data + `, "month", ["Sales", "Costs"])`,
		"area":    `Chart("area", ` + data + `, "month", null, null, true)`,
		"pie":     `Chart("pie", ` + data + `, "month", ["Sales"], "percent")`,
		"donut":   `Chart("donut", ` + data + `, "month", ["Sales"])`,
		"scatter": `Chart("scatter", [{x: 1, Sales: 120, Costs: 80}, {x: 2, Sales: 150, Costs: 95}, {x: 3, Sales: 90, Costs: 70}], "x")`,
	} {
		t.Run(kind, func(t *testing.T) {
			tt, _ := shown(t, statementView(t, `root = Col([`+statement+`])`), 640, nil)
			described := false
			for _, el := range tt.AccessAll() {
				if el.Role == ui.RoleImage && strings.EqualFold(el.Name, kind+" chart") {
					described = strings.Contains(el.Description, "Sales 150")
				}
			}
			if !described {
				t.Errorf("no %s chart describes its data to assistive technology", kind)
			}
			img := tt.Image()
			if painted(img, firstSeries) < 40 || painted(img, secondSeries) < 40 {
				t.Errorf("the series colors cover %d and %d pixels", painted(img, firstSeries), painted(img, secondSeries))
			}
			wantLegend := "Costs"
			switch kind {
			case "pie":
				wantLegend = "41.7%"
			case "donut":
				wantLegend = "150"
			}
			if !tt.HasText(wantLegend) {
				t.Errorf("%q is not shown", wantLegend)
			}
		})
	}
}

func TestStackedBarsReachTheSumOfTheirSeries(t *testing.T) {
	tt, _ := shown(t, statementView(t, `root = Col([Chart("bar", [{k: "A", One: 60, Two: 60}], "k", null, null, true)])`), 640, nil)
	if !tt.HasText("120") || tt.HasText("70") {
		t.Fatalf("the value axis does not run to the stacked total: %v", tt.Texts())
	}
}

func TestTableAlignsNumberColumnsRightAndShowsComponentCells(t *testing.T) {
	tt, text := shown(t, statementView(t, `root = Col([Table(["Item", "Price", "State"], [["Castle", "€15", Badge("Open", "success")], ["Monastery ticket", 1200.5, "n/a"]])])`), 640, nil)
	if !strings.Contains(text, "Open") || !strings.Contains(text, "1,200.5") {
		t.Fatalf("shown:\n%s", text)
	}
	short, _ := tt.Find("€15")
	long, _ := tt.Find("1,200.5")
	if short.W == 0 || abs(int(short.X+short.W)-int(long.X+long.W)) > 1 {
		t.Errorf("number cells end at %v and %v", short.X+short.W, long.X+long.W)
	}
	first, _ := tt.Find("Castle")
	second, _ := tt.Find("Monastery ticket")
	if first.X != second.X {
		t.Errorf("text cells start at %v and %v", first.X, second.X)
	}
}

func TestWideTableScrollsSidewaysInANarrowWindow(t *testing.T) {
	tt, text := shown(t, statementView(t, `root = Col([Table(["One", "Two", "Three", "Four", "Five"], [["a", "b", "c", "d", "last cell"]])])`), 300, nil)
	tt.Frame()
	if !strings.Contains(text, "last cell") {
		t.Fatalf("shown:\n%s", text)
	}
	if box, ok := tt.Find("One"); !ok || box.X < 0 || box.X > 300 {
		t.Errorf("the first column is at %v", box)
	}
}

func TestIconsDrawByPhosphorAndLucideNames(t *testing.T) {
	if err := nativeui.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	ink := func(name string) int {
		tt, text := shown(t, statementView(t, `root = Col([Icon("`+name+`", "xl", "#0285ff")])`), 200, nil)
		if strings.Contains(text, "not a component") {
			t.Fatalf("shown:\n%s", text)
		}
		return painted(tt.Image(), firstSeries)
	}
	for _, name := range []string{"map-pin", "magnifying-glass", "search", "rotate-ccw", "x"} {
		if ink(name) == 0 {
			t.Errorf("%s draws nothing", name)
		}
	}
	if ink("no-such-icon") != 0 {
		t.Error("an unknown name draws something")
	}
}

func TestImagesComeOnlyFromFilesTheDocumentShows(t *testing.T) {
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "plan.png"))
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(file, image.NewRGBA(image.Rect(0, 0, 4, 3)))
	file.Close()
	os.WriteFile(filepath.Join(dir, "secret.png"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "site.png"), []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o600)
	load := nativegenui.DocumentImages("# Report\n\n![Plan](plan.png)\n\n![Site][site]\n\n![Remote](https://example.com/a.png)\n\n[site]: site.png\n", dir)
	if source, err := load("plan.png"); err != nil || source == nil {
		t.Errorf("the document's own image did not load: %v", err)
	}
	if _, err := load("site.png"); err == nil || strings.Contains(err.Error(), "does not contain") {
		t.Errorf("an image the document shows by reference was refused before it was read: %v", err)
	}
	for _, src := range []string{"secret.png", "https://example.com/a.png", "https://example.com/other.png", "../plan.png", "file:///etc/hosts"} {
		if source, err := load(src); err == nil || source != nil {
			t.Errorf("%s loaded", src)
		}
	}
}

func TestImageShowsItsDescriptionUntilItLoads(t *testing.T) {
	_, text := shown(t, statementView(t, `root = Col([Image("plan.png", "Floor plan", 1.5)])`), 400, nil)
	if !strings.Contains(text, "Floor plan") {
		t.Fatalf("shown:\n%s", text)
	}
}

func TestCarouselKeepsItsSlidesSideBySide(t *testing.T) {
	tt, _ := shown(t, statementView(t, `root = Col([Carousel([Card([Text("First slide")]), Card([Text("Second slide")])], 200)])`), 320, nil)
	first, _ := tt.Find("First slide")
	second, ok := tt.Find("Second slide")
	if !ok || second.X <= first.X || second.Y != first.Y {
		t.Fatalf("slides are at %v and %v", first, second)
	}
}

func TestPopoverShowsItsContentWhenItsTriggerIsPressed(t *testing.T) {
	tt, text := shown(t, statementView(t, `root = Col([Popover(Badge("Why?"), [Text("Because the palace sells out.")])])`), 400, nil)
	if strings.Contains(text, "Because") {
		t.Fatalf("the aside is open before its trigger is pressed:\n%s", text)
	}
	if err := tt.Click("Why?"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Because the palace sells out.") {
		t.Fatalf("shown: %v", tt.Texts())
	}
}

func TestTripExampleShowsEveryDataPrimitiveWithoutNotices(t *testing.T) {
	data, err := os.ReadFile("testdata/trip.report.md.genui.json")
	if err != nil {
		t.Fatal(err)
	}
	view, err := nativegenui.Open(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{820, 340} {
		tt, text := shown(t, view, width, nil)
		if strings.Contains(text, "not a component") || strings.Contains(text, "never defined") {
			t.Errorf("width %d:\n%s", width, text)
		}
		for _, want := range []string{"Three days in Lisbon", "Pena Palace", "Jerónimos Monastery", "€20", "47.8%", "Rossio station"} {
			if !tt.HasText(want) {
				t.Errorf("width %d: %q is not shown", width, want)
			}
		}
	}
}

func TestChartWithoutAKnownTypeDrawsBars(t *testing.T) {
	tt, _ := shown(t, statementView(t, `$kind = ""`, `root = Col([Chart($kind, [{k: "A", One: 60}], "k")])`), 400, nil)
	if painted(tt.Image(), firstSeries) < 40 {
		t.Fatal("no bars are drawn")
	}
}

func TestImageThatCannotBeReadKeepsItsDescription(t *testing.T) {
	view := statementView(t, `root = Col([Image("broken.png", "Site plan")])`)
	loaded := make(chan struct{})
	tt, _ := shown(t, view, 400, func(r *nativegenui.Reading) {
		r.LoadImage = func(string) (ui.ImageSource, error) {
			defer close(loaded)
			var none *ui.SVG
			return none, os.ErrInvalid
		}
	})
	<-loaded
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Site plan") {
		t.Fatalf("shown: %v", tt.Texts())
	}
}
