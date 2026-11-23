package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	Alert        Component = "Alert"
	AlertSuccess Component = "AlertSuccess"
	AlertInfo    Component = "AlertInfo"
	AlertWarning Component = "AlertWarning"
	AlertDanger  Component = "AlertDanger"
)

func init() {
	for _, d := range []struct {
		name             Component
		label, longLabel string
	}{
		{Alert, "Status", "当前文档状态已更新"},
		{AlertSuccess, "Saved", "文档已成功保存"},
		{AlertInfo, "For your information", "这里有一条需要阅读的信息"},
		{AlertWarning, "Check this file", "磁盘上的文件已经发生变化"},
		{AlertDanger, "Something went wrong", "操作失败，请检查错误详情"},
	} {
		register(Definition{Name: d.name, Role: ui.RoleAlert, Label: d.label, LongLabel: d.longLabel, build: alertCell})
	}
}

func alertCell(c *ui.Context, cell Cell) ui.Element {
	return controlState(portrait.Alert(c, alertTone(cell.Component), labelFor(cell), ""), cell)
}

func alertTone(component Component) portrait.Tone {
	switch component {
	case AlertSuccess:
		return portrait.Tones.Success
	case AlertInfo:
		return portrait.Tones.Info
	case AlertWarning:
		return portrait.Tones.Warning
	case AlertDanger:
		return portrait.Tones.Danger
	default:
		return portrait.Tones.Neutral
	}
}
