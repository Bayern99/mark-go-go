package portrait_test

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestBadgeToneAndPreferenceMatrix(t *testing.T) {
	tones := []struct {
		name string
		tone portrait.Tone
	}{
		{"neutral", portrait.Tones.Neutral},
		{"accent", portrait.Tones.Accent},
		{"info", portrait.Tones.Info},
		{"warning", portrait.Tones.Warning},
		{"danger", portrait.Tones.Danger},
	}
	preferences := []struct {
		name  string
		prefs ui.Preferences
	}{
		{"default", ui.Preferences{}},
		{"high-contrast", ui.Preferences{HighContrast: true}},
		{"reduce-transparency", ui.Preferences{ReduceTransparency: true}},
	}
	for _, tone := range tones {
		for _, preference := range preferences {
			t.Run(tone.name+"/"+preference.name, func(t *testing.T) {
				const label = "所有文档更改均已同步完成"
				tt := ui.NewTesterWith(onChrome(func(c *ui.Context) {
					portrait.Badge(c, label, tone.tone).MaxWidth(180)
				}), 280, 120, ui.TesterOptions{Preferences: preference.prefs})
				tt.Frame()
				info, ok := tt.AccessByRole(ui.RoleText, label)
				if !ok || info.Bounds.W <= 0 || info.Bounds.H != portrait.Sizes.Compact {
					t.Fatalf("badge accessibility or size: %+v, found %v", info, ok)
				}
				if info.Bounds.W > 180 {
					t.Fatalf("long CJK badge escaped its width constraint: %+v", info.Bounds)
				}
			})
		}
	}
}

func TestBadgesPaintDistinctToneSoftBackgrounds(t *testing.T) {
	tones := []struct {
		name string
		tone portrait.Tone
	}{
		{"neutral", portrait.Tones.Neutral},
		{"accent", portrait.Tones.Accent},
		{"success", portrait.Tones.Success},
		{"warning", portrait.Tones.Warning},
		{"danger", portrait.Tones.Danger},
	}
	seen := map[[4]uint32]string{}
	for _, tone := range tones {
		tt := ui.NewTester(onChrome(func(c *ui.Context) { portrait.Badge(c, "Status", tone.tone) }), 200, 120)
		tt.Frame()
		info, _ := tt.AccessByRole(ui.RoleText, "Status")
		r, g, b, a := tt.Image().At(int(info.Bounds.X+3), int(info.Bounds.Y+info.Bounds.H/2)).RGBA()
		sample := [4]uint32{r, g, b, a}
		if previous, exists := seen[sample]; exists {
			t.Errorf("%s and %s have the same Soft background sample %v", previous, tone.name, sample)
		}
		seen[sample] = tone.name
	}
}

func TestBadgeLeadingIconIsDecorative(t *testing.T) {
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.BadgeLeadingIcon(c, portrait.IconCheckCircle, "Ready", portrait.Tones.Accent)
	}), 200, 120)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleText, "Ready"); !ok {
		t.Fatal("badge text is not its accessible name")
	}
	for _, info := range tt.AccessAll() {
		if strings.Contains(info.Name+info.Value+info.Description, portrait.IconCheckCircle.Glyph) {
			t.Fatalf("leading icon leaked into accessibility: %+v", info)
		}
	}
}
