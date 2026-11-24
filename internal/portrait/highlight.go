package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type TintFrame struct {
	At      float32
	Color   ui.Color
	Opacity float32
}

type Highlight struct {
	Duration time.Duration
	Ease     CubicBezier
	Radius   float32
	Frames   []TintFrame
}

type HighlightSet struct {
	Jump Highlight
}

var Highlights = HighlightSet{
	Jump: Highlight{Duration: Motion.Jump, Ease: Motion.EaseOut, Radius: 6, Frames: []TintFrame{
		{At: 0, Color: Colors.Accent.Alpha(0.16), Opacity: 0.65},
		{At: 0.35, Color: Colors.Accent.Alpha(0.12), Opacity: 1},
		{At: 1, Color: ui.Transparent, Opacity: 1},
	}},
}

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
