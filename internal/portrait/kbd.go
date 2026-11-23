package portrait

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Kbd presents one or more keyboard keys as a compact group of keycaps.
func Kbd(c *ui.Context, keys ...string) ui.Element {
	group := ui.Row(c).Gap(3).AlignItems(ui.Center).MinWidth(0).
		Role(ui.RoleText).Label(strings.Join(keys, " "))
	group.Children(func() {
		for _, key := range keys {
			cap := ui.Row(c).Height(Sizes.Compact).MinWidth(Sizes.Compact).Padding(0, 4).
				Radius(Radii.S).Background(Colors.Fill).Border(0.5, Colors.LineStrong).
				AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleNone)
			cap.Children(func() {
				Type.Meta.Apply(ui.Text(c, key)).MaxLines(1)
			})
		}
	})
	return group
}
