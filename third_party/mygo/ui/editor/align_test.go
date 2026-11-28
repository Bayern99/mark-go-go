package editor

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

const alignTestWidth = 160

const alignTestSource = "ab\nThe quick brown fox 中文跳过了 the lazy dog 然后继续奔跑 over the fields 一直到尽头。\nlast"

func alignTestState(source string) *State {
	return &State{
		Buffer: NewBuffer(source), SoftWrap: true,
		LineNumbers: LineNumbersOff, NoFolding: true, NoActiveLine: true,
		NoScrollbars: true, NoScrollPastEnd: true,
		ReadingColumn: ReadingColumn{Width: alignTestWidth},
		Font:          ui.Font{Size: 16}, LineHeight: 1.5,
		Theme: &Theme{
			Background: ui.Hex("#ffffff"), Text: ui.Hex("#000000"),
			Selection: ui.Hex("#0000ff"), InactiveSelection: ui.Hex("#0000ff"),
			Cursor: ui.Hex("#ff0000"),
		},
	}
}

func alignTestEditor(t *testing.T, source string, align TextAlign, wrap bool) (*State, *ui.Tester) {
	t.Helper()
	st := alignTestState(source)
	st.SoftWrap = wrap
	st.SetTextAlign(align)
	tt := ui.NewTester(func(c *ui.Context) { View(c, st).Fill().AutoFocus() }, 320, 240)
	tt.Frame()
	if got := st.ed.view.alignWidth; got != alignTestWidth {
		t.Fatalf("width of the text = %v, want the reading column %v", got, alignTestWidth)
	}
	if got := st.TextAlign(); got != align {
		t.Fatalf("State.TextAlign() = %v, want %v", got, align)
	}
	return st, tt
}

func wantAlignOffset(align TextAlign, avail, used float32) float32 {
	slack := avail - used
	if slack <= 0 {
		return 0
	}
	switch align {
	case AlignCenter:
		return float32(math.Round(float64(slack) / 2))
	case AlignRight:
		return float32(math.Floor(float64(slack)))
	}
	return 0
}

func alignRows(v *view, line int) (l *lineLayout, starts, ends []int) {
	l = v.line(line)
	for _, row := range l.rows {
		starts = append(starts, l.start+l.fromDisp(row.from))
		ends = append(ends, l.start+l.fromDisp(row.to))
	}
	return l, starts, ends
}

func rowMidY(st *State, line, ri int) float32 {
	v := &st.ed.view
	y := st.LineTop(line) - st.ScrollY() + float64(ri)*float64(v.rowHeight(line))
	return float32(y) + v.rowHeight(line)/2
}

func selected(c image.Image, x, y int) bool {
	r, g, b, _ := c.At(x, y).RGBA()
	return b > r && b > g && b > 0x8000
}

func paintedExtent(img *image.RGBA, y int) (lo, hi int) {
	lo, hi = -1, -1
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		if selected(img, x, y) {
			if lo < 0 {
				lo = x
			}
			hi = x
		}
	}
	return lo, hi
}

func TestTextAlignDefaultsToLeft(t *testing.T) {
	st := alignTestState(alignTestSource)
	if got := st.TextAlign(); got != AlignLeft {
		t.Fatalf("TextAlign() of a zero State = %v, want AlignLeft", got)
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, st).Fill().AutoFocus() }, 320, 240)
	tt.Frame()
	if got := st.TextAlign(); got != AlignLeft {
		t.Fatalf("TextAlign() = %v, want AlignLeft", got)
	}
	v := &st.ed.view
	if got := v.alignWidth; got != alignTestWidth {
		t.Fatalf("width of the text = %v, want %v", got, alignTestWidth)
	}
	textX := st.ed.textX - st.ed.scrollX
	for line := 0; line < st.Buffer.Snapshot().LineCount(); line++ {
		l, starts, _ := alignRows(v, line)
		for ri := range l.rows {
			row := &l.rows[ri]
			if row.align != 0 {
				t.Errorf("line %d row %d offset = %v, want 0 at the left", line, ri, row.align)
			}
			if got := row.startX(); got != row.indent {
				t.Errorf("line %d row %d starts at %v, want its indent %v", line, ri, got, row.indent)
			}
		}
		r, ok := st.Rect(starts[0])
		if !ok {
			t.Fatalf("line %d has no caret", line)
		}
		if r.X != textX {
			t.Errorf("line %d first caret x = %v, want the text to start at %v", line, r.X, textX)
		}
	}
}

