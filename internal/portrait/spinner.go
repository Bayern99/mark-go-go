package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// SpinPeriod is one turn of a spinner (the WebUI's fy-spin, a literal in its
// CSS rather than a token).
const SpinPeriod = 900 * time.Millisecond

// Spinner draws a turning busy indicator named label, an indeterminate
// progress for assistive technology. Under reduced motion it holds still
// (MyGo's Loop keeps running, see Loops), so say what is busy in text too.
func Spinner(c *ui.Context, label string) ui.Element {
	box := ui.Box(c.Key("spinner:"+label)).Size(15, 15).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Role(ui.RoleProgress).Label(label)
	box.Children(func() {
		g := Glyph(c, IconSpinnerGap, 15, Colors.Ink3)
		if Loops(c) {
			g.TransformOrigin(0.5, 0.5).Rotate(360 * box.Loop("spin", SpinPeriod, ui.Linear))
		}
	})
	return box
}
