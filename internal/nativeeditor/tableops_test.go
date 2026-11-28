package nativeeditor

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func tableWith(t *testing.T, raw string, op tableOp) (string, cellPosition, bool) {
	t.Helper()
	source, ok := readTableSource(raw)
	if !ok {
		t.Fatalf("%q is no table", raw)
	}
	edited, focus, ok := source.with(op)
	return edited.String(), focus, ok
}

func TestTableSourceReadsItsBytesBack(t *testing.T) {
	for _, raw := range []string{
		"| A | B |\n| --- | --- |\n| x | y |\n",
		"| A | B |\r\n| --- | --- |\r\n| x | y |\r\n",
		"| A | B |\r\n| --- | --- |\n| x | y |\r",
		"| A | B |\n| --- | --- |\n| x | y |",
		"a | b\n--- | ---\nx | y",
		"  | a | b |\n --- | ---\n x | y |\n",
		"| a \\| b | c |\n| --- | --- |\n| x | y |\n",
		"| `x|y` | z |\n| --- | --- |\n| 1 | 2 |\n",
		"| A |\n| --- |\n| x |\n| only |\n| p | q | r |\n",
		"| A | B |\n| :--- | ---: |\n|  |  |\n",
	} {
		source, ok := readTableSource(raw)
		if !ok {
			t.Fatalf("readTableSource(%q) read no table", raw)
		}
		if got := source.String(); got != raw {
			t.Errorf("table of %q reads back as %q", raw, got)
		}
	}
}

func TestTableRowCellsMatchTheParser(t *testing.T) {
	for _, row := range []string{
		"| a | b |",
		"a | b",
		"| a |",
		"| a",
		"a",
		"| a || b |",
		"||",
		"| |",
		"| a | b |   ",
		"a | b | c | d",
		"| a\\|b | c |",
		"| `x|y` | z |",
		"  | a | b |",
		"|a|b|",
	} {
		const columns = 3
		document := parser.New(parser.Default | parser.ExtDollarMath)
		source := "| h1 | h2 | h3 |\n| --- | --- | --- |\n" + row + "\n"
		document.Set(source)
		want := make([]string, 0, columns)
		found := false
		for i := range document.Len() {
			if document.Block(i).Kind != parser.Table {
				continue
			}
			found = true
			at, n := 0, 0
			for block := range document.Children(i) {
				if n == 1 {
					at = block
				}
				n++
			}
			if at == 0 {
				t.Fatalf("no data row parsed for %q", row)
			}
			for cell := range document.Children(at) {
				block := document.Block(cell)
				want = append(want, source[block.Start:block.End])
			}
		}
		if !found {
			t.Fatalf("no table parsed for row %q", row)
		}
		cells := tableRowCells(row)
		got := make([]string, 0, columns)
		for k := 0; k < columns; k++ {
			if k < len(cells) {
				got = append(got, row[cells[k][0]:cells[k][1]])
			} else {
				got = append(got, "")
			}
		}
		if len(got) > len(want) {
			got = got[:len(want)]
		}
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("cells of %q = %q, want the parser's %q", row, got, want)
		}
	}
}

func TestTableSourceInsertsRows(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		op    tableOp
		want  string
		focus cellPosition
		ok    bool
	}{
		{
			name:  "above the first data row",
			raw:   "| A | B |\n| --- | --- |\n| x | y |\n\nafter\n",
			op:    tableOp{kind: tableInsertRowAbove, row: 1},
			want:  "| A | B |\n| --- | --- |\n|  |  |\n| x | y |\n\nafter\n",
			focus: cellPosition{1, 0}, ok: true,
		},
		{
			name:  "above a later data row",
			raw:   "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n",
			op:    tableOp{kind: tableInsertRowAbove, row: 2},
			want:  "| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n| p | q |\n",
			focus: cellPosition{2, 0}, ok: true,
		},
		{
			name:  "below the header is the first data row",
			raw:   "| A | B |\n| --- | --- |\n| x | y |\n",
			op:    tableOp{kind: tableInsertRowBelow, row: 0},
			want:  "| A | B |\n| --- | --- |\n|  |  |\n| x | y |\n",
			focus: cellPosition{1, 0}, ok: true,
		},
		{
			name:  "below the last row at the end of the file",
			raw:   "| A | B |\n| --- | --- |\n| x | y |",
			op:    tableOp{kind: tableInsertRowBelow, row: 1},
			want:  "| A | B |\n| --- | --- |\n| x | y |\n|  |  |",
			focus: cellPosition{2, 0}, ok: true,
		},
		{
			name: "above the header keeps the single header row",
			raw:  "| A | B |\n| --- | --- |\n| x | y |\n",
			op:   tableOp{kind: tableInsertRowAbove, row: 0},
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, focus, ok := tableWith(t, tc.raw, tc.op)
			if ok != tc.ok {
				t.Fatalf("with(%+v) ok = %v, want %v", tc.op, ok, tc.ok)
			}
			if !ok {
				if got != tc.raw {
					t.Errorf("a refused edit rewrote the table as %q", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("with(%+v) = %q, want %q", tc.op, got, tc.want)
			}
			if focus != tc.focus {
				t.Errorf("with(%+v) focuses %v, want %v", tc.op, focus, tc.focus)
			}
		})
	}
}

