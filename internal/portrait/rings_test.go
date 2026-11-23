package portrait_test

import (
	"reflect"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

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
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 0.5, Color: ui.RGBA(0, 0, 0, 0.16)},
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 3, Color: ui.RGBA(47, 111, 98, 0.34)},
			},
		},
		{
			name: "editing",
			got:  portrait.Rings.Editing,
			want: portrait.Shadows{
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 1, Color: ui.Hex("#2f6f62")},
				{Inset: false, X: 0, Y: 0, Blur: 0, Spread: 3, Color: ui.RGBA(47, 111, 98, 0.34)},
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
