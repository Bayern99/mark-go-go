package nativeeditor

import (
	"github.com/egoist/mygo/ui/editor"
	"regexp"
	"strings"
)

var noteHead = regexp.MustCompile(`^[ \t]*<!--[ \t]*note(?:\s+[A-Za-z_][\w-]*="[^"]*")*\s*:`)
var escapeNoteEnd = regexp.MustCompile(`--(\\*)(!?)>`)
var unescapeNoteEnd = regexp.MustCompile(`--\\(\\*)(!?)>`)

func readNote(raw string) (head, text string, ok bool) {
	trimmed := strings.TrimRight(raw, " \t\r\n")
	inner, ok := strings.CutSuffix(trimmed, "-->")
	if !ok || strings.Contains(inner, "-->") {
		return "", "", false
	}
	loc := noteHead.FindStringIndex(inner)
	if loc == nil {
		return "", "", false
	}
	return inner[:loc[1]], unescapeNoteEnd.ReplaceAllString(strings.TrimSpace(inner[loc[1]:]), `--${1}${2}>`), true
}
func (d *Document) deleteNote(snap editor.Snapshot, start, end int) {
	a := start
	for a > 0 && snap.Slice(a-1, a) == "\n" {
		a--
	}
	idx := d.sourceIndex()
	a, z := idx.rawAtEditor(a), idx.rawAtEditor(end)
	d.Apply(snap.Version(), a, z, idx.raw[a:z], "")
}
func (d *Document) beginNote() {
	if d.state.ReadOnly {
		return
	}
	at := d.state.Cursor()
	snap := d.state.Buffer.Snapshot()
	for i := range d.preview.document.Children(0) {
		b := d.preview.document.Block(i)
		if at <= b.End {
			at = snap.LineEnd(snap.LineOf(b.End))
			break
		}
	}
	if at < d.preview.frontEnd {
		at = d.preview.frontEnd
	}
	idx := d.sourceIndex()
	raw := idx.rawAtEditor(at)
	newline := "\n"
	if strings.Contains(idx.raw, "\r\n") {
		newline = "\r\n"
	}
	d.draft = &blockDraft{rev: idx.version, anchor: at, start: raw, end: raw, kind: "new note", focus: true}
	if raw > 0 {
		d.draft.prefix = newline + newline
	}
	if raw < len(idx.raw) && !strings.HasPrefix(normalizeLines(idx.raw[raw:]), "\n\n") {
		d.draft.suffix = newline + newline
	}
	d.preview.dirty = true
}