func TestTableSourceDeletesRows(t *testing.T) {
	const raw = "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n"
	if got, want := mustTableWith(t, raw, tableOp{kind: tableDeleteRow, row: 1}), "| A | B |\n| --- | --- |\n| p | q |\n"; got != want {
		t.Errorf("a deleted first data row = %q, want %q", got, want)
	}
	if got, want := mustTableWith(t, raw, tableOp{kind: tableDeleteRow, row: 2}), "| A | B |\n| --- | --- |\n| x | y |\n"; got != want {
		t.Errorf("a deleted last data row = %q, want %q", got, want)
	}
	if _, _, ok := tableWith(t, raw, tableOp{kind: tableDeleteRow, row: 0}); ok {
		t.Error("the header row was deleted")
	}
	const single = "| A | B |\n| --- | --- |\n| x | y |"
	if got, want := mustTableWith(t, single, tableOp{kind: tableDeleteRow, row: 1}), "| A | B |\n| --- | --- |"; got != want {
		t.Errorf("a deleted only data row = %q, want %q", got, want)
	}
}

func TestTableSourceInsertsColumns(t *testing.T) {
	const raw = "| A | B |\n| --- | :---: |\n| x | y |\n"
	cases := []struct {
		name  string
		op    tableOp
		want  string
		focus cellPosition
	}{
		{
			name:  "left of the first column",
			op:    tableOp{kind: tableInsertColumnLeft, row: 0, col: 0},
			want:  "| | A | B |\n| --- | --- | :---: |\n| | x | y |\n",
			focus: cellPosition{0, 0},
		},
		{
			name:  "right of the last column",
			op:    tableOp{kind: tableInsertColumnRight, row: 1, col: 1},
			want:  "| A | B | |\n| --- | :---: | --- |\n| x | y | |\n",
			focus: cellPosition{1, 2},
		},
		{
			name:  "left of the aligned column",
			op:    tableOp{kind: tableInsertColumnLeft, row: 1, col: 1},
			want:  "| A | | B |\n| --- | --- | :---: |\n| x | | y |\n",
			focus: cellPosition{1, 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, focus, ok := tableWith(t, raw, tc.op)
			if !ok {
				t.Fatalf("with(%+v) refused", tc.op)
			}
			if got != tc.want {
				t.Errorf("with(%+v) = %q, want %q", tc.op, got, tc.want)
			}
			if focus != tc.focus {
				t.Errorf("with(%+v) focuses %v, want %v", tc.op, focus, tc.focus)
			}
		})
	}
}

func TestTableSourceDeletesColumns(t *testing.T) {
	const raw = "| A | B | C |\n| --- | :---: | ---: |\n| x | y | z |\n"
	if got, want := mustTableWith(t, raw, tableOp{kind: tableDeleteColumn, row: 0, col: 0}), "| B | C |\n| :---: | ---: |\n| y | z |\n"; got != want {
		t.Errorf("a deleted first column = %q, want %q", got, want)
	}
	if got, want := mustTableWith(t, raw, tableOp{kind: tableDeleteColumn, row: 1, col: 2}), "| A | B |\n| --- | :---: |\n| x | y |\n"; got != want {
		t.Errorf("a deleted last column = %q, want %q", got, want)
	}
	const one = "| A |\n| --- |\n| x |\n"
	if _, _, ok := tableWith(t, one, tableOp{kind: tableDeleteColumn, row: 0, col: 0}); ok {
		t.Error("the only column was deleted")
	}
}

func TestTableSourceAlignsColumns(t *testing.T) {
	const raw = "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n"
	for _, tc := range []struct {
		align parser.Align
		want  string
	}{
		{align: parser.AlignLeft, want: "| A | B | C |\n| --- | :--- | --- |\n| x | y | z |\n"},
		{align: parser.AlignCenter, want: "| A | B | C |\n| --- | :---: | --- |\n| x | y | z |\n"},
		{align: parser.AlignRight, want: "| A | B | C |\n| --- | ---: | --- |\n| x | y | z |\n"},
		{align: parser.AlignNone, want: "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n"},
	} {
		got, focus, ok := tableWith(t, raw, tableOp{kind: tableAlignColumn, row: 1, col: 1, align: tc.align})
		if !ok {
			t.Fatalf("aligning %v refused", tc.align)
		}
		if got != tc.want {
			t.Errorf("align %v = %q, want %q", tc.align, got, tc.want)
		}
		if focus != (cellPosition{1, 1}) {
			t.Errorf("align %v moved the caret to %v", tc.align, focus)
		}
	}
}

func TestTableSourceKeepsPipesItDoesNotSplit(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		op   tableOp
		want string
	}{
		{
			name: "escaped pipe stays in its cell",
			raw:  "| a \\| b | c |\n| --- | --- |\n| x | y |\n",
			op:   tableOp{kind: tableInsertColumnRight, row: 1, col: 1},
			want: "| a \\| b | c | |\n| --- | --- | --- |\n| x | y | |\n",
		},
		{
			name: "code span pipes split as the parser reads them",
			raw:  "| `x|y` | z | q |\n| --- | --- | --- | --- |\n| 1 | 2 | 3 | 4 |\n",
			op:   tableOp{kind: tableInsertColumnLeft, row: 1, col: 1},
			want: "| `x|| y` | z | q |\n| --- | --- | --- | --- | --- |\n| 1 | | 2 | 3 | 4 |\n",
		},
		{
			name: "rows without outer pipes",
			raw:  "a | b\n--- | ---\nx | y",
			op:   tableOp{kind: tableInsertRowBelow, row: 1},
			want: "a | b\n--- | ---\nx | y\n|  |  |",
		},
		{
			name: "a short row gains the columns it lacks",
			raw:  "| A | B | C |\n| --- | --- | --- |\n| only  |\n",
			op:   tableOp{kind: tableInsertColumnRight, row: 0, col: 2},
			want: "| A | B | C | |\n| --- | --- | --- | --- |\n| only  |  |  | |\n",
		},
		{
			name: "a long row keeps its extra cells",
			raw:  "| A | B |\n| --- | --- |\n| p | q | r |\n",
			op:   tableOp{kind: tableInsertColumnLeft, row: 1, col: 0},
			want: "| | A | B |\n| --- | --- | --- |\n| | p | q | r |\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustTableWith(t, tc.raw, tc.op); got != tc.want {
				t.Errorf("with(%+v) = %q, want %q", tc.op, got, tc.want)
			}
		})
	}
}

