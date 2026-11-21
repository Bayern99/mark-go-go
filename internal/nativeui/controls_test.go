package nativeui_test

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func pixels(img *image.RGBA, r ui.Rect) []uint32 {
	var px []uint32
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			px = append(px, cr+cg+cb)
		}
	}
	return px
}

func changed(a, b []uint32) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

var placeholderIcons = []string{"×", "▾", "▸", "✕", "☰", "⋯", "…", "←", "→", "↑", "↓", "↗", "↖", "↘", "↙", "↩", "↪", "⟵", "⟶"}

func TestShellIconsArePhosphorGlyphsWithNamesAndTooltips(t *testing.T) {
	var shell nativeui.Shell
	model := nativeui.Model{Feynman: true, ReturnToGenUI: true, Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library", ActiveID: "a",
		Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md"}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{GenUI: func(*ui.Context) {}})
	}, 480, 360)
	app.Frame()
	if _, ok := app.Access("Back to GenUI"); !ok {
		t.Fatal("missing Back to GenUI button")
	}
	if !slices.Contains(app.Texts(), portrait.IconArrowLeft.Glyph) {
		t.Error("Back to GenUI lacks the Phosphor arrow-left glyph")
	}
	check := func(names ...string) {
		t.Helper()
		for _, name := range names {
			info, ok := app.Access(name)
			if !ok {
				t.Fatalf("missing icon button %q", name)
			}
			if info.Description != name {
				t.Errorf("%s: tooltip %q", name, info.Description)
			}
		}
	}
	check("Show navigation", "Close Paper.md", "Plugins", "Toggle research")
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Filter files and headings"); err != nil {
		t.Fatal(err)
	}
	app.Type("notes")
	app.Advance(time.Second)
	check("Close Navigation", "Clear filter")
	for _, text := range app.Texts() {
		for _, glyph := range placeholderIcons {
			if slices.Contains(strings.Fields(text), glyph) {
				t.Errorf("Unicode placeholder icon %q in the window shell", glyph)
			}
		}
	}
}

func TestTabBarButtonSinksWhilePressed(t *testing.T) {
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, nativeui.Model{Feynman: true}, nativeui.Actions{}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	info, ok := app.AccessByRole(ui.RoleButton, "Toggle research")
	if !ok {
		t.Fatal("missing research toggle")
	}
	b := info.Bounds
	rest := pixels(app.Image(), b)
	app.Press(b.X+b.W/2, b.Y+b.H/2)
	app.Advance(300 * time.Millisecond)
	if n := changed(rest, pixels(app.Image(), b)); n < len(rest)/4 {
		t.Fatalf("pressing changes %d of %d pixels", n, len(rest))
	}
}

func viewTrack(t *testing.T, app *ui.Tester) ui.Rect {
	t.Helper()
	track, ok := app.AccessByRole(ui.RoleTabList, "Document views")
	if !ok {
		t.Fatal("missing view modes")
	}
	return track.Bounds
}

func TestViewModeCapsuleSlidesForThePointer(t *testing.T) {
	var shell nativeui.Shell
	m := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "Paper.md", ViewMode: "preview"}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, m, nativeui.Actions{SetMode: func(mode string) { m.Workspace.Documents[0].ViewMode = mode }}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	box := viewTrack(t, app)
	if err := app.Click("Markdown"); err != nil {
		t.Fatal(err)
	}
	app.Advance(50 * time.Millisecond)
	mid := pixels(app.Image(), box)
	app.Advance(time.Second)
	if changed(mid, pixels(app.Image(), box)) == 0 {
		t.Fatal("capsule had already landed mid-way through the switch")
	}
}

func TestDocumentTabCapsuleLandsAtOnceForTheKeyboard(t *testing.T) {
	var shell nativeui.Shell
	m := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "One.md"}, {ID: "b", Name: "Two.md"}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, m, nativeui.Actions{Activate: func(id string) { m.Workspace.ActiveID = id }}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	tabs, ok := app.AccessByRole(ui.RoleTabList, "Documents")
	if !ok {
		t.Fatal("missing document tabs")
	}
	tabFocused := func() bool {
		tab, ok := app.AccessByRole(ui.RoleTab, "One.md")
		return ok && tab.Focused
	}
	for range 12 {
		if tabFocused() {
			break
		}
		app.Key(0, ui.KeyTab)
	}
	if !tabFocused() {
		t.Fatal("Tab never reached the active document tab")
	}
	app.Key(0, ui.KeyRight)
	app.Frame()
	if m.Workspace.ActiveID != "b" {
		t.Fatal("Right did not activate the next tab")
	}
	first := pixels(app.Image(), tabs.Bounds)
	app.Advance(time.Second)
	if n := changed(first, pixels(app.Image(), tabs.Bounds)); n != 0 {
		t.Fatalf("capsule still moving after a keyboard switch: %d pixels changed", n)
	}
}
