package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Kbd Component = "Kbd"

func init() {
	register(Definition{Name: Kbd, Role: ui.RoleText, Label: "K", LongLabel: "搜索", build: kbdCell})
}

func kbdCell(c *ui.Context, cell Cell) ui.Element {
	keys := portrait.Kbd(c, labelFor(cell))
	if cell.State == FocusVisible {
		keys.Focusable()
	}
	return controlState(keys, cell)
}
