package portraitgallery

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const Confirm Component = "Confirm"

func init() {
	register(Definition{Name: Confirm, Role: ui.RoleButton, Label: "Delete…", LongLabel: "删除会话并清除全部消息", build: confirmCell})
}

func confirmCell(c *ui.Context, cell Cell) ui.Element {
	label := labelFor(cell)
	owner := ui.Column(c)
	open := ui.Local(owner, "confirm open", func() bool { return false })
	trigger := ui.Local(owner, "confirm trigger", func() ui.Handle { return ui.Handle{} })
	button := controlState(portrait.Button(c, label, label).Bind(trigger).MaxWidth(cellWidth-24), cell)
	if button.Clicked() {
		*open = true
	}
	portrait.Confirm(c, open, portrait.ConfirmSpec{
		Title:        "Delete conversation?",
		Body:         "This action cannot be undone.",
		ConfirmLabel: "Delete",
		CancelLabel:  "Cancel",
		Return:       trigger,
	})
	return owner
}