func TestTextAlignCentersEachWrappedRow(t *testing.T) {
	st, tt := alignTestEditor(t, alignTestSource, AlignCenter, true)
	v := &st.ed.view
	textX := st.ed.textX - st.ed.scrollX
	l, starts, ends := alignRows(v, 1)
	if len(l.rows) < 3 {
		t.Fatalf("the test line wrapped into %d rows, want at least 3", len(l.rows))
	}
	offsets := map[float32]bool{}
	for ri := range l.rows {
		row := &l.rows[ri]
		used := row.indent + row.width
		want := row.indent + wantAlignOffset(AlignCenter, alignTestWidth, used)
		if got := row.startX(); math.Abs(float64(got-want)) > 0.01 {
			t.Errorf("row %d starts at %v, want %v (row %v wide in %v)", ri, got, want, used, alignTestWidth)
		}
		if want <= 0 {
			t.Errorf("row %d is not centered: offset %v", ri, want)
		}
		offsets[row.startX()] = true
		st.SetSelections(Selection{Anchor: ends[ri], Head: starts[ri]})
		tt.Frame()
		lo, hi := paintedExtent(tt.Image(), int(rowMidY(st, 1, ri)))
		if lo < 0 {
			t.Fatalf("row %d painted no selection", ri)
		}
		leftGap := float32(lo) - textX
		rightGap := (textX + alignTestWidth) - float32(hi+1)
		if math.Abs(float64(leftGap-rightGap)) > 3 {
			t.Errorf("row %d selection leaves %v at the left and %v at the right, want them equal", ri, leftGap, rightGap)
		}
		if math.Abs(float64(leftGap-row.startX())) > 3 {
			t.Errorf("row %d selection starts at %v, want the row to start at %v", ri, leftGap, row.startX())
		}
	}
	if len(offsets) < 2 {
		t.Errorf("every row of the wrapped line starts at the same offset %v; want each centered on its own width", offsets)
	}
}

func TestTextAlignRightEndsRowsAtTheRightEdge(t *testing.T) {
	st, tt := alignTestEditor(t, alignTestSource, AlignRight, true)
	v := &st.ed.view
	textX := st.ed.textX - st.ed.scrollX
	l, starts, ends := alignRows(v, 1)
	if len(l.rows) < 3 {
		t.Fatalf("the test line wrapped into %d rows, want more", len(l.rows))
	}
	for ri := range l.rows {
		row := &l.rows[ri]
		want := row.indent + wantAlignOffset(AlignRight, alignTestWidth, row.indent+row.width)
		if got := row.startX(); math.Abs(float64(got-want)) > 0.01 {
			t.Errorf("row %d starts at %v, want %v", ri, got, want)
		}
		if end := row.startX() + row.width; math.Abs(float64(end-alignTestWidth)) > 1 {
			t.Errorf("row %d ends at %v, want the right edge %v", ri, end, alignTestWidth)
		}
		st.SetSelections(Selection{Anchor: ends[ri], Head: starts[ri]})
		tt.Frame()
		lo, hi := paintedExtent(tt.Image(), int(rowMidY(st, 1, ri)))
		if lo < 0 {
			t.Fatalf("row %d painted no selection", ri)
		}
		if rightGap := (textX + alignTestWidth) - float32(hi+1); rightGap > 3 || rightGap < -3 {
			t.Errorf("row %d selection ends %v before the right edge, want it flush", ri, rightGap)
		}
		if lo <= int(textX) {
			t.Errorf("row %d selection starts at %d, in the left of the text", ri, lo)
		}
	}
}

func TestTextAlignTranslatesWhatItPaints(t *testing.T) {
	left, leftTester := alignTestEditor(t, alignTestSource, AlignLeft, true)
	center, centerTester := alignTestEditor(t, alignTestSource, AlignCenter, true)
	if left.ed.textX != center.ed.textX {
		t.Fatalf("the editors lay their text out at %v and %v", left.ed.textX, center.ed.textX)
	}
	l, starts, ends := alignRows(&left.ed.view, 1)
	centerRows := center.ed.view.line(1).rows
	for ri := range l.rows {
		left.Select(starts[ri], ends[ri])
		leftTester.Frame()
		center.Select(starts[ri], ends[ri])
		centerTester.Frame()
		y := int(rowMidY(left, 1, ri))
		leftLo, leftHi := paintedExtent(leftTester.Image(), y)
		centerLo, centerHi := paintedExtent(centerTester.Image(), y)
		if leftLo < 0 || centerLo < 0 {
			t.Fatalf("row %d painted no selection", ri)
		}
		offset := centerRows[ri].startX()
		if got := float32(centerLo - leftLo); math.Abs(float64(got-offset)) > 3 {
			t.Errorf("row %d selection moved %v, want the row's offset %v", ri, got, offset)
		}
		if got, want := centerHi-centerLo, leftHi-leftLo; got < want-3 || got > want+3 {
			t.Errorf("row %d selection is %d wide when centered, %d wide at the left", ri, got, want)
		}
	}
}

