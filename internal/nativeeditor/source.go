package nativeeditor

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type Document struct {
	state editor.State
}

func New(source string) *Document {
	buffer := editor.NewBuffer(source)
	buffer.SetLanguage(editor.LanguageNamed("Markdown"))
	return &Document{state: editor.State{
		Buffer: buffer, Label: "Markdown source", SoftWrap: true,
		NoAutoClose: true, NoIndentGuides: true, NoScrollPastEnd: true,
	}}
}

func (d *Document) Source() string {
	var out strings.Builder
	_, _ = d.state.Buffer.WriteTo(&out)
	return out.String()
}

func (d *Document) Reload(source string) { d.state.Buffer.Reset(source) }
func (d *Document) Version() uint64      { return d.state.Buffer.Version() }
func (d *Document) CanUndo() bool        { return d.state.Buffer.CanUndo() }
func (d *Document) CanRedo() bool        { return d.state.Buffer.CanRedo() }
func (d *Document) Undo() bool           { return d.state.Run(editor.Undo) }
func (d *Document) Redo() bool           { return d.state.Run(editor.Redo) }

func (d *Document) View(c *ui.Context, readOnly bool) ui.Element {
	d.state.ReadOnly = readOnly
	return editor.View(c, &d.state)
}
