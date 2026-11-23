package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const InputFrame Component = "InputFrame"

func init() {
	register(Definition{Name: InputFrame, Role: ui.RoleTextField, Label: "Search documents", LongLabel: "搜索文档标题与全部正文内容", build: inputCell})
}

func inputCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	value := ""
	frame := portrait.InputFrame(c).Width(cellWidth - 24)
	frame.Children(func() {
		portrait.Glyph(c, portrait.IconMagnifyingGlass, portrait.Type.UI.Size, portrait.Colors.Ink3)
		input := portrait.Type.UI.Apply(ui.TextInputBase(c, &value)).FocusRing(false).Grow(1).MinWidth(0).
			Label(label).Placeholder(label).Invalid(cell.State == Invalid)
		controlState(input, cell)
	})
	return frame
}
