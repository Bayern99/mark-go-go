package portrait

type RadiusScale struct {
	XS, S, M, L, XL, Pill float32
	Track, Segment        float32
}

var Radii = RadiusScale{
	XS: 4, S: 6, M: 8, L: 12, XL: 16, Pill: 999,
	Track: 10, Segment: 8,
}

type SizeScale struct {
	Compact, Regular, Large float32
}

var Sizes = SizeScale{Compact: 22, Regular: 28, Large: 32}
