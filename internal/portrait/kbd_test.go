package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestKbdKeycapsHaveOneAccessibleName(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.Kbd(c, "⌘", "⇧", "K") }), 240, 120)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleText, "⌘ ⇧ K")
	if !ok || info.Bounds.H != portrait.Sizes.Compact {
		t.Fatalf("Kbd accessibility or size: %+v, found %v", info, ok)
	}
	for _, key := range []string{"⌘", "⇧", "K"} {
		if bounds, found := tt.Find(key); !found || bounds.W <= 0 || bounds.H <= 0 {
			t.Fatalf("key %q is not painted: %+v, found %v", key, bounds, found)
		}
	}
}

func TestKbdLongCJKAcrossPreferences(t *testing.T) {
	for _, preference := range []ui.Preferences{
		{},
		{HighContrast: true},
		{ReduceTransparency: true},
	} {
		tt := ui.NewTesterWith(onChrome(func(c *ui.Context) { portrait.Kbd(c, "搜索") }), 240, 120, ui.TesterOptions{Preferences: preference})
		tt.Frame()
		if info, ok := tt.AccessByRole(ui.RoleText, "搜索"); !ok || info.Bounds.W <= 0 || info.Bounds.H != portrait.Sizes.Compact {
			t.Fatalf("CJK Kbd with %+v: %+v, found %v", preference, info, ok)
		}
	}
}

func TestTooltipRendersShortcutKeycaps(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		button := portrait.Button(c, "Quick open", "Quick open")
		portrait.Tooltip(c, button, "Quick open", "⌘", "K")
	}), 320, 180)
	tt.Frame()
	button, _ := tt.AccessByRole(ui.RoleButton, "Quick open")
	if button.Description != "Quick open (⌘ K)" {
		t.Fatalf("shortcut is absent from the accessible description: %q", button.Description)
	}
	tt.Key(0, ui.KeyTab)
	tt.Advance(time.Second)
	for _, text := range []string{"Quick open", "⌘", "K"} {
		if bounds, found := tt.Find(text); !found || bounds.W <= 0 || bounds.H <= 0 {
			t.Fatalf("tooltip text or keycap %q is not visible: %+v, found %v", text, bounds, found)
		}
	}
}
