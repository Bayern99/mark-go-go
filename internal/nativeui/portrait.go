package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func panel(c *ui.Context, docked bool) ui.Element {
	e := ui.Column(c).Fill().Padding(8).Gap(6).Radius(13).Clip()
	if docked {
		portrait.Materials.Glass.Apply(e)
	}
	return e
}
