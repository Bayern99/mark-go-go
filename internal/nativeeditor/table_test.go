package nativeeditor

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

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
		if est := table.height(labels); est < 3*tableRowMin {
			t.Errorf("block height estimate %v, want room for the wrapped row", est)
		}
	}
}
