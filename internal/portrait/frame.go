package portrait

import "github.com/egoist/mygo/ui"

const frameInset = 4

var frameTray = Colors.Fill.Alpha(0.72)

// Card presents content on an opaque sheet with a hairline edge. It is safe
// on Paper, Chrome, and Glass surfaces.
func Card(c *ui.Context, content ...func()) ui.Element {
	e := ui.Column(c).Radius(Radii.M)
	Materials.Sheet.Apply(e)
	if len(content) > 0 {
		e.Children(content[0])
	}
	return e
}

// Frame presents a white panel inside a shallow gray tray. Its outer radius
// includes the four-point inset around the inner card. Gallery registrations
// restrict Frame to Paper so the translucent tray has a predictable backdrop.
func Frame(c *ui.Context, content ...func()) ui.Element {
	tray := ui.Column(c).Padding(frameInset).Radius(Radii.L).Background(frameTray)
	tray.Children(func() {
		panel := Card(c).FillWidth().Grow(1)
		if len(content) > 0 {
			panel.Children(content[0])
		}
	})
	return tray
}
