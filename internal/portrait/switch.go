package portrait

import "github.com/egoist/mygo/ui"

const (
	switchWidth  = 36
	switchHeight = 20
	switchInset  = 2
)

type switchThumbKey struct{}

// Switch creates a Portrait switch editing on. Its thumb follows the switch
// with Motion.Switch, except that ReduceMotion puts it directly at its end.
func Switch(c *ui.Context, on *bool) ui.Element {
	control := ui.SwitchBase(c, on).Size(switchWidth, switchHeight).Radius(Radii.Pill).
		FocusRing(false).Cursor(ui.CursorPointer)
	duration := Motion.Switch
	if c.KeyboardInput() || c.Preferences().ReduceMotion {
		c.DisableAnimations()
		duration = 0
	}
	position := control.AnimateTo(switchThumbKey{}, flag(*on), ui.Animation{
		Duration: duration,
		Ease:     Motion.EaseInOut.Easing(),
	})
	off := Colors.Ink3
	if c.Preferences().HighContrast {
		off = Colors.Ink2
	}
	control.Background(off.Mix(Tones.Accent.Solid, position))
	at := position
	if c.Direction() == ui.RightToLeft {
		at = 1 - at
	}
	focusVisible := control.FocusVisible()
	control.DrawOver(func(p *ui.Painter, r ui.Rect) {
		diameter := r.H - 2*switchInset
		thumb := ui.Rect{
			X: r.X + switchInset + at*(r.W-r.H),
			Y: r.Y + switchInset,
			W: diameter,
			H: diameter,
		}
		p.Shadow(thumb, diameter/2, 0, 1, 3, 0, Colors.Ink.Alpha(0.25))
		p.Fill(thumb, Colors.Paper, diameter/2)
		if focusVisible {
			p.FocusRing(r, [4]float32{Radii.Pill, Radii.Pill, Radii.Pill, Radii.Pill})
		}
	})
	return control
}
