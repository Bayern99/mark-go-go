package portrait_test

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestUncheckedRadioBoundaryMeetsNonTextContrast(t *testing.T) {
	eachLook(t, func(t *testing.T, _ portrait.Space, _ bool) {
		for _, highContrast := range []bool{false, true} {
			pick := "other"
			tt := ui.NewTesterWith(onChrome(func(c *ui.Context) {
				portrait.Radio(c, &pick, "paper", "Published paper")
			}), 260, 120, ui.TesterOptions{Preferences: ui.Preferences{HighContrast: highContrast}})
			tt.Frame()
			info, _ := tt.AccessByRole(ui.RoleRadio, "Published paper")
			border := portrait.Colors.Ink3
			if highContrast {
				border = portrait.Colors.Ink2
			}
			if ratio := wcagContrast(wcagColor(border), wcagColor(portrait.Colors.Paper)); ratio < 3 {
				t.Errorf("high contrast %v: boundary token contrast %.3f, want >= 3", highContrast, ratio)
			}
			px := tt.Image().RGBAAt(int(info.Bounds.X), int(info.Bounds.Y+info.Bounds.H/2))
			ratio := wcagContrast(wcagColor(ui.RGB(px.R, px.G, px.B)), wcagColor(portrait.Colors.Paper))
			if ratio < 3 {
				t.Errorf("high contrast %v: rendered boundary #%02x%02x%02x has contrast %.3f, want >= 3", highContrast, px.R, px.G, px.B, ratio)
			}
		}
	})
}

func TestCompactRadioSetsItsLabelAtUIscale(t *testing.T) {
	pick := "paper"
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		ui.RadioGroup(c, func() {
			for _, option := range []struct{ value, label string }{{"paper", "Published paper"}, {"notes", "Local notes"}} {
				portrait.CompactRadio(c, &pick, option.value, option.label)
			}
		}).Label("Source").Gap(2)
	}), 260, 160)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleRadio, "Local notes")
	if !ok {
		t.Fatal("compact radio missing from the accessibility tree")
	}
	if info.Bounds.H < portrait.Sizes.Compact {
		t.Fatalf("compact radio is %v tall, want at least %v", info.Bounds.H, portrait.Sizes.Compact)
	}
	first, _ := tt.AccessByRole(ui.RoleRadio, "Published paper")
	if pitch := info.Bounds.Y - first.Bounds.Y; pitch < 24 {
		t.Fatalf("compact radio pitch is %v, want at least 24", pitch)
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

func TestSegmentedHasVisibleFocusAndPressedFeedback(t *testing.T) {
	selected := 0
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Segmented(c, &selected, []string{"Preview", "Markdown"}, "Document view")
	}), 320, 160)
	tt.Frame()
	track, _ := tt.AccessByRole(ui.RoleRadioGroup, "Document view")
	rest := region(tt.Image(), track.Bounds)
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Preview") {
		t.Fatal("Tab did not focus the selected segment")
	}
	if differs(rest, region(tt.Image(), track.Bounds)) == 0 {
		t.Fatal("focused segment has no visible focus ring")
	}
	segment, _ := tt.AccessByRole(ui.RoleRadio, "Markdown")
	beforePress := region(tt.Image(), segment.Bounds)
	tt.Press(segment.Bounds.X+segment.Bounds.W/2, segment.Bounds.Y+segment.Bounds.H/2)
	tt.Advance(portrait.Motion.Press + 20*time.Millisecond)
	if differs(beforePress, region(tt.Image(), segment.Bounds)) == 0 {
		t.Fatal("pressed segment changed no pixels")
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
