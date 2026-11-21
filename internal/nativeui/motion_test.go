package nativeui_test

import (
	"image"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func narrowShell(prefs ui.Preferences, model nativeui.Model) *ui.Tester {
	var shell nativeui.Shell
	return ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 480, 360, ui.TesterOptions{Preferences: prefs})
}

func dialogBounds(t *testing.T, app *ui.Tester, name string) ui.Rect {
	t.Helper()
	d, ok := app.AccessByRole(ui.RoleDialog, name)
	if !ok {
		t.Fatalf("%s drawer is not open", name)
	}
	return d.Bounds
}

func changedPixels(a, b *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
			n++
		}
	}
	return n
}

func TestDrawersSlideInFromTheirSide(t *testing.T) {
	for _, tc := range []struct {
		name, open string
		left       bool
		model      nativeui.Model
	}{
		{"Navigation", "Show navigation", true, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}}},
		{"Research", "Toggle research", false, nativeui.Model{Feynman: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := narrowShell(ui.Preferences{}, tc.model)
			if err := app.Click(tc.open); err != nil {
				t.Fatal(err)
			}
			start := dialogBounds(t, app, tc.name)
			app.Advance(60 * time.Millisecond)
			mid := dialogBounds(t, app, tc.name)
			app.Advance(400 * time.Millisecond)
			end := dialogBounds(t, app, tc.name)
			if tc.left {
				if !(start.X < mid.X && mid.X < end.X) || end.X != 8 {
					t.Fatalf("left drawer X: start %v mid %v end %v, want rising to 8", start.X, mid.X, end.X)
				}
			} else if !(start.X > mid.X && mid.X > end.X) || end.X+end.W != 480-8 {
				t.Fatalf("right drawer: start %v mid %v end %v, want falling to an 8 DIP margin", start, mid, end)
			}
			app.Advance(time.Second)
			if settled := dialogBounds(t, app, tc.name); settled != end {
				t.Fatalf("drawer moved after the animation ended: %v then %v", end, settled)
			}
		})
	}
}

func TestClosingDrawerLeavesThroughAnExit(t *testing.T) {
	app := narrowShell(ui.Preferences{}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	open := app.Image()
	app.Key(0, ui.KeyEscape)
	if !app.Focused("Show navigation") {
		t.Fatal("closing the drawer should return focus to its opener")
	}
	app.Advance(60 * time.Millisecond)
	mid := app.Image()
	app.Advance(400 * time.Millisecond)
	closed := app.Image()
	if changedPixels(mid, closed) == 0 {
		t.Fatal("the drawer vanished at once; want an exit while it closes")
	}
	if changedPixels(mid, open) == 0 {
		t.Fatal("the drawer stayed in place after closing began")
	}
}

func TestReducedMotionDrawersLandAtOnce(t *testing.T) {
	app := narrowShell(ui.Preferences{ReduceMotion: true}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	if b := dialogBounds(t, app, "Navigation"); b.X != 8 {
		t.Fatalf("drawer X at frame 0 = %v, want 8", b.X)
	}
	app.Move(470, 350)
	app.Advance(time.Second)
	open := app.Image()
	app.Advance(time.Second)
	if changedPixels(open, app.Image()) != 0 {
		t.Fatal("drawer kept animating with reduced motion")
	}
	app.Key(0, ui.KeyEscape)
	closed := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(closed, app.Image()) != 0 {
		t.Fatal("drawer played an exit with reduced motion")
	}
}

func overflowShell(prefs ui.Preferences) *ui.Tester {
	return narrowShell(prefs, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "One.md"}, {ID: "b", Name: "Two.md"}, {ID: "c", Name: "Three.md"}, {ID: "d", Name: "Four.md"}, {ID: "e", Name: "Five.md"}}}})
}

func TestPopoverScalesInFromItsAnchor(t *testing.T) {
	app := overflowShell(ui.Preferences{})
	if err := app.Click("More documents"); err != nil {
		t.Fatal(err)
	}
	field := func() ui.Rect {
		f, ok := app.Access("Filter open tabs")
		if !ok {
			t.Fatal("overflow popover is not open")
		}
		return f.Bounds
	}
	start := field()
	app.Advance(50 * time.Millisecond)
	mid := field()
	app.Advance(time.Second)
	end := field()
	if !(start.W < mid.W && mid.W < end.W) {
		t.Fatalf("popover field width: start %v mid %v end %v, want growing", start.W, mid.W, end.W)
	}
	app.Key(0, ui.KeyEscape)
	closing := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(closing, app.Image()) == 0 {
		t.Fatal("popover vanished at once; want an exit")
	}
}

func TestReducedMotionPopoverLandsAtOnce(t *testing.T) {
	app := overflowShell(ui.Preferences{ReduceMotion: true})
	if err := app.Click("More documents"); err != nil {
		t.Fatal(err)
	}
	f, _ := app.Access("Filter open tabs")
	app.Advance(time.Second)
	g, _ := app.Access("Filter open tabs")
	if f.Bounds != g.Bounds {
		t.Fatalf("popover moved with reduced motion: %v then %v", f.Bounds, g.Bounds)
	}
}

func tooltipShell(prefs ui.Preferences) (*ui.Tester, ui.Rect) {
	app := narrowShell(prefs, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	b, _ := app.Access("Show navigation")
	app.Move(b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2)
	return app, b.Bounds
}

func tooltipSettled(app *ui.Tester) (shown bool, frames []*image.RGBA) {
	for range 80 {
		app.Advance(10 * time.Millisecond)
		if _, ok := app.AccessByRole(ui.RoleTooltip, "Show navigation"); ok || len(frames) > 0 {
			frames = append(frames, app.Image())
		}
	}
	return len(frames) > 0, frames
}

func TestTooltipFadesIn(t *testing.T) {
	app, _ := tooltipShell(ui.Preferences{})
	shown, frames := tooltipSettled(app)
	if !shown {
		t.Fatal("tooltip never showed")
	}
	if changedPixels(frames[0], frames[len(frames)-1]) == 0 {
		t.Fatal("tooltip appeared at full strength; want a fade in")
	}
}

func TestReducedMotionTooltipShowsAtOnce(t *testing.T) {
	app, _ := tooltipShell(ui.Preferences{ReduceMotion: true})
	shown, frames := tooltipSettled(app)
	if !shown {
		t.Fatal("tooltip never showed")
	}
	if changedPixels(frames[0], frames[len(frames)-1]) != 0 {
		t.Fatal("tooltip faded in with reduced motion")
	}
}

func TestKeyboardFocusShowsTooltipAtOnce(t *testing.T) {
	app := narrowShell(ui.Preferences{}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	for range 10 {
		if app.Focused("Show navigation") {
			break
		}
		app.Key(0, ui.KeyTab)
	}
	if !app.Focused("Show navigation") {
		t.Fatal("Tab never reached Show navigation")
	}
	if _, ok := app.AccessByRole(ui.RoleTooltip, "Show navigation"); !ok {
		t.Fatal("keyboard focus did not show the tooltip")
	}
	shown := app.Image()
	app.Advance(100 * time.Millisecond)
	if changedPixels(shown, app.Image()) != 0 {
		t.Fatal("tooltip brought by the keyboard faded in")
	}
}
