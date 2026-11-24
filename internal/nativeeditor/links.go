package nativeeditor

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// previewLink is the link at a position of Preview, with the source ranges
// its actions edit.
type previewLink struct {
	link                             parser.Inline
	destination                      string
	labelStart, labelEnd             int
	destinationStart, destinationEnd int
	canOpen, canEdit, canRemove      bool
}

// linkAt returns the link under an offset of Preview, as the pointer finds
// one: the inline itself, its label and destination, and which actions it
// takes.
func (d *Document) linkAt(offset int) (previewLink, bool) {
	if d.mode != Preview || d.preview.document == nil || d.preview.version != d.Version() {
		return previewLink{}, false
	}
	snapshot := d.state.Buffer.Snapshot()
	source := snapshot.String()
	for i := 1; i < d.preview.document.Len(); i++ {
		block := d.preview.document.Block(i)
		if offset < block.Start || offset > block.End {
			continue
		}
		for _, in := range d.preview.document.Inlines(i) {
			if in.Kind != parser.Link || offset < in.Start || offset > in.End {
				continue
			}
			found := previewLink{link: in, destination: in.Dest, canOpen: ui.SafeToOpen(in.Dest)}
			found.labelStart, found.labelEnd, found.canRemove = inlineLinkLabelRange(source, in)
			found.canRemove = found.canRemove && !in.Auto
			found.destinationStart, found.destinationEnd, found.canEdit = inlineDestinationRange(source, in)
			if !found.canEdit {
				if label, ok := referenceLinkLabel(source, in); ok {
					found.destinationStart, found.destinationEnd, found.canEdit = d.preview.referenceDestination(snapshot, label)
				}
			}
			return found, true
		}
	}
	return previewLink{}, false
}

func (d *Document) previewHover(req editor.HoverRequest) {
	if req.Version != d.Version() {
		return
	}
	found, ok := d.linkAt(req.Offset)
	if !ok {
		return
	}
	in, destination := found.link, found.destination
	// The card draws the URL itself as its mono line: the hover's own
	// title would show it twice.
	d.state.ShowHover(in.Start, in.End, "", func(c *ui.Context) {
		if draft := d.draft; draft != nil && draft.kind == "link" && draft.anchor == in.Start {
			d.drawDraft(c, draft)
			return
		}
		if d.Version() != req.Version {
			portrait.Alert(c, portrait.Tones.Warning, "Link changed", "Reopen the link after the document settles.")
			return
		}
		ui.Column(c).Gap(portrait.Form.PopoverGap).Children(func() {
			portrait.Type.CodeSmall.Apply(ui.Text(c, destination)).TextColor(portrait.Colors.Ink3).MaxLines(1).Ellipsis("…").Padding(0, portrait.Form.PadX)
			if found.canOpen {
				open := portrait.ListRow(c, portrait.ListRowSpec{Icon: portrait.IconArrowUpRight, Title: "Open link", Label: "Open link"})
				open.Description(destination)
				if open.Clicked() {
					c.Services().OpenURL(destination)
				}
			}
			if portrait.ListRow(c, portrait.ListRowSpec{Icon: portrait.IconCopy, Title: "Copy link", Label: "Copy link"}).Clicked() {
				c.Services().WriteClipboard(destination)
			}
			if d.state.ReadOnly {
				return
			}
			if found.canEdit && portrait.ListRow(c, portrait.ListRowSpec{Icon: portrait.IconPencil, Title: "Edit link", Label: "Edit link"}).Clicked() && d.Version() == req.Version {
				d.openDraft(found.destinationStart, found.destinationEnd, "link")
				d.draft.anchor = in.Start
				c.Invalidate()
			}
			if found.canRemove {
				remove := portrait.ListRow(c, portrait.ListRowSpec{Icon: portrait.IconTrash, Title: "Remove link", Label: "Remove link"})
				if remove.Clicked() && d.Version() == req.Version && d.removeLink(req.Version, in, found.labelStart, found.labelEnd) == nil {
					d.refocus = true
					c.Invalidate()
				}
			}
		})
	})
}

// previewLinkMenu puts the link's actions in the context menu too, so the
// keyboard reaches what the pointer's hover card shows: the same open, copy,
// edit and remove, on the link the caret is in. It reports whether it added
// them.
func (d *Document) previewLinkMenu(services ui.Services, m *ui.Menu) bool {
	found, ok := d.linkAt(d.state.Cursor())
	if !ok {
		return false
	}
	if found.canOpen {
		if m.Item("Open link").Chosen() {
			services.OpenURL(found.destination)
		}
	}
	if m.Item("Copy link").Chosen() {
		services.WriteClipboard(found.destination)
	}
	if d.state.ReadOnly {
		return true
	}
	if found.canEdit && m.Item("Edit link").Chosen() {
		d.openDraft(found.destinationStart, found.destinationEnd, "link")
		d.draft.anchor = found.link.Start
		// No hover card shows the draft here, so the link's own line
		// carries it instead.
		d.preview.dirty = true
		services.Invalidate()
	}
	if found.canRemove && m.Item("Remove link").Chosen() && d.removeLink(d.Version(), found.link, found.labelStart, found.labelEnd) == nil {
		d.refocus = true
		services.Invalidate()
	}
	return true
}

