package nativeeditor

import (
	"slices"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// cellGuard begins the text a table cell's content is parsed in. GFM reads a
// cell's content as inline Markdown alone, while the block parser reads a
// leading '#' or '-' as a heading, a list or a quote a cell never carries: a
// no-break space starts no block and joins no delimiter run, and takes no
// room in the cell, which shows the offsets after it as its own.
const cellGuard = "\u00a0"

// newTableCell returns the document a table cell edits its content in.
func newTableCell(text string) *Document {
	d := New(text)
	d.inlineOnly = true
	return d
}

// buildInline presents a table cell's content as inline Markdown only: its
// marks hidden, its blocks never built, so that '# x' and '- x' read as
// written rather than as a heading or a list.
func (p *previewLayout) buildInline(buffer *editor.Buffer) {
	if p.document == nil {
		p.document = parser.New(parser.Default | parser.ExtDollarMath)
	}
	p.version = buffer.Version()
	p.dirty = false
	p.draftAnchored = false
	p.foldUntil = 0
	p.hiddenRanges = p.hiddenRanges[:0]
	p.sections = p.sections[:0]
	p.headings = map[string]int{}
	p.refs = map[string][]int{}
	p.readable = make(map[int]string)
	p.replacements = p.replacements[:0]
	p.styles = p.styles[:0]
	p.backgrounds = p.backgrounds[:0]
	p.blocks = p.blocks[:0]
	snap := buffer.Snapshot()
	for line := range snap.LineCount() {
		guarded := cellGuard + snap.Line(line)
		p.document.Set(guarded)
		p.shift = snap.LineStart(line) - len(cellGuard)
		for i := 1; i < p.document.Len(); i++ {
			b := p.document.Block(i)
			if b.Kind != parser.Paragraph {
				continue
			}
			p.inlineContent(guarded, b, p.document.Inlines(i))
		}
	}
	p.shift = 0
	slices.SortStableFunc(p.replacements, func(a, b editor.Replacement) int { return a.Start - b.Start })
	p.indexDisplay(snap.String())
}