func TestTableSourceKeepsEachLinesOwnEnding(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		op   tableOp
		want string
	}{
		{
			name: "CRLF rows",
			raw:  "| A | B |\r\n| --- | --- |\r\n| x | y |\r\n",
			op:   tableOp{kind: tableInsertRowBelow, row: 1},
			want: "| A | B |\r\n| --- | --- |\r\n| x | y |\r\n|  |  |\r\n",
		},
		{
			name: "mixed endings keep their own",
			raw:  "| A | B |\r\n| --- | --- |\n| x | y |\r\n",
			op:   tableOp{kind: tableInsertRowBelow, row: 0},
			want: "| A | B |\r\n| --- | --- |\n|  |  |\r\n| x | y |\r\n",
		},
		{
			name: "a table at the end of a file without a newline",
			raw:  "| A | B |\n| --- | --- |\n| x | y |",
			op:   tableOp{kind: tableInsertRowBelow, row: 1},
			want: "| A | B |\n| --- | --- |\n| x | y |\n|  |  |",
		},
		{
			name: "a deleted last row leaves the file's newline",
			raw:  "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n\nafter\n",
			op:   tableOp{kind: tableDeleteRow, row: 2},
			want: "| A | B |\n| --- | --- |\n| x | y |\n\nafter\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustTableWith(t, tc.raw, tc.op); got != tc.want {
				t.Errorf("with(%+v) = %q, want %q", tc.op, got, tc.want)
			}
		})
	}
}

var savedWidths string

func saveWidths(into *string) func(string, string) {
	return func(key, value string) {
		if key == "table-widths" {
			*into = value
		}
	}
}

func mustTableWith(t *testing.T, raw string, op tableOp) string {
	t.Helper()
	got, _, ok := tableWith(t, raw, op)
	if !ok {
		t.Fatalf("with(%+v) refused", op)
	}
	return got
}

func laidOutTable(t *testing.T, src string, width, height int) (*Document, *ui.Tester, *readingTable) {
	t.Helper()
	return laidOutTableWith(t, src, width, height, func(*Document) {})
}

func laidOutTableWith(t *testing.T, src string, width, height int, prepare func(*Document)) (*Document, *ui.Tester, *readingTable) {
	t.Helper()
	document := New(src)
	prepare(document)
	tester := ui.NewTester(func(c *ui.Context) { document.View(c, false).Fill() }, width, height)
	tester.Frame()
	return document, tester, firstTable(t, document)
}

func TestTableStructureEditsUndoInOneStep(t *testing.T) {
	const src = "intro\n\n| A | B |\n| --- | --- |\n| x | y |\n\nafter\n"
	cases := []struct {
		name string
		op   tableOp
		want string
	}{
		{
			name: "insert row above",
			op:   tableOp{kind: tableInsertRowAbove, row: 1},
			want: "intro\n\n| A | B |\n| --- | --- |\n|  |  |\n| x | y |\n\nafter\n",
		},
		{
			name: "insert row below",
			op:   tableOp{kind: tableInsertRowBelow, row: 1},
			want: "intro\n\n| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n\nafter\n",
		},
		{
			name: "insert column left",
			op:   tableOp{kind: tableInsertColumnLeft, row: 0, col: 0},
			want: "intro\n\n| | A | B |\n| --- | --- | --- |\n| | x | y |\n\nafter\n",
		},
		{
			name: "insert column right",
			op:   tableOp{kind: tableInsertColumnRight, row: 1, col: 1},
			want: "intro\n\n| A | B | |\n| --- | --- | --- |\n| x | y | |\n\nafter\n",
		},
		{
			name: "delete row",
			op:   tableOp{kind: tableDeleteRow, row: 1},
			want: "intro\n\n| A | B |\n| --- | --- |\n\nafter\n",
		},
		{
			name: "delete column",
			op:   tableOp{kind: tableDeleteColumn, row: 0, col: 1},
			want: "intro\n\n| A |\n| --- |\n| x |\n\nafter\n",
		},
		{
			name: "align column",
			op:   tableOp{kind: tableAlignColumn, row: 0, col: 1, align: parser.AlignCenter},
			want: "intro\n\n| A | B |\n| --- | :---: |\n| x | y |\n\nafter\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, _, table := laidOutTable(t, src, 760, 420)
			if !table.editTable(document, tc.op) {
				t.Fatalf("with(%+v) refused", tc.op)
			}
			if got := document.Source(); got != tc.want {
				t.Fatalf("with(%+v) = %q, want %q", tc.op, got, tc.want)
			}
			if !document.Undo() || document.Source() != src {
				t.Fatalf("one undo = %q, want the table as it was %q", document.Source(), src)
			}
			if !document.Redo() || document.Source() != tc.want {
				t.Fatalf("one redo = %q, want %q", document.Source(), tc.want)
			}
			if document.Undo() && document.Undo() {
				t.Fatalf("the edit took more than one step of undo")
			}
			if got := document.Source(); got != src {
				t.Fatalf("a second undo = %q, want %q", got, src)
			}
		})
	}
}

func TestTableStructureEditsReachTheMarkdownView(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document, tester, table := laidOutTable(t, src, 760, 420)
	if !table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1}) {
		t.Fatal("the table refused a row")
	}
	tester.Frame()
	const want = "| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n"
	if got := document.state.Buffer.Snapshot().String(); got != want {
		t.Fatalf("the shared buffer holds %q, want %q", got, want)
	}
	if !document.SetMode(Markdown) {
		t.Fatal("the document did not switch to Markdown")
	}
	tester.Frame()
	if got := document.state.Buffer.Snapshot().String(); got != want {
		t.Fatalf("the Markdown view shows %q, want %q", got, want)
	}
	tester.Frame()
	if _, ok := tester.Find("Insert table row below row 1"); ok {
		t.Fatal("the Markdown view shows table handles")
	}
}

