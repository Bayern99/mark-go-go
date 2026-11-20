package nativeeditor

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type foldSection struct {
	start, body, end int
	key              string
}

func (p *previewLayout) heading(snap editor.Snapshot, i, firstText int) {
	b := p.document.Block(i)
	title := strings.TrimSpace(p.document.Lines(i))
	key := headingKey(b.Level, title, p.headings[title])
	p.headings[title]++
	end := snap.Len()
	for j := b.Next; j < p.document.Len(); j++ {
		next := p.document.Block(j)
		if next.Kind == parser.Heading && next.Level <= b.Level {
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
	action, icon := "Fold ", "▾"
	if folded {
		action, icon = "Expand ", "▸"
		p.foldUntil = end
		p.replace(from, end, "")
		p.hiddenRanges = append(p.hiddenRanges, [2]int{from, end})
	}
	p.replace(b.Start, firstText, "")
	p.blocks = append(p.blocks, editor.Block{Line: snap.LineOf(b.Start), Above: true, Height: 1, Interactive: true, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Height(1).Children(func() {
			btn := ui.ButtonBase(c).Absolute().Left(-24).Top(3).Label(action + title).Width(19).Height(22).KeepFocus()
			btn.Children(func() { ui.Text(c, icon).FontSize(12) })
			if btn.Clicked() {
				p.toggleFold(key)
				c.Invalidate()
			}
		})
	}})
}

func (p *previewLayout) toggleFold(key string) {
	if p.owner.folded == nil {
		p.owner.folded = map[string]bool{}
	}
	p.owner.folded[key] = !p.owner.folded[key]
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
		if ev.Mods == ui.Cmd|ui.Alt && off >= section.start && (off < section.end || off == d.state.Buffer.Len()) {
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
	p.replacements = append(p.replacements, editor.Replacement{Start: in.Start, End: in.End, Width: max(28, float32(len(shown))*8), Interactive: true, Label: label, Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.Children(func() {
			if ui.Button(c, shown).Label(label).KeepFocus().Clicked() {
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
