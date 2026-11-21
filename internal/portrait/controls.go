package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Look is how a control rests and answers the pointer.
type Look int

const (
	// Raised rests slightly above the surface (the WebUI's .pill).
	Raised Look = iota
	// Accent is Raised in the accent color (.pill.primary).
	Accent
	// Quiet is flat until hovered (.pill.quiet).
	Quiet
)

type feedback struct {
	rest, hover, press Surface
	ink, hoverInk      ui.Color
	sink               float32
}

var looks = [...]feedback{
	Raised: {Materials.Raised, Materials.RaisedHover, Materials.RaisedPress, Colors.Ink, Colors.Ink, 0.98},
	Accent: {Materials.AccentRaised, Materials.AccentRaisedHover, Materials.AccentRaisedPress, Colors.OnAccent, Colors.OnAccent, 0.985},
	Quiet:  {Surface{ui.Transparent, ui.Transparent, nil}, Materials.Row, Materials.RaisedPress, Colors.Ink2, Colors.Ink, 0.97},
}

// Rings are focus and editing rings drawn around a field.
var Rings = struct{ Field, Editing Shadows }{
	Field:   Shadows{out(0, 0, 0, 0.5, Colors.LineStrong), out(0, 0, 0, 3, Colors.Focus)},
	Editing: Shadows{out(0, 0, 0, 1, Colors.Accent), out(0, 0, 0, 3, Colors.Focus)},
}

// hoverShadow is the WebUI's box-shadow transition on raised controls.
const hoverShadow = 140 * time.Millisecond

type hoverKey struct{}
type pressKey struct{}

// Press paints b, a button, in look and gives it pointer feedback: the
// shadow deepens on hover and the button sinks while pressed, easing over
// Motion.Press. It returns the text color for b's content.
func Press(b ui.Element, look Look) ui.Color {
	f := looks[look]
	ease := Motion.EaseOut.Easing()
	hovered, pressed := b.Hovered(), b.Pressed()
	h := b.AnimateTo(hoverKey{}, flag(hovered && !pressed), ui.Animation{Duration: hoverShadow, Ease: ease, NonMotion: true})
	p := b.AnimateTo(pressKey{}, flag(pressed), ui.Animation{Duration: Motion.Press, Ease: ease})
	f.rest.Mix(f.hover, h).Mix(f.press, p).Apply(b)
	scale := 1 - (1-f.sink)*p
	b.TransformOrigin(0.5, 0.5).Translate(0, 0.5*p).Scale(scale, scale)
	if hovered || pressed {
		return f.hoverInk
	}
	return f.ink
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// Button is a raised text button. label names it for assistive technology
// and becomes its tooltip when it differs from title.
func Button(c *ui.Context, title, label string) ui.Element {
	return textButton(c, title, label, Raised)
}

// AccentButton is the raised button for a view's main action.
func AccentButton(c *ui.Context, title, label string) ui.Element {
	return textButton(c, title, label, Accent)
}

// QuietButton is a text button without a surface until hovered.
func QuietButton(c *ui.Context, title, label string) ui.Element {
	return textButton(c, title, label, Quiet)
}

func textButton(c *ui.Context, title, label string, look Look) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(28).Padding(0, 11).Radius(8).Gap(6).AlignItems(ui.Center).Justify(ui.Center)
	if label != title {
		b.Tooltip(label)
	}
	ink := Press(b, look)
	b.Children(func() {
		Type.UI.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…")
	})
	return b
}

// IconButton is a raised 28×28 button showing icon; label is its
// accessible name and tooltip.
func IconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return iconButton(c, icon, label, Raised)
}

// QuietIconButton is an icon button without a surface until hovered, as
// the close buttons of tabs and drawers.
func QuietIconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return iconButton(c, icon, label, Quiet)
}

func iconButton(c *ui.Context, icon Icon, label string, look Look) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(label).Size(28, 28).Radius(8).AlignItems(ui.Center).Justify(ui.Center)
	ink := Press(b, look)
	b.Children(func() { Glyph(c, icon, 15, ink) })
	return b
}

// Segments builds the segments of a Track.
type Segments struct {
	c             *ui.Context
	group         string
	keyboard, saw bool
}

// Track draws a segmented control: a shallow sunken track whose selected
// segment sits on a raised capsule. The capsule slides between segments
// sharing group over Motion.Switch, and lands at once when the keyboard
// moved the selection or the system asks for reduced motion.
func Track(c *ui.Context, group string, build func(s *Segments)) ui.Element {
	t := ui.Row(c.Key("track:" + group)).Role(ui.RoleTabList).Padding(2).Gap(2).Radius(9).AlignItems(ui.Center)
	Surface{Materials.GlassSelected, Materials.GlassSelected, Materials.Track.Shadows}.Apply(t)
	keyboard := ui.Local(t, "keyboard", func() bool { return false })
	s := &Segments{c: c, group: group, keyboard: keyboard != nil && *keyboard}
	t.Children(func() { build(s) })
	if keyboard != nil {
		*keyboard = s.saw
	}
	return t
}

// Segment draws a segment button. label names it.
func (s *Segments) Segment(title, label string, selected bool) ui.Element {
	c := s.c
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(24).Padding(0, 10).Radius(7).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0)
	ink := Colors.Ink3
	if selected || b.Hovered() {
		ink = Colors.Ink
	}
	b.Children(func() {
		s.Capsule(b, selected)
		Type.UI.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).ZIndex(1).MaxLines(1).Ellipsis("…")
	})
	return b
}

// Capsule draws the selected capsule filling the current parent when
// selected, for segments built by hand; owner is the segment's focusable
// button. Content drawn over the capsule needs ZIndex(1).
func (s *Segments) Capsule(owner ui.Element, selected bool) {
	if owner.FocusVisible() {
		s.saw = true
	}
	if !selected {
		return
	}
	capsule := ui.Row(s.c.Key("capsule")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(7).AccessHidden(true).IgnoreInput(true)
	Materials.Raised.Apply(capsule)
	if s.group != "" {
		capsule.Transition(Transition(s.c, s.keyboard || s.saw, ui.ElementTransition{Match: "capsule:" + s.group, Duration: Motion.Switch, Ease: Motion.EaseInOut.Easing()}))
	}
}

// Field is a sunken 30-high row for a text field and its buttons; it rises
// to paper with a focus ring while the field has focus.
func Field(c *ui.Context) ui.Element {
	f := ui.Row(c).Height(30).Padding(0, 4, 0, 8).Gap(6).Radius(8).AlignItems(ui.Center)
	if f.FocusWithin() {
		Surface{Colors.Paper, Colors.Paper, Rings.Field}.Apply(f)
	} else {
		Materials.Sunken.Apply(f)
	}
	return f
}

// Popover shows build in a floating panel of overlay glass attached below
// anchor while *open; Escape or a press outside closes it. It returns the
// panel, nil while closed.
func Popover(c *ui.Context, anchor ui.Element, open *bool, build func()) ui.Element {
	return ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.MinWidth(anchor.Bounds().W).Margin(6, 0, 0, 0).Padding(5).Radius(12).Gap(2)
		Materials.GlassOverlay.Apply(panel)
		build()
	})
}
