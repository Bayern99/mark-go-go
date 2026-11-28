package nativeeditor

import (
	"slices"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const cellGuard = "\u00a0"

func newInlineMarkdownHighlighter(text editor.Snapshot, _ func()) editor.Highlighter {
	base := editor.LanguageNamed("Markdown").Tokenizer
	guarded := editor.TokenizerFunc(func(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
		raw, next := base.Tokenize(cellGuard+line, state, nil)
		shift := len(cellGuard)
		for _, tok := range raw {
			if tok.End <= shift {
				continue
			}
			tokens = append(tokens, editor.Token{End: tok.End - shift, Kind: tok.Kind})
		}
		return tokens, next
	})
	return editor.NewTokenHighlighter(guarded, text)
}

func newTableCell(text string) *Document {
	d := New(text)
	d.inlineOnly = true
	language := *editor.LanguageNamed("Markdown")
	language.Highlighter = newInlineMarkdownHighlighter
	d.state.Buffer.SetLanguage(&language)
	return d
}

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
