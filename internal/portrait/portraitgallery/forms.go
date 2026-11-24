package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	TextFieldComponent     Component = "TextField"
	SearchFieldComponent   Component = "SearchField"
	TextAreaComponent      Component = "TextArea"
	SelectComponent        Component = "Select"
	StepperComponent       Component = "Stepper"
	MenuComponent          Component = "Menu"
	ContextMenuComponent   Component = "ContextMenu"
	PopoverComponent       Component = "Popover"
	DialogShellComponent   Component = "DialogShell"
	SettingsGroupComponent Component = "SettingsGroup"
	DisclosureComponent    Component = "Disclosure"
	OutputBlockComponent   Component = "OutputBlock"
	ChipComponent          Component = "Chip"
	ListRowComponent       Component = "ListRow"
	SpaceCardComponent     Component = "SpaceCard"
)

func init() {
	register(Definition{Name: TextFieldComponent, Role: ui.RoleTextField, Label: "Profile name", LongLabel: "个人资料显示名称与完整描述", build: textFieldCell})
	register(Definition{Name: SearchFieldComponent, Role: ui.RoleTextField, Label: "Search settings", LongLabel: "搜索所有设置与首选项", build: searchFieldCell})
	register(Definition{Name: TextAreaComponent, Role: ui.RoleTextField, Label: "Reader note", LongLabel: "阅读笔记与详细批注内容", build: textAreaCell})
	register(Definition{Name: SelectComponent, Role: ui.RoleMenuButton, Label: "Model select", LongLabel: "模型提供方与默认模型选择", build: selectCell})
	register(Definition{Name: StepperComponent, Role: ui.RoleGroup, Label: "Value", LongLabel: "阅读文字大小数值", build: stepperCell})
	register(Definition{Name: MenuComponent, Role: ui.RoleMenuButton, Label: "Actions", LongLabel: "文档操作与更多命令", build: menuCell})
	register(Definition{Name: ContextMenuComponent, Role: ui.RoleButton, Label: "Paragraph actions", LongLabel: "段落编辑与链接操作", build: contextMenuCell})
	register(Definition{Name: PopoverComponent, Role: ui.RoleButton, Label: "Open details", LongLabel: "打开详细设置与说明", build: popoverCell})
	register(Definition{Name: DialogShellComponent, Role: ui.RoleButton, Label: "Open dialog", LongLabel: "打开设置对话框与维护操作", build: dialogCell})
	register(Definition{Name: SettingsGroupComponent, Role: ui.RoleGroup, Label: "Text size", LongLabel: "阅读文字大小与行宽设置", build: settingsGroupCell})
	register(Definition{Name: DisclosureComponent, Role: ui.RoleDisclosure, Label: "Advanced", LongLabel: "高级诊断与命令输出", build: disclosureCell})
	register(Definition{Name: OutputBlockComponent, Role: ui.RoleGroup, Label: "Doctor output", LongLabel: "诊断命令完整输出内容", build: outputCell})
	register(Definition{Name: ChipComponent, Role: ui.RoleButton, Label: "Topic", LongLabel: "研究主题与附加来源", build: chipCell})
	register(Definition{Name: ListRowComponent, Role: ui.RoleButton, Label: "Open notes", LongLabel: "打开工作区阅读笔记文档", build: listRowCell})
	register(Definition{Name: SpaceCardComponent, Role: ui.RoleButton, Label: "Space: Wood–Fire", LongLabel: "Space: 木火工作区完整名称", build: spaceCardCell})
}

