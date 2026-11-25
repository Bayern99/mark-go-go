package nativeui_test

import (
	"image"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func narrowShell(prefs ui.Preferences, model nativeui.Model) *ui.Tester {
	return narrowShellActions(prefs, model, nativeui.Actions{})
}

func narrowShellActions(prefs ui.Preferences, model nativeui.Model, a nativeui.Actions) *ui.Tester {
	var shell nativeui.Shell
	return ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, a, nativeui.Slots{}) }, 480, 360, ui.TesterOptions{Preferences: prefs})
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

func focus(t *testing.T, app *ui.Tester, name string) {
	t.Helper()
	for range 20 {
		if app.Focused(name) {
			return
		}
		app.Key(0, ui.KeyTab)
	}
	t.Fatalf("Tab never reached %s", name)
}

func TestDrawersLandAtOnceForKeyboard(t *testing.T) {
	t.Run("toggle", func(t *testing.T) {
		app := narrowShell(ui.Preferences{}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
		focus(t, app, "Show navigation")
		app.Key(0, ui.KeyEnter)
		if b := dialogBounds(t, app, "Navigation"); b.X != 8 {
			t.Fatalf("keyboard-opened drawer X at frame 0 = %v, want 8", b.X)
		}
		app.Advance(time.Second)
		if b := dialogBounds(t, app, "Navigation"); b.X != 8 {
			t.Fatalf("drawer X after settling = %v, want 8", b.X)
		}
	})
	t.Run("shortcut", func(t *testing.T) {
		app := narrowShell(ui.Preferences{}, nativeui.Model{Feynman: true})
		app.Key(ui.Cmd|ui.Alt, ui.Key0)
		if b := dialogBounds(t, app, "Research"); b.X+b.W != 480-8 {
			t.Fatalf("shortcut-opened drawer right edge at frame 0 = %v, want %v", b.X+b.W, 480-8)
		}
		app.Advance(time.Second)
		if b := dialogBounds(t, app, "Research"); b.X+b.W != 480-8 {
			t.Fatalf("drawer right edge after settling = %v, want %v", b.X+b.W, 480-8)
		}
	})
}

func TestDrawerKeyboardCloseLandsAtOnce(t *testing.T) {
	app := narrowShell(ui.Preferences{}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	app.Key(0, ui.KeyEscape)
	if !app.Focused("Show navigation") {
		t.Fatal("closing the drawer should return focus to its opener")
	}
	app.Advance(50 * time.Millisecond)
	closed := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(closed, app.Image()) != 0 {
		t.Fatal("Esc-close played an exit; keyboard close should land at once")
	}
}

func TestDrawerPointerCloseLeavesThroughAnExit(t *testing.T) {
	app := narrowShell(ui.Preferences{}, nativeui.Model{Workspace: workspace.WorkspaceSnapshot{Kind: "folder", Name: "Library", Root: "/library"}})
	if err := app.Click("Show navigation"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	open := app.Image()
	app.ClickAt(470, 350)
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

func checkpointsShell(prefs ui.Preferences, restore func()) *ui.Tester {
	var shell nativeui.Shell
	var label string
	model := nativeui.Model{Git: true, Workspace: workspace.WorkspaceSnapshot{ActiveID: "a", Documents: []workspace.DocumentInfo{{ID: "a", Name: "One.md", ViewMode: "preview"}}}}
	slots := nativeui.Slots{Checkpoints: func(c *ui.Context) {
		ui.Column(c).Width(240).Gap(6).Children(func() {
			ui.TextInputBase(c, &label).Label("Checkpoint label")
			if portrait.Button(c, "Restore checkpoint", "Restore checkpoint").Clicked() && restore != nil {
				restore()
			}
		})
	}}
	return ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, slots) }, 480, 360, ui.TesterOptions{Preferences: prefs})
}

func changingFramesIn(frames []*image.RGBA, r ui.Rect) int {
	n := 0
	for i := 1; i < len(frames); i++ {
		if changedPixelsIn(frames[i-1], frames[i], r) > 0 {
			n++
		}
	}
	return n
}

func changedPixelsIn(a, b *image.RGBA, r ui.Rect) int {
	if r == (ui.Rect{}) {
		return changedPixels(a, b)
	}
	n := 0
	x0, y0 := max(0, int(r.X)), max(0, int(r.Y))
	x1, y1 := min(a.Rect.Dx(), int(r.X+r.W+1)), min(a.Rect.Dy(), int(r.Y+r.H+1))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := a.PixOffset(x, y)
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				n++
			}
		}
	}
	return n
}

