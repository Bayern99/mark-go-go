package nativeeditor

import (
	"strings"
	"testing"

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
