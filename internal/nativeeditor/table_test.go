package nativeeditor

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestTableGridAssignsEachSharedEdgeOnce(t *testing.T) {
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			top, right, bottom, left := tableCellEdges(r, c)
			if !right || !bottom || top != (r == 0) || left != (c == 0) {
				t.Errorf("cell %d,%d edges = top %v right %v bottom %v left %v", r, c, top, right, bottom, left)
			}
		}
	}
}

func TestTableCellPaddingAndMetricsUseOuterColumnWidth(t *testing.T) {
	d := New("| A | B |\n|---|---|\n| x | y |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 300)
	tt.Frame()
	for _, table := range d.tables {
		field, ok := table.bounds[cellPosition{0, 0}]
		if !ok {
			t.Fatal("missing table field")
		}
		want := table.widths[0] - 2*portrait.Editor.TableCellPadX
		if field.W != want {
			t.Errorf("field width = %v, want outer column %v less horizontal padding = %v", field.W, table.widths[0], want)
		}
		text := "abcd"
		outer := textAdvance(text, portrait.Type.Small.Size) + 2*portrait.Editor.TableCellPadX
		if got := cellLines(text, outer); got != 1 {
			t.Errorf("text exactly filling the padded content width wraps to %d lines", got)
		}
		if got, want := table.rowHeight([]string{"A", "B"}), portrait.Editor.TableRowMin; got != want {
			t.Errorf("one-line row height = %v, want %v", got, want)
		}
		return
	}
	t.Fatal("no table was laid out")
}

func TestTableHeaderAndOuterCornersPaintPortraitTokens(t *testing.T) {
	d := New("| A | B |\n|---|---|\n| x | y |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 300)
	tt.Frame()
	tt.Frame()
	for _, table := range d.tables {
		field := table.bounds[cellPosition{0, 0}]
		x, y := field.X-portrait.Editor.TableCellPadX, field.Y-portrait.Editor.TableCellPadY
		img := tt.Image()
		assert := func(name string, x, y float32, want ui.Color) {
			t.Helper()
			r, g, b, _ := img.At(int(x), int(y)).RGBA()
			got := []int{int(r >> 8), int(g >> 8), int(b >> 8)}
			expected := []int{int(want.R), int(want.G), int(want.B)}
			for channel := range got {
				if diff := got[channel] - expected[channel]; diff < -3 || diff > 3 {
					t.Errorf("%s pixel = %v, want %v", name, got, expected)
					return
				}
			}
		}
		assert("rounded outer corner", x, y, portrait.Colors.Paper)
		fill, paper := portrait.Colors.FillStrong, portrait.Colors.Paper
		alpha := float32(fill.A) / 255
		header := ui.RGBA(
			uint8(float32(fill.R)*alpha+float32(paper.R)*(1-alpha)+0.5),
			uint8(float32(fill.G)*alpha+float32(paper.G)*(1-alpha)+0.5),
			uint8(float32(fill.B)*alpha+float32(paper.B)*(1-alpha)+0.5),
			1,
		)
		assert("table header", x+3, field.Y+field.H/2, header)
		return
	}
	t.Fatal("no table was laid out")
}

func TestTableAlignmentReachesSingleAndWrappedCJKCells(t *testing.T) {
	body := "中文 alignment 混合内容 " + strings.Repeat("五行", 12)
	src := "| 同 | 同 | 同 |\n| :--- | :---: | ---: |\n| " + body + " | " + body + " | " + body + " |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	for _, table := range d.tables {
		for col := range table.widths {
			table.widths[col] = 208
		}
		table.manual = true
	}
	tt.Frame()
	tt.Frame()
	for _, table := range d.tables {
		want := []editor.TextAlign{editor.AlignLeft, editor.AlignCenter, editor.AlignRight}
		var first, last [3]ui.Rect
		for col, align := range want {
			header, bodyCell := table.cells[0][col], table.cells[1][col]
			if header.state.TextAlign() != align || bodyCell.state.TextAlign() != align {
				t.Errorf("column %d alignment = header %v body %v, want %v", col, header.state.TextAlign(), bodyCell.state.TextAlign(), align)
			}
			first[col], _ = header.state.Rect(0)
			last[col], _ = bodyCell.state.Rect(len(body))
		}
		if !(first[0].X < first[1].X && first[1].X < first[2].X) {
			t.Errorf("single-line left / center / right starts = %v / %v / %v", first[0].X, first[1].X, first[2].X)
		}
		if !(last[0].X < last[1].X && last[1].X < last[2].X) {
			t.Errorf("last wrapped-row left / center / right ends = %v / %v / %v", last[0].X, last[1].X, last[2].X)
		}

		field := table.bounds[cellPosition{1, 2}]
		tt.ClickAt(field.X+last[2].X+0.1, field.Y+last[2].Y+last[2].H/2)
		if got := table.cells[1][2].state.Selection().Head; got != len(body) {
			t.Errorf("click at the aligned last row selected byte %d, want %d (field %v, caret %v, click %v,%v)", got, len(body), field, last[2], field.X+last[2].X+0.1, field.Y+last[2].Y+last[2].H/2)
		}

		center := table.cells[1][1]
		start, _ := center.state.Rect(0)
		field = table.bounds[cellPosition{1, 1}]
		tt.Press(field.X+start.X+0.1, field.Y+start.Y+start.H/2)
		tt.Move(field.X+last[1].X+0.1, field.Y+last[1].Y+last[1].H/2)
		tt.Release(field.X+last[1].X+0.1, field.Y+last[1].Y+last[1].H/2)
		if sel := center.state.Selection(); sel.Anchor != 0 || sel.Head != len(body) {
			t.Errorf("drag across aligned wrapped rows selected %d..%d, want 0..%d", sel.Anchor, sel.Head, len(body))
		}
		return
	}
	t.Fatal("no table was laid out")
}

func TestTableEdgeFadesFollowActualHorizontalOverflow(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		wide      bool
	}{
		{name: "ordinary", src: "| A | B |\n|---|---|\n| x | y |\n"},
		{name: "wide", src: "| A | B | C | D | E | F | G | H |\n|---|---|---|---|---|---|---|---|\n| a | b | c | d | e | f | g | h |\n", wide: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := New(tc.src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 480, 300)
			tt.Frame()
			tt.Frame()
			for _, table := range d.tables {
				if overflow := table.scroll.MaxX > 0; overflow != tc.wide {
					t.Fatalf("horizontal overflow = %v (MaxX %v), want %v", overflow, table.scroll.MaxX, tc.wide)
				}
				_, left := portrait.ScrollEdgeMaskX(&table.scroll, true)
				_, right := portrait.ScrollEdgeMaskX(&table.scroll, false)
				if left || right != tc.wide {
					t.Fatalf("at start left / right fades = %v / %v", left, right)
				}
				if tc.wide {
					table.scroll.X = table.scroll.MaxX / 2
					tt.Frame()
					_, left = portrait.ScrollEdgeMaskX(&table.scroll, true)
					_, right = portrait.ScrollEdgeMaskX(&table.scroll, false)
					if !left || !right {
						t.Fatalf("mid-scroll left / right fades = %v / %v", left, right)
					}
					table.scroll.X = table.scroll.MaxX
					tt.Frame()
					_, left = portrait.ScrollEdgeMaskX(&table.scroll, true)
					_, right = portrait.ScrollEdgeMaskX(&table.scroll, false)
					if !left || right {
						t.Fatalf("at end left / right fades = %v / %v", left, right)
					}
				}
				return
			}
			t.Fatal("no table was laid out")
		})
	}
}

