package portrait

import "github.com/egoist/mygo/ui"

// SpaceCard is the shared Wu Xing preview and selection control.
func SpaceCard(c *ui.Context, space Space, selected, dark bool) ui.Element {
	sc := space.In(dark)
	card := ui.ButtonBase(c.Key("settings-space-" + space.Name)).Label("Space: " + space.Label).AccessSelected(selected).
		Grow(1).MinWidth(0).Padding(Form.AreaPadY).Radius(Radii.L).Background(Colors.Paper)
	if selected {
		card.Border(1.5, Colors.Accent)
	} else {
		card.Border(1, Colors.Line)
		if card.Hovered() {
			card.Background(Colors.Fill)
		}
	}
	card.Children(func() {
		ui.Column(c).FillWidth().Gap(Form.Gap).Children(func() {
			preview := ui.Row(c).FillWidth().Height(Settings.SpacePreview).Padding(Form.PopoverPad).Gap(Form.Gap).Radius(Radii.M).AlignItems(ui.Center)
			space.Preview(preview, dark)
			preview.Children(func() {
				ui.Box(c).Size(IconSizes.XS, IconSizes.XS).Radius(Radii.Pill).Background(sc.Mark).Shrink(0)
				ui.Row(c).Grow(1).FillHeight().Radius(Radii.S).Background(sc.Paper).Padding(0, Form.PadX).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(IconSizes.L+Form.PopoverPad/2, Form.PopoverPad-1).Radius(Radii.Pill).Background(sc.Accent)
				})
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(Form.Gap).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					Label(c, space.Label).FontWeight(600).MaxLines(1).Ellipsis("…")
					Caption(c, space.Element).MaxLines(1)
				})
				if selected {
					Glyph(c, IconCheckCircle, IconSizes.M, Colors.Accent).Shrink(0)
				}
			})
		})
	})
	return card
}
