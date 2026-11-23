package nativegenui_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

type pixelExtent struct {
	minX, minY, maxX, maxY, count int
}

func svgPixels(img *image.RGBA, want color.RGBA) pixelExtent {
	out := pixelExtent{minX: img.Rect.Max.X, minY: img.Rect.Max.Y, maxX: -1, maxY: -1}
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			got := img.RGBAAt(x, y)
			if abs(int(got.R)-int(want.R))+abs(int(got.G)-int(want.G))+abs(int(got.B)-int(want.B)) >= 12 || got.A == 0 {
				continue
			}
			out.count++
			out.minX, out.minY = min(out.minX, x), min(out.minY, y)
			out.maxX, out.maxY = max(out.maxX, x), max(out.maxY, y)
		}
	}
	return out
}

func saveSvgScreenshot(t *testing.T, tt *ui.Tester) {
	t.Helper()
	path := os.Getenv("MGO_SVG_SCREENSHOT")
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, tt.Image()); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("SVG verification screenshot: %s", path)
}

func TestSvgPixelRegressionScalesViewBoxToAvailableWidth(t *testing.T) {
	const drawing = `root = Col([Svg([Rect(0, 0, 50, 50, "#0285ff"), Circle(75, 25, 20, "#00a67d")], "0 0 100 50", "Two shapes")])`
	var previous pixelExtent
	var previousWidth float32
	for _, width := range []int{324, 524} {
		tt, text := shown(t, statementView(t, drawing), width, nil)
		if strings.Contains(text, "was left out") || strings.Contains(text, "not a component") {
			t.Fatalf("width %d:\n%s", width, text)
		}
		drawingBox, ok := tt.Access("Two shapes")
		if !ok || drawingBox.Role != ui.RoleImage {
			t.Fatalf("width %d: no accessible drawing in %v", width, tt.AccessAll())
		}
		if got, want := drawingBox.Bounds.H/drawingBox.Bounds.W, float32(0.5); abs(int(got*1000)-int(want*1000)) > 2 {
			t.Errorf("width %d: viewBox aspect ratio is %v, want %v (bounds %v)", width, got, want, drawingBox.Bounds)
		}
		blue := svgPixels(tt.Image(), firstSeries)
		green := svgPixels(tt.Image(), secondSeries)
		if blue.count < 1000 || green.count < 1000 {
			t.Fatalf("width %d: shapes cover %d and %d pixels", width, blue.count, green.count)
		}
		pixelWidth := blue.maxX - blue.minX + 1
		if want := int(drawingBox.Bounds.W/2 + 0.5); abs(pixelWidth-want) > 2 {
			t.Errorf("width %d: the 50-unit rect is %d pixels wide, want %d", width, pixelWidth, want)
		}
		if abs(blue.minX-int(drawingBox.Bounds.X+0.5)) > 1 {
			t.Errorf("width %d: the rect starts at x %d, drawing bounds %v", width, blue.minX, drawingBox.Bounds)
		}
		if previous.count > 0 {
			got := float32(blue.maxX-blue.minX+1) / float32(previous.maxX-previous.minX+1)
			want := drawingBox.Bounds.W / previousWidth
			if difference := got - want; difference < -0.03 || difference > 0.03 {
				t.Errorf("shape width scaled by %v, available width scaled by %v", got, want)
			}
		}
		previous, previousWidth = blue, drawingBox.Bounds.W
		if width == 524 {
			saveSvgScreenshot(t, tt)
		}
	}
}

