package portrait

import "github.com/egoist/mygo/ui"

type Appearance string

const (
	System Appearance = "System"
	Light  Appearance = "Light"
	Dark   Appearance = "Dark"
)

var Appearances = [...]Appearance{System, Light, Dark}

func AppearanceNamed(name string) Appearance {
	switch Appearance(name) {
	case Light, Dark:
		return Appearance(name)
	}
	return System
}

// IsDark resolves the appearance against the system's.
func (a Appearance) IsDark(systemDark bool) bool {
	switch a {
	case Light:
		return false
	case Dark:
		return true
	}
	return systemDark
}

type lookKey struct {
	space string
	dark  bool
}

type tokens struct {
	colors     Palette
	materials  MaterialSet
	tones      ToneScale
	highlights HighlightSet
	diff       DiffTokens
	skeleton   Skeleton
	genUI      GenUITokens
	reader     ReaderTokens
	tray       ui.Color
	danger     [6]Surface
	looks      [Link + 1]feedback
	rings      struct{ Field, Editing, Invalid Shadows }
}

var (
	active struct {
		space Space
		dark  bool
		key   lookKey
	}
	resolved = map[lookKey]*tokens{}
)

func init() { Use(DefaultSpace, false) }

// Use has every token follow space in the light or the dark appearance.
// A window calls it as it starts to build, so all it builds and paints that
// frame takes the colours of the Space it shows.
func Use(space Space, dark bool) {
	key := lookKey{space.Name, dark}
	if t := resolved[key]; t != nil {
		if active.key != key {
			apply(space, dark, t)
		}
		return
	}
	Colors = paletteFor(space.In(dark), dark)
	Materials = materialsFor(Colors, dark)
	Tones = tonesFor(Colors)
	Highlights = highlightsFor(Colors)
	Diff = diffFor(Colors, dark)
	DefaultSkeleton = skeletonFor(Colors, dark)
	GenUI = genUIFor(Colors)
	Reader = readerFor(Colors, dark)
	frameTray = Colors.Fill.Alpha(0.72)
	dangerSurfaces()
	controlLooks()
	t := &tokens{Colors, Materials, Tones, Highlights, Diff, DefaultSkeleton, GenUI, Reader, frameTray,
		[6]Surface{dangerRest, dangerHover, dangerPress, dangerOutlineRest, dangerOutlineHover, dangerOutlinePress}, looks, Rings}
	resolved[key] = t
	active.space, active.dark, active.key = space, dark, key
}

func apply(space Space, dark bool, t *tokens) {
	Colors, Materials, Tones, Highlights, Diff = t.colors, t.materials, t.tones, t.highlights, t.diff
	DefaultSkeleton, GenUI, Reader, frameTray = t.skeleton, t.genUI, t.reader, t.tray
	dangerRest, dangerHover, dangerPress = t.danger[0], t.danger[1], t.danger[2]
	dangerOutlineRest, dangerOutlineHover, dangerOutlinePress = t.danger[3], t.danger[4], t.danger[5]
	looks, Rings = t.looks, t.rings
	active.space, active.dark, active.key = space, dark, lookKey{space.Name, dark}
}

// ActiveSpace is the Space the tokens follow, and whether they are dark.
func ActiveSpace() (Space, bool) { return active.space, active.dark }

// IsDark reports whether the tokens follow the dark appearance.
func IsDark() bool { return active.dark }
