package nativeeditor

import (
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

type revealScroll struct {
	active  bool
	at      int
	version uint64
	passes  int
}

func (d *Document) Reveal(offset int) error {
	if err := d.SelectUTF16(offset, offset); err != nil {
		return err
	}
	head := d.state.Selection().Head
	d.landing = jumpHighlight{active: true, at: head, version: d.Version()}
	d.revealScroll = revealScroll{active: true, at: head, version: d.Version(), passes: 3}
	return nil
}

func (d *Document) applyRevealScroll(c *ui.Context) {
	r := &d.revealScroll
	if !r.active {
		return
	}
	if r.version != d.Version() || d.state.Selection().Head != r.at {
		r.active = false
		return
	}
	vh := d.state.ViewportHeight()
	if vh <= 0 {
		c.Invalidate()
		return
	}
	rect, ok := d.state.Rect(r.at)
	if !ok {
		r.passes--
		if r.passes <= 0 {
			r.active = false
		} else {
			c.Invalidate()
		}
		return
	}
	snap := d.state.Buffer.Snapshot()
	line := snap.LineOf(min(r.at, snap.Len()))
	target := 0.0
	if line > 0 {
		inset := min(float64(portrait.Sizes.Large), vh/4)
		target = max(0, d.state.ScrollY()+float64(rect.Y)-inset)
	}
	d.state.SetScrollY(target)
	r.passes--
	if r.passes <= 0 {
		r.active = false
	} else {
		c.Invalidate()
	}
}

// decorate gives the editor the line backgrounds it paints in its own text
// geometry. Keeping them here, rather than drawing around the editor, makes
// panels follow wrapping, folds and scrolling in the same frame as the text.
func (d *Document) decorate(c *ui.Context) {
	d.state.LineBackgrounds = append([]editor.LineBackground(nil), d.preview.backgrounds...)
	if d.mode == Preview {
		d.jumpFill(c)
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
	d.state.LineBackgrounds = append(d.state.LineBackgrounds, editor.LineBackground{
		First: line, Last: line,
		Color: h.At(elapsed), Radius: h.Radius,
		ReadingColumn: true, PaddingLeft: 8, PaddingRight: 8,
	})
}