func TestTableStructureEditsRefuseReadOnlyStaleAndComposing(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	t.Run("read only", func(t *testing.T) {
		document := New(src)
		tester := ui.NewTester(func(c *ui.Context) { document.View(c, true).Fill() }, 760, 320)
		tester.Frame()
		table := firstTable(t, document)
		if table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1}) {
			t.Fatal("a read-only table took a structural edit")
		}
		if document.Source() != src {
			t.Fatalf("a read-only table wrote %q", document.Source())
		}
	})
	t.Run("stale layout", func(t *testing.T) {
		document, tester, table := laidOutTable(t, src, 760, 320)
		if err := document.Apply(document.Version(), len(src), len(src), "", "tail"); err != nil {
			t.Fatal(err)
		}
		if table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1}) {
			t.Fatal("a table laid out before the change took an edit")
		}
		if got, want := document.Source(), src+"tail"; got != want {
			t.Fatalf("a stale table wrote %q, want %q", got, want)
		}
		tester.Frame()
	})
	t.Run("composition", func(t *testing.T) {
		document, _, table := laidOutTable(t, src, 760, 320)
		child := tableCellDocument(t, document, 1, 0)
		table.composing = map[*Document]tableCell{child: table.spans[1][0]}
		if table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1}) {
			t.Fatal("a table composing took a structural edit")
		}
		if document.Source() != src {
			t.Fatalf("a composing table wrote %q", document.Source())
		}
		clear(table.composing)
		if !table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1}) {
			t.Fatal("the table refused an edit once the composition ended")
		}
	})
}

func TestTableStructureEditsMigrateColumnWidths(t *testing.T) {
	const src = "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n"
	document, tester, table := laidOutTableWith(t, src, 900, 420, func(document *Document) {
		document.SetReadingPreferences("", "", saveWidths(&savedWidths))
	})
	table.widths[0], table.widths[1], table.widths[2] = 260, 180, 220
	table.manual = true
	document.saveTables()

	if !table.editTable(document, tableOp{kind: tableInsertColumnLeft, row: 0, col: 1}) {
		t.Fatal("the table refused a column")
	}
	tester.Frame()
	want := []float32{260, portrait.Editor.TableColumnMin, 180, 220}
	if len(table.widths) != len(want) {
		t.Fatalf("widths = %v, want %v", table.widths, want)
	}
	for i := range want {
		if table.widths[i] != want[i] {
			t.Fatalf("widths after an inserted column = %v, want %v", table.widths, want)
		}
	}
	if len(table.minimum) != len(want) {
		t.Fatalf("minimum widths = %v, want %d of them", table.minimum, len(want))
	}
	if savedWidths == "" {
		t.Fatal("the widths were not saved")
	}
	var records map[string][]float32
	if err := json.Unmarshal([]byte(savedWidths), &records); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, widths := range records {
		if len(widths) == len(want) && widths[0] == 260 && widths[1] == portrait.Editor.TableColumnMin && widths[3] == 220 {
			found = true
		}
	}
	if !found {
		t.Fatalf("the saved widths %s do not carry the inserted column", savedWidths)
	}

	if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 1}) {
		t.Fatal("the table refused to delete the column")
	}
	tester.Frame()
	if got := table.widths; len(got) != 3 || got[0] != 260 || got[1] != 180 || got[2] != 220 {
		t.Fatalf("widths after the column was deleted = %v, want [260 180 220]", got)
	}
}

func TestTableStructureEditsKeepWidthsAcrossReopen(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	savedWidths = ""
	document, tester, table := laidOutTableWith(t, src, 760, 320, func(document *Document) {
		document.SetReadingPreferences("", "", saveWidths(&savedWidths))
	})
	table.widths[0], table.widths[1] = 260, 180
	table.manual = true
	document.saveTables()
	if !table.editTable(document, tableOp{kind: tableInsertColumnRight, row: 1, col: 1}) {
		t.Fatal("the table refused a column")
	}
	tester.Frame()
	if savedWidths == "" {
		t.Fatal("the inserted column was not saved")
	}

	reopened := New(document.Source())
	reopened.SetReadingPreferences("", savedWidths, nil)
	again := ui.NewTester(func(c *ui.Context) { reopened.View(c, false).Fill() }, 760, 320)
	again.Frame()
	restored := firstTable(t, reopened)
	if len(restored.widths) != 3 {
		t.Fatalf("reopened widths = %v, want three", restored.widths)
	}
	if restored.widths[0] != 260 || restored.widths[1] != 180 || !restored.manual {
		t.Fatalf("reopened widths = %v (manual %v), want the columns the user set kept", restored.widths, restored.manual)
	}
}

func TestTableStructureEditsFocusTheCellTheyMake(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	cases := []struct {
		name  string
		op    tableOp
		focus string
	}{
		{name: "row", op: tableOp{kind: tableInsertRowBelow, row: 1}, focus: "Table row 3 column 1"},
		{name: "column", op: tableOp{kind: tableInsertColumnLeft, row: 1, col: 1}, focus: "Table row 2 column 2"},
		{name: "deleted column", op: tableOp{kind: tableDeleteColumn, row: 1, col: 1}, focus: "Table row 2 column 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, tester, table := laidOutTable(t, src, 760, 420)
			if !table.editTable(document, tc.op) {
				t.Fatalf("with(%+v) refused", tc.op)
			}
			tester.Frame()
			tester.Frame()
			if !tester.Focused(tc.focus) {
				t.Fatalf("the edit did not give %q the keyboard", tc.focus)
			}
			if table.stale() {
				t.Fatalf("the edit left stale focus, selection or bounds: %+v", table)
			}
		})
	}
}

