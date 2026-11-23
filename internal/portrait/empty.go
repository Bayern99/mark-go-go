package portrait

import "github.com/egoist/mygo/ui"

// EmptySpec describes an empty state. Action builds its optional call to
// action inside the empty state's layout.
type EmptySpec struct {
	Icon        Icon
	Title       string
	Description string
	Action      func(*ui.Context) ui.Element
}

// Empty presents an icon, title, description, and optional action as a
// vertically stacked empty state.
func Empty(c *ui.Context, spec EmptySpec) ui.Element {
	e := ui.Column(c).Gap(6).AlignItems(ui.Center).MinWidth(0).
		Role(ui.RoleGroup).Label(spec.Title).Description(spec.Description)
	e.Children(func() {
		if spec.Icon.Glyph != "" {
			media := ui.Row(c).Size(Sizes.Large, Sizes.Large).Radius(Radii.M).
				Background(Colors.Fill).AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleNone)
			media.Children(func() { Glyph(c, spec.Icon, Type.Reading.Size, Colors.Ink3) })
		}
		if spec.Title != "" {
			Type.Small.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).
				FillWidth().MinWidth(0).Wrap().TextAlign(ui.Center)
		}
		if spec.Description != "" {
			Type.Meta.Apply(ui.Text(c, spec.Description)).TextColor(Colors.Ink3).
				FillWidth().MinWidth(0).Wrap().TextAlign(ui.Center)
		}
		if spec.Action != nil {
			spec.Action(c)
		}
	})
	return e
}
