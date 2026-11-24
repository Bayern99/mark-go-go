package nativeui

import (
	"image"
	"os"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const sampleRowTitle = "Quiet Software Notes.md"

// inkArea is the pixels of one band of the image that are not the paper
// behind them: an icon's column is skipped, so a title's own ink is what the
// measurement sees.
func inkArea(img *image.RGBA, area image.Rectangle, paper ui.Color) (width, height, count int) {
	minX, minY, maxX, maxY := area.Max.X, area.Max.Y, area.Min.X-1, area.Min.Y-1
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			px := img.RGBAAt(x, y)
			if near(ui.Color{R: px.R, G: px.G, B: px.B}, paper, 20) {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
			count++
		}
	}
	if maxX < minX || maxY < minY {
		return 0, 0, 0
	}
	return maxX - minX + 1, maxY - minY + 1, count
}

// paperBands renders a test image at twice the scale, so a pixel measurement
// sees the glyphs' own shapes.
func paperBands(height int, build func(c *ui.Context)) (*ui.Tester, *image.RGBA) {
	app := ui.NewTester(build, 400, height)
	app.SetScale(2)
	app.Frame()
	app.Frame()
	return app, app.Image()
}

// TestFileRowTitlesAreMetaSized checks the sidebar's step: the file tree and
// the outline carry their names at 12pt Meta, so they sit under the open tabs'
// 13pt UI and the workspace name's 14pt, in a row that stays 28pt tall.
func TestFileRowTitlesAreMetaSized(t *testing.T) {
	// Where the title starts: the row's 8pt padding, the 14pt icon and the
	// 7pt gap between them.
	const titleLeft = 30

	app, img := paperBands(84, func(c *ui.Context) {
		// The same name at the two steps above the file tree, in bands the
		// row heights match so the measurements line up.
		textBand := func(style portrait.TypeStyle) {
			ui.Row(c).Height(portrait.Sizes.Regular).Padding(0, 0, 0, titleLeft).AlignItems(ui.Center).Children(func() {
				style.Apply(ui.Text(c, sampleRowTitle)).TextColor(portrait.Colors.Ink)
			})
		}
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Children(func() {
			navRow(c, navItem{key: "file", title: sampleRowTitle, icon: portrait.IconFileText})
			textBand(portrait.Type.Meta)
			textBand(portrait.Type.UI)
		})
	})
	paper := portrait.Colors.Paper
	rowEnd := int(portrait.Sizes.Regular * 2)
	row := image.Rect(titleLeft*2, 0, 400*2, rowEnd)
	meta := image.Rect(titleLeft*2, rowEnd, 400*2, 2*rowEnd)
	uiBand := image.Rect(titleLeft*2, 2*rowEnd, 400*2, 3*rowEnd)
	_, rowHeight, _ := inkArea(img, row, paper)
	_, metaHeight, _ := inkArea(img, meta, paper)
	_, uiHeight, _ := inkArea(img, uiBand, paper)
	if rowHeight != metaHeight {
		t.Fatalf("file row title ink is %dpx tall, want Meta's %dpx", rowHeight, metaHeight)
	}
	if rowHeight >= uiHeight {
		t.Fatalf("file row title ink is %dpx tall, want it under the UI step's %dpx", rowHeight, uiHeight)
	}
	// The row itself stays airy: 28pt for a 12pt name.
	info, ok := app.AccessByRole(ui.RoleTreeItem, sampleRowTitle)
	if !ok {
		t.Fatalf("the file row is not in the tree")
	}
	if info.Bounds.H != portrait.Sizes.Regular {
		t.Fatalf("file row height = %v, want %v", info.Bounds.H, portrait.Sizes.Regular)
	}
}

// TestSelectedFileRowIsHeavier checks the gentle selection: the active file's
// name is drawn at 500 over the 400 of the rows around it, without a bolder
// ink than theirs.
func TestSelectedFileRowIsHeavier(t *testing.T) {
	_, img := paperBands(80, func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Children(func() {
			navRow(c, navItem{key: "plain", title: "Quiet Notes", icon: portrait.IconFileText, ink: portrait.Colors.Ink})
			navRow(c, navItem{key: "selected", title: "Quiet Notes", icon: portrait.IconFileText, ink: portrait.Colors.Ink, selected: true})
		})
	})
	paper := portrait.Colors.Paper
	rowEnd := int(portrait.Sizes.Regular * 2)
	_, _, plain := inkArea(img, image.Rect(30*2, 0, 400*2, rowEnd), paper)
	_, _, selected := inkArea(img, image.Rect(30*2, rowEnd, 400*2, 2*rowEnd), paper)
	if selected <= plain {
		t.Fatalf("the selected row's title has %d ink pixels, no more than the %d of an inactive row", selected, plain)
	}
}

