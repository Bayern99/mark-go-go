package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	Toast        Component = "Toast"
	ToastSuccess Component = "ToastSuccess"
	ToastInfo    Component = "ToastInfo"
	ToastWarning Component = "ToastWarning"
	ToastDanger  Component = "ToastDanger"
)

func init() {
	for _, d := range []struct {
		name             Component
		label, longLabel string
	}{
		{Toast, "Status", "当前操作状态已更新"},
		{ToastSuccess, "Saved", "文档已成功保存"},
		{ToastInfo, "Generating…", "正在生成文档视图"},
		{ToastWarning, "Check this file", "磁盘上的文件已经发生变化"},
		{ToastDanger, "Something went wrong", "操作失败，请检查错误详情"},
	} {
		register(Definition{Name: d.name, Role: ui.RoleStatus, Label: d.label, LongLabel: d.longLabel, build: toastCell})
	}
}

func toastCell(c *ui.Context, cell Cell) ui.Element {
	return controlState(portrait.Toast(c, portrait.ToastSpec{
		ID:    "gallery:" + string(cell.Component),
		Tone:  toastTone(cell.Component),
		Title: labelFor(cell),
	}), cell)
}

func toastTone(component Component) portrait.Tone {
	switch component {
	case ToastSuccess:
		return portrait.Tones.Success
	case ToastInfo:
		return portrait.Tones.Info
	case ToastWarning:
		return portrait.Tones.Warning
	case ToastDanger:
		return portrait.Tones.Danger
	default:
		return portrait.Tones.Neutral
	}
}