func TestPopoverEntersThroughIntermediateStatesThenSettles(t *testing.T) {
	app := checkpointsShell(ui.Preferences{}, nil)
	if err := app.Click("Checkpoints"); err != nil {
		t.Fatal(err)
	}
	popup := func() ui.Rect {
		p, ok := app.AccessByRole(ui.RolePopup, "")
		if !ok {
			t.Fatal("checkpoints popover is not open")
		}
		return p.Bounds
	}
	var samples []ui.Rect
	samples = append(samples, popup())
	for range 25 {
		app.Advance(8 * time.Millisecond)
		samples = append(samples, popup())
	}
	app.Advance(time.Second)
	settled := popup()
	for i := 1; i < len(samples); i++ {
		if samples[i].W < samples[i-1].W {
			t.Fatalf("popover width shrank while entering: %v then %v at %dms", samples[i-1].W, samples[i].W, i*8)
		}
	}
	if !(samples[0].W < settled.W) {
		t.Fatalf("popover opened at its settled width %v", settled.W)
	}
	distinct := map[ui.Rect]bool{}
	for i := 1; i <= 20; i++ {
		if samples[i] != samples[0] && samples[i] != settled {
			distinct[samples[i]] = true
		}
	}
	if len(distinct) < 3 {
		t.Fatalf("only %d distinct intermediate popover bounds in the first 160ms, want at least 3", len(distinct))
	}
	if samples[25] != settled {
		t.Fatalf("popover bounds at 200ms %v, settled %v", samples[25], settled)
	}
}

func TestPopoverPointerCloseLeavesThroughAnExitThenTakesNoInput(t *testing.T) {
	var activated []string
	app := checkpointsShell(ui.Preferences{}, func() { activated = append(activated, "restore") })
	if err := app.Click("Checkpoints"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)
	pop, ok := app.AccessByRole(ui.RolePopup, "")
	if !ok {
		t.Fatal("checkpoints popover is not open")
	}
	item, ok := app.Access("Restore checkpoint")
	if !ok {
		t.Fatal("Restore checkpoint is not in the popover")
	}
	cx, cy := item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2
	app.ClickAt(240, 300)
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("the popover is still in the access tree after the outside click")
	}
	var frames []*image.RGBA
	frames = append(frames, app.Image())
	for range 25 {
		app.Advance(8 * time.Millisecond)
		frames = append(frames, app.Image())
	}
	if n := changingFramesIn(frames, pop.Bounds); n < 3 {
		t.Fatalf("the popover's exit painted %d new frames in 200ms, want at least 3", n)
	}
	rest := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixelsIn(rest, app.Image(), pop.Bounds) != 0 {
		t.Fatal("the popover kept animating after its exit")
	}
	activated = nil
	app.ClickAt(cx, cy)
	if len(activated) != 0 {
		t.Fatalf("clicking where Restore checkpoint was activated %v", activated)
	}
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("clicking where the popover was reopened it")
	}
	t.Run("during the exit", func(t *testing.T) {
		var hits []string
		fresh := checkpointsShell(ui.Preferences{}, func() { hits = append(hits, "restore") })
		if err := fresh.Click("Checkpoints"); err != nil {
			t.Fatal(err)
		}
		fresh.Advance(time.Second)
		item, ok := fresh.Access("Restore checkpoint")
		if !ok {
			t.Fatal("Restore checkpoint is not in the popover")
		}
		fresh.ClickAt(240, 300)
		fresh.Advance(16 * time.Millisecond)
		fresh.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
		if len(hits) != 0 {
			t.Fatalf("clicking where Restore checkpoint was during its exit activated %v", hits)
		}
	})
}

