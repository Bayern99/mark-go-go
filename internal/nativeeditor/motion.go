package nativeeditor

import (
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type jumpHighlight struct {
	active  bool
	at      int
	version uint64
	start   time.Time
}

type foldMotion struct {
	key            string
	folding        bool
	start          time.Time
	height         float32
	text           string
	body, end      int
	version        uint64
	headingLeading float32
}

func (m *foldMotion) progress(c *ui.Context) float32 {
	if m.start.IsZero() {
		m.start = c.Now()
	}
	t := float32(c.Now().Sub(m.start)) / float32(portrait.Motion.Expand)
	if t >= 1 {
		return 1
	}
	c.AnimationFrame()
	return portrait.Motion.EaseOut.Easing()(max(t, 0))
}

var unpainted = portrait.Clear

type pageFill struct {
	rect        ui.Rect
	left, right float32
	radius      float32
	color, rim  ui.Color
}

type page struct {
	paper  ui.Color
	column float32
	fills  []pageFill
}

func (pg page) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	p.FillRadii(box, radii, pg.paper)
	for _, f := range pg.fills {
		r := f.rect
		r.X += box.X
		r.Y += box.Y
		if r.W <= 0 {
			right := box.X + box.W - 40
			if pg.column > 0 {
				right = min(r.X+pg.column, right)
			}
			r.W = right - r.X
		}
		r.X -= f.left
		r.W += f.left + f.right
		if r.W > 0 && r.H > 0 {
			p.Fill(r, f.color, f.radius)
			if f.rim.A > 0 {
				p.Stroke(r, f.rim, f.radius, 0.5)
			}
		}
	}
}

func (d *Document) Reveal(offset int) error {
	if err := d.SelectUTF16(offset, offset); err != nil {
		return err
	}
	d.landing = jumpHighlight{active: true, at: d.state.Selection().Head, version: d.Version()}
	return nil
}

func (d *Document) decorate(c *ui.Context, field ui.Element) {
	d.page = d.page[:0]
	column := float32(0)
	if d.mode == Preview {
		column = d.state.ReadingColumn.Width
		d.codePanels()
	}
	d.jumpFill(c)
	d.expandVeil(c, field)
	field.Material(page{paper: c.Theme().Background, column: column, fills: append([]pageFill(nil), d.page...)})
}

func (d *Document) codePanels() {
	for _, cp := range d.preview.code {
		top, ok := d.state.Rect(cp.first)
		if !ok {
			continue
		}
		last, ok := d.state.Rect(cp.last)
		if !ok {
			continue
		}
		r := ui.Rect{X: top.X, Y: top.Y - codeHeaderHeight, H: last.Y + last.H + 12 - (top.Y - codeHeaderHeight)}
		d.page = append(d.page, pageFill{rect: r, left: 16, radius: 8, color: portrait.Colors.Code, rim: portrait.Colors.Line})
	}
}

func (d *Document) jumpFill(c *ui.Context) {
	j := &d.landing
	if !j.active {
		return
	}
	if j.version != d.Version() || !portrait.Animates(c, false) {
		j.active = false
		return
	}
	if j.start.IsZero() {
		j.start = c.Now()
	}
	h := portrait.Highlights.Jump
	elapsed := c.Now().Sub(j.start)
	if elapsed >= h.Duration {
		j.active = false
		return
	}
	c.AnimationFrame()
	snap := d.state.Buffer.Snapshot()
	line := snap.LineOf(min(j.at, snap.Len()))
	first, ok := d.state.Rect(snap.LineStart(line))
	if !ok {
		return
	}
	last, ok := d.state.Rect(snap.LineEnd(line))
	if !ok {
		last = first
	}
	r := ui.Rect{X: first.X, Y: first.Y, H: last.Y + last.H - first.Y}
	d.page = append(d.page, pageFill{rect: r, left: 8, right: 8, radius: h.Radius, color: h.At(elapsed)})
}

func (d *Document) expandVeil(c *ui.Context, field ui.Element) {
	m := d.fold
	if m == nil || m.folding {
		return
	}
	if m.version != d.Version() || d.mode != Preview {
		d.fold = nil
		return
	}
	f := m.progress(c)
	if f >= 1 {
		d.fold = nil
		return
	}
	top, ok := d.state.Rect(m.body)
	if !ok {
		return
	}
	bottom, ok := d.state.Rect(m.end)
	if !ok {
		return
	}
	y1 := bottom.Y
	if m.end >= d.state.Buffer.Len() {
		y1 += bottom.H
	}
	paper := c.Theme().Background
	field.DrawOver(func(p *ui.Painter, r ui.Rect) {
		h := y1 - top.Y
		if h <= 0 {
			return
		}
		lifted := top.Y + h*f
		fade := paper
		fade.A = uint8(float32(paper.A)*(1-f) + 0.5)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + top.Y, W: r.W, H: lifted - top.Y}, fade, 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + lifted, W: r.W, H: y1 - lifted}, paper, 0)
	})
}

func (p *previewLayout) startFold(section foldSection) {
	d := p.owner
	m := &foldMotion{key: section.key, folding: d.folded[section.key], body: section.body, end: section.end, version: d.Version()}
	if m.folding {
		top, ok := d.state.Rect(section.body)
		bottom, ok2 := d.state.Rect(section.end)
		if !ok || !ok2 {
			return
		}
		m.height = bottom.Y - top.Y
		if section.end >= d.state.Buffer.Len() {
			m.height += bottom.H
		}
		m.text = strings.TrimSpace(p.display[p.displayOffset(section.body):p.displayOffset(section.end)])
	}
	d.fold = m
}

func (p *previewLayout) foldGhost(line int, key string) (editor.Block, bool) {
	d := p.owner
	m := d.fold
	if m == nil || !m.folding || m.key != key || m.version != d.Version() {
		return editor.Block{}, false
	}
	return editor.Block{Line: line, Height: m.height, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		if d.fold != m {
			e.Height(0)
			return
		}
		f := m.progress(c)
		if f >= 1 {
			d.fold = nil
			d.preview.dirty = true
			e.Height(0)
			c.Invalidate()
			return
		}
		e.Height(m.height * (1 - f)).Clip().AccessHidden(true).IgnoreInput(true)
		e.Children(func() {
			portrait.Type.Reading.Apply(ui.Text(c, m.text)).Wrap().PreserveSpaces().TextColor(portrait.Colors.Ink).Opacity(1 - f).FillWidth()
		})
	}}, true
}
