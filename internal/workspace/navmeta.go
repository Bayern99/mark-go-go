package workspace

import (
	"strings"
	"unicode/utf16"
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

func utf16Count(s string) int {
	units := 0
	for _, r := range s {
		l := utf16.RuneLen(r)
		if l < 0 {
			l = 1
		}
		units += l
	}
	return units
}

func isFenceOpen(line string) (ok bool, mark byte) {
	s := strings.TrimRight(line, "\r")
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') && n < 3 {
		n++
	}
	if n >= len(s) || (s[n] != '`' && s[n] != '~') {
		return false, 0
	}
	mark = s[n]
	count := 0
	for n < len(s) && s[n] == mark {
		count++
		n++
	}
	return count >= 3, mark
}

func fenceClosed(line string, mark byte) bool {
	ok, got := isFenceOpen(line)
	return ok && got == mark
}

func atxHeading(line string) (level int, text string, ok bool) {
	s := strings.TrimRight(line, "\r")
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') && i < 3 {
		i++
	}
	if i >= len(s) || s[i] != '#' {
		return 0, "", false
	}
	level = 0
	for i < len(s) && s[i] == '#' && level < 6 {
		level++
		i++
	}
	if level == 0 || i >= len(s) || (s[i] != ' ' && s[i] != '\t') {
		return 0, "", false
	}
	text = strings.TrimSpace(s[i:])
	text = strings.TrimRight(text, "#")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}

func setextLevel(line string) int {
	s := strings.TrimRight(line, "\r")
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') && i < 3 {
		i++
	}
	if i >= len(s) {
		return 0
	}
	mark := s[i]
	if mark != '=' && mark != '-' {
		return 0
	}
	for i < len(s) && s[i] == mark {
		i++
	}
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i != len(s) {
		return 0
	}
	if mark == '=' {
		return 1
	}
	return 2
}

func isNoteLine(line string) bool {
	s := strings.TrimSpace(strings.TrimRight(line, "\r"))
	return strings.HasPrefix(s, "<!--") && strings.Contains(s, "note") && strings.Contains(s, ":")
}

func paragraphText(line string) string {
	s := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "#") || strings.HasPrefix(s, ">") || strings.HasPrefix(s, "```") || strings.HasPrefix(s, "---") {
		return ""
	}
	if strings.HasPrefix(s, "<!--") {
		return ""
	}
	return s
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
		utfAt += utf16Count(line)
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
			if ok, mark := isFenceOpen(raw); ok {
				inFence = true
				fenceMark = mark
				prev = ""
				prevBlank = true
				continue
			}
		} else {
			if fenceClosed(raw, fenceMark) {
				inFence = false
			}
			prev = ""
			prevBlank = true
			continue
		}

		if isNoteLine(raw) {
			meta.Notes++
			prev = ""
			prevBlank = true
			continue
		}

		if level, heading, ok := atxHeading(raw); ok {
			meta.Headings = append(meta.Headings, NavHeading{Text: heading, Offset: lineOff, Level: level})
			if meta.Title == "" {
				meta.Title = heading
			}
			prev = ""
			prevBlank = false
			continue
		}

		if lvl := setextLevel(raw); lvl > 0 && prev != "" && !prevBlank {
			meta.Headings = append(meta.Headings, NavHeading{Text: prev, Offset: prevOff, Level: lvl})
			if meta.Title == "" {
				meta.Title = prev
			}
			prev = ""
			prevBlank = true
			continue
		}

		if para := paragraphText(raw); para != "" {
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
