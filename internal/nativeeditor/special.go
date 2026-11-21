package nativeeditor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
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
	kind, body, detail := "", "", ""
	switch b.Kind {
	case parser.MathBlock:
		kind = "math"
		body = strings.TrimSuffix(p.document.Lines(i), "\n")
	case parser.CodeBlock:
		info := b.Info()
		meta := strings.TrimSpace(strings.TrimPrefix(info, b.Language()))
		if !strings.EqualFold(b.Language(), "mermaid") && meta == "" {
			return false
		}
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
		label = b.Language()
		detail = strings.TrimSpace(strings.TrimPrefix(b.Info(), b.Language()))
	}
	p.specialBlock(snap, b.Start, b.End, kind, label, detail, body)
	return true
}

func (p *previewLayout) codeBlock(snap editor.Snapshot, i int) {
	b := p.document.Block(i)
	body := strings.TrimSuffix(p.document.Lines(i), "\n")
	firstLine := snap.LineOf(b.Start)
	lastLine := snap.LineOf(max(b.End-1, b.Start))
	if b.Fenced {
		p.hideFenceLine(snap, firstLine)
		if !b.Open && lastLine > firstLine {
			p.hideFenceLine(snap, lastLine)
		}
	}

	for line := range p.document.LineCount(i) {
		_, start := p.document.Line(i, line)
		p.styles = append(p.styles, editor.LineStyle{
			Line:       snap.LineOf(start),
			Font:       portrait.Type.Code.Font(400),
			LineHeight: portrait.Type.Code.LineHeight(),
		})
	}

	label := b.Language()
	if label == "" {
		label = "Code"
	}
	headerLine := firstLine
	if b.Fenced && firstLine < lastLine {
		headerLine++
	}
	if n := p.document.LineCount(i); n > 0 {
		_, last := p.document.Line(i, n-1)
		p.code = append(p.code, codePanel{first: snap.LineStart(headerLine), last: snap.LineStart(snap.LineOf(last))})
	}
	p.blocks = append(p.blocks, editor.Block{
		Line:        headerLine,
		Above:       true,
		Height:      codeHeaderHeight,
		Interactive: true,
		Label:       "Code block: " + label,
		Content:     codeHeader(label, body),
	})
}

func (p *previewLayout) hideFenceLine(snap editor.Snapshot, line int) {
	start, end := snap.LineStart(line), snap.LineEnd(line)
	if end < snap.Len() && snap.Slice(end, end+1) == "\n" {
		end++
	}
	p.replace(start, end, "")
}

const codeHeaderHeight = 30

type codePanel struct{ first, last int }

func codeHeader(label, body string) func(*ui.Context, ui.Element, editor.Block) {
	return func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Height(codeHeaderHeight).Children(func() {
			ui.Row(c).Height(codeHeaderHeight).AlignItems(ui.Center).Gap(8).Children(func() {
				kicker(c, label, portrait.Colors.Ink3)
				ui.Spacer(c)
				copyButton(c, body)
			})
		})
	}
}

type copiedKey struct{}

func copyButton(c *ui.Context, body string) {
	b := ui.ButtonBase(c.Key("copy")).Label("Copy code").KeepFocus().Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
	portrait.Tooltip(c, b, "Copy code")
	ink := portrait.Press(b, portrait.Quiet)
	copied := ui.Local(b, copiedKey{}, func() time.Time { return time.Time{} })
	if b.Clicked() {
		c.Services().WriteClipboard(body)
		*copied = c.Now()
	}
	icon := portrait.IconCopy
	morphing := false
	if !copied.IsZero() && c.Now().Sub(*copied) < copiedFor {
		icon, ink, morphing = portrait.IconCheck, portrait.Colors.Accent, true
		c.After(copiedFor - c.Now().Sub(*copied))
	}
	b.Children(func() {
		if !morphing {
			portrait.Glyph(c, icon, 14, ink)
			return
		}
		from := ui.Motion{Opacity: 0.25, Scale: 0.76, Blur: 1.5}
		ui.Box(c.Key("check")).Transition(portrait.Transition(c, false, ui.ElementTransition{Duration: portrait.Motion.Morph, Ease: portrait.Motion.EaseOut.Easing(), Enter: &from})).Children(func() {
			portrait.Glyph(c, icon, 14, ink)
		})
	})
}

const copiedFor = 1200 * time.Millisecond

func kicker(c *ui.Context, label string, ink ui.Color) ui.Element {
	return portrait.Type.Meta.In(portrait.FontDisplay).Apply(ui.Text(c, strings.ToUpper(label))).FontWeight(500).LetterSpacing(0.04 * portrait.Type.Meta.Size).TextColor(ink).MaxLines(1).Ellipsis("…")
}

