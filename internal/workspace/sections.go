package workspace

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

type Section struct {
	ID         string `json:"id"`
	Heading    string `json:"heading"`
	Level      int    `json:"level"`
	Occurrence int    `json:"occurrence"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Hash       string `json:"hash"`
}

type Note struct {
	Text              string `json:"text"`
	Start             int    `json:"start"`
	End               int    `json:"end"`
	StartLine         int    `json:"startLine"`
	EndLine           int    `json:"endLine"`
	SectionID         string `json:"sectionId,omitempty"`
	SectionHeading    string `json:"sectionHeading,omitempty"`
	SectionOccurrence int    `json:"sectionOccurrence,omitempty"`
	cutStart          int
	startByte         int
	endByte           int
	cutStartByte      int
}

type parsedHeading struct {
	level      int
	text       string
	offset     int
	byteOffset int
	line       int
}

var (
	noteHeadRe    = regexp.MustCompile(`^[ \t]*<!--[ \t]*note(?:\s+[A-Za-z_][\w-]*="[^"]*")*\s*:`)
	noteUnescRe   = regexp.MustCompile(`--\\(\\*)(!?)>`)
	inlineHTMLRe  = regexp.MustCompile(`<[^>]+>`)
	inlineImageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	inlineLinkRe  = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	inlineRefRe   = regexp.MustCompile(`\[([^\]]+)\]\[[^\]]*\]`)
	htmlStartRe   = regexp.MustCompile(`^[ \t]{0,3}</?[A-Za-z!?]`)
)

func SectionHash(text string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	return fmt.Sprintf("fnv1a32:%08x", h.Sum32())
}

func NormalizeHeadingText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func unescapeNoteText(text string) string {
	return noteUnescRe.ReplaceAllString(text, `--$1$2>`)
}

func parseNoteBlock(raw string) (string, bool) {
	trimmed := strings.TrimRight(raw, "\r\n")
	trimmedRight := strings.TrimRight(trimmed, " \t")
	if !strings.HasSuffix(trimmedRight, "-->") {
		return "", false
	}
	inner := strings.TrimSuffix(trimmedRight, "-->")
	if strings.Contains(inner, "-->") {
		return "", false
	}
	loc := noteHeadRe.FindStringIndex(inner)
	if loc == nil {
		return "", false
	}
	return unescapeNoteText(strings.TrimSpace(inner[loc[1]:])), true
}

