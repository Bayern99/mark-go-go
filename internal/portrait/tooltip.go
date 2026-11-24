package portrait

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Tooltip describes anchor and, when keys are supplied, shows their Kbd
// keycaps alongside the description.
func Tooltip(c *ui.Context, anchor ui.Element, text string, keys ...string) ui.Element {
	if text == "" {
		return anchor
	}
	description := text
	if len(keys) != 0 {
		description += " (" + strings.Join(keys, " ") + ")"
	}
	anchor.Description(description)
	t := c.Theme()
	fill, ink := t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if ink == ui.Transparent {
		ink = t.Background
	}
	keyboard := anchor.FocusVisible()
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.MaxWidth(Form.MenuMax).Padding(Form.LabelGap, Form.PadX).Radius(Radii.S).
			Background(fill).TextColor(ink)
		Type.Meta.Apply(tip)
		Shadows{out(0, 2, 8, 0, black(0.2))}.Apply(tip)
		tip.Children(func() {
			if len(keys) == 0 {
				ui.Text(c, text)
				return
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
				ui.Text(c, text).MinWidth(0).Wrap()
				Kbd(c, keys...).Shrink(0)
			})
		})
		tip.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, Form.Gap, 0).
			AttachedOrigin().Transition(Overlays.Tooltip.Transition(c, keyboard))
	})
	return anchor
}
