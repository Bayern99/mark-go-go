package nativeeditor

import (
	"fmt"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type foldSection struct {
	start, body, end int
	key              string
}

func (s foldSection) contains(off, end int) bool {
	return off >= s.start && (off < s.end || (off == s.end && off == end))
}

func (p *previewLayout) heading(snap editor.Snapshot, i, firstText int) {
	b := p.document.Block(i)
	if b.Parent != 0 {
		p.replace(b.Start, firstText, "")
		return
	}
	title := strings.TrimSpace(p.document.Lines(i))
	key := headingKey(b.Level, title, p.headings[title])
	p.headings[title]++
	end := snap.Len()
	for j := b.Next; j < p.document.Len(); j++ {
		next := p.document.Block(j)
		if next.Parent == 0 && next.Kind == parser.Heading && next.Level <= b.Level {
			end = snap.LineStart(snap.LineOf(next.Start))
			break
		}
	}
	from := snap.LineEnd(snap.LineOf(b.End))
	if from < end && snap.Slice(from, from+1) == "\n" {
		from++
	}
	if from >= end || strings.TrimSpace(snap.Slice(from, end)) == "" {
		p.replace(b.Start, firstText, "")
		return
	}
	p.sections = append(p.sections, foldSection{b.Start, from, end, key})
	folded := p.owner.folded[key]
	action, icon := "Fold ", portrait.IconCaretDown
	if folded {
		action, icon = "Expand ", portrait.IconCaretRight
		p.foldUntil = end
		p.replace(from, end, "")
		p.hiddenRanges = append(p.hiddenRanges, [2]int{from, end})
	}
	p.replace(b.Start, firstText, "")
	section := p.sections[len(p.sections)-1]
	leading := headingStyle(0, b.Level).LineHeight * headingStyle(0, b.Level).Font.Size
	p.blocks = append(p.blocks, editor.Block{Line: snap.LineOf(b.Start), Above: true, Height: 1, Interactive: true, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Height(1).Children(func() {
			btn := ui.ButtonBase(c).Absolute().Left(-26).Top(1+(leading-foldToggleSize)/2).Label(action+title).Size(foldToggleSize, foldToggleSize).Radius(5).KeepFocus().AlignItems(ui.Center).Justify(ui.Center)
			portrait.Tooltip(c, btn, action+title)
			ink := portrait.Press(btn, portrait.Quiet)
			if !btn.Hovered() && !btn.Pressed() {
				ink = portrait.Colors.Ink3
			}
			btn.Children(func() { portrait.Glyph(c, icon, 14, ink) })
			if btn.Clicked() {
				p.toggleFold(key)
				if portrait.Animates(c, btn.FocusVisible()) {
					p.startFold(section)
				}
				c.Invalidate()
			}
		})
	}})
	if folded {
		if ghost, ok := p.foldGhost(snap.LineOf(b.End), key); ok {
			p.blocks = append(p.blocks, ghost)
		}
	}
}

// foldToggleSize is the side of a heading's fold toggle (.mgo-fold-toggle).
const foldToggleSize = 18

func (p *previewLayout) toggleFold(key string) {
	if p.owner.folded == nil {
		p.owner.folded = map[string]bool{}
	}
	folded := !p.owner.folded[key]
	p.owner.folded[key] = folded
	if folded {
		sel := p.owner.state.Selection()
		end := p.owner.state.Buffer.Len()
		for _, section := range p.sections {
			if section.key != key {
				continue
			}
			if section.contains(sel.Anchor, end) || section.contains(sel.Head, end) || (min(sel.Anchor, sel.Head) < section.start && max(sel.Anchor, sel.Head) >= section.end) {
				p.owner.state.Select(section.start, section.start)
			}
			break
		}
	}
	p.saveFolds()
	p.dirty = true
}

func (d *Document) previewInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	off := d.state.Cursor()
	for i := len(d.preview.sections) - 1; i >= 0; i-- {
		section := d.preview.sections[i]
		folded := d.folded[section.key]
		boundary := ev.Mods == 0 && ((ev.Key == ui.KeyBackspace && off == section.end) || (ev.Key == ui.KeyDelete && off == section.body))
		if folded && boundary {
			d.preview.toggleFold(section.key)
			return true
		}
		if ev.Mods == ui.Cmd|ui.Alt && section.contains(off, d.state.Buffer.Len()) {
			if ev.Key == ui.KeyLeft || (ev.Key == ui.KeyRight && folded) {
				d.preview.toggleFold(section.key)
				if !folded {
					d.state.Select(section.start, section.start)
				}
				return true
			}
		}
	}
	return false
}

func (p *previewLayout) footnote(in parser.Inline) {
	at, ok := p.document.Footnote(in.Text)
	if !ok {
		return
	}
	r := len(p.refs[in.Text])
	p.refs[in.Text] = append(p.refs[in.Text], in.Start)
	label := fmt.Sprintf("Read footnote %s, reference %d", in.Text, r+1)
	shown := "[" + in.Text + "]"
	p.readable[in.Start] = shown
	target := p.document.Block(at).Start
	p.replacements = append(p.replacements, editor.Replacement{Start: in.Start, End: in.End, Width: max(18, float32(len(shown))*7), Interactive: true, Label: label, Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.Children(func() {
			ref := ui.ButtonBase(c).Label(label).KeepFocus().Padding(0, 2).AlignItems(ui.Center)
			ref.Children(func() {
				portrait.Type.Meta.In(portrait.FontDisplay).Apply(ui.Text(c, shown)).FontFeatures("tnum").TextColor(portrait.Colors.Accent)
			})
			if ref.Clicked() {
				d := p.owner
				if d.returns == nil {
					d.returns = map[string]int{}
				}
				d.returns[in.Text] = r
				d.jump(target)
				c.Invalidate()
			}
		})
	}})
}
func (p *previewLayout) returnTo(label string) {
	refs := p.refs[label]
	if len(refs) > 0 {
		p.owner.jump(refs[min(len(refs)-1, p.owner.returns[label])])
	}
}
func (d *Document) jump(off int) {
	d.state.Select(off, off)
	d.state.ScrollTo(d.state.Buffer.Pos(off).Line)
	d.refocus = true
}
