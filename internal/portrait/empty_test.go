package portrait_test

import (
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestEmptyStacksContentAndRunsItsAction(t *testing.T) {
	clicked := false
	tt := ui.NewTester(onChrome(func(c *ui.Context) {
		portrait.Empty(c, portrait.EmptySpec{
			Icon:        portrait.IconCube,
			Title:       "No checkpoints",
			Description: "Save a version to compare or restore it later.",
			Action: func(c *ui.Context) ui.Element {
				button := portrait.QuietButton(c, "Create checkpoint", "Create checkpoint")
				if button.Clicked() {
					clicked = true
				}
				return button
			},
		}).Width(240)
	}), 320, 240)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleGroup, "No checkpoints")
	if !ok || info.Description != "Save a version to compare or restore it later." {
		t.Fatalf("empty state accessibility: %+v, found %v", info, ok)
	}
	for _, text := range []string{"No checkpoints", "Save a version to compare or restore it later."} {
		if bounds, found := tt.Find(text); !found || bounds.W <= 0 || bounds.H <= 0 {
			t.Fatalf("empty state text %q is missing: %+v, found %v", text, bounds, found)
		}
	}
	if err := tt.Click("Create checkpoint"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("empty state action did not run")
	}
	for _, access := range tt.AccessAll() {
		if strings.Contains(access.Name+access.Value+access.Description, portrait.IconCube.Glyph) {
			t.Fatalf("empty state icon leaked into accessibility: %+v", access)
		}
	}
}

func TestEmptyLongCJKAcrossPreferences(t *testing.T) {
	preferences := []ui.Preferences{{}, {HighContrast: true}, {ReduceTransparency: true}}
	for _, preference := range preferences {
		tt := ui.NewTesterWith(onChrome(func(c *ui.Context) {
			portrait.Empty(c, portrait.EmptySpec{
				Icon:        portrait.IconFileText,
				Title:       "这份文档还没有任何历史检查点",
				Description: "创建检查点后，可以在这里比较或恢复先前保存的版本。",
			}).Width(200)
		}), 280, 220, ui.TesterOptions{Preferences: preference})
		tt.Frame()
		if info, ok := tt.AccessByRole(ui.RoleGroup, "这份文档还没有任何历史检查点"); !ok || info.Bounds.W > 200 || info.Bounds.H <= 0 {
			t.Fatalf("long CJK empty state with %+v: %+v, found %v", preference, info, ok)
		}
		if bounds, found := tt.Find("创建检查点后，可以在这里比较或恢复先前保存的版本。"); !found || bounds.W > 200 || bounds.H <= 16 {
			t.Fatalf("wrapped CJK description with %+v: %+v, found %v", preference, bounds, found)
		}
	}
}
