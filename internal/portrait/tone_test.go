package portrait_test

import (
	"math"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestTonesMeetWCAGAA(t *testing.T) {
	paper := wcagColor(portrait.Colors.Paper)
	black := ui.RGB(0, 0, 0)
	for _, tc := range []struct {
		name string
		tone portrait.Tone
	}{
		{"Neutral", portrait.Tones.Neutral},
		{"Accent", portrait.Tones.Accent},
		{"Info", portrait.Tones.Info},
		{"Warning", portrait.Tones.Warning},
		{"Danger", portrait.Tones.Danger},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tone := tc.tone
			wash := wcagOver(wcagColor(tone.Wash), paper)
			inkPaper := wcagContrast(wcagColor(tone.Ink), paper)
			inkWash := wcagContrast(wcagColor(tone.Ink), wash)
			assertWCAGContrast(t, "Ink on Paper", inkPaper, 4.5)
			assertWCAGContrast(t, "Ink on Wash", inkWash, 4.5)

			border := wcagContrast(wcagColor(tone.Border), paper)
			ring := wcagContrast(wcagColor(tone.Ring), paper)
			assertWCAGContrast(t, "Border on Paper", border, 3)
			assertWCAGContrast(t, "Ring on Paper", ring, 3)

			states := []struct {
				name  string
				solid ui.Color
			}{
				{"rest", tone.Solid},
				{"hover", tone.Solid.Mix(portrait.Colors.Paper, 0.08)},
				{"press", tone.Solid.Mix(black, 0.08)},
			}
			onSolid := make([]float64, len(states))
			for i, state := range states {
				onSolid[i] = wcagContrast(wcagColor(tone.OnSolid), wcagColor(state.solid))
				assertWCAGContrast(t, "OnSolid on "+state.name, onSolid[i], 4.5)
			}
			t.Logf("Ink/Paper %.3f; Ink/Wash %.3f; Border/Paper %.3f; Ring/Paper %.3f; OnSolid rest/hover/press %.3f/%.3f/%.3f",
				inkPaper, inkWash, border, ring, onSolid[0], onSolid[1], onSolid[2])
		})
	}
}

func assertWCAGContrast(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got < want {
		t.Errorf("%s contrast = %.4f, want at least %.1f", name, got, want)
	}
}

func wcagColor(c ui.Color) [4]float64 {
	return [4]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255, float64(c.A) / 255}
}

func wcagOver(foreground, background [4]float64) [4]float64 {
	alpha := foreground[3] + background[3]*(1-foreground[3])
	if alpha == 0 {
		return [4]float64{}
	}
	var composite [4]float64
	for i := 0; i < 3; i++ {
		composite[i] = (foreground[i]*foreground[3] + background[i]*background[3]*(1-foreground[3])) / alpha
	}
	composite[3] = alpha
	return composite
}

func wcagContrast(foreground, background [4]float64) float64 {
	foreground = wcagOver(foreground, background)
	l1 := wcagLuminance(foreground)
	l2 := wcagLuminance(background)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func wcagLuminance(c [4]float64) float64 {
	linear := func(channel float64) float64 {
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c[0]) + 0.7152*linear(c[1]) + 0.0722*linear(c[2])
}
