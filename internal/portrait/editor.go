package portrait

import "math"

// EditorTokens are reading and source typography metrics, independent from
// the fixed interface type scale.
type EditorTokens struct {
	Body, Code                        TypeStyle
	Column, GutterLeft, GutterRight   float32
	BlockRadius, BlockPadX, BlockPadY float32
	Hairline                          float32
}

var Editor = EditorFor(16, 13, 624)

func EditorFor(readingSize, codeSize, width float32) EditorTokens {
	if readingSize < 15 || readingSize > 18 {
		readingSize = 16
	}
	if codeSize < 12 || codeSize > 14 {
		codeSize = 13
	}
	switch width {
	case 560, 720:
	default:
		width = 624
	}
	return EditorTokens{
		Body:   TypeStyle{Family: FontSans, Size: readingSize, Leading: float32(math.Round(float64(readingSize * 1.75)))},
		Code:   TypeStyle{Family: FontMono, Size: codeSize, Leading: codeSize + 9},
		Column: width, GutterLeft: 56, GutterRight: 40,
		BlockRadius: Radii.M, BlockPadX: 14, BlockPadY: 10,
		Hairline: 1,
	}
}

func UseEditor(readingSize, codeSize, width float32) {
	Editor = EditorFor(readingSize, codeSize, width)
}
