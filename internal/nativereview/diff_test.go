package nativereview

import (
	"image"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func fixtureHunks() []review.DiffHunk {
	return []review.DiffHunk{{
		ID: "hunk-1", Header: "@@ -4,3 +4,3 @@",
		Lines: []review.DiffLine{
			{Kind: review.DiffLineContext, Text: "context line", OldLine: 4, NewLine: 4},
			{Kind: review.DiffLineDelete, Text: "removed line", OldLine: 5},
			{Kind: review.DiffLineInsert, Text: "inserted line", NewLine: 5},
			{Kind: review.DiffLineContext, Text: "写作 trailing", OldLine: 6, NewLine: 6},
		},
	}, {
		ID: "hunk-2", Header: "@@ -20 +20 @@", Conflict: true, ConflictReason: "edited concurrently",
		Lines: []review.DiffLine{{Kind: review.DiffLineDelete, Text: "stale", OldLine: 20}},
	}}
}

func diffTester(t *testing.T, scale float32, decisions map[string]review.HunkDecision) *ui.Tester {
	t.Helper()
	hunks := fixtureHunks()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(8).Children(func() {
			drawDiff(c, hunks, decisions)
		})
	}, 640, 600)
	tt.SetScale(scale)
	tt.Frame()
	return tt
}

func find(t *testing.T, tt *ui.Tester, s string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q is not drawn; texts %q", s, tt.Texts())
	}
	return r
}

func pixel(img *image.RGBA, scale, x, y float32) [3]int {
	c := img.RGBAAt(int(x*scale), int(y*scale))
	return [3]int{int(c.R), int(c.G), int(c.B)}
}

func over(c ui.Color) [3]int {
	o := c.Over(portrait.Colors.Paper)
	return [3]int{int(o.R), int(o.G), int(o.B)}
}

func close3(a, b [3]int) bool {
	for i := range a {
		if d := a[i] - b[i]; d > 3 || d < -3 {
			return false
		}
	}
	return true
}

func TestDiffLinesAreTintedWithPortraitInsertAndDeleteColors(t *testing.T) {
	tt := diffTester(t, 1, nil)
	img := tt.Image()
	for _, tc := range []struct {
		text string
		want ui.Color
	}{
		{"removed line", portrait.Diff.Delete},
		{"inserted line", portrait.Diff.Insert},
	} {
		r := find(t, tt, tc.text)
		if got, want := pixel(img, 1, r.X+r.W-4, r.Y+r.H/2), over(tc.want); !close3(got, want) {
			t.Errorf("%q row is painted %v, want %v", tc.text, got, want)
		}
	}
	ctx := find(t, tt, "context line")
	if got := pixel(img, 1, ctx.X+ctx.W-4, ctx.Y+ctx.H/2); got != [3]int{255, 255, 255} {
		t.Errorf("context row is painted %v, want paper", got)
	}
}

func TestDiffLinesAreSetInMonoMetaLeading(t *testing.T) {
	tt := diffTester(t, 1, nil)
	a := find(t, tt, "removed line")
	b := find(t, tt, "inserted line")
	want := portrait.Diff.Lines.Leading + 2
	if got := b.Y - a.Y; got < want-0.5 || got > want+0.5 {
		t.Fatalf("diff lines are %v apart, want %v", got, want)
	}
	if portrait.Diff.Lines.Family != portrait.FontMono {
		t.Fatalf("diff lines use %q", portrait.Diff.Lines.Family)
	}
}

func TestDiffShowsLineNumbersAndSigilsInTheirColumns(t *testing.T) {
	tt := diffTester(t, 1, nil)
	removed := find(t, tt, "removed line")
	inserted := find(t, tt, "inserted line")
	if removed.X != inserted.X {
		t.Fatalf("line text starts at %v and %v; columns must align", removed.X, inserted.X)
	}
	for _, s := range []string{"−", "+", "20"} {
		find(t, tt, s)
	}
	minus := find(t, tt, "−")
	if minus.X >= removed.X || minus.Y > removed.Y+removed.H || minus.Y+minus.H < removed.Y {
		t.Fatalf("sigil %v is not left of its line %v", minus, removed)
	}
}

func TestDiffLinesDescribeTheirChangeAndLineNumbers(t *testing.T) {
	tt := diffTester(t, 1, nil)
	for _, tc := range []struct{ text, desc string }{
		{"context line", "Unchanged, lines 4→4"},
		{"removed line", "Removed, line 5"},
		{"inserted line", "Added, line 5"},
		{"写作 trailing", "Unchanged, lines 6→6"},
		{"stale", "Removed, line 20"},
	} {
		info, ok := tt.Access(tc.text)
		if !ok {
			t.Fatalf("%q is not in the access tree", tc.text)
		}
		if info.Description != tc.desc {
			t.Errorf("%q has description %q, want %q", tc.text, info.Description, tc.desc)
		}
	}
}

