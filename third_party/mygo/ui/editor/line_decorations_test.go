package editor

import (
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func decorationTestState(source string) *State {
	return &State{
		Buffer: NewBuffer(source), SoftWrap: true,
		LineNumbers: LineNumbersOff, NoFolding: true, NoActiveLine: true,
		NoScrollbars: true, NoScrollPastEnd: true,
		ReadingColumn: ReadingColumn{Width: 160},
		Font:          ui.Font{Size: 16}, LineHeight: 1.5,
		Theme: &Theme{Background: ui.Hex("#ffffff"), Text: ui.Hex("#000000")},
	}
}

func rgba(c ui.Color) color.RGBA { return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

func TestLineBackgroundJoinsRoundedLinesInsideReadingColumn(t *testing.T) {
	st := decorationTestState("\n\n")
	fill, rim := ui.Hex("#f6f6f8"), ui.Hex("#345678")
	st.LineBackgrounds = []LineBackground{{
		First: 0, Last: 1, Color: fill, Radius: 6,
		Stroke: 2, StrokeColor: rim, ReadingColumn: true,
	}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, st).Fill() }, 320, 120)
	tt.Frame()
	caret, ok := st.Rect(0)
	if !ok {
		t.Fatal("first line has no geometry")
	}
	x, y, h := caret.X, caret.Y, caret.H
	pixel := func(x, y float32) color.RGBA { return tt.Image().RGBAAt(int(x), int(y)) }

	if got, want := pixel(x+4, y+h), rgba(fill); got != want {
		t.Errorf("internal edge at the rounded corner = %v, want one continuous fill %v", got, want)
	}
	if got, want := pixel(x+80, y), rgba(rim); got != want {
		t.Errorf("top border = %v, want %v", got, want)
	}
	if got, want := pixel(x-1, y+h/2), rgba(st.Theme.Background); got != want {
		t.Errorf("left of reading column = %v, want paper %v", got, want)
	}
	if got, want := pixel(x+161, y+h/2), rgba(st.Theme.Background); got != want {
		t.Errorf("right of reading column = %v, want paper %v", got, want)
	}
	if got, want := pixel(x, y), rgba(st.Theme.Background); got != want {
		t.Errorf("outside rounded top-left corner = %v, want paper %v", got, want)
	}
}

func TestPlainLineBackgroundKeepsFullWidthGeometry(t *testing.T) {
	st := decorationTestState("plain")
	st.LineBackgrounds = []LineBackground{{First: 0, Last: 0, Color: ui.Hex("#f6f6f8")}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, st).Fill() }, 320, 80)
	tt.Frame()
	caret, _ := st.Rect(0)
	// A zero-options background still fills the old full text rectangle,
	// even when the editor centers a narrower reading column.
	if got, want := tt.Image().RGBAAt(int(caret.X-4), int(caret.Y+caret.H/2)), rgba(st.LineBackgrounds[0].Color); got != want {
		t.Fatalf("plain background outside reading column = %v, want %v", got, want)
	}
}

func TestLineStyleInsetsWrapCaretAndPointerWithoutChangingOffsets(t *testing.T) {
	const source = "ab甲乙cd efgh ijkl mnop qrst uvwx yz\nplain"
	st := decorationTestState(source)
	st.SetLineStyles([]LineStyle{{Line: 0, Left: 24, Right: 16}})
	tt := ui.NewTester(func(c *ui.Context) { View(c, st).Fill().AutoFocus() }, 320, 240)
	tt.Frame()

	line := st.ed.view.line(0)
	if len(line.rows) < 2 {
		t.Fatal("inset test line did not wrap")
	}
	if got := line.owner.wrapWidth; got != 120 {
		t.Errorf("wrapped width = %v, want 160 - 24 - 16 = 120", got)
	}
	for i, row := range line.rows {
		if row.indent != 24 {
			t.Errorf("row %d indent = %v, want 24", i, row.indent)
		}
	}
	plainAt := strings.Index(source, "plain")
	plain, _ := st.Rect(plainAt)
	start, _ := st.Rect(0)
	if start.X != plain.X+24 {
		t.Errorf("inset first caret x = %v, plain x = %v; want a 24 DIP inset", start.X, plain.X)
	}

	// 乙 starts at byte 5 and ends at byte 8. Its source bytes and selection
	// offsets must not move merely because its display is inset and wrapped.
	const at, after = 5, 8
	r, ok := st.Rect(at)
	if !ok {
		t.Fatal("missing inset UTF-8 caret")
	}
	tt.ClickAt(r.X+.1, r.Y+r.H/2)
	if got := st.Selection().Head; got != at {
		t.Fatalf("click selected byte %d, want %d", got, at)
	}
	tt.Key(0, ui.KeyRight)
	if got := st.Selection().Head; got != after {
		t.Fatalf("Right moved to byte %d, want %d", got, after)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Key(ui.Shift, ui.KeyRight)
	sel := st.Selection()
	if sel.Anchor != at || sel.Head != after || source[sel.Anchor:sel.Head] != "乙" {
		t.Fatalf("inset selection = %d..%d %q, want %d..%d 乙", sel.Anchor, sel.Head, source[sel.Anchor:sel.Head], at, after)
	}

	secondRowOffset := line.start + line.fromDisp(line.rows[1].from)
	second, _ := st.Rect(secondRowOffset)
	if second.X != start.X {
		t.Errorf("second wrapped row x = %v, first row x = %v", second.X, start.X)
	}
	tt.ClickAt(second.X+.1, second.Y+second.H/2)
	if got := st.Selection().Head; got != secondRowOffset {
		t.Errorf("click in second inset row selected byte %d, want %d", got, secondRowOffset)
	}
	if st.Buffer.Snapshot().String() != source {
		t.Fatal("layout, carets and selections changed the source bytes")
	}
}
