package portrait

import "github.com/egoist/mygo/ui"

type DiffTokens struct {
	Range, Lines, Conflict  TypeStyle
	Insert, Delete          ui.Color
	NumberWidth, SigilWidth float32
}

const MonoAdvance = 0.6

var Diff DiffTokens

func diffFor(p Palette, dark bool) DiffTokens {
	insert, remove := float32(0.12), float32(0.10)
	if dark {
		insert, remove = 0.16, 0.14
	}
	return DiffTokens{
		Range:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.3, 0},
		Lines:       TypeStyle{FontMono, Type.Meta.Size, Type.Meta.Size * 1.45, 0},
		Conflict:    Type.Meta,
		Insert:      p.Success.Alpha(insert),
		Delete:      p.Danger.Alpha(remove),
		NumberWidth: 3 * MonoAdvance * Type.Meta.Size,
		SigilWidth:  1.5 * MonoAdvance * Type.Meta.Size,
	}
}
