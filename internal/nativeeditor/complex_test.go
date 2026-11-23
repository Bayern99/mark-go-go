package nativeeditor_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
)

func TestTableCellEditsPreserveOtherCellsAndUndo(t *testing.T) {
	const src = "before\r\n\r\n| Name  | Tag |\r\n| --- | --- |\n| 猫 | a\\|b |\r\n\r\nafter"
	d := nativeeditor.New(src)
	testr := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 760, 420)
	testr.Frame()
	if err := testr.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	testr.Command("selectAll")
	testr.Type("雪")
	testr.Frame()
	if want := "before\r\n\r\n| Name  | Tag |\r\n| --- | --- |\n| 雪 | a\\|b |\r\n\r\nafter"; d.Source() != want {
		t.Fatalf("cell edit produced %q", d.Source())
	}
	if !d.Undo() || d.Source() != src {
		t.Fatalf("Undo produced %q", d.Source())
	}
}

func TestTableCellEditsKeepLineBreaksAndEscapedPipeCaret(t *testing.T) {
	const multiline = "| Name | Tag |\n| --- | --- |\n| cat | x |\n\nafter"
	d := nativeeditor.New(multiline)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("one\ntwo")
	tt.Frame()
	want := "| Name | Tag |\n| --- | --- |\n| one<br>two | x |\n\nafter"
	if d.Source() != want {
		t.Fatalf("multiline cell produced %q", d.Source())
	}
	if !d.Undo() || d.Source() != multiline {
		t.Fatal("multiline cell edit did not undo in one step")
	}

	const piped = "| Name | Tag |\n| --- | --- |\n| ab | x |\n\nafter"
	d = nativeeditor.New(piped)
	tt = ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("a|b")
	tt.Frame()
	if want := "| Name | Tag |\n| --- | --- |\n| a\\|b | x |\n\nafter"; d.Source() != want {
		t.Fatalf("inserting an escaped pipe produced %q", d.Source())
	}
	if a, z := d.SelectionUTF16(); a != 35 || z != 35 {
		t.Fatalf("escaped pipe caret = %d..%d", a, z)
	}
	tt.Frame()
	tt.Type("c")
	tt.Frame()
	if want := "| Name | Tag |\n| --- | --- |\n| a\\|bc | x |\n\nafter"; d.Source() != want {
		t.Fatalf("typing after an escaped pipe produced %q", d.Source())
	}
	if a, z := d.SelectionUTF16(); a != 36 || z != 36 {
		t.Fatalf("caret after typing past an escaped pipe = %d..%d", a, z)
	}
}

func TestTableSelectionAndPairedColumnResize(t *testing.T) {
	d := nativeeditor.New("| A | B | C |\n|---|---|---|\n| x | y | z |\n\nafter")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 780, 300)
	tt.Frame()
	bounds := func(label string) ui.Rect {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		return r
	}
	first, second, third := bounds("Table row 1 column 1"), bounds("Table row 1 column 2"), bounds("Table row 1 column 3")
	handle := bounds("Resize table column 1")
	x, y := handle.X+handle.W/2, handle.Y+handle.H/2
	tt.Press(x, y)
	tt.Move(x+32, y)
	tt.Release(x+32, y)
	if a, b, c := bounds("Table row 1 column 1"), bounds("Table row 1 column 2"), bounds("Table row 1 column 3"); a.W != first.W+32 || b.W != second.W-32 || c != third {
		t.Fatalf("column sizes after drag: %v %v %v", a, b, c)
	}
	a, b := bounds("Table row 1 column 1"), bounds("Table row 2 column 2")
	tt.Press(a.X+10, a.Y+10)
	tt.Move(b.X+10, b.Y+10)
	tt.Release(b.X+10, b.Y+10)
	tt.Command("copy")
	if tt.Clipboard() != "A\tB\nx\ty" {
		t.Fatalf("copied table cells %q", tt.Clipboard())
	}
}

