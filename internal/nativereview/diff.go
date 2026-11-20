package nativereview

import (
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func drawDiff(c *ui.Context, hunks []review.DiffHunk, decisions map[string]review.HunkDecision) {
	_, height := c.Size()
	if len(hunks) == 0 {
		ui.Text(c, "No changes.")
		return
	}
	ui.Scroll(c).Height(min(250, height-160)).MinHeight(76).Label("Document diff").Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			for _, h := range hunks {
				ctx := c.Key(h.ID)
				ui.Column(ctx).Gap(2).Children(func() {
					ui.Text(ctx, h.Header).Bold().Font("IBM Plex Mono").FontSize(12)
					if h.Conflict {
						ui.Text(ctx, h.ConflictReason).Wrap().Role(ui.RoleStatus)
					}
					for _, line := range h.Lines {
						label := line.Text
						ink := c.Theme().TextMuted
						switch line.Kind {
						case review.DiffLineInsert:
							label = "+ " + label
							ink = ui.RGB(34, 108, 68)
						case review.DiffLineDelete:
							label = "− " + label
							ink = ui.RGB(157, 44, 47)
						}
						ui.Text(ctx, label).Wrap().Selectable().Font("IBM Plex Mono").FontSize(12).TextColor(ink)
					}
					if decisions != nil {
						hunkButtons(ctx, h, decisions)
					}
				})
			}
		})
	})
}

func hunkButtons(c *ui.Context, h review.DiffHunk, decisions map[string]review.HunkDecision) {
	ui.Row(c).Gap(6).Children(func() {
		if ui.Button(c, "Accept hunk").Selected(decisions[h.ID] == review.HunkAccepted).Disabled(h.Conflict).Clicked() {
			decisions[h.ID] = review.HunkAccepted
		}
		if ui.Button(c, "Reject hunk").Selected(decisions[h.ID] == review.HunkRejected).Clicked() {
			decisions[h.ID] = review.HunkRejected
		}
		if decisions[h.ID] != "" && ui.Button(c, "Clear choice").Clicked() {
			delete(decisions, h.ID)
		}
	})
}