func inlineLinkLabelRange(source string, link parser.Inline) (start, end int, ok bool) {
	if link.Start < 0 || link.End > len(source) || link.End < link.Start {
		return 0, 0, false
	}
	raw := source[link.Start:link.End]
	if link.Auto {
		if len(raw) >= 2 && raw[0] == '<' && raw[len(raw)-1] == '>' {
			return link.Start + 1, link.End - 1, true
		}
		return link.Start, link.End, true
	}
	if raw == "" || raw[0] != '[' {
		return 0, 0, false
	}
	close := closingLinkLabel(raw)
	if close < 0 {
		return 0, 0, false
	}
	return link.Start + 1, link.Start + close, true
}

func inlineDestinationRange(source string, link parser.Inline) (start, end int, ok bool) {
	if link.Start < 0 || link.End > len(source) || link.End < link.Start {
		return 0, 0, false
	}
	raw := source[link.Start:link.End]
	if link.Auto {
		if len(raw) >= 2 && raw[0] == '<' && raw[len(raw)-1] == '>' {
			return link.Start + 1, link.End - 1, true
		}
		return link.Start, link.End, true
	}
	closeLabel := closingLinkLabel(raw)
	if closeLabel < 0 {
		return 0, 0, false
	}
	i := closeLabel + 1
	for i < len(raw) && linkSpace(raw[i]) {
		i++
	}
	if i >= len(raw) || raw[i] != '(' {
		return 0, 0, false
	}
	i++
	for i < len(raw) && linkSpace(raw[i]) {
		i++
	}
	if i >= len(raw) || raw[i] == ')' {
		return link.Start + i, link.Start + i, true
	}
	if raw[i] == '<' {
		i++
		start = i
		for i < len(raw) {
			if raw[i] == '\\' && i+1 < len(raw) {
				i += 2
				continue
			}
			if raw[i] == '>' {
				return link.Start + start, link.Start + i, true
			}
			i++
		}
		return 0, 0, false
	}
	start = i
	depth := 0
	for i < len(raw) {
		if raw[i] == '\\' && i+1 < len(raw) {
			i += 2
			continue
		}
		switch raw[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return link.Start + start, link.Start + i, true
			}
			depth--
		default:
			if depth == 0 && linkSpace(raw[i]) {
				return link.Start + start, link.Start + i, true
			}
		}
		i++
	}
	return 0, 0, false
}

func referenceLinkLabel(source string, link parser.Inline) (string, bool) {
	if link.Start < 0 || link.End > len(source) || link.End < link.Start {
		return "", false
	}
	raw := source[link.Start:link.End]
	close := closingLinkLabel(raw)
	if close < 0 {
		return "", false
	}
	label := raw[1:close]
	i := close + 1
	for i < len(raw) && linkSpace(raw[i]) {
		i++
	}
	if i < len(raw) && raw[i] == '(' {
		return "", false
	}
	if i < len(raw) && raw[i] == '[' {
		closeReference := closingLinkLabel(raw[i:])
		if closeReference < 0 {
			return "", false
		}
		if reference := raw[i+1 : i+closeReference]; reference != "" {
			label = reference
		}
	}
	label = parser.NormalizeLabel(label)
	return label, label != ""
}

func closingLinkLabel(raw string) int {
	if len(raw) == 0 || raw[0] != '[' {
		return -1
	}
	depth := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			continue
		}
		if raw[i] == '`' {
			run := 1
			for i+run < len(raw) && raw[i+run] == '`' {
				run++
			}
			if end := matchingBacktickRun(raw, i+run, run); end >= 0 {
				i = end + run - 1
				continue
			}
			i += run - 1
			continue
		}
		switch raw[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func matchingBacktickRun(source string, start, run int) int {
	for i := start; i < len(source); {
		if source[i] != '`' {
			i++
			continue
		}
		end := i + 1
		for end < len(source) && source[end] == '`' {
			end++
		}
		if end-i == run {
			return i
		}
		i = end
	}
	return -1
}

func linkSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func (d *Document) removeLink(revision uint64, link parser.Inline, labelStart, labelEnd int) error {
	idx := d.sourceIndex()
	if idx.version != revision {
		return ErrRevision
	}
	start, end := idx.rawAtEditor(link.Start), idx.rawAtEditor(link.End)
	textStart, textEnd := idx.rawAtEditor(labelStart), idx.rawAtEditor(labelEnd)
	return d.Apply(revision, start, end, idx.raw[start:end], idx.raw[textStart:textEnd])
}

func linkMarker(destination string) func(*ui.Context, ui.Element, editor.Replacement) {
	return func(c *ui.Context, e ui.Element, _ editor.Replacement) {
		e.AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			portrait.Tooltip(c, portrait.IconArrowUpRight.Apply(ui.Link(c, portrait.IconArrowUpRight.Glyph, destination), portrait.IconSizes.S).Label("Open "+destination), destination)
		})
	}
}
