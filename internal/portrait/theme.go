package portrait

import "github.com/egoist/mygo/ui"

func Theme() ui.Theme {
	t := *ui.LightTheme()
	t.Font, t.FontSize = FontSans, Type.UI.Size
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = Colors.Paper, Colors.Paper, Colors.Code, Colors.Chrome
	t.Text, t.TextMuted, t.Border = Colors.Ink, Colors.Ink3, Colors.LineStrong
	t.Accent, t.AccentText = Colors.Accent, Colors.OnAccent
	t.AccentHover, t.AccentPressed = Materials.AccentRaisedHover.From, Materials.AccentRaisedPress.From
	t.Danger, t.Warning, t.Focus = Colors.Danger, Colors.Warn, Colors.Accent
	return t
}

type ShellTokens struct {
	WorkspaceName TypeStyle
}

var Shell = ShellTokens{
	WorkspaceName: TypeStyle{FontSans, Type.Small.Size, Type.UI.Leading, -0.005 * Type.Small.Size},
}

var Clear = white(0)
