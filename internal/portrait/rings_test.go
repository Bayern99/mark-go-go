package portrait_test

import (
	"reflect"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestFocusRingInnerEdgeMeetsNonTextContrast(t *testing.T) {
	eachLook(t, func(t *testing.T, space portrait.Space, dark bool) {
		ring := portrait.Rings.Field[0]
		if ring.Spread != 1 || ring.Color != portrait.Colors.Accent {
			t.Fatalf("inner focus ring = %+v, want the one-point Accent edge", ring)
		}
		contrast := wcagContrast(wcagColor(ring.Color), wcagColor(portrait.Colors.Paper))
		if contrast < 3 {
			t.Errorf("%s/%s focus contrast %.3f, want >= 3", space.Name, appearanceName(dark), contrast)
		}
	})
}

func TestRingsMatchTheirBaseline(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  portrait.Shadows
		want portrait.Shadows
	}{
		{
			name: "field",
			got:  portrait.Rings.Field,
			want: portrait.Shadows{
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 1, Color: ui.Hex("#2E5D8C")},
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 3, Color: ui.Hex("#2E5D8C").Alpha(0.28)},
			},
		},
		{
			name: "editing",
			got:  portrait.Rings.Editing,
			want: portrait.Shadows{
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 1, Color: ui.Hex("#2E5D8C")},
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 3, Color: ui.Hex("#2E5D8C").Alpha(0.28)},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Errorf("Rings.%s = %+v, want %+v", tc.name, tc.got, tc.want)
			}
		})
	}
}
