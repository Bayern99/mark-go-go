package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestDiffTokensMatchTheirLockedValues(t *testing.T) {
	d := portrait.Diff
	for _, tc := range []struct {
		name          string
		got           portrait.TypeStyle
		size, leading float32
		family        string
	}{
		{"range", d.Range, 12, 12 * 1.3, portrait.FontMono},
		{"lines", d.Lines, 12, 12 * 1.45, portrait.FontMono},
		{"conflict", d.Conflict, 12, 16, portrait.FontSans},
	} {
		if tc.got.Size != tc.size || !near(tc.got.Leading, tc.leading) || tc.got.Family != tc.family || tc.got.Tracking != 0 {
			t.Errorf("%s = %+v, want %v/%v in %q", tc.name, tc.got, tc.size, tc.leading, tc.family)
		}
	}
	if !sameColor(d.Delete, ui.RGBA(168, 58, 58, 0.10)) {
		t.Errorf("delete %+v", d.Delete)
	}
	if !sameColor(d.Insert, portrait.Colors.Success.Alpha(0.12)) {
		t.Errorf("insert %+v", d.Insert)
	}
	if !near(d.NumberWidth, 21.6) || !near(d.SigilWidth, 10.8) {
		t.Errorf("columns %v %v, want 21.6 10.8", d.NumberWidth, d.SigilWidth)
	}
}
