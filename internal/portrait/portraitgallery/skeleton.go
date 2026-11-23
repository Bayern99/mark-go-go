package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Skeleton Component = "Skeleton"

func init() {
	register(Definition{Name: Skeleton, Role: ui.RoleProgress, Label: "Loading content", LongLabel: "正在加载文档内容", build: skeletonCell})
}

func skeletonCell(c *ui.Context, cell Cell) ui.Element {
	skeleton := portrait.DefaultSkeleton.View(c, labelFor(cell)).Width(112)
	if cell.State == FocusVisible {
		skeleton.Focusable()
	}
	return controlState(skeleton, cell)
}
