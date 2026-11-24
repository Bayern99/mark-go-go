package nativeeditor

import (
	"slices"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type markdownHighlighter struct {
	editor.Highlighter
	document  *parser.Document
	text      editor.Snapshot
	code      []editor.Range
	completed []editor.Range
	quotes    []editor.Range
}

func newMarkdownHighlighter(text editor.Snapshot, _ func()) editor.Highlighter {
	h := &markdownHighlighter{
		Highlighter: editor.NewTokenHighlighter(editor.LanguageNamed("Markdown").Tokenizer, text),
		document:    parser.New(parser.Default | parser.ExtDollarMath),
	}
	h.parse(text)
	return h
}

func (h *markdownHighlighter) parse(text editor.Snapshot) {
	h.text = text
	h.document.Set(text.String())
	h.code = h.code[:0]
	h.completed = h.completed[:0]
	h.quotes = h.quotes[:0]
	for i := 1; i < h.document.Len(); i++ {
		block := h.document.Block(i)
		if block.Task == parser.Checked {
			h.completed = append(h.completed, editor.Range{Start: block.Start, End: block.End})
		}
		if block.Kind == parser.Quote {
			h.quotes = append(h.quotes, editor.Range{Start: block.Start, End: block.End})
		}
		for _, in := range h.document.Inlines(i) {
			if in.Kind == parser.Code {
				h.code = append(h.code, editor.Range{Start: in.Start, End: in.End})
			}
		}
	}
	slices.SortFunc(h.code, func(a, b editor.Range) int { return a.Start - b.Start })
	slices.SortFunc(h.completed, func(a, b editor.Range) int { return a.Start - b.Start })
	slices.SortFunc(h.quotes, func(a, b editor.Range) int { return a.Start - b.Start })
}

func (h *markdownHighlighter) Edit(old, next editor.Snapshot, changes []editor.Change) {
	h.Highlighter.Edit(old, next, changes)
	h.parse(next)
}

func (h *markdownHighlighter) Tokens(line int, text string, dst []editor.Token) ([]editor.Token, bool) {
	start := len(dst)
	dst, final := h.Highlighter.Tokens(line, text, dst)
	offset := h.text.LineStart(line)
	base := slices.Clone(dst[start:])
	dst = dst[:start]
	pos := 0
	for _, token := range base {
		for pos < token.End {
			at := offset + pos
			end, kind := token.End, token.Kind
			apply := func(ranges []editor.Range, tokenKind editor.TokenKind) {
				for _, span := range ranges {
					if span.End <= at {
						continue
					}
					if span.Start > at {
						end = min(end, span.Start-offset)
						break
					}
					if span.Start <= at && at < span.End {
						kind = tokenKind
						end = min(end, span.End-offset)
					}
					break
				}
			}
			apply(h.code, editor.TokenCode)
			apply(h.quotes, editor.TokenType)
			apply(h.completed, editor.TokenDeleted)
			dst = append(dst, editor.Token{End: max(pos+1, end), Kind: kind})
			pos = max(pos+1, end)
		}
	}
	return dst, final
}
