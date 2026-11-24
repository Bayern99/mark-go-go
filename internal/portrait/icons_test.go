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
	icons := []struct {
		name string
		icon Icon
		want rune
	}{
		{"puzzle-piece", IconPuzzlePiece, '\ue596'},
		{"wrench", IconWrench, '\ue5d4'},
		{"keyboard", IconKeyboard, '\ue2d8'},
		{"minus", IconMinus, '\ue32a'},
		{"arrow-counter-clockwise", IconArrowCounterClockwise, '\ue038'},
		{"trash", IconTrash, '\ue4a6'},
		{"paper-plane-right", IconPaperPlaneRight, '\ue396'},
		{"stop", IconStop, '\ue46c'},
		{"caret-up", IconCaretUp, '\ue13c'},
	}
	for _, tc := range icons {
		runes := []rune(tc.icon.Glyph)
		if len(runes) != 1 || runes[0] != tc.want {
			t.Errorf("%s = %q, want U+%04X", tc.name, tc.icon.Glyph, tc.want)
			continue
		}
		index, err := font.GlyphIndex(nil, runes[0])
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			t.Errorf("%s U+%04X is missing from Phosphor.ttf", tc.name, runes[0])
		}
	}
}
