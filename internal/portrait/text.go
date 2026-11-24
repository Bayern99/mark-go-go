package portrait

import "github.com/egoist/mygo/ui"

func Label(c *ui.Context, value string) ui.Element {
	return Type.UI.Apply(ui.Text(c, value)).FontWeight(500).TextColor(Colors.Ink)
}

func Caption(c *ui.Context, value string) ui.Element {
	return Type.Meta.Apply(ui.Text(c, value)).TextColor(Colors.Ink3)
}

func SectionLabel(c *ui.Context, value string) ui.Element {
	return Shell.Section.Apply(ui.Text(c, value)).FontWeight(600).TextColor(Colors.Ink3)
}

func GroupHeading(c *ui.Context, value string) ui.Element {
	return Type.Small.Apply(ui.Text(c, value)).FontWeight(600).TextColor(Colors.Ink).Role(ui.RoleHeading)
}

func PanelTitle(c *ui.Context, value string) ui.Element {
	return Type.UI.Apply(ui.Text(c, value)).FontWeight(600).TextColor(Colors.Ink).Role(ui.RoleHeading)
}
