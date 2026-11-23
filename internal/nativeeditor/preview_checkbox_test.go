package nativeeditor

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPreviewTaskCheckboxKeyboardToggleAndFocusRing(t *testing.T) {
	checked := true
	tt := ui.NewTester(func(c *ui.Context) {
		previewTaskCheckbox(c, &checked).Label("Task")
	}, 100, 80)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	node, ok := tt.AccessByRole(ui.RoleCheckBox, "Task")
	if !ok || !node.Focused || !node.Checked {
		t.Fatalf("keyboard-focused task accessibility = %+v, %v", node, ok)
	}
	theme := ui.LightTheme()
	focus := theme.Focus
	blend := func(front, back uint8) uint8 {
		return uint8((int(front)*int(focus.A) + int(back)*(255-int(focus.A)) + 127) / 255)
	}
	ringColor := color.RGBA{R: blend(focus.R, theme.Background.R), G: blend(focus.G, theme.Background.G), B: blend(focus.B, theme.Background.B), A: 255}
	foundRing := false
	r := node.Bounds
	for y := max(0, int(r.Y)-5); y < int(r.Y+r.H)+5; y++ {
		for x := max(0, int(r.X)-5); x < int(r.X+r.W)+5; x++ {
			px := tt.Image().RGBAAt(x, y)
			distance := abs(int(px.R)-int(ringColor.R)) + abs(int(px.G)-int(ringColor.G)) + abs(int(px.B)-int(ringColor.B))
			foundRing = foundRing || distance < 12
		}
	}
	if !foundRing {
		t.Fatal("keyboard focus did not draw the task checkbox focus ring")
	}
	tt.Key(0, ui.KeySpace)
	if checked {
		t.Fatal("Space did not uncheck the focused task checkbox")
	}
	if node, _ := tt.AccessByRole(ui.RoleCheckBox, "Task"); node.Checked {
		t.Fatal("assistive technology still sees the task checked after Space")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