func TestTextAlignEmptyLineCaret(t *testing.T) {
	const source = "\n\nab"
	for _, tc := range []struct {
		align TextAlign
		want  float32
	}{
		{AlignLeft, 0},
		{AlignCenter, alignTestWidth / 2},
		{AlignRight, alignTestWidth},
	} {
		st, tt := alignTestEditor(t, source, tc.align, true)
		v := &st.ed.view
		textX := st.ed.textX - st.ed.scrollX
		for line := range 2 {
			l := v.line(line)
			row := &l.rows[0]
			if row.width != 0 {
				t.Fatalf("%v: line %d is not empty: %v wide", tc.align, line, row.width)
			}
			if got := row.startX(); math.Abs(float64(got-tc.want)) > 0.01 {
				t.Errorf("%v: empty line %d starts at %v, want %v", tc.align, line, got, tc.want)
			}
			r, ok := st.Rect(l.start)
			if !ok {
				t.Fatalf("%v: empty line %d has no caret", tc.align, line)
			}
			if got := r.X - textX; math.Abs(float64(got-tc.want)) > 0.01 {
				t.Errorf("%v: empty line %d caret x = %v, want %v", tc.align, line, got, tc.want)
			}
			if r.X < textX || r.X > textX+alignTestWidth {
				t.Errorf("%v: empty line %d caret at %v is outside the text", tc.align, line, r.X)
			}
			tt.ClickAt(r.X+0.5, rowMidY(st, line, 0))
			if got := st.Selection().Head; got != l.start {
				t.Errorf("%v: click on empty line %d put the caret at %d, want %d", tc.align, line, got, l.start)
			}
		}
	}
}

func TestTextAlignClicksMatchRowGeometry(t *testing.T) {
	for _, align := range []TextAlign{AlignCenter, AlignRight} {
		st, tt := alignTestEditor(t, alignTestSource, align, true)
		v := &st.ed.view
		textX := st.ed.textX - st.ed.scrollX
		l, starts, ends := alignRows(v, 1)
		for ri := range l.rows {
			row := &l.rows[ri]
			y := rowMidY(st, 1, ri)
			for off := starts[ri]; off < ends[ri]; off++ {
				if !isRuneStart(l.text, off-l.start) {
					continue
				}
				x := textX + v.xOf(l, row, l.toDisp(off-l.start, false))
				tt.ClickAt(x+0.5, y)
				if got := st.Selection().Head; got != off {
					t.Errorf("%v: click on byte %d of row %d put the caret at %d", align, off, ri, got)
				}
			}
			tt.ClickAt(textX+0.5, y)
			if got := st.Selection().Head; got != starts[ri] {
				t.Errorf("%v: click in the left blank of row %d put the caret at %d, want %d", align, ri, got, starts[ri])
			}
			tt.ClickAt(textX+alignTestWidth-0.5, y)
			want := ends[ri]
			if ri < len(l.rows)-1 {
				want = runeStartBefore(l.text, ends[ri]-l.start) + l.start
			}
			if got := st.Selection().Head; got != want {
				t.Errorf("%v: click in the right blank of row %d put the caret at %d, want %d", align, ri, got, want)
			}
		}
	}
}

func runeStartBefore(s string, end int) int {
	for end > 0 && end < len(s) && !isRuneStart(s, end) {
		end--
	}
	end--
	for end > 0 && !isRuneStart(s, end) {
		end--
	}
	return max(end, 0)
}

