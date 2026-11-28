package portrait

import "math"

// EditorTokens are reading and source typography metrics, independent from
// the fixed interface type scale.
type EditorTokens struct {
	Body, Code                           TypeStyle
	Column, GutterLeft, GutterRight      float32
	BlockRadius, BlockPadX, BlockPadY    float32
	TableCellPadX, TableCellPadY         float32
	TableRadius, TableColumnMin          float32
	TableColumnMax, TableColumnResizeMin float32
	TableRowMin                          float32
	TableEdgeFade                        float32
	ListGap, NestedListGap, ParagraphGap float32
	HeadingBefore, HeadingAfter          [6]float32
	CodeBlockBottom, ThematicBreakHeight float32
	QuoteBarWidth, Hairline              float32
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
	body := TypeStyle{Family: FontSans, Size: readingSize, Leading: float32(math.Round(float64(readingSize * 1.75)))}
	tablePadX, tablePadY := float32(8), float32(4)
	scale := func(points float32) float32 {
		return float32(math.Round(float64(points * readingSize / 16)))
	}
	return EditorTokens{
		Body:   body,
		Code:   TypeStyle{Family: FontMono, Size: codeSize, Leading: codeSize + 9},
		Column: width, GutterLeft: 56, GutterRight: 40,
		BlockRadius: Radii.M, BlockPadX: 14, BlockPadY: 10,
		TableCellPadX: tablePadX, TableCellPadY: tablePadY,
		TableRadius: Radii.M, TableColumnMin: 112, TableColumnMax: 340,
		TableColumnResizeMin: 72, TableRowMin: Type.Small.Leading + 2*tablePadY, TableEdgeFade: 24,
		ListGap: scale(6), NestedListGap: scale(5), ParagraphGap: readingSize,
		HeadingBefore:   [6]float32{scale(38), scale(30), scale(24), scale(22), scale(20), scale(20)},
		HeadingAfter:    [6]float32{scale(14), scale(10), scale(10), scale(6), scale(5), scale(4)},
		CodeBlockBottom: scale(12), ThematicBreakHeight: body.Leading,
		QuoteBarWidth: 3, Hairline: 1,
	}
}

func UseEditor(readingSize, codeSize, width float32) {
	Editor = EditorFor(readingSize, codeSize, width)
}
