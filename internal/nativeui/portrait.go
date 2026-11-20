package nativeui

import "github.com/egoist/mygo/ui"

var (
	paper = ui.Hex("#ffffff")
	ink = ui.Hex("#1d1d1f")
	muted = ui.Hex("#6e6e73")
	accent = ui.Hex("#2f6f62")
	danger = ui.Hex("#a83a3a")
	warning = ui.Hex("#9a4d12")
	rim = ui.RGBA(0,0,0,0.10)
	track = ui.Hex("#f2f2f4")
)

func portraitTheme() ui.Theme {
	t := *ui.LightTheme()
	t.Font, t.FontSize = "IBM Plex Sans",13
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = paper,paper,ui.Hex("#f6f6f8"),track
	t.Text,t.TextMuted,t.Border,t.Accent,t.AccentText = ink,muted,rim,accent,paper
	t.AccentHover,t.AccentPressed,t.Danger,t.Warning,t.Focus = ui.Hex("#37796b"),ui.Hex("#2a6357"),danger,warning,accent
	return t
}

func panel(c *ui.Context) ui.Element {
	return ui.Column(c).Fill().Padding(8).Gap(6).Background(ui.Hex("#fafafb")).Radius(13).Border(0.7,rim).Shadow(0,2,6,0,ui.RGBA(0,0,0,0.04)).Clip()
}

func control(c *ui.Context, title, label string) ui.Element {
	b:=ui.Button(c.Key(label),title).Label(label).Tooltip(label).Height(28).Padding(3,8).Radius(8).FontSize(13).TextColor(ink).Border(0.6,rim)
	if b.PressedInside(){ b.Background(track) } else if b.Hovered(){ b.Background(paper) }
	return b
}

func banner(c *ui.Context,text string) {
	ui.Text(c,text).Role(ui.RoleAlert).FontSize(13).TextColor(danger).Padding(5,12).Background(ui.Hex("#fff4f3"))
}

func discrete(c *ui.Context,title,label string,selected bool) ui.Element {
	b:=ui.ButtonBase(c.Key(label)).Label(label).Padding(4,8).MinHeight(27).Radius(7).Clip()
	if selected { b.Background(paper).Border(0.5,rim).Shadow(0,1,2,0,ui.RGBA(0,0,0,0.06)) } else if b.Hovered(){ b.Background(track) }
	b.Children(func(){ui.Text(c,title).FontSize(13).MaxLines(1).Ellipsis("…").TextColor(ink)})
	return b
}
