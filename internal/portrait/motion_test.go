package portrait_test

import (
	"image"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestChangesAnimateUnlessReducedMotionOrKeyboard(t *testing.T) {
	for _, tc := range []struct {
		name           string
		reduce         bool
		keyboard       bool
		animates, loop bool
	}{
		{"pointer", false, false, true, true},
		{"keyboard", false, true, false, true},
		{"reduced motion", true, false, false, false},
		{"reduced motion and keyboard", true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var animates, loops bool
			view := func(c *ui.Context) {
				animates, loops = portrait.Animates(c, tc.keyboard), portrait.Loops(c)
				ui.Text(c, "x")
			}
			tt := ui.NewTesterWith(view, 100, 100, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: tc.reduce}})
			tt.Frame()
			if animates != tc.animates || loops != tc.loop {
				t.Fatalf("animates %v loops %v, want %v %v", animates, loops, tc.animates, tc.loop)
			}
		})
	}
}

func TestTransitionSettlesInstantlyWhenItShouldNotAnimate(t *testing.T) {
	base := ui.ElementTransition{Duration: portrait.Motion.Switch, Match: "capsule"}
	view := func(keyboard bool, reduce bool) ui.ElementTransition {
		var got ui.ElementTransition
		tt := ui.NewTesterWith(func(c *ui.Context) { got = portrait.Transition(c, keyboard, base); ui.Text(c, "x") }, 100, 100, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		tt.Frame()
		return got
	}
	if got := view(false, false); got.Duration != portrait.Motion.Switch || got.Match != "capsule" {
		t.Fatalf("pointer transition %+v", got)
	}
	for _, got := range []ui.ElementTransition{view(true, false), view(false, true)} {
		if got.Duration >= 0 || got.Match != "capsule" {
			t.Fatalf("instant transition %+v, want negative duration keeping identity", got)
		}
	}
}

func TestOverlayMotionsMatchTheWebUIKeyframes(t *testing.T) {
	m := portrait.Motion
	for _, tc := range []struct {
		name     string
		got      portrait.OverlayMotion
		duration time.Duration
		ease     portrait.CubicBezier
		from     ui.Motion
		exits    bool
	}{
		{"drawer-left", portrait.Overlays.DrawerLeft, 180 * time.Millisecond, m.EaseInOut, ui.Motion{X: -14, Scale: 0.985}, true},
		{"drawer-right", portrait.Overlays.DrawerRight, 180 * time.Millisecond, m.EaseInOut, ui.Motion{X: 14, Scale: 0.985}, true},
		{"scrim", portrait.Overlays.Scrim, 160 * time.Millisecond, m.EaseOut, ui.Motion{}, true},
		{"popup", portrait.Overlays.Popup, m.Menu, m.EaseOut, ui.Motion{Scale: 0.96, Blur: 2}, true},
		{"tooltip", portrait.Overlays.Tooltip, 125 * time.Millisecond, m.EaseOut, ui.Motion{Scale: 0.96}, false},
	} {
		if tc.got.Duration != tc.duration || tc.got.Ease != tc.ease || tc.got.From != tc.from || tc.got.Exits != tc.exits {
			t.Errorf("%s = %+v, want %v %+v from %+v exits %v", tc.name, tc.got, tc.duration, tc.ease, tc.from, tc.exits)
		}
	}
}

func TestOverlayTransitionEntersAndExitsUnlessItShouldNotAnimate(t *testing.T) {
	get := func(o portrait.OverlayMotion, reduce, keyboard bool) ui.ElementTransition {
		var got ui.ElementTransition
		tt := ui.NewTesterWith(func(c *ui.Context) { got = o.Transition(c, keyboard); ui.Text(c, "x") }, 100, 100, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		tt.Frame()
		return got
	}
	d := get(portrait.Overlays.DrawerLeft, false, false)
	if d.Duration != 180*time.Millisecond || d.Enter == nil || *d.Enter != portrait.Overlays.DrawerLeft.From || d.Exit == nil || *d.Exit != *d.Enter {
		t.Fatalf("drawer transition %+v, want enter and reverse exit", d)
	}
	if tip := get(portrait.Overlays.Tooltip, false, false); tip.Enter == nil || tip.Exit != nil {
		t.Fatalf("tooltip transition %+v, want enter only", tip)
	}
	for _, got := range []ui.ElementTransition{get(portrait.Overlays.DrawerLeft, true, false), get(portrait.Overlays.Popup, false, true)} {
		if got.Duration >= 0 || got.ExitDuration >= 0 {
			t.Fatalf("instant overlay transition %+v, want negative durations", got)
		}
	}
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

func changingFrames(frames []*image.RGBA) int {
	n := 0
	for i := 1; i < len(frames); i++ {
		if changedPixels(frames[i-1], frames[i]) > 0 {
			n++
		}
	}
	return n
}

func sampleFrames(app *ui.Tester) []*image.RGBA {
	frames := []*image.RGBA{app.Image()}
	for range 25 {
		app.Advance(8 * time.Millisecond)
		frames = append(frames, app.Image())
	}
	return frames
}

func entering(t *testing.T, app *ui.Tester, bounds func() (ui.Rect, bool)) ui.Rect {
	t.Helper()
	b, ok := bounds()
	if !ok {
		t.Fatal("the element is not showing")
	}
	samples := []ui.Rect{b}
	for range 25 {
		app.Advance(8 * time.Millisecond)
		b, ok = bounds()
		if !ok {
			t.Fatal("the element went away while entering")
		}
		samples = append(samples, b)
	}
	app.Advance(time.Second)
	settled, ok := bounds()
	if !ok {
		t.Fatal("the element went away")
	}
	if samples[0] == settled {
		t.Fatalf("the element opened at its settled bounds %v", settled)
	}
	distinct := map[ui.Rect]bool{}
	for i := 1; i <= 20; i++ {
		if samples[i] != samples[0] && samples[i] != settled {
			distinct[samples[i]] = true
		}
	}
	if len(distinct) < 3 {
		t.Fatalf("only %d distinct intermediate bounds in the first 160ms, want at least 3", len(distinct))
	}
	if samples[25] != settled {
		t.Fatalf("bounds at 200ms %v, settled %v", samples[25], settled)
	}
	return settled
}

func exiting(t *testing.T, app *ui.Tester) {
	t.Helper()
	frames := sampleFrames(app)
	if n := changingFrames(frames); n < 3 {
		t.Fatalf("the exit painted %d new frames in 200ms, want at least 3", n)
	}
	rest := app.Image()
	app.Advance(400 * time.Millisecond)
	if changedPixels(rest, app.Image()) != 0 {
		t.Fatal("the element kept animating after its exit")
	}
}

func landsAtOnce(t *testing.T, app *ui.Tester, bounds func() (ui.Rect, bool), close func()) {
	t.Helper()
	first, ok := bounds()
	if !ok {
		t.Fatal("the element is not showing")
	}
	app.Advance(time.Second)
	settled, ok := bounds()
	if !ok {
		t.Fatal("the element went away")
	}
	if first != settled {
		t.Fatalf("the element moved with reduced motion: %v then %v", first, settled)
	}
	close()
	frames := sampleFrames(app)
	if n := changingFrames(frames); n != 0 {
		t.Fatalf("the close painted %d new frames with reduced motion", n)
	}
}

func TestPopoverBasePanelEntersAndExitsWithItsTransition(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		name := "normal motion"
		if reduced {
			name = "reduced motion"
		}
		t.Run(name, func(t *testing.T) {
			open := false
			app := ui.NewTesterWith(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					anchor := ui.Button(c, "Anchor")
					if anchor.Clicked() {
						open = true
					}
					ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
						panel.Size(140, 90).Label("Panel").Transition(portrait.Overlays.Popup.Transition(c, false))
						panel.Children(func() { ui.Text(c, "Pop") })
					})
				})
			}, 400, 360, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduced}})
			if err := app.Click("Anchor"); err != nil {
				t.Fatal(err)
			}
			panel := func() (ui.Rect, bool) {
				p, ok := app.AccessByRole(ui.RolePopup, "Panel")
				return p.Bounds, ok
			}
			close := func() { app.ClickAt(390, 350) }
			if reduced {
				landsAtOnce(t, app, panel, close)
				if _, ok := app.AccessByRole(ui.RolePopup, "Panel"); ok {
					t.Fatal("the popover is still in the access tree after the outside click")
				}
				return
			}
			settled := entering(t, app, panel)
			if settled.W < 139.5 || settled.W > 140.5 || settled.H < 89.5 || settled.H > 90.5 {
				t.Fatalf("settled popover bounds %v, want 140x90", settled)
			}
			close()
			if _, ok := app.AccessByRole(ui.RolePopup, "Panel"); ok {
				t.Fatal("the popover is still in the access tree after the outside click")
			}
			exiting(t, app)
		})
	}
}