func TestBlankDiffChangesHaveAccessibleNames(t *testing.T) {
	for _, tc := range []struct{ name, before, after, want string }{
		{"insert", "A\nB\n", "A\n\nB\n", "Added, line 2"},
		{"delete", "A\n\nB\n", "A\nB\n", "Removed, line 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hunks := review.ComputeDiff(tc.before, tc.after)
			tt := ui.NewTester(func(c *ui.Context) { drawDiff(c, hunks, nil) }, 640, 600)
			tt.Frame()
			info, ok := tt.Access(tc.want)
			if !ok || info.Offscreen || info.Bounds.H <= 0 {
				t.Fatalf("blank change %q is missing from the visible access tree: %+v", tc.want, info)
			}
		})
	}
}

func TestDiffHunkBorderMarksConflicts(t *testing.T) {
	tt := diffTester(t, 1, nil)
	img := tt.Image()
	clean := find(t, tt, "@@ -4,3 +4,3 @@")
	conflict := find(t, tt, "@@ -20 +20 @@")
	edge := func(r ui.Rect) [3]int {
		for x := r.X - 1; x > r.X-12; x-- {
			if p := pixel(img, 1, x, r.Y+r.H/2); p != [3]int{255, 255, 255} {
				return p
			}
		}
		return [3]int{255, 255, 255}
	}
	if got, want := edge(clean), over(portrait.Colors.Line); !close3(got, want) {
		t.Errorf("clean hunk border %v, want line %v", got, want)
	}
	if got, want := edge(conflict), over(portrait.Colors.Danger); !close3(got, want) {
		t.Errorf("conflict hunk border %v, want danger %v", got, want)
	}
	if info, ok := tt.AccessByRole(ui.RoleStatus, "Conflict: edited concurrently"); !ok || info.Bounds.Y > conflict.Y+conflict.H {
		t.Errorf("conflict status %+v (found %v) is not in the hunk header", info, ok)
	}
}

func TestDiffHunkActionsSitInTheHeader(t *testing.T) {
	decisions := map[string]review.HunkDecision{}
	tt := diffTester(t, 1, decisions)
	header := find(t, tt, "@@ -4,3 +4,3 @@")
	accept, ok := tt.Access("Accept hunk")
	if !ok {
		t.Fatal("no Accept hunk button")
	}
	if accept.Bounds.Y > header.Y+header.H || accept.Bounds.Y+accept.Bounds.H < header.Y {
		t.Fatalf("Accept hunk %v is not beside the range %v", accept.Bounds, header)
	}
	if err := tt.Click("Accept hunk"); err != nil {
		t.Fatal(err)
	}
	if decisions["hunk-1"] != review.HunkAccepted {
		t.Fatalf("decisions %v", decisions)
	}
}

const glyphScale = 4

func inkBox(img *image.RGBA, r ui.Rect) (image.Rectangle, int) {
	box, ink := image.Rectangle{}, 0
	for y := int(r.Y * glyphScale); y < int((r.Y+r.H)*glyphScale); y++ {
		for x := int(r.X * glyphScale); x < int((r.X+r.W)*glyphScale); x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 3*160 {
				ink += 255*3 - int(c.R) - int(c.G) - int(c.B)
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box.Sub(box.Min), ink
}

func referenceGlyph(t *testing.T, family, glyph string) (image.Rectangle, int) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(portrait.Colors.Paper).Padding(20).Children(func() {
			ui.Text(c, glyph).Font(family).FontSize(portrait.Diff.Lines.Size).TextColor(portrait.Colors.Ink)
		})
	}, 200, 120)
	tt.SetScale(glyphScale)
	tt.Frame()
	r := find(t, tt, glyph)
	return inkBox(tt.Image(), ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4})
}

func TestDiffSetsHanInIBMPlexSansSC(t *testing.T) {
	tt := diffTester(t, glyphScale, nil)
	line := find(t, tt, "写作 trailing")
	size := portrait.Diff.Lines.Size
	got, gotInk := inkBox(tt.Image(), ui.Rect{X: line.X - 1, Y: line.Y, W: size + 1, H: line.H})
	want, wantInk := referenceGlyph(t, "IBM Plex Sans SC", "写")
	system, _ := referenceGlyph(t, "sans-serif", "写")
	if want == system {
		t.Skip("system fallback draws 写 like IBM Plex Sans SC; the test cannot tell them apart")
	}
	if !similar(got, want) || similar(got, system) || gotInk < wantInk*8/10 || gotInk > wantInk*12/10 {
		t.Fatalf("写 in the diff has ink box %v (%d), want IBM Plex Sans SC %v (%d); system %v", got, gotInk, want, wantInk, system)
	}
}

func similar(a, b image.Rectangle) bool {
	dx, dy := a.Dx()-b.Dx(), a.Dy()-b.Dy()
	return dx*dx <= 4 && dy*dy <= 4
}
