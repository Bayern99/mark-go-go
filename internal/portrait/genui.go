package portrait

import (
	"math"
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

type Shimmer struct {
	Period      time.Duration
	Base, Glint ui.Color
}

const (
	shimmerTile          = 2.4
	shimmerPeak          = 0.46 * shimmerTile
	shimmerStartPosition = 1.0
	shimmerEndPosition   = -1.4
)

func premultipliedMix(a, b ui.Color, t float32) ui.Color {
	aa, ba := float32(a.A)/255, float32(b.A)/255
	alpha := aa*(1-t) + ba*t
	if alpha == 0 {
		return ui.Transparent
	}
	ch := func(x, y uint8) uint8 {
		return uint8((float32(x)*aa*(1-t)+float32(y)*ba*t)/alpha + 0.5)
	}
	return ui.Color{R: ch(a.R, b.R), G: ch(a.G, b.G), B: ch(a.B, b.B), A: uint8(alpha*255 + 0.5)}
}

func (s Shimmer) Gradient(progress float32) ui.LinearGradient {
	pos := shimmerStartPosition + (shimmerEndPosition-shimmerStartPosition)*progress
	left := -(shimmerTile - 1) * pos
	var centers []float32
	for k := -1; k <= 1; k++ {
		c := left + shimmerPeak + float32(k)*shimmerTile
		if c+shimmerPeak > 0 && c-shimmerPeak < 1 {
			centers = append(centers, c)
		}
	}
	at := func(x float32) ui.Color {
		for _, c := range centers {
			if d := float32(math.Abs(float64(x - c))); d < shimmerPeak {
				return premultipliedMix(s.Glint, s.Base, d/shimmerPeak)
			}
		}
		return s.Base
	}
	stops := []ui.GradientStop{{Color: at(0), At: 0}}
	for _, c := range centers {
		for _, x := range []float32{c - shimmerPeak, c, c + shimmerPeak} {
			if x > 0 && x < 1 {
				stops = append(stops, ui.GradientStop{Color: at(x), At: x})
			}
		}
	}
	stops = append(stops, ui.GradientStop{Color: at(1), At: 1})
	return ui.LinearGradient{Angle: 90, Stops: stops}
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
	Card:           Surface{Colors.Paper, Colors.Paper, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
	CardRadius:     10,
	CardPadding:    12,
	CardGap:        14,
	SkeletonRadius: 8,
	Replacing:      Colors.Code,
	Enter:          CardMotion{Duration: 220 * time.Millisecond, Stagger: 36 * time.Millisecond, MaxStagger: 6, Rise: 5, Blur: 2, Ease: Motion.EaseOut},
	Shimmer:        Shimmer{Period: 1300 * time.Millisecond, Base: Colors.Fill, Glint: ui.RGBA(253, 253, 253, 0.828)},
	Halo: Halo{Duration: 950 * time.Millisecond, Ease: CubicBezier{0.16, 1, 0.3, 1}, From: 0.32, To: 2.15, Center: ui.Point{X: 0.86, Y: 0.24}, Inset: 24,
		Fill: ui.RadialGradient{Center: ui.Point{X: 0.86, Y: 0.24}, Stops: []ui.GradientStop{
			{Color: ui.Transparent, At: 0}, {Color: ui.Transparent, At: 0.24},
			{Color: ui.RGBA(47, 111, 98, 0.11), At: 0.38},
			{Color: ui.RGBA(130, 182, 169, 0.19), At: 0.50},
			{Color: ui.Transparent, At: 0.68}}}},
	Rim: Rim{Duration: 900 * time.Millisecond, Ease: CubicBezier{0.22, 1, 0.36, 1}, From: -60, To: 300, Width: 1.5,
		Stops: []ui.GradientStop{
			{Color: ui.Transparent, At: 0}, {Color: ui.Transparent, At: turn(250)},
			{Color: ui.RGBA(109, 154, 145, 1), At: turn(312)},
			{Color: white(0.96), At: turn(338)},
			{Color: ui.Transparent, At: 1}}},
	Source:      Surface{Colors.Paper, Colors.Paper, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
	SourceHover: Surface{raised[0], raised[1], Materials.Row.Shadows},
	SourcePress: Surface{Materials.RaisedPress.From, Materials.RaisedPress.To, Materials.Row.Shadows},
}
