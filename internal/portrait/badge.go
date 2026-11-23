package portrait

import "github.com/egoist/mygo/ui"

// Badge presents a short status label with a semantic tone.
func Badge(c *ui.Context, text string, tone Tone) ui.Element {
	return badge(c, Icon{}, text, tone)
}

// BadgeLeadingIcon presents a badge with a decorative Phosphor icon.
func BadgeLeadingIcon(c *ui.Context, icon Icon, text string, tone Tone) ui.Element {
	return badge(c, icon, text, tone)
}

func badge(c *ui.Context, icon Icon, text string, tone Tone) ui.Element {
	b := ui.Row(c).Height(Sizes.Compact).Padding(0, 7).Gap(4).Radius(Radii.Pill).
		Background(tone.Soft).Border(0.5, tone.Border).AlignItems(ui.Center).MinWidth(0).
		Role(ui.RoleText).Label(text)
	b.Children(func() {
		if icon.Glyph != "" {
			Glyph(c, icon, Type.Meta.Size, tone.Ink).Shrink(0)
		}
		Type.Meta.Apply(ui.Text(c, text)).TextColor(tone.Ink).MinWidth(0).MaxLines(1).Ellipsis("…")
	})
	return b
}
