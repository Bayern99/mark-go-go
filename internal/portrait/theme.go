package portrait

import "github.com/egoist/mygo/ui"

func Theme() ui.Theme {
	base, selection, scrollbar := ui.LightTheme(), Colors.Accent.Alpha(0.24), black(0.32)
	if active.dark {
		base, selection, scrollbar = ui.DarkTheme(), Colors.Accent.Alpha(0.32), white(0.30)
	}
	t := *base
	t.Selection, t.Scrollbar, t.Success = selection, scrollbar, Colors.Success
	t.Font, t.FontSize = FontSans, Type.UI.Size
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = Colors.Paper, Colors.Paper, Colors.Code, Colors.Chrome
	t.Text, t.TextMuted, t.Border = Colors.Ink, Colors.Ink3, Colors.LineStrong
	t.Accent, t.AccentText = Colors.Accent, Colors.OnAccent
	t.AccentHover, t.AccentPressed = Materials.AccentRaisedHover.From, Materials.AccentRaisedPress.From
	t.Danger, t.Warning, t.Focus = Colors.Danger, Colors.Warn, Colors.Accent
	return t
}

type ShellTokens struct {
	WorkspaceName, Section               TypeStyle
	PageInset, PageRadius, PageBar       float32
	Strip, Row, SidebarWidth, DrawerEdge float32
	CommandWidth, CommandTop, CommandRow float32
	Documents, Results, Swatch           float32
	CommandInput                         float32
	SettingsWidth, SettingsHeight        float32
}

var Shell = ShellTokens{
	WorkspaceName:  TypeStyle{FontSans, Type.Small.Size, Type.UI.Leading, -0.005 * Type.Small.Size},
	Section:        TypeStyle{FontSans, Type.Meta.Size, Type.Meta.Leading, 0.02 * Type.Meta.Size},
	PageInset:      8,
	PageRadius:     Radii.L,
	PageBar:        40,
	Strip:          40,
	Row:            Sizes.Large,
	SidebarWidth:   248,
	DrawerEdge:     8,
	CommandWidth:   620,
	CommandTop:     96,
	CommandRow:     36,
	Documents:      8 * Sizes.Large,
	Results:        10 * 36,
	Swatch:         16,
	CommandInput:   52,
	SettingsWidth:  740,
	SettingsHeight: 560,
}

var Clear = white(0)
