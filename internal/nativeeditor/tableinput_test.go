package nativeeditor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func firstTable(t *testing.T, d *Document) *readingTable {
	t.Helper()
	for _, table := range d.tables {
		return table
	}
	t.Fatal("no table was laid out")
	return nil
}

func tableCellDocument(t *testing.T, d *Document, row, col int) *Document {
	t.Helper()
	table := firstTable(t, d)
	if row >= len(table.cells) || col >= len(table.cells[row]) || table.cells[row][col] == nil {
		t.Fatalf("table has no cell %d,%d", row, col)
	}
	return table.cells[row][col]
}

func TestTableCellKeepsBlockSyntaxLiteral(t *testing.T) {
	for _, content := range []string{"# x", "- x", "> x", "1. x", "---"} {
		t.Run(content, func(t *testing.T) {
			d := New("| A | B |\n| --- | --- |\n| " + content + " | y |\n")
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
			tt.Frame()
			child := tableCellDocument(t, d, 1, 0)
			if got := child.preview.display; got != content {
				t.Errorf("cell shows %q, want the literal %q", got, content)
			}
			if styles := child.state.LineStyles(); len(styles) != 0 {
				t.Errorf("cell took %d block line styles", len(styles))
			}
			if blocks := child.state.Blocks(); len(blocks) != 0 {
				t.Errorf("cell took %d blocks", len(blocks))
			}
			info, ok := tt.Access("Table row 2 column 1")
			if !ok || info.Value != content {
				t.Errorf("cell reads %q, want the literal %q", info.Value, content)
			}
		})
	}
}