func TestTableWidthsSurviveCellEditsAcrossReopen(t *testing.T) {
	const src = "| Name | Value |\n|---|---|\n| before | x |\n"
	for _, tc := range []struct{ name, from, to string }{
		{name: "body", from: "before", to: "after"},
		{name: "header", from: "Name", to: "Renamed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := New(src)
			var saved string
			d.SetReadingPreferences("", "", func(key, value string) {
				if key == "table-widths" {
					saved = value
				}
			})
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 300)
			tt.Frame()
			for _, table := range d.tables {
				table.widths[0], table.widths[1] = 260, 180
				table.manual = true
				d.saveTables()
			}
			if saved == "" {
				t.Fatal("column widths were not saved")
			}

			reopened := New(strings.Replace(src, tc.from, tc.to, 1))
			reopened.SetReadingPreferences("", saved, nil)
			reopenedTester := ui.NewTester(func(c *ui.Context) { reopened.View(c, false).Fill() }, 760, 300)
			reopenedTester.Frame()
			for _, table := range reopened.tables {
				if table.widths[0] != 260 || table.widths[1] != 180 || !table.manual {
					t.Fatalf("restored outer widths = %v, manual %v; want [260 180], true", table.widths, table.manual)
				}
				return
			}
			t.Fatal("no reopened table was laid out")
		})
	}
}

