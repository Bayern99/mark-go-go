package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// TintFrame is one keyframe of a highlight: a background color shown at
// Opacity, At a fraction of the way through.
type TintFrame struct {
	At      float32
	Color   ui.Color
	Opacity float32
}

// Highlight is a tint that plays once over a landing place, as a CSS
// animation whose timing function eases each segment between frames.
type Highlight struct {
	Duration time.Duration
	Ease     CubicBezier
	Radius   float32
	Frames   []TintFrame
}

// HighlightSet lists the Portrait highlights.
type HighlightSet struct {
	// Jump marks the heading an outline link landed on (mgo-jump-fade).
	Jump Highlight
}

// Highlights holds the Portrait highlights.
var Highlights = HighlightSet{
	Jump: Highlight{Duration: Motion.Jump, Ease: Motion.EaseOut, Radius: 6, Frames: []TintFrame{
		{At: 0, Color: ui.RGBA(47, 111, 98, 0.16), Opacity: 0.65},
		{At: 0.35, Color: ui.RGBA(47, 111, 98, 0.12), Opacity: 1},
		{At: 1, Color: ui.Transparent, Opacity: 1},
	}},
}

// At returns the tint elapsed into the highlight, its opacity folded into
// the color; transparent once it has played.
func (h Highlight) At(elapsed time.Duration) ui.Color {
	if len(h.Frames) == 0 || h.Duration <= 0 || elapsed >= h.Duration {
		if len(h.Frames) == 0 {
			return ui.Transparent
		}
		return h.Frames[len(h.Frames)-1].shown()
	}
	p := float32(max(elapsed, 0)) / float32(h.Duration)
	ease := h.Ease.Easing()
	for i := 1; i < len(h.Frames); i++ {
		a, b := h.Frames[i-1], h.Frames[i]
		if p <= b.At {
			return mix(a.shown(), b.shown(), ease((p-a.At)/max(b.At-a.At, 1e-6)))
		}
	}
	return h.Frames[len(h.Frames)-1].shown()
}

func (f TintFrame) shown() ui.Color {
	c := f.Color
	c.A = uint8(float32(c.A)*f.Opacity + 0.5)
	return c
}
