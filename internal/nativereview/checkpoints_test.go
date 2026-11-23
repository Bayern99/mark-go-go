package nativereview

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCheckpointEmptyUsesPortraitEmpty(t *testing.T) {
	const (
		title       = "这份文档还没有任何历史检查点"
		description = "创建检查点后，可以在这里比较或恢复先前保存的版本。"
	)
	for _, preference := range []ui.Preferences{{}, {HighContrast: true}, {ReduceTransparency: true}} {
		tt := ui.NewTesterWith(func(c *ui.Context) {
			checkpointEmpty(c, title, description).Width(220)
		}, 260, 180, ui.TesterOptions{Preferences: preference})
		tt.Frame()
		info, ok := tt.AccessByRole(ui.RoleGroup, title)
		if !ok || info.Description != description || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
			t.Fatalf("checkpoint empty state with %+v: %+v, found %v", preference, info, ok)
		}
		if bounds, found := tt.Find(description); !found || bounds.W > 220 || bounds.H <= 16 {
			t.Fatalf("checkpoint empty description did not wrap with %+v: %+v, found %v", preference, bounds, found)
		}
	}
}
