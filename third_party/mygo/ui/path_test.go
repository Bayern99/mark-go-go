package ui

import (
	"image/color"
	"math"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

func wideTestPath() *Path {
	return new(Path).MoveTo(0, -100).LineTo(1000000, -100).LineTo(1000000, 150).LineTo(0, 150).Close()
}

func TestPathRasterScaleKeepsOrdinaryPathsAtFullResolution(t *testing.T) {
	path := new(Path).MoveTo(0, 0).LineTo(100, 0).LineTo(100, 50).LineTo(0, 50).Close()
	scale, q := pathRasterScale(path, 0, 2, 10, pathSpanMax)
	if scale != 20 || q != 10 {
		t.Fatalf("an ordinary path rasterizes at scale %v, q %v; want 20, 10", scale, q)
	}
}

func TestPathRasterScaleLimitsLargeFillAndStrokeSpans(t *testing.T) {
	path := wideTestPath()
	fillScale, fillQ := pathRasterScale(path, 0, 2, 10, pathSpanMax)
	if fillQ <= 1 || fillQ >= 10 {
		t.Fatalf("a large fill uses q %v, want a partial increase below 10", fillQ)
	}
	if span := float64(fillScale) * float64(path.Bounds().W); span > pathSpanMax {
		t.Fatalf("the large fill spans %v pixels, limit %v", span, pathSpanMax)
	}

	const width = 8
	strokeScale, strokeQ := pathRasterScale(path, width, 2, 10, pathSpanMax)
	if strokeQ <= 1 || strokeQ >= fillQ {
		t.Fatalf("a large stroke uses q %v, want between 1 and the fill's %v", strokeQ, fillQ)
	}
	spanWithStroke := float64(path.Bounds().W + width)
	if span := float64(strokeScale) * spanWithStroke; span > pathSpanMax {
		t.Fatalf("the large stroke spans %v pixels including its width, limit %v", span, pathSpanMax)
	}
}

func TestPathRasterScaleFallsBackToTheOriginalScale(t *testing.T) {
	// A straight line has no miter joins in practice. Its conservative miter
	// allowance must not reduce the scale at which it already drew.
	path := new(Path).MoveTo(0, 25).LineTo(1000000, 25).Join(MiterJoin).MiterLimit(10000)
	scale, q := pathRasterScale(path, 1000, 2, 10, pathSpanMax)
	if scale != 2 || q != 1 {
		t.Fatalf("the original scale changed to %v, q %v; want 2, 1", scale, q)
	}
}

func TestClipPathDoesNotTruncateWhenItsRasterScaleIsLimited(t *testing.T) {
	ink := Color{R: 0x02, G: 0x85, B: 0xff, A: 0xff}
	want := color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A}
	tt := NewTester(func(c *Context) {
		Box(c).Size(120, 120).Draw(func(p *Painter, r Rect) {
			path := new(Path).MoveTo(r.X, r.Y).LineTo(r.X+100, r.Y).LineTo(r.X+100, r.Y+1).LineTo(r.X, r.Y+1).Close()
			p.Transform(Scaling(1, 100), r.X, r.Y, func() {
				p.ClipPath(path, func() {
					p.Fill(Rect{X: r.X, Y: r.Y, W: 100, H: 1}, ink, 0)
				})
			})
		})
	}, 120, 120)
	tt.SetScale(2)
	img := tt.Image()
	// At 2x DPR the transformed clip covers 200 physical pixels. Its q=100
	// flattening would span 20,000 pixels and used to be truncated to 8,192,
	// leaving only about 41 DIPs (82 physical pixels) clipped in.
	for _, x := range []int{10, 190} {
		if got := img.RGBAAt(x, 100); got != want {
			t.Errorf("the transformed clip at physical x %d is %v, want %v", x, got, want)
		}
	}
}

func largeCoordinateTestPath() *Path {
	return new(Path).MoveTo(1073741824, 0).LineTo(1074266112, 0).LineTo(1074266112, 10).LineTo(1073741824, 10).Close()
}

func TestFlattenPathFallsBackWhenFloat32RoundingExceedsPathSpan(t *testing.T) {
	transform := scene.Translation(-1073741824, 0).Mul(scene.Scaling(4, 4))
	p := Painter{rt: new(engine), scale: 2, transform: transform}
	f, scale, saved := p.flattenPath(largeCoordinateTestPath(), 0, pathSpanMax, false)
	if scale != p.scale {
		t.Fatalf("the rounded oversized raster used scale %v, want the original %v", scale, p.scale)
	}
	if p.transform != transform || saved != transform {
		t.Fatalf("fallback changed the transform to %v, saved %v, want %v", p.transform, saved, transform)
	}
	if !drawPathRasterFits(f, 0, pathSpanMax) {
		t.Fatal("the path no longer fits at the scale at which it originally drew")
	}
}

func TestFlattenPathKeepsLargeAbsoluteCoordinatesFinite(t *testing.T) {
	const huge = float32(math.MaxFloat32 / 4)
	path := new(Path).MoveTo(huge, 0).LineTo(huge, 10)
	p := Painter{rt: new(engine), scale: 2, transform: scene.Scaling(4, 4)}
	f, scale, _ := p.flattenPath(path, 1, pathSpanMax, false)
	if scale != p.scale {
		t.Fatalf("the narrow path used scale %v, want the original %v", scale, p.scale)
	}
	for _, pt := range f.pts {
		if !finitePathNumber(pt[0]) || !finitePathNumber(pt[1]) {
			t.Fatalf("the fallback flattened a non-finite point %v", pt)
		}
	}
	if !drawPathRasterFits(f, scale, pathSpanMax) {
		t.Fatal("the narrow path no longer fits at its original scale")
	}
}

func TestLargeCoordinatesTranslatedIntoViewportStillDraw(t *testing.T) {
	ink := Color{R: 0x02, G: 0x85, B: 0xff, A: 0xff}
	want := color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A}
	tt := NewTester(func(c *Context) {
		Box(c).Size(120, 50).Draw(func(p *Painter, r Rect) {
			p.Transform(Translation(-1073741824, 0).Then(Scaling(4, 4)), r.X, r.Y, func() {
				p.FillPath(largeCoordinateTestPath(), ink)
			})
		})
	}, 120, 50)
	tt.SetScale(2)
	// The conservative q clamp gets just below 4, but float32 rounds the two
	// scaled X endpoints to a 4,194,304-pixel span; the endpoint pixel would
	// make drawPathPaint reject the whole path. Fallback must keep it visible.
	if got := tt.Image().RGBAAt(120, 20); got != want {
		t.Fatalf("the translated large path's viewport center is %v, want %v", got, want)
	}
}
