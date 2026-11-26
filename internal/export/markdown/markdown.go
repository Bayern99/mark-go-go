package markdown

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/assets"
	"github.com/Bayern99/mark-go-go/internal/export/theme"
	md "github.com/Bayern99/mark-go-go/internal/markdown"
	"github.com/Bayern99/mark-go-go/internal/markdown/svgfigure"
	"github.com/egoist/mygo/ui/markdown/parser"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

type renderer struct {
	source       string
	options      export.Options
	document     *parser.Document
	assets       *assets.Resolver
	warnings     []string
	headingIDs   map[string]int
	footnotes    []string
	footnoteUses map[string]int
	attached     map[int][]md.Note
	pending      map[string][]md.Note
	sections     map[int]md.Section
	active       []md.Section
}

func Render(source string, options export.Options) (export.Result, error) {
	resolver := assets.NewWithOptions(options.BaseDir, options.OfflineImages)
	defer resolver.Close()
	body, warnings, err := Fragment(source, options, resolver)
	if err != nil {
		return export.Result{}, err
	}
	warnings = append(warnings, resolver.Warnings()...)
	return export.Result{Data: theme.Document(options, body, ""), Warnings: warnings}, nil
}

func Fragment(source string, options export.Options, resolver *assets.Resolver) (string, []string, error) {
	if resolver == nil {
		resolver = assets.NewWithOptions(options.BaseDir, options.OfflineImages)
		defer resolver.Close()
	}
	notes := md.ExtractNotes(source)
	sections := map[int]md.Section{}
	for _, section := range md.ExtractSections(source) {
		sections[section.ByteStart()] = section
	}
	r := &renderer{
		source: withoutNotes(source, notes, !options.IncludeNotes), options: options,
		assets: resolver, headingIDs: map[string]int{}, footnoteUses: map[string]int{},
		attached: map[int][]md.Note{}, pending: map[string][]md.Note{}, sections: sections,
	}
	r.document = parser.New(parser.Default | parser.ExtDollarMath)
	r.document.Set(r.source)
	if options.IncludeNotes {
		r.placeNotes(notes)
	}
	var b strings.Builder
	r.rootBlocks(&b)
	r.writeFootnotes(&b)
	return b.String(), r.warnings, nil
}

func withoutNotes(source string, notes []md.Note, stripAll bool) string {
	data := []byte(source)
	changed := false
	for _, note := range notes {
		start := note.CutStartByte()
		_, end := note.ByteRange()
		start, end = max(0, start), min(len(data), end)
		for i := start; i < end; i++ {
			if data[i] != '\r' && data[i] != '\n' {
				data[i] = ' '
				changed = true
			}
		}
	}
	if stripAll {
		if maskResidualNoteComments(data) {
			changed = true
		}
	}
	if !changed {
		return source
	}
	return string(data)
}

type byteSpan struct {
	start, end int
}

func maskResidualNoteComments(data []byte) bool {
	doc := parser.New(parser.Default | parser.ExtDollarMath)
	doc.Set(string(data))
	var codeSpans []byteSpan
	var walk func(parent int)
	walk = func(parent int) {
		for i := range doc.Children(parent) {
			block := doc.Block(i)
			if block.Kind == parser.CodeBlock {
				codeSpans = append(codeSpans, byteSpan{start: block.Start, end: block.End})
				continue
			}
			if block.Kind != parser.HTMLBlock {
				for _, inline := range doc.Inlines(i) {
					if inline.Kind == parser.Code && inline.Start >= 0 && inline.End > inline.Start {
						codeSpans = append(codeSpans, byteSpan{start: inline.Start, end: inline.End})
					}
				}
			}
			walk(i)
		}
	}
	walk(0)

	changed := false
	n := len(data)
	for i := 0; i < n; {
		if spanEnd, inside := insideSpan(codeSpans, i); inside {
			i = spanEnd
			continue
		}
		if data[i] == '<' && bytes.HasPrefix(data[i:], []byte("<!--")) {
			closeOffset := bytes.Index(data[i+4:], []byte("-->"))
			if closeOffset >= 0 {
				end := i + 4 + closeOffset + 3
				if _, _, ok := md.ReadNote(string(data[i:end])); ok {
					for j := i; j < end; j++ {
						if data[j] != '\r' && data[j] != '\n' {
							data[j] = ' '
							changed = true
						}
					}
					i = end
					continue
				}
			}
		}
		i++
	}
	return changed
}

