package nativeui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShortcutFormatterUsesPlatformConventions(t *testing.T) {
	original := shortcutPlatform
	t.Cleanup(func() { shortcutPlatform = original })
	shortcutPlatform = "darwin"
	if got, want := shortcutKeys(ui.Cmd|ui.Shift, "O"), []string{"⇧", "⌘", "O"}; !slices.Equal(got, want) {
		t.Fatalf("macOS shortcut = %v, want %v", got, want)
	}
	if got := shortcutText(ui.Cmd|ui.Alt, "0"); got != "⌥⌘0" {
		t.Fatalf("macOS shortcut text = %q", got)
	}
	if got, want := shortcutKeys(ui.Ctrl, "Tab"), []string{"⌃", "Tab"}; !slices.Equal(got, want) {
		t.Fatalf("macOS Control shortcut = %v, want %v", got, want)
	}
	shortcutPlatform = "linux"
	if got, want := shortcutKeys(ui.Cmd|ui.Shift, "O"), []string{"Ctrl+Shift+O"}; !slices.Equal(got, want) {
		t.Fatalf("Ctrl-platform shortcut = %v, want %v", got, want)
	}
	if got, want := shortcutKeys(ui.Cmd|ui.Alt, "K"), []string{"Ctrl+Alt+K"}; !slices.Equal(got, want) {
		t.Fatalf("Ctrl-platform Alt shortcut = %v, want %v", got, want)
	}
}
