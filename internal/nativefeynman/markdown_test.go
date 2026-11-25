package nativefeynman

import (
	"math"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func TestFeynmanMarkdownSecondaryTextUsesPortraitMeta(t *testing.T) {
	o := markdownOptions(nil, &markdown.Options{})
	body := portrait.Type.UI.Size
	meta := portrait.Type.Meta.Size
	for name, got := range map[string]float32{
		"code label":        o.Theme.CodeLabelSize * body,
		"list number":       o.Theme.ListNumberSize * body,
		"image placeholder": o.Theme.ImagePlaceholderSize * body,
	} {
		if !sameMarkdownSize(got, meta) {
			t.Errorf("%s size = %v, want Portrait meta %v", name, got, meta)
		}
	}

	const src = "1. First item\n\n```go\nfmt.Println(1)\n```\n\n![Image placeholder](missing.png)\n"
	app := ui.NewTester(func(c *ui.Context) {
		markdown.View(c, src, o)
	}, 500, 500)
	app.Frame()
	lineHeight := markdown.DefaultTheme(&ui.Theme{}).LineHeight
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
		if !sameMarkdownSize(info.Bounds.H, tc.want) {
			t.Errorf("%q height = %v, want %v for %vpx text", tc.name, info.Bounds.H, tc.want, meta)
		}
	}
}

func sameMarkdownSize(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.0001
}