func insideSpan(spans []byteSpan, pos int) (int, bool) {
	for _, span := range spans {
		if pos >= span.start && pos < span.end {
			return span.end, true
		}
	}
	return 0, false
}

func mdSectionKey(heading string, occurrence int) string {
	if heading == "" {
		return ""
	}
	return md.NormalizeHeadingText(heading) + "\x00" + strconv.Itoa(max(1, occurrence))
}

func (r *renderer) placeNotes(notes []md.Note) {
	var blocks []int
	for i := range r.document.Children(0) {
		blocks = append(blocks, i)
	}
	for _, note := range notes {
		start, _ := note.ByteRange()
		previous := -1
		for _, i := range blocks {
			block := r.document.Block(i)
			if block.End > start {
				break
			}
			if block.End <= start && start <= len(r.source) && strings.TrimSpace(r.source[block.End:start]) == "" {
				previous = i
			}
		}
		if previous >= 0 && r.document.Block(previous).Kind == parser.Paragraph {
			r.attached[previous] = append(r.attached[previous], note)
			continue
		}
		key := mdSectionKey(note.SectionHeading, note.SectionOccurrence)
		r.pending[key] = append(r.pending[key], note)
	}
}

func (r *renderer) rootBlocks(out *strings.Builder) {
	for i := range r.document.Children(0) {
		block := r.document.Block(i)
		if section, ok := r.sections[block.Start]; block.Kind == parser.Heading && ok {
			for len(r.active) > 0 && r.active[len(r.active)-1].Level >= section.Level {
				last := r.active[len(r.active)-1]
				key := mdSectionKey(last.Heading, last.Occurrence)
				r.notes(out, r.pending[key])
				delete(r.pending, key)
				r.active = r.active[:len(r.active)-1]
			}
			r.active = append(r.active, section)
		}
		r.block(out, i)
		r.notes(out, r.attached[i])
	}
	for len(r.active) > 0 {
		last := r.active[len(r.active)-1]
		key := mdSectionKey(last.Heading, last.Occurrence)
		r.notes(out, r.pending[key])
		delete(r.pending, key)
		r.active = r.active[:len(r.active)-1]
	}
	r.notes(out, r.pending[""])
}

func (r *renderer) blocks(out *strings.Builder, parent int) {
	for i := range r.document.Children(parent) {
		r.block(out, i)
	}
}

func (r *renderer) tight(i int) bool {
	block := r.document.Block(i)
	if block.Parent <= 0 {
		return false
	}
	parent := r.document.Block(block.Parent)
	return parent.Kind == parser.Item && parent.Parent >= 0 && r.document.Block(parent.Parent).Tight
}

