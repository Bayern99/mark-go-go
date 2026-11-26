package portrait

import (
	"io/fs"

	"github.com/egoist/mygo/ui/editor"
)

type ExportSnapshot struct {
	Space     Space
	Dark      bool
	Colors    Palette
	Materials MaterialSet
	Tones     ToneScale
	GenUI     GenUITokens
	Reader    ReaderTokens
	Code      editor.Theme
	Type      TypeScale
	Radii     RadiusScale
}

func Snapshot(space Space, dark bool) ExportSnapshot {
	colors := paletteFor(space.In(dark), dark)
	materials := materialsFor(colors, dark)
	shimmer := skeletonFor(colors, dark)
	return ExportSnapshot{
		Space: space, Dark: dark,
		Colors: colors, Materials: materials, Tones: tonesFor(colors),
		GenUI: genUIFor(colors, materials, shimmer), Reader: readerFor(colors, dark),
		Code: *codeThemeFor(colors, dark, colors.Focus), Type: Type, Radii: Radii,
	}
}

func FontFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, fs.ErrInvalid
	}
	return typefaces.ReadFile("fonts/" + name)
}