func (m *readingTable) stale() bool {
	return m.selected || m.dragging || m.focus != nil || m.hasTarget || m.hoverCol != -1 || m.dragCol != -1
}

func TestTableHandlesSelectRowsAndColumns(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n"
	document, tester, table := laidOutTable(t, src, 720, 420)
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()

	if err := tester.Click("Select table row 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if table.kind != tableRows || !table.chosen(cellPosition{1, 0}) || !table.chosen(cellPosition{1, 1}) {
		t.Fatalf("a row handle chose %v..%v of %v cells", table.anchor, table.head, table.kind)
	}
	if table.chosen(cellPosition{2, 0}) {
		t.Fatal("a row handle chose the row below it")
	}

	tester.ClickWith(ui.Shift, "Select table row 3")
	tester.Frame()
	if !table.chosen(cellPosition{2, 1}) {
		t.Fatalf("Shift-click on a second row handle chose %v..%v", table.anchor, table.head)
	}

	if err := tester.Click("Select table column 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if table.kind != tableCols || !table.chosen(cellPosition{0, 1}) || !table.chosen(cellPosition{2, 1}) {
		t.Fatalf("a column handle chose %v..%v", table.anchor, table.head)
	}
	tester.ClickWith(ui.Shift, "Select table column 1")
	tester.Frame()
	if !table.chosen(cellPosition{0, 0}) {
		t.Fatalf("Shift-click on a second column handle chose %v..%v", table.anchor, table.head)
	}
	if document.Source() != src {
		t.Fatalf("the handles edited the table: %q", document.Source())
	}
}

func TestTableInsertPinsAddRowsAndColumns(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document, tester, _ := laidOutTable(t, src, 720, 420)
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()

	if err := tester.Click("Insert table row below row 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n"; got != want {
		t.Fatalf("a row pin = %q, want %q", got, want)
	}
	if !tester.Focused("Table row 3 column 1") {
		t.Fatal("the added row did not take the keyboard")
	}

	if err := tester.Click("Insert table column right of column 1"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if got, want := document.Source(), "| A | | B |\n| --- | --- | --- |\n| x | | y |\n|  |  | |\n"; got != want {
		t.Fatalf("a column pin = %q, want %q", got, want)
	}
	if !tester.Focused("Table row 3 column 2") {
		t.Fatal("the added column did not take the keyboard")
	}
}

func TestTableMenuInsertsAlignsAndDeletes(t *testing.T) {
	cases := []struct {
		name   string
		row    int
		item   string
		want   string
		before string
	}{
		{
			name: "insert row above", row: 2, item: "Insert Row Above",
			want: "| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n| p | q |\n",
		},
		{
			name: "insert row below", row: 2, item: "Insert Row Below",
			want: "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n|  |  |\n",
		},
		{
			name: "insert column left", row: 1, item: "Insert Column Left",
			want: "| | A | B |\n| --- | --- | --- |\n| | x | y |\n| | p | q |\n",
		},
		{
			name: "insert column right", row: 1, item: "Insert Column Right",
			want: "| A | | B |\n| --- | --- | --- |\n| x | | y |\n| p | | q |\n",
		},
		{
			name: "align center", row: 1, item: "Align Center",
			want: "| A | B |\n| :---: | --- |\n| x | y |\n| p | q |\n",
		},
		{
			name: "align right", row: 1, item: "Align Right",
			want: "| A | B |\n| ---: | --- |\n| x | y |\n| p | q |\n",
		},
		{
			name: "delete row", row: 2, item: "Delete Row",
			want: "| A | B |\n| --- | --- |\n| x | y |\n",
		},
		{
			name: "delete column", row: 0, item: "Delete Column",
			want: "| B |\n| --- |\n| y |\n| q |\n",
		},
	}
	const src = "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, tester, _ := laidOutTable(t, src, 760, 560)
			label := "Table row " + strconv.Itoa(tc.row+1) + " column 1"
			cell, ok := tester.Find(label)
			if !ok {
				t.Fatalf("missing %q", label)
			}
			tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
			tester.Frame()
			if err := tester.Click(tc.item); err != nil {
				t.Fatalf("%s: %v", tc.item, err)
			}
			tester.Frame()
			if got := document.Source(); got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.item, got, tc.want)
			}
			if !document.Undo() || document.Source() != src {
				t.Fatalf("%s did not undo in one step: %q", tc.item, document.Source())
			}
		})
	}
}

func TestTableMenuClearsChosenCells(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n"
	document, tester, table := laidOutTable(t, src, 760, 560)
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	table.selectCells(cellPosition{1, 0}, cellPosition{2, 1})
	tester.Frame()
	if err := tester.Click("Clear Cells"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n|  |  |\n|  |  |\n"; got != want {
		t.Fatalf("cleared cells = %q, want %q", got, want)
	}
	if !document.Undo() || document.Source() != src {
		t.Fatalf("clearing did not undo in one step: %q", document.Source())
	}
}