func (r *renderer) block(out *strings.Builder, i int) {
	block := r.document.Block(i)
	switch block.Kind {
	case parser.Paragraph:
		if block.Start == block.End {
			return
		}
		if !r.tight(i) {
			out.WriteString("<p>")
		}
		r.inlines(out, i)
		if !r.tight(i) {
			out.WriteString("</p>\n")
		}
	case parser.Heading:
		level := min(6, max(1, block.Level))
		text := r.inlineText(i)
		id := r.headingID(text)
		fmt.Fprintf(out, `<h%d id="%s">`, level, html.EscapeString(id))
		r.inlines(out, i)
		fmt.Fprintf(out, "</h%d>\n", level)
	case parser.ThematicBreak:
		out.WriteString("<hr>\n")
	case parser.CodeBlock:
		body := strings.TrimSuffix(r.document.Lines(i), "\n")
		if strings.EqualFold(block.Language(), "mermaid") {
			r.mermaid(out, body)
			return
		}
		out.WriteString(CodeBlockHTML(body, block.Language()))
	case parser.HTMLBlock:
		r.rawHTML(out, r.document.Lines(i), true)
	case parser.MathBlock:
		body := strings.TrimSuffix(r.document.Lines(i), "\n")
		r.math(out, body, true)
	case parser.Quote:
		if block.Alert == parser.NoAlert {
			out.WriteString("<blockquote>\n")
		} else {
			name := strings.ToLower(block.Alert.String())
			out.WriteString(`<div class="markdown-alert markdown-alert-` + name + `">`)
			out.WriteString(`<p class="markdown-alert-title">` + block.Alert.String() + "</p>\n")
		}
		r.blocks(out, i)
		if block.Alert == parser.NoAlert {
			out.WriteString("</blockquote>\n")
		} else {
			out.WriteString("</div>\n")
		}
	case parser.List:
		tag := "ul"
		if block.Ordered {
			tag = "ol"
		}
		out.WriteString("<" + tag)
		if block.Ordered && block.Number != 1 {
			out.WriteString(` start="` + strconv.Itoa(block.Number) + `"`)
		}
		out.WriteString(">\n")
		r.blocks(out, i)
		out.WriteString("</" + tag + ">\n")
	case parser.Item:
		class := ""
		if block.Task != parser.NoTask {
			class = ` class="task"`
		}
		out.WriteString("<li" + class + ">")
		if block.Task != parser.NoTask {
			out.WriteString(`<input type="checkbox" disabled`)
			if block.Task == parser.Checked {
				out.WriteString(" checked")
			}
			out.WriteString("> ")
		}
		r.blocks(out, i)
		out.WriteString("</li>\n")
	case parser.Table:
		r.table(out, i)
	case parser.FootnoteDefinition:
	}
}

func (r *renderer) table(out *strings.Builder, i int) {
	block := r.document.Block(i)
	out.WriteString(`<div class="table-wrap"><table>`)
	body := false
	for rowIndex := range r.document.Children(i) {
		row := r.document.Block(rowIndex)
		if row.Header {
			out.WriteString("<thead>")
		} else if !body {
			out.WriteString("<tbody>")
			body = true
		}
		out.WriteString("<tr>")
		column := 0
		for cellIndex := range r.document.Children(rowIndex) {
			tag := "td"
			if row.Header {
				tag = "th"
			}
			out.WriteString("<" + tag)
			if column < len(block.Align) {
				switch block.Align[column] {
				case parser.AlignLeft:
					out.WriteString(` align="left"`)
				case parser.AlignCenter:
					out.WriteString(` align="center"`)
				case parser.AlignRight:
					out.WriteString(` align="right"`)
				}
			}
			out.WriteString(">")
			r.inlines(out, cellIndex)
			out.WriteString("</" + tag + ">")
			column++
		}
		out.WriteString("</tr>")
		if row.Header {
			out.WriteString("</thead>")
		}
	}
	if body {
		out.WriteString("</tbody>")
	}
	out.WriteString("</table></div>\n")
}

func (r *renderer) inlines(out *strings.Builder, block int) {
	inlines := r.document.Inlines(block)
	r.inlineRange(out, inlines, 0, len(inlines))
}

func (r *renderer) inlineRange(out *strings.Builder, inlines []parser.Inline, from, to int) {
	for i := from; i < to; i = inlines[i].Next {
		node := &inlines[i]
		switch node.Kind {
		case parser.Text, parser.Mention, parser.Reference:
			out.WriteString(html.EscapeString(node.Text))
		case parser.SoftBreak:
			out.WriteByte('\n')
		case parser.LineBreak:
			out.WriteString("<br>\n")
		case parser.Code:
			out.WriteString("<code>" + html.EscapeString(node.Text) + "</code>")
		case parser.Emphasis:
			out.WriteString("<em>")
			r.inlineRange(out, inlines, i+1, node.Next)
			out.WriteString("</em>")
		case parser.Strong:
			out.WriteString("<strong>")
			r.inlineRange(out, inlines, i+1, node.Next)
			out.WriteString("</strong>")
		case parser.Strikethrough:
			out.WriteString("<del>")
			r.inlineRange(out, inlines, i+1, node.Next)
			out.WriteString("</del>")
		case parser.Link:
			if href, ok := assets.SafeLink(node.Dest); ok {
				out.WriteString(`<a href="` + html.EscapeString(href) + `"`)
				if node.Title != "" {
					out.WriteString(` title="` + html.EscapeString(node.Title) + `"`)
				}
				out.WriteString(">")
				r.inlineRange(out, inlines, i+1, node.Next)
				out.WriteString("</a>")
			} else {
				r.inlineRange(out, inlines, i+1, node.Next)
			}
		case parser.Image:
			r.image(out, inlines, i, node)
		case parser.HTML:
			r.rawHTML(out, node.Text, false)
		case parser.Math:
			r.math(out, node.Text, node.Display)
		case parser.FootnoteRef:
			r.footnoteRef(out, node.Text)
		}
	}
}

