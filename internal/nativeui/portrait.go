package nativeui

import (
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

type portraitGlass struct{ overlay, opaque bool }

func (g portraitGlass) PaintMaterial(p *ui.Painter, r ui.Rect, radii [4]float32) {
	tint := ui.RGBA(255, 255, 255, 0.82)
	if g.overlay {
		tint = ui.RGBA(255, 255, 255, 0.96)
	}
	if g.opaque {
		tint = paper
	}
	p.FillRadii(r, radii, tint)
}
func (g portraitGlass) NativeGlass() (ui.NativeGlassSpec, bool) {
	return ui.NativeGlassSpec{Tint: ui.RGBA(255, 255, 255, 0.82)}, !g.opaque
}
func glass(c *ui.Context, overlay bool) portraitGlass {
	return portraitGlass{overlay: overlay, opaque: c.Preferences().ReduceTransparency || c.Preferences().HighContrast}
}
func panel(c *ui.Context) ui.Element {
	return ui.Column(c).Fill().Padding(8).Gap(6).Material(glass(c, false)).Radius(13).Border(0.7, rim).InsetShadow(0, 1, 0, 0, paper).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.05)).Clip()
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
