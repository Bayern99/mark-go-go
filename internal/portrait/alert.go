package portrait

import "github.com/egoist/mygo/ui"

// AlertAction is the optional action attached to an Alert.
type AlertAction struct {
	Label    string
	OnAction func()
}

// Alert presents a titled message with a semantic tone. The optional body
// adds detail and the optional action offers one next step.
func Alert(c *ui.Context, tone Tone, title, body string, actions ...AlertAction) ui.Element {
	label := title
	if label == "" {
		label = body
	}
	e := ui.Row(c).FillWidth().Gap(8).Padding(10, 12).Radius(Radii.M).Border(0.5, tone.Border).Background(tone.Wash).
		AlignItems(ui.Start).Role(ui.RoleAlert).Label(label)
	e.Children(func() {
		Glyph(c, toneIcon(tone), IconSizes.M, tone.Ink).Margin(1, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			if title != "" {
				Type.UI.Apply(ui.Text(c, title)).FontWeight(600).TextColor(tone.Ink)
			}
			if body != "" {
				Type.UI.Apply(ui.Text(c, body)).TextColor(tone.Ink)
			}
		})
		if len(actions) > 0 && actions[0].Label != "" {
			action := actions[0]
			b := ui.ButtonBase(c.Key("alert action")).Label(action.Label).Height(Sizes.Regular).Padding(0, 8).Radius(Radii.M).AlignItems(ui.Center).Shrink(0)
			ink := Press(b, Quiet)
			b.Children(func() { Type.UI.Apply(ui.Text(c, action.Label)).FontWeight(600).TextColor(ink).MaxLines(1) })
			if b.Clicked() && action.OnAction != nil {
				action.OnAction()
			}
		}
	})
	return e
}

func toneIcon(tone Tone) Icon {
	switch tone {
	case Tones.Success:
		return IconCheckCircle
	case Tones.Warning:
		return IconWarning
	case Tones.Danger:
		return IconWarningCircle
	default:
		return IconInfo
	}
}
