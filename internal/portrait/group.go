package portrait

import "github.com/egoist/mygo/ui"

// GroupBuilder collects the action buttons joined by Group.
type GroupBuilder struct {
	buttons []ui.Handle
}

// Add joins button to the group and returns it for state and click handling.
func (g *GroupBuilder) Add(button ui.Element) ui.Element {
	var handle ui.Handle
	button.Bind(&handle)
	g.buttons = append(g.buttons, handle)
	return button
}

// Group lays out action buttons as one horizontally connected control. Unlike
// Track, it has no tab-list or selection behavior; each button keeps its own
// action, name, and place in the keyboard order.
func Group(c *ui.Context, label string, build func(*GroupBuilder)) ui.Element {
	group := ui.Row(c).Role(ui.RoleGroup).Label(label).AlignItems(ui.Stretch).Radius(Radii.M).Border(1, Colors.LineStrong).Clip()
	builder := &GroupBuilder{}
	group.Children(func() { build(builder) })
	for i, handle := range builder.buttons {
		button := c.Resolve(handle)
		button.NoShadow().BorderWidth(0)
		switch {
		case len(builder.buttons) == 1:
			button.Radius(Radii.M)
		case i == 0:
			button.Radius(Radii.M, 0, 0, Radii.M)
		case i == len(builder.buttons)-1:
			button.Radius(0, Radii.M, Radii.M, 0)
		default:
			button.Radius(0)
		}
		if i > 0 {
			button.BorderWidth(0, 0, 0, 1).BorderColor(Colors.LineStrong)
		}
	}
	return group
}
