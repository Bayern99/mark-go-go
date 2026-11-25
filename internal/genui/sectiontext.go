package genui

import (
	"strings"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/markdown"
)

func SectionHash(section, document string, runsToDocumentEnd bool) string {
	return markdown.SectionHash(section, document, runsToDocumentEnd)
}

func SectionHashAsWritten(section string) string {
	return markdown.SectionHashAsWritten(section)
}

// SectionHashOf hashes the text a SectionRef names, in UTF-16 offsets, without
// the ranges the section excludes.
func SectionHashOf(text string, sec SectionRef) (string, bool) {
	var hashed strings.Builder
	at := sec.Start
	for _, cut := range sec.Exclude {
		if cut.Start < at || cut.End < cut.Start || cut.End > sec.End {
			return "", false
		}
		part, ok := sliceUTF16(text, at, cut.Start)
		if !ok {
			return "", false
		}
		hashed.WriteString(part)
		at = cut.End
	}
	part, ok := sliceUTF16(text, at, sec.End)
	if !ok {
		return "", false
	}
	hashed.WriteString(part)
	return SectionHash(hashed.String(), text, sec.End == UTF16Len(text)), true
}

func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		l := utf16.RuneLen(r)
		if l < 0 {
			l = 1
		}
		n += l
	}
	return n
}

func sliceUTF16(text string, start, end int) (string, bool) {
	if start < 0 || end < start {
		return "", false
	}
	byteStart, byteEnd := -1, -1
	if start == 0 {
		byteStart = 0
	}
	units := 0
	for i, ch := range text {
		if units == start {
			byteStart = i
		}
		if units == end {
			byteEnd = i
			break
		}
		l := utf16.RuneLen(ch)
		if l < 0 {
			l = 1
		}
		units += l
	}
	if byteEnd < 0 {
		if units != end {
			return "", false
		}
		byteEnd = len(text)
	}
	if byteStart < 0 && units == start {
		byteStart = len(text)
	}
	if byteStart < 0 || byteStart > byteEnd {
		return "", false
	}
	return text[byteStart:byteEnd], true
}
