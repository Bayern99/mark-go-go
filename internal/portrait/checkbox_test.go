package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCheckboxStateMatrix(t *testing.T) {
	for _, initial := range []ui.CheckState{ui.CheckOff, ui.CheckOn, ui.CheckMixed} {
		t.Run(map[ui.CheckState]string{ui.CheckOff: "unchecked", ui.CheckOn: "checked", ui.CheckMixed: "mixed"}[initial], func(t *testing.T) {
			state := initial
			tt := ui.NewTester(onChrome(func(c *ui.Context) {
				portrait.CheckboxState(c, &state, "Select all")
			}), 240, 120)
			tt.Frame()
			info, ok := tt.AccessByRole(ui.RoleCheckBox, "Select all")
			if !ok {
				t.Fatal("checkbox missing from the accessibility tree")
			}
			if info.Checked != (initial == ui.CheckOn) || info.Mixed != (initial == ui.CheckMixed) {
				t.Fatalf("initial accessibility state = %+v, want %v", info, initial)
			}
			if err := tt.Click("Select all"); err != nil {
				t.Fatal(err)
			}
			want := ui.CheckOn
			if initial == ui.CheckOn {
				want = ui.CheckOff
			}
			if state != want {
				t.Fatalf("click changed %v to %v, want %v", initial, state, want)
			}
			info, _ = tt.AccessByRole(ui.RoleCheckBox, "Select all")
			if info.Checked != (want == ui.CheckOn) || info.Mixed {
				t.Fatalf("accessibility state after click = %+v, want %v", info, want)
			}
		})
	}
}

func TestCheckboxKeyboardDisabledAndInvalidStates(t *testing.T) {
	t.Run("keyboard", func(t *testing.T) {
		checked := false
		tt := ui.NewTester(onChrome(func(c *ui.Context) {
			portrait.Checkbox(c, &checked, "Sync changes")
		}), 240, 120)
		tt.Frame()
		tt.Key(0, ui.KeyTab)
		info, _ := tt.AccessByRole(ui.RoleCheckBox, "Sync changes")
		if !info.Focused {
			t.Fatal("Tab did not focus the checkbox")
		}
		tt.Key(0, ui.KeySpace)
		if !checked {
			t.Fatal("Space did not check the focused checkbox")
		}
		info, _ = tt.AccessByRole(ui.RoleCheckBox, "Sync changes")
		if !info.Checked {
			t.Fatalf("checked state was not exposed: %+v", info)
		}
		tt.Key(0, ui.KeySpace)
		if checked {
			t.Fatal("a second Space did not uncheck the checkbox")
		}
	})

	t.Run("disabled and invalid", func(t *testing.T) {
		state := ui.CheckMixed
		tt := ui.NewTester(onChrome(func(c *ui.Context) {
			portrait.CheckboxState(c, &state, "Select all").Disabled(true).Invalid(true)
		}), 240, 120)
		tt.Frame()
		info, _ := tt.AccessByRole(ui.RoleCheckBox, "Select all")
		if !info.Disabled || !info.Invalid || !info.Mixed {
			t.Fatalf("disabled invalid mixed checkbox = %+v", info)
		}
		tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		if state != ui.CheckMixed {
			t.Fatalf("disabled checkbox changed to %v", state)
		}
	})
}

func TestCheckboxMarksAreVisuallyDistinct(t *testing.T) {
	images := make(map[ui.CheckState][]uint32)
	for _, state := range []ui.CheckState{ui.CheckOff, ui.CheckOn, ui.CheckMixed} {
		state := state
		tt := ui.NewTester(onChrome(func(c *ui.Context) {
			portrait.CheckboxState(c, &state, "Select all")
		}), 240, 120)
		tt.Frame()
		info, _ := tt.AccessByRole(ui.RoleCheckBox, "Select all")
		images[state] = region(tt.Image(), info.Bounds)
	}
	if differs(images[ui.CheckOff], images[ui.CheckOn]) == 0 {
		t.Fatal("checked and unchecked checkboxes look identical")
	}
	if differs(images[ui.CheckOn], images[ui.CheckMixed]) == 0 {
		t.Fatal("checked and mixed checkboxes look identical")
	}
}

func TestCompactCheckboxSetsItsLabelAtUIscale(t *testing.T) {
	fullText := false
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.CompactCheckbox(c, &fullText, "Full text")
	}), 240, 120)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleCheckBox, "Full text")
	if !ok {
		t.Fatal("compact checkbox missing from the accessibility tree")
	}
	if info.Bounds.H != portrait.Type.UI.Leading {
		t.Fatalf("compact checkbox is %v tall, want the 13/18 UI line %v", info.Bounds.H, portrait.Type.UI.Leading)
	}
	if err := tt.Click("Full text"); err != nil {
		t.Fatal(err)
	}
	if !fullText {
		t.Fatal("compact checkbox does not toggle")
	}
}

func TestCheckboxKeepsTheReaderSizeForDialogs(t *testing.T) {
	checked := true
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Checkbox(c, &checked, "Notes.md")
	}), 240, 120)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleCheckBox, "Notes.md")
	if !ok || info.Bounds.H != portrait.Type.Small.Leading {
		t.Fatalf("checkbox = %+v, want the 14/22 reader line %v", info.Bounds, portrait.Type.Small.Leading)
	}
}