func textFieldCell(c *ui.Context, cell Cell) ui.Element {
	value := ""
	field := portrait.TextField(c, &value, portrait.FieldSpec{Label: labelFor(cell), Placeholder: labelFor(cell), Invalid: cell.State == Invalid, Disabled: cell.State == Disabled})
	field.Frame().Width(112)
	return controlState(field.Input(), cell)
}
func searchFieldCell(c *ui.Context, cell Cell) ui.Element {
	value := ""
	field := portrait.SearchField(c, &value, portrait.SearchSpec{Label: labelFor(cell), Size: portrait.Sizes.Regular})
	field.Frame().Width(112)
	return controlState(field.Input(), cell)
}
func textAreaCell(c *ui.Context, cell Cell) ui.Element {
	value := labelFor(cell)
	entry := portrait.TextArea(c, &value, portrait.AreaSpec{Label: labelFor(cell), Rows: 1, MinHeight: 48, Disabled: cell.State == Disabled, Invalid: cell.State == Invalid})
	return controlState(entry, cell)
}
func selectCell(c *ui.Context, cell Cell) ui.Element {
	value := labelFor(cell)
	return controlState(portrait.Select(c, &value, []portrait.Option{{Value: value, Label: value}}, portrait.SelectSpec{Label: labelFor(cell), Width: 112}), cell)
}
func stepperCell(c *ui.Context, cell Cell) ui.Element {
	value := float32(16)
	var stepper ui.Element
	ui.Column(c).Children(func() {
		portrait.Caption(c, labelFor(cell))
		stepper = portrait.Stepper(c, &value, 15, 18, 1, nil).Label(labelFor(cell))
	})
	if cell.State == FocusVisible {
		stepper.Focusable()
	}
	return controlState(stepper, cell)
}
func menuCell(c *ui.Context, cell Cell) ui.Element {
	open := false
	trigger := portrait.Menu(c, &open, func(menu *portrait.MenuBuilder) { menu.Item(portrait.MenuItemSpec{Label: "Edit"}) })
	trigger.Label(labelFor(cell)).Height(portrait.Sizes.Regular).Padding(0, 8).Radius(portrait.Radii.M)
	trigger.Children(func() { portrait.Label(c, labelFor(cell)) })
	return controlState(trigger, cell)
}
func contextMenuCell(c *ui.Context, cell Cell) ui.Element {
	open := false
	target := ui.ButtonBase(c).Label(labelFor(cell)).Height(portrait.Sizes.Regular).Padding(0, 8)
	target.Children(func() { portrait.Label(c, labelFor(cell)) })
	portrait.ContextMenu(c, target, &open, func(menu *portrait.MenuBuilder) { menu.Item(portrait.MenuItemSpec{Label: "Copy"}) })
	return controlState(target, cell)
}
func popoverCell(c *ui.Context, cell Cell) ui.Element {
	open := false
	anchor := portrait.QuietButton(c, labelFor(cell), labelFor(cell))
	portrait.Popover(c, anchor, &open, func() { portrait.Caption(c, "Details") })
	return controlState(anchor, cell)
}
func dialogCell(c *ui.Context, cell Cell) ui.Element {
	open := false
	portrait.DialogShell(c, "gallery dialog", &open, portrait.DialogSpec{Label: "Gallery"}, func() {})
	return controlState(portrait.Button(c, labelFor(cell), labelFor(cell)), cell)
}
func settingsGroupCell(c *ui.Context, cell Cell) ui.Element {
	var row ui.Element
	group := portrait.SettingsGroup(c, "Group", func(rows *portrait.Rows) {
		row = rows.Row(portrait.RowSpec{Title: labelFor(cell)}, func() {})
	})
	_ = group
	if cell.State == FocusVisible {
		row.Focusable()
	}
	return controlState(row, cell)
}
func disclosureCell(c *ui.Context, cell Cell) ui.Element {
	open := false
	root := portrait.Disclosure(c, labelFor(cell), &open, portrait.DisclosureSpec{}, func() { portrait.Caption(c, "Body") }).Role(ui.RoleDisclosure).Label(labelFor(cell))
	if cell.State == FocusVisible {
		root.Focusable()
	}
	return controlState(root, cell)
}
func outputCell(c *ui.Context, cell Cell) ui.Element {
	root := portrait.OutputBlock(c, labelFor(cell), "Output").Role(ui.RoleGroup).Label(labelFor(cell))
	if cell.State == FocusVisible {
		root.Focusable()
	}
	return controlState(root, cell)
}
func chipCell(c *ui.Context, cell Cell) ui.Element {
	return controlState(portrait.Chip(c, portrait.IconHash, labelFor(cell), nil), cell)
}
func listRowCell(c *ui.Context, cell Cell) ui.Element {
	return controlState(portrait.ListRow(c, portrait.ListRowSpec{Title: labelFor(cell), Label: labelFor(cell)}), cell)
}
func spaceCardCell(c *ui.Context, cell Cell) ui.Element {
	space := portrait.Spaces[1]
	space.Label = labelFor(cell)[len("Space: "):]
	return controlState(portrait.SpaceCard(c, space, cell.State == Selected, false).Width(112), cell)
}
