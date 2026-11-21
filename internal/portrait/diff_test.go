package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

// Baseline: c6504af plugins/desktop-plugins.css.
func TestDiffTokensMatchTheWebUIBaseline(t *testing.T) {
	d := portrait.Diff
	for _, tc := range []struct {
		name          string
		got           portrait.TypeStyle
		size, leading float32
		family        string
	}{
		// .diff-hunk-range { font: var(--text-meta-size)/1.3 var(--font-mono) }
		{"range", d.Range, 12, 12 * 1.3, portrait.FontMono},
		// .diff-lines { font: var(--text-meta-size)/1.45 var(--font-mono) }
		{"lines", d.Lines, 12, 12 * 1.45, portrait.FontMono},
		// .diff-hunk-conflict { font-size: var(--text-meta-size) } in the sans root font
		{"conflict", d.Conflict, 12, 16, portrait.FontSans},
	} {
		if tc.got.Size != tc.size || !near(tc.got.Leading, tc.leading) || tc.got.Family != tc.family || tc.got.Tracking != 0 {
			t.Errorf("%s = %+v, want %v/%v in %q", tc.name, tc.got, tc.size, tc.leading, tc.family)
		}
	}
	// color-mix(in srgb, var(--danger) 10%, transparent), var(--accent) 12%.
	if !sameColor(d.Delete, ui.RGBA(168, 58, 58, 0.10)) {
		t.Errorf("delete %+v", d.Delete)
	}
	if !sameColor(d.Insert, ui.RGBA(47, 111, 98, 0.12)) {
		t.Errorf("insert %+v", d.Insert)
	}
	// .diff-lines { max-height: 16em } at 12px.
	if d.MaxLinesHeight != 192 {
		t.Errorf("max lines height %v, want 192", d.MaxLinesHeight)
	}
	// grid-template-columns: 3ch 3ch 1.5ch; IBM Plex Mono advances 600/1000 em.
	if !near(d.NumberWidth, 21.6) || !near(d.SigilWidth, 10.8) {
		t.Errorf("columns %v %v, want 21.6 10.8", d.NumberWidth, d.SigilWidth)
	}
}
