package ui

import "testing"

func wideTestPath() *Path {
	return new(Path).MoveTo(0, -100).LineTo(1000000, -100).LineTo(1000000, 150).LineTo(0, 150).Close()
}

func TestPathRasterScaleKeepsOrdinaryPathsAtFullResolution(t *testing.T) {
	path := new(Path).MoveTo(0, 0).LineTo(100, 0).LineTo(100, 50).LineTo(0, 50).Close()
	scale, q := pathRasterScale(path, 0, 2, 10)
	if scale != 20 || q != 10 {
		t.Fatalf("an ordinary path rasterizes at scale %v, q %v; want 20, 10", scale, q)
	}
}

func TestPathRasterScaleLimitsLargeFillAndStrokeSpans(t *testing.T) {
	path := wideTestPath()
	fillScale, fillQ := pathRasterScale(path, 0, 2, 10)
	if fillQ <= 1 || fillQ >= 10 {
		t.Fatalf("a large fill uses q %v, want a partial increase below 10", fillQ)
	}
	if span := float64(fillScale) * float64(path.Bounds().W); span > pathSpanMax {
		t.Fatalf("the large fill spans %v pixels, limit %v", span, pathSpanMax)
	}

	const width = 8
	strokeScale, strokeQ := pathRasterScale(path, width, 2, 10)
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
	scale, q := pathRasterScale(path, 1000, 2, 10)
	if scale != 2 || q != 1 {
		t.Fatalf("the original scale changed to %v, q %v; want 2, 1", scale, q)
	}
}