type blockLook struct {
	fill, ink, kickerInk ui.Color
	icon                 portrait.Icon
	rim, stripes         bool
	padY, padX           float32
	text                 portrait.TypeStyle
}

func lookOf(kind string) blockLook {
	paper := blockLook{fill: portrait.Colors.Code, ink: portrait.Colors.Ink, kickerInk: portrait.Colors.Ink3, rim: true, padY: 10, padX: 14, text: portrait.Type.Code}
	switch kind = strings.ToLower(kind); kind {
	case "note alert", "tip alert", "important alert", "warning alert", "caution alert":
		l := blockLook{fill: portrait.Colors.NoteSoft, ink: portrait.Colors.Ink, kickerInk: portrait.Colors.Note, padY: 10, padX: 14, text: portrait.Type.Reading}
		switch kind {
		case "note alert":
			l.icon = portrait.IconNote
		case "tip alert":
			l.icon = portrait.IconLightbulb
		case "important alert":
			l.icon = portrait.IconInfo
		case "warning alert":
			l.icon, l.fill, l.kickerInk = portrait.IconWarning, portrait.Colors.WarningSoft, portrait.Colors.Warning
		case "caution alert":
			l.icon, l.fill, l.kickerInk = portrait.IconWarningOctagon, portrait.Colors.CautionSoft, portrait.Colors.Caution
		}
		return l
	case "note":
		return blockLook{fill: portrait.Colors.Fill, ink: portrait.Colors.Ink2, kickerInk: portrait.Colors.Accent, icon: portrait.IconNote, padY: 8, padX: 10, text: portrait.Type.Small}
	case "footnote":
		return blockLook{fill: portrait.Colors.Fill, ink: portrait.Colors.Ink2, kickerInk: portrait.Colors.Accent, padY: 8, padX: 10, text: portrait.Type.Small}
	case "html", "frontmatter", "reference definition":
		paper.fill, paper.stripes, paper.ink = ui.Transparent, true, portrait.Colors.Ink2
		if kind == "frontmatter" {
			paper.stripes, paper.text = false, portrait.Type.Control
		}
		return paper
	}
	return paper
}

func (l blockLook) paint(e ui.Element) ui.Element {
	e.Radius(8).Padding(l.padY, l.padX).Gap(4).Clip()
	if l.fill.A > 0 {
		e.Background(l.fill)
	}
	if l.stripes {
		e.Stripes(portrait.Colors.Fill, 1, 6, 135)
	}
	if l.rim {
		e.InsetShadow(0, 0, 0, 0.5, portrait.Colors.Line)
	}
	return e
}

func (p *previewLayout) specialBlock(snap editor.Snapshot, start, end int, kind, label, detail, body string) {
	d := p.owner
	if d.draft != nil && d.draft.anchor == start && d.draft.kind == kind {
		p.draftAnchored = true
	}
	look := lookOf(kind)
	p.standIn(snap, start, end, body, editor.Block{Label: label + " block", Height: 80, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Children(func() {
			frame := look.paint(ui.Column(c).FillWidth())
			frame.Children(func() {
				if draft := d.draft; draft != nil && draft.anchor == start && draft.kind == kind {
					d.drawDraft(c, draft)
					return
				}
				var art blockImage
				if kind == "math" || kind == "Mermaid" {
					art = d.renderImage(kind, body)
				}
				heading := func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						if look.icon.Glyph != "" {
							portrait.Glyph(c, look.icon, 14, look.kickerInk)
						}
						name := strings.TrimSuffix(label, " alert")
						k := kicker(c, name, look.kickerInk)
						if look.icon.Glyph != "" {
							k.FontWeight(600).LetterSpacing(0.07 * portrait.Type.Meta.Size)
						}
						if detail != "" {
							portrait.Type.Meta.In(portrait.FontMono).Apply(ui.Text(c, detail)).TextColor(portrait.Colors.Ink3)
						}
						ui.Spacer(c)
						d.blockActions(c, e, snap, start, end, kind, label)
					})
				}
				switch {
				case art.reason != "":
					heading()
					portrait.Type.Meta.Apply(ui.Text(c, label+" could not render: "+art.reason)).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).Wrap()
					portrait.Type.Code.Apply(ui.Text(c, snap.Slice(start, end))).TextColor(portrait.Colors.Ink).Wrap()
				case art.svg != nil:
					heading()
					w, h := art.svg.Size()
					scale := min(float32(1), 600/max(1, w), 360/max(1, h))
					name := "Typeset equation"
					if kind == "Mermaid" {
						name = "Mermaid diagram"
					}
					ui.Row(c).FillWidth().Justify(ui.Center).Padding(4, 2).Children(func() {
						ui.Image(c, art.svg).Size(max(1, w*scale), max(1, h*scale)).Label(name).Tooltip(body)
					})
				case kind == "frontmatter":
					heading()
					frontmatterRows(c, body)
				case kind == "code":
					heading()
					look.text.Apply(ui.Text(c, body)).TextColor(look.ink).Wrap().PreserveSpaces().Selectable()
				case kind == "footnote" || kind == "note":
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
						if kind == "note" {
							portrait.Glyph(c, look.icon, 15, look.kickerInk).Margin(3, 0, 0, 0)
						} else {
							portrait.Type.Meta.In(portrait.FontDisplay).Apply(ui.Text(c, label)).FontFeatures("tnum").TextColor(look.kickerInk).Margin(3, 0, 0, 0)
						}
						look.text.Apply(ui.Text(c, body)).TextColor(look.ink).Wrap().Selectable().Grow(1).Shrink(1).MinWidth(0)
						d.blockActions(c, e, snap, start, end, kind, label)
					})
				default:
					heading()
					look.text.Apply(ui.Text(c, body)).TextColor(look.ink).Wrap().Selectable()
				}
			})
		})
	}})
}

