package nativeui

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
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
func capsule(c *ui.Context, group string, keyboard bool) {
	bg := ui.Row(c.Key("selected")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(7).Background(paper).Border(0.5, rim).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06)).AccessHidden(true)
	if group != "" && !keyboard && !c.Preferences().ReduceMotion {
		bg.Transition(ui.ElementTransition{Match: group, Duration: 160 * time.Millisecond})
	}
}

func control(c *ui.Context, title, label string) ui.Element {
	b := ui.Button(c.Key(label), title).Label(label).Tooltip(label).Height(28).Padding(3, 8).Radius(8).FontSize(13).TextColor(ink).Font(bodyFont(title)).Background(paper).Border(0.6, rim)
	if b.PressedInside() {
		b.Background(track)
	} else if b.Hovered() {
		b.Background(paper)
	}
	return b
}

func banner(c *ui.Context, text string) {
	ui.Text(c, text).Role(ui.RoleAlert).FontSize(13).TextColor(danger).Padding(5, 12).Background(ui.Hex("#fff4f3"))
}

func discrete(c *ui.Context, title, label string, selected bool, group ...string) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Padding(4, 8).MinHeight(27).Radius(7).Clip()
	if !selected && b.Hovered() {
		b.Background(track)
	}
	b.Children(func() {
		if selected {
			key := ""
			if len(group) > 0 {
				key = group[0]
			}
			capsule(c, key, b.FocusVisible())
		}
		ui.Text(c, title).Font(bodyFont(title)).FontSize(13).MaxLines(1).Ellipsis("…").TextColor(ink)
	})
	return b
}

func iconButton(c *ui.Context, glyph, label string) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(label).Size(30, 28).Radius(7).AlignItems(ui.Center).Justify(ui.Center)
	if b.PressedInside() {
		b.Background(track)
	} else if b.Hovered() {
		b.Background(paper).Border(0.5, rim)
	}
	b.Children(func() { ui.Text(c, glyph).Font("Phosphor").FontSize(16).TextColor(ink).AccessHidden(true) })
	return b
}
