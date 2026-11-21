package portrait

import "github.com/egoist/mygo/ui"

// Theme is the MyGo widget theme in Portrait colors, after the WebUI's
// --mgo-* bridge variables.
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

// ShellTokens are the window shell's own type styles.
type ShellTokens struct {
	// WorkspaceName is .ws-name: small size at ui leading, tracked -0.005em.
	WorkspaceName TypeStyle
}

// Shell holds the window shell tokens.
var Shell = ShellTokens{
	WorkspaceName: TypeStyle{FontSans, Type.Small.Size, Type.UI.Leading, -0.005 * Type.Small.Size},
}

// Clear paints nothing, for APIs that read the zero Color as "default".
var Clear = white(0)
