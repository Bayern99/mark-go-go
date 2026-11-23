package portrait

import "github.com/egoist/mygo/ui"

// ConfirmSpec describes a destructive confirmation dialog. Tone defaults to
// Danger. Return identifies the control that receives focus after any close.
type ConfirmSpec struct {
	Title, Body               string
	ConfirmLabel, CancelLabel string
	Tone                      Tone
	Return                    *ui.Handle
	OnConfirm                 func()
}

// Confirm shows a modal confirmation while open is true. Cancel, Escape, and
// a backdrop click dismiss without confirming; confirming closes first and
// then calls OnConfirm. Every close path returns focus to spec.Return.
func Confirm(c *ui.Context, open *bool, spec ConfirmSpec) ui.Element {
	if spec.ConfirmLabel == "" {
		spec.ConfirmLabel = "Confirm"
	}
	if spec.CancelLabel == "" {
		spec.CancelLabel = "Cancel"
	}
	if spec.Tone == (Tone{}) {
		spec.Tone = Tones.Danger
	}
	width, _ := c.Size()
	owner := ui.Box(c.Key("confirm:"+spec.Title)).Absolute().Size(0, 0)
	wasOpen := ui.Local(owner, "was open", func() bool { return false })
	var panel ui.Element
	owner.Children(func() {
		panel = ui.DialogBase(c, open, func(back, panel ui.Element) {
			back.Background(Colors.ModalScrim)
			panel.Width(min(420, width-48)).Padding(20).Gap(12).Radius(Radii.L).
				Role(ui.RoleAlertDialog).Label(spec.Title).Description(spec.Body).
				Transition(Overlays.Popup.Transition(c, c.KeyboardInput()))
			Materials.Sheet.Apply(panel)
			panel.Children(func() {
				Type.Title.Apply(ui.Text(c, spec.Title)).FontWeight(600).TextColor(Colors.Ink).
					MinWidth(0).Wrap().AccessHidden(true)
				if spec.Body != "" {
					Type.Small.Apply(ui.Text(c, spec.Body)).TextColor(Colors.Ink2).
						MinWidth(0).Wrap().AccessHidden(true)
				}
				ui.Row(c).Justify(ui.End).AlignItems(ui.Center).Gap(12).Wrap().Children(func() {
					cancel := Button(c, spec.CancelLabel, spec.CancelLabel).AutoFocus()
					if cancel.Clicked() {
						*open = false
					}
					confirm := confirmToneButton(c, spec.ConfirmLabel, spec.Tone)
					if confirm.Clicked() {
						*open = false
						if spec.OnConfirm != nil {
							spec.OnConfirm()
						}
					}
				})
			})
		})
	})
	if *wasOpen && !*open {
		spec.Return.Focus(c)
	}
	*wasOpen = *open
	return panel
}

func confirmToneButton(c *ui.Context, label string, tone Tone) ui.Element {
	button := ui.ButtonBase(c.Key("confirm action:"+label)).Label(label).
		Height(Sizes.Regular).Padding(0, 11).Radius(Radii.M).
		AlignItems(ui.Center).Justify(ui.Center).Background(tone.Solid)
	button.Children(func() {
		Type.Control.Apply(ui.Text(c, label)).FontWeight(500).TextColor(tone.OnSolid).
			MaxLines(1).Ellipsis("…")
	})
	return button
}
