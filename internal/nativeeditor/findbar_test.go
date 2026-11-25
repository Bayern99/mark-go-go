package nativeeditor

import (
	"fmt"
	"strings"
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

func TestFindBarCounterKeepsTheQueryFieldInPlace(t *testing.T) {
	doc := New(strings.Repeat("one\n", 100))
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { doc.View(c, false).Grow(1).FillWidth().AutoFocus() })
	}, 900, 700)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyF)
	app.Frame()
	app.Type("one")
	app.Frame()
	queryX := func(index int) float32 {
		t.Helper()
		found := false
		for _, text := range app.Texts() {
			if text == fmt.Sprintf("%d of 100", index) {
				found = true
			}
		}
		if !found {
			t.Fatalf("the counter does not show %d of 100: %v", index, app.Texts())
		}
		field, ok := app.Access("Find")
		if !ok {
			t.Fatal("the query field is missing")
		}
		return field.Bounds.X
	}
	at := map[int]float32{}
	for index := 1; index <= 100; index++ {
		at[index] = queryX(index)
		app.Key(0, ui.KeyEnter)
		app.Frame()
	}
	for _, index := range []int{10, 99, 100} {
		if at[index] != at[9] {
			t.Errorf("the query field sits at %v for match %d and at %v for match 9; the counter must not move it", at[index], index, at[9])
		}
	}
}
