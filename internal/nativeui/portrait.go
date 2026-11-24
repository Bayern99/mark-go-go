package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const collapsible float32 = 0

func panel(c *ui.Context, docked bool) ui.Element {
	e := ui.Column(c).Fill().Padding(8).Gap(6).Radius(portrait.Shell.PageRadius).Clip()
	if docked {
		portrait.Materials.Page.Apply(e)
	}
	return e
}

func sheet(c *ui.Context) ui.Element {
	return portrait.Materials.Page.Apply(ui.Column(c).Fill().MinWidth(0).Radius(portrait.Shell.PageRadius).Clip())
}

func scrollEdgeMask(angle float32) ui.LinearGradient {
	return ui.LinearGradient{Angle: angle, Stops: []ui.GradientStop{
		{Color: ui.Transparent, At: 0},
		{Color: portrait.Colors.Paper, At: 0.035},
		{Color: portrait.Colors.Paper, At: 0.965},
		{Color: ui.Transparent, At: 1},
	}}
}
