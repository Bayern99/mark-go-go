package nativeeditor

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

type Editor struct {
	source   *editor.State
	raw      string
	version  uint64
	onChange func(string)
	mode     string
	readOnly bool
	parse    *parser.Document
	blocks   []block
	rich     *richBlock
	draft    *blockDraft
	anchor   int
	scrollTo int
	focus    bool
	options  *markdown.Options
	images   map[string]ui.ImageSource
	path     string
	folds    map[int]bool
	tables   map[string][]float32
}

type richBlock struct {
	at, end           int
	doc               *richtext.Document
	version           uint64
	original, written string
	base              richtext.Text
}

type blockDraft struct {
	block block
	value *editor.State
	kind  string
}

func New(text string, onChange func(string)) *Editor {
	e := &Editor{source: newSource(text), raw: text, onChange: onChange, mode: "preview", scrollTo: -1,
		parse: parser.New(parser.Default | parser.ExtDollarMath), images: make(map[string]ui.ImageSource), folds: make(map[int]bool), tables: make(map[string][]float32)}
	e.reparse()
	e.options = e.renderOptions()
	return e
}

func newSource(text string) *editor.State {
	buf := editor.NewBuffer(text)
	buf.SetLanguage(editor.LanguageForText("document.md", text))
	return &editor.State{Buffer: buf, Font: ui.Font{Family: "IBM Plex Mono", Size: 13}, LineHeight: 1.69, SoftWrap: true, NoIndentGuides: true,
		NoActiveLine: true, NoAutoClose: true, LineNumbers: editor.LineNumbersOff, Label: "Markdown source", Placeholder: "Start writing…"}
}

func rawText(buf *editor.Buffer) string {
	var b strings.Builder
	_, _ = buf.WriteTo(&b)
	return b.String()
}

func (e *Editor) sync() {
	if r := e.rich; r != nil && r.doc.Version() != r.version {
		r.version = r.doc.Version()
		value := r.doc.Text()
		next := r.original
		if !value.Equal(r.base) {
			next = value.Markdown()
		}
		if next != r.written && r.end <= len(e.raw) && e.raw[r.at:r.end] == r.written {
			start, end := r.at, r.end
			r.end = r.at + len(next)
			r.written = next
			e.replaceRaw(start, end, next)
		}
	}
	e.observe()
}

func (e *Editor) observe() {
	if v := e.source.Buffer.Version(); v != e.version {
		e.version = v
		next := rawText(e.source.Buffer)
		if next == e.raw {
			return
		}
		e.raw = next
		e.reparse()
		if e.onChange != nil {
			e.onChange(next)
		}
	}
}

func (e *Editor) Text() string { e.sync(); return e.raw }
func (e *Editor) Mode() string { return e.mode }
func (e *Editor) SetMode(mode string) {
	if mode == "source" {
		mode = "markdown"
	}
	if mode != "preview" && mode != "markdown" || mode == e.mode {
		return
	}
	e.sync()
	if e.mode == "markdown" {
		e.anchor = rawByte(e.raw, e.source.Cursor())
	}
	if e.rich != nil {
		e.anchor = e.rich.at
	}
	e.rich = nil
	e.draft = nil
	e.mode = mode
	if mode == "markdown" {
		at := bufferByte(e.raw, e.anchor)
		e.source.Select(at, at)
		e.focus = true
	} else {
		e.scrollTo = e.anchor
	}
}
func (e *Editor) SetReadOnly(value bool) {
	e.sync()
	if value {
		e.draft = nil
	}
	e.source.ReadOnly = value
	e.readOnly = value
}
func (e *Editor) Reload(text string) {
	e.rich = nil
	e.draft = nil
	e.source.Buffer.Reset(text)
	e.source.Select(0, 0)
	e.raw = text
	e.version = e.source.Buffer.Version()
	e.anchor = 0
	e.scrollTo = 0
	e.reparse()
}
func (e *Editor) Apply(expected, next string) bool {
	if e.readOnly || e.Text() != expected {
		return false
	}
	if expected == next {
		return true
	}
	e.rich = nil
	e.draft = nil
	a := 0
	for a < len(expected) && a < len(next) && expected[a] == next[a] {
		a++
	}
	for a > 0 && ((a < len(expected) && !utf8.RuneStart(expected[a])) || (a < len(next) && !utf8.RuneStart(next[a])) || splitCRLF(expected, a) || splitCRLF(next, a)) {
		a--
	}
	end, z := len(expected), len(next)
	for end > a && z > a && expected[end-1] == next[z-1] {
		end--
		z--
	}
	for end < len(expected) && z < len(next) && (!utf8.RuneStart(expected[end]) || !utf8.RuneStart(next[z]) || splitCRLF(expected, end) || splitCRLF(next, z)) {
		end++
		z++
	}
	e.replaceRaw(a, end, next[a:z])
	return true
}

