package nativeeditor

import (
	"slices"
	"sort"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// quoteInk is the token that paints blockquote prose: a quote is quiet body
// text, not a caution, so it takes Ink2 rather than TokenType's Warn, as
// chat's blockquotes do. portrait.CodeTheme gives TokenRegexp that ink, and
// Markdown has no regexp to lose it to.
const quoteInk = editor.TokenRegexp

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
	cursors := [3]int{
		cursorAt(h.code, offset),
		cursorAt(h.quotes, offset),
		cursorAt(h.completed, offset),
	}
	// A line no range reaches only needs ATX heading-marker separation.
	if !reaches(h.code, cursors[0], offset, len(text)) &&
		!reaches(h.quotes, cursors[1], offset, len(text)) &&
		!reaches(h.completed, cursors[2], offset, len(text)) {
		return splitATXHeadingMarker(text, dst, start), final
	}
	base := slices.Clone(dst[start:])
	dst = dst[:start]
	pos := 0
	for _, token := range base {
		for pos < token.End {
			at := offset + pos
			end, kind := token.End, token.Kind
			apply := func(ranges []editor.Range, cursor *int, tokenKind editor.TokenKind) {
				for *cursor < len(ranges) && ranges[*cursor].End <= at {
					*cursor++
				}
				if *cursor >= len(ranges) {
					return
				}
				span := ranges[*cursor]
				if span.Start > at {
					end = min(end, span.Start-offset)
					return
				}
				kind = tokenKind
				end = min(end, span.End-offset)
			}
			apply(h.code, &cursors[0], editor.TokenCode)
			apply(h.quotes, &cursors[1], quoteInk)
			apply(h.completed, &cursors[2], editor.TokenDeleted)
			dst = append(dst, editor.Token{End: max(pos+1, end), Kind: kind})
			pos = max(pos+1, end)
		}
	}
	return splitATXHeadingMarker(text, dst, start), final
}

// splitATXHeadingMarker keeps the bold title apart from its leading hashes.
// In particular, adjacent hashes in IBM Plex Mono stay distinct in regular
// weight instead of fusing into the bold heading's ink.
func splitATXHeadingMarker(text string, dst []editor.Token, start int) []editor.Token {
	marker := atxHeadingMarkerEnd(text)
	if marker == 0 {
		return dst
	}
	tokenStart := 0
	for i := start; i < len(dst); i++ {
		token := dst[i]
		if token.End < marker {
			tokenStart = token.End
			continue
		}
		if token.Kind != editor.TokenHeading || tokenStart >= marker {
			return dst
		}
		if token.End == marker {
			dst[i].Kind = editor.TokenPunctuation
			return dst
		}
		out := make([]editor.Token, 0, len(dst)+1)
		out = append(out, dst[:i]...)
		out = append(out,
			editor.Token{End: marker, Kind: editor.TokenPunctuation},
			editor.Token{End: token.End, Kind: editor.TokenHeading},
		)
		out = append(out, dst[i+1:]...)
		return out
	}
	return dst
}

func atxHeadingMarkerEnd(text string) int {
	indent := 0
	for indent < len(text) && text[indent] == ' ' {
		indent++
	}
	if indent > 3 {
		return 0
	}
	end := indent
	for end < len(text) && text[end] == '#' {
		end++
	}
	if end == indent || end-indent > 6 {
		return 0
	}
	if end < len(text) && text[end] != ' ' && text[end] != '\t' {
		return 0
	}
	return end
}

// cursorAt is the first range that can still reach offset. The ranges are
// sorted, so a line's pass starts where the line does rather than at the
// beginning of the document.
func cursorAt(ranges []editor.Range, offset int) int {
	return sort.Search(len(ranges), func(i int) bool { return ranges[i].End > offset })
}

// reaches reports whether the range at cursor meets the line at offset.
func reaches(ranges []editor.Range, cursor, offset, length int) bool {
	return cursor < len(ranges) && ranges[cursor].Start < offset+length
}
