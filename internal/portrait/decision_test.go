package portrait

import (
	"strings"
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

func TestDecisionMultipleDocumentSaveSelection(t *testing.T) {
	documents := []DecisionDocument{{ID: "a", Name: "A.md"}, {ID: "b", Name: "B.md"}, {ID: "c", Name: "C.md"}}
	for _, tc := range []struct {
		name      string
		selected  []bool
		wantLabel string
		wantSave  bool
		wantWarn  bool
	}{
		{name: "0 of N disables Save and ignores Command S", selected: []bool{false, false, false}, wantLabel: "Save Selected"},
		{name: "k of N names both outcomes and warns", selected: []bool{true, false, true}, wantLabel: "Save 2, Discard 1", wantSave: true, wantWarn: true},
		{name: "N of N keeps the original action", selected: []bool{true, true, true}, wantLabel: "Save Selected", wantSave: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			open, saves := true, 0
			app := ui.NewTester(func(c *ui.Context) {
				Decision(c, &open, DecisionSpec{Title: "Save changes?", Documents: documents, Selected: &tc.selected, SaveLabel: "Save Selected", OnSave: func() { saves++ }})
			}, 640, 420)
			app.Frame()
			save, ok := app.AccessByRole(ui.RoleButton, tc.wantLabel)
			if !ok || save.Disabled == tc.wantSave {
				t.Fatalf("Save accessibility = %+v, found %v; want enabled %v", save, ok, tc.wantSave)
			}
			if got := app.HasText("Unchecked documents will be discarded."); got != tc.wantWarn {
				t.Fatalf("discard warning visible = %v, want %v", got, tc.wantWarn)
			}
			app.Key(ui.Cmd, ui.KeyS)
			app.Frame()
			wantSaves, wantOpen := 0, true
			if tc.wantSave {
				wantSaves, wantOpen = 1, false
			}
			if saves != wantSaves || open != wantOpen {
				t.Fatalf("Command S: open=%v saves=%d, want open=%v saves=%d", open, saves, wantOpen, wantSaves)
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

func TestPromptValidationWaitsForSubmitAndLinksItsError(t *testing.T) {
	open := true
	value := ""
	app := ui.NewTester(func(c *ui.Context) {
		Prompt(c, &open, PromptSpec{Title: "Rename", Label: "File name", Value: &value, Validate: func(name string) string {
			if name == "" {
				return "Enter a file name"
			}
			return ""
		}})
	}, 640, 420)
	app.Frame()
	if app.HasText("Enter a file name") {
		t.Fatal("untouched prompt shows an error")
	}
	if submit, _ := app.AccessByRole(ui.RoleButton, "OK"); !submit.Disabled {
		t.Fatal("invalid prompt does not disable submit")
	}
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if !app.HasText("Enter a file name") {
		t.Fatal("invalid submit did not reveal its error")
	}
	field, _ := app.AccessByRole(ui.RoleTextField, "File name")
	if !field.Invalid || !strings.Contains(field.Description, "Enter a file name") {
		t.Fatalf("error is not linked to the input: %+v", field)
	}
	app.Type("Draft.md")
	app.Frame()
	if app.HasText("Enter a file name") {
		t.Fatal("typing a replacement keeps showing the stale error")
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
