package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// Baseline: styles.css mgo-jump-fade (1828-1845): accent at 16% with
// opacity .65, at 12% by 35%, transparent at the end, over --duration-jump
// along --ease-out.
func TestJumpFadeMatchesTheWebUIKeyframes(t *testing.T) {
	accent := func(a float32) ui.Color { return ui.RGBA(47, 111, 98, a) }
	want := []portrait.TintFrame{{At: 0, Color: accent(0.16), Opacity: 0.65}, {At: 0.35, Color: accent(0.12), Opacity: 1}, {At: 1, Color: ui.Transparent, Opacity: 1}}
	got := portrait.Highlights.Jump
	if got.Duration != 420*time.Millisecond || got.Ease != portrait.Motion.EaseOut || got.Radius != 6 || len(got.Frames) != len(want) {
		t.Fatalf("jump highlight %+v", got)
	}
	for i := range want {
		if got.Frames[i] != want[i] {
			t.Errorf("frame %d = %+v, want %+v", i, got.Frames[i], want[i])
		}
	}
}

func TestJumpFadeFadesOutAndEndsTransparent(t *testing.T) {
	j := portrait.Highlights.Jump
	start, peak, late, end := j.At(0), j.At(j.Duration*35/100), j.At(j.Duration*80/100), j.At(j.Duration)
	if !(start.A > 0 && peak.A > start.A && late.A < peak.A && end.A == 0) {
		t.Fatalf("jump tint alphas %v %v %v %v, want a rise to 35%% then a fade to nothing", start.A, peak.A, late.A, end.A)
	}
	if after := j.At(time.Second); after.A != 0 {
		t.Fatalf("jump tint after it ends = %v", after)
	}
}

// Baseline: @phosphor-icons/web regular codepoints the editor uses.
func TestEditorIconsArePhosphorCodepoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		icon portrait.Icon
		want string
	}{
		{"caret-down", portrait.IconCaretDown, "\ue136"},
		{"caret-right", portrait.IconCaretRight, "\ue13a"},
		{"arrow-up-right", portrait.IconArrowUpRight, "\ue092"},
		{"note", portrait.IconNote, "\ue348"},
		{"lightbulb", portrait.IconLightbulb, "\ue2dc"},
		{"info", portrait.IconInfo, "\ue2ce"},
		{"warning", portrait.IconWarning, "\ue4e0"},
		{"warning-octagon", portrait.IconWarningOctagon, "\ue4e4"},
		{"check", portrait.IconCheck, "\ue182"},
	} {
		if tc.icon.Glyph != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.icon.Glyph, tc.want)
		}
	}
}
