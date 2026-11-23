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

	ButtonCompact Component = "ButtonCompact"
	ButtonLarge   Component = "ButtonLarge"

	AccentButtonCompact Component = "AccentButtonCompact"
	AccentButtonLarge   Component = "AccentButtonLarge"

	QuietButtonCompact Component = "QuietButtonCompact"
	QuietButtonLarge   Component = "QuietButtonLarge"

	DangerButton        Component = "DangerButton"
	DangerButtonCompact Component = "DangerButtonCompact"
	DangerButtonLarge   Component = "DangerButtonLarge"

	DangerOutlineButton        Component = "DangerOutlineButton"
	DangerOutlineButtonCompact Component = "DangerOutlineButtonCompact"
	DangerOutlineButtonLarge   Component = "DangerOutlineButtonLarge"

	LinkButton        Component = "LinkButton"
	LinkButtonCompact Component = "LinkButtonCompact"
	LinkButtonLarge   Component = "LinkButtonLarge"
)

type buttonAxis struct {
	name             Component
	look             portrait.Look
	size             float32
	label, longLabel string
}

var buttonAxes = []buttonAxis{
	{Button, portrait.Raised, portrait.Sizes.Regular, "Save", "保存文档并同步所有更改"},
	{ButtonCompact, portrait.Raised, portrait.Sizes.Compact, "Save", "保存文档并同步所有更改"},
	{ButtonLarge, portrait.Raised, portrait.Sizes.Large, "Save", "保存文档并同步所有更改"},
	{AccentButton, portrait.Accent, portrait.Sizes.Regular, "Open", "打开文档并继续上次编辑"},
	{AccentButtonCompact, portrait.Accent, portrait.Sizes.Compact, "Open", "打开文档并继续上次编辑"},
	{AccentButtonLarge, portrait.Accent, portrait.Sizes.Large, "Open", "打开文档并继续上次编辑"},
	{QuietButton, portrait.Quiet, portrait.Sizes.Regular, "Settings", "打开偏好设置与账户页面"},
	{QuietButtonCompact, portrait.Quiet, portrait.Sizes.Compact, "Settings", "打开偏好设置与账户页面"},
	{QuietButtonLarge, portrait.Quiet, portrait.Sizes.Large, "Settings", "打开偏好设置与账户页面"},
	{DangerButton, portrait.Danger, portrait.Sizes.Regular, "Delete", "删除文档并确认所有更改"},
	{DangerButtonCompact, portrait.Danger, portrait.Sizes.Compact, "Delete", "删除文档并确认所有更改"},
	{DangerButtonLarge, portrait.Danger, portrait.Sizes.Large, "Delete", "删除文档并确认所有更改"},
	{DangerOutlineButton, portrait.DangerOutline, portrait.Sizes.Regular, "Remove", "移除服务器并清除全部配置"},
	{DangerOutlineButtonCompact, portrait.DangerOutline, portrait.Sizes.Compact, "Remove", "移除服务器并清除全部配置"},
	{DangerOutlineButtonLarge, portrait.DangerOutline, portrait.Sizes.Large, "Remove", "移除服务器并清除全部配置"},
	{LinkButton, portrait.Link, portrait.Sizes.Regular, "Learn more", "打开文档并了解完整操作说明"},
	{LinkButtonCompact, portrait.Link, portrait.Sizes.Compact, "Learn more", "打开文档并了解完整操作说明"},
	{LinkButtonLarge, portrait.Link, portrait.Sizes.Large, "Learn more", "打开文档并了解完整操作说明"},
}

func init() {
	for _, axis := range buttonAxes {
		register(Definition{Name: axis.name, Role: ui.RoleButton, Label: axis.label, LongLabel: axis.longLabel, build: buttonCell})
	}
	register(Definition{Name: IconButton, Role: ui.RoleButton, Label: "Toggle sidebar", LongLabel: "显示或隐藏文档导航侧栏", build: buttonCell})
}

func buttonCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	spec := portrait.ButtonSpec{Loading: cell.State == Loading, Selected: cell.State == Selected}
	if cell.Component == IconButton {
		spec.Look = portrait.Raised
		spec.Leading = portrait.IconSidebar
		return controlState(portrait.ButtonWith(c, "", label, spec), cell)
	}
	for _, axis := range buttonAxes {
		if axis.name == cell.Component {
			spec.Look = axis.look
			spec.Size = axis.size
			return controlState(portrait.ButtonWith(c, label, label, spec), cell)
		}
	}
	panic("portraitgallery: not a button: " + string(cell.Component))
}
