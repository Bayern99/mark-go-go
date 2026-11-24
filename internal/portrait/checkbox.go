package portrait

import "github.com/egoist/mygo/ui"

const (
	checkboxSize   = 16
	checkboxGap    = 8
	checkboxStroke = 1.5
)

// Checkbox creates a Portrait check box editing checked. Disable or mark it
// invalid with the returned element's Disabled and Invalid methods.
func Checkbox(c *ui.Context, checked *bool, label string) ui.Element {
	return checkboxWithLabel(c, checked, label, Type.Small, 500)
}

// CompactCheckbox is the check box for toolbars and cards, where its label has
// to sit at 13/18 like the buttons and card titles beside it. Checkbox keeps
// the 14/22 label that Decision dialogs and reader surfaces are set in.
func CompactCheckbox(c *ui.Context, checked *bool, label string) ui.Element {
	return checkboxWithLabel(c, checked, label, Type.UI, 400)
}

func checkboxWithLabel(c *ui.Context, checked *bool, label string, style TypeStyle, weight int) ui.Element {
	control := ui.CheckboxBase(c, checked).Gap(checkboxGap).AlignItems(ui.Center).FocusRing(false)
	state := ui.CheckOff
	if *checked {
		state = ui.CheckOn
	}
	checkboxFace(c, control, state, label, style, weight)
	return control
}

// CheckboxState creates a Portrait check box that can also report a mixed
// state, as a select-all control over a partially selected group does.
func CheckboxState(c *ui.Context, state *ui.CheckState, label string) ui.Element {
	control := ui.CheckboxStateBase(c, state).Gap(checkboxGap).AlignItems(ui.Center).FocusRing(false)
	CheckboxFace(c, control, *state, label)
	return control
}

// CheckboxFace paints the one checkbox organ shared by settings, GenUI and
// Markdown Preview. The control supplies interaction state; the face owns all
// geometry and color so every surface stays pixel-identical.
func CheckboxFace(c *ui.Context, control ui.Element, state ui.CheckState, label string) {
	checkboxFace(c, control, state, label, Type.Small, 500)
}

func checkboxFace(c *ui.Context, control ui.Element, state ui.CheckState, label string, style TypeStyle, weight int) {
	control.Children(func() {
		fill, border := Colors.Paper, Colors.LineStrong
		if state != ui.CheckOff {
			fill, border = Tones.Accent.Solid, Tones.Accent.Solid
		} else if control.Hovered() {
			fill, border = Colors.AccentSoft, Tones.Accent.Solid
		}
		if c.Preferences().HighContrast && state == ui.CheckOff {
			border = Colors.Ink2
		}
		box := ui.Box(c).Size(checkboxSize, checkboxSize).Radius(Radii.XS).Shrink(0).
			Background(fill).Border(1, border).AccessHidden(true)
		box.DrawOver(func(p *ui.Painter, r ui.Rect) {
			switch state {
			case ui.CheckOn:
				var mark ui.Path
				mark.MoveTo(r.X+r.W*0.22, r.Y+r.H*0.52).
					LineTo(r.X+r.W*0.42, r.Y+r.H*0.71).
					LineTo(r.X+r.W*0.78, r.Y+r.H*0.31)
				p.StrokePath(&mark, checkboxStroke, Tones.Accent.OnSolid)
			case ui.CheckMixed:
				p.Line(r.X+r.W*0.25, r.Y+r.H*0.5, r.X+r.W*0.75, r.Y+r.H*0.5, checkboxStroke, Tones.Accent.OnSolid)
			}
			if control.FocusVisible() {
				p.FocusRing(r, [4]float32{Radii.XS, Radii.XS, Radii.XS, Radii.XS})
			}
		})
		if label != "" {
			style.Apply(ui.Text(c, label)).FontWeight(weight).TextColor(Colors.Ink).MinWidth(0).Wrap().AccessHidden(true)
		}
	})
	if label != "" {
		control.Label(label)
	}
}
