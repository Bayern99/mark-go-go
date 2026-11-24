package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Slider draws a tokenized slider on MyGo's accessible slider primitive.
func Slider(c *ui.Context, value *float64, lo, hi float64) ui.Element {
	s := ui.SliderBase(c, value, lo, hi).Height(Sizes.Regular).Padding(0, 8).MinWidth(0)
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		inset := float32(8)
		left, right := r.X+inset, r.X+r.W-inset
		y := r.Y + r.H/2
		progress := float32(0)
		if hi > lo {
			progress = float32(min(max((*value-lo)/(hi-lo), 0), 1))
		}
		x := left + (right-left)*progress
		p.Fill(ui.Rect{X: left, Y: y - 2, W: max(0, right-left), H: 4}, Colors.FillStrong, 2)
		p.Fill(ui.Rect{X: left, Y: y - 2, W: max(0, x-left), H: 4}, Colors.Accent, 2)
		thumb := ui.Rect{X: x - 8, Y: y - 8, W: 16, H: 16}
		p.Fill(thumb, Colors.Paper, 8)
		p.Stroke(thumb, Colors.Accent, 8, 1.5)
		if s.FocusVisible() {
			p.FocusRing(thumb, [4]float32{8, 8, 8, 8})
		}
	})
	return s
}

// Radio draws the shared radio organ on MyGo's accessible radio primitive.
func Radio[T comparable](c *ui.Context, selected *T, value T, label string) ui.Element {
	r := ui.RadioBase(c, selected, value).Gap(8).AlignItems(ui.Center).FocusRing(false)
	chosen := *selected == value
	r.Children(func() {
		box := ui.Box(c).Size(16, 16).Shrink(0).AccessHidden(true)
		box.Draw(func(p *ui.Painter, bounds ui.Rect) {
			fill, border := Colors.Paper, Colors.LineStrong
			if chosen {
				border = Colors.Accent
			} else if r.Hovered() {
				fill, border = Colors.AccentSoft, Colors.Accent
			}
			p.Fill(bounds, fill, 8)
			p.Stroke(bounds, border, 8, 1)
			if chosen {
				p.Fill(ui.Rect{X: bounds.X + 5, Y: bounds.Y + 5, W: 6, H: 6}, Colors.Accent, 3)
			}
			if r.FocusVisible() {
				p.FocusRing(bounds, [4]float32{8, 8, 8, 8})
			}
		})
		Type.Small.Apply(ui.Text(c, label)).TextColor(Colors.Ink).MinWidth(0).Wrap().AccessHidden(true)
	})
	return r.Label(label)
}

// DateField gives the native accessible date editor Portrait field chrome.
// MyGo currently exposes DateInput rather than a DateInputBase; this wrapper
// keeps that framework detail out of every application surface.
func DateField(c *ui.Context, date *time.Time, label string) ui.Element {
	d := ui.DateInput(c, date).Label(label).Height(Form.Field).Padding(0, Form.PadX).Radius(Form.Radius)
	Press(d, Raised)
	return d
}

// Segmented builds an accessible segmented control with Portrait's track and
// selected capsule, backed by MyGo's keyboard and radio-group behavior.
type SegmentedSpec struct {
	Track, Capsule Surface
	Group          string
}

func Segmented(c *ui.Context, selected *int, options []string, label string) ui.Element {
	return SegmentedWithSpec(c, selected, options, label, SegmentedSpec{Track: Materials.Track, Capsule: Materials.Raised})
}

// SegmentedWithSpec permits document readers to derive density from their
// content theme while sharing the same accessible control and motion.
func SegmentedWithSpec(c *ui.Context, selected *int, options []string, label string, spec SegmentedSpec) ui.Element {
	parts := ui.SegmentedBase(c, selected, len(options))
	track := parts.Track.Label(label).Padding(2).Gap(2).Radius(Radii.Track).AlignItems(ui.Center)
	spec.Track.Apply(track)
	track.Children(func() {
		for i, option := range options {
			segment := parts.Segment(i).Height(Sizes.Regular-4).Padding(0, 10).Radius(Radii.Segment).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0)
			ink := Colors.Ink3
			if i == *selected || segment.Hovered() {
				ink = Colors.Ink
			}
			segment.Children(func() {
				if i == *selected {
					capsule := ui.Box(c.Key("selected")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(Radii.Segment).AccessHidden(true).IgnoreInput(true)
					spec.Capsule.Apply(capsule)
					if spec.Group != "" {
						capsule.Transition(Transition(c, c.KeyboardInput(), ui.ElementTransition{Match: "capsule:" + spec.Group, Duration: Motion.Switch, Ease: Motion.EaseInOut.Easing()}))
					}
				}
				Type.UI.Apply(ui.Text(c, option)).FontWeight(500).TextColor(ink).ZIndex(1).MaxLines(1).Ellipsis("…")
			})
		}
	})
	return track
}
