package portrait

import "github.com/egoist/mygo/ui"

// Icon is a Phosphor glyph, drawn mirrored left to right when Mirror is set
// (the WebUI's transform: scaleX(-1)).
type Icon struct {
	Glyph  string
	Mirror bool
}

// Mirrored returns the icon flipped left to right.
func (i Icon) Mirrored() Icon { i.Mirror = !i.Mirror; return i }

// Phosphor glyphs used by the WebUI baseline (@phosphor-icons/web, regular).
var (
	IconX               = Icon{Glyph: "\ue4f6"}
	IconSidebar         = Icon{Glyph: "\uec24"}
	IconPlugs           = Icon{Glyph: "\ueb56"}
	IconCaretDown       = Icon{Glyph: "\ue136"}
	IconCaretRight      = Icon{Glyph: "\ue13a"}
	IconFileText        = Icon{Glyph: "\ue23a"}
	IconFolder          = Icon{Glyph: "\ue24a"}
	IconFolderOpen      = Icon{Glyph: "\ue256"}
	IconMagnifyingGlass = Icon{Glyph: "\ue30c"}
	IconDotsThree       = Icon{Glyph: "\ue1fe"}
	IconAtom            = Icon{Glyph: "\ue5e4"}
	IconSpinnerGap      = Icon{Glyph: "\ue66c"}
	IconPaperclip       = Icon{Glyph: "\ue39a"}
	IconCheck           = Icon{Glyph: "\ue182"}
	IconCheckCircle     = Icon{Glyph: "\ue184"}
	IconCircle          = Icon{Glyph: "\ue18a"}
	IconWarningCircle   = Icon{Glyph: "\ue4e2"}
)

// Glyph draws the icon at size in color. It is decoration: the control
// holding it carries the accessible name.
func Glyph(c *ui.Context, icon Icon, size float32, color ui.Color) ui.Element {
	return icon.Apply(ui.Text(c, icon.Glyph), size).TextColor(color).AccessHidden(true)
}

// Apply sets e, an element showing the icon's glyph, in the icon font at
// size, mirrored when the icon is.
func (i Icon) Apply(e ui.Element, size float32) ui.Element {
	e.Font(FontIcons).FontSize(size).FixedLineHeight(size)
	if i.Mirror {
		e.TransformOrigin(0.5, 0.5).Scale(-1, 1)
	}
	return e
}

// Phosphor glyphs of the editor's blocks and links.
var (
	IconArrowUpRight   = Icon{Glyph: "\ue092"}
	IconNote           = Icon{Glyph: "\ue348"}
	IconLightbulb      = Icon{Glyph: "\ue2dc"}
	IconInfo           = Icon{Glyph: "\ue2ce"}
	IconWarning        = Icon{Glyph: "\ue4e0"}
	IconWarningOctagon = Icon{Glyph: "\ue4e4"}
	IconCopy           = Icon{Glyph: "\ue1ca"}
)
