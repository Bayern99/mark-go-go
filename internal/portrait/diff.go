package portrait

import "github.com/egoist/mygo/ui"

type DiffTokens struct {
	Range, Lines, Conflict  TypeStyle
	Insert, Delete          ui.Color
	NumberWidth, SigilWidth float32
}

const MonoAdvance = 0.6

var Diff = DiffTokens{
	Range:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.3, 0},
	Lines:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.45, 0},
	Conflict:    Type.Meta,
	Insert:      Colors.Accent.Alpha(0.12),
	Delete:      Colors.Danger.Alpha(0.10),
	NumberWidth: 3 * MonoAdvance * Type.Meta.Size,
	SigilWidth:  1.5 * MonoAdvance * Type.Meta.Size,
}
