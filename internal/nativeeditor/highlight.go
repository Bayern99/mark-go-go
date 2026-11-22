package nativeeditor

import (
	"slices"
	"sort"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type markdownHighlighter struct {
	editor.Highlighter
	document *parser.Document
	text     editor.Snapshot
	code     []editor.Range
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
	for i := 1; i < h.document.Len(); i++ {
		for _, in := range h.document.Inlines(i) {
			if in.Kind == parser.Code {
				h.code = append(h.code, editor.Range{Start: in.Start, End: in.End})
			}
		}
	}
	slices.SortFunc(h.code, func(a, b editor.Range) int { return a.Start - b.Start })
}

func (h *markdownHighlighter) Edit(old, next editor.Snapshot, changes []editor.Change) {
	h.Highlighter.Edit(old, next, changes)
	h.parse(next)
}

func (h *markdownHighlighter) Tokens(line int, text string, dst []editor.Token) ([]editor.Token, bool) {
	start := len(dst)
	dst, final := h.Highlighter.Tokens(line, text, dst)
	offset := h.text.LineStart(line)
	i := sort.Search(len(h.code), func(i int) bool { return h.code[i].End > offset })
	if i == len(h.code) || h.code[i].Start >= offset+len(text) {
		return dst, final
	}
	base := slices.Clone(dst[start:])
	dst = dst[:start]
	pos := 0
	for _, token := range base {
		for pos < token.End {
			for i < len(h.code) && h.code[i].End <= offset+pos {
				i++
			}
			end, kind := token.End, token.Kind
			if i < len(h.code) {
				span := h.code[i]
				if span.Start <= offset+pos {
					end, kind = min(end, span.End-offset), editor.TokenCode
				} else {
					end = min(end, span.Start-offset)
				}
			}
			dst = append(dst, editor.Token{End: end, Kind: kind})
			pos = end
		}
	}
	return dst, final
}
