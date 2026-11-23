package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Switch Component = "Switch"

func init() {
	register(Definition{Name: Switch, Role: ui.RoleSwitch, Label: "Live sync", LongLabel: "实时同步文档更改", build: switchCell})
}

func switchCell(c *ui.Context, cell Cell) ui.Element {
	on := cell.State == Selected
	control := portrait.Switch(c, &on).Label(labelFor(cell)).Invalid(cell.State == Invalid)
	return controlState(control, cell)
}