func TestDialogBaseBackdropEntersAndExitsWithItsTransition(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		name := "normal motion"
		if reduced {
			name = "reduced motion"
		}
		t.Run(name, func(t *testing.T) {
			open := false
			app := ui.NewTesterWith(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					if ui.Button(c, "Open dialog").Clicked() {
						open = true
					}
				})
				ui.DialogBase(c, &open, func(back, panel ui.Element) {
					back.Background(ui.RGBA(0, 0, 0, 0.3)).Transition(portrait.Overlays.DrawerLeft.Transition(c, false))
					panel.Size(160, 100).Label("Dialog").Background(ui.RGB(245, 245, 245))
					panel.Children(func() { ui.Text(c, "Body") })
				})
			}, 400, 360, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduced}})
			if err := app.Click("Open dialog"); err != nil {
				t.Fatal(err)
			}
			panel := func() (ui.Rect, bool) {
				p, ok := app.AccessByRole(ui.RoleDialog, "Dialog")
				return p.Bounds, ok
			}
			close := func() { app.ClickAt(10, 10) }
			if reduced {
				landsAtOnce(t, app, panel, close)
				if _, ok := app.AccessByRole(ui.RoleDialog, "Dialog"); ok {
					t.Fatal("the dialog is still in the access tree after the backdrop click")
				}
				return
			}
			settled := entering(t, app, panel)
			if settled.W < 159.5 || settled.W > 160.5 || settled.H < 99.5 || settled.H > 100.5 {
				t.Fatalf("settled dialog bounds %v, want 160x100", settled)
			}
			close()
			if _, ok := app.AccessByRole(ui.RoleDialog, "Dialog"); ok {
				t.Fatal("the dialog is still in the access tree after the backdrop click")
			}
			exiting(t, app)
		})
	}
}

