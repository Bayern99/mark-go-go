package portrait

import "github.com/egoist/mygo/ui"

var dangerRest, dangerHover, dangerPress, dangerOutlineRest, dangerOutlineHover, dangerOutlinePress Surface

func dangerSurfaces() {
	dangerRest = Surface{Tones.Danger.Solid, Tones.Danger.Solid, Shadows{
		in(0, 1, 0, 0, white(0.16)),
		in(0, -1, 0, 0, black(0.12)),
		out(0, 1, 2, 0, colorWithAlpha(Tones.Danger.Solid, 0.24)),
	}}
	dangerHover = Surface{Tones.Danger.Solid.Mix(Colors.OnAccent, 0.08), Tones.Danger.Solid.Mix(Colors.OnAccent, 0.08), Shadows{
		in(0, 1, 0, 0, white(0.2)),
		in(0, -1, 0, 0, black(0.14)),
		out(0, 2, 4, 0, colorWithAlpha(Tones.Danger.Solid, 0.3)),
	}}
	dangerPress = Surface{Tones.Danger.Solid.Mix(black(1), 0.08), Tones.Danger.Solid.Mix(black(1), 0.08), Shadows{
		in(0, 1, 2, 0, black(0.2)),
		out(0, 0.5, 1, 0, colorWithAlpha(Tones.Danger.Solid, 0.2)),
	}}
	dangerOutlineRest = Surface{ui.Transparent, ui.Transparent, Shadows{
		in(0, 0, 0, 1, Tones.Danger.Border),
	}}
	dangerOutlineHover = Surface{Tones.Danger.Wash, Tones.Danger.Wash, Shadows{
		in(0, 0, 0, 1, Tones.Danger.Border),
	}}
	dangerOutlinePress = Surface{Tones.Danger.Soft, Tones.Danger.Soft, Shadows{
		in(0, 0, 0, 1, Tones.Danger.Ring),
	}}
}

// ButtonSpec describes the independent axes of a button. Size is one of
// Sizes.Compact, Sizes.Regular, or Sizes.Large; its zero value is Regular.
type ButtonSpec struct {
	Look              Look
	Size              float32
	Leading, Trailing Icon
	Loading, Selected bool
}

// ButtonWith builds a text or icon button from spec. A loading button exposes
// its progress and is disabled, so it cannot trigger the action again.
func ButtonWith(c *ui.Context, title, label string, spec ButtonSpec) ui.Element {
	size, radius, padding, textStyle := buttonSize(spec.Size)
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(size).Radius(radius).Gap(6).AlignItems(ui.Center).Justify(ui.Center)
	if title == "" {
		b.Size(size, size)
	} else {
		b.Padding(0, padding)
	}
	if label != title {
		Tooltip(c, b, label)
	}
	b.Selected(spec.Selected).Disabled(spec.Loading).Busy(spec.Loading)
	ink := Press(b, spec.Look)
	if spec.Selected && !b.Pressed() {
		ink = selectedButtonLook(b, spec.Look, ink)
	}
	b.Children(func() {
		if spec.Loading {
			Spinner(c, "Loading "+label)
		} else if spec.Leading.Glyph != "" {
			Glyph(c, spec.Leading, IconSizes.S, ink).Shrink(0)
		}
		if title != "" {
			text := textStyle.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…")
			if spec.Look == Link && (b.Hovered() || b.Pressed() || spec.Selected) {
				text.Underline().UnderlineOffset(2)
			}
		}
		if spec.Trailing.Glyph != "" {
			Glyph(c, spec.Trailing, IconSizes.S, ink).Shrink(0)
		}
	})
	return b
}

func buttonSize(size float32) (height, radius, padding float32, text TypeStyle) {
	switch size {
	case Sizes.Compact:
		return size, Radii.S, 8, Type.Meta
	case Sizes.Large:
		return size, Radii.M, 12, Type.UI
	default:
		return Sizes.Regular, Radii.M, 11, Type.UI
	}
}

func selectedButtonLook(b ui.Element, look Look, ink ui.Color) ui.Color {
	switch look {
	case Accent:
		Materials.AccentRaisedPress.Apply(b)
	case Danger:
		dangerPress.Apply(b)
	case DangerOutline:
		dangerOutlinePress.Apply(b)
	default:
		Materials.Row.Apply(b)
	}
	return ink
}

func Button(c *ui.Context, title, label string) ui.Element {
	return ButtonWith(c, title, label, ButtonSpec{Look: Raised})
}

func ButtonLeadingIcon(c *ui.Context, icon Icon, title, label string) ui.Element {
	return ButtonWith(c, title, label, ButtonSpec{Look: Raised, Leading: icon})
}

func AccentButton(c *ui.Context, title, label string) ui.Element {
	return ButtonWith(c, title, label, ButtonSpec{Look: Accent})
}

func QuietButton(c *ui.Context, title, label string) ui.Element {
	return ButtonWith(c, title, label, ButtonSpec{Look: Quiet})
}

func IconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return ButtonWith(c, "", label, ButtonSpec{Look: Raised, Leading: icon})
}

func QuietIconButton(c *ui.Context, icon Icon, label string) ui.Element {
	return ButtonWith(c, "", label, ButtonSpec{Look: Quiet, Leading: icon})
}
