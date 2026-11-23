package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Empty Component = "Empty"

func init() {
	register(Definition{Name: Empty, Role: ui.RoleGroup, Label: "No checkpoints", LongLabel: "暂无文档检查点", build: emptyCell})
}

func emptyCell(c *ui.Context, cell Cell) ui.Element {
	empty := portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconCube, Title: labelFor(cell)})
	if cell.State == FocusVisible {
		empty.Focusable()
	}
	return controlState(empty, cell)
}
