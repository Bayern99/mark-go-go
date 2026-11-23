package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Badge Component = "Badge"

func init() {
	register(Definition{Name: Badge, Role: ui.RoleText, Label: "Ready", LongLabel: "所有文档更改均已同步", build: badgeCell})
}

func badgeCell(c *ui.Context, cell Cell) ui.Element {
	tones := map[State]portrait.Tone{
		Rest: portrait.Tones.Neutral, Hover: portrait.Tones.Accent,
		Pressed: portrait.Tones.Success, FocusVisible: portrait.Tones.Info,
		Disabled: portrait.Tones.Warning, Loading: portrait.Tones.Danger,
		Selected: portrait.Tones.Accent, Invalid: portrait.Tones.Danger,
		LongCJK: portrait.Tones.Neutral,
	}
	b := portrait.BadgeLeadingIcon(c, portrait.IconCheckCircle, labelFor(cell), tones[cell.State])
	if cell.State == FocusVisible {
		b.Focusable()
	}
	return controlState(b, cell)
}
