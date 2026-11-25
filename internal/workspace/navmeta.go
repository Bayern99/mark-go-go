package workspace

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/markdown"
)

type NavHeading struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Level  int    `json:"level"`
}

type mdMeta struct {
	Title    string
	Excerpt  string
	Notes    int
	Headings []NavHeading
}

func clipExcerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > 88 {
		return string(runes[:88]) + "…"
	}
	return s
}

func parseMarkdownMeta(text string) mdMeta {
	var meta mdMeta
	inFence := false
	var fenceMark byte
	inFront := false
	frontChecked := false
	utfAt := 0
	var prev string
	prevOff := 0
	prevBlank := true

	for len(text) > 0 {
		nl := strings.IndexByte(text, '\n')
		var line string
		var consumed int
		if nl < 0 {
			line = text
			consumed = len(text)
		} else {
			line = text[:nl+1]
			consumed = nl + 1
		}
		lineOff := utfAt
		utfAt += markdown.UTF16Count(line)
		text = text[consumed:]

		raw := strings.TrimRight(line, "\n")
		trimmed := strings.TrimSpace(strings.TrimRight(raw, "\r"))

		if !frontChecked && !inFence {
			frontChecked = true
			if trimmed == "---" {
				inFront = true
				prev = ""
				prevBlank = true
				continue
			}
		}
		if inFront {
			if trimmed == "---" || trimmed == "..." {
				inFront = false
			}
			prev = ""
			prevBlank = true
			continue
		}

		if !inFence {
			if ok, mark := markdown.IsFenceOpen(raw); ok {
				inFence = true
				fenceMark = mark
				prev = ""
				prevBlank = true
				continue
			}
		} else {
			if markdown.FenceClosed(raw, fenceMark) {
				inFence = false
			}
			prev = ""
			prevBlank = true
			continue
		}

		if markdown.IsNoteLine(raw) {
			meta.Notes++
			prev = ""
			prevBlank = true
			continue
		}

		if level, heading, ok := markdown.ATXHeading(raw); ok {
			meta.Headings = append(meta.Headings, NavHeading{Text: heading, Offset: lineOff, Level: level})
			if meta.Title == "" {
				meta.Title = heading
			}
			prev = ""
			prevBlank = false
			continue
		}

		if lvl := markdown.SetextLevel(raw); lvl > 0 && prev != "" && !prevBlank {
			meta.Headings = append(meta.Headings, NavHeading{Text: prev, Offset: prevOff, Level: lvl})
			if meta.Title == "" {
				meta.Title = prev
			}
			prev = ""
			prevBlank = true
			continue
		}

		if para := markdown.ParagraphText(raw); para != "" {
			if meta.Excerpt == "" {
				meta.Excerpt = clipExcerpt(para)
			}
			prev = para
			prevOff = lineOff
			prevBlank = false
		} else {
			prev = ""
			prevBlank = trimmed == ""
		}
	}
	if meta.Title == "" {
		meta.Title = meta.Excerpt
	}
	return meta
}
