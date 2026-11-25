package portrait

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Kbd presents one or more keyboard keys as a compact group of keycaps.
func Kbd(c *ui.Context, keys ...string) ui.Element {
	return KbdTint(c, Colors.Ink3, Colors.Fill, keys...)
}

func KbdTint(c *ui.Context, ink, fill ui.Color, keys ...string) ui.Element {
	group := ui.Row(c).Gap(3).AlignItems(ui.Center).MinWidth(0).
		Role(ui.RoleText).Label(strings.Join(keys, " "))
	group.Children(func() {
		for _, key := range keys {
			cap := ui.Row(c).Height(Sizes.Compact).MinWidth(Sizes.Compact).Padding(0, 4).
				Radius(Radii.S).Background(fill).Border(0.5, ink.Alpha(0.35)).
				AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleNone)
			cap.Children(func() {
				Type.Meta.Apply(ui.Text(c, key)).TextColor(ink).MaxLines(1)
			})
		}
	})
	return group
}
