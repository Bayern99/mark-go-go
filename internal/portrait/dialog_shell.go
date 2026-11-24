package portrait

import "github.com/egoist/mygo/ui"

type DialogSpec struct {
	Label         string
	Width, Height float32
	Margin        float32
	Radius        float32
	Glass         bool
	PlainScrim    bool
	Nested        bool
	Drawer        bool
	DrawerRight   bool
	DrawerMotion  OverlayMotion
	Keyboard      bool
	OriginX       float32
	Align         ui.Align
	Top           float32
	Role          ui.Role
}

func DialogShell(c *ui.Context, key string, open *bool, spec DialogSpec, build func()) ui.Element {
	windowWidth, windowHeight := c.Size()
	if spec.Width == 0 {
		spec.Width = Form.DialogWidth
	}
	if spec.Radius == 0 {
		spec.Radius = Radii.L
	}
	if spec.Margin == 0 {
		spec.Margin = Form.DialogPad + 4
	}
	if spec.Role == ui.RoleAuto {
		spec.Role = ui.RoleDialog
	}
	return ui.DialogBase(c.Key(key), open, func(back, panel ui.Element) {
		if spec.Nested {
			// A dialog above another modal needs separation, not a second
			// full-strength backdrop blur and scrim.
			back.Background(Colors.Scrim)
		} else if spec.Glass {
			Materials.Backdrop.Apply(back)
		} else if spec.PlainScrim {
			back.Background(Colors.Scrim)
		} else {
			back.Background(Colors.ModalScrim)
		}
		back.AlignItems(ui.Center).Justify(ui.Center)
		if spec.Drawer {
			back.AlignItems(ui.Start).Justify(ui.Start)
			back.TransformOrigin(spec.OriginX, 0.5).Transition(spec.DrawerMotion.Transition(c, spec.Keyboard))
			panel.Absolute().Top(spec.Top)
			if spec.DrawerRight {
				panel.Right(spec.Margin)
			} else {
				panel.Left(spec.Margin)
			}
		}
		if spec.Align == ui.End {
			back.Justify(ui.End)
		}
		if spec.Top > 0 && !spec.Drawer {
			back.Justify(ui.Start)
			panel.Margin(spec.Top, 0, 0, 0)
		}
		panel.Width(min(spec.Width, max(0, windowWidth-2*spec.Margin))).
			Radius(spec.Radius).Clip().Role(spec.Role).Label(spec.Label).
			TransformOrigin(0.5, 0.5)
		if !spec.Drawer {
			back.Transition(Overlays.Scrim.Transition(c, c.KeyboardInput()))
		}
		if spec.Height != 0 {
			panel.Height(min(spec.Height, max(0, windowHeight-2*spec.Margin)))
		}
		Materials.Pop.Apply(panel)
		panel.Children(build)
	})
}