func TestTableColumnResizeFromBodyCellBoundary(t *testing.T) {
	const src = "| A | B | C |\n|---|---|---|\n| x | y | z |\n\nafter"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 780, 300)
	tt.Frame()
	bounds := func(label string) ui.Rect {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		return r
	}
	first, second, third := bounds("Table row 1 column 1"), bounds("Table row 1 column 2"), bounds("Table row 1 column 3")
	handle := bounds("Resize table column 1")
	x := handle.X + handle.W/2
	body := bounds("Table row 2 column 1")
	y := body.Y + body.H/2

	tt.Move(x, y)
	img := tt.Image()
	r, g, b, _ := img.At(int(x), int(y)).RGBA()
	if dr, dg, db := int(r>>8)-47, int(g>>8)-111, int(b>>8)-98; dr*dr+dg*dg+db*db > 3*8*8 {
		t.Fatalf("no accent guide at the boundary: pixel %x %x %x", r, g, b)
	}

	tt.Press(x, y)
	tt.Move(x+32, y)
	tt.Release(x+32, y)
	if a, b, c := bounds("Table row 1 column 1"), bounds("Table row 1 column 2"), bounds("Table row 1 column 3"); a.W != first.W+32 || b.W != second.W-32 || c != third {
		t.Fatalf("column sizes after body drag: %v %v %v", a, b, c)
	}
	if d.Source() != src {
		t.Fatalf("resize changed the source %q", d.Source())
	}
}

func TestTableBoundaryClicksAndCanceledDragKeepAutomaticWidths(t *testing.T) {
	d := nativeeditor.New("| A | B |\n|---|---|\n| x | y |\n\nafter")
	var saved string
	d.SetReadingPreferences("", "", func(key, value string) {
		if key == "table-widths" {
			saved = value
		}
	})
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 490, 300)
	tt.Frame()
	const cell = "Table row 2 column 2"
	before, ok := tt.Find(cell)
	if !ok {
		t.Fatal("missing body cell")
	}
	handle, ok := tt.Find("Resize table column 1")
	if !ok {
		t.Fatal("missing column boundary")
	}
	x, y := handle.X+handle.W/2, before.Y+before.H/2
	tt.Press(x, y)
	tt.Release(x, y)

	tt.RightClickAt(before.X+1, y)
	if len(tt.Menu()) == 0 || !tt.Focused(cell) {
		t.Fatalf("right click near the column boundary missed the cell: %v", tt.Menu())
	}
	tt.CloseMenu()

	tt.Press(x, y)
	tt.Move(x+32, y)
	if during, _ := tt.Find(cell); during.W != before.W-32 {
		t.Fatalf("column did not resize during drag: %v", during)
	}
	tt.CancelPress()
	if after, _ := tt.Find(cell); after.W != before.W || saved != "" {
		t.Fatalf("clicks or canceled drag committed widths: %v, saved %q", after, saved)
	}

	tt.SetSize(750, 300)
	if wider, _ := tt.Find(cell); wider.W <= before.W {
		t.Fatalf("column did not grow with the viewport: %v, before %v", wider, before)
	}
}

