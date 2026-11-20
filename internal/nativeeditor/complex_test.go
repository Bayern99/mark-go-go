package nativeeditor_test

import (
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
	for _, label := range []string{"Frontmatter", "研究", "Edit reference definition", "<div>keep me</div>", "$$\n\\frac{1}{\n$$", "Mermaid could not render: diagram references an external resource"} {
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
