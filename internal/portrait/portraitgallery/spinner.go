package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Spinner Component = "Spinner"

func init() {
	register(Definition{Name: Spinner, Role: ui.RoleProgress, Label: "Loading document", LongLabel: "正在加载文档并准备全部内容", build: spinnerCell})
}

func spinnerCell(c *ui.Context, cell Cell) ui.Element {
	return controlState(portrait.Spinner(c, labelFor(cell)), cell)
}
