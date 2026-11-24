package portrait

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDecisionKeyboardChoices(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		key                   ui.Key
		mods                  ui.Modifiers
		wantSave, wantDiscard int
	}{
		{name: "Enter saves", key: ui.KeyEnter, wantSave: 1},
		{name: "Command S saves", key: ui.KeyS, mods: ui.Cmd, wantSave: 1},
		{name: "Command D discards", key: ui.KeyD, mods: ui.Cmd, wantDiscard: 1},
		{name: "Escape cancels", key: ui.KeyEscape},
	} {
		t.Run(tc.name, func(t *testing.T) {
			open := true
			var saves, discards int
			app := ui.NewTester(func(c *ui.Context) {
				Decision(c, &open, DecisionSpec{Title: "Save changes?", Body: "Choose", OnSave: func() { saves++ }, OnDiscard: func() { discards++ }})
			}, 640, 420)
			app.Frame()
			app.Key(tc.mods, tc.key)
			app.Frame()
			if open || saves != tc.wantSave || discards != tc.wantDiscard {
				t.Fatalf("open=%v saves=%d discards=%d", open, saves, discards)
			}
		})
	}
}

func TestDecisionArrowsAndFocusReturn(t *testing.T) {
	open := true
	var returnFocus ui.Handle
	discards := 0
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			Button(c, "Close tab", "Close tab").Bind(&returnFocus)
			Decision(c, &open, DecisionSpec{Title: "Save changes?", Return: &returnFocus, OnDiscard: func() { discards++ }})
		})
	}, 640, 420)
	app.Frame()
	app.Key(0, ui.KeyLeft)
	app.Key(0, ui.KeyLeft)
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if open || discards != 1 {
		t.Fatalf("arrow-selected Don't Save: open=%v discards=%d", open, discards)
	}
	if node, ok := app.Access("Close tab"); !ok || !node.Focused {
		t.Fatalf("focus did not return to the close control: %+v, %v", node, ok)
	}
}

func TestPromptPreselectsStemAndValidates(t *testing.T) {
	open := true
	value := "Draft.md"
	submitted := 0
	app := ui.NewTester(func(c *ui.Context) {
		Prompt(c, &open, PromptSpec{Title: "Rename", Label: "File name", Value: &value, SelectTo: 5, SubmitLabel: "Rename", Validate: func(name string) string {
			if name == "" {
				return "Enter a file name"
			}
			return ""
		}, OnSubmit: func() { submitted++ }})
	}, 640, 420)
	app.Frame()
	node, ok := app.Access("File name")
	if !ok {
		t.Fatal("rename field is missing")
	}
	if node.Value != "Draft.md" {
		t.Fatalf("rename field shows %q, want the document's name", node.Value)
	}
	if !node.Focused {
		t.Fatal("the rename field does not take the keyboard focus")
	}
	// The stem is preselected, so typing replaces "Draft" and leaves ".md".
	app.Type("Notes")
	app.Frame()
	if value != "Notes.md" {
		t.Fatalf("typing into the preselected stem gave %q, want %q", value, "Notes.md")
	}
	value = ""
	app.Frame()
	if !app.HasText("Enter a file name") {
		t.Fatal("empty rename has no inline validation")
	}
	value = "Notes.md"
	app.Frame()
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if open || submitted != 1 {
		t.Fatalf("valid rename did not submit: open=%v submitted=%d", open, submitted)
	}
}
