package nativeeditor

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFindBarUsesPortraitControls(t *testing.T) {
	doc := New("one two one")
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { doc.View(c, false).Grow(1).FillWidth() })
	}, 720, 420)
	app.Frame()
	if err := app.Click("Markdown Preview"); err != nil {
		t.Fatal(err)
	}
	app.Key(ui.Cmd, ui.KeyF)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleToolbar, "Find"); !ok {
		t.Fatal("find toolbar is missing")
	}
	for _, name := range []string{"Find", "Previous match", "Next match", "Show replacement", "Close find"} {
		if _, ok := app.Access(name); !ok {
			t.Errorf("missing find control %q", name)
		}
	}
	app.Type("one")
	app.Frame()
	if !app.HasText("2 of 2") && !app.HasText("1 of 2") {
		t.Fatalf("find match count is missing: %v", app.Texts())
	}
	if err := app.Click("Show replacement"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	for _, name := range []string{"Replace current match", "Replace all matches"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("missing replacement control %q", name)
		}
	}
}