func splitCRLF(s string, i int) bool { return i > 0 && i < len(s) && s[i-1] == '\r' && s[i] == '\n' }

func (e *Editor) replaceRaw(a, b int, text string) {
	next := e.raw[:a] + text + e.raw[b:]
	buf := e.source.Buffer
	first, last := bufferByte(e.raw, a), bufferByte(e.raw, b)
	buf.Transact(func() {
		buf.Edit(editor.Edit{Start: first, End: last, Text: text})
		if rawText(buf) != next {
			buf.Edit(editor.Edit{Start: 0, End: buf.Len()})
			buf.Edit(editor.Edit{Text: next})
		}
	})
	e.observe()
}

func (e *Editor) CanUndo() bool {
	e.sync()
	if e.readOnly {
		return false
	}
	if d := e.draft; d != nil {
		return d.value.Buffer.CanUndo()
	}
	return e.source.Buffer.CanUndo()
}
func (e *Editor) CanRedo() bool {
	e.sync()
	if e.readOnly {
		return false
	}
	if d := e.draft; d != nil {
		return d.value.Buffer.CanRedo()
	}
	return e.source.Buffer.CanRedo()
}
func (e *Editor) Undo() bool { return e.history(false) }
func (e *Editor) Redo() bool { return e.history(true) }
func (e *Editor) history(redo bool) bool {
	e.sync()
	if e.readOnly {
		return false
	}
	b := e.source.Buffer
	if d := e.draft; d != nil {
		b = d.value.Buffer
	} else {
		e.rich = nil
	}
	var ok bool
	if redo {
		ok = b.Redo()
	} else {
		ok = b.Undo()
	}
	e.observe()
	return ok
}

func byteAtUTF16(s string, units int) (int, bool) {
	if units < 0 {
		return 0, false
	}
	for pos, r := range s {
		if units == 0 {
			return pos, true
		}
		units -= utf16.RuneLen(r)
		if units < 0 {
			return 0, false
		}
	}
	return len(s), units == 0
}
func bufferByte(s string, pos int) int {
	pos = min(len(s), max(0, pos))
	return pos - strings.Count(s[:pos], "\r\n")
}
func rawByte(s string, norm int) int {
	pos := 0
	for pos < len(s) && norm > 0 {
		if splitCRLF(s, pos+1) {
			pos++
		}
		pos++
		norm--
	}
	return pos
}
func (e *Editor) Select(from, to int) bool {
	text := e.Text()
	a, ok := byteAtUTF16(text, from)
	if !ok {
		return false
	}
	b, ok := byteAtUTF16(text, to)
	if !ok || from > to {
		return false
	}
	e.anchor = a
	e.source.Select(bufferByte(text, a), bufferByte(text, b))
	if e.mode == "preview" {
		e.scrollTo = a
	} else {
		e.focus = true
	}
	return true
}
func (e *Editor) Reveal(offset int) bool { return e.Select(offset, offset) }
func (e *Editor) SetPath(path string) {
	if e.path == path {
		return
	}
	e.path = path
	e.options = e.renderOptions()
	e.loadLayout()
}
