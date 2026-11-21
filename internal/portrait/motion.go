package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type CubicBezier struct{ X1, Y1, X2, Y2 float32 }

func (b CubicBezier) Easing() ui.Easing { return ui.CubicBezier(b.X1, b.Y1, b.X2, b.Y2) }

type MotionTokens struct {
	Press, Switch, Expand, Enter, Menu, Jump time.Duration
	EaseOut, EaseInOut                       CubicBezier
}

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

func Animates(c *ui.Context, keyboard bool) bool {
	return !keyboard && !c.Preferences().ReduceMotion
}

func Loops(c *ui.Context) bool { return !c.Preferences().ReduceMotion }

func Transition(c *ui.Context, keyboard bool, t ui.ElementTransition) ui.ElementTransition {
	if Animates(c, keyboard) {
		return t
	}
	t.Duration, t.ExitDuration, t.Animation = -1, -1, nil
	return t
}

type OverlayMotion struct {
	Duration time.Duration
	Ease     CubicBezier
	From     ui.Motion
	Exits    bool
}

type OverlayMotions struct {
	DrawerLeft, DrawerRight, Scrim, Popup, Tooltip OverlayMotion
}

var Overlays = OverlayMotions{
	DrawerLeft:  OverlayMotion{180 * time.Millisecond, Motion.EaseInOut, ui.Motion{X: -14, Scale: 0.985}, true},
	DrawerRight: OverlayMotion{180 * time.Millisecond, Motion.EaseInOut, ui.Motion{X: 14, Scale: 0.985}, true},
	Scrim:       OverlayMotion{160 * time.Millisecond, Motion.EaseOut, ui.Motion{}, true},
	Popup:       OverlayMotion{Motion.Menu, Motion.EaseOut, ui.Motion{Scale: 0.96, Blur: 2}, true},
	Tooltip:     OverlayMotion{125 * time.Millisecond, Motion.EaseOut, ui.Motion{Scale: 0.96}, false},
}

func (o OverlayMotion) Transition(c *ui.Context, keyboard bool) ui.ElementTransition {
	from := o.From
	t := ui.ElementTransition{Duration: o.Duration, Ease: o.Ease.Easing(), Colors: true, Enter: &from}
	if o.Exits {
		t.Exit = &from
	}
	return Transition(c, keyboard, t)
}
