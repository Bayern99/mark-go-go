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
	preferences      readingPreferences
	state            editor.State
	preview          previewLayout
	mode             Mode
	index            *sourceIndex
	tables           map[int]*readingTable
	draft            *blockDraft
	artwork          map[string]blockImage
	imageDir         string
	images           *inlineImages
	refocus          bool
	pendingScrollY   float32
	hasPendingScroll bool
	folded           map[string]bool
	returns          map[string]int
}

func New(source string) *Document {
	buffer := editor.NewSourceBuffer(source)
	buffer.SetLanguage(editor.LanguageNamed("Markdown"))
	return &Document{mode: Preview, images: &inlineImages{}, state: editor.State{
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
	return d.apply(revision, start, end, expected, replacement, nil)
}

func (d *Document) apply(revision uint64, start, end int, expected, replacement string, owner any) error {
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
	if end < start || !validByteBoundary(raw, start) || !validByteBoundary(raw, end) || !utf8.ValidString(replacement) {
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
	var applied bool
	if owner != nil {
		applied = d.state.Buffer.ReplaceCompositionAtVersion(revision, idx.editorAtRaw(start), idx.editorAtRaw(end), replacement, owner)
	} else {
		applied = d.state.Buffer.ReplaceAtVersion(revision, idx.editorAtRaw(start), idx.editorAtRaw(end), replacement)
	}
	if !applied {
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
	if d.mode == Preview {
		for _, section := range d.preview.sections {
			if d.folded[section.key] && ((idx.editorAtRaw(a) >= section.body && idx.editorAtRaw(a) < section.end) || (idx.editorAtRaw(z) >= section.body && idx.editorAtRaw(z) < section.end)) {
				d.preview.toggleFold(section.key)
			}
		}
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
	d.draft = nil
	transferred := false
	if d.mode == Preview && mode == Markdown {
		for _, table := range d.tables {
			for child, cell := range table.composing {
				source := child.Source()
				if child.state.TransferComposition(&d.state, func(off int) int {
					return cell.start + cellRawOffset(source, off)
				}) {
					transferred = true
					delete(table.composing, child)
				}
			}
		}
	}
	if !transferred {
		d.state.Buffer.EndTyping()
	}
	d.mode = mode
	return true
}

func (d *Document) Reload(source string) {
	d.state.Buffer.Reset(source)
	d.retryFailedInlineImages()
}
func (d *Document) Version() uint64 { return d.state.Buffer.Version() }
func (d *Document) CanUndo() bool   { return !d.state.ReadOnly && d.state.Buffer.CanUndo() }
func (d *Document) CanRedo() bool   { return !d.state.ReadOnly && d.state.Buffer.CanRedo() }
func (d *Document) Undo() bool      { return d.CanUndo() && d.state.Run(editor.Undo) }
func (d *Document) Redo() bool      { return d.CanRedo() && d.state.Run(editor.Redo) }
func (d *Document) Modified() bool  { return d.state.Buffer.Modified() }
func (d *Document) MarkSaved()      { d.state.Buffer.MarkSaved() }
func (d *Document) Focused() bool   { return d.state.Focused() }
func (d *Document) ScrollY() float32 {
	if d.hasPendingScroll {
		return d.pendingScrollY
	}
	return float32(d.state.ScrollY())
}

func (d *Document) SetScrollY(y float32) {
	if y < 0 {
		y = 0
	}
	if d.state.ViewportHeight() <= 0 {
		if y == 0 && !d.hasPendingScroll {
			return
		}
		d.pendingScrollY = y
		d.hasPendingScroll = y > 0
		return
	}
	d.hasPendingScroll = false
	d.state.SetScrollY(float64(y))
}

func (d *Document) View(c *ui.Context, readOnly bool) ui.Element {
	d.state.ReadOnly = readOnly
	d.prepareView()
	d.state.Theme = documentTheme(c.Theme(), d.mode)
	if d.hasPendingScroll && d.state.ViewportHeight() > 0 {
		d.hasPendingScroll = false
		d.state.SetScrollY(float64(d.pendingScrollY))
	}
	field := editor.View(c, &d.state)
	if d.hasPendingScroll {
		c.Invalidate()
	}
	if d.refocus {
		field.Focus()
		d.refocus = false
	}
	return field
}

func (d *Document) RectUTF16(pos int) (ui.Rect, bool) {
	idx := d.sourceIndex()
	raw, err := idx.rawAtUTF16(pos)
	if err != nil {
		return ui.Rect{}, false
	}
	return d.state.Rect(idx.editorAtRaw(raw))
}