func TestTableColumnResizeCoversWrappedRows(t *testing.T) {
	const src = "| A | B |\n|---|---|\n| a body cell whose text wraps inside the column instead of fitting on one line | z |\n| p | q |\n\nafter"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 780, 300)
	tt.Frame()
	bounds := func(label string) ui.Rect {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		return r
	}
	first, second := bounds("Table row 1 column 1"), bounds("Table row 1 column 2")
	handle := bounds("Resize table column 1")
	x := handle.X + handle.W/2
	top, bottom := bounds("Table row 1 column 1"), bounds("Table row 3 column 1")
	if handle.Y > top.Y || handle.Y+handle.H < bottom.Y+bottom.H {
		t.Fatalf("resize zone %v does not span the measured table %v..%v", handle, top, bottom)
	}
	wrapped := bounds("Table row 2 column 1")
	y := wrapped.Y + wrapped.H - 2
	tt.Move(x, y)
	img := tt.Image()
	accent := func(x, y float32) bool {
		r, g, b, _ := img.At(int(x), int(y)).RGBA()
		dr, dg, db := int(r>>8)-47, int(g>>8)-111, int(b>>8)-98
		return dr*dr+dg*dg+db*db <= 3*8*8
	}
	if !accent(x, y) {
		t.Fatal("no accent guide at the wrapped row boundary")
	}
	for _, gy := range []float32{top.Y + 1, bottom.Y + bottom.H - 1} {
		if !accent(x, gy) {
			t.Fatalf("guide stops short of the measured table extent at y %v", gy)
		}
	}
	tt.Press(x, y)
	tt.Move(x+32, y)
	tt.Release(x+32, y)
	if a, b := bounds("Table row 1 column 1"), bounds("Table row 1 column 2"); a.W != first.W+32 || b.W != second.W-32 {
		t.Fatalf("column sizes after wrapped-row drag: %v %v", a, b)
	}
	if d.Source() != src {
		t.Fatalf("resize changed the source %q", d.Source())
	}
}

func TestTableColumnResizeInScrolledWideTable(t *testing.T) {
	const src = "Intro\n\n| A | B | C | D | E | F | G | H |\n|---|---|---|---|---|---|---|---|\n| a | b | c | d | e | f | g | h |\n\nAfter.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 480, 400)
	tt.Frame()
	bounds := func(label string) ui.Rect {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal(label)
		}
		return r
	}
	cell := bounds("Table row 2 column 1")
	tt.Wheel(cell.X+30, cell.Y+10, 380, 0)
	tt.Advance(time.Second)
	tt.Frame()
	fifth, sixth := bounds("Table row 1 column 5"), bounds("Table row 1 column 6")
	before4, handle, before6 := bounds("Resize table column 4"), bounds("Resize table column 5"), bounds("Resize table column 6")
	x := handle.X + handle.W/2
	body := bounds("Table row 2 column 5")
	y := body.Y + body.H/2
	tt.Press(x, y)
	tt.Move(x+32, y)
	tt.Release(x+32, y)
	b, c := bounds("Table row 1 column 5"), bounds("Table row 1 column 6")
	after4, after6 := bounds("Resize table column 4"), bounds("Resize table column 6")
	if b.W != fifth.W+32 || c.W != sixth.W-32 || after4.X != before4.X || after6.X != before6.X {
		t.Fatalf("scrolled column sizes after drag: %v %v, boundaries %v %v", b, c, after4, after6)
	}
	if d.Source() != src {
		t.Fatalf("resize changed the source %q", d.Source())
	}
}