func TestTableCellKeepsInlineFormatting(t *testing.T) {
	const content = "**bold** *it* `code` [label](https://example.com)"
	d := New("| A | B |\n| --- | --- |\n| " + content + " | y |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	child := tableCellDocument(t, d, 1, 0)
	if got, want := child.preview.display, "bold it code label"; got != want {
		t.Errorf("cell shows %q, want %q", got, want)
	}
	for _, text := range []string{"bold", "it", "code", "label"} {
		if !strings.Contains(child.preview.display, text) {
			t.Errorf("cell shows %q, not the word %q", child.preview.display, text)
		}
	}
	info, ok := tt.Access("Table row 2 column 1")
	if !ok || info.Value != "bold it code label" {
		t.Errorf("cell reads %q, want its inline text alone", info.Value)
	}
}

func TestTableCellKeepsLiteralBlocksOnTypedLines(t *testing.T) {
	d := New("| A | B |\n| --- | --- |\n| x | y |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("one\n# two")
	tt.Frame()
	if want := "| A | B |\n| --- | --- |\n| one<br># two | y |\n"; d.Source() != want {
		t.Fatalf("typed lines = %q, want %q", d.Source(), want)
	}
	child := tableCellDocument(t, d, 1, 0)
	if got, want := child.preview.display, "one\n# two"; got != want {
		t.Errorf("cell shows %q, want %q", got, want)
	}
	if styles := child.state.LineStyles(); len(styles) != 0 {
		t.Errorf("a typed line took %d block line styles", len(styles))
	}
}

func TestTableCellContextMenuLeavesDocumentNotesOut(t *testing.T) {
	d := New("| A | B |\n| --- | --- |\n| x | y |\n\nafter\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	cell, ok := tt.Find("Table row 2 column 1")
	if !ok {
		t.Fatal("missing the body cell")
	}
	tt.RightClickAt(cell.X+cell.W/2, cell.Y+cell.H/2)
	tt.Frame()
	if _, ok := tt.Find("Insert Row Below"); !ok {
		t.Fatalf("the cell has no table menu: %v", tt.Texts())
	}
	if menu := tt.Menu(); len(menu) != 0 {
		t.Fatalf("the cell opened the editor's text menu instead: %v", menu)
	}
	for _, item := range []string{"Add note", "Cut", "Paste"} {
		if _, ok := tt.Find(item); ok {
			t.Fatalf("cell menu offers %q", item)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	r, ok := d.RectUTF16(strings.Index(d.Source(), "after"))
	if !ok {
		t.Fatal("missing the body caret")
	}
	tt.RightClickAt(r.X, r.Y+r.H/2)
	added := false
	for _, item := range tt.Menu() {
		added = added || item == "Add note"
	}
	if !added {
		t.Fatalf("the document itself lost its note action: %v", tt.Menu())
	}
}

func TestTableTabWalksCellsInReadingOrder(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 1 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Table row 1 column 2", "Table row 2 column 1", "Table row 2 column 2"} {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if !tt.Focused(want) {
			t.Fatalf("Tab did not focus %q", want)
		}
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Frame()
	if !tt.Focused("Table row 3 column 1") {
		t.Fatal("Tab in the last cell did not add a row to the table's end")
	}
	if want := src + "|  |  |\n"; d.Source() != want {
		t.Fatalf("Tab in the last cell = %q, want a row added in %q", d.Source(), want)
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("one undo did not take the added row back: %q", d.Source())
	}
	tt.Frame()
	for _, want := range []string{"Table row 2 column 2", "Table row 2 column 1", "Table row 1 column 2"} {
		tt.Key(ui.Shift, ui.KeyTab)
		tt.Frame()
		if !tt.Focused(want) {
			t.Fatalf("Shift+Tab did not focus %q", want)
		}
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Table row 1 column 1") {
		t.Fatal("Shift+Tab left the first cell")
	}
	tt.Frame()
	if d.Source() != src {
		t.Fatalf("Shift+Tab typed into the table: %q", d.Source())
	}
}

func TestTableArrowsCrossRowsAtWrappedCellEdges(t *testing.T) {
	wrapped := strings.Repeat("五行", 20)
	src := "| A | B |\n| --- | --- |\n| p | q |\n| mid | " + wrapped + " |\n| tail | r |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 700, 460)
	tt.Frame()
	table := firstTable(t, d)
	table.widths[0], table.widths[1] = 220, 150
	table.manual = true
	tt.Frame()
	tt.Frame()
	child := tableCellDocument(t, d, 2, 1)
	const cell = "Table row 3 column 2"
	if err := tt.Click(cell); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	child.state.Select(0, 0)
	tt.Frame()
	row, rows, ok := cellVisualRow(child)
	if !ok || row != 0 || rows < 2 {
		t.Fatalf("caret starts on row %d of %d (%v), want the first of a wrapped cell", row, rows, ok)
	}

	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if !tt.Focused(cell) {
		t.Fatal("Down left the cell from its first wrapped row")
	}
	if row, _, _ := cellVisualRow(child); row != 1 {
		t.Fatalf("Down moved to row %d of the wrapped cell, want 1", row)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if !tt.Focused(cell) {
		t.Fatal("Up left the cell before its first wrapped row")
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if !tt.Focused("Table row 2 column 2") {
		t.Fatal("Up at the first wrapped row did not focus the cell above")
	}

	if err := tt.Click(cell); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	child.state.Select(0, child.state.Buffer.Len())
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if !tt.Focused(cell) {
		t.Fatal("Down left the cell while its own selection was being collapsed")
	}
	child.state.Select(child.state.Buffer.Len(), child.state.Buffer.Len())
	tt.Frame()
	if row, rows, ok := cellVisualRow(child); !ok || row != rows-1 || rows < 2 {
		t.Fatalf("caret ends on row %d of %d (%v), want the last of a wrapped cell", row, rows, ok)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if !tt.Focused("Table row 4 column 2") {
		t.Fatal("Down at the last wrapped row did not focus the cell below")
	}
	if d.Source() != src {
		t.Fatalf("arrow keys edited the table: %q", d.Source())
	}
}

func TestTableCompositionKeepsKeysAndCrossCellUndo(t *testing.T) {
	const src = "| Name | Tag |\n| --- | --- |\n| cat | x |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Compose("n", 1)
	tt.Frame()
	tt.Compose("ni", 2)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if want := strings.Replace(src, "cat", "ni", 1); d.Source() != want {
		t.Fatalf("navigation during composition = %q, want the candidates kept in %q", d.Source(), want)
	}
	if !tt.Focused("Table row 2 column 1") {
		t.Fatal("navigation during composition moved the focus")
	}
	tt.Type("你")
	tt.Frame()
	want := strings.Replace(src, "cat", "你", 1)
	if d.Source() != want {
		t.Fatalf("commit = %q, want %q", d.Source(), want)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Table row 2 column 2") {
		t.Fatal("Tab after the commit did not move to the next cell")
	}
	tt.Command("undo")
	tt.Frame()
	if d.Source() != src {
		t.Fatalf("one undo after crossing cells = %q, want %q", d.Source(), src)
	}
}

func TestTableNavigationWaitsForCompositionToCommit(t *testing.T) {
	const src = "| Name | Tag |\n| --- | --- |\n| cat | x |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Compose("ni", 2)
	tt.Frame()
	child := tableCellDocument(t, d, 1, 0)
	table := firstTable(t, d)
	from := cellPosition{1, 0}
	if !child.state.Composing() {
		t.Fatal("the cell is not composing")
	}
	if table.cellInput(child, from, d, ui.Services{})(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab}) != true {
		t.Fatal("Tab during a composition was left to the editor")
	}
	if table.pending == nil || table.pending.from != from || table.pending.step != stepForward {
		t.Fatalf("Tab during a composition recorded %+v", table.pending)
	}
	if !tt.Focused("Table row 2 column 1") {
		t.Fatal("Tab during a composition moved the focus before the commit")
	}
	tt.Type("你")
	tt.Frame()
	tt.Frame()
	if want := strings.Replace(src, "cat", "你", 1); d.Source() != want {
		t.Fatalf("commit = %q, want %q", d.Source(), want)
	}
	if !tt.Focused("Table row 2 column 2") {
		t.Fatal("the move did not follow the commit")
	}
	if table.pending != nil {
		t.Fatal("the move survived the commit")
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("undo = %q, want %q", d.Source(), src)
	}
}

func TestTableSelectionsClearCellsInOneStep(t *testing.T) {
	const src = "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q | r |\n"
	cases := []struct {
		name   string
		src    string
		choose func(m *readingTable)
		key    ui.Key
		want   string
		inside []cellPosition
		out    []cellPosition
	}{
		{
			name:   "row backspace",
			choose: func(m *readingTable) { m.selectRows(1, 1) },
			key:    ui.KeyBackspace,
			want:   "| A | B | C |\n| --- | --- | --- |\n|  |  |  |\n| p | q | r |\n",
			inside: []cellPosition{{1, 0}, {1, 1}, {1, 2}},
			out:    []cellPosition{{0, 0}, {2, 0}},
		},
		{
			name:   "row backspace on CRLF lines",
			src:    "| A | B | C |\r\n| --- | --- | --- |\r\n| x | y | z |\r\n| p | q | r |\r\n",
			choose: func(m *readingTable) { m.selectRows(1, 1) },
			key:    ui.KeyBackspace,
			want:   "| A | B | C |\r\n| --- | --- | --- |\r\n|  |  |  |\r\n| p | q | r |\r\n",
			inside: []cellPosition{{1, 0}, {1, 2}},
			out:    []cellPosition{{0, 0}, {2, 0}},
		},
		{
			name:   "column delete",
			choose: func(m *readingTable) { m.selectColumns(1, 1) },
			key:    ui.KeyDelete,
			want:   "| A |  | C |\n| --- | --- | --- |\n| x |  | z |\n| p |  | r |\n",
			inside: []cellPosition{{0, 1}, {1, 1}, {2, 1}},
			out:    []cellPosition{{0, 0}, {1, 2}},
		},
		{
			name:   "rectangle delete",
			choose: func(m *readingTable) { m.selectCells(cellPosition{1, 0}, cellPosition{2, 1}) },
			key:    ui.KeyDelete,
			want:   "| A | B | C |\n| --- | --- | --- |\n|  |  | z |\n|  |  | r |\n",
			inside: []cellPosition{{1, 0}, {1, 1}, {2, 0}, {2, 1}},
			out:    []cellPosition{{0, 1}, {1, 2}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.src
			if src == "" {
				src = "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q | r |\n"
			}
			d := New(src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
			tt.Frame()
			if err := tt.Click("Table row 2 column 1"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			table := firstTable(t, d)
			tc.choose(table)
			tt.Frame()
			for _, pos := range tc.inside {
				if !table.chosen(pos) {
					t.Fatalf("selection misses cell %v", pos)
				}
			}
			for _, pos := range tc.out {
				if table.chosen(pos) {
					t.Fatalf("selection covers cell %v it should not", pos)
				}
			}
			tt.Key(0, tc.key)
			tt.Frame()
			if d.Source() != tc.want {
				t.Fatalf("cleared cells = %q, want %q", d.Source(), tc.want)
			}
			if !d.Undo() || d.Source() != src {
				t.Fatalf("clearing did not undo in one step: %q", d.Source())
			}
		})
	}
}

func TestTableSelectionCopiesWholeRowsAndColumns(t *testing.T) {
	d := New("| A | B |\n| --- | --- |\n| x | y |\n| p | q |\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	table := firstTable(t, d)
	table.selectColumns(1, 1)
	tt.Frame()
	tt.Command("copy")
	if got, want := tt.Clipboard(), "B\ny\nq"; got != want {
		t.Errorf("a column copies %q, want %q", got, want)
	}
	table.selectRows(1, 1)
	tt.Frame()
	tt.Command("copy")
	if got, want := tt.Clipboard(), "x\ty"; got != want {
		t.Errorf("a row copies %q, want %q", got, want)
	}
}

func TestTableClearSelectionLeavesReadOnlyDocumentsAlone(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| x | y |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, true).Fill() }, 760, 320)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	table := firstTable(t, d)
	table.selectRows(1, 1)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if d.Source() != src {
		t.Fatalf("read-only clearing wrote %q", d.Source())
	}
}

func TestTableRetiresRemovedCellEditors(t *testing.T) {
	const src = "| A | B |\n| --- | --- |\n| 1 | 2 |\n| 3 | 4 |\n| 5 | 6 |\n"
	d := New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	table := firstTable(t, d)
	if len(table.cells) != 4 {
		t.Fatalf("table has %d rows of cells, want 4", len(table.cells))
	}
	if err := tt.Click("Table row 4 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	last := cellPosition{3, 0}
	if _, ok := table.bounds[last]; !ok {
		t.Fatal("missing the bounds of the last row")
	}
	table.selectCells(last, cellPosition{3, 1})
	table.focus = table.cells[3][0]

	row := "| 5 | 6 |\n"
	at := strings.Index(src, row)
	if err := d.Apply(d.Version(), at, at+len(row), row, ""); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()

	table = firstTable(t, d)
	if len(table.cells) != 3 {
		t.Fatalf("cells kept %d rows after the row was removed, want 3", len(table.cells))
	}
	if table.focus != nil {
		t.Error("the removed row kept the keyboard focus")
	}
	if _, ok := table.bounds[last]; ok {
		t.Error("the removed row kept its bounds")
	}
	if table.selected {
		t.Errorf("the selection survived its only row, %v..%v", table.anchor, table.head)
	}
	if got := table.anchor; got.row >= len(table.cells) || got.col >= len(table.widths) {
		t.Errorf("the caret stayed at retired cell %v", got)
	}
	if len(table.composing) != 0 {
		t.Errorf("the removed row kept %d composing cells", len(table.composing))
	}
}
