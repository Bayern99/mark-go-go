package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Tooltip Component = "Tooltip"

func init() {
	register(Definition{Name: Tooltip, Role: ui.RoleButton, Label: "Document details", LongLabel: "查看文档详细信息与完整编辑历史", build: tooltipCell})
}

func tooltipCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	button := portrait.Button(c, label, label)
	portrait.Tooltip(c, button, label)
	return controlState(button, cell)
}
