package portrait

import (
	"hash/fnv"

	"github.com/egoist/mygo/ui"
)

// Space is the scenery of one workspace: the tinted field the sidebar sits
// on and the page floats over. Text on it uses Ink or Ink2.
type Space struct {
	Name     string
	From, To ui.Color
	Mark     ui.Color
}

const (
	sceneryAngle       = 160
	sceneryTranslucent = 0.78
)

// Spaces is the closed set of sceneries. Mist is the neutral one.
var Spaces = [...]Space{
	{"Mist", ui.Hex("#eeeef2"), ui.Hex("#e2e2e8"), ui.Hex("#8e8e98")},
	{"Sky", ui.Hex("#cfdcff"), ui.Hex("#ddd3ff"), ui.Hex("#5b7cfa")},
	{"Dawn", ui.Hex("#ffd9c7"), ui.Hex("#ffc9d6"), ui.Hex("#ff7e5f")},
	{"Twilight", ui.Hex("#ddd4ff"), ui.Hex("#f6cfe6"), ui.Hex("#9a6cf0")},
	{"Aurora", ui.Hex("#c8efdc"), ui.Hex("#c5e2fa"), ui.Hex("#22b08a")},
	{"Sand", ui.Hex("#fff6d6"), ui.Hex("#f3e6c4"), ui.Hex("#d9a441")},
}

// DefaultSpace is the scenery of a window without a workspace folder.
var DefaultSpace = Spaces[1]

// SpaceNamed finds a scenery by the name stored in preferences.
func SpaceNamed(name string) (Space, bool) {
	for _, s := range Spaces {
		if s.Name == name {
			return s, true
		}
	}
	return Space{}, false
}

// SpaceFor gives a workspace root a stable coloured scenery, so each folder
// keeps its identity until the reader picks another.
func SpaceFor(root string) Space {
	if root == "" {
		return DefaultSpace
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	return Spaces[1+int(h.Sum32()%uint32(len(Spaces)-1))]
}

// Paint fills e with the scenery. Over a window material the scenery is
// translucent so the desktop shows through it.
func (s Space) Paint(e ui.Element, translucent bool) ui.Element {
	from, to := s.From, s.To
	if translucent {
		from, to = from.Alpha(sceneryTranslucent), to.Alpha(sceneryTranslucent)
	}
	return e.Gradient(from, to, sceneryAngle)
}

// Swatch fills a small mark with the scenery's colour at full strength.
func (s Space) Swatch(e ui.Element) ui.Element {
	return e.Gradient(s.Mark.Mix(Colors.Paper, 0.28), s.Mark, sceneryAngle)
}
