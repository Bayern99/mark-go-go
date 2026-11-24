package portrait

import (
	"hash/fnv"

	"github.com/egoist/mygo/ui"
)

type Scheme struct {
	From, To, Paper, Code ui.Color
	Accent, Mark          ui.Color
	Success, Warn, Danger ui.Color
}

// Space is the scenery and accent of one workspace, in the light and the
// dark appearance.
type Space struct {
	Name, Label string
	Light, Dark Scheme
}

const (
	sceneryAngle       = 160
	sceneryTranslucent = 0.78
)

var (
	lightSuccess, lightWarn, lightDanger = ui.Hex("#1A6840"), ui.Hex("#815F25"), ui.Hex("#AB372F")
	darkSuccess, darkWarn, darkDanger    = ui.Hex("#A4C2A8"), ui.Hex("#D8C37C"), ui.Hex("#F6AD8F")
)

func light(from, to, paper, code, accent, mark string) Scheme {
	return Scheme{ui.Hex(from), ui.Hex(to), ui.Hex(paper), ui.Hex(code), ui.Hex(accent), ui.Hex(mark), lightSuccess, lightWarn, lightDanger}
}

func dark(from, to, paper, code, accent, mark string) Scheme {
	return Scheme{ui.Hex(from), ui.Hex(to), ui.Hex(paper), ui.Hex(code), ui.Hex(accent), ui.Hex(mark), darkSuccess, darkWarn, darkDanger}
}

func (s Scheme) elements(success, warn, danger ui.Color) Scheme {
	s.Success, s.Warn, s.Danger = success, warn, danger
	return s
}

// Spaces is the closed set of Wu Xing Spaces (wuxing@adr-0014).
var Spaces = [...]Space{
	{"FiveElements", "Five Elements",
		light("#F2F0E6", "#D0DFE6", "#FFFFFF", "#FBF7EE", "#2E5D8C", "#AB372F"),
		dark("#0E100F", "#101F30", "#1A1A1A", "#25211C", "#B0D8E8", "#F6AD8F")},
	{"MetalWater", "Metal–Water",
		light("#F2F3F4", "#CCCCD6", "#FFFFFF", "#F3F4F6", "#2E5D8C", "#2E5D8C").elements(lightSuccess, ui.Hex("#685E48"), lightDanger),
		dark("#0F1A20", "#22202E", "#16181C", "#1E2228", "#A8C9E4", "#A8C9E4").elements(ui.Hex("#A4C2A8"), ui.Hex("#B5AA90"), darkDanger)},
	{"WoodFire", "Wood–Fire",
		light("#EEF7F2", "#F6DCCE", "#FFFFFF", "#FBF6EE", "#AB372F", "#AB372F").elements(lightSuccess, ui.Hex("#964D22"), lightDanger),
		dark("#141E1B", "#481E1C", "#1B1817", "#25201E", "#F6AD8F", "#F6AD8F").elements(ui.Hex("#A0D6B4"), ui.Hex("#E2C17C"), darkDanger)},
	{"Earth", "Earth",
		light("#F9E9CD", "#F6DEAD", "#FFFFFF", "#FAF6ED", "#815F25", "#815F25"),
		dark("#1D1B1C", "#503E2A", "#1A1816", "#24201C", "#E2C17C", "#E2C17C")},
	{"Metal", "Metal",
		light("#F2F3F4", "#E2E1E4", "#FFFFFF", "#F5F5F6", "#4B6C8C", "#4B6C8C"),
		dark("#0E100F", "#2B333E", "#16181A", "#1F2226", "#D0E7F9", "#D0E7F9")},
	{"Wood", "Wood",
		light("#EEF7F2", "#C6E3E1", "#FFFFFF", "#F3F7F6", "#126E82", "#126E82"),
		dark("#141E1B", "#132C33", "#151B1A", "#1D2625", "#A0D6B4", "#A0D6B4")},
	{"Water", "Water",
		light("#EDE3E7", "#CCCCD6", "#FFFFFF", "#F5F4F7", "#2E317C", "#2E317C"),
		dark("#0E100F", "#131124", "#16151C", "#1F1E28", "#B8A5D2", "#B8A5D2")},
	{"Fire", "Fire",
		light("#F8EBE6", "#F2CAC9", "#FFFFFF", "#FAF4F3", "#9E2F2F", "#9E2F2F"),
		dark("#1D1B1C", "#481E1C", "#1B1717", "#251F1F", "#F0ADA0", "#F0ADA0")},
}

// DefaultSpace is the Space of a window without a workspace folder.
var DefaultSpace = Spaces[0]

// SpaceNamed finds a Space by the name stored in preferences.
func SpaceNamed(name string) (Space, bool) {
	for _, s := range Spaces {
		if s.Name == name {
			return s, true
		}
	}
	return Space{}, false
}

// SpaceFor gives a workspace root a stable Space, so each folder keeps its
// identity until the reader picks another.
func SpaceFor(root string) Space {
	if root == "" {
		return DefaultSpace
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	return Spaces[int(h.Sum32()%uint32(len(Spaces)))]
}

func (s Space) In(dark bool) Scheme {
	if dark {
		return s.Dark
	}
	return s.Light
}

func (s Space) Scheme() Scheme { return s.In(active.dark) }

// Paint fills e with the scenery. Over a window material the scenery is
// translucent so the desktop shows through it.
func (s Space) Paint(e ui.Element, translucent bool) ui.Element {
	sc := s.Scheme()
	from, to := sc.From, sc.To
	if translucent {
		from, to = from.Alpha(sceneryTranslucent), to.Alpha(sceneryTranslucent)
	}
	return e.Gradient(from, to, sceneryAngle)
}

// Swatch fills a small mark with the Space's colour at full strength.
func (s Space) Swatch(e ui.Element) ui.Element {
	sc := s.Scheme()
	return e.Gradient(sc.Mark.Mix(sc.Paper, 0.28), sc.Mark, sceneryAngle)
}
