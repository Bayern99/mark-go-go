package nativeeditor

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type previewLayout struct {
	document      *parser.Document
	owner         *Document
	dirty         bool
	draftAnchored bool
	foldUntil     int
	hiddenRanges  [][2]int
	sections      []foldSection
	headings      map[string]int
	refs          map[string][]int
	version       uint64
	applied       bool
	replacements  []editor.Replacement
	readable      map[int]string
	styles        []editor.LineStyle
	blocks        []editor.Block
	display       string
	segments      []displaySegment
}

type displaySegment struct {
	from, to, shownFrom, shownTo int
}

func (p *previewLayout) replace(start, end int, text string) {
	if end > start {
		p.replacements = append(p.replacements, editor.Replacement{Start: start, End: end, Text: text})
	}
}

func (p *previewLayout) build(buffer *editor.Buffer) {
	if p.document == nil {
		p.document = parser.New(parser.Default | parser.ExtDollarMath)
	}
	p.version = buffer.Version()
	p.dirty = false
	p.draftAnchored = false
	p.foldUntil = 0
	p.hiddenRanges = p.hiddenRanges[:0]
	p.sections = p.sections[:0]
	p.headings = map[string]int{}
	p.refs = map[string][]int{}
	snap := buffer.Snapshot()
	src := snap.String()
	p.document.Set(src)
	p.restoreFolds()
	p.readable = make(map[int]string)
	p.replacements = p.replacements[:0]
	p.styles = p.styles[:0]
	p.blocks = append(p.blocks[:0], pageMargin(0, true, 44), pageMargin(snap.LineCount()-1, false, 120))
	frontEnd := p.frontmatter(snap)
	for i := 1; i < p.document.Len(); i++ {
		b := p.document.Block(i)
		if b.Start < frontEnd || b.Start < p.foldUntil {
			continue
		}
		if p.special(snap, i) {
			i = b.Next - 1
			continue
		}
		if b.Kind == parser.Table {
			p.table(snap, i)
			i = b.Next - 1
			continue
		}
		switch b.Kind {
		case parser.Heading:
			line := snap.LineOf(b.Start)
			p.styles = append(p.styles, headingStyle(line, b.Level))
			if p.document.LineCount(i) > 0 {
				_, start := p.document.Line(i, 0)
				p.heading(snap, i, start)
			}
			last := snap.LineOf(b.End)
			p.replace(b.End, snap.LineEnd(last), "")
			if last+1 < snap.LineCount() {
				underline := strings.TrimSpace(snap.Line(last + 1))
				if underline != "" && (strings.Trim(underline, "=") == "" || strings.Trim(underline, "-") == "") && !strings.HasPrefix(src[b.Start:], "#") {
					p.replace(snap.LineEnd(last), snap.LineEnd(last+1), "")
				}
			}
		case parser.Item:
			parent := p.document.Block(b.Parent)
			markerEnd := b.Start + 1
			if parent.Ordered {
				markerEnd = b.Start
				for markerEnd < b.End && src[markerEnd] >= '0' && src[markerEnd] <= '9' {
					markerEnd++
				}
				markerEnd = min(markerEnd+1, b.End)
			}
			if b.Task != parser.NoTask {
				task := markerEnd
				for task < b.End && (src[task] == ' ' || src[task] == '\t') {
					task++
				}
				if task+3 <= b.End && src[task] == '[' && src[task+2] == ']' {
					mark := "☐"
					if b.Task == parser.Checked {
						mark = "☑"
					}
					p.replace(b.Start, task+3, mark)
				}
			} else if !parent.Ordered {
				p.replace(b.Start, markerEnd, "•")
			}
		}
		if b.Kind != parser.Paragraph && b.Kind != parser.Heading {
			continue
		}
		for k := range p.document.LineCount(i) {
			_, start := p.document.Line(i, k)
			lineStart := snap.LineStart(snap.LineOf(start))
			for n := lineStart; n < start; n++ {
				if src[n] == '>' {
					p.replace(n, n+1, "│")
				}
			}
		}
		inlines := p.document.Inlines(i)
		for j, in := range inlines {
			switch in.Kind {
			case parser.Strong, parser.Emphasis, parser.Strikethrough, parser.Link:
				if in.Next > j+1 {
					p.replace(in.Start, inlines[j+1].Start, "")
					p.replace(inlines[in.Next-1].End, in.End, "")
				}
			case parser.Code:
				raw := src[in.Start:in.End]
				n := len(raw) - len(strings.TrimLeft(raw, "`"))
				if n > 0 && len(raw) >= 2*n {
					a, z := in.Start+n, in.End-n
					if z-a >= 2 && src[a] == ' ' && src[z-1] == ' ' && strings.TrimSpace(src[a:z]) != "" {
						a++
						z--
					}
					p.replace(in.Start, a, "")
					p.replace(z, in.End, "")
				}
			case parser.FootnoteRef:
				p.footnote(in)
			case parser.Math:
				p.inlineMath(in)
			case parser.Text:
				p.literal(src, in)
			}
		}
	}
	for offset, t := range p.owner.tables {
		if t.seen == p.version {
			continue
		}
		hidden := false
		for _, limits := range p.hiddenRanges {
			if offset >= limits[0] && offset < limits[1] {
				hidden = true
				break
			}
		}
		if !hidden {
			delete(p.owner.tables, offset)
		}
	}
	p.referenceDefinitions(snap, frontEnd)
	if draft := p.owner.draft; draft != nil && (draft.kind == "new note" || !p.draftAnchored) {
		p.blocks = append(p.blocks, editor.Block{Line: snap.LineOf(min(snap.Len(), draft.anchor)), Height: 140, Interactive: true, Label: "Note composer", Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
			if p.owner.draft == draft {
				e.Children(func() { p.owner.drawDraft(c, draft) })
			}
		}})
	}
	slices.SortStableFunc(p.replacements, func(a, b editor.Replacement) int { return a.Start - b.Start })
	p.indexDisplay(src)
}

