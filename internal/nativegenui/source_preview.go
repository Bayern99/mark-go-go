package nativegenui

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type sourcePreviewState struct {
	key, cardID string
	open        bool
}

type sourceExcerptKey struct {
	source     string
	start, end int
}

type sourcePreviewTarget struct {
	anchor     *ui.Handle
	key, label string
	cardID     string
	ref        Reference
	section    workspace.Section
	node       Node
	found      bool
}

const sourceExcerptLimit = 600

func (v *documentView) toggleSourcePreview(key, cardID string) {
	if v.previewSource.open && v.previewSource.key == key {
		v.previewSource = sourcePreviewState{}
		return
	}
	v.previewSource = sourcePreviewState{key: key, cardID: cardID, open: true}
}

// drawSourcePreview draws only the preview explicitly opened by a source
// click. Pointer motion and keyboard focus never own or dismiss this state.
func (v *documentView) drawSourcePreview(c *ui.Context, targets []sourcePreviewTarget) {
	state := &v.previewSource
	if !state.open {
		return
	}
	target := sourcePreviewTargetForKey(targets, state.key)
	if target == nil {
		*state = sourcePreviewState{}
		return
	}
	anchor := c.Resolve(*target.anchor)
	if !anchor.Valid() {
		*state = sourcePreviewState{}
		return
	}

	pane := v.paneBounds(c)
	width := min(portrait.Form.PopoverWide, max(0, pane.W-2*portrait.Shell.PageInset))
	above, maxHeight := notePopoverRoom(pane, anchor)
	placement := ui.AnchorBottomStart
	if above {
		placement = ui.AnchorTopStart
	}
	panel := portrait.PopoverWithSpec(c, anchor, &state.open, portrait.PopoverSpec{
		Width: width, MaxHeight: maxHeight, Title: target.label, Opaque: true, Placement: placement,
	}, func() {
		ui.Column(c).FillWidth().Gap(portrait.Form.Gap).Padding(0, portrait.Form.PadX, portrait.Form.PadX, portrait.Form.PadX).Children(func() {
			switch {
			case !target.found:
				portrait.Alert(c, portrait.Tones.Danger, "Source missing", "This section is no longer in the document.")
			case target.ref.Hash != "" && !target.section.UnchangedSince(target.ref.Hash):
				portrait.Alert(c, portrait.Tones.Warning, "Source changed", "This section changed after the view was generated. Open in Markdown to read its current text.")
			default:
				if target.ref.Hash == "" {
					portrait.Type.Meta.Apply(ui.Text(c, "Source version at generation time cannot be verified.")).TextColor(portrait.Colors.Ink3)
				}
				portrait.Type.Small.Apply(ui.Text(c, v.excerpt(target.key, target.section))).TextColor(portrait.Colors.Ink2).Selectable()
			}
			ui.Row(c).FillWidth().Justify(ui.End).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
				openMarkdown := portrait.ButtonWith(c, "Open in Markdown", "Open in Markdown", portrait.ButtonSpec{
					Look: portrait.Raised, Size: portrait.Sizes.Compact, Disabled: !target.found,
				})
				if openMarkdown.Clicked() && target.found {
					highlightCard(v.reader(), target.node)
					v.openSourceInMarkdown(target.section)
					*state = sourcePreviewState{}
				}
				close := portrait.ButtonWith(c, "×", "Close", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact})
				if close.Clicked() {
					*state = sourcePreviewState{}
					anchor.Focus()
				}
			})
		})
	})
	if above {
		placePopoverAbove(panel, anchor, pane, width, maxHeight)
	} else {
		placePopoverBelow(panel, anchor, pane, width, maxHeight)
	}
	panel.Label("Source preview: " + target.label)
	if panel.Dismissed() {
		restoreFocus := !panel.PressedOutside()
		*state = sourcePreviewState{}
		if restoreFocus {
			anchor.Focus()
		}
	}
}

// openSourceInMarkdown is deliberately separate from opening the preview:
// only the explicit button in the pinned popover changes the document view.
func (v *documentView) openSourceInMarkdown(section workspace.Section) {
	// The panel supplies the window-backed action through this field in draw.
	if v.openSource != nil {
		v.openSource(section)
	}
}

func sourcePreviewTargetForKey(targets []sourcePreviewTarget, key string) *sourcePreviewTarget {
	for i := range targets {
		if targets[i].key == key {
			return &targets[i]
		}
	}
	return nil
}

func (v *documentView) excerpt(source string, section workspace.Section) string {
	key := sourceExcerptKey{source: source, start: section.Start, end: section.End}
	if excerpt, ok := v.excerpts[key]; ok {
		return excerpt
	}
	excerpt := ""
	if v.excerptSource != nil {
		excerpt = v.excerptSource(v.sourceText, section)
	} else {
		excerpt = sourceExcerpt(v.sourceText, section)
	}
	if v.excerpts == nil {
		v.excerpts = map[sourceExcerptKey]string{}
	}
	v.excerpts[key] = excerpt
	return excerpt
}

func sourceExcerpt(raw string, section workspace.Section) string {
	excerpt := strings.TrimSpace(raw[byteAt(raw, section.Start):byteAt(raw, section.End)])
	runes := 0
	for i := range excerpt {
		if runes == sourceExcerptLimit {
			return strings.TrimSpace(excerpt[:i]) + "…"
		}
		runes++
	}
	return excerpt
}
