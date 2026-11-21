package nativeeditor

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/egoist/mygo/ui/markdown/parser"
)

type foldEntry struct {
	Base      string `json:"base"`
	Index     int    `json:"idx"`
	Signature string `json:"sig"`
}
type readingPreferences struct {
	pending []foldEntry
	widths  map[string][]float32
	save    func(string, string)
}

func (d *Document) SetReadingPreferences(folds, widths string, save func(string, string)) {
	var entries struct {
		Entries []foldEntry `json:"entries"`
	}
	var stored map[string][]float32
	json.Unmarshal([]byte(folds), &entries)
	json.Unmarshal([]byte(widths), &stored)
	d.preferences = readingPreferences{pending: entries.Entries, widths: stored, save: save}
	d.folded = nil
	d.tables = nil
	d.preview.dirty = true
}

func inlineText(d *parser.Document, i int) string {
	var s strings.Builder
	for _, in := range d.Inlines(i) {
		switch in.Kind {
		case parser.Text, parser.Code, parser.Mention, parser.Reference, parser.Math:
			s.WriteString(in.Text)
		case parser.SoftBreak:
			s.WriteByte('\n')
		case parser.FootnoteRef:
			s.WriteString("[^" + in.Text + "]")
		}
	}
	return s.String()
}

func (p *previewLayout) foldEntries() map[string]foldEntry {
	result := map[string]foldEntry{}
	headings := map[string]int{}
	for i := range p.document.Children(0) {
		b := p.document.Block(i)
		if b.Kind != parser.Heading {
			continue
		}
		title := strings.TrimSpace(p.document.Lines(i))
		label := inlineText(p.document, i)
		var body []string
		for j := b.Next; j < p.document.Len(); j++ {
			n := p.document.Block(j)
			if n.Kind == parser.Heading && n.Parent == 0 && n.Level <= b.Level {
				break
			}
			switch n.Kind {
			case parser.Heading, parser.Paragraph, parser.TableCell:
				body = append(body, inlineText(p.document, j))
			case parser.CodeBlock:
				if len(strings.Fields(n.Info())) <= 1 && n.Language() != "mermaid" {
					body = append(body, strings.TrimSuffix(p.document.Lines(j), "\n"))
				}
			case parser.FootnoteDefinition, parser.MathBlock, parser.HTMLBlock:
				j = n.Next - 1
			case parser.Quote:
				if n.Alert != parser.NoAlert {
					j = n.Next - 1
				}
			}
		}
		level := strconv.Itoa(b.Level)
		h := uint32(5381)
		for _, u := range utf16.Encode([]rune("heading|" + level + "|" + label + "|" + strings.Join(body, " "))) {
			h = h*33 + uint32(u)
		}
		key := headingKey(b.Level, title, headings[title])
		result[key] = foldEntry{Base: "h" + level + ":" + strings.ToLower(strings.Join(strings.Fields(label), " ")), Index: headings[title], Signature: strconv.FormatUint(uint64(h), 10)}
		headings[title]++
	}
	return result
}

func headingKey(level int, title string, i int) string {
	return strconv.Itoa(level) + ":" + title + ":" + strconv.Itoa(i)
}

func (p *previewLayout) restoreFolds() {
	prefs := &p.owner.preferences
	if len(prefs.pending) == 0 {
		return
	}
	bySig := map[string][]string{}
	for key, entry := range p.foldEntries() {
		bySig[entry.Signature] = append(bySig[entry.Signature], key)
	}
	p.owner.folded = map[string]bool{}
	for _, entry := range prefs.pending {
		if keys := bySig[entry.Signature]; len(keys) == 1 {
			p.owner.folded[keys[0]] = true
		}
	}
	prefs.pending = nil
}
func (p *previewLayout) saveFolds() {
	if p.owner.preferences.save == nil {
		return
	}
	entries := []foldEntry{}
	for key, e := range p.foldEntries() {
		if p.owner.folded[key] {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Base == entries[j].Base {
			return entries[i].Index < entries[j].Index
		}
		return entries[i].Base < entries[j].Base
	})
	data, _ := json.Marshal(struct {
		Entries []foldEntry `json:"entries"`
	}{entries})
	p.owner.preferences.save("folds", string(data))
}

func tableSignature(n int, rows [][]string) string {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.Encode([]any{n, rows})
	hash := uint32(5381)
	for _, r := range strings.TrimSuffix(out.String(), "\n") {
		hash = hash*33 ^ uint32(r)
	}
	return strconv.Itoa(n) + ":" + strconv.FormatUint(uint64(hash), 36)
}
func (p *readingPreferences) restoreTable(m *readingTable) {
	var saved []float32
	count := 0
	for key, w := range p.widths {
		_, sig, _ := strings.Cut(key, ":")
		if sig == m.signature {
			saved = w
			count++
		}
	}
	if count != 1 || len(saved) != len(m.widths) {
		return
	}
	for i, w := range saved {
		if w >= 72 {
			m.widths[i] = w
			m.manual = true
		}
	}
}
func (d *Document) saveTables() {
	if d.preferences.save == nil {
		return
	}
	widths := map[string][]float32{}
	for at, t := range d.tables {
		if t.manual {
			widths[strconv.Itoa(at)+":"+t.signature] = t.widths
		}
	}
	data, _ := json.Marshal(widths)
	d.preferences.save("table-widths", string(data))
}
