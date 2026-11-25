package portrait

import "github.com/egoist/mygo/ui"

const (
	toneSoftAlpha   = 0.16
	toneWashAlpha   = 0.08
	toneBorderAlpha = 0.72
	toneRingAlpha   = 0.85
)

type Tone struct {
	Solid, OnSolid, Ink, Soft, Wash, Border, Ring ui.Color
}

type ToneScale struct {
	Neutral, Accent, Success, Info, Warning, Danger Tone
}

var Tones ToneScale

func tonesFor(p Palette) ToneScale {
	accent := newTone(p, p.Accent, p.Accent)
	neutral := newTone(p, p.Ink, p.Ink2)
	// A neutral notice announces no semantic boundary: a hairline keeps it
	// calm and lets the wash carry the edge, where the semantic tones keep
	// the border that has to hold 3:1 on paper.
	neutral.Border = p.Line
	return ToneScale{
		Neutral: neutral,
		Accent:  accent,
		Success: newTone(p, p.Success, p.Success),
		Info:    accent,
		Warning: newTone(p, p.Warn, p.Warn),
		Danger:  newTone(p, p.Danger, p.Danger),
	}
}

func newTone(p Palette, solid, ink ui.Color) Tone {
	return Tone{
		Solid:   solid,
		OnSolid: p.OnAccent,
		Ink:     ink,
		Soft:    colorWithAlpha(solid, toneSoftAlpha),
		Wash:    colorWithAlpha(solid, toneWashAlpha),
		Border:  colorWithAlpha(ink, toneBorderAlpha),
		Ring:    colorWithAlpha(ink, toneRingAlpha),
	}
}

func colorWithAlpha(c ui.Color, alpha float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, alpha)
}
