package nativeeditor_test

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
)

func TestTableCellCompositionUndo(t *testing.T) {
	for _, keyboard := range []bool{false, true} {
		t.Run(map[bool]string{false: "command", true: "keyboard"}[keyboard], func(t *testing.T) {
			const src = "before\r\n\r\n| Name | Tag |\r\n| --- | --- |\n| 猫 | a\\|b |\r\n\r\nafter"
			d := nativeeditor.New(src)
			tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
			tt.Frame()
			if err := tt.Click("Table row 2 column 1"); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Compose("x", 1)
			tt.Frame()
			tt.Compose("xue", 3)
			tt.Frame()
			tt.Type("雪")
			tt.Frame()
			want := strings.Replace(src, "猫", "雪", 1)
			if d.Source() != want {
				t.Fatalf("commit = %q", d.Source())
			}
			if keyboard {
				tt.Key(ui.Cmd, ui.KeyZ)
			} else {
				tt.Command("undo")
			}
			tt.Frame()
			if d.Source() != src {
				t.Fatalf("undo = %q, want %q", d.Source(), src)
			}
			if keyboard {
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			} else {
				tt.Command("redo")
			}
			tt.Frame()
			if d.Source() != want {
				t.Fatalf("redo = %q, want %q", d.Source(), want)
			}
		})
	}
}

func TestTableCellCompositionLifecycle(t *testing.T) {
	for _, initial := range []string{"", "猫"} {
		for _, end := range []string{"cancel", "blur", "commit-blur", "cancel-blur", "consecutive"} {
			t.Run(initial+"/"+end, func(t *testing.T) {
				src := "| Name | Tag |\n| --- | --- |\n|" + initial + "| x |"
				d := nativeeditor.New(src)
				tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
				tt.Frame()
				if err := tt.Click("Table row 2 column 1"); err != nil {
					t.Fatal(err)
				}
				tt.Key(0, ui.KeyEnd)
				tt.Compose("n", 1)
				tt.Frame()
				tt.Compose("ni", 2)
				tt.Frame()
				if d.Source() != src || d.CanUndo() {
					t.Fatalf("preedit published: %q, undo=%v", d.Source(), d.CanUndo())
				}
				text := "你"
				switch end {
				case "cancel", "cancel-blur":
					tt.Compose("", 0)
					text = ""
				case "blur":
					text = "ni"
				default:
					tt.Type(text)
				}
				if strings.Contains(end, "blur") {
					if err := tt.Click("Table row 2 column 2"); err != nil {
						t.Fatal(err)
					}
				}
				tt.Frame()
				want := strings.Replace(src, "|"+initial+"| x |", "|"+initial+text+"| x |", 1)
				if d.Source() != want {
					t.Fatalf("composition end = %q, want %q", d.Source(), want)
				}
				if text == "" {
					if d.CanUndo() || d.Modified() {
						t.Fatal("cancel changed source history or modified state")
					}
					return
				}
				if end == "consecutive" {
					tt.Compose("h", 1)
					tt.Frame()
					tt.Compose("hao", 3)
					tt.Frame()
					tt.Type("好")
					tt.Frame()
					if got := strings.Replace(want, "你", "你好", 1); d.Source() != got {
						t.Fatalf("second commit = %q, want %q", d.Source(), got)
					}
					if !d.Undo() || d.Source() != want {
						t.Fatalf("second undo = %q, want %q", d.Source(), want)
					}
				}
				if !d.Undo() || d.Source() != src || d.CanUndo() {
					t.Fatalf("first undo = %q, remaining undo=%v", d.Source(), d.CanUndo())
				}
			})
		}
	}
}

func TestParagraphCompositionUndo(t *testing.T) {
	const src = "猫"
	d := nativeeditor.New(src)
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill().AutoFocus() }, 760, 420)
	tt.Frame()
	tt.Command("selectAll")
	tt.Compose("x", 1)
	tt.Frame()
	tt.Compose("xue", 3)
	tt.Frame()
	tt.Type("雪")
	tt.Frame()
	if d.Source() != "雪" {
		t.Fatalf("commit = %q", d.Source())
	}
	tt.Command("undo")
	tt.Frame()
	if d.Source() != src || d.CanUndo() {
		t.Fatalf("undo = %q, remaining undo=%v", d.Source(), d.CanUndo())
	}
	tt.Command("redo")
	tt.Frame()
	if d.Source() != "雪" {
		t.Fatalf("redo = %q", d.Source())
	}
}
