package nativeeditor_test

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestTableCompositionSourceAndViewSwitch(t *testing.T) {
	for _, switchView := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "markdown"}[switchView], func(t *testing.T) {
			const src = "| Name | Tag |\r\n| --- | --- |\n| cat | x |"
			d := nativeeditor.New(src)
			focus := false
			tt := ui.NewTester(func(c *ui.Context) {
				field := d.View(c, false).Fill()
				if focus {
					field.Focus()
					focus = false
				}
			}, 760, 420)
			tt.Frame()
			if err := tt.Click("Table row 2 column 1"); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Compose("n", 1)
			tt.Frame()
			tt.Compose("ni", 2)
			tt.Frame()
			if want := strings.Replace(src, "cat", "ni", 1); d.Source() != want {
				t.Fatalf("source while saving preedit = %q, want %q", d.Source(), want)
			}
			if switchView {
				d.SetMode(nativeeditor.Markdown)
				focus = true
				tt.Frame()
				tt.Compose("nin", 3)
				tt.Frame()
			}
			tt.Type("你")
			tt.Frame()
			want := strings.Replace(src, "cat", "你", 1)
			if d.Source() != want {
				t.Fatalf("later commit = %q, want %q", d.Source(), want)
			}
			tt.Command("undo")
			tt.Frame()
			if d.Source() != src || d.CanUndo() {
				t.Fatalf("undo = %q, remaining undo=%v", d.Source(), d.CanUndo())
			}
		})
	}
}

func TestTableCompositionFoldRestores(t *testing.T) {
	const src = "# Top\n\n| Name | Tag |\n| --- | --- |\n| cat | x |\n\n# Next\n\nEnd."
	d := nativeeditor.New(src)
	written := map[string]string{}
	d.SetReadingPreferences("", "", func(key, value string) { written[key] = value })
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 760, 420)
	tt.Frame()
	if err := tt.Click("Table row 2 column 1"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Compose("ni", 2)
	tt.Frame()
	if err := tt.Click("Fold Top"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := strings.Replace(src, "cat", "ni", 1); d.Source() != want {
		t.Fatalf("folded source = %q, want %q", d.Source(), want)
	}
	restored := nativeeditor.New(d.Source())
	restored.SetReadingPreferences(written["folds"], written["widths"], nil)
	app := ui.NewTester(func(c *ui.Context) { restored.View(c, false).Fill() }, 760, 420)
	app.Frame()
	if _, ok := app.Find("Expand Top"); !ok {
		t.Fatalf("composition fold did not restore: %s", written["folds"])
	}
}

func TestCompositionBufferHistory(t *testing.T) {
	for _, original := range []string{"", "猫\r\n狗\r\n"} {
		t.Run(original, func(t *testing.T) {
			b := editor.NewSourceBuffer(original)
			owner := new(int)
			if b.ReplaceCompositionAtVersion(1, 0, 0, "n", owner) || b.CanUndo() {
				t.Fatal("stale revision changed history")
			}
			if b.ReplaceCompositionAtVersion(0, -1, 0, "n", owner) || b.ReplaceCompositionAtVersion(0, 0, len(b.String())+1, "n", owner) {
				t.Fatal("invalid range accepted")
			}
			if !b.ReplaceCompositionAtVersion(b.Version(), 0, 0, "n", owner) || !b.ReplaceCompositionAtVersion(b.Version(), 0, 1, "你", owner) {
				t.Fatal("composition rejected")
			}
			b.EndComposition(owner)
			if !b.ReplaceCompositionAtVersion(b.Version(), 0, 0, "好", owner) {
				t.Fatal("next composition rejected")
			}
			b.EndTyping()
			if !b.Undo() || b.String() != "你"+strings.ReplaceAll(original, "\r\n", "\n") {
				t.Fatalf("EndComposition did not close the group: %q", b.String())
			}
			if !b.Undo() || b.Modified() || b.CanUndo() {
				t.Fatal("first composition did not undo separately")
			}
			for range 2 {
				if !b.ReplaceCompositionAtVersion(b.Version(), 0, 0, "n", owner) {
					t.Fatal("composition rejected after EndTyping")
				}
				b.EndTyping()
			}
			if !b.Undo() || !strings.HasPrefix(b.String(), "n") || !b.Undo() || b.Modified() {
				t.Fatal("EndTyping did not separate editCompose groups")
			}
		})
	}
}

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
				if want := strings.Replace(src, "|"+initial+"| x |", "|"+initial+"ni| x |", 1); d.Source() != want {
					t.Fatalf("preedit = %q, want %q", d.Source(), want)
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
