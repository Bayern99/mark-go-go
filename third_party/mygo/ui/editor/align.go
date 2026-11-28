package editor

import "math"

// TextAlign is how an editor places each row of its lines in the width of
// its text: at the start of it, as editors have always drawn them, or
// centered in it, or at its end, as the cells of a Markdown table's
// columns are aligned.
//
// Every visual row is placed on its own: the rows of a line a soft wrap
// breaks place by their widths, not the line's. What is drawn, the
// carets, the selections, the pointer's hits and the anchor of an input
// method's candidates all share the place a row is given, and a row
// wider than the width of the text stays at its start, rather than being
// pushed past it.
type TextAlign uint8

const (
	// AlignLeft starts each row at the left of the text, the default.
	AlignLeft TextAlign = iota
	// AlignCenter centers each row in the width of the text.
	AlignCenter
	// AlignRight ends each row at the right of the width of the text.
	AlignRight
)

// setTextAlign sets how the rows of the lines are placed in width, the
// width of the text they have, and lays out again the lines whose rows
// move. The offsets are whole DIPs, as the text is drawn on them.
func (v *view) setTextAlign(align TextAlign, width float32) {
	width = max(0, float32(math.Floor(float64(width))))
	if align == v.align && width == v.alignWidth {
		return
	}
	changed := align != v.align
	v.align, v.alignWidth = align, width
	// Only the aligned lines move with the width: at the left, the rows
	// of the lines laid out are where they would be.
	if changed || v.align != AlignLeft {
		v.lines.m = nil
	}
}

// alignOffset returns how far a row used DIPs wide is drawn past the start
// of the text: centered in the width of the text, at its end, or not at
// all, and never past its left edge.
func (v *view) alignOffset(used float32) float32 {
	slack := v.alignWidth - used
	if slack <= 0 {
		return 0
	}
	switch v.align {
	case AlignCenter:
		return min(max(0, float32(math.Round(float64(slack)/2))), slack)
	case AlignRight:
		return float32(math.Floor(float64(slack)))
	}
	return 0
}

// startX returns where the row's text starts, past the start of the
// editor's text: its insets, then the offset its editor's TextAlign gives
// it.
func (r *rowLayout) startX() float32 { return r.indent + r.align }