func (r *renderer) rawHTML(out *strings.Builder, raw string, block bool) {
	if _, text, ok := md.ReadNote(raw); ok {
		if r.options.IncludeNotes {
			r.notes(out, []md.Note{{Text: text}})
		}
		return
	}
	if !r.options.IncludeNotes {
		raw = stripEmbeddedNoteComments(raw)
		if strings.TrimSpace(raw) == "" {
			return
		}
	}
	if block {
		out.WriteString(`<pre class="raw-html"><code>`)
		out.WriteString(html.EscapeString(raw))
		out.WriteString("</code></pre>\n")
		return
	}
	out.WriteString(html.EscapeString(raw))
}

func stripEmbeddedNoteComments(raw string) string {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		rel := strings.Index(raw[i:], "<!--")
		if rel < 0 {
			b.WriteString(raw[i:])
			break
		}
		start := i + rel
		closeRel := strings.Index(raw[start+4:], "-->")
		if closeRel < 0 {
			b.WriteString(raw[i:])
			break
		}
		end := start + 4 + closeRel + 3
		if _, _, ok := md.ReadNote(raw[start:end]); ok {
			b.WriteString(raw[i:start])
			i = end
			continue
		}
		b.WriteString(raw[i:end])
		i = end
	}
	return b.String()
}

func (r *renderer) image(out *strings.Builder, inlines []parser.Inline, i int, node *parser.Inline) {
	alt := inlinePlainText(inlines[i+1 : node.Next])
	resolved := r.assets.Image(node.Dest)
	if resolved.Missing {
		name := resolved.Name
		if name == "" {
			name = alt
		}
		out.WriteString(`<span class="image-missing" role="img" aria-label="Missing image: ` + html.EscapeString(name) + `">Image unavailable: ` + html.EscapeString(name) + `</span>`)
		return
	}
	out.WriteString(`<img src="` + html.EscapeString(resolved.URL) + `" alt="` + html.EscapeString(alt) + `"`)
	if node.Title != "" {
		out.WriteString(` title="` + html.EscapeString(node.Title) + `"`)
	}
	out.WriteString(" loading=" + `"lazy"` + ">")
}

