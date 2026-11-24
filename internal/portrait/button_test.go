package portrait_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestButtonSpecLookSizeAndStateMatrix(t *testing.T) {
	looks := []struct {
		name string
		look portrait.Look
	}{
		{"raised", portrait.Raised},
		{"accent", portrait.Accent},
		{"quiet", portrait.Quiet},
		{"danger", portrait.Danger},
		{"danger-outline", portrait.DangerOutline},
		{"link", portrait.Link},
	}
	sizes := []struct {
		name string
		size float32
	}{
		{"compact", portrait.Sizes.Compact},
		{"regular", portrait.Sizes.Regular},
		{"large", portrait.Sizes.Large},
	}
	states := []string{"rest", "hover", "pressed", "focus-visible", "disabled", "loading", "selected", "invalid", "long-cjk"}
	for _, look := range looks {
		for _, size := range sizes {
			for _, state := range states {
				t.Run(fmt.Sprintf("%s/%s/%s", look.name, size.name, state), func(t *testing.T) {
					title, label := "Save", "Save document"
					if state == "long-cjk" {
						title = "保存文档并同步所有更改"
						label = title
					}
					clicks := 0
					tt := ui.NewTester(onChrome(func(c *ui.Context) {
						b := portrait.ButtonWith(c, title, label, portrait.ButtonSpec{
							Look: look.look, Size: size.size,
							Loading: state == "loading", Selected: state == "selected",
						})
						if state == "disabled" {
							b.Disabled(true)
						}
						if b.Clicked() {
							clicks++
						}
					}), 360, 160)
					tt.Frame()
					info, ok := tt.AccessByRole(ui.RoleButton, label)
					if !ok {
						t.Fatal("button missing from the accessibility tree")
					}
					if info.Bounds.H != size.size {
						t.Fatalf("height %v, want %v", info.Bounds.H, size.size)
					}
					x, y := info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2
					switch state {
					case "hover":
						tt.Move(x, y)
						tt.Advance(300 * time.Millisecond)
					case "pressed":
						tt.Press(x, y)
						tt.Advance(300 * time.Millisecond)
					case "focus-visible":
						tt.Key(0, ui.KeyTab)
						tt.Frame()
					case "disabled", "loading":
						tt.ClickAt(x, y)
						tt.ClickAt(x, y)
					}
					info, _ = tt.AccessByRole(ui.RoleButton, label)
					switch state {
					case "focus-visible":
						if !info.Focused {
							t.Fatal("keyboard focus did not reach the button")
						}
					case "disabled", "loading":
						if !info.Disabled || clicks != 0 {
							t.Fatalf("disabled button state %+v, clicks %d", info, clicks)
						}
						if state == "loading" && !info.Busy {
							t.Fatal("loading state is not exposed as busy")
						}
					case "selected":
						if !info.Checked {
							t.Fatal("selected state is not exposed")
						}
					}
				})
			}
		}
	}
}

func TestButtonSpecDefaultsToRegularAndSupportsBothIcons(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.ButtonWith(c, "Synchronize", "Synchronize document", portrait.ButtonSpec{
			Leading: portrait.IconArrowUpRight, Trailing: portrait.IconCheck,
		})
	}), 300, 160)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleButton, "Synchronize document")
	if !ok || info.Bounds.H != portrait.Sizes.Regular {
		t.Fatalf("zero-size button %+v (found %v), want regular height", info, ok)
	}
	for _, access := range tt.AccessAll() {
		if access.Name == portrait.IconArrowUpRight.Glyph || access.Name == portrait.IconCheck.Glyph {
			t.Fatalf("decorative icon exposed to assistive technology: %+v", access)
		}
	}
}

func TestDangerButtonUsesTheDangerTone(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.ButtonWith(c, "Delete", "Delete", portrait.ButtonSpec{Look: portrait.Danger})
	}), 240, 160)
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleButton, "Delete")
	r, g, b, _ := tt.Image().At(int(info.Bounds.X+info.Bounds.W)-6, int(info.Bounds.Y+info.Bounds.H/2)).RGBA()
	if r>>8 != 171 || g>>8 != 55 || b>>8 != 47 {
		t.Fatalf("danger fill rgb(%d, %d, %d), want 鹅血石红 rgb(171, 55, 47)", r>>8, g>>8, b>>8)
	}
}
