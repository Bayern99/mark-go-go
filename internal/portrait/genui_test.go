package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestGenUICardIsASheetOnTheCardRadius(t *testing.T) {
	g := portrait.GenUI
	if g.Card.From != portrait.Colors.Paper || g.Card.To != portrait.Colors.Paper {
		t.Fatalf("card fill %+v, want paper", g.Card)
	}
	want := portrait.Materials.Sheet.Shadows
	if len(g.Card.Shadows) != len(want) || g.Card.Shadows[0] != want[0] || g.Card.Shadows[1] != want[1] {
		t.Fatalf("card shadows %+v, want the sheet's hairline and drop", g.Card.Shadows)
	}
	if g.CardRadius != portrait.Radii.L || g.CardPadding != 12 || g.CardGap != 14 || g.Replacing != portrait.Colors.Code {
		t.Fatalf("card radius %v padding %v gap %v replacing %v", g.CardRadius, g.CardPadding, g.CardGap, g.Replacing)
	}
	if g.SkeletonRadius != 8 {
		t.Fatalf("skeleton radius %v, want 8", g.SkeletonRadius)
	}
}

func TestGenUICardsEnterStaggeredUpToTheSeventh(t *testing.T) {
	e := portrait.GenUI.Enter
	if e.Duration != 220*time.Millisecond || e.Ease != portrait.Motion.EaseOut || e.Rise != 5 || e.Blur != 2 {
		t.Fatalf("card entry %+v, want 220ms ease-out from 5px down, blurred 2px", e)
	}
	for i, want := range []time.Duration{0, 36, 72, 108, 144, 180, 216, 216, 216} {
		if got := e.Delay(i); got != want*time.Millisecond {
			t.Errorf("delay of card %d = %v, want %vms", i, got, want)
		}
	}
}

func TestGenUIHaloAndRimFollowTheirKeyframes(t *testing.T) {
	h, r := portrait.GenUI.Halo, portrait.GenUI.Rim
	if h.Duration != 950*time.Millisecond || h.Ease != (portrait.CubicBezier{0.16, 1, 0.3, 1}) || h.From != 0.32 || h.To != 2.15 || h.Center != (ui.Point{X: 0.86, Y: 0.24}) || h.Inset != 24 {
		t.Fatalf("halo %+v", h)
	}
	if r.Duration != 900*time.Millisecond || r.Ease != (portrait.CubicBezier{0.22, 1, 0.36, 1}) || r.From != -60 || r.To != 300 || r.Width != 1.5 {
		t.Fatalf("rim %+v", r)
	}
	for _, tc := range []struct {
		name   string
		frames []ui.Keyframe
		want   []ui.Keyframe
	}{
		{"halo opacity", h.Opacity(), []ui.Keyframe{{At: 0, Value: 0}, {At: 171 * time.Millisecond, Value: 1}, {At: 950 * time.Millisecond, Value: 0}}},
		{"rim opacity", r.Opacity(), []ui.Keyframe{{At: 0, Value: 0}, {At: 135 * time.Millisecond, Value: 0.95}, {At: 675 * time.Millisecond, Value: 0.72}, {At: 900 * time.Millisecond, Value: 0}}},
	} {
		if len(tc.frames) != len(tc.want) {
			t.Fatalf("%s = %+v", tc.name, tc.frames)
		}
		for i, f := range tc.frames {
			if f.At != tc.want[i].At || !near(f.Value, tc.want[i].Value) || (i > 0 && f.Ease == nil) {
				t.Errorf("%s frame %d = %+v, want %+v eased", tc.name, i, f, tc.want[i])
			}
		}
	}
}

func TestShimmerGlintSweepsAcrossOnceAPeriod(t *testing.T) {
	s := portrait.GenUI.Shimmer
	if s.Period != 1300*time.Millisecond || s.Base != portrait.Colors.Fill {
		t.Fatalf("shimmer %+v, want 1.3s over fill", s)
	}
	if s.Glint.A < 209 || s.Glint.A > 214 || s.Glint.R < 250 {
		t.Fatalf("glint %+v", s.Glint)
	}
	brightest := func(p float32) float32 {
		g := s.Gradient(p)
		if g.Angle != 90 || len(g.Stops) < 2 || g.Stops[0].At != 0 || g.Stops[len(g.Stops)-1].At != 1 {
			t.Fatalf("gradient at %v = %+v, want stops across the box left to right", p, g)
		}
		at, alpha := float32(-1), s.Base.A
		for _, st := range g.Stops {
			if st.Color.A > alpha {
				at, alpha = st.At, st.Color.A
			}
		}
		return at
	}
	early, mid := brightest(0.2), brightest(0.4)
	if early < 0 || mid < 0 || !(early < mid) {
		t.Fatalf("glint at 20%% = %v, 40%% = %v; want moving right", early, mid)
	}
	if g := s.Gradient(0); g.Stops[0].Color.A <= s.Base.A || g.Stops[len(g.Stops)-1].Color.A > s.Base.A+1 {
		t.Fatalf("at the start the glint's trailing half covers the left: %+v", g.Stops)
	}
}

func TestGenUISourceButtonsRestOnPaperAndWashOnHover(t *testing.T) {
	g := portrait.GenUI
	if g.Source.From != portrait.Colors.Paper || len(g.Source.Shadows) != 1 || g.Source.Shadows[0].Spread != 0.5 || g.Source.Shadows[0].Color != portrait.Colors.LineStrong {
		t.Fatalf("source button %+v, want paper with a 0.5px line-strong ring", g.Source)
	}
	if g.SourceHover.From != portrait.Colors.Code || g.SourceHover.To != portrait.Colors.Code || len(g.SourceHover.Shadows) != 1 || g.SourceHover.Shadows[0] != g.Source.Shadows[0] {
		t.Fatalf("hovered source button %+v, want the code wash inside the same ring", g.SourceHover)
	}
	if g.SourcePress.From != portrait.Materials.RaisedPress.From || g.SourcePress.To != portrait.Materials.RaisedPress.To {
		t.Fatalf("pressed source button %+v, want raised-press", g.SourcePress)
	}
}