func TestSpecialBlockRenderingAndInPlaceEdit(t *testing.T) {
	src := "# Intro\r\n\r\n$$\r\nE=mc^2\r\n$$\n\n```mermaid\ngraph TD\n A[Start] --> B[Done]\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 800, 720)
	tt.Frame()
	for _, label := range []string{"Typeset equation", "Mermaid diagram"} {
		if area, ok := tt.Find(label); !ok || area.W < 20 || area.H < 12 {
			t.Fatalf("%s %v %v", label, area, ok)
		}
	}
	if err := tt.Click("Edit math"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("$$\np = 2\n$$")
	tt.Key(ui.Cmd, ui.KeyEnter)
	tt.Frame()
	want := "# Intro\r\n\r\n$$\r\np = 2\r\n$$\n\n```mermaid\ngraph TD\n A[Start] --> B[Done]\n```\n"
	if d.Source() != want {
		t.Fatalf("edited equation %q", d.Source())
	}
	if !d.Undo() || d.Source() != src {
		t.Fatal("could not undo equation edit")
	}
	tt.Frame()
	if err := tt.Click("Edit math"); err != nil {
		t.Fatal(err)
	}
	tt.Type("ignored")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if d.Source() != src {
		t.Fatal("cancel wrote the draft")
	}
}

func TestFootnotesReturnToTheChosenReferenceAndFoldHeading(t *testing.T) {
	const src = "# First\n\ntext[^one] again[^one]\n\n[^one]: footnote body\n\n# Next\n\nlast\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 640)
	tt.Frame()
	if err := tt.Click("Read footnote one, reference 2"); err != nil {
		t.Fatal(err)
	}
	if a, _ := d.SelectionUTF16(); a != 33 {
		t.Fatalf("jumped to %d", a)
	}
	if err := tt.Click("Return to reference one"); err != nil {
		t.Fatal(err)
	}
	if a, _ := d.SelectionUTF16(); a != 25 {
		t.Fatalf("returned to %d", a)
	}
	if err := tt.Click("Fold First"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Return to reference one"); ok {
		t.Fatal("fold kept footnote on screen")
	}
	if err := tt.Click("Expand First"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Return to reference one"); !ok || d.Source() != src {
		t.Fatal("expand did not restore section")
	}
}

func TestNotesKeepAttributesAndRefuseStaleDraft(t *testing.T) {
	const src = "Read.\r\n\r\n<!-- note id=\"7\": Old -->\n\nEnd."
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 740, 420)
	tt.Frame()
	if err := tt.Click("Edit note"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("A --> B")
	tt.Key(ui.Cmd, ui.KeyEnter)
	want := "Read.\r\n\r\n<!-- note id=\"7\": A --\\> B -->\n\nEnd."
	if d.Source() != want {
		t.Fatalf("edited note %q", d.Source())
	}
	if err := tt.Click("Edit note"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("stale")
	if err := d.Apply(d.Version(), 0, 4, "Read", "Seen"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEnter)
	if _, ok := tt.Find(nativeeditor.ErrRevision.Error()); !ok || d.Source() != "Seen."+want[5:] {
		t.Fatalf("stale edit %q", d.Source())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.Click("Delete note"); err != nil {
		t.Fatal(err)
	}
	if d.Source() != "Seen.\n\nEnd." {
		t.Fatalf("delete note %q", d.Source())
	}
}

func TestKeptBlocksAndInvalidGraphicsHaveVisibleSource(t *testing.T) {
	const src = "---\ntitle: \"研究\"\ntags:\n  - α\n---\n\n[paper]: https://example.com \"Reading\"\n\n<div>keep me</div>\n\n$$\n\\frac{1}{\n$$\n\n```mermaid\nflowchart TD\n A@{ img: \"https://example.com/a.png\" }\n```\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 890, 1080)
	tt.Frame()
	for _, label := range []string{"FRONTMATTER", "研究", "Edit reference definition", "<div>keep me</div>", "$$\n\\frac{1}{\n$$", "Mermaid could not render: diagram references an external resource"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("missing %s", label)
		}
	}
	if d.Source() != src {
		t.Fatal("presenting kept blocks changed source")
	}
}

func TestAddNoteFromContextMenuAndKeyboardFolding(t *testing.T) {
	const src = "# Heading\r\n\r\nRead here.\n\nAfter.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 740, 420)
	tt.Frame()
	r, ok := d.RectUTF16(13)
	if !ok {
		t.Fatal("missing body caret")
	}
	tt.ClickAt(r.X, r.Y+5)
	tt.RightClickAt(r.X, r.Y+5)
	if err := tt.ChooseMenuItem("Add note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("注")
	tt.Key(ui.Cmd, ui.KeyEnter)
	tt.Frame()
	if d.Source() != "# Heading\r\n\r\nRead here.\r\n\r\n<!-- note: 注 -->\n\nAfter.\n" {
		t.Fatalf("inserted note %q", d.Source())
	}
	if !d.Undo() || d.Source() != src {
		t.Fatal("note insertion history")
	}
	d.SelectUTF16(13, 13)
	tt.Frame()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyLeft)
	tt.Frame()
	if _, ok := tt.Find("Expand Heading"); !ok {
		t.Fatal("keyboard did not fold section")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyRight)
	tt.Frame()
	if _, ok := tt.Find("Fold Heading"); !ok {
		t.Fatal("keyboard did not expand section")
	}
}

func TestChevronFoldMovesCaretOutOfHiddenSection(t *testing.T) {
	const src = "# One\n\nBody text.\n\n# Two\n\nLast.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 740, 420)
	tt.Frame()
	if err := d.SelectUTF16(8, 8); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != 0 || z != 0 {
		t.Fatalf("chevron fold left the caret at %d..%d", a, z)
	}
	if err := tt.Click("Expand One"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != 0 || z != 0 {
		t.Fatalf("expanding moved the caret to %d..%d", a, z)
	}
	if err := d.SelectUTF16(8, 12); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != 0 || z != 0 {
		t.Fatalf("chevron fold left the selection at %d..%d", a, z)
	}
	tt.Type("X")
	tt.Frame()
	if want := "X# One\n\nBody text.\n\n# Two\n\nLast.\n"; d.Source() != want {
		t.Fatalf("typing after folding changed hidden source: %q", d.Source())
	}
}

func TestChevronFoldMovesEOFCaretToLastVisibleHeading(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "trailing newline",
			src:  "# One\n\nBody.\n\n# Two\n\nLast.\n",
			want: "# One\n\nBody.\n\nX# Two\n\nLast.\n",
		},
		{
			name: "no trailing newline",
			src:  "# One\n\nBody.\n\n# Two\n\nLast.",
			want: "# One\n\nBody.\n\nX# Two\n\nLast.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := nativeeditor.New(tc.src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 740, 420)
			tt.Frame()
			twoStart := strings.Index(tc.src, "# Two")
			eof := len(tc.src)
			if err := d.SelectUTF16(eof, eof); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click("Fold Two"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a, z := d.SelectionUTF16(); a != twoStart || z != twoStart {
				t.Fatalf("EOF fold left caret at %d..%d, want %d", a, z, twoStart)
			}
			tt.Type("X")
			tt.Frame()
			if d.Source() != tc.want {
				t.Fatalf("typing at EOF after folding produced %q, want %q", d.Source(), tc.want)
			}
		})
	}

	// Selection ending at EOF also collapses to the visible heading.
	const src = "# One\n\nBody.\n\n# Two\n\nLast."
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 740, 420)
	tt.Frame()
	twoStart := strings.Index(src, "# Two")
	if err := d.SelectUTF16(twoStart+len("# Two\n\n"), len(src)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Fold Two"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != twoStart || z != twoStart {
		t.Fatalf("selection ending at EOF left caret at %d..%d, want %d", a, z, twoStart)
	}

	// Non-last section boundary: caret at the start of the next heading stays there.
	d2 := nativeeditor.New(src)
	tt2 := ui.NewTester(func(c *ui.Context) { d2.View(c, false).Fill().AutoFocus() }, 740, 420)
	tt2.Frame()
	if err := d2.SelectUTF16(twoStart, twoStart); err != nil {
		t.Fatal(err)
	}
	tt2.Frame()
	if err := tt2.Click("Fold One"); err != nil {
		t.Fatal(err)
	}
	tt2.Frame()
	if a, z := d2.SelectionUTF16(); a != twoStart || z != twoStart {
		t.Fatalf("non-last boundary fold moved caret to %d..%d, want %d", a, z, twoStart)
	}

	// Keyboard fold at EOF after an empty final heading must not fold an earlier section.
	const emptyLast = "# One\n\nBody.\n\n# Empty"
	d3 := nativeeditor.New(emptyLast)
	tt3 := ui.NewTester(func(c *ui.Context) { d3.View(c, false).Fill().AutoFocus() }, 740, 420)
	tt3.Frame()
	if err := d3.SelectUTF16(len(emptyLast), len(emptyLast)); err != nil {
		t.Fatal(err)
	}
	tt3.Frame()
	tt3.Key(ui.Cmd|ui.Alt, ui.KeyLeft)
	tt3.Frame()
	if _, ok := tt3.Find("Expand One"); ok {
		t.Fatal("keyboard fold at EOF after empty final heading folded earlier section")
	}
}

