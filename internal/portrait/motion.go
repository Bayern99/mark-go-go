package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// CubicBezier is a CSS cubic-bezier() timing function.
type CubicBezier struct{ X1, Y1, X2, Y2 float32 }

// Easing returns the curve as a MyGo easing.
func (b CubicBezier) Easing() ui.Easing { return ui.CubicBezier(b.X1, b.Y1, b.X2, b.Y2) }

// MotionTokens lists Portrait durations and curves.
type MotionTokens struct {
	Press, Switch, Expand, Enter, Menu, Jump time.Duration
	EaseOut, EaseInOut                       CubicBezier
}

// Motion holds the Portrait motion tokens.
var Motion = MotionTokens{
	Press:     100 * time.Millisecond,
	Switch:    160 * time.Millisecond,
	Expand:    150 * time.Millisecond,
	Enter:     200 * time.Millisecond,
	Menu:      150 * time.Millisecond,
	Jump:      420 * time.Millisecond,
	EaseOut:   CubicBezier{0.23, 1, 0.32, 1},
	EaseInOut: CubicBezier{0.32, 0.72, 0, 1},
}

// Animates reports whether a change should play its transition. It lands at
// once when the system asks for reduced motion or the keyboard caused it.
func Animates(c *ui.Context, keyboard bool) bool {
	return !keyboard && !c.Preferences().ReduceMotion
}

// Loops reports whether looping animations (spinners, shimmer) may run.
// MyGo's Loop ignores reduced motion, so callers draw a static state instead.
func Loops(c *ui.Context) bool { return !c.Preferences().ReduceMotion }

// Transition returns t, or t made instant (keeping its identity) when the
// change should not animate.
func Transition(c *ui.Context, keyboard bool, t ui.ElementTransition) ui.ElementTransition {
	if Animates(c, keyboard) {
		return t
	}
	t.Duration, t.ExitDuration, t.Animation = -1, -1, nil
	return t
}