func TestTextAlignRowGeometryIsShared(t *testing.T) {
	for _, align := range []TextAlign{AlignLeft, AlignCenter, AlignRight} {
		st, _ := alignTestEditor(t, alignTestSource, align, true)
		v := &st.ed.view
		textX := st.ed.textX - st.ed.scrollX
		l, _, _ := alignRows(v, 1)
		for ri := range l.rows {
			row := &l.rows[ri]
			for d := row.from; d < row.to; d++ {
				if !isRuneStart(l.disp, d) {
					continue
				}
				next := row.to
				for n := d + 1; n < row.to; n++ {
					if isRuneStart(l.disp, n) {
						next = n
						break
					}
				}
				rects := v.rangeRects(l, row, d, next, nil)
				if len(rects) != 1 {
					t.Fatalf("%v: byte %d of row %d has %d rectangles, want one", align, d, ri, len(rects))
				}
				from := textX + v.xOf(l, row, d)
				to := textX + v.xOf(l, row, next)
				if math.Abs(float64(rects[0].X+textX-from)) > 1 {
					t.Errorf("%v: row %d byte %d starts at %v, its caret at %v", align, ri, d, rects[0].X+textX, from)
				}
				if math.Abs(float64(rects[0].X+rects[0].W+textX-to)) > 1 {
					t.Errorf("%v: row %d byte %d ends at %v, its caret at %v", align, ri, d, rects[0].X+rects[0].W+textX, to)
				}
			}
		}
	}
}

func isRuneStart(s string, i int) bool {
	return i >= 0 && i < len(s) && (s[i]&0xC0) != 0x80
}

func TestTextAlignKeepsRowsWithinTheText(t *testing.T) {
	const source = "one two three four five six seven eight nine ten eleven twelve thirteen"
	for _, align := range []TextAlign{AlignCenter, AlignRight} {
		st, _ := alignTestEditor(t, source, align, false)
		v := &st.ed.view
		l := v.line(0)
		if len(l.rows) != 1 {
			t.Fatalf("%v: the line wrapped into %d rows without wrapping", align, len(l.rows))
		}
		row := &l.rows[0]
		if row.width <= alignTestWidth {
			t.Fatalf("%v: the line is %v wide, not wider than the text %v", align, row.width, alignTestWidth)
		}
		if got := row.startX(); got != row.indent {
			t.Errorf("%v: the overflowing row starts at %v, want its indent %v", align, got, row.indent)
		}
		if got := row.align; got != 0 {
			t.Errorf("%v: the overflowing row is offset by %v, want 0", align, got)
		}
		if r, _ := st.Rect(0); r.X < st.ed.textX-st.ed.scrollX {
			t.Errorf("%v: the caret of the overflowing line is at %v, left of the text at %v", align, r.X, st.ed.textX)
		}
	}
}

func TestTextAlignIMEAnchorSharesCaret(t *testing.T) {
	for _, align := range []TextAlign{AlignLeft, AlignCenter, AlignRight} {
		st, _ := alignTestEditor(t, alignTestSource, align, true)
		v := &st.ed.view
		l, starts, ends := alignRows(v, 1)
		for ri := range l.rows {
			from := starts[ri]
			if ri == 0 {
				from = starts[0] + len("The ")
			}
			to := from + len("中文")
			if to > ends[ri] {
				continue
			}
			cr, ok := st.ed.caretRect(from)
			if !ok {
				t.Fatalf("%v: no caret at %d", align, from)
			}
			want := st.ed.textX + cr.X - st.ed.scrollX
			bounds, _, ok := st.ed.BoundsForRange(ui.TextInputRange{Start: v.text.UTF16(from), End: v.text.UTF16(to)})
			if !ok {
				t.Fatalf("%v: no bounds for %d..%d", align, from, to)
			}
			if math.Abs(float64(bounds.X-want)) > 0.01 {
				t.Errorf("%v: input method anchor at %v, caret at %v", align, bounds.X, want)
			}
			if r, ok := st.Rect(from); !ok || math.Abs(float64(r.X-want)) > 0.01 {
				t.Errorf("%v: Rect(%d).X = %v, caret at %v", align, from, r.X, want)
			}
			if zr, ok := st.ed.caretRect(to); ok {
				if got, want := bounds.W, zr.X-cr.X; math.Abs(float64(got-want)) > 0.01 {
					t.Errorf("%v: input method anchor %v wide, caret span %v", align, got, want)
				}
			}
		}
	}
}

func TestTextAlignKeepsSourceAndSelections(t *testing.T) {
	st, _ := alignTestEditor(t, alignTestSource, AlignLeft, true)
	st.Select(4, 9)
	st.SetTextAlign(AlignCenter)
	if got := st.Buffer.Snapshot().String(); got != alignTestSource {
		t.Fatalf("the text changed to %q", got)
	}
	if sel := st.Selection(); sel.Anchor != 4 || sel.Head != 9 {
		t.Fatalf("selection = %d..%d, want 4..9", sel.Anchor, sel.Head)
	}
	if got := st.TextAlign(); got != AlignCenter {
		t.Fatalf("TextAlign() = %v after SetTextAlign(AlignCenter)", got)
	}
	st.SetTextAlign(AlignLeft)
	if got := st.TextAlign(); got != AlignLeft {
		t.Fatalf("TextAlign() = %v after SetTextAlign(AlignLeft)", got)
	}
}