func TestTableMenuGivesTheKeyboardBackToItsCell(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	_, tester, _ := laidOutTable(t, src, 760, 560)
	cell, ok := tester.Find("Table row 2 column 2")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	if _, ok := tester.Find("Insert Row Below"); !ok {
		t.Fatal("the cell menu did not open")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("Table row 2 column 2") {
		t.Fatal("Escape left the keyboard outside the cell whose menu closed")
	}
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	if err := tester.Click("Insert Row Below"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if !tester.Focused("Table row 3 column 1") {
		t.Fatal("a chosen item did not give the keyboard to the cell it made")
	}
}

func TestTableMenuMasksItsEditsInReadOnlyDocuments(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document := New(src)
	tester := ui.NewTester(func(c *ui.Context) { document.View(c, true).Fill() }, 760, 560)
	tester.Frame()
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	if _, ok := tester.Find("Insert Row Below"); !ok {
		t.Fatal("a read-only cell has no menu")
	}
	for _, item := range []string{"Insert Row Below", "Insert Column Right", "Align Center", "Delete Row", "Delete Column"} {
		if err := tester.Click(item); err != nil {
			t.Fatalf("%s: %v", item, err)
		}
		tester.Frame()
		tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
		tester.Frame()
	}
	if document.Source() != src {
		t.Fatalf("a read-only document's menu wrote %q", document.Source())
	}
}

func TestTableMenuLeavesComposingTablesAlone(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document, tester, table := laidOutTable(t, src, 760, 560)
	if err := tester.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tester.Command("selectAll")
	tester.Compose("ni", 2)
	tester.Frame()
	if len(table.composing) == 0 {
		t.Fatal("the cell is not composing")
	}
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	if _, ok := tester.Find("Insert Row Below"); ok {
		t.Fatal("a composing cell opened the table's structural menu")
	}
	for _, pin := range []string{"Insert table row below row 2", "Insert table column right of column 1"} {
		if err := tester.Click(pin); err == nil {
			tester.Frame()
			if strings.Contains(document.Source(), "\n|  |  |") || strings.Contains(document.Source(), "| |") {
				t.Fatalf("a composition let %q insert: %q", pin, document.Source())
			}
		}
	}
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n| ni | y |\n"; got != want {
		t.Fatalf("a composition lost its candidates: %q, want %q", got, want)
	}
	tester.Type("你")
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n| 你 | y |\n"; got != want {
		t.Fatalf("the committed composition = %q, want %q", got, want)
	}
}

func TestTableHandlesWorkInANarrowWindow(t *testing.T) {
	const src = "| 名称 | 说明 |\n| --- | --- |\n| 素白 | 与霜华适合阅读的一段较长的中文说明文字 |\n"
	_, tester, table := laidOutTable(t, src, 360, 480)
	cell, ok := tester.Find("Table row 2 column 2")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	for _, label := range []string{"Select table row 2", "Select table column 2", "Insert table row below row 2", "Insert table column right of column 2"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("a narrow window has no %q", label)
		}
	}
	if table.columns() != 2 {
		t.Fatalf("narrow table has %d columns", table.columns())
	}
}

func TestTableRowHandlesStayPinnedWhileColumnsScroll(t *testing.T) {
	const src = "| A | B | C | D | E | F | G | H |\n|---|---|---|---|---|---|---|---|\n| a | b | c | d | e | f | g | h |\n"
	_, tester, table := laidOutTable(t, src, 420, 320)
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tester.Move(cell.X+4, cell.Y+cell.H/2)
	tester.Frame()
	pinned, ok := tester.Find("Select table row 2")
	if !ok {
		t.Fatal("the scrolled table shows no row handle")
	}
	if table.scroll.MaxX <= 0 {
		t.Fatal("the wide table does not scroll sideways")
	}
	table.scroll.X = table.scroll.MaxX
	tester.Frame()
	tester.Frame()
	scrolled, ok := tester.Find("Select table row 2")
	if !ok {
		t.Fatal("the row handle left with the columns it scrolled")
	}
	if scrolled.X != pinned.X {
		t.Fatalf("the row handle moved from %v to %v as the columns scrolled", pinned.X, scrolled.X)
	}
}

func TestTableTabInTheLastCellAddsOneRowPerTab(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n\n\nafter\n"
	document, tester, _ := laidOutTable(t, src, 760, 420)
	if err := tester.Click("Table row 2 column 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n| x | y |\n|  |  |\n\n\nafter\n"; got != want {
		t.Fatalf("Tab in the last cell = %q, want %q", got, want)
	}
	if !tester.Focused("Table row 3 column 1") {
		t.Fatal("Tab in the last cell did not go to the row it added")
	}
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("Table row 3 column 2") {
		t.Fatal("Tab inside the added row did not move to its second cell")
	}
	if !document.Undo() || document.Source() != src {
		t.Fatalf("the added row did not undo in one step: %q", document.Source())
	}
}

func TestTableTabInTheLastCellLeavesReadOnlyTablesAlone(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document := New(src)
	tester := ui.NewTester(func(c *ui.Context) { document.View(c, true).Fill() }, 760, 420)
	tester.Frame()
	if err := tester.Click("Table row 2 column 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	if document.Source() != src {
		t.Fatalf("a read-only table added a row: %q", document.Source())
	}
}

func TestTableOpsOuterPipeLessTablesStayValidTables(t *testing.T) {
	const src = "A | B\n--- | ---\nx | y\n"
	cases := []struct {
		name string
		op   tableOp
		cols int
	}{
		{name: "insert column left at col 0", op: tableOp{kind: tableInsertColumnLeft, row: 0, col: 0}, cols: 3},
		{name: "delete first column", op: tableOp{kind: tableDeleteColumn, row: 0, col: 0}, cols: 1},
		{name: "delete last column", op: tableOp{kind: tableDeleteColumn, row: 0, col: 1}, cols: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, tester, table := laidOutTable(t, src, 760, 420)
			if !table.editTable(document, tc.op) {
				t.Fatalf("editTable(%+v) returned false", tc.op)
			}
			tester.Frame()
			if len(document.tables) == 0 {
				t.Fatalf("table disappeared after %s: %q", tc.name, document.Source())
			}
			got := firstTable(t, document)
			if got.columns() != tc.cols {
				t.Fatalf("columns = %d, want %d; source = %q", got.columns(), tc.cols, document.Source())
			}
		})
	}
}

