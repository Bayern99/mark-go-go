package portrait_test

import (
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

// Baseline: styles.css mgo-overlay-in, mgo-drawer-left-in/right-in and
// mgo-scrim-in (lines 1679-1758), components/ui.css mgo-tooltip-in (30-41).
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