func TestTextAlignLaysOutWithoutAFrame(t *testing.T) {
	st, _ := alignTestEditor(t, alignTestSource, AlignLeft, true)
	textX := st.ed.textX - st.ed.scrollX
	_, starts, _ := alignRows(&st.ed.view, 1)
	st.SetTextAlign(AlignRight)
	l := st.ed.view.line(1)
	row := &l.rows[0]
	if want := row.indent + wantAlignOffset(AlignRight, alignTestWidth, row.indent+row.width); math.Abs(float64(row.startX()-want)) > 0.01 {
		t.Fatalf("row 0 starts at %v after SetTextAlign, want %v", row.startX(), want)
	}
	r, ok := st.Rect(starts[0])
	if !ok {
		t.Fatal("no caret after SetTextAlign")
	}
	if want := textX + row.startX(); math.Abs(float64(r.X-want)) > 0.01 {
		t.Errorf("caret at %v after SetTextAlign, want the row's start %v", r.X, want)
	}
}

func TestTextAlignDragSelectsAlignedRows(t *testing.T) {
	for _, align := range []TextAlign{AlignCenter, AlignRight} {
		st, tt := alignTestEditor(t, alignTestSource, align, true)
		v := &st.ed.view
		textX := st.ed.textX - st.ed.scrollX
		l, starts, _ := alignRows(v, 1)
		if len(l.rows) < 2 {
			t.Fatalf("%v: the test line wrapped into %d rows, want more", align, len(l.rows))
		}
		target := starts[1] + len("fox ")
		wantFrom, wantTo := starts[0], target
		tt.Press(textX+0.5, rowMidY(st, 1, 0))
		tt.Move(textX+0.5, rowMidY(st, 1, 0))
		x := textX + v.xOf(l, &l.rows[1], l.toDisp(target-l.start, false))
		tt.Move(x+0.5, rowMidY(st, 1, 1))
		tt.Release(x+0.5, rowMidY(st, 1, 1))
		sel := st.Selection()
		if sel.Anchor != wantFrom || sel.Head != wantTo {
			t.Fatalf("%v: drag selected %d..%d, want %d..%d", align, sel.Anchor, sel.Head, wantFrom, wantTo)
		}
		tt.Frame()
		if lo, _ := paintedExtent(tt.Image(), int(rowMidY(st, 1, 0))); math.Abs(float64(float32(lo)-textX-l.rows[0].startX())) > 3 {
			t.Errorf("%v: the selection of the first row starts at %v, want the row's start %v", align, float32(lo)-textX, l.rows[0].startX())
		}
		if lo, hi := paintedExtent(tt.Image(), int(rowMidY(st, 1, 1))); lo < 0 {
			t.Errorf("%v: the second row is not selected", align)
		} else {
			if math.Abs(float64(float32(lo)-textX-l.rows[1].startX())) > 3 {
				t.Errorf("%v: the selection of the second row starts at %v, want the row's start %v", align, float32(lo)-textX, l.rows[1].startX())
			}
			if hi < int(x)-3 {
				t.Errorf("%v: the selection of the second row ends at %v, before the glyph at %v", align, hi, x)
			}
		}
	}
}

func TestTextAlignRightInsetDoesNotDoubleSubtractLeft(t *testing.T) {
	st0, tt0 := alignTestEditor(t, "abc", AlignRight, true)
	st0.SetLineStyles([]LineStyle{{Line: 0, Left: 0, Right: 10}})
	tt0.Frame()
	r0, ok0 := st0.Rect(len("abc"))
	if !ok0 {
		t.Fatal("missing caret for left=0")
	}

	st20, tt20 := alignTestEditor(t, "abc", AlignRight, true)
	st20.SetLineStyles([]LineStyle{{Line: 0, Left: 20, Right: 10}})
	tt20.Frame()
	r20, ok20 := st20.Rect(len("abc"))
	if !ok20 {
		t.Fatal("missing caret for left=20")
	}
	if math.Abs(float64(r20.X-r0.X)) > 0.5 {
		t.Fatalf("right-aligned caret moved with left inset: left=0 X=%v, left=20 X=%v", r0.X, r20.X)
	}
}