// TestOnlyTheSelectedTabIsHeavier keeps the tab strip's weight to its one
// selected tab: the weight is set inside the selection check, never outside it.
func TestOnlyTheSelectedTabIsHeavier(t *testing.T) {
	src, err := os.ReadFile("tabs.go")
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	if got := strings.Count(code, "FontWeight(500)"); got != 1 {
		t.Fatalf("tabs.go sets a weight %d times, want the selected tab's one", got)
	}
	at := strings.Index(code, "FontWeight(500)")
	before := code[:at]
	branch := strings.LastIndex(before, "if selected {")
	if branch < 0 || strings.Contains(before[branch:], "}") {
		t.Fatal("the tab weight must sit inside the selected tab's branch")
	}
}

// TestScrollEdgeMaskOnlyFadesWhatScrollsUnderIt keeps the fade off the rows at
// rest: the top fades only once the list scrolled past its first row, and the
// bottom only while rows wait below.
func TestScrollEdgeMaskOnlyFadesWhatItMust(t *testing.T) {
	stops := func(scroll *ui.ScrollState) []float32 {
		mask := scrollEdgeMask(180, scroll)
		at := make([]float32, len(mask.Stops))
		for i, stop := range mask.Stops {
			at[i] = stop.At
		}
		return at
	}
	atRest := stops(&ui.ScrollState{})
	if atRest[0] != 0 || atRest[1] != 0 || atRest[2] != 1 || atRest[3] != 1 {
		t.Fatalf("a list at rest is masked at %v, want no fade at either edge", atRest)
	}
	scrolled := stops(&ui.ScrollState{Y: 200, MaxY: 900})
	if scrolled[1] != scrollEdgeFade || scrolled[2] != 1-scrollEdgeFade {
		t.Fatalf("a scrolled list is masked at %v, want a fade at both edges", scrolled)
	}
	atEnd := stops(&ui.ScrollState{Y: 900, MaxY: 900})
	if atEnd[1] != scrollEdgeFade || atEnd[2] != 1 {
		t.Fatalf("a list scrolled to its end is masked at %v, want the top fade only", atEnd)
	}
	if scrollEdgeFade > 0.03 {
		t.Fatalf("the edge fade is %v of the viewport, too wide to keep a row whole", scrollEdgeFade)
	}
}

// TestMarkdownExtensionIsMutedWithoutBreakingTruncation draws file names with
// their extension in the secondary ink, and keeps a long name on the row's one
// line: the extension is a span of the same paragraph, so the ellipsis still
// cuts the whole name.
func TestMarkdownExtensionIsMutedWithoutBreakingTruncation(t *testing.T) {
	paper := portrait.Colors.Paper
	_, short := paperBands(40, func(c *ui.Context) {
		ui.Column(c).Fill().Background(paper).Children(func() {
			navRow(c, navItem{key: "short", title: "Notes.md", icon: portrait.IconFileText})
		})
	})
	muted := 0
	for y := 0; y < 40*2; y++ {
		for x := 30 * 2; x < 400*2; x++ {
			px := short.RGBAAt(x, y)
			if near(ui.Color{R: px.R, G: px.G, B: px.B}, portrait.Colors.Ink3, 12) {
				muted++
			}
		}
	}
	if muted < 8 {
		t.Fatalf("the .md extension shows %d pixels of the secondary ink, too few to be muted", muted)
	}

	long := "2024-11-03-a-very-long-analysis-document-name-that-keeps-going-and-going-and-going.md"
	app, tall := paperBands(40, func(c *ui.Context) {
		ui.Column(c).Fill().Background(paper).Children(func() {
			navRow(c, navItem{key: "long", title: long, icon: portrait.IconFileText})
		})
	})
	_, height, _ := inkArea(tall, image.Rect(30*2, 0, 400*2, 40*2), paper)
	if height > int(portrait.Type.Meta.Leading*2) {
		t.Fatalf("a long file name draws %dpx tall, want one %v line", height, portrait.Type.Meta.Leading)
	}
	info, ok := app.AccessByRole(ui.RoleTreeItem, long)
	if !ok {
		t.Fatal("the long name is not in the tree")
	}
	if info.Bounds.H != portrait.Sizes.Regular {
		t.Fatalf("a long file name's row is %v tall, want %v", info.Bounds.H, portrait.Sizes.Regular)
	}
}
