package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestScrollEdgeMaskXShowsOnlyTheOverflowingSide(t *testing.T) {
	tests := []struct {
		name                string
		scroll              ui.ScrollState
		wantLeft, wantRight bool
	}{
		{name: "not laid out", wantLeft: false, wantRight: false},
		{name: "no overflow", scroll: ui.ScrollState{}, wantLeft: false, wantRight: false},
		{name: "overflow at start", scroll: ui.ScrollState{MaxX: 200}, wantLeft: false, wantRight: true},
		{name: "overflow on both sides", scroll: ui.ScrollState{X: 40, MaxX: 200}, wantLeft: true, wantRight: true},
		{name: "end", scroll: ui.ScrollState{X: 200, MaxX: 200}, wantLeft: true, wantRight: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left, gotLeft := portrait.ScrollEdgeMaskX(&tc.scroll, true)
			right, gotRight := portrait.ScrollEdgeMaskX(&tc.scroll, false)
			if gotLeft != tc.wantLeft || gotRight != tc.wantRight {
				t.Fatalf("left / right visibility = %v / %v, want %v / %v", gotLeft, gotRight, tc.wantLeft, tc.wantRight)
			}
			if gotLeft && (left.Angle != 90 || left.From != portrait.Colors.Paper || left.To != ui.Transparent) {
				t.Errorf("left fade = %+v, want paper fading rightward to transparent", left)
			}
			if gotRight && (right.Angle != 90 || right.From != ui.Transparent || right.To != portrait.Colors.Paper) {
				t.Errorf("right fade = %+v, want transparent fading rightward to paper", right)
			}
		})
	}
}
