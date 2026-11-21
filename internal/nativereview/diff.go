package nativereview

import (
	"strconv"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func drawDiff(c *ui.Context, hunks []review.DiffHunk, decisions map[string]review.HunkDecision) {
	_, height := c.Size()
	if len(hunks) == 0 {
		portrait.Type.UI.Apply(ui.Text(c, "No changes.")).TextColor(portrait.Colors.Ink)
		return
	}
	ui.Scroll(c).Height(min(250, height-160)).MinHeight(76).Label("Document diff").Children(func() {
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
	ui.Column(c).Border(1, border).Radius(8).Clip().Background(portrait.Colors.Paper).Children(func() {
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
		sigil, sigilInk, tint = "+", portrait.Colors.Accent, portrait.Diff.Insert
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
		text(line.Text).Wrap().Grow(1).Shrink(1).Selectable().TextColor(portrait.Colors.Ink).Description(desc)
	})
}

func hunkButtons(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision) {
	ui.Row(c).Gap(6).Wrap().Children(func() {
		if decide(c, "Accept", "Accept hunk", decisions[h.ID] == review.HunkAccepted, h.Conflict) {
			decisions[h.ID] = review.HunkAccepted
		}
		if decide(c, "Reject", "Reject hunk", decisions[h.ID] == review.HunkRejected, false) {
			decisions[h.ID] = review.HunkRejected
		}
		if decisions[h.ID] != "" && decide(c, "Clear", "Clear choice", false, false) {
			delete(decisions, h.ID)
		}
	})
}

func decide(c *ui.Context, title, label string, selected, disabled bool) bool {
	b := portrait.QuietButton(c, title, label).Selected(selected).Disabled(disabled)
	switch {
	case disabled:
		b.Opacity(0.5)
	case selected && !b.Pressed():
		portrait.Materials.Row.Apply(b)
	}
	return b.Clicked()
}
