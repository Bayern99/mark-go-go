package portrait

import "github.com/egoist/mygo/ui"

func Tooltip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor.Description(text)
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
		tip.MaxWidth(t.Space(80)).Padding(t.Space(1.25), t.Space(2)).Radius(t.Space(1.25)).
			Background(fill).TextColor(ink).FontSize(t.FontSize-1).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.2))
		tip.Children(func() { ui.Text(c, text) })
		tip.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, t.Space(1.5), 0).
			AttachedOrigin().Transition(Overlays.Tooltip.Transition(c, keyboard))
	})
	return anchor
}
