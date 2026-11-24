package nativereview

import (
	"strconv"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func drawDiff(c *ui.Context, hunks []review.DiffHunk, decisions map[string]review.HunkDecision) {
	if len(hunks) == 0 {
		portrait.Type.UI.Apply(ui.Text(c, "No changes.")).TextColor(portrait.Colors.Ink)
		return
	}
	ui.Scroll(c).MaxHeight(portrait.Diff.PanelMax).MinHeight(portrait.Diff.PanelMin).Label("Document diff").Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, h := range hunks {
				drawHunk(c.Key(h.ID), h, decisions)
			}
		})
	})
}

func drawHunk(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision) {
	border := portrait.Colors.Line
	if h.Conflict {
		border = portrait.Colors.Danger
	}
	ui.Column(c).Border(1, border).Radius(portrait.Diff.Radius).Clip().Background(portrait.Colors.Paper).Children(func() {
		ui.Row(c).Wrap().Gap(6).Padding(5, 8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			BorderWidth(0, 0, 1, 0).BorderColor(portrait.Colors.Line).Children(func() {
			portrait.Diff.Range.Apply(ui.Text(c, h.Header)).TextColor(portrait.Colors.Ink3).FontFeatures("tnum")
			if h.Conflict {
				conflict := "Conflict"
				if h.ConflictReason != "" {
					conflict += ": " + h.ConflictReason
				}
				portrait.Diff.Conflict.Apply(ui.Text(c, conflict)).Wrap().Role(ui.RoleStatus).FontWeight(500).TextColor(portrait.Colors.Danger)
			}
			if decisions != nil {
				hunkButtons(c, h, decisions)
			}
		})
		ui.Column(c).Role(ui.RoleGroup).Label("Hunk " + h.Header).Children(func() {
			for i, line := range h.Lines {
				diffLine(c.Key(i), line)
			}
		})
	})
}

func diffLine(c *ui.Context, line review.DiffLine) {
	sigil, sigilInk, tint := " ", portrait.Colors.Ink3, ui.Transparent
	switch line.Kind {
	case review.DiffLineInsert:
		sigil, sigilInk, tint = "+", portrait.Colors.Success, portrait.Diff.Insert
	case review.DiffLineDelete:
		sigil, sigilInk, tint = "−", portrait.Colors.Danger, portrait.Diff.Delete
	}
	text := func(s string) ui.Element {
		return portrait.Diff.Lines.Apply(ui.Text(c, s))
	}
	number := func(n int) {
		label := ""
		if n > 0 {
			label = strconv.Itoa(n)
		}
		text(label).Width(portrait.Diff.NumberWidth).TextAlign(ui.End).FontFeatures("tnum").TextColor(portrait.Colors.Ink3).AccessHidden(true)
	}
	desc := "Unchanged, lines " + strconv.Itoa(line.OldLine) + "→" + strconv.Itoa(line.NewLine)
	switch line.Kind {
	case review.DiffLineInsert:
		desc = "Added, line " + strconv.Itoa(line.NewLine)
	case review.DiffLineDelete:
		desc = "Removed, line " + strconv.Itoa(line.OldLine)
	}
	ui.Row(c).Gap(4).Padding(1, 6).Background(tint).Children(func() {
		number(line.OldLine)
		number(line.NewLine)
		text(sigil).Width(portrait.Diff.SigilWidth).TextAlign(ui.Center).TextColor(sigilInk).AccessHidden(true)
		content := text(line.Text).Wrap().Grow(1).Shrink(1).TextColor(portrait.Colors.Ink)
		if line.Text == "" {
			content.Role(ui.RoleText).Label(desc)
		} else {
			content.Selectable().Description(desc)
		}
	})
}

func hunkButtons(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision) {
	portrait.Group(c, "Hunk actions", func(g *portrait.GroupBuilder) {
		if decide(c, g, "Accept", "Accept hunk", decisions[h.ID] == review.HunkAccepted, h.Conflict) {
			decisions[h.ID] = review.HunkAccepted
		}
		if decide(c, g, "Reject", "Reject hunk", decisions[h.ID] == review.HunkRejected, false) {
			decisions[h.ID] = review.HunkRejected
		}
		if decisions[h.ID] != "" && decide(c, g, "Clear", "Clear choice", false, false) {
			delete(decisions, h.ID)
		}
	})
}

func decide(c *ui.Context, g *portrait.GroupBuilder, title, label string, selected, disabled bool) bool {
	b := portrait.ButtonWith(c, title, label, portrait.ButtonSpec{Look: portrait.Quiet, Selected: selected}).Disabled(disabled)
	if disabled {
		b.Opacity(0.5)
	}
	g.Add(b)
	return b.Clicked()
}
