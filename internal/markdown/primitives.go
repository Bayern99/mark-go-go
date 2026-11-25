package markdown

import (
	"strings"
	"unicode/utf16"
)

func UTF16Count(s string) int {
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

func IsFenceOpen(line string) (ok bool, mark byte) {
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

func FenceClosed(line string, mark byte) bool {
	ok, got := IsFenceOpen(line)
	return ok && got == mark
}

func ATXHeading(line string) (level int, text string, ok bool) {
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

func SetextLevel(line string) int {
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

func IsNoteLine(line string) bool {
	s := strings.TrimSpace(strings.TrimRight(line, "\r"))
	return strings.HasPrefix(s, "<!--") && strings.Contains(s, "note") && strings.Contains(s, ":")
}

func ParagraphText(line string) string {
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
