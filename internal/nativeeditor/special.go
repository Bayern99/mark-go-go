package nativeeditor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
	mathtex "github.com/go-tex/math"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

type blockImage struct {
	svg    *ui.SVG
	reason string
}
type blockDraft struct {
	rev                  uint64
	start, end, anchor   int
	original, text, kind string
	focus                bool
	err                  error
	prefix, suffix       string
}

var typesetter = sync.OnceValues(func() (*mathtex.Renderer, error) { return mathtex.New(mathtex.DefaultFont()) })
var mermaidDirective = regexp.MustCompile("(?s)%%\\{.*?\\}%%")
var diagramHeader = regexp.MustCompile("(?s)^\\s*---\\n.*?\\n---\\s*\\n")
var externalDiagram = regexp.MustCompile(`(?i)<img\b|!\[[^\]]*\]\(|@\{[^}]*\b(?:img|icon)\s*:|\b(?:href|src)\s*=|url\(\s*['"]?[^#'"]`)

func (d *Document) renderImage(kind, body string) blockImage {
	key := kind + "\x00" + body
	if img, ok := d.artwork[key]; ok {
		return img
	}
	var svg []byte
	var err error
	switch kind {
	case "Mermaid":
		input := strings.TrimSpace(diagramHeader.ReplaceAllString(mermaidDirective.ReplaceAllString(body, ""), ""))
		switch {
		case externalDiagram.MatchString(input):
			err = fmt.Errorf("diagram references an external resource")
		case input == "":
			err = fmt.Errorf("empty diagram")
		default:
			svg, err = mermaid.Render(input)
		}
	default:
		var r *mathtex.Renderer
		if r, err = typesetter(); err == nil {
			var output string
			if kind == "inline math" {
				output, _, err = r.RenderSVGMetrics(body, 16)
			} else {
				output, _, err = r.RenderDisplaySVGMetrics(body, 22)
			}
			svg = []byte(output)
		}
	}
	img := blockImage{}
	if err == nil {
		img.svg, err = ui.ParseSVG(svg)
	}
	if err != nil {
		img.reason = err.Error()
	}
	if len(d.artwork) >= 128 || d.artwork == nil {
		d.artwork = map[string]blockImage{}
	}
	d.artwork[key] = img
	return img
}

func (p *previewLayout) inlineMath(in parser.Inline) {
	img := p.owner.renderImage("inline math", in.Text)
	if img.svg == nil {
		return
	}
	w, h := img.svg.Size()
	width := min(w, 280) * min(float32(1), 25/max(1, h))
	p.readable[in.Start] = in.Text
	p.replacements = append(p.replacements, editor.Replacement{Start: in.Start, End: in.End, Width: width, Label: in.Text, Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.Children(func() { ui.Image(c, img.svg).Size(width, min(25, h)).Label(in.Text) })
	}})
}

func (p *previewLayout) special(snap editor.Snapshot, i int) bool {
	b := p.document.Block(i)
	kind, body := "", ""
	switch b.Kind {
	case parser.MathBlock:
		kind = "math"
		body = strings.TrimSuffix(p.document.Lines(i), "\n")
	case parser.CodeBlock:
		kind = "code"
		if strings.EqualFold(b.Language(), "mermaid") {
			kind = "Mermaid"
		}
		body = strings.TrimSuffix(p.document.Lines(i), "\n")
	case parser.FootnoteDefinition:
		kind = "footnote"
		body = snap.Slice(b.Start, b.End)
		_, body, _ = strings.Cut(body, "]:")
		body = strings.TrimSpace(body)
	case parser.HTMLBlock:
		kind = "HTML"
		body = snap.Slice(b.Start, b.End)
		if _, value, ok := readNote(body); ok {
			kind = "note"
			body = value
		}
	case parser.Quote:
		if b.Alert == parser.NoAlert {
			return false
		}
		kind = b.Alert.String() + " alert"
		body = strings.TrimSpace(snap.Slice(b.Start, b.End))
		lines := strings.Split(body, "\n")
		if len(lines) > 0 {
			lines[0] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(lines[0], ">")), "[!"+strings.ToUpper(b.Alert.String())+"]"))
		}
		for n := 1; n < len(lines); n++ {
			lines[n] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[n]), ">"))
		}
		body = strings.TrimSpace(strings.Join(lines, "\n"))
	default:
		return false
	}
	label := kind
	if b.Kind == parser.FootnoteDefinition {
		label = b.Label()
	}
	if kind == "code" && b.Language() != "" {
		label = b.Info()
	}
	p.specialBlock(snap, b.Start, b.End, kind, label, body)
	return true
}

