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

var Rings = struct{ Field, Editing Shadows }{
	Field:   Shadows{out(0, 0, 0, 0.5, Colors.LineStrong), out(0, 0, 0, 3, Colors.Focus)},
	Editing: Shadows{out(0, 0, 0, 1, Colors.Accent), out(0, 0, 0, 3, Colors.Focus)},
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

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func Button(c *ui.Context, title, label string) ui.Element {
	return textButton(c, Icon{}, title, label, Raised)
}

func ButtonLeadingIcon(c *ui.Context, icon Icon, title, label string) ui.Element {
	return textButton(c, icon, title, label, Raised)
}

func AccentButton(c *ui.Context, title, label string) ui.Element {
	return textButton(c, Icon{}, title, label, Accent)
}

func QuietButton(c *ui.Context, title, label string) ui.Element {
	return textButton(c, Icon{}, title, label, Quiet)
}

func textButton(c *ui.Context, icon Icon, title, label string, look Look) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(28).Padding(0, 11).Radius(8).Gap(6).AlignItems(ui.Center).Justify(ui.Center)
	if label != title {
		Tooltip(c, b, label)
	}
	ink := Press(b, look)
	b.Children(func() {
		if icon.Glyph != "" {
			Glyph(c, icon, Type.UI.Size, ink).Shrink(0)
		}
		Type.UI.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…")
	})
	return b
}

func IconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return iconButton(c, icon, label, Raised)
}

func QuietIconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return iconButton(c, icon, label, Quiet)
}

func iconButton(c *ui.Context, icon Icon, label string, look Look) ui.Element {
	b := Tooltip(c, ui.ButtonBase(c.Key(label)).Label(label), label).Size(28, 28).Radius(8).AlignItems(ui.Center).Justify(ui.Center)
	ink := Press(b, look)
	b.Children(func() { Glyph(c, icon, 15, ink) })
	return b
}

type Segments struct {
	group         string
	keyboard, saw bool
}

func Track(c *ui.Context, group string, build func(s *Segments)) ui.Element {
	t := ui.Row(c.Key("track:" + group)).Role(ui.RoleTabList).Padding(2).Gap(2).Radius(9).AlignItems(ui.Center)
	Surface{Materials.GlassSelected, Materials.GlassSelected, Materials.Track.Shadows}.Apply(t)
	keyboard := ui.Local(t, "keyboard", func() bool { return false })
	s := &Segments{group: group, keyboard: keyboard != nil && *keyboard}
	t.Children(func() { build(s) })
	if keyboard != nil {
		*keyboard = s.saw
	}
	return t
}

func (s *Segments) Segment(c *ui.Context, title, label string, selected bool) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(24).Padding(0, 10).Radius(7).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0)
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
	if owner.FocusVisible() {
		s.saw = true
	}
	if !selected {
		return
	}
	capsule := ui.Row(c.Key("capsule")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(7).AccessHidden(true).IgnoreInput(true)
	Materials.Raised.Apply(capsule)
	if s.group != "" {
		capsule.Transition(Transition(c, s.keyboard || s.saw, ui.ElementTransition{Match: "capsule:" + s.group, Duration: Motion.Switch, Ease: Motion.EaseInOut.Easing()}))
	}
}

func Field(c *ui.Context) ui.Element {
	f := ui.Row(c).Height(30).Padding(0, 4, 0, 8).Gap(6).Radius(8).AlignItems(ui.Center)
	if f.FocusWithin() {
		Surface{Colors.Paper, Colors.Paper, Rings.Field}.Apply(f)
	} else {
		Materials.Sunken.Apply(f)
	}
	return f
}

func Popover(c *ui.Context, anchor ui.Element, open *bool, build func()) ui.Element {
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
		panel.MinWidth(anchor.Bounds().W).Margin(6, 0, 0, 0).Padding(5).Radius(12).Gap(2)
		Materials.GlassOverlay.Apply(panel).AttachedOrigin().Transition(Overlays.Popup.Transition(c, keyboard))
		build()
	})
}