func TestTableOpsIndentedBlockquoteAndShortRowDelete(t *testing.T) {
	t.Run("indented table insert row and delete col", func(t *testing.T) {
		const src = "  | A | B |\n  | --- | --- |\n  | x | y |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1, col: 0}) {
			t.Fatal("indented table Insert Row Below failed")
		}
		tester.Frame()
		if got, want := document.Source(), "  | A | B |\n  | --- | --- |\n  | x | y |\n  |  |  |\n"; got != want {
			t.Fatalf("indented insert row = %q, want %q", got, want)
		}
		table = firstTable(t, document)
		if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 0}) {
			t.Fatal("indented table Delete Column failed")
		}
		tester.Frame()
		if strings.Contains(document.Source(), "x") {
			t.Fatalf("deleted column value leaked into surviving column: %q", document.Source())
		}
	})
	t.Run("blockquote table insert row", func(t *testing.T) {
		const src = "> | A | B |\n> | --- | --- |\n> | x | y |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableInsertRowBelow, row: 1, col: 0}) {
			t.Fatal("blockquote table Insert Row Below failed")
		}
		tester.Frame()
		if got, want := document.Source(), "> | A | B |\n> | --- | --- |\n> | x | y |\n> |  |  |\n"; got != want {
			t.Fatalf("blockquote insert row = %q, want %q", got, want)
		}
	})
	t.Run("short single cell data row deletes cleanly", func(t *testing.T) {
		const src = "| A | B |\n| --- | --- |\nx\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 0}) {
			t.Fatal("delete col 0 failed on short row")
		}
		tester.Frame()
		if strings.Contains(document.Source(), "x") {
			t.Fatalf("deleted col 0 short row leaked x: %q", document.Source())
		}
	})
}

