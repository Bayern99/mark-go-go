package nativeeditor_test

import (
	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestSourceTransactionsAndUTF16Anchors(t *testing.T) {
	const source = "甲🎵e\u0301\r\n_two_ **two**\n"
	d := nativeeditor.New(source)
	if err := d.SelectUTF16(8, 3); err != nil {
		t.Fatal(err)
	}
	if a, z := d.SelectionUTF16(); a != 8 || z != 3 {
		t.Fatalf("backwards selection %d,%d", a, z)
	}
	for _, bad := range []int{-1, 2, 6, 100} {
		if d.SelectUTF16(0, bad) == nil {
			t.Errorf("accepted invalid anchor %d", bad)
		}
	}
	rev := d.Version()
	start := len("甲🎵e\u0301\r\n_two_ **")
	if err := d.Apply(rev, start, start+3, "two", "乙"); err != nil {
		t.Fatal(err)
	}
	want := "甲🎵e\u0301\r\n_two_ **乙**\n"
	if d.Source() != want {
		t.Fatalf("edit changed other bytes %q", d.Source())
	}
	for _, run := range []func() error{
		func() error { return d.Apply(rev, start, start+3, "乙", "A") },
		func() error { return d.Apply(d.Version(), start, start+3, "two", "A") },
		func() error { return d.Apply(d.Version(), 1, 2, d.Source()[1:2], "B") },
	} {
		if run() == nil || d.Source() != want {
			t.Fatal("unexpected successful write")
		}
	}
	if !d.Undo() || d.Source() != source || d.CanUndo() {
		t.Fatal("proposal did not undo in one step")
	}
	if !d.Redo() || d.Source() != want {
		t.Fatal("redo did not restore exact bytes")
	}
}

func TestContinuousEditingAcrossModes(t *testing.T) {
	d := nativeeditor.New("**甲🎵乙** and _two_\r\n")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 560, 320)
	tt.Frame()
	if d.Mode() != nativeeditor.Preview {
		t.Fatal("expected Preview initially")
	}
	if err := d.SelectUTF16(3, 5); err != nil {
		t.Fatal(err)
	}
	tt.Type("新")
	tt.Frame()
	const first = "**甲新乙** and _two_\r\n"
	if d.Source() != first {
		t.Fatalf("preview changed %q", d.Source())
	}
	d.SetMode(nativeeditor.Markdown)
	tt.Frame()
	tt.Type("加")
	tt.Frame()
	const second = "**甲新加乙** and _two_\r\n"
	if d.Source() != second {
		t.Fatalf("source changed %q", d.Source())
	}
	d.SetMode(nativeeditor.Preview)
	tt.Frame()
	for _, want := range []string{first, "**甲🎵乙** and _two_\r\n"} {
		tt.Command("undo")
		tt.Frame()
		if d.Source() != want {
			t.Fatalf("undo got %q; want %q", d.Source(), want)
		}
	}
	for _, want := range []string{first, second} {
		tt.Command("redo")
		tt.Frame()
		if d.Source() != want {
			t.Fatalf("redo got %q; want %q", d.Source(), want)
		}
	}
}