func TestNestedHeadingsDoNotCreateFoldsOrShiftHeadingKeys(t *testing.T) {
	const src = "# Top\n\n> # Quoted\n>\n> > # Same\n\n- # Listed\n\n# Same\n\nReal body.\n\n# Next\n\nDone.\n"
	d := nativeeditor.New(src)
	written := map[string]string{}
	d.SetReadingPreferences("", "", func(key, value string) { written[key] = value })
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 740, 720)
	tt.Frame()
	for _, label := range []string{"Fold Quoted", "Fold Listed"} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("nested heading created fold button %q", label)
		}
	}
	if err := tt.Click("Fold Same"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := d.RectUTF16(strings.Index(src, "Real body.")); ok {
		t.Fatal("folding top-level Same kept its body visible")
	}
	if _, ok := d.RectUTF16(strings.Index(src, "Done.")); !ok {
		t.Fatal("folding top-level Same hid the next section")
	}
	if written["folds"] == "" {
		t.Fatal("folding top-level Same did not save fold preferences")
	}
	restored := nativeeditor.New(src)
	restored.SetReadingPreferences(written["folds"], written["widths"], nil)
	tt2 := ui.NewTester(func(c *ui.Context) { restored.View(c, false).Fill() }, 740, 720)
	tt2.Frame()
	if _, ok := tt2.Find("Expand Same"); !ok {
		t.Fatalf("fold preference for top-level Same did not restore (saved %s)", written["folds"])
	}
}

