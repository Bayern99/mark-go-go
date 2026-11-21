package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"strings"
	"unicode"
)

var (
	paper   = ui.Hex("#ffffff")
	ink     = ui.Hex("#1d1d1f")
	muted   = ui.Hex("#6e6e73")
	accent  = ui.Hex("#2f6f62")
	danger  = ui.Hex("#a83a3a")
	warning = ui.Hex("#9a4d12")
	rim     = ui.RGBA(0, 0, 0, 0.10)
	track   = ui.Hex("#f2f2f4")
)

func portraitTheme() ui.Theme {
	t := *ui.LightTheme()
	t.Font, t.FontSize = "IBM Plex Sans", 13
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = paper, paper, ui.Hex("#f6f6f8"), track
	t.Text, t.TextMuted, t.Border, t.Accent, t.AccentText = ink, muted, rim, accent, paper
	t.AccentHover, t.AccentPressed, t.Danger, t.Warning, t.Focus = ui.Hex("#37796b"), ui.Hex("#2a6357"), danger, warning, accent
	return t
}

// panel is the pane of navigation or research. Docked it is glass; inside a
// drawer it lies on the drawer's overlay glass and has none of its own.
func panel(c *ui.Context, docked bool) ui.Element {
	e := ui.Column(c).Fill().Padding(8).Gap(6).Radius(13).Clip()
	if docked {
		portrait.Materials.Glass.Apply(e)
	}
	return e
}

func bodyFont(text string) string {
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.In(r, unicode.Han) }) {
		return "IBM Plex Sans SC"
	}
	return "IBM Plex Sans"
}
func banner(c *ui.Context, text string) {
	ui.Text(c, text).Role(ui.RoleAlert).FontSize(13).TextColor(danger).Padding(5, 12).Background(ui.Hex("#fff4f3"))
}
