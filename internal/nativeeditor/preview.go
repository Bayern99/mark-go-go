package nativeeditor

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/portrait"
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
	frontEnd      int
	version       uint64
	applied       bool
	replacements  []editor.Replacement
	readable      map[int]string
	styles        []editor.LineStyle
	backgrounds   []editor.LineBackground
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

func previewTaskCheckbox(c *ui.Context, checked *bool) ui.Element {
	t := c.Theme()
	control := ui.CheckboxBase(c, checked).FocusRing(false)
	control.Children(func() {
		fill, border := t.Background, t.Border.Mix(t.Text, 0.25)
		if *checked {
			fill, border = portrait.Colors.Ink3, portrait.Colors.Ink3
		}
		radius := t.Space(1)
		box := ui.Box(c).Size(t.Space(4), t.Space(4)).Radius(radius).Shrink(0).Background(fill).Border(1, border)
		box.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if !*checked && control.Hovered() {
				p.Stroke(r, t.Accent, radius, 1)
			}
			if *checked {
				var mark ui.Path
				mark.MoveTo(r.X+r.W*0.22, r.Y+r.H*0.52).LineTo(r.X+r.W*0.42, r.Y+r.H*0.71).LineTo(r.X+r.W*0.78, r.Y+r.H*0.31)
				p.StrokePath(&mark, t.Space(0.5), portrait.Colors.Paper)
			}
			if control.FocusVisible() {
				p.FocusRing(r, [4]float32{radius, radius, radius, radius})
			}
		})
	})
	return control
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
	p.owner.pruneInlineImages(previewImageURLs(p.document))
	p.restoreFolds()
	p.readable = make(map[int]string)
	p.replacements = p.replacements[:0]
	p.styles = p.styles[:0]
	p.backgrounds = p.backgrounds[:0]
	p.blocks = append(p.blocks[:0], pageMargin(0, true, 44), pageMargin(snap.LineCount()-1, false, 120))
	p.frontEnd = p.frontmatter(snap)
	frontEnd := p.frontEnd
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
		case parser.CodeBlock:
			p.codeBlock(snap, i)
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
					checked := b.Task == parser.Checked
					if checked {
						mark = "☑"
					}
					p.replace(b.Start, task, "")
					p.readable[task] = mark
					line := snap.LineOf(task + 3)
					lineText := snap.Line(line)
					textAt := task + 3 - snap.LineStart(line)
					description := strings.TrimSpace(lineText[min(textAt, len(lineText)):])
					if description != "" {
						description = "Task: " + description
					} else {
						description = "Task"
					}
					p.replacements = append(p.replacements, editor.Replacement{
						Start:       task,
						End:         task + 3,
						Width:       20,
						Interactive: true,
						Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
							e.Children(func() {
								value := checked
								control := previewTaskCheckbox(c, &value).Label(description).Disabled(p.owner.state.ReadOnly)
								if control.Changed() && !p.owner.state.ReadOnly {
									idx := p.owner.sourceIndex()
									a, z := idx.rawAtEditor(task), idx.rawAtEditor(task+3)
									next := "[ ]"
									if value {
										next = "[x]"
									}
									if p.owner.Apply(snap.Version(), a, z, src[task:task+3], next) == nil {
										c.Invalidate()
									}
								}
							})
						},
					})
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
			line := snap.LineOf(start)
			lineStart := snap.LineStart(line)
			quoteDepth := 0
			for n := lineStart; n < start; n++ {
				if src[n] == '>' {
					quoteDepth++
					p.readable[n] = ""
					p.replacements = append(p.replacements, editor.Replacement{Start: n, End: n + 1, Width: quoteBarWidth, Content: quoteBar})
				}
			}
			if quoteDepth > 0 {
				p.addIndent(line, float32(quoteDepth*quoteBarInset), 0)
			}
		}
		inlines := p.document.Inlines(i)
		if b.Kind == parser.Paragraph && imageOnlyParagraph(inlines) {
			p.standaloneImages(snap, i, inlines)
			continue
		}
		for j, in := range inlines {
			switch in.Kind {
			case parser.Strong, parser.Emphasis, parser.Strikethrough:
				if in.Next > j+1 {
					p.replace(in.Start, inlines[j+1].Start, "")
					p.replace(inlines[in.Next-1].End, in.End, "")
				}
			case parser.Link:
				if in.Next > j+1 {
					prefixEnd := inlines[j+1].Start
					if ui.SafeToOpen(in.Dest) {
						p.replacements = append(p.replacements, editor.Replacement{
							Start:       in.Start,
							End:         prefixEnd,
							Width:       16,
							Interactive: true,
							Label:       "Open " + in.Dest,
							Content:     linkMarker(in.Dest),
						})
					} else {
						p.replace(in.Start, prefixEnd, "")
					}
					p.replace(inlines[in.Next-1].End, in.End, "")
				}
			case parser.Image:
				alt := parser.PlainText(inlines[j+1 : in.Next])
				label := alt
				if label == "" {
					label = "Image"
				}
				p.readable[in.Start] = label
				p.replacements = append(p.replacements, editor.Replacement{
					Start: in.Start,
					End:   in.End,
					Width: min(240, max(72, float32(utf8.RuneCountInString(label))*7)),
					Label: label,
					Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
						e.Children(func() {
							img := p.owner.loadInlineImage(c, in.Dest)
							if img == nil {
								portrait.Type.Meta.Apply(ui.Text(c, label)).TextColor(c.Theme().TextMuted).SingleLine().Ellipsis("…")
								return
							}
							w, h := ui.ImageSize(img)
							scale := min(float32(1), 68/max(1, w), 24/max(1, h))
							view := ui.Image(c, img).Size(max(1, w*scale), max(1, h*scale)).Label(label)
							if in.Title != "" {
								view.Tooltip(in.Title)
							}
						})
					},
				})
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

const (
	quoteBarWidth = 2
	quoteBarInset = 14
)

// addIndent adds horizontal insets to a line's existing font style, or
// gives it a style of its own. Quoted headings need both.
func (p *previewLayout) addIndent(line int, left, right float32) {
	for i := len(p.styles) - 1; i >= 0; i-- {
		if p.styles[i].Line == line {
			p.styles[i].Left += left
			p.styles[i].Right += right
			return
		}
	}
	p.styles = append(p.styles, editor.LineStyle{Line: line, Left: left, Right: right})
}

func quoteBar(c *ui.Context, e ui.Element, _ editor.Replacement) {
	e.Children(func() {
		ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(2).Background(portrait.Colors.LineStrong)
	})
}

func previewImageURLs(doc *parser.Document) map[string]struct{} {
	used := map[string]struct{}{}
	for i := 1; i < doc.Len(); i++ {
		for _, in := range doc.Inlines(i) {
			if in.Kind == parser.Image {
				used[in.Dest] = struct{}{}
			}
		}
	}
	return used
}

type previewImage struct {
	destination string
	title       string
	label       string
}

func imageOnlyParagraph(inlines []parser.Inline) bool {
	count := 0
	for i := 0; i < len(inlines); {
		in := inlines[i]
		switch in.Kind {
		case parser.Image:
			count++
		case parser.Text:
			if strings.TrimSpace(in.Text) != "" {
				return false
			}
		case parser.SoftBreak, parser.LineBreak:
		default:
			return false
		}
		i = max(i+1, in.Next)
	}
	return count > 0
}

func (p *previewLayout) standaloneImages(snap editor.Snapshot, blockIndex int, inlines []parser.Inline) {
	images := make([]previewImage, 0, len(inlines))
	for i := 0; i < len(inlines); {
		in := inlines[i]
		switch in.Kind {
		case parser.Image:
			label := parser.PlainText(inlines[i+1 : in.Next])
			if label == "" {
				label = "Image"
			}
			p.readable[in.Start] = label
			p.replace(in.Start, in.End, "")
			images = append(images, previewImage{destination: in.Dest, title: in.Title, label: label})
		case parser.Text:
			if strings.TrimSpace(in.Text) == "" {
				p.readable[in.Start] = " "
				p.replace(in.Start, in.End, "")
			}
		case parser.SoftBreak, parser.LineBreak:
			p.readable[in.Start] = " "
			p.replace(in.Start, in.End, "")
		}
		i = max(i+1, in.Next)
	}
	block := p.document.Block(blockIndex)
	p.blocks = append(p.blocks, editor.Block{
		Line:   snap.LineOf(block.Start),
		Above:  true,
		Height: 180,
		Width:  624,
		Label:  "Markdown image",
		Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
			maxWidth := e.LayoutBounds().W
			if maxWidth <= 0 || maxWidth > 624 {
				maxWidth = 624
			}
			e.Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					for _, figure := range images {
						src := p.owner.loadInlineImage(c, figure.destination)
						if src == nil {
							ui.Text(c, figure.label).Role(ui.RoleImage).Label(figure.label).TextColor(c.Theme().TextMuted)
							continue
						}
						w, h := ui.ImageSize(src)
						scale := min(float32(1), maxWidth/max(1, w), float32(420)/max(1, h))
						img := ui.Image(c, src).Size(max(1, w*scale), max(1, h*scale)).Label(figure.label).AlignSelf(ui.Center)
						if figure.title != "" {
							img.Tooltip(figure.title)
						}
					}
				})
			})
		},
	})
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
	style := [6]portrait.TypeStyle{
		portrait.Type.Display.In(portrait.FontDisplay),
		portrait.Type.Title.In(portrait.FontDisplay),
		portrait.Type.Reading,
		portrait.Type.H4,
		portrait.Type.H5,
		portrait.Type.H6,
	}[level-1]
	style.Leading = max(style.Leading, portrait.Type.Reading.Leading)
	return editor.LineStyle{Line: line, Font: style.Font(600), LineHeight: style.LineHeight()}
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
		st.OnHover = nil
		st.Label = "Markdown source"
		st.Font = portrait.Type.Code.Font(400)
		st.ReadingColumn = editor.ReadingColumn{}
		st.LineBackgrounds = nil
		st.LineNumbers = editor.LineNumbersOn
		st.LineHeight = portrait.Type.Code.LineHeight()
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
	st.OnHover = d.previewHover
	st.Label = "Markdown Preview"
	st.Font = portrait.Type.Reading.Font(400)
	st.LineHeight = portrait.Type.Reading.LineHeight()
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
