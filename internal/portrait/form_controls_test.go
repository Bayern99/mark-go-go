package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestPortraitFieldsExposeStatesAndEditing(t *testing.T) {
	value, query, area := "draft", "dark", "notes"
	app := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.Column(c).Gap(8).Children(func() {
			portrait.TextField(c, &value, portrait.FieldSpec{Label: "Service tier", Placeholder: "Tier", Invalid: true})
			portrait.SearchField(c, &query, portrait.SearchSpec{Label: "Search", Shortcut: []string{"/"}})
			portrait.TextArea(c, &area, portrait.AreaSpec{Label: "Notes", Rows: 2})
			portrait.TextField(c, new(string), portrait.FieldSpec{Label: "Disabled field", Disabled: true})
		})
	}), 520, 300)
	app.Frame()
	if field, ok := app.AccessByRole(ui.RoleTextField, "Service tier"); !ok || !field.Invalid {
		t.Fatalf("invalid field = %+v", field)
	}
	if field, ok := app.AccessByRole(ui.RoleTextField, "Disabled field"); !ok || !field.Disabled {
		t.Fatalf("disabled field = %+v", field)
	}
	if err := app.Click("Clear Search"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if query != "" {
		t.Fatalf("clear left query %q", query)
	}
	if err := app.Click("Service tier"); err != nil {
		t.Fatal(err)
	}
	app.Type(" priority")
	if value != "draft priority" {
		t.Fatalf("field value = %q", value)
	}
}

func TestPortraitSelectStepperAndSettingsRows(t *testing.T) {
	choice, resets := "light", 0
	value := float32(16)
	app := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.Column(c).Gap(10).Children(func() {
			portrait.Select(c, &choice, []portrait.Option{{Value: "light", Label: "Light"}, {Value: "dark", Label: "Dark"}}, portrait.SelectSpec{Label: "Appearance select"})
			portrait.Stepper(c, &value, 15, 18, 1, nil)
			portrait.SettingsGroup(c, "Reading", func(rows *portrait.Rows) {
				rows.Row(portrait.RowSpec{Title: "Text size", Description: "Reading size", Reset: func() { resets++ }}, func() {
					portrait.Caption(c, "16 pt")
				})
			})
		})
	}), 560, 320)
	app.Frame()
	if err := app.Click("Appearance select"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if choice != "dark" {
		t.Fatalf("select = %q", choice)
	}
	if err := app.Click("Increase value"); err != nil {
		t.Fatal(err)
	}
	if value != 17 {
		t.Fatalf("stepper = %v", value)
	}
	if err := app.Click("Reset Text size"); err != nil {
		t.Fatal(err)
	}
	if resets != 1 {
		t.Fatalf("resets = %d", resets)
	}
	if _, ok := app.AccessByRole(ui.RoleGroup, "Text size"); !ok {
		t.Fatal("settings row lacks its accessible group")
	}
}

func TestPortraitDisclosureOutputAndListRow(t *testing.T) {
	open, clicked := false, false
	app := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.Column(c).Gap(8).Children(func() {
			portrait.Disclosure(c, "CLI status", &open, portrait.DisclosureSpec{Status: "Healthy"}, func() {
				portrait.Caption(c, "Runtime ready")
			})
			portrait.OutputBlock(c, "Doctor", "No issues")
			if portrait.ListRow(c, portrait.ListRowSpec{Title: "Open document", Detail: "Notes.md"}).Clicked() {
				clicked = true
			}
		})
	}), 520, 360)
	app.Frame()
	if app.HasText("Runtime ready") {
		t.Fatal("closed disclosure shows its body")
	}
	if err := app.Click("CLI status"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Runtime ready") {
		t.Fatal("open disclosure hides its body")
	}
	if err := app.Click("Copy Doctor"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Open document"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("list row did not activate")
	}
}

func TestTextAreaBareHandsItsEdgeToTheContainer(t *testing.T) {
	value := "Reply"
	for _, tc := range []struct {
		name       string
		bare       bool
		wantBorder bool
	}{
		{"framed", false, true},
		{"bare", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := ui.NewTester(onChrome(func(c *ui.Context) {
				portrait.TextArea(c, &value, portrait.AreaSpec{Label: "Composer", Rows: 2, Bare: tc.bare})
			}), 320, 200)
			tt.Frame()
			input, ok := tt.AccessByRole(ui.RoleTextField, "Composer")
			if !ok || !input.Multiline {
				t.Fatalf("text area = %+v, found %v", input, ok)
			}
			edge := int(input.Bounds.X - portrait.Form.PadX)
			mid := int(input.Bounds.Y + input.Bounds.H/2)
			chrome := tt.Image().RGBAAt(1, 1)
			got := tt.Image().RGBAAt(edge, mid)
			if tc.wantBorder && got == chrome {
				t.Fatal("framed text area painted no edge")
			}
			if !tc.wantBorder && got != chrome {
				t.Fatalf("bare text area painted %v over the chrome behind it", got)
			}
			inside := tt.Image().RGBAAt(edge+1, mid)
			if !tc.wantBorder && inside != chrome {
				t.Fatalf("bare text area painted its own background %v", inside)
			}
			if tc.wantBorder && inside == chrome {
				t.Fatal("framed text area painted no background")
			}
		})
	}
}