func pageMargin(line int, above bool, height float32) editor.Block {
	return editor.Block{Line: line, Above: above, Height: height, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		if width, _ := c.Size(); above && width <= 640 {
			e.Height(12)
			return
		}
		e.Height(height)
	}}
}

func headingStyle(line, level int) editor.LineStyle {
	sizes := [6]float32{28, 20, 16, 15, 14, 13}
	size := sizes[level-1]
	family := "IBM Plex Sans"
	leading := float32(28)
	if level <= 2 {
		family = "Barlow Semi Condensed"
	}
	if level == 1 {
		leading = 36
	}
	return editor.LineStyle{Line: line, Font: ui.Font{Family: family, Size: size, Weight: 600}, LineHeight: leading / size}
}

func (p *previewLayout) literal(src string, in parser.Inline) {
	if src[in.Start:in.End] == in.Text {
		return
	}
	lit := in.Text
	for i := in.Start; i < in.End && len(lit) > 0; {
		switch {
		case src[i] == '\\' && i+1 < in.End && src[i+1] >= 33 && src[i+1] <= 126 && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", rune(src[i+1])) && src[i+1] == lit[0]:
			p.replace(i, i+2, lit[:1])
			i += 2
			lit = lit[1:]
		case src[i] == '&':
			n, decoded := parser.Entity(src[i:in.End])
			if n > 0 && strings.HasPrefix(lit, decoded) {
				p.replace(i, i+n, decoded)
				i += n
				lit = lit[len(decoded):]
				continue
			}
			if lit[0] != '&' {
				return
			}
			i++
			lit = lit[1:]
		default:
			if lit[0] != src[i] {
				return
			}
			i++
			lit = lit[1:]
		}
	}
}

func (p *previewLayout) indexDisplay(src string) {
	var out strings.Builder
	off := 0
	p.segments = p.segments[:0]
	for _, r := range p.replacements {
		if r.Start < off {
			continue
		}
		out.WriteString(src[off:r.Start])
		begin := out.Len()
		if visible, ok := p.readable[r.Start]; ok {
			out.WriteString(visible)
		} else {
			out.WriteString(r.Text)
		}
		p.segments = append(p.segments, displaySegment{r.Start, r.End, begin, out.Len()})
		off = r.End
	}
	out.WriteString(src[off:])
	p.display = out.String()
}

func (p *previewLayout) displayOffset(off int) int {
	i := sort.Search(len(p.segments), func(i int) bool { return p.segments[i].to > off })
	if i < len(p.segments) && off >= p.segments[i].from {
		return p.segments[i].shownFrom
	}
	if i == 0 {
		return off
	}
	last := p.segments[i-1]
	return last.shownTo + off - last.to
}

func (p *previewLayout) sourceOffset(runes int) int {
	off := 0
	for ; off < len(p.display) && runes > 0; runes-- {
		_, n := utf8.DecodeRuneInString(p.display[off:])
		off += n
	}
	i := sort.Search(len(p.segments), func(i int) bool { return p.segments[i].shownTo > off })
	if i < len(p.segments) && off >= p.segments[i].shownFrom {
		return p.segments[i].from
	}
	if i == 0 {
		return off
	}
	last := p.segments[i-1]
	return last.to + off - last.shownTo
}

func (d *Document) prepareView() {
	st, p := &d.state, &d.preview
	p.owner = d
	st.NoActiveLine = d.mode == Preview
	if d.mode == Markdown {
		st.ContextMenu = nil
		st.OnInput = nil
		st.Label = "Markdown source"
		st.Font = ui.Font{Family: "IBM Plex Mono", Size: 14}
		st.ReadingColumn = editor.ReadingColumn{}
		st.LineNumbers = editor.LineNumbersOn
		st.LineHeight = 1.5
		if st.Projection != nil {
			st.SetReplacements(nil)
			st.SetBlocks(nil)
			st.SetLineStyles(nil)
			st.Projection = nil
			p.applied = false
		}
		return
	}
	st.ContextMenu = func(m *ui.Menu) {
		if !d.state.ReadOnly && m.Item("Add note").Chosen() {
			d.beginNote()
		}
	}
	st.OnInput = d.previewInput
	st.Label = "Markdown Preview"
	st.Font = ui.Font{Family: "IBM Plex Sans", Size: 16}
	st.LineHeight = 28.0 / 16.0
	st.LineNumbers = editor.LineNumbersOff
	st.ReadingColumn = editor.ReadingColumn{Width: 624, Left: 56, Right: 40}
	if p.applied && !p.dirty && p.version == d.Version() {
		return
	}
	if p.dirty || p.document == nil || p.version != d.Version() {
		p.build(st.Buffer)
	}
	st.SetBlocks(p.blocks)
	st.SetReplacements(p.replacements)
	st.SetLineStyles(p.styles)
	p.applied = true
	st.Projection = &editor.TextProjection{
		Text:         func(a, z int) string { d.refreshProjection(); return p.display[p.displayOffset(a):p.displayOffset(z)] },
		SourceOffset: func(n int) int { d.refreshProjection(); return p.sourceOffset(n) },
		Markdown:     func(a, z int) string { idx := d.sourceIndex(); return idx.raw[idx.rawAtEditor(a):idx.rawAtEditor(z)] },
	}
}

func (d *Document) refreshProjection() {
	if d.preview.version != d.Version() {
		d.preview.build(d.state.Buffer)
		d.preview.applied = false
	}
}