func TestTooltipBaseTipFadesInWithItsTransition(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		name := "normal motion"
		if reduced {
			name = "reduced motion"
		}
		t.Run(name, func(t *testing.T) {
			app := ui.NewTesterWith(func(c *ui.Context) {
				ui.Column(c).Fill().Justify(ui.Center).Children(func() {
					anchor := ui.Button(c, "Anchor")
					ui.TooltipBase(c, anchor, func(tip ui.Element) {
						tip.Padding(4, 8).Radius(6).Background(ui.RGB(20, 20, 24)).TextColor(ui.RGB(255, 255, 255))
						tip.Children(func() { ui.Text(c, "Hint") })
						tip.Transition(portrait.Overlays.Tooltip.Transition(c, false))
					})
				})
			}, 400, 360, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduced}})
			a, ok := app.Access("Anchor")
			if !ok {
				t.Fatal("the anchor is not showing")
			}
			app.Move(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
			shown := false
			for range 150 {
				app.Advance(8 * time.Millisecond)
				if _, ok := app.AccessByRole(ui.RoleTooltip, ""); ok {
					shown = true
					break
				}
			}
			if !shown {
				t.Fatal("the tooltip never showed")
			}
			first := app.Image()
			if reduced {
				app.Advance(200 * time.Millisecond)
				if changedPixels(first, app.Image()) != 0 {
					t.Fatal("the tooltip faded in with reduced motion")
				}
				return
			}
			frames := []*image.RGBA{first}
			for range 25 {
				app.Advance(8 * time.Millisecond)
				frames = append(frames, app.Image())
			}
			if n := changingFrames(frames); n < 3 {
				t.Fatalf("the tooltip's enter painted %d new frames in 200ms, want at least 3", n)
			}
			rest := app.Image()
			app.Advance(400 * time.Millisecond)
			if changedPixels(rest, app.Image()) != 0 {
				t.Fatal("the tooltip kept animating after its enter")
			}
		})
	}
}