func (p *previewLayout) specialBlock(snap editor.Snapshot, start, end int, kind, label, body string) {
	d := p.owner
	p.standIn(snap, start, end, body, editor.Block{Label: label + " block", Height: 80, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Children(func() {
			ui.Column(c).FillWidth().Padding(8, 12).Gap(5).Children(func() {
				if draft := d.draft; draft != nil && draft.anchor == start {
					d.drawDraft(c, draft)
					return
				}
				var art blockImage
				if kind == "math" || kind == "Mermaid" {
					art = d.renderImage(kind, body)
				}
				if art.reason != "" {
					ui.Text(c, label+" could not render: "+art.reason).Role(ui.RoleAlert).TextColor(c.Theme().Danger)
					ui.Text(c, snap.Slice(start, end)).Font("IBM Plex Mono").FontSize(14).Wrap()
				} else if art.svg != nil {
					w, h := art.svg.Size()
					scale := min(float32(1), 600/max(1, w), 360/max(1, h))
					name := "Typeset equation"
					if kind == "Mermaid" {
						name = "Mermaid diagram"
					}
					ui.Image(c, art.svg).Size(max(1, w*scale), max(1, h*scale)).Label(name).Tooltip(body)
				} else if kind == "frontmatter" {
					ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
					frontmatterRows(c, body)
				} else {
					ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
					ui.Text(c, body).Wrap().Selectable()
				}
				if kind == "footnote" && ui.Button(c, "Return to reference "+label).Clicked() {
					p.returnTo(label)
					c.Invalidate()
				}
				if kind == "note" && !d.state.ReadOnly && ui.Button(c, "Delete note").Clicked() {
					d.deleteNote(snap, start, end)
					c.Invalidate()
				}
				if !d.state.ReadOnly && ui.Button(c, "Edit "+kind).Clicked() && d.Version() == snap.Version() {
					d.openDraft(start, end, kind)
				}
			})
		})
	}})
}

func (d *Document) openDraft(start, end int, kind string) {
	idx := d.sourceIndex()
	a, z := idx.rawAtEditor(start), idx.rawAtEditor(end)
	src := idx.raw[a:z]
	d.draft = &blockDraft{rev: d.Version(), anchor: start, start: a, end: z, original: src, text: normalizeLines(src), kind: kind, focus: true}
	if kind == "note" {
		_, d.draft.text, _ = readNote(src)
	}
}

func normalizeLines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func preserveLines(raw, next string) string {
	var endings []string
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\r' && raw[i] != '\n' {
			continue
		}
		a := i
		if raw[i] == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
			i++
		}
		endings = append(endings, raw[a:i+1])
	}
	lines := strings.Split(normalizeLines(next), "\n")
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			ending := "\n"
			if len(endings) > 0 {
				ending = endings[min(i-1, len(endings)-1)]
			}
			out.WriteString(ending)
		}
		out.WriteString(line)
	}
	return out.String()
}

func (d *Document) drawDraft(c *ui.Context, draft *blockDraft) {
	name := "Markdown block editor"
	if draft.kind == "note" || draft.kind == "new note" {
		name = "Note"
	}
	entry := ui.TextArea(c, &draft.text).Label(name).Height(100).FillWidth()
	if draft.focus {
		entry.Focus()
		draft.focus = false
	}
	save, cancel := entry.Shortcut(ui.Cmd, ui.KeyEnter), entry.Shortcut(0, ui.KeyEscape)
	ui.Row(c).Gap(6).Children(func() {
		if ui.Button(c, "Save").Clicked() {
			save = true
		}
		if ui.Button(c, "Cancel").Clicked() {
			cancel = true
		}
	})
	if cancel {
		d.draft = nil
		d.preview.dirty = true
		d.refocus = true
		c.Invalidate()
		return
	}
	if save {
		next := preserveLines(draft.original, draft.text)
		if draft.kind == "note" || draft.kind == "new note" {
			if strings.TrimSpace(draft.text) == "" {
				return
			}
			head, _, ok := readNote(draft.original)
			if !ok {
				head = "<!-- note:"
			}
			next = head + " " + escapeNoteEnd.ReplaceAllString(strings.TrimSpace(next), `--\${1}${2}>`) + " -->"
			if draft.kind == "new note" {
				next = draft.prefix + next + draft.suffix
			}
		}
		draft.err = d.Apply(draft.rev, draft.start, draft.end, draft.original, next)
		if draft.err == nil {
			d.draft = nil
			d.refocus = true
		}
		c.Invalidate()
	}
	if draft.err != nil {
		ui.Text(c, draft.err.Error()).Role(ui.RoleAlert).TextColor(c.Theme().Danger)
	}
}