func inlinePlainText(inlines []parser.Inline) string {
	var b strings.Builder
	for i := 0; i < len(inlines); i++ {
		node := inlines[i]
		switch node.Kind {
		case parser.Text, parser.Code, parser.Math, parser.Mention, parser.Reference:
			b.WriteString(node.Text)
		case parser.SoftBreak, parser.LineBreak:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func (r *renderer) inlineText(block int) string {
	return inlinePlainText(r.document.Inlines(block))
}

func (r *renderer) headingID(text string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
			lastDash = false
		} else if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "section"
	}
	base := id
	r.headingIDs[base]++
	if count := r.headingIDs[base]; count > 1 {
		id += "-" + strconv.Itoa(count)
	}
	return id
}

func (r *renderer) math(out *strings.Builder, source string, display bool) {
	size := 16
	if display {
		size = 22
	}
	svg, err := svgfigure.RenderMathSVG(source, display, size)
	if err != nil {
		r.warnings = append(r.warnings, "Math could not be rendered: "+err.Error())
		out.WriteString("<code>" + html.EscapeString(source) + "</code>")
		return
	}
	svg = printableSVG(svg)
	if display {
		out.WriteString(`<div class="math-display"><span class="math">` + svg + `</span></div>` + "\n")
	} else {
		out.WriteString(`<span class="math">` + svg + `</span>`)
	}
}

func printableSVG(svg string) string {
	replacer := strings.NewReplacer(
		`fill="#000000"`, `fill="currentColor"`, `fill="#000"`, `fill="currentColor"`,
		`stroke="#000000"`, `stroke="currentColor"`, `stroke="#000"`, `stroke="currentColor"`,
	)
	return replacer.Replace(svg)
}

func (r *renderer) mermaid(out *strings.Builder, source string) {
	palette := mermaid.Palette{Background: "transparent", NodeFill: "var(--fill)", NodeStroke: "var(--accent)", Text: "currentColor", Edge: "currentColor"}
	svg, err := svgfigure.RenderMermaidSVG(source,
		mermaid.WithTransparentBackground(),
		mermaid.WithCustomTheme("mark-go-go-export", palette),
		mermaid.WithFont("Inter, sans-serif", 14),
	)
	if err != nil {
		r.warnings = append(r.warnings, "Mermaid diagram could not be rendered: "+err.Error())
		out.WriteString(CodeBlockHTML(source, "mermaid"))
		return
	}
	out.WriteString(`<figure class="mermaid">` + printableMermaidSVG(svg) + `</figure>` + "\n")
}

func printableMermaidSVG(svg []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return printableSVG(string(svg))
		}
		if start, ok := token.(xml.StartElement); ok {
			for i := range start.Attr {
				attr := &start.Attr[i]
				name := strings.ToLower(attr.Name.Local)
				if name != "fill" && name != "stroke" {
					continue
				}
				value := strings.ToLower(strings.TrimSpace(attr.Value))
				if value == "" || value == "none" || value == "transparent" || value == "currentcolor" || strings.HasPrefix(value, "var(") || strings.HasPrefix(value, "url(#") {
					continue
				}
				if name == "stroke" {
					attr.Value = "currentColor"
					continue
				}
				element := strings.ToLower(start.Name.Local)
				switch {
				case element == "text" || element == "tspan" || value == "#000" || value == "#000000" || value == "black":
					attr.Value = "currentColor"
				case value == "#fff5ad":
					attr.Value = "var(--note-soft)"
				default:
					attr.Value = "var(--fill)"
				}
			}
			token = start
		}
		if err := encoder.EncodeToken(token); err != nil {
			return printableSVG(string(svg))
		}
	}
	if err := encoder.Flush(); err != nil {
		return printableSVG(string(svg))
	}
	return out.String()
}

func (r *renderer) footnoteRef(out *strings.Builder, label string) {
	if _, ok := r.document.Footnote(label); !ok {
		out.WriteString(html.EscapeString("[^" + label + "]"))
		return
	}
	r.footnoteUses[label]++
	if r.footnoteUses[label] == 1 {
		r.footnotes = append(r.footnotes, label)
	}
	id := slug(label)
	use := r.footnoteUses[label]
	fmt.Fprintf(out, `<sup id="fnref-%s-%d"><a class="footnote-ref" href="#fn-%s">%d</a></sup>`, id, use, id, len(r.footnotes))
}

func (r *renderer) writeFootnotes(out *strings.Builder) {
	if len(r.footnotes) == 0 {
		return
	}
	out.WriteString(`<section class="footnotes"><h2>Footnotes</h2><ol>`)
	for _, label := range r.footnotes {
		index, ok := r.document.Footnote(label)
		if !ok {
			continue
		}
		id := slug(label)
		out.WriteString(`<li id="fn-` + id + `">`)
		r.blocks(out, index)
		for use := 1; use <= r.footnoteUses[label]; use++ {
			fmt.Fprintf(out, ` <a class="footnote-back" href="#fnref-%s-%d" aria-label="Back to reference">↩</a>`, id, use)
		}
		out.WriteString("</li>")
	}
	out.WriteString("</ol></section>\n")
}

func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		return "note"
	}
	if utf8.RuneCountInString(id) > 80 {
		runes := []rune(id)
		id = string(runes[:80])
	}
	return id
}

func (r *renderer) notes(out *strings.Builder, notes []md.Note) {
	for _, note := range notes {
		out.WriteString(`<aside class="reader-note"><p class="reader-note-title">Reader Note</p><p>`)
		out.WriteString(strings.ReplaceAll(html.EscapeString(note.Text), "\n", "<br>"))
		out.WriteString("</p></aside>\n")
	}
}
