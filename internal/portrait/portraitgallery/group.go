package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Group Component = "Group"

func init() {
	register(Definition{Name: Group, Role: ui.RoleButton, Label: "Accept", LongLabel: "接受此段全部更改", build: groupCell})
}

func groupCell(c *ui.Context, cell Cell) ui.Element {
	first := labelFor(cell)
	second := "Reject"
	if cell.State == LongCJK {
		second = "拒绝"
	}
	group := portrait.Group(c, "Hunk actions", func(g *portrait.GroupBuilder) {
		spec := portrait.ButtonSpec{Look: portrait.Quiet, Loading: cell.State == Loading, Selected: cell.State == Selected}
		accept := portrait.ButtonWith(c, first, first, spec)
		controlState(accept, cell).MaxWidth(72).Shrink(1).MinWidth(0)
		g.Add(accept)
		g.Add(portrait.QuietButton(c, second, second)).MaxWidth(48).Shrink(1).MinWidth(0)
	})
	if cell.State == Disabled {
		group.Disabled(true)
	}
	return group
}
