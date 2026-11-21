package portrait

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// CardMotion is how GenUI cards come in: up from Rise DIPs below, out of a
// Blur, fading in over Duration, each card Stagger after the one before.
type CardMotion struct {
	Duration, Stagger time.Duration
	// MaxStagger is the last card index whose delay still grows.
	MaxStagger int
	Rise, Blur float32
	Ease       CubicBezier
}

// Delay is when card i (0-based, in reading order) starts to come in.
func (m CardMotion) Delay(i int) time.Duration {
	return time.Duration(min(max(i, 0), m.MaxStagger)) * m.Stagger
}

// Shimmer is the skeleton's sweep: a Glint band crossing a Base fill once
// a Period.
type Shimmer struct {
	Period      time.Duration
	Base, Glint ui.Color
}

// The WebUI paints linear-gradient(90deg, fill 0%, glint 46%, fill 92%) on
// a tile 240% as wide as the box and moves it from background-position
// 100% to -140%.
const (
	shimmerTile = 2.4
	shimmerPeak = 0.46 * shimmerTile
)

// premultipliedMix mixes a and b t of the way as CSS does, with
// premultiplied alpha.
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

// Gradient is the skeleton's fill progress (0 to 1) through a period.
func (s Shimmer) Gradient(progress float32) ui.LinearGradient {
	pos := 1 - 2.4*progress
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

// Halo is the glow that spreads over the view when a generation is done:
// a radial ring around Center scaling From to To while it flares and fades.
type Halo struct {
	Duration time.Duration
	Ease     CubicBezier
	From, To float32
	Center   ui.Point
	// Inset is how far the halo reaches past the view on every side.
	Inset float32
	Fill  ui.RadialGradient
}

// Opacity is the halo's fade: in until 18% of the way, out by the end.
func (h Halo) Opacity() []ui.Keyframe {
	e := h.Ease.Easing()
	return []ui.Keyframe{{At: 0, Value: 0}, {At: h.Duration * 18 / 100, Value: 1, Ease: e}, {At: h.Duration, Value: 0, Ease: e}}
}

// Rim is the light that sweeps once around the view's edge with the halo:
// a conic band turning From to To degrees along a Width DIP ring.
type Rim struct {
	Duration time.Duration
	Ease     CubicBezier
	From, To float32
	Width    float32
	Stops    []ui.GradientStop
}

// Opacity is the rim's fade: up by 15%, dimming by 75%, gone at the end.
func (r Rim) Opacity() []ui.Keyframe {
	e := r.Ease.Easing()
	return []ui.Keyframe{{At: 0, Value: 0}, {At: r.Duration * 15 / 100, Value: 0.95, Ease: e}, {At: r.Duration * 75 / 100, Value: 0.72, Ease: e}, {At: r.Duration, Value: 0, Ease: e}}
}

// GenUITokens lists the GenUI reader's look and motion.
type GenUITokens struct {
	// Card is a block's surface; Replacing fills a block being regenerated.
	Card                    Surface
	CardRadius, CardPadding float32
	CardGap, SkeletonRadius float32
	Replacing               ui.Color
	Enter                   CardMotion
	Shimmer                 Shimmer
	Halo                    Halo
	Rim                     Rim
	// Source, SourceHover and SourcePress are the looks of a card's
	// source buttons (Edit in source, Note): paper with a hairline ring,
	// raised as a row while hovered, pressed in while held.
	Source, SourceHover, SourcePress Surface
}

func turn(deg float32) float32 { return deg / 360 }

// GenUI holds the GenUI reader's tokens.
var GenUI = GenUITokens{
	Card:           Surface{Colors.Paper, Colors.Paper, Shadows{out(0, 0, 0, 0.5, Colors.LineStrong)}},
	CardRadius:     10,
	CardPadding:    12,
	CardGap:        14,
	SkeletonRadius: 8,
	Replacing:      Colors.Code,
	Enter:          CardMotion{Duration: 220 * time.Millisecond, Stagger: 36 * time.Millisecond, MaxStagger: 6, Rise: 5, Blur: 2, Ease: Motion.EaseOut},
	// color-mix(in srgb, paper 82%, fill) mixes premultiplied: alpha
	// 0.82 + 0.18 × 0.045.
	Shimmer: Shimmer{Period: 1300 * time.Millisecond, Base: Colors.Fill, Glint: ui.RGBA(253, 253, 253, 0.828)},
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
