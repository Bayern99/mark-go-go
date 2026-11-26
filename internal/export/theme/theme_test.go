package theme

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestCSSFreezesEverySpaceAndPrintsLight(t *testing.T) {
	for _, space := range portrait.Spaces {
		for _, dark := range []bool{false, true} {
			css := CSS(export.Options{SpaceName: space.Name, Dark: dark, PrintLight: true})
			for _, want := range []string{
				"--paper:" + Color(space.In(dark).Paper),
				"--accent:" + Color(space.In(dark).Accent),
				"@media print{:root{color-scheme:light;",
				"--paper:" + Color(space.Light.Paper),
				"--accent:" + Color(space.Light.Accent),
				"--chart-6:", ".tok-keyword", "break-inside:avoid", "break-after:avoid",
			} {
				if !strings.Contains(css, want) {
					t.Errorf("%s dark=%v CSS is missing %q", space.Name, dark, want)
				}
			}
			if strings.Contains(css, "prefers-color-scheme") {
				t.Errorf("%s dark=%v follows the system appearance", space.Name, dark)
			}
		}
	}
}

func TestPrintLightCanKeepCurrentAppearance(t *testing.T) {
	css := CSS(export.Options{SpaceName: "Water", Dark: true})
	if strings.Contains(css, "@media print{:root") {
		t.Fatal("print appearance was overridden while PrintLight is off")
	}
	if !strings.Contains(css, "@media print{") {
		t.Fatal("print pagination rules are missing")
	}
}

func TestColorSerialization(t *testing.T) {
	if got := Color(ui.Hex("#A1B2C3")); got != "#a1b2c3" {
		t.Fatalf("opaque color = %q", got)
	}
	if got := Color(ui.RGBA(1, 2, 3, 0.5)); got != "rgba(1, 2, 3, 0.502)" {
		t.Fatalf("translucent color = %q", got)
	}
	if got := NamedColor("chart4"); got != "var(--chart-4)" {
		t.Fatalf("chart color = %q", got)
	}
	if got := NamedColor("red;behavior:url(x)"); got != "" {
		t.Fatalf("unsafe color = %q", got)
	}
}
