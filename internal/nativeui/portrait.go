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

// scrollEdgeFade is how much of a scroll viewport fades at an edge the content
// slides under: a narrow stop, so the rows at rest keep their full ink.
const scrollEdgeFade float32 = 0.02

// scrollEdgeMask fades a scrolling list at the edge its content goes under.
// The top fades only once the list has scrolled past its first row, and the
// bottom only while rows wait to be scrolled into view, so the first and the
// last row are whole at rest. ScrollState reports how far the container
// scrolled and how far it can, as the last frame laid it out; the frame after
// a resize or a first fill therefore shows the list without a fade.
func scrollEdgeMask(angle float32, scroll *ui.ScrollState) ui.LinearGradient {
	top, bottom := float32(0), float32(1)
	if scroll != nil {
		if scroll.Y > 1 {
			top = scrollEdgeFade
		}
		if scroll.MaxY-scroll.Y > 1 {
			bottom = 1 - scrollEdgeFade
		}
	}
	return ui.LinearGradient{Angle: angle, Stops: []ui.GradientStop{
		{Color: ui.Transparent, At: 0},
		{Color: portrait.Colors.Paper, At: top},
		{Color: portrait.Colors.Paper, At: bottom},
		{Color: ui.Transparent, At: 1},
	}}
}
