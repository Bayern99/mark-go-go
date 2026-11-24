package portrait

import "github.com/egoist/mygo/ui"

type Icon struct {
	Glyph  string
	Mirror bool
}

func (i Icon) Mirrored() Icon { i.Mirror = !i.Mirror; return i }

var (
	IconX                 = Icon{Glyph: "\ue4f6"}
	IconSidebar           = Icon{Glyph: "\uec24"}
	IconPlugs             = Icon{Glyph: "\ueb56"}
	IconCaretDown         = Icon{Glyph: "\ue136"}
	IconCaretRight        = Icon{Glyph: "\ue13a"}
	IconCube              = Icon{Glyph: "\ue1da"}
	IconSlidersHorizontal = Icon{Glyph: "\ue434"}
	IconFileText          = Icon{Glyph: "\ue23a"}
	IconFolder            = Icon{Glyph: "\ue24a"}
	IconFolderOpen        = Icon{Glyph: "\ue256"}
	IconMagnifyingGlass   = Icon{Glyph: "\ue30c"}
	IconDotsThree         = Icon{Glyph: "\ue1fe"}
	IconAtom              = Icon{Glyph: "\ue5e4"}
	IconSpinnerGap        = Icon{Glyph: "\ue66c"}
	IconPaperclip         = Icon{Glyph: "\ue39a"}
	IconCheck             = Icon{Glyph: "\ue182"}
	IconCheckCircle       = Icon{Glyph: "\ue184"}
	IconCircle            = Icon{Glyph: "\ue18a"}
	IconWarningCircle     = Icon{Glyph: "\ue4e2"}
)

func Glyph(c *ui.Context, icon Icon, size float32, color ui.Color) ui.Element {
	return icon.Apply(ui.Text(c, icon.Glyph), size).TextColor(color).AccessHidden(true)
}

func (i Icon) Apply(e ui.Element, size float32) ui.Element {
	e.Font(FontIcons).FontSize(size).FixedLineHeight(size)
	if i.Mirror {
		e.TransformOrigin(0.5, 0.5).Scale(-1, 1)
	}
	return e
}

var (
	IconArrowUpRight   = Icon{Glyph: "\ue092"}
	IconArrowLeft      = Icon{Glyph: "\ue058"}
	IconNote           = Icon{Glyph: "\ue348"}
	IconLightbulb      = Icon{Glyph: "\ue2dc"}
	IconInfo           = Icon{Glyph: "\ue2ce"}
	IconWarning        = Icon{Glyph: "\ue4e0"}
	IconWarningOctagon = Icon{Glyph: "\ue4e4"}
	IconCopy           = Icon{Glyph: "\ue1ca"}
)

var (
	IconPlus                  = Icon{Glyph: "\ue3d4"}
	IconCommand               = Icon{Glyph: "\ue1c4"}
	IconHash                  = Icon{Glyph: "\ue2a2"}
	IconClockCounterClockwise = Icon{Glyph: "\ue1a0"}
	IconArrowElbowDownLeft    = Icon{Glyph: "\ue044"}
	IconPalette               = Icon{Glyph: "\ue6c8"}
	IconEye                   = Icon{Glyph: "\ue220"}
	IconCode                  = Icon{Glyph: "\ue1bc"}
	IconSparkle               = Icon{Glyph: "\ue6a2"}
	IconArrowRight            = Icon{Glyph: "\ue06c"}
	IconFolderPlus            = Icon{Glyph: "\ue258"}
	IconFunnel                = Icon{Glyph: "\ue266"}
)
