package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Checkbox Component = "Checkbox"

func init() {
	register(Definition{Name: Checkbox, Role: ui.RoleCheckBox, Label: "Sync changes", LongLabel: "同步文档的全部更改", build: checkboxCell})
}

func checkboxCell(c *ui.Context, cell Cell) ui.Element {
	state := ui.CheckOff
	if cell.State == Selected {
		state = ui.CheckOn
	}
	control := portrait.CheckboxState(c, &state, labelFor(cell)).Invalid(cell.State == Invalid)
	return controlState(control, cell)
}
