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
	check("Show navigation", "Toggle research")
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	check("Close Navigation", "Close Paper.md", "Plugins", "Settings", "New document", "Filter files")
	if err := app.Click("Filter files"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Filter files and headings"); err != nil {
		t.Fatal(err)
	}
	app.Type("notes")
	app.Advance(time.Second)
	check("Clear filter")
	for _, text := range app.Texts() {
		for _, glyph := range placeholderIcons {
			if slices.Contains(strings.Fields(text), glyph) {
				t.Errorf("Unicode placeholder icon %q in the window shell", glyph)
			}
		}
	}
}

func documentStatusDot(t *testing.T, app *ui.Tester, label string) ui.AccessInfo {
	t.Helper()
	dot, ok := app.AccessByRole(ui.RoleText, label)
	if !ok || dot.Offscreen || dot.Bounds.W != 6 || dot.Bounds.H != 6 {
		t.Fatalf("%s dot = %+v, found %v; want a visible 6px status dot", label, dot, ok)
	}
	if dot.Description != label {
		t.Errorf("%s dot tooltip = %q, want %q", label, dot.Description, label)
	}
	return dot
}

func TestDocumentTabStatesAreCompactToneDots(t *testing.T) {
	var shell nativeui.Shell
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{
		ID: "a", Name: "Paper.md", Edited: true, Conflict: true, Deleted: true, ReadOnly: true, Generating: true,
	}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	tab, ok := app.AccessByRole(ui.RoleTab, "Paper.md")
	if !ok {
		t.Fatalf("document title was combined with its states: %v", app.Texts())
	}
	wantDescription := "Unsaved · Conflict · Deleted · Read only · Generating"
	if tab.Description != wantDescription {
		t.Errorf("tab description = %q, want %q", tab.Description, wantDescription)
	}
	for _, label := range []string{"Unsaved", "Conflict", "Deleted", "Read only", "Generating"} {
		documentStatusDot(t, app, label)
		for _, info := range app.AccessAll() {
			if info.Role == ui.RoleText && info.Name == label && info.Bounds.H == portrait.Sizes.Compact {
				t.Errorf("%s is still rendered as a text badge: %+v", label, info)
			}
		}
	}
}

func TestNarrowUnsavedDocumentTabExposesStatusWithoutATextBadge(t *testing.T) {
	const name = "Untitled document with a long name.md"
	var shell nativeui.Shell
	model := nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{
		ID: "a", Name: name, Edited: true,
	}}}}
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{})
	}, 1280, 820)
	app.Frame()
	tab, ok := app.AccessByRole(ui.RoleTab, name)
	if !ok || tab.Description != "Unsaved" {
		t.Fatalf("narrow tab = %+v, found %v; want its accessible name and Unsaved description", tab, ok)
	}
	title, ok := app.AccessByRole(ui.RoleText, name)
	if !ok || title.Bounds.W < 60 {
		t.Fatalf("document title bounds = %+v, found %v; want room for the filename", title.Bounds, ok)
	}
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleText && info.Name == "Unsaved" && info.Bounds.H == portrait.Sizes.Compact {
			t.Fatalf("Unsaved is still rendered as a text capsule: %+v", info)
		}
	}
	dot := documentStatusDot(t, app, "Unsaved")
	x, y := int(dot.Bounds.X+dot.Bounds.W/2), int(dot.Bounds.Y+dot.Bounds.H/2)
	r, g, b, a := app.Image().At(x, y).RGBA()
	solid := portrait.Tones.Warning.Solid
	if uint8(r>>8) != solid.R || uint8(g>>8) != solid.G || uint8(b>>8) != solid.B || uint8(a>>8) != solid.A {
		t.Errorf("Unsaved dot color = (%d, %d, %d, %d), want Warning.Solid %+v", r, g, b, a, solid)
	}
	app.Move(dot.Bounds.X+dot.Bounds.W/2, dot.Bounds.Y+dot.Bounds.H/2)
	app.Advance(time.Second)
	if _, ok := app.AccessByRole(ui.RoleTooltip, "Unsaved"); !ok {
		t.Fatal("hovering the Unsaved dot did not show its tooltip")
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

func TestHoverLandsAtOnceUnderReducedMotion(t *testing.T) {
	var shell nativeui.Shell
	app := ui.NewTesterWith(func(c *ui.Context) {
		shell.Draw(c, nativeui.Model{}, nativeui.Actions{}, nativeui.Slots{})
	}, 480, 360, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	app.Frame()
	info, ok := app.AccessByRole(ui.RoleButton, "Open Folder")
	if !ok {
		t.Fatal("missing Open Folder")
	}
	b := ui.Rect{X: info.Bounds.X - 8, Y: info.Bounds.Y - 8, W: info.Bounds.W + 16, H: info.Bounds.H + 16}
	app.Move(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	first := pixels(app.Image(), b)
	app.Advance(300 * time.Millisecond)
	if n := changed(first, pixels(app.Image(), b)); n != 0 {
		t.Fatalf("hover still animating under ReduceMotion: %d pixels changed", n)
	}
}

func TestHoverStillTransitionsWithPointerMotion(t *testing.T) {
	var shell nativeui.Shell
	app := ui.NewTester(func(c *ui.Context) {
		shell.Draw(c, nativeui.Model{}, nativeui.Actions{}, nativeui.Slots{})
	}, 480, 360)
	app.Frame()
	info, ok := app.AccessByRole(ui.RoleButton, "Open Folder")
	if !ok {
		t.Fatal("missing Open Folder")
	}
	b := ui.Rect{X: info.Bounds.X - 8, Y: info.Bounds.Y - 8, W: info.Bounds.W + 16, H: info.Bounds.H + 16}
	app.Move(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	first := pixels(app.Image(), b)
	app.Advance(300 * time.Millisecond)
	if changed(first, pixels(app.Image(), b)) == 0 {
		t.Fatal("hover landed on the first frame without ReduceMotion")
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
