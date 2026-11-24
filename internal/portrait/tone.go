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

var Tones = func() ToneScale {
	accent := newTone(Colors.Accent, Colors.Accent)
	return ToneScale{
		Neutral: newTone(Colors.Ink, Colors.Ink2),
		Accent:  accent,
		Success: newTone(Colors.Success, Colors.Success),
		Info:    accent,
		Warning: newTone(Colors.Warn, Colors.Warn),
		Danger:  newTone(Colors.Danger, Colors.Danger),
	}
}()

func newTone(solid, ink ui.Color) Tone {
	return Tone{
		Solid:   solid,
		OnSolid: Colors.OnAccent,
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
