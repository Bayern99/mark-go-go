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
	action := "Fold "
	if folded {
		action = "Expand "
		p.foldUntil = end
		p.replace(from, end, "")
		p.hiddenRanges = append(p.hiddenRanges, [2]int{from, end})
	}
	p.replace(b.Start, firstText, "")
	style := headingStyle(0, b.Level)
	headingLine := snap.LineOf(b.Start)
	leading := style.LineHeight * style.Font.Size
	mark := foldMarkSize(style.Font.Size)
	p.blocks = append(p.blocks, editor.Block{Line: snap.LineOf(b.Start), Above: true, Height: portrait.Editor.Hairline, Interactive: true, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		e.Height(portrait.Editor.Hairline).Children(func() {
			btn := ui.ButtonBase(c).Absolute().Left(-foldToggleSize-foldToggleGap).Top((leading-foldToggleSize)/2).Label(action+title).Size(foldToggleSize, foldToggleSize).Radius(portrait.Radii.S).KeepFocus().AlignItems(ui.Center).Justify(ui.Center)
			portrait.Tooltip(c, btn, action+title)
			ink := portrait.Press(btn, portrait.Quiet)
			if !btn.Hovered() && !btn.Pressed() {
				ink = portrait.Colors.Ink3
			}
			selection := p.owner.state.Selection()
			selectionOnHeading := selection.Start() <= snap.LineEnd(headingLine) && selection.End() >= snap.LineStart(headingLine)
			showMark := folded || btn.Hovered() || btn.Pressed() || btn.FocusVisible() || !c.Preferences().Hover() || selectionOnHeading
			btn.Children(func() {
				ui.Box(c).Size(mark, mark).Shrink(0).AccessHidden(true).Draw(func(p *ui.Painter, r ui.Rect) {
					if showMark {
						drawFoldMark(p, r, folded, ink)
					}
				})
			})
			if btn.Clicked() {
				p.toggleFold(key)
				c.Invalidate()
			}
		})
	}})
}

// The fold toggle is a compact control in the reading gutter, a hair clear of
// the heading's first glyph.
var foldToggleSize, foldToggleGap = portrait.Sizes.Compact, portrait.Editor.Hairline * 4

// foldMarkSize scales the disclosure triangle with its heading, so an H1 does
// not wear the same small mark as body-sized headings.
func foldMarkSize(fontSize float32) float32 {
	return 4.5 + fontSize*0.2
}

// drawFoldMark draws a filled disclosure triangle with softened corners,
// pointing down while the section is open and right once it is folded. Its
// optical centre sits on the box's centre in both directions.
func drawFoldMark(p *ui.Painter, r ui.Rect, folded bool, ink ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	long, short := r.W*0.86, r.W*0.62
	var mark ui.Path
	if folded {
		mark.MoveTo(cx-short*0.42, cy-long/2).LineTo(cx+short*0.58, cy).LineTo(cx-short*0.42, cy+long/2).Close()
	} else {
		mark.MoveTo(cx-long/2, cy-short*0.42).LineTo(cx+long/2, cy-short*0.42).LineTo(cx, cy+short*0.58).Close()
	}
	mark.Join(ui.RoundJoin)
	p.FillPath(&mark, ink)
	p.StrokePath(&mark, r.W*0.16, ink)
}

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
	p.reading(in.Start, shown)
	target := p.document.Block(at).Start
	p.replacements = append(p.replacements, editor.Replacement{Start: p.at(in.Start), End: p.at(in.End), Width: max(18, float32(len(shown))*7), Interactive: true, Label: label, Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.Children(func() {
			ref := ui.ButtonBase(c).Label(label).KeepFocus().Padding(0, 2).AlignItems(ui.Center)
			ref.Children(func() {
				portrait.Type.Meta.Apply(ui.Text(c, shown)).FontFeatures("tnum").TextColor(portrait.Colors.Accent)
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
