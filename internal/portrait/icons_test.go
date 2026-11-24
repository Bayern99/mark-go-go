package portrait

import (
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestNewIconsExistInBundledPhosphor(t *testing.T) {
	data, err := typefaces.ReadFile("fonts/Phosphor.ttf")
	if err != nil {
		t.Fatal(err)
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	icons := map[string]Icon{
		"puzzle-piece": IconPuzzlePiece, "wrench": IconWrench, "keyboard": IconKeyboard,
		"minus": IconMinus, "arrow-counter-clockwise": IconArrowCounterClockwise,
		"trash": IconTrash, "paper-plane-right": IconPaperPlaneRight, "stop": IconStop,
		"caret-up": IconCaretUp,
	}
	for name, icon := range icons {
		for _, r := range icon.Glyph {
			index, err := font.GlyphIndex(nil, r)
			if err != nil {
				t.Fatal(err)
			}
			if index == 0 {
				t.Errorf("%s U+%04X is missing from Phosphor.ttf", name, r)
			}
		}
	}
}
