package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const SpinPeriod = 900 * time.Millisecond

func Spinner(c *ui.Context, label string) ui.Element {
	box := ui.Box(c.Key("spinner:"+label)).Size(IconSizes.M, IconSizes.M).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Role(ui.RoleProgress).Label(label)
	box.Children(func() {
		g := Glyph(c, IconSpinnerGap, IconSizes.M, Colors.Ink3)
		if Loops(c) {
			g.TransformOrigin(0.5, 0.5).Rotate(360 * box.Loop("spin", SpinPeriod, ui.Linear))
		}
	})
	return box
}
