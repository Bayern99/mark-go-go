package nativeeditor

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type Mode string

const (
	Preview  Mode = "preview"
	Markdown Mode = "markdown"
)

var (
	ErrRevision = errors.New("the document changed since the edit was prepared")
	ErrExpected = errors.New("the source at the requested range differs from the expected text")
	ErrReadOnly = errors.New("the document is read only")
)

type Document struct {
	state   editor.State
	preview previewLayout
	mode    Mode
	index   *sourceIndex
}

func New(source string) *Document {
	buffer := editor.NewSourceBuffer(source)
	buffer.SetLanguage(editor.LanguageNamed("Markdown"))
	return &Document{mode: Preview, state: editor.State{
		Buffer: buffer, Label: "Markdown Preview", SoftWrap: true,
		NoAutoClose: true, NoIndentGuides: true, NoScrollPastEnd: true,
	}}
}

func (d *Document) sourceIndex() *sourceIndex {
	snap := d.state.Buffer.Snapshot()
	if d.index == nil || d.index.version != snap.Version() {
		var out strings.Builder
		snap.WriteTo(&out)
		d.index = indexSource(out.String(), snap.Version())
	}
	return d.index
}

func (d *Document) Source() string { return d.sourceIndex().raw }

func (d *Document) Apply(revision uint64, start, end int, expected, replacement string) error {
	if d.state.ReadOnly {
		return ErrReadOnly
	}
	if revision != d.Version() {
		return ErrRevision
	}
	idx := d.sourceIndex()
	if idx.version != revision {
		return ErrRevision
	}
	raw := idx.raw
	if start < 0 || end < start || end > len(raw) || start < len(raw) && !utf8.RuneStart(raw[start]) || end < len(raw) && !utf8.RuneStart(raw[end]) || !utf8.ValidString(replacement) {
		return ErrPosition
	}
	if raw[start:end] != expected {
		return ErrExpected
	}
	if idx.insideCRLF(start) {
		start--
		replacement = "\r" + replacement
	}
	if idx.insideCRLF(end) {
		end++
		replacement += "\n"
	}
	if !d.state.Buffer.ReplaceAtVersion(revision, idx.editorAtRaw(start), idx.editorAtRaw(end), replacement) {
		return ErrRevision
	}
	return nil
}

func (d *Document) SelectUTF16(anchor, head int) error {
	idx := d.sourceIndex()
	a, err := idx.rawAtUTF16(anchor)
	if err != nil {
		return err
	}
	z, err := idx.rawAtUTF16(head)
	if err != nil {
		return err
	}
	d.state.Select(idx.editorAtRaw(a), idx.editorAtRaw(z))
	return nil
}

func (d *Document) SelectionUTF16() (anchor, head int) {
	s := d.state.Selection()
	idx := d.sourceIndex()
	return idx.utf16AtEditor(s.Anchor), idx.utf16AtEditor(s.Head)
}

func (d *Document) Mode() Mode { return d.mode }
func (d *Document) SetMode(mode Mode) bool {
	if (mode != Preview && mode != Markdown) || mode == d.mode {
		return false
	}
	d.state.Buffer.EndTyping()
	d.mode = mode
	return true
}

func (d *Document) Reload(source string) { d.state.Buffer.Reset(source) }
func (d *Document) Version() uint64      { return d.state.Buffer.Version() }
func (d *Document) CanUndo() bool        { return !d.state.ReadOnly && d.state.Buffer.CanUndo() }
func (d *Document) CanRedo() bool        { return !d.state.ReadOnly && d.state.Buffer.CanRedo() }
func (d *Document) Undo() bool           { return d.CanUndo() && d.state.Run(editor.Undo) }
func (d *Document) Redo() bool           { return d.CanRedo() && d.state.Run(editor.Redo) }
func (d *Document) Modified() bool       { return d.state.Buffer.Modified() }
func (d *Document) MarkSaved()           { d.state.Buffer.MarkSaved() }
func (d *Document) Focused() bool        { return d.state.Focused() }
func (d *Document) ScrollY() float32     { return float32(d.state.ScrollY()) }
func (d *Document) SetScrollY(y float32) { d.state.SetScrollY(float64(y)) }

func (d *Document) View(c *ui.Context, readOnly bool) ui.Element {
	d.state.ReadOnly = readOnly
	d.prepareView()
	return editor.View(c, &d.state)
}

func (d *Document) RectUTF16(pos int) (ui.Rect, bool) {
	idx := d.sourceIndex()
	raw, err := idx.rawAtUTF16(pos)
	if err != nil {
		return ui.Rect{}, false
	}
	return d.state.Rect(idx.editorAtRaw(raw))
}
