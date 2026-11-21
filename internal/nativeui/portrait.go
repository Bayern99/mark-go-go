package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// panel is the pane of navigation or research. Docked it is glass; inside a
// drawer it lies on the drawer's overlay glass and has none of its own.
func panel(c *ui.Context, docked bool) ui.Element {
	e := ui.Column(c).Fill().Padding(8).Gap(6).Radius(13).Clip()
	if docked {
		portrait.Materials.Glass.Apply(e)
	}
	return e
}

// banner is the WebUI's .banner-conflict: danger on danger-soft.
func banner(c *ui.Context, text string) {
	portrait.Type.UI.Apply(ui.Text(c, text)).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).Padding(5, 12).Background(portrait.Colors.DangerSoft)
}