func (d *Document) blockActions(c *ui.Context, block ui.Element, snap editor.Snapshot, start, end int, kind, label string) {
	row := ui.Row(c).Gap(4).AlignItems(ui.Center)
	shown := block.Hovered() || block.FocusWithin() || !c.Preferences().Hover()
	row.Opacity(row.AnimateTo(actionsKey{}, flag(shown), ui.Animation{Duration: portrait.Motion.Switch, Ease: portrait.Motion.EaseOut.Easing(), NonMotion: true}))
	row.Children(func() {
		if kind == "footnote" && smallButton(c, "Return to reference "+label, "Return to reference "+label, portrait.Raised).Clicked() {
			d.preview.returnTo(label)
			c.Invalidate()
		}
		if kind == "note" && !d.state.ReadOnly && smallButton(c, "Delete note", "Delete note", portrait.Raised).Clicked() {
			d.deleteNote(snap, start, end)
			c.Invalidate()
		}
		if !d.state.ReadOnly && smallButton(c, "Edit", "Edit "+kind, portrait.Quiet).Clicked() && d.Version() == snap.Version() {
			d.openDraft(start, end, kind)
		}
	})
}

type actionsKey struct{}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func smallButton(c *ui.Context, title, label string, look portrait.Look) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Height(22).Padding(0, 8).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
	ink := portrait.Press(b, look)
	b.Children(func() { portrait.Type.Meta.Apply(ui.Text(c, title)).FontWeight(500).TextColor(ink).MaxLines(1) })
	return b
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
	in := ui.Motion{Y: -3, Scale: 0.99, Blur: 1.5}
	ui.Column(c.Key("composer")).FillWidth().TransformOrigin(0.5, 0).Transition(portrait.Transition(c, false, ui.ElementTransition{Duration: portrait.Motion.Expand, Ease: portrait.Motion.EaseOut.Easing(), Enter: &in})).Children(func() {
		d.composer(c, draft)
	})
}

func (d *Document) composer(c *ui.Context, draft *blockDraft) {
	name := "Markdown block editor"
	if draft.kind == "link" {
		name = "Link destination"
	} else if draft.kind == "note" || draft.kind == "new note" {
		name = "Note"
	}
	entry := ui.TextArea(c, &draft.text).Label(name).Height(100).FillWidth().Radius(8).Background(portrait.Colors.Paper)
	portrait.Rings.Editing.Apply(entry)
	if draft.focus {
		entry.Focus()
		draft.focus = false
	}
	composing := entry.Composing()
	save, cancel := entry.Shortcut(ui.Cmd, ui.KeyEnter) && !composing, entry.Shortcut(0, ui.KeyEscape) && !composing
	ui.Row(c).Gap(6).Margin(8, 0, 0, 0).Children(func() {
		emptyLink := draft.kind == "link" && strings.TrimSpace(draft.text) == ""
		emptyNote := (draft.kind == "note" || draft.kind == "new note") && strings.TrimSpace(draft.text) == ""
		if portrait.AccentButton(c, "Save", "Save").Disabled(d.state.ReadOnly || emptyLink || emptyNote).Clicked() {
			save = true
		}
		if portrait.Button(c, "Cancel", "Cancel").Clicked() {
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
