package nativeeditor

import (
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

type jumpHighlight struct {
	active  bool
	at      int
	version uint64
	start   time.Time
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
