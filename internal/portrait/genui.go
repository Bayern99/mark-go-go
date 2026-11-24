package portrait

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type CardMotion struct {
	Duration, Stagger time.Duration
	MaxStagger        int
	Rise, Blur        float32
	Ease              CubicBezier
}

func (m CardMotion) Delay(i int) time.Duration {
	return time.Duration(min(max(i, 0), m.MaxStagger)) * m.Stagger
}

type Halo struct {
	Duration time.Duration
	Ease     CubicBezier
	From, To float32
	Center   ui.Point
	Inset    float32
	Fill     ui.RadialGradient
}

func (h Halo) Opacity() []ui.Keyframe {
	e := h.Ease.Easing()
	return []ui.Keyframe{{At: 0, Value: 0}, {At: h.Duration * 18 / 100, Value: 1, Ease: e}, {At: h.Duration, Value: 0, Ease: e}}
}

type Rim struct {
	Duration time.Duration
	Ease     CubicBezier
	From, To float32
	Width    float32
	Stops    []ui.GradientStop
}

func (r Rim) Opacity() []ui.Keyframe {
	e := r.Ease.Easing()
	return []ui.Keyframe{{At: 0, Value: 0}, {At: r.Duration * 15 / 100, Value: 0.95, Ease: e}, {At: r.Duration * 75 / 100, Value: 0.72, Ease: e}, {At: r.Duration, Value: 0, Ease: e}}
}

type GenUITokens struct {
	Card                             Surface
	CardRadius, CardPadding          float32
	CardGap, SkeletonRadius          float32
	Replacing                        ui.Color
	Enter                            CardMotion
	Shimmer                          Shimmer
	Halo                             Halo
	Rim                              Rim
	Source, SourceHover, SourcePress Surface
}

func turn(deg float32) float32 { return deg / 360 }

var GenUI = GenUITokens{
	Card:           Materials.Sheet,
	CardRadius:     Radii.L,
	CardPadding:    12,
	CardGap:        14,
	SkeletonRadius: 8,
	Replacing:      Colors.Code,
	Enter:          CardMotion{Duration: 220 * time.Millisecond, Stagger: 36 * time.Millisecond, MaxStagger: 6, Rise: 5, Blur: 2, Ease: Motion.EaseOut},
	Shimmer:        DefaultSkeleton,
	Halo: Halo{Duration: 950 * time.Millisecond, Ease: CubicBezier{0.16, 1, 0.3, 1}, From: 0.32, To: 2.15, Center: ui.Point{X: 0.86, Y: 0.24}, Inset: 24,
		Fill: ui.RadialGradient{Center: ui.Point{X: 0.86, Y: 0.24}, Stops: []ui.GradientStop{
			{Color: ui.Transparent, At: 0}, {Color: ui.Transparent, At: 0.24},
			{Color: Colors.Accent.Alpha(0.11), At: 0.38},
			{Color: Colors.Accent.Mix(Colors.Paper, 0.45).Alpha(0.19), At: 0.50},
			{Color: ui.Transparent, At: 0.68}}}},
	Rim: Rim{Duration: 900 * time.Millisecond, Ease: CubicBezier{0.22, 1, 0.36, 1}, From: -60, To: 300, Width: 1.5,
		Stops: []ui.GradientStop{
			{Color: ui.Transparent, At: 0}, {Color: ui.Transparent, At: turn(250)},
			{Color: Colors.Accent.Mix(Colors.Paper, 0.35), At: turn(312)},
			{Color: white(0.96), At: turn(338)},
			{Color: ui.Transparent, At: 1}}},
	Source:      Surface{Colors.Paper, Colors.Paper, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
	SourceHover: Surface{Colors.Code, Colors.Code, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
	SourcePress: Surface{Materials.RaisedPress.From, Materials.RaisedPress.To, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
}
