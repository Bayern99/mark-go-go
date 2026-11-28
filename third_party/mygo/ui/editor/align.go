package editor

import "math"

type TextAlign uint8

const (
	AlignLeft TextAlign = iota
	AlignCenter
	AlignRight
)

func (v *view) setTextAlign(align TextAlign, width float32) {
	width = max(0, float32(math.Floor(float64(width))))
	if align == v.align && width == v.alignWidth {
		return
	}
	changed := align != v.align
	v.align, v.alignWidth = align, width
	if changed || v.align != AlignLeft {
		v.lines.m = nil
	}
}

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

func (r *rowLayout) startX() float32 { return r.indent + r.align }