func TestPopoverLandsAtOnceForKeyboard(t *testing.T) {
	app := checkpointsShell(ui.Preferences{}, nil)
	focus(t, app, "Checkpoints")
	app.Key(0, ui.KeyEnter)
	f, ok := app.Access("Checkpoint label")
	if !ok {
		t.Fatal("checkpoints popover is not open")
	}
	start := f.Bounds
	app.Advance(time.Second)
	g, ok := app.Access("Checkpoint label")
	if !ok {
		t.Fatal("checkpoints popover closed")
	}
	if start != g.Bounds {
		t.Fatalf("keyboard-opened popover animated in: %v then %v", start, g.Bounds)
	}
	app.Key(0, ui.KeyEscape)
	if !app.Focused("Checkpoints") {
		t.Fatal("closing the popover should return focus to its opener")
	}
	app.Advance(50 * time.Millisecond)
	closed := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(closed, app.Image()) != 0 {
		t.Fatal("Esc-close played an exit; keyboard close should land at once")
	}
}

func TestReducedMotionPopoverLandsAtOnce(t *testing.T) {
	app := checkpointsShell(ui.Preferences{ReduceMotion: true}, nil)
	if err := app.Click("Checkpoints"); err != nil {
		t.Fatal(err)
	}
	f, opened := app.Access("Checkpoint label")
	app.Advance(time.Second)
	g, stayed := app.Access("Checkpoint label")
	if !opened || !stayed {
		t.Fatal("checkpoints popover is not open")
	}
	if f.Bounds != g.Bounds {
		t.Fatalf("popover moved with reduced motion: %v then %v", f.Bounds, g.Bounds)
	}
	app.ClickAt(240, 300)
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("the popover is still in the access tree after the outside click")
	}
	closed := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(closed, app.Image()) != 0 {
		t.Fatal("popover played an exit with reduced motion")
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

func TestDockedSidebarAnimatesOnPointerToggleAndLandsAtOnceForKeyboardAndReducedMotion(t *testing.T) {
	t.Run("pointer collapse and expand", func(t *testing.T) {
		var shell nativeui.Shell
		model := libraryModel()
		app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
		app.Frame()
		docked := mustAccess(t, app, ui.RoleMain, "Document").Bounds

		if err := app.Click("Hide sidebar"); err != nil {
			t.Fatal(err)
		}
		app.Frame()
		if _, ok := app.Access("Workspace files"); ok {
			t.Fatal("Hide sidebar kept Workspace files exposed in accessibility while collapsing")
		}
		var hideXs []float32
		hideXs = append(hideXs, mustAccess(t, app, ui.RoleMain, "Document").Bounds.X)
		for range 16 {
			app.Advance(16 * time.Millisecond)
			hideXs = append(hideXs, mustAccess(t, app, ui.RoleMain, "Document").Bounds.X)
		}
		app.Advance(time.Second)
		hidden := mustAccess(t, app, ui.RoleMain, "Document").Bounds
		if hidden.X != portrait.Shell.PageInset {
			t.Fatalf("settled hidden document X = %v, want %v", hidden.X, portrait.Shell.PageInset)
		}
		hideDistinct := map[int]bool{}
		for _, x := range hideXs {
			if x > hidden.X+1 && x < docked.X-1 {
				hideDistinct[int(x)] = true
			}
		}
		if len(hideDistinct) < 3 {
			t.Fatalf("collapse painted only %d intermediate document X positions (%v), want smooth transition", len(hideDistinct), hideXs)
		}

		if err := app.Click("Show navigation"); err != nil {
			t.Fatal(err)
		}
		app.Frame()
		var showXs []float32
		showXs = append(showXs, mustAccess(t, app, ui.RoleMain, "Document").Bounds.X)
		for range 16 {
			app.Advance(16 * time.Millisecond)
			showXs = append(showXs, mustAccess(t, app, ui.RoleMain, "Document").Bounds.X)
		}
		app.Advance(time.Second)
		reopened := mustAccess(t, app, ui.RoleMain, "Document").Bounds
		if reopened.X != docked.X {
			t.Fatalf("settled reopened document X = %v, want %v", reopened.X, docked.X)
		}
		showDistinct := map[int]bool{}
		for _, x := range showXs {
			if x > hidden.X+1 && x < docked.X-1 {
				showDistinct[int(x)] = true
			}
		}
		if len(showDistinct) < 3 {
			t.Fatalf("expand painted only %d intermediate document X positions (%v), want smooth transition", len(showDistinct), showXs)
		}
		hasSplitter := false
		for _, info := range app.AccessAll() {
			if info.Role == ui.RoleSplitter {
				hasSplitter = true
				break
			}
		}
		if !hasSplitter {
			t.Fatal("settled docked sidebar did not restore its resizable splitter")
		}
	})

	t.Run("reduced motion lands at once", func(t *testing.T) {
		var shell nativeui.Shell
		model := libraryModel()
		app := ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
		app.Frame()
		docked := mustAccess(t, app, ui.RoleMain, "Document").Bounds
		if err := app.Click("Hide sidebar"); err != nil {
			t.Fatal(err)
		}
		app.Frame()
		if got := mustAccess(t, app, ui.RoleMain, "Document").Bounds.X; got != portrait.Shell.PageInset {
			t.Fatalf("reduced-motion Hide sidebar landed at X=%v, want %v immediately", got, portrait.Shell.PageInset)
		}
		if err := app.Click("Show navigation"); err != nil {
			t.Fatal(err)
		}
		app.Frame()
		if got := mustAccess(t, app, ui.RoleMain, "Document").Bounds.X; got != docked.X {
			t.Fatalf("reduced-motion Show navigation landed at X=%v, want %v immediately", got, docked.X)
		}
	})
}

func TestOutlineSelectionPersistsAndSlidesBetweenHeadings(t *testing.T) {
	var shell nativeui.Shell
	model := libraryModel()
	model.Sections = []workspace.Section{
		{ID: "s1", Heading: "Overview", Level: 1, Start: 0},
		{ID: "s2", Heading: "Method", Level: 2, Start: 120},
		{ID: "s3", Heading: "Results", Level: 2, Start: 260},
		{ID: "s4", Heading: "Discussion", Level: 2, Start: 420},
	}
	model.HasDocumentOffset = true
	model.DocumentOffset = 0
	var revealed []int
	actions := nativeui.Actions{
		Reveal: func(offset int) {
			revealed = append(revealed, offset)
			model.DocumentOffset = offset
		},
	}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	if err := app.Click("Outline"); err != nil {
		t.Fatal(err)
	}
	app.Advance(time.Second)

	if first := mustAccess(t, app, ui.RoleTreeItem, "Overview"); !first.Selected {
		t.Fatal("initial active heading Overview is not selected")
	}
	if last := mustAccess(t, app, ui.RoleTreeItem, "Discussion"); last.Selected {
		t.Fatal("inactive heading Discussion should not be selected")
	}

	outlineInfo, ok := app.Access("Document outline")
	if !ok {
		t.Fatal("missing Document outline")
	}
	outline := outlineInfo.Bounds
	if err := app.Click("Discussion"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if len(revealed) != 1 || revealed[0] != 420 {
		t.Fatalf("clicking Discussion revealed %v, want [420]", revealed)
	}
	if last := mustAccess(t, app, ui.RoleTreeItem, "Discussion"); !last.Selected {
		t.Fatal("clicked heading Discussion did not stay selected")
	}
	if first := mustAccess(t, app, ui.RoleTreeItem, "Overview"); first.Selected {
		t.Fatal("previous heading Overview stayed selected")
	}
	var frames []*image.RGBA
	frames = append(frames, app.Image())
	for range 12 {
		app.Advance(16 * time.Millisecond)
		frames = append(frames, app.Image())
	}
	if n := changingFramesIn(frames, outline); n < 2 {
		t.Fatalf("outline selection wash painted %d changing frames, want smooth slide animation", n)
	}
	rest := app.Image()
	app.Advance(time.Second)
	if changedPixelsIn(rest, app.Image(), outline) != 0 {
		t.Fatal("outline selection kept animating after settling")
	}

	model.DocumentOffset = 280
	app.Frame()
	if res := mustAccess(t, app, ui.RoleTreeItem, "Results"); !res.Selected {
		t.Fatal("moving document cursor into Results did not update Outline selection")
	}
}
