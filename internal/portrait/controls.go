package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type Look int

const (
	Raised Look = iota
	Accent
	Quiet
	Danger
	DangerOutline
	Link
)

type feedback struct {
	rest, hover, press Surface
	ink, hoverInk      ui.Color
	sink               float32
}

var (
	looks [Link + 1]feedback
	Rings struct{ Field, Editing, Invalid Shadows }
)

func controlLooks() {
	looks = [...]feedback{
		Raised:        {Materials.Raised, Materials.RaisedHover, Materials.RaisedPress, Colors.Ink, Colors.Ink, 0.98},
		Accent:        {Materials.AccentRaised, Materials.AccentRaisedHover, Materials.AccentRaisedPress, Colors.OnAccent, Colors.OnAccent, 0.985},
		Quiet:         {Surface{ui.Transparent, ui.Transparent, nil}, Materials.Row, Materials.RaisedPress, Colors.Ink2, Colors.Ink, 0.97},
		Danger:        {dangerRest, dangerHover, dangerPress, Tones.Danger.OnSolid, Tones.Danger.OnSolid, 0.985},
		DangerOutline: {dangerOutlineRest, dangerOutlineHover, dangerOutlinePress, Tones.Danger.Ink, Tones.Danger.Ink, 0.98},
		Link:          {Surface{ui.Transparent, ui.Transparent, nil}, Surface{ui.Transparent, ui.Transparent, nil}, Surface{Colors.AccentSoft, Colors.AccentSoft, nil}, Colors.Accent, Colors.Accent, 0.98},
	}
	Rings = struct{ Field, Editing, Invalid Shadows }{
		Field:   Shadows{out(0, 0, 0, 0.5, Colors.LineStrong), out(0, 0, 0, 3, Colors.Focus)},
		Editing: Shadows{out(0, 0, 0, 1, Colors.Accent), out(0, 0, 0, 3, Colors.Focus)},
		Invalid: Shadows{out(0, 0, 0, 1, Colors.Danger), out(0, 0, 0, 3, Tones.Danger.Soft)},
	}
}

const hoverShadow = 140 * time.Millisecond

type hoverKey struct{}
type pressKey struct{}

func Press(b ui.Element, look Look) ui.Color {
	f := looks[look]
	ease := Motion.EaseOut.Easing()
	hovered, pressed := b.Hovered(), b.Pressed()
	h := b.AnimateTo(hoverKey{}, flag(hovered && !pressed), ui.Animation{Duration: hoverShadow, Ease: ease})
	p := b.AnimateTo(pressKey{}, flag(pressed), ui.Animation{Duration: Motion.Press, Ease: ease})
	f.rest.Mix(f.hover, h).Mix(f.press, p).Apply(b)
	scale := 1 - (1-f.sink)*p
	b.TransformOrigin(0.5, 0.5).Translate(0, 0.5*p).Scale(scale, scale)
	if hovered || pressed {
		return f.hoverInk
	}
	return f.ink
}

func RowHover(row ui.Element, on bool) {
	h := row.AnimateTo(hoverKey{}, flag(on), ui.Animation{Duration: hoverShadow, Ease: Motion.EaseOut.Easing()})
	if h > 0 {
		Surface{ui.Transparent, ui.Transparent, nil}.Mix(Materials.Row, h).Apply(row)
	}
}

// Reveal eases e in while on, as a row's secondary action shows on hover or
// focus; ReduceMotion shows it at once.
func Reveal(e ui.Element, on bool) ui.Element {
	return e.Opacity(e.AnimateTo(revealKey{}, flag(on), ui.Animation{Duration: hoverShadow, Ease: Motion.EaseOut.Easing()}))
}

type revealKey struct{}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

type Segments struct {
	group string
}

func Track(c *ui.Context, group string, build func(s *Segments)) ui.Element {
	t := ui.Row(c.Key("track:" + group)).Role(ui.RoleTabList).Padding(2).Gap(2).Radius(Radii.Track).AlignItems(ui.Center)
	Materials.Track.Apply(t)
	s := &Segments{group: group}
	t.Children(func() { build(s) })
	return t
}

