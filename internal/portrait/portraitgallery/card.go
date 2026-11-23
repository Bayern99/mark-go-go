package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Card Component = "Card"

func init() {
	register(Definition{Name: Card, Role: ui.RoleGroup, Label: "Document summary", LongLabel: "文档内容摘要", build: cardCell})
}

func cardCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	card := portrait.Card(c, func() {
		portrait.Type.UI.Apply(ui.Text(c, label)).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…")
	}).Size(112, 48).Padding(8).Role(ui.RoleGroup).Label(label)
	if cell.State == FocusVisible {
		card.Focusable()
	}
	return controlState(card, cell)
}
