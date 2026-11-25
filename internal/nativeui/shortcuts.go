package nativeui

import (
	"runtime"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// shortcutPlatform is replaceable by tests so the Ctrl-platform presentation
// is covered even when the test binary runs on macOS.
var shortcutPlatform = runtime.GOOS

// shortcutKeys formats the platform's primary modifier through ui.Cmd: Command
// on macOS and Control elsewhere. Key names stay separate keycaps on macOS;
// Ctrl platforms use their conventional joined spelling in one keycap.
func shortcutKeys(mods ui.Modifiers, keys ...string) []string {
	if shortcutPlatform == "darwin" {
		parts := make([]string, 0, len(keys)+4)
		if mods&ui.Ctrl != 0 {
			parts = append(parts, "⌃")
		}
		if mods&ui.Alt != 0 {
			parts = append(parts, "⌥")
		}
		if mods&ui.Shift != 0 {
			parts = append(parts, "⇧")
		}
		if mods&ui.Super != 0 {
			parts = append(parts, "⌘")
		}
		return append(parts, keys...)
	}
	var joined strings.Builder
	if mods&ui.Cmd != 0 || mods&ui.Ctrl != 0 {
		joined.WriteString("Ctrl+")
	}
	if mods&ui.Alt != 0 {
		joined.WriteString("Alt+")
	}
	if mods&ui.Shift != 0 {
		joined.WriteString("Shift+")
	}
	joined.WriteString(strings.Join(keys, "+"))
	return []string{joined.String()}
}

func shortcutText(mods ui.Modifiers, keys ...string) string {
	parts := shortcutKeys(mods, keys...)
	if shortcutPlatform == "darwin" {
		return strings.Join(parts, "")
	}
	return strings.Join(parts, " ")
}

func shortcutKbd(c *ui.Context, mods ui.Modifiers, keys ...string) ui.Element {
	return portrait.KbdTint(c, portrait.Colors.Ink2, portrait.Colors.Fill, shortcutKeys(mods, keys...)...)
}