func TestFoldStopsAtTopLevelHeadingPastQuotedHeading(t *testing.T) {
	const src = "# Top\n\nIntro.\n\n> # Quoted\n\nInside quote.\n\n## Child\n\nStill top.\n\n# Next\n\nDone.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 740, 720)
	tt.Frame()
	if err := tt.Click("Fold Top"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	visible := func(text string) bool {
		_, ok := d.RectUTF16(strings.Index(src, text))
		return ok
	}
	for _, hidden := range []string{"Quoted", "Child", "Still top."} {
		if visible(hidden) {
			t.Fatalf("folding Top kept %q on screen", hidden)
		}
	}
	if !visible("Done.") {
		t.Fatal("folding Top hid the next top-level section")
	}
	if err := tt.Click("Expand Top"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.Source() != src {
		t.Fatalf("folding a quoted heading changed source %q", d.Source())
	}
}

func TestAddNoteAtDefaultCaretStaysOutOfFrontmatter(t *testing.T) {
	const src = "---\ntitle: \"Read\"\ntags:\n  - one\n---\n\n# Heading\n\nBody.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 740, 520)
	tt.Frame()
	if err := d.SelectUTF16(0, 0); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, ok := tt.Find("Frontmatter block")
	if !ok {
		t.Fatal("missing frontmatter block")
	}
	tt.RightClickAt(r.X+8, r.Y+8)
	if err := tt.ChooseMenuItem("Add note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Metadata safe")
	tt.Key(ui.Cmd, ui.KeyEnter)
	tt.Frame()
	want := "---\ntitle: \"Read\"\ntags:\n  - one\n---\n\n<!-- note: Metadata safe -->\n\n# Heading\n\nBody.\n"
	if d.Source() != want {
		t.Fatalf("note at the default caret produced %q", d.Source())
	}
}

func TestWideTableScrollsInsideTheReadingColumn(t *testing.T) {
	const src = "Intro\n\n| A | B | C | D | E | F | G | H |\n|---|---|---|---|---|---|---|---|\n| a | b | c | d | e | f | g | h |\n\nAfter.\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 480, 400)
	tt.Frame()
	cell, _ := tt.Find("Table row 2 column 1")
	before, _ := d.RectUTF16(0)
	tt.Wheel(cell.X+30, cell.Y+10, 380, 0)
	tt.Advance(time.Second)
	tt.Frame()
	scrolled, _ := tt.Find("Table row 2 column 1")
	after, _ := d.RectUTF16(0)
	if scrolled.X >= cell.X || after.X != before.X || after.Y != before.Y {
		t.Fatalf("horizontal scroll moved cell %v to %v and prose %v to %v", cell, scrolled, before, after)
	}
	d2 := nativeeditor.New("| A | B |\n|---|---|\n| x | y |\n")
	narrow := ui.NewTester(func(c *ui.Context) { d2.View(c, false).Fill() }, 480, 300)
	narrow.Frame()
	first, _ := narrow.Find("Table row 1 column 1")
	last, _ := narrow.Find("Table row 1 column 2")
	if last.X+last.W > 440 || first.X < 54 || last.W != first.W {
		t.Fatalf("ordinary table overflowed the reading column: %v %v", first, last)
	}
}

func TestCellSelectionReturnsToItsSourceAfterAModeSwitch(t *testing.T) {
	d := nativeeditor.New("Top\r\n\r\n| A | B |\n|---|---|\n| cat | 中文 |\n\nEnd")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 700, 370)
	tt.Frame()
	if err := tt.Click("Table row 2 column 2"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("雪")
	tt.Frame()
	d.SetMode(nativeeditor.Markdown)
	tt.Frame()
	if a, z := d.SelectionUTF16(); a != 36 || a != z {
		t.Fatalf("source caret after editing cell = %d..%d", a, z)
	}
}

func TestInitialNativeLayoutKeepsTallDiagramsAboveFollowingTables(t *testing.T) {
	const src = "# Reading\n\n$$\nx^2\n$$\n\n```mermaid\ngraph TD\n A[Start] --> B[Done]\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 1000, 820)
	diagram, _ := tt.Find("Mermaid block")
	table, _ := tt.Find("Markdown table")
	if diagram.H < 150 || table.Y < diagram.Y+diagram.H {
		t.Fatalf("first presented frame overlaps: diagram %v table %v", diagram, table)
	}
}

func TestLegacyReadingPreferencesRestoreByContent(t *testing.T) {
	d := nativeeditor.New("# Read\n\nBody\n\n# More\n\n| A | B |\n|---|---|\n| x | y |\n")
	written := map[string]string{}
	d.SetReadingPreferences(`{"entries":[{"base":"h1:read","idx":0,"sig":"2517378916"}]}`, `{"4:2:emrdlh":[410,214]}`, func(key, value string) { written[key] = value })
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 900, 540)
	tt.Frame()
	if _, ok := tt.Find("Expand Read"); !ok {
		t.Error("legacy fold did not restore")
	}
	a, _ := tt.Find("Table row 1 column 1")
	b, _ := tt.Find("Table row 1 column 2")
	if a.W != 400 || b.W != 204 {
		t.Errorf("legacy column widths %v %v", a, b)
	}
	if err := tt.Click("Expand Read"); err != nil {
		t.Fatal(err)
	}
	if written["folds"] != `{"entries":[]}` {
		t.Errorf("saved folds %s", written["folds"])
	}
}

func TestSetScrollYBeforeFirstLayoutAppliesAfterLayout(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Long Document\n\n")
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&b, "## Section %03d\n\nParagraph text for section %03d.\n\n", i, i)
	}
	d := nativeeditor.New(b.String())
	d.SetScrollY(900)
	if got := d.ScrollY(); got != 900 {
		t.Fatalf("pending ScrollY() before first frame = %v, want 900", got)
	}
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 820, 640)
	tt.Frame()
	if got := d.ScrollY(); got != 900 {
		t.Fatalf("ScrollY() after frame 1 before pending apply = %v, want 900", got)
	}
	tt.Frame()
	tt.Frame()
	if got := d.ScrollY(); got < 898 || got > 902 {
		t.Fatalf("ScrollY() after layout settled = %v, want ~900", got)
	}
}