func TestLegacyTableWidthFallbackRequiresAUniqueContentSignature(t *testing.T) {
	model := readingTable{signature: "2:shared", preferenceKey: "v2:0:2:header", widths: []float32{112, 112}}
	preferences := readingPreferences{widths: map[string][]float32{
		"4:2:shared": {260, 180},
		"8:2:shared": {300, 200},
	}}
	preferences.restoreTable(&model)
	if model.manual || model.widths[0] != 112 || model.widths[1] != 112 {
		t.Fatalf("ambiguous duplicate legacy tables restored widths %v", model.widths)
	}
}

func TestMarkdownViewDoesNotInheritTableComposition(t *testing.T) {
	const src = "| Name | Tag |\n| --- | --- |\n| cat | x |"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Compose("n", 1)
	tt.Frame()
	tt.Compose("ni", 2)
	tt.Frame()
	d.SetMode(Markdown)
	tt.Frame()
	if want := strings.Replace(src, "cat", "ni", 1); d.Source() != want {
		t.Fatalf("source = %q, want %q", d.Source(), want)
	}
	if d.state.Composing() {
		t.Fatal("Markdown editor inherited marked text")
	}
	for _, table := range d.tables {
		if len(table.composing) != 0 {
			t.Fatal("table composition group survived mode switch")
		}
	}
}

func TestEscapeCellPreservesRawBytes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "a|b", want: `a\|b`},
		{in: `a\|b`, want: `a\|b`},
		{in: `a\\|b`, want: `a\\\|b`},
		{in: "one\r\ntwo\rthree\nfour", want: "one<br>two<br>three<br>four"},
		{in: "中文🙂|cell", want: `中文🙂\|cell`},
		{in: "raw\xff|x\xfe\r\ny\rz\n", want: "raw\xff\\|x\xfe<br>y<br>z<br>"},
	}
	for _, tc := range cases {
		if got := escapeCell(tc.in); got != tc.want {
			t.Errorf("escapeCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWideRunesMeasureAFullEm(t *testing.T) {
	size := float32(14)
	if got := textAdvance("五行", size); got != 2*size {
		t.Errorf("two Han runes advance %v, want %v", got, 2*size)
	}
	if latin, han := textAdvance("abcd", size), textAdvance("金木水火", size); han <= latin*1.5 {
		t.Errorf("four Han runes %v are not much wider than four Latin %v", han, latin)
	}
	if got := textAdvance("，。：", size); got != 3*size {
		t.Errorf("fullwidth punctuation advance %v, want %v", got, 3*size)
	}
}

func TestCJKColumnsAreWiderThanLatinColumnsOfTheSameLength(t *testing.T) {
	d := New("| 名称 | Name |\n| --- | --- |\n| 素白与霜华适合阅读 | abcdefghi |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 900, 400)
	tt.Frame()
	for _, table := range d.tables {
		if table.minimum[0] <= table.minimum[1] {
			t.Fatalf("CJK column minimum %v is not wider than the Latin one %v", table.minimum[0], table.minimum[1])
		}
		return
	}
	t.Fatal("no table was laid out")
}

func TestCJKTableRowsGrowToTheirWrappedLines(t *testing.T) {
	long := strings.Repeat("中文长句应自动换行而不被裁切", 4)
	src := "| 五行 | 说明 |\n| --- | --- |\n| 木 | " + long + " |\n| 火 | 短 |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 720, 600)
	tt.Frame()
	tt.Frame()
	cell, ok := tt.Access("Table row 2 column 2")
	if !ok {
		t.Fatal("missing the wrapped cell")
	}
	next, ok := tt.Access("Table row 3 column 2")
	if !ok {
		t.Fatal("missing the row below")
	}
	if lines := cell.Bounds.H / portrait.Type.Small.Leading; lines < 2.5 {
		t.Fatalf("wrapped cell is %v tall (%.1f lines), want the three or more lines it wraps to", cell.Bounds.H, lines)
	}
	if cell.Bounds.Y+cell.Bounds.H > next.Bounds.Y {
		t.Fatalf("wrapped cell %v runs into the row below at %v", cell.Bounds, next.Bounds)
	}
	short, _ := tt.Access("Table row 3 column 1")
	if short.Bounds.H > 2*portrait.Type.Small.Leading {
		t.Errorf("a one-line cell is %v tall, want one line", short.Bounds.H)
	}
	for _, table := range d.tables {
		labels := [][]string{{"五行", "说明"}, {"木", long}, {"火", "短"}}
		if est := table.height(labels); est < 3*portrait.Editor.TableRowMin {
			t.Errorf("block height estimate %v, want room for the wrapped row", est)
		}
	}
}
