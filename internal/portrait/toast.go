package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// ToastSpec describes one transient operation notification. Reusing ID
// updates the visible toast in place.
type ToastSpec struct {
	ID       string
	Tone     Tone
	Title    string
	Action   string
	OnAction func()
	Timeout  time.Duration
}

// Notify shows or updates a toast in the viewport built by Toasts.
func Notify(c *ui.Context, spec ToastSpec) {
	c.AddToast(ui.Toast{
		ID:       spec.ID,
		Type:     toneType(spec.Tone),
		Title:    spec.Title,
		Action:   spec.Action,
		OnAction: spec.OnAction,
		Timeout:  spec.Timeout,
	})
}

// Toast builds one toast with the same look used by Toasts.
func Toast(c *ui.Context, spec ToastSpec) ui.Element {
	return toast(c, ui.Toast{
		ID:       spec.ID,
		Type:     toneType(spec.Tone),
		Title:    spec.Title,
		Action:   spec.Action,
		OnAction: spec.OnAction,
		Timeout:  spec.Timeout,
	})
}

// Toasts builds the shell's toast viewport. ToastViewportBase owns the
// newest-three limit, timeouts, focus pausing, and Escape dismissal.
func Toasts(c *ui.Context) ui.Element {
	return ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(16).AlignItems(ui.End).Gap(8)
		for _, t := range toasts {
			toast(c, t)
		}
	})
}

func toast(c *ui.Context, t ui.Toast) ui.Element {
	tone := toneFromType(t.Type)
	parts := ui.ToastBase(c, t)
	root := parts.Root.Row().Gap(10).Padding(10, 12).Radius(Radii.L).Border(0.5, tone.Border).
		MaxWidth(380).AlignItems(ui.Center).Label(t.Title)
	Materials.Pop.Apply(root)
	root.Background(tone.Wash)
	enter := ui.Motion{Y: 8, Opacity: 0}
	root.Transition(Transition(c, c.KeyboardInput(), ui.ElementTransition{
		Duration: Motion.Enter,
		Ease:     Motion.EaseOut.Easing(),
		Enter:    &enter,
		Exit:     &enter,
	}))
	root.Children(func() {
		Glyph(c, toneIcon(tone), IconSizes.M, tone.Ink).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			Type.UI.Apply(ui.Text(c, t.Title)).FontWeight(600).TextColor(tone.Ink).MaxLines(2).Ellipsis("…")
			if t.Description != "" {
				Type.Meta.Apply(ui.Text(c, t.Description)).TextColor(tone.Ink).MaxLines(2).Ellipsis("…")
			}
		})
		if t.Action != "" {
			b := parts.ActionButton().Label(t.Action).Height(Sizes.Regular).Padding(0, 8).Radius(Radii.M).AlignItems(ui.Center).Shrink(0)
			ink := Press(b, Quiet)
			b.Children(func() { Type.UI.Apply(ui.Text(c, t.Action)).FontWeight(600).TextColor(ink).MaxLines(1) })
		}
		close := parts.CloseButton().Label("Close").Size(Sizes.Regular, Sizes.Regular).Radius(Radii.M).AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
		ink := Press(close, Quiet)
		close.Children(func() { Glyph(c, IconX, IconSizes.S, ink) })
	})
	return root
}

func toneType(tone Tone) string {
	switch tone {
	case Tones.Success:
		return "success"
	case Tones.Accent:
		return "info"
	case Tones.Warning:
		return "warning"
	case Tones.Danger:
		return "danger"
	default:
		return "neutral"
	}
}

func toneFromType(kind string) Tone {
	switch kind {
	case "success":
		return Tones.Success
	case "accent", "info":
		return Tones.Accent
	case "warning":
		return Tones.Warning
	case "danger", "error":
		return Tones.Danger
	default:
		return Tones.Neutral
	}
}
