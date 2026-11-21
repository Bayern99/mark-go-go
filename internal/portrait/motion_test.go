package portrait_test

import (
	"testing"

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
