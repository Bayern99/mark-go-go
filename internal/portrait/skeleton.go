package portrait

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Skeleton describes a loading placeholder's base fill and moving glint.
// Apply can paint any element; View provides an accessible progress placeholder.
type Skeleton struct {
	Period      time.Duration
	Base, Glint ui.Color
}

// Shimmer is the original GenUI name for Skeleton.
type Shimmer = Skeleton

// DefaultSkeleton is the shared loading-placeholder appearance.
var DefaultSkeleton Skeleton

func skeletonFor(p Palette, dark bool) Skeleton {
	glint := ui.RGBA(253, 253, 253, 0.828)
	if dark {
		glint = white(0.07)
	}
	return Skeleton{Period: 1300 * time.Millisecond, Base: p.Fill, Glint: glint}
}

const (
	shimmerTile          = 2.4
	shimmerPeak          = 0.46 * shimmerTile
	shimmerStartPosition = 1.0
	shimmerEndPosition   = -1.4
)

type skeletonLoopKey struct{}

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

// Gradient returns the glint at progress through one skeleton period.
func (s Skeleton) Gradient(progress float32) ui.LinearGradient {
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

// Apply paints a skeleton onto e. Reduced-motion preferences hold the base
// fill still and schedule no animation frames.
func (s Skeleton) Apply(c *ui.Context, e ui.Element) ui.Element {
	if !Loops(c) {
		return e.Background(s.Base)
	}
	return e.LinearGradient(s.Gradient(e.Loop(skeletonLoopKey{}, s.Period, ui.Linear)))
}

// View builds an accessible loading placeholder with the standard radius.
func (s Skeleton) View(c *ui.Context, label string) ui.Element {
	e := ui.Box(c.Key("skeleton:" + label)).Height(Sizes.Regular).Radius(Radii.M).
		Role(ui.RoleProgress).Label(label)
	return s.Apply(c, e)
}
