package nativeui

import (
	"image/color"
	"math"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// darkRenameWindow is the production rename prompt in a dark Space, with a
// saved document so the prompt preselects a real stem.
func darkRenameWindow(t *testing.T) *Window {
	t.Helper()
	snapshot := workspace.WorkspaceSnapshot{
		ID: "w1", Root: "/Users/ada/Field Notes", Name: "Field Notes", Kind: "folder", ActiveID: "a",
		Documents: []workspace.DocumentInfo{{WorkspaceRoot: "/Users/ada/Field Notes", ID: "a", Name: "Quiet Software.md", Path: "/Users/ada/Field Notes/Quiet Software.md"}},
	}
	space, ok := portrait.SpaceNamed("MetalWater")
	if !ok {
		t.Fatal("MetalWater is not a Space")
	}
	w := SceneWindow(snapshot, workspace.NavSnapshot{}, nil, "# Quiet Software\n", space, true)
	if !w.SceneDialog("rename_prompt") {
		t.Fatal("rename_prompt is not a production dialog")
	}
	return w
}

// rgba is a Portrait token as an 8-bit color.
func rgba(c ui.Color) color.RGBA { return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

func near(a, b ui.Color, tolerance int) bool {
	return abs(int(a.R)-int(b.R)) < tolerance && abs(int(a.G)-int(b.G)) < tolerance && abs(int(a.B)-int(b.B)) < tolerance
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// luminance is the relative luminance WCAG's contrast ratio uses.
func luminance(c color.RGBA) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func contrastRatio(a, b color.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestDarkRenamePromptKeepsItsFieldReadable draws the production prompt in a
// dark Space and checks what the review found unreadable: the field's ink and
// its selection against the field's own background.
func TestDarkRenamePromptKeepsItsFieldReadable(t *testing.T) {
	w := darkRenameWindow(t)
	app := ui.NewTester(func(c *ui.Context) { w.DrawDialog(c) }, 620, 340)
	app.SetScale(2)
	app.Frame()
	app.Frame()

	node, ok := app.Find("File name")
	if !ok || node.W < 40 || node.H < 10 {
		t.Fatalf("rename field bounds = %+v, %v", node, ok)
	}
	img := app.Image()
	counts := map[color.RGBA]int{}
	best, bestRatio := color.RGBA{}, 0.0
	for y := int(node.Y * 2); y < int((node.Y+node.H)*2); y++ {
		for x := int(node.X * 2); x < int((node.X+node.W)*2); x++ {
			px := img.RGBAAt(x, y)
			counts[px]++
			if ratio := contrastRatio(px, rgba(portrait.Colors.Paper)); ratio > bestRatio {
				best, bestRatio = px, ratio
			}
		}
	}
	if bestRatio < 4.5 {
		t.Fatalf("field ink %v has contrast %.2f against the field background, want at least 4.5", best, bestRatio)
	}
	// The value's stem is preselected, so the field must show the selection
	// tint somewhere inside it.
	selection := portrait.Theme().Selection
	found := 0
	for px, n := range counts {
		if px.A == 255 && near(ui.Color{R: px.R, G: px.G, B: px.B}, selection, 26) {
			found += n
		}
	}
	if found < 20 {
		t.Fatalf("the preselectd stem shows no selection tint: %d pixels of %v", found, selection)
	}
}

// TestDarkRenamePromptTitleIsReadable checks the sheet's heading, the other
// dark-mode text the review found near-invisible.
func TestDarkRenamePromptTitleIsReadable(t *testing.T) {
	w := darkRenameWindow(t)
	app := ui.NewTester(func(c *ui.Context) { w.DrawDialog(c) }, 620, 340)
	app.SetScale(2)
	app.Frame()
	app.Frame()
	title, ok := app.Find("Rename “Quiet Software.md”")
	if !ok {
		t.Fatal("the prompt's title is missing")
	}
	best := 0.0
	img := app.Image()
	for y := int(title.Y * 2); y < int((title.Y+title.H)*2); y++ {
		for x := int(title.X * 2); x < int((title.X+title.W)*2); x++ {
			best = max(best, contrastRatio(img.RGBAAt(x, y), rgba(portrait.Colors.Paper)))
		}
	}
	if best < 4.5 {
		t.Fatalf("title contrast %.2f is below 4.5 in dark mode", best)
	}
}