func TestSvgSupportsEveryPrimitiveAndAllCommonPathCommands(t *testing.T) {
	const allCommands = `M1 1 m1 1 L10 10 l1 0 H20 h2 V20 v2 C25 20 25 25 30 25 c2 0 2 2 4 2 S40 30 42 25 s2 -2 4 -2 Q45 20 50 25 q2 2 4 0 T60 25 t4 0 A5 5 0 0 1 70 30 a5 5 0 0 1 5 5 Z z`
	view := statementView(t, `root = Col([Svg([
		Circle(12, 12, 8, "#0285ff"),
		Ellipse(32, 12, 10, 6, "none", "#00a67d", 2),
		Rect(45, 4, 15, 15, "#8b5cf6", null, null, null, 3),
		Line(65, 5, 80, 20, "#f4a340", 3),
		Path("`+allCommands+`", "none", "#e46d91", 1),
		Polyline("2,35 10,30 18,35", "none", "#4b9ca6", 2),
		Polygon("25,30 35,30 30,40", "#0285ff"),
		SvgText("Label", 55, 42, 10, "#e46d91", "middle", "semibold"),
		G([Rect(0, 0, 8, 8, "#00a67d"), SvgText("G", 12, 8, 8, "#00a67d")], "translate(75 30) rotate(10 4 4)", 0.8)
	], "0 0 100 50", "All drawing primitives")])`)
	tt, text := shown(t, view, 640, nil)
	if strings.Contains(text, "was left out") || strings.Contains(text, "not a component") || strings.Contains(text, "never defined") {
		t.Fatalf("a supported drawing reported a problem:\n%s", text)
	}
	saveSvgScreenshot(t, tt)
	for name, ink := range map[string]color.RGBA{
		"circle and polygon": firstSeries,
		"ellipse and group":  secondSeries,
		"rect":               {0x8b, 0x5c, 0xf6, 0xff},
		"line":               {0xf4, 0xa3, 0x40, 0xff},
		"path and text":      {0xe4, 0x6d, 0x91, 0xff},
		"polyline":           {0x4b, 0x9c, 0xa6, 0xff},
	} {
		if got := svgPixels(tt.Image(), ink).count; got == 0 {
			t.Errorf("%s draws no pixels (unfiltered count %d)", name, painted(tt.Image(), ink))
		}
	}
}

func TestSvgDropsOnlyAPathWithUnsupportedCommandsAndReportsIt(t *testing.T) {
	unsupported := color.RGBA{0xff, 0x00, 0xff, 0xff}
	view := statementView(t, `root = Col([Svg([
		Circle(20, 20, 15, "#0285ff"),
		Path("M0 0 R10 10 L20 20", "#ff00ff"),
		Rect(55, 5, 30, 30, "#00a67d")
	], "0 0 100 50", "Isolated path error")])`)
	tt, text := shown(t, view, 520, nil)
	if !strings.Contains(text, "Path was left out") {
		t.Fatalf("the unsupported path is not reported:\n%s", text)
	}
	if got := svgPixels(tt.Image(), unsupported).count; got != 0 {
		t.Errorf("the unsupported path still covers %d pixels", got)
	}
	if svgPixels(tt.Image(), firstSeries).count == 0 || svgPixels(tt.Image(), secondSeries).count == 0 {
		t.Error("an unsupported path removed a sibling shape")
	}
}

func TestSvgGraphicExpressionsRepaintWhenStateChanges(t *testing.T) {
	green := color.RGBA{0x00, 0xff, 0x00, 0xff}
	magenta := color.RGBA{0xff, 0x00, 0xff, 0xff}
	view := statementView(t,
		`$radius = 4`,
		`$color = "#ff00ff"`,
		`root = Col([Svg([Circle(20, 20, $radius, $color), Rect($radius, 35, 10, 5, $color)], "0 0 100 50", "State-driven drawing"), Button("Grow", @Set($radius, $radius + 8)), Button("Recolor", @Set($color, "#00ff00"))])`)
	tt, text := shown(t, view, 520, nil)
	if strings.Contains(text, "was left out") {
		t.Fatalf("the state-driven drawing did not open:\n%s", text)
	}
	before := svgPixels(tt.Image(), magenta).count
	if before == 0 {
		saveSvgScreenshot(t, tt)
		t.Fatalf("the starting expressions draw no pixels (unfiltered count %d)", painted(tt.Image(), magenta))
	}
	if err := tt.Click("Grow"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	grown := svgPixels(tt.Image(), magenta).count
	if grown <= before {
		t.Errorf("growing the bound radius changed the drawing from %d to %d pixels", before, grown)
	}
	if err := tt.Click("Recolor"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if old := svgPixels(tt.Image(), magenta).count; old != 0 {
		t.Errorf("the old state color still covers %d pixels", old)
	}
	if got := svgPixels(tt.Image(), green).count; got < grown/2 {
		t.Errorf("the new state color covers %d pixels, before recoloring the drawing covered %d", got, grown)
	}
}
