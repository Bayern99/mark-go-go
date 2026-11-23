package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestSkeletonShimmersAndHoldsStillUnderReducedMotion(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		t.Run(map[bool]string{false: "motion", true: "reduced motion"}[reduced], func(t *testing.T) {
			tt := ui.NewTesterWith(onChrome(func(c *ui.Context) {
				portrait.DefaultSkeleton.View(c, "Loading content").Width(120)
			}), 200, 120, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduced}})
			tt.Frame()
			info, ok := tt.AccessByRole(ui.RoleProgress, "Loading content")
			if !ok || info.Bounds.H != portrait.Sizes.Regular {
				t.Fatalf("skeleton accessibility or size: %+v, found %v", info, ok)
			}
			before := region(tt.Image(), info.Bounds)
			tt.Advance(400 * time.Millisecond)
			moved := differs(before, region(tt.Image(), info.Bounds)) != 0
			if moved == reduced || tt.FramePending() == reduced {
				t.Fatalf("reduced %v: skeleton moved %v, frames pending %v", reduced, moved, tt.FramePending())
			}
		})
	}
}

func TestDefaultSkeletonKeepsGenUIShimmerValues(t *testing.T) {
	if portrait.GenUI.Shimmer != portrait.DefaultSkeleton {
		t.Fatalf("GenUI shimmer = %+v, want DefaultSkeleton %+v", portrait.GenUI.Shimmer, portrait.DefaultSkeleton)
	}
	if portrait.DefaultSkeleton.Period != 1300*time.Millisecond || portrait.DefaultSkeleton.Base != portrait.Colors.Fill {
		t.Fatalf("default skeleton = %+v, want 1.3s over Fill", portrait.DefaultSkeleton)
	}
}
