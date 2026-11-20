package nativeeditor

import (
	"errors"
	"sort"
	"unicode/utf16"
	"unicode/utf8"
)

var ErrPosition = errors.New("position falls outside the source or splits a character or line ending")

type sourceLine struct {
	raw, editor, utf16 int
}

type sourceIndex struct {
	version uint64
	raw     string
	lines   []sourceLine
}

func indexSource(raw string, version uint64) *sourceIndex {
	idx := &sourceIndex{raw: raw, version: version, lines: []sourceLine{{}}}
	var norm, units int
	for at := 0; at < len(raw); {
		r, n := utf8.DecodeRuneInString(raw[at:])
		at += n
		norm += n
		units += utf16.RuneLen(r)
		if r == '\r' || r == '\n' {
			if r == '\r' && at < len(raw) && raw[at] == '\n' {
				at++
				units++
			}
			idx.lines = append(idx.lines, sourceLine{at, norm, units})
		}
	}
	return idx
}

func (s *sourceIndex) lineAtRaw(off int) sourceLine {
	i := sort.Search(len(s.lines), func(i int) bool { return s.lines[i].raw > off })
	return s.lines[i-1]
}

func (s *sourceIndex) rawAtEditor(off int) int {
	i := sort.Search(len(s.lines), func(i int) bool { return s.lines[i].editor > off })
	l := s.lines[i-1]
	return l.raw + off - l.editor
}

func (s *sourceIndex) editorAtRaw(off int) int {
	l := s.lineAtRaw(off)
	return l.editor + off - l.raw
}

func (s *sourceIndex) rawAtUTF16(pos int) (int, error) {
	if pos < 0 {
		return 0, ErrPosition
	}
	i := sort.Search(len(s.lines), func(i int) bool { return s.lines[i].utf16 > pos })
	l := s.lines[i-1]
	at := l.raw
	for units := l.utf16; units < pos; {
		if at >= len(s.raw) {
			return 0, ErrPosition
		}
		r, n := utf8.DecodeRuneInString(s.raw[at:])
		units += utf16.RuneLen(r)
		if units > pos {
			return 0, ErrPosition
		}
		at += n
	}
	if s.insideCRLF(at) {
		return 0, ErrPosition
	}
	return at, nil
}

func (s *sourceIndex) insideCRLF(at int) bool {
	return at > 0 && at < len(s.raw) && s.raw[at-1] == '\r' && s.raw[at] == '\n'
}

func (s *sourceIndex) utf16AtEditor(off int) int {
	raw := s.rawAtEditor(off)
	line := s.lineAtRaw(raw)
	n := line.utf16
	for _, r := range s.raw[line.raw:raw] {
		n += utf16.RuneLen(r)
	}
	return n
}

func validByteBoundary(text string, off int) bool {
	return off >= 0 && off <= len(text) && (off == len(text) || utf8.RuneStart(text[off]))
}
