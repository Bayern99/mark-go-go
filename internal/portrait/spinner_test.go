package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestSpinnerTurnsAndHoldsStillUnderReducedMotion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reduced bool
	}{{"motion", false}, {"reduced motion", true}} {
		t.Run(tc.name, func(t *testing.T) {
			tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.Spinner(c, "Feynman is working") }), 160, 120)
			tt.SetPreferences(ui.Preferences{ReduceMotion: tc.reduced})
			tt.Frame()
			info, ok := tt.AccessByRole(ui.RoleProgress, "Feynman is working")
			if !ok {
				t.Fatal("spinner has no accessible progress indicator")
			}
			if info.Bounds.W < 12 || info.Bounds.H < 12 {
				t.Fatalf("spinner bounds %+v, want a visible glyph", info.Bounds)
			}
			before := region(tt.Image(), info.Bounds)
			tt.Advance(225 * time.Millisecond)
			n := differs(before, region(tt.Image(), info.Bounds))
			if tc.reduced && n != 0 {
				t.Fatalf("spinner moved %d pixels under reduced motion", n)
			}
			if !tc.reduced && n == 0 {
				t.Fatal("spinner did not turn")
			}
		})
	}
}

func TestSpinnerTurnsOncePerPeriod(t *testing.T) {
	if portrait.SpinPeriod != 900*time.Millisecond {
		t.Fatalf("SpinPeriod = %v, want the WebUI's fy-spin 900ms", portrait.SpinPeriod)
	}
}
