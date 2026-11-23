package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Frame Component = "Frame"

func init() {
	register(Definition{Name: Frame, Role: ui.RoleGroup, Label: "Preview frame", LongLabel: "文档预览框架", AllowedSurfaces: []Surface{Paper}, build: frameCell})
}

func frameCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	frame := portrait.Frame(c, func() {
		ui.Column(c).FillWidth().Padding(8).Children(func() {
			portrait.Type.UI.Apply(ui.Text(c, label)).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…")
		})
	}).Size(112, 56).Role(ui.RoleGroup).Label(label)
	if cell.State == FocusVisible {
		frame.Focusable()
	}
	return controlState(frame, cell)
}
