package nativeeditor

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (d *Document) previewHover(req editor.HoverRequest) {
	if d.mode != Preview || req.Version != d.Version() || d.preview.document == nil || d.preview.version != req.Version {
		return
	}
	for i := 1; i < d.preview.document.Len(); i++ {
		block := d.preview.document.Block(i)
		if req.Offset < block.Start || req.Offset > block.End {
			continue
		}
		for _, in := range d.preview.document.Inlines(i) {
			if in.Kind != parser.Link || req.Offset < in.Start || req.Offset > in.End {
				continue
			}
			destination := in.Dest
			canOpen := ui.SafeToOpen(destination)
			d.state.ShowHover(in.Start, in.End, destination, func(c *ui.Context) {
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, destination).Font("IBM Plex Mono").FontSize(12).Wrap().Selectable()
					if canOpen {
						ui.Link(c, "Open link", destination).Description(destination)
					}
					if ui.Button(c, "Copy link").Clicked() {
						c.Services().WriteClipboard(destination)
					}
				})
			})
			return
		}
	}
}

func linkMarker(destination string) func(*ui.Context, ui.Element, editor.Replacement) {
	return func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.Children(func() {
			ui.Link(c, "↗", destination).Label("Open " + destination).Description(destination)
		})
	}
}