func TestTableOpsCJKSharedUTF8LeadByteEditsSucceed(t *testing.T) {
	t.Run("delete col with shared utf8 lead byte", func(t *testing.T) {
		const src = "| 你 | 他 |\n| --- | --- |\n| 你 | 他 |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 0}) {
			t.Fatal("delete col 0 failed on CJK table")
		}
		tester.Frame()
		if got, want := document.Source(), "| 他 |\n| --- |\n| 他 |\n"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("delete row with shared utf8 lead byte", func(t *testing.T) {
		const src = "| A |\n| --- |\n| 好 |\n| 如 |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableDeleteRow, row: 1, col: 0}) {
			t.Fatal("delete row 1 failed on CJK table")
		}
		tester.Frame()
		if got, want := document.Source(), "| A |\n| --- |\n| 如 |\n"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestTableUndoInsertColumnRestoresManualWidths(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	document := New(src)
	var saved string
	document.SetReadingPreferences("", "", func(key, value string) {
		if key == "table-widths" {
			saved = value
		}
	})
	tester := ui.NewTester(func(c *ui.Context) { document.View(c, false).Fill() }, 760, 420)
	tester.Frame()
	table := firstTable(t, document)
	table.widths[0], table.widths[1], table.manual = 260, 180, true
	document.saveTables()

	if !table.editTable(document, tableOp{kind: tableInsertColumnRight, row: 0, col: 1}) {
		t.Fatal("insert column failed")
	}
	tester.Frame()
	if !document.Undo() {
		t.Fatal("undo failed")
	}
	tester.Frame()
	restored := firstTable(t, document)
	if !restored.manual || restored.widths[0] != 260 || restored.widths[1] != 180 {
		t.Fatalf("after Undo widths = %v (manual %v), want [260 180] (true)", restored.widths, restored.manual)
	}
	if !strings.Contains(saved, "260") || !strings.Contains(saved, "180") {
		t.Fatalf("saved preferences after Undo = %q, want [260,180]", saved)
	}
}

func TestTableHandleAccessibilityReverseShiftClearAndMenus(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n"
	document, tester, table := laidOutTable(t, src, 760, 560)
	cell, ok := tester.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing cell")
	}
	tester.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()

	if _, ok := tester.Access("Select table row 2"); !ok {
		t.Fatal("row handle missing from accessibility tree")
	}
	if _, ok := tester.Access("Insert table row below row 2"); !ok {
		t.Fatal("row insert pin missing from accessibility tree")
	}

	if err := tester.Click("Select table row 2"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tester.Frame()
	if !table.selected {
		t.Fatal("right-clicking selected row cleared selection")
	}
	if err := tester.Click("Clear Cells"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got, want := document.Source(), "| A | B |\n| --- | --- |\n|  |  |\n| p | q |\n"; got != want {
		t.Fatalf("Clear Cells after right-clicking selected row = %q, want %q", got, want)
	}
	if !document.Undo() {
		t.Fatal("undo failed")
	}
	tester.Frame()

	fresh, freshTester, _ := laidOutTable(t, src, 760, 560)
	freshCell, _ := freshTester.Find("Table row 2 column 1")
	freshTester.Move(freshCell.X+freshCell.W/2, freshCell.Y+freshCell.H/2)
	freshTester.Frame()
	if err := freshTester.Click("Select table row 2"); err != nil {
		t.Fatal(err)
	}
	freshTester.Frame()
	freshTester.Key(0, ui.KeyBackspace)
	freshTester.Frame()
	if got, want := fresh.Source(), "| A | B |\n| --- | --- |\n|  |  |\n| p | q |\n"; got != want {
		t.Fatalf("Backspace after direct row handle click = %q, want %q", got, want)
	}

	_, shiftTester, shiftTable := laidOutTable(t, src, 760, 560)
	shiftCell, _ := shiftTester.Find("Table row 3 column 1")
	shiftTester.Move(shiftCell.X+shiftCell.W/2, shiftCell.Y+shiftCell.H/2)
	shiftTester.Frame()
	if err := shiftTester.Click("Select table row 3"); err != nil {
		t.Fatal(err)
	}
	shiftTester.Frame()
	if err := shiftTester.ClickWith(ui.Shift, "Select table row 1"); err != nil {
		t.Fatal(err)
	}
	shiftTester.Frame()
	if err := shiftTester.ClickWith(ui.Shift, "Select table row 2"); err != nil {
		t.Fatal(err)
	}
	shiftTester.Frame()
	top, bottom, _, _, ok := shiftTable.selectedBounds()
	if !ok || top != 1 || bottom != 2 {
		t.Fatalf("reverse Shift row selection bounds = (%d..%d, %v), want (1..2, true)", top, bottom, ok)
	}

	pin, ok := shiftTester.Find("Insert table row below row 2")
	if !ok {
		t.Fatal("missing row 2 insert pin")
	}
	shiftTester.RightClickAt(pin.X+pin.W/2, pin.Y+pin.H/2)
	shiftTester.Frame()
	if _, ok := shiftTester.Find("Insert Column Left"); !ok {
		t.Fatal("right-clicking insert pin did not open shared table menu")
	}

	_, imeTester, _ := laidOutTable(t, src, 760, 560)
	if err := imeTester.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	imeTester.Frame()
	imeTester.Compose("ni", 2)
	imeTester.Frame()
	grip, ok := imeTester.Find("Select table row 2")
	if !ok {
		t.Fatal("missing row grip during composition")
	}
	imeTester.RightClickAt(grip.X+grip.W/2, grip.Y+grip.H/2)
	imeTester.Frame()
	if _, ok := imeTester.Find("Insert Row Below"); ok {
		t.Fatal("right-clicking row grip opened structural menu during IME composition")
	}
}

func TestTableOpsCodexRoundTwoEdgeCases(t *testing.T) {
	t.Run("delete col next to empty header cell without outer pipes", func(t *testing.T) {
		for _, col := range []int{0, 2} {
			const src = "A | | C\n--- | --- | ---\nx | y | z\n"
			document, tester, table := laidOutTable(t, src, 760, 420)
			if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: col}) {
				t.Fatalf("delete col %d failed", col)
			}
			tester.Frame()
			if len(document.tables) != 1 || firstTable(t, document).columns() != 2 {
				t.Fatalf("delete col %d broke table: %q", col, document.Source())
			}
		}
	})

	t.Run("delete col preserves NBSP in surviving cell", func(t *testing.T) {
		const src = "A | B\n--- | ---\nx |\u00a0y\u00a0\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 0}) {
			t.Fatal("delete col 0 failed")
		}
		tester.Frame()
		if !strings.Contains(document.Source(), "\u00a0y\u00a0") {
			t.Fatalf("NBSP stripped from surviving cell: %q", document.Source())
		}
	})

	t.Run("delete last row preserves previous line CRLF in mixed endings", func(t *testing.T) {
		const src = "| A |\r\n| --- |\r\n| x |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		if !table.editTable(document, tableOp{kind: tableDeleteRow, row: 1, col: 0}) {
			t.Fatal("delete row 1 failed")
		}
		tester.Frame()
		if got, want := document.Source(), "| A |\r\n| --- |\r\n"; got != want {
			t.Fatalf("mixed EOL delete last row = %q, want %q", got, want)
		}
	})

	t.Run("clear first column and header row on outer-pipe-less table", func(t *testing.T) {
		const src = "A | B\n--- | ---\nx | y\n"
		for _, grip := range []string{"Select table column 1", "Select table row 1"} {
			document, tester, _ := laidOutTable(t, src, 760, 420)
			cell, _ := tester.Find("Table row 2 column 1")
			tester.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
			tester.Frame()
			if err := tester.Click(grip); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			tester.Key(0, ui.KeyBackspace)
			tester.Frame()
			if len(document.tables) != 1 || firstTable(t, document).columns() != 2 {
				t.Fatalf("%s + Backspace broke table: %q", grip, document.Source())
			}
			if !document.Undo() || document.Source() != src {
				t.Fatalf("%s + Backspace did not undo cleanly: %q", grip, document.Source())
			}
		}
	})

	t.Run("clicking handle during IME composition does not steal focus", func(t *testing.T) {
		const src = "| A | B |\n| --- | --- |\n| x | y |\n"
		document, tester, _ := laidOutTable(t, src, 760, 420)
		if err := tester.Click("Table row 2 column 2"); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		tester.Compose("ni", 2)
		tester.Frame()
		if err := tester.Click("Select table row 1"); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		tester.Type("你")
		tester.Frame()
		if strings.Contains(document.Source(), "你A") || strings.Contains(document.Source(), "A你") {
			t.Fatalf("IME commit leaked into header cell: %q", document.Source())
		}
		if !strings.Contains(document.Source(), "你") {
			t.Fatalf("IME commit lost: %q", document.Source())
		}
	})

	t.Run("multi-step undo restores initial widths even with identical re-inserted header", func(t *testing.T) {
		const src = "| A |  |\n| --- | --- |\n| x | y |\n"
		document, tester, table := laidOutTable(t, src, 760, 420)
		table.widths[0], table.widths[1], table.manual = 260, 180, true
		document.saveTables()

		if !table.editTable(document, tableOp{kind: tableDeleteColumn, row: 0, col: 1}) {
			t.Fatal("delete col 1 failed")
		}
		tester.Frame()
		table = firstTable(t, document)
		if !table.editTable(document, tableOp{kind: tableInsertColumnRight, row: 0, col: 0}) {
			t.Fatal("re-insert col failed")
		}
		tester.Frame()
		if !document.Undo() {
			t.Fatal("first undo failed")
		}
		tester.Frame()
		if !document.Undo() {
			t.Fatal("second undo failed")
		}
		tester.Frame()
		restored := firstTable(t, document)
		if !restored.manual || restored.widths[0] != 260 || restored.widths[1] != 180 {
			t.Fatalf("after 2x Undo widths = %v (manual %v), want [260 180] (true)", restored.widths, restored.manual)
		}
	})
}