func (s *Segments) Segment(c *ui.Context, title, label string, selected bool) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(Sizes.Regular-4).Padding(0, 10).Radius(Radii.Segment).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0)
	ink := Colors.Ink3
	if selected || b.Hovered() {
		ink = Colors.Ink
	}
	b.Children(func() {
		s.Capsule(c, b, selected)
		Type.UI.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).ZIndex(1).MaxLines(1).Ellipsis("…")
	})
	return b
}

func (s *Segments) Capsule(c *ui.Context, owner ui.Element, selected bool) {
	capsule(c, owner, selected, s.group, Radii.Segment, Materials.Raised)
}

// RowCapsule marks the selected row of a list with a raised sheet that
// slides between rows of the same group.
func RowCapsule(c *ui.Context, owner ui.Element, selected bool, group string) {
	capsule(c, owner, selected, group, Radii.M, Materials.Raised)
}

// RowWash marks a selected row that echoes a selection made elsewhere, as a
// file whose document already has the raised capsule among the open ones.
func RowWash(c *ui.Context, owner ui.Element, selected bool, group string) {
	capsule(c, owner, selected, group, Radii.M, flat(Materials.GlassHover))
}

func capsule(c *ui.Context, _ ui.Element, selected bool, group string, radius float32, surface Surface) {
	if !selected {
		return
	}
	capsule := ui.Row(c.Key("capsule")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).AccessHidden(true).IgnoreInput(true)
	surface.Apply(capsule)
	if group != "" {
		capsule.Transition(Transition(c, c.KeyboardInput(), ui.ElementTransition{Match: "capsule:" + group, Duration: Motion.Switch, Ease: Motion.EaseInOut.Easing()}))
	}
}

func InputFrame(c *ui.Context) ui.Element {
	f := ui.Row(c).Height(Form.Field).Padding(0, Form.PopoverPad, 0, Form.PadX).Gap(Form.Gap).Radius(Radii.M).AlignItems(ui.Center)
	if f.FocusWithin() {
		Surface{Colors.Paper, Colors.Paper, Rings.Field}.Apply(f)
	} else {
		Materials.Sunken.Apply(f)
	}
	return f
}

type PopoverSpec struct {
	Width, MaxHeight float32
	Title            string
	Placement        ui.Anchor
	Opaque           bool
}

func PopoverWithSpec(c *ui.Context, anchor ui.Element, open *bool, spec PopoverSpec, build func()) ui.Element {
	return popover(c, anchor, open, spec, func() {
		body := func() {
			if spec.Title != "" {
				ui.Row(c).FillWidth().Height(Form.PopoverHeader).Padding(0, Form.PadX).AlignItems(ui.Center).Children(func() {
					PanelTitle(c, spec.Title)
				})
			}
			build()
		}
		if spec.MaxHeight > 0 {
			ui.Scroll(c).MaxHeight(spec.MaxHeight).Children(body)
		} else {
			body()
		}
	})
}

func Popover(c *ui.Context, anchor ui.Element, open *bool, build func()) ui.Element {
	return popover(c, anchor, open, PopoverSpec{}, build)
}

func popover(c *ui.Context, anchor ui.Element, open *bool, spec PopoverSpec, build func()) ui.Element {
	keys := ui.Local(anchor, "popover keys", func() KeyTrigger { return KeyTrigger{} })
	keyboard := keys != nil && keys.Live(c) || anchor.FocusVisible()
	return ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.ObserveInput(func(ev ui.InputEvent) {
			if keys != nil {
				keys.Observe(ev)
			}
		})
		anchor.ObserveInput(func(ev ui.InputEvent) {
			if keys != nil && ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEscape {
				keys.dismiss = true
			}
		})
		panel.MinWidth(anchor.Bounds().W).Padding(Form.PopoverPad).Radius(Radii.L).Gap(Form.PopoverGap)
		if spec.Placement == ui.AnchorTopStart {
			panel.Margin(0, 0, Form.PopoverPad, 0).AttachTo(anchor, ui.AnchorTopStart, ui.AnchorBottomStart)
		} else {
			panel.Margin(Form.PopoverPad, 0, 0, 0)
		}
		if spec.Width > 0 {
			panel.Width(spec.Width)
		}
		if spec.Opaque {
			Materials.Pop.Apply(panel)
		} else {
			Materials.GlassOverlay.Apply(panel)
		}
		panel.AttachedOrigin().Transition(Overlays.Popup.Transition(c, keyboard))
		build()
	})
}
