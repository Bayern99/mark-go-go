package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Track Component = "Track"

func init() {
	register(Definition{Name: Track, Role: ui.RoleButton, Label: "Preview", LongLabel: "文档预览与源码编辑", build: trackCell})
}

func trackCell(c *ui.Context, cell Cell) ui.Element {
	first := labelFor(cell)
	second := "Markdown"
	if cell.State == LongCJK {
		second = "原始标记文本"
	}
	track := portrait.Track(c, "gallery", func(s *portrait.Segments) {
		controlState(s.Segment(c, first, first, cell.State != Selected).Width(48), cell)
		controlState(s.Segment(c, second, second, cell.State == Selected).Width(48), Cell{})
	}).Label("Document views")
	if cell.State == Disabled {
		track.Disabled(true)
	}
	return track
}
