package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func panel(c *ui.Context, docked bool) ui.Element {
	e := ui.Column(c).Fill().Padding(8).Gap(6).Radius(portrait.Radii.XL).Clip()
	if docked {
		portrait.Materials.Glass.Apply(e)
	}
	return e
}

func scrollEdgeMask(angle float32) ui.LinearGradient {
	return ui.LinearGradient{Angle: angle, Stops: []ui.GradientStop{
		{Color: ui.Transparent, At: 0},
		{Color: portrait.Colors.Paper, At: 0.035},
		{Color: portrait.Colors.Paper, At: 0.965},
		{Color: ui.Transparent, At: 1},
	}}
}
