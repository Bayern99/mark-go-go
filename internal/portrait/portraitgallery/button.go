package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	Button       Component = "Button"
	AccentButton Component = "AccentButton"
	QuietButton  Component = "QuietButton"
	IconButton   Component = "IconButton"
)

func init() {
	register(Definition{Name: Button, Role: ui.RoleButton, Label: "Save", LongLabel: "保存文档并同步所有更改", build: buttonCell})
	register(Definition{Name: AccentButton, Role: ui.RoleButton, Label: "Open", LongLabel: "打开文档并继续上次编辑", build: buttonCell})
	register(Definition{Name: QuietButton, Role: ui.RoleButton, Label: "Settings", LongLabel: "打开偏好设置与账户页面", build: buttonCell})
	register(Definition{Name: IconButton, Role: ui.RoleButton, Label: "Toggle sidebar", LongLabel: "显示或隐藏文档导航侧栏", build: buttonCell})
}

func buttonCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	var b ui.Element
	switch cell.Component {
	case Button:
		b = portrait.Button(c, label, label)
	case AccentButton:
		b = portrait.AccentButton(c, label, label)
	case QuietButton:
		b = portrait.QuietButton(c, label, label)
	case IconButton:
		b = portrait.IconButton(c, portrait.IconSidebar, label)
	default:
		panic("portraitgallery: not a button: " + string(cell.Component))
	}
	return controlState(b, cell)
}