func stripInlineMarkdown(s string) string {
	s = inlineHTMLRe.ReplaceAllString(s, "")
	s = inlineImageRe.ReplaceAllString(s, "$1")
	s = inlineLinkRe.ReplaceAllString(s, "$1")
	s = inlineRefRe.ReplaceAllString(s, "$1")
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if ch == '`' {
			ticks := 0
			for i+ticks < len(s) && s[i+ticks] == '`' {
				ticks++
			}
			end := strings.Index(s[i+ticks:], strings.Repeat("`", ticks))
			if end >= 0 {
				b.WriteString(s[i+ticks : i+ticks+end])
				i += ticks + end + ticks
				continue
			}
		}
		if ch == '*' || ch == '_' || ch == '~' {
			i++
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return strings.TrimSpace(b.String())
}

type lineSpan struct {
	raw        string
	body       string
	startByte  int
	endByte    int
	bodyEnd    int
	startUTF16 int
	endUTF16   int
	bodyUTF16  int
	lineNo     int
}

func splitLineSpans(markdown string) ([]lineSpan, int) {
	spans := make([]lineSpan, 0, 32)
	byteAt := 0
	utfAt := 0
	lineNo := 1
	for byteAt < len(markdown) {
		nl := strings.IndexByte(markdown[byteAt:], '\n')
		var endByte int
		if nl < 0 {
			endByte = len(markdown)
		} else {
			endByte = byteAt + nl + 1
		}
		raw := markdown[byteAt:endByte]
		body := strings.TrimRight(raw, "\r\n")
		bodyEnd := byteAt + len(body)
		rawUTF16 := utf16Count(raw)
		bodyUTF16 := utf16Count(body)
		spans = append(spans, lineSpan{
			raw:        raw,
			body:       body,
			startByte:  byteAt,
			endByte:    endByte,
			bodyEnd:    bodyEnd,
			startUTF16: utfAt,
			endUTF16:   utfAt + rawUTF16,
			bodyUTF16:  utfAt + bodyUTF16,
			lineNo:     lineNo,
		})
		byteAt = endByte
		utfAt += rawUTF16
		lineNo++
	}
	return spans, utfAt
}

func parseMarkdownBlocks(markdown string) ([]parsedHeading, []Note, []int, int) {
	spans, totalUTF16 := splitLineSpans(markdown)
	lineStarts := make([]int, len(spans)+1)
	for i, sp := range spans {
		lineStarts[i] = sp.startUTF16
	}
	lineStarts[len(spans)] = totalUTF16

	var headings []parsedHeading
	var notes []Note

	inFront := false
	frontChecked := false
	inFence := false
	var fenceMark byte
	inHTMLComment := false
	inHTMLTagBlock := false

	hasPrevBlock := false
	prevBlockEndUTF16 := 0
	prevBlockEndByte := 0

	hasPrevPara := false
	prevParaText := ""
	prevParaStartUTF16 := 0
	prevParaStartByte := 0
	prevParaLine := 0

	for i := 0; i < len(spans); i++ {
		sp := spans[i]
		trimmed := strings.TrimSpace(sp.body)

		if !frontChecked {
			frontChecked = true
			if trimmed == "---" {
				inFront = true
				hasPrevPara = false
				continue
			}
		}
		if inFront {
			if trimmed == "---" || trimmed == "..." {
				inFront = false
				hasPrevBlock = true
				prevBlockEndUTF16 = sp.bodyUTF16
				prevBlockEndByte = sp.bodyEnd
			}
			continue
		}

		if inFence {
			if fenceClosed(sp.body, fenceMark) {
				inFence = false
				hasPrevBlock = true
				prevBlockEndUTF16 = sp.bodyUTF16
				prevBlockEndByte = sp.bodyEnd
			}
			continue
		}
		if ok, mark := isFenceOpen(sp.body); ok {
			inFence = true
			fenceMark = mark
			hasPrevPara = false
			continue
		}

		if inHTMLComment {
			if strings.Contains(sp.body, "-->") {
				inHTMLComment = false
				hasPrevBlock = true
				prevBlockEndUTF16 = sp.bodyUTF16
				prevBlockEndByte = sp.bodyEnd
			}
			continue
		}
		if inHTMLTagBlock {
			if trimmed == "" {
				inHTMLTagBlock = false
			} else {
				hasPrevBlock = true
				prevBlockEndUTF16 = sp.bodyUTF16
				prevBlockEndByte = sp.bodyEnd
			}
			continue
		}

		if strings.HasPrefix(trimmed, "<!--") {
			commentStart := i
			commentEnd := i
			for commentEnd < len(spans) && !strings.Contains(spans[commentEnd].body, "-->") {
				commentEnd++
			}
			if commentEnd < len(spans) {
				blockRaw := markdown[spans[commentStart].startByte:spans[commentEnd].bodyEnd]
				if noteText, ok := parseNoteBlock(blockRaw); ok {
					noteStartUTF16 := spans[commentStart].startUTF16
					noteStartByte := spans[commentStart].startByte
					noteEndUTF16 := spans[commentEnd].bodyUTF16
					noteEndByte := spans[commentEnd].bodyEnd
					cutStartUTF16 := noteStartUTF16
					cutStartByte := noteStartByte
					if hasPrevBlock {
						cutStartUTF16 = prevBlockEndUTF16
						cutStartByte = prevBlockEndByte
					}
					notes = append(notes, Note{
						Text:         noteText,
						Start:        noteStartUTF16,
						End:          noteEndUTF16,
						StartLine:    spans[commentStart].lineNo,
						EndLine:      spans[commentEnd].lineNo,
						cutStart:     cutStartUTF16,
						startByte:    noteStartByte,
						endByte:      noteEndByte,
						cutStartByte: cutStartByte,
					})
					hasPrevBlock = true
					prevBlockEndUTF16 = noteEndUTF16
					prevBlockEndByte = noteEndByte
					hasPrevPara = false
					i = commentEnd
					continue
				}
				hasPrevBlock = true
				prevBlockEndUTF16 = spans[commentEnd].bodyUTF16
				prevBlockEndByte = spans[commentEnd].bodyEnd
				hasPrevPara = false
				i = commentEnd
				continue
			}
			inHTMLComment = true
			hasPrevPara = false
			continue
		}

		if htmlStartRe.MatchString(sp.body) {
			inHTMLTagBlock = true
			hasPrevBlock = true
			prevBlockEndUTF16 = sp.bodyUTF16
			prevBlockEndByte = sp.bodyEnd
			hasPrevPara = false
			continue
		}

		if level, heading, ok := atxHeading(sp.body); ok {
			headings = append(headings, parsedHeading{
				level:      level,
				text:       stripInlineMarkdown(heading),
				offset:     sp.startUTF16,
				byteOffset: sp.startByte,
				line:       sp.lineNo,
			})
			hasPrevBlock = true
			prevBlockEndUTF16 = sp.bodyUTF16
			prevBlockEndByte = sp.bodyEnd
			hasPrevPara = false
			continue
		}

		if lvl := setextLevel(sp.body); lvl > 0 && hasPrevPara {
			headings = append(headings, parsedHeading{
				level:      lvl,
				text:       stripInlineMarkdown(prevParaText),
				offset:     prevParaStartUTF16,
				byteOffset: prevParaStartByte,
				line:       prevParaLine,
			})
			hasPrevBlock = true
			prevBlockEndUTF16 = sp.bodyUTF16
			prevBlockEndByte = sp.bodyEnd
			hasPrevPara = false
			continue
		}

		if trimmed == "" {
			hasPrevPara = false
			continue
		}

		if para := paragraphText(sp.body); para != "" {
			hasPrevPara = true
			prevParaText = para
			prevParaStartUTF16 = sp.startUTF16
			prevParaStartByte = sp.startByte
			prevParaLine = sp.lineNo
		} else {
			hasPrevPara = false
		}
		hasPrevBlock = true
		prevBlockEndUTF16 = sp.bodyUTF16
		prevBlockEndByte = sp.bodyEnd
	}

	return headings, notes, lineStarts, totalUTF16
}

func lineForUTF16Offset(lineStarts []int, offset int) int {
	if len(lineStarts) <= 1 {
		return 1
	}
	lo, hi := 0, len(lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if offset > 0 && lo > 0 && lineStarts[lo] == offset {
		return lo
	}
	return lo + 1
}

func ExtractSections(markdown string) []Section {
	headings, notes, lineStarts, totalUTF16 := parseMarkdownBlocks(markdown)
	if len(headings) == 0 {
		return []Section{}
	}
	seen := make(map[string]int, len(headings))
	out := make([]Section, len(headings))
	for i, h := range headings {
		key := NormalizeHeadingText(h.text)
		occ := seen[key] + 1
		seen[key] = occ

		endUTF16 := totalUTF16
		endByte := len(markdown)
		for _, next := range headings[i+1:] {
			if next.level <= h.level {
				endUTF16 = next.offset
				endByte = next.byteOffset
				break
			}
		}

		hasher := fnv.New32a()
		at := h.byteOffset
		for _, n := range notes {
			if n.Start >= h.offset && n.End <= endUTF16 {
				if n.cutStartByte > at {
					_, _ = hasher.Write([]byte(markdown[at:n.cutStartByte]))
				}
				if n.endByte > at {
					at = n.endByte
				}
			}
		}
		if endByte > at {
			_, _ = hasher.Write([]byte(markdown[at:endByte]))
		}
		hash := fmt.Sprintf("fnv1a32:%08x", hasher.Sum32())

		endLine := lineForUTF16Offset(lineStarts, endUTF16)
		if endLine < h.line {
			endLine = h.line
		}

		out[i] = Section{
			ID:         hash,
			Heading:    h.text,
			Level:      h.level,
			Occurrence: occ,
			Start:      h.offset,
			End:        endUTF16,
			StartLine:  h.line,
			EndLine:    endLine,
			Hash:       hash,
		}
	}
	return out
}

func ExtractNotes(markdown string) []Note {
	_, notes, _, _ := parseMarkdownBlocks(markdown)
	if len(notes) == 0 {
		return []Note{}
	}
	sections := ExtractSections(markdown)
	out := make([]Note, len(notes))
	for i, n := range notes {
		for _, sec := range sections {
			if n.Start >= sec.Start && n.End <= sec.End {
				n.SectionID = sec.ID
				n.SectionHeading = sec.Heading
				n.SectionOccurrence = sec.Occurrence
			}
		}
		out[i] = n
	}
	return out
}

func GenerationSections(text string) []genui.SectionRef {
	sections := ExtractSections(text)
	notes := ExtractNotes(text)
	refs := make([]genui.SectionRef, 0, len(sections))
	for _, s := range sections {
		ref := genui.SectionRef{Heading: s.Heading, Occurrence: s.Occurrence, Start: s.Start, End: s.End}
		for _, n := range notes {
			if n.Start >= s.Start && n.End <= s.End {
				ref.Exclude = append(ref.Exclude, genui.TextRange{Start: n.cutStart, End: n.End})
			}
		}
		refs = append(refs, ref)
	}
	return refs
}
