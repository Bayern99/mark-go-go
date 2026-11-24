package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCompactRadioSetsItsLabelAtUIscale(t *testing.T) {
	pick := "paper"
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.RadioGroup(c, func() {
			for _, option := range []struct{ value, label string }{{"paper", "Published paper"}, {"notes", "Local notes"}} {
				portrait.CompactRadio(c, &pick, option.value, option.label)
			}
		}).Label("Source")
	}), 260, 160)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleRadio, "Local notes")
	if !ok {
		t.Fatal("compact radio missing from the accessibility tree")
	}
	if info.Bounds.H != portrait.Type.UI.Leading {
		t.Fatalf("compact radio is %v tall, want the 13/18 UI line %v", info.Bounds.H, portrait.Type.UI.Leading)
	}
	if err := tt.Click("Local notes"); err != nil {
		t.Fatal(err)
	}
	if pick != "notes" {
		t.Fatalf("clicking chose %q, want notes", pick)
	}
	if chosen, _ := tt.AccessByRole(ui.RoleRadio, "Local notes"); !chosen.Checked {
		t.Fatalf("compact radio does not expose its checked state: %+v", chosen)
	}
}

func TestRadioKeepsTheReaderSize(t *testing.T) {
	pick := "paper"
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Radio(c, &pick, "paper", "Published paper")
	}), 260, 160)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleRadio, "Published paper")
	if !ok || info.Bounds.H != portrait.Type.Small.Leading {
		t.Fatalf("radio = %+v, want the 14/22 reader line %v", info.Bounds, portrait.Type.Small.Leading)
	}
}
