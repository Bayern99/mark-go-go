package portrait

import "github.com/egoist/mygo/ui"

// DiffTokens are the Review diff's type and line tints
// (WebUI plugins/desktop-plugins.css).
type DiffTokens struct {
	Range, Lines, Conflict TypeStyle
	Insert, Delete         ui.Color
	// NumberWidth and SigilWidth are the 3ch and 1.5ch columns of .diff-line.
	NumberWidth, SigilWidth float32
}

// MonoAdvance is IBM Plex Mono's advance, the CSS ch unit, in em.
const MonoAdvance = 0.6

// Diff holds the Review diff tokens: meta-size mono for ranges and lines,
// and accent/danger mixed with transparency for inserted and deleted lines.
var Diff = DiffTokens{
	Range:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.3, 0},
	Lines:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.45, 0},
	Conflict:    Type.Meta,
	Insert:      Colors.Accent.Alpha(0.12),
	Delete:      Colors.Danger.Alpha(0.10),
	NumberWidth: 3 * MonoAdvance * Type.Meta.Size,
	SigilWidth:  1.5 * MonoAdvance * Type.Meta.Size,
}
