package nativeui_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func stripes(c *ui.Context) {
	ui.Row(c).Fill().Children(func() {
		for i := range 120 {
			bar := ui.Hex("#000000")
			if i%2 == 0 {
				bar = ui.Hex("#ff0000")
			}
			ui.Box(c).Width(4).FillHeight().Background(bar)
		}
	})
}

func pixelRow(app *ui.Tester, x, y int) [][3]uint8 {
	img := app.Image()
	var row [][3]uint8
	for i := range 20 {
		c := color.NRGBAModel.Convert(img.At(x+i, y)).(color.NRGBA)
		row = append(row, [3]uint8{c.R, c.G, c.B})
	}
	return row
}

func assertBlurredStripes(t *testing.T, row [][3]uint8) {
	t.Helper()
	lo, hi := 255, 0
	for _, c := range row {
		lo, hi = min(lo, int(c[0])), max(hi, int(c[0]))
		if int(c[0]) <= int(c[1])+8 {
			t.Fatalf("content does not show through the glass: %v", c)
		}
	}
	if hi-lo > 24 {
		t.Fatalf("content behind the glass is not blurred: red ranges %d..%d", lo, hi)
	}
}

func openDrawerOverStripes(t *testing.T, prefs ui.Preferences) [][3]uint8 {
	t.Helper()
	var shell nativeui.Shell
	model := nativeui.Model{Feynman: true, Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview"}}}}
	app := ui.NewTesterWith(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{Document: stripes})
	}, 480, 360, ui.TesterOptions{Preferences: prefs})
	app.Key(ui.Cmd|ui.Alt, ui.Key0)
	drawer, ok := app.AccessByRole(ui.RoleDialog, "Research")
	if !ok {
		t.Fatal("research drawer did not open")
	}
	app.Advance(time.Second)
	b := drawer.Bounds
	return pixelRow(app, int(b.X+b.W/2), int(b.Y+b.H/2))
}

func TestResearchDrawerBlursTheDocumentBehindIt(t *testing.T) {
	assertBlurredStripes(t, openDrawerOverStripes(t, ui.Preferences{}))
}

func TestResearchDrawerIsOpaqueWhenTransparencyIsReducedOrContrastIncreased(t *testing.T) {
	for name, prefs := range map[string]ui.Preferences{"reduce transparency": {ReduceTransparency: true}, "high contrast": {HighContrast: true}} {
		for _, c := range openDrawerOverStripes(t, prefs) {
			if c != [3]uint8{255, 255, 255} {
				t.Fatalf("%s: drawer pixel %v, want opaque paper", name, c)
			}
		}
	}
}

func stripedShell(prefs ui.Preferences) *ui.Tester {
	var shell nativeui.Shell
	model := nativeui.Model{Git: true, Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview"}}}}
	return ui.NewTesterWith(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{Document: stripes, Checkpoints: func(c *ui.Context) { ui.Box(c).Size(220, 120) }})
	}, 480, 360, ui.TesterOptions{Preferences: prefs})
}

func openCheckpointsOverStripes(t *testing.T, prefs ui.Preferences) [][3]uint8 {
	t.Helper()
	app := stripedShell(prefs)
	if err := app.Click("Checkpoints"); err != nil {
		t.Fatal(err)
	}
	pop, ok := app.AccessByRole(ui.RolePopup, "")
	if !ok {
		t.Fatal("checkpoints popover did not open")
	}
	app.Advance(time.Second)
	b := pop.Bounds
	return pixelRow(app, int(b.X+b.W/2), int(b.Y+b.H-2))
}

func TestCheckpointsPopoverBlursTheDocumentBehindIt(t *testing.T) {
	assertBlurredStripes(t, openCheckpointsOverStripes(t, ui.Preferences{}))
}

func TestCheckpointsPopoverIsOpaqueWhenTransparencyIsReducedOrContrastIncreased(t *testing.T) {
	for name, prefs := range map[string]ui.Preferences{"reduce transparency": {ReduceTransparency: true}, "high contrast": {HighContrast: true}} {
		for _, c := range openCheckpointsOverStripes(t, prefs) {
			if c != [3]uint8{255, 255, 255} {
				t.Fatalf("%s: popover pixel %v, want opaque paper", name, c)
			}
		}
	}
}

func openCommandBarOverStripes(t *testing.T, prefs ui.Preferences) [][3]uint8 {
	t.Helper()
	app := stripedShell(prefs)
	app.Key(ui.Cmd, ui.KeyK)
	bar, ok := app.AccessByRole(ui.RoleDialog, "Command Bar")
	if !ok {
		t.Fatal("command bar did not open")
	}
	app.Advance(time.Second)
	b := bar.Bounds
	return pixelRow(app, int(b.X+b.W/2), int(b.Y+b.H-3))
}

func TestCommandBarBlursTheDocumentBehindIt(t *testing.T) {
	assertBlurredStripes(t, openCommandBarOverStripes(t, ui.Preferences{}))
}

func TestCommandBarIsOpaqueWhenTransparencyIsReducedOrContrastIncreased(t *testing.T) {
	for name, prefs := range map[string]ui.Preferences{"reduce transparency": {ReduceTransparency: true}, "high contrast": {HighContrast: true}} {
		for _, c := range openCommandBarOverStripes(t, prefs) {
			if c != [3]uint8{255, 255, 255} {
				t.Fatalf("%s: command bar pixel %v, want opaque paper", name, c)
			}
		}
	}
}
