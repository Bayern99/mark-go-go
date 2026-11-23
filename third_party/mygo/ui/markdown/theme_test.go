package markdown

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

const secondaryTextMarkdown = "1. First item\n\n```go\nfmt.Println(1)\n```\n\n![Image placeholder](missing.png)\n"

func TestSecondaryTextSizesPreserveDefaults(t *testing.T) {
	const body = float32(13)
	th := resolve(&Theme{}, DefaultTheme(&ui.Theme{FontSize: body}))
	for name, tc := range map[string]struct {
		got  float32
		want float32
	}{
		"code label":        {th.em(th.CodeLabelSize), body * 0.8},
		"list number":       {th.em(th.ListNumberSize), body},
		"image placeholder": {th.em(th.ImagePlaceholderSize), body * 0.875},
	} {
		if !sameFontSize(tc.got, tc.want) {
			t.Errorf("%s size = %v, want %v", name, tc.got, tc.want)
		}
	}
}

func TestSecondaryTextSizesResolveAndRender(t *testing.T) {
	const body = float32(13)
	const meta = float32(12)
	th := resolve(&Theme{
		FontSize:             body,
		CodeLabelSize:        meta / body,
		ListNumberSize:       meta / body,
		ImagePlaceholderSize: meta / body,
	}, DefaultTheme(&ui.Theme{FontSize: body}))
	for name, got := range map[string]float32{
		"code label":        th.em(th.CodeLabelSize),
		"list number":       th.em(th.ListNumberSize),
		"image placeholder": th.em(th.ImagePlaceholderSize),
	} {
		if !sameFontSize(got, meta) {
			t.Errorf("%s size = %v, want %v", name, got, meta)
		}
	}

	app := ui.NewTester(func(c *ui.Context) {
		View(c, secondaryTextMarkdown, &Options{Theme: &th})
	}, 500, 500)
	app.Frame()
	lineHeight := DefaultTheme(&ui.Theme{}).LineHeight
	for _, tc := range []struct {
		role ui.Role
		name string
		want float32
	}{
		{ui.RoleText, "go", meta * lineHeight},
		{ui.RoleButton, "Copy code", meta*lineHeight + body*0.4},
		{ui.RoleText, "1.", meta * lineHeight},
		{ui.RoleImage, "Image placeholder", meta*lineHeight + body},
	} {
		info, ok := app.AccessByRole(tc.role, tc.name)
		if !ok {
			t.Errorf("no %v named %q", tc.role, tc.name)
			continue
		}
		if !sameFontSize(info.Bounds.H, tc.want) {
			t.Errorf("%q height = %v, want %v for %vpx text", tc.name, info.Bounds.H, tc.want, meta)
		}
	}
}

func sameFontSize(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.0001
}
