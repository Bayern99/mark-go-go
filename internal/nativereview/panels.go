package nativereview

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type reading struct {
	document *nativeeditor.Document
	path, id string
	text     string
	revision uint64
}

func read(win *nativeui.Window) reading {
	doc := win.Source()
	if doc == nil {
		return reading{}
	}
	info := win.Active()
	return reading{document: doc, path: info.Path, id: info.ID, text: doc.Source(), revision: doc.Version()}
}

func (r reading) current(win *nativeui.Window) bool {
	return r.document != nil && win.Source() == r.document && win.Active().ID == r.id &&
		win.Active().Path == r.path && r.document.Version() == r.revision
}

func (r reading) apply(win *nativeui.Window, replacement string) error {
	if !r.current(win) || win.Active().ReadOnly {
		return review.ErrStaleComparison
	}
	return r.document.Apply(r.revision, 0, len(r.text), r.text, replacement)
}

type panel struct {
	win            *nativeui.Window
	workspace      *workspace.Service
	proposals      *review.Service
	checkpoints    *review.CheckpointHost
	ctx            context.Context
	root           string
	latest         review.ReviewUpdateEvent
	selected       string
	compared       review.ChangeProposal
	base           reading
	decisions      map[string]review.HunkDecision
	open           bool
	proposalError  string
	checkpointDocs map[string]*checkpointState
}

func Install(work *workspace.Service, proposals *review.Service, checkpoints *review.CheckpointHost) func(*nativeui.Window) nativeui.Panels {
	return func(win *nativeui.Window) nativeui.Panels {
		ctx, cancel := context.WithCancel(win.Context())
		p := &panel{win: win, workspace: work, proposals: proposals, checkpoints: checkpoints, ctx: ctx, checkpointDocs: map[string]*checkpointState{}}
		review.ReviewUpdated.Listen(win.Native(), func(event review.ReviewUpdateEvent) {
			if event.WorkspaceRoot == p.root && event.Revision >= p.latest.Revision {
				p.latest = event
			}
		})
		return nativeui.Panels{Review: p.drawProposals, Checkpoints: p.drawCheckpoints, Changed: p.changed, Closed: cancel}
	}
}

func (p *panel) changed() {
	next := p.win.Workspace().Root
	if next != p.root {
		p.root = next
		p.latest = p.proposals.Snapshot(next)
		p.selected = ""
		p.open = false
		p.clearComparison()
	}
	open := make(map[string]bool)
	for _, document := range p.win.Workspace().Documents {
		open[document.ID] = true
	}
	for id := range p.checkpointDocs {
		if !open[id] {
			delete(p.checkpointDocs, id)
		}
	}
}

func (p *panel) clearComparison() {
	p.compared = review.ChangeProposal{}
	p.base = reading{}
	p.decisions = map[string]review.HunkDecision{}
	p.proposalError = ""
}

func (p *panel) drawProposals(c *ui.Context) {
	if len(p.latest.Proposals) == 0 {
		p.open = false
		return
	}
	ui.Row(c).Padding(3, 12).Children(func() {
		button := ui.Button(c, fmt.Sprintf("Change proposals (%d)", len(p.latest.Proposals))).Label("Change proposals")
		if button.Clicked() {
			p.open = !p.open
		}
		ui.Popover(c, button, &p.open, func() { p.proposalPopover(c) })
	})
}

func (p *panel) proposalPopover(c *ui.Context) {
	width, _ := c.Size()
	ui.Column(c).Width(min(520, max(280, width-40))).Gap(8).Padding(8).Children(func() {
		ui.Text(c, "Change proposals").Bold()
		var active review.ChangeProposal
		ui.ScrollHorizontal(c).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				for _, item := range p.latest.Proposals {
					if p.selected == "" {
						p.selected = item.ID
					}
					if ui.Button(c.Key(item.ID), proposalLabel(item)).Selected(p.selected == item.ID).Clicked() {
						p.selected = item.ID
						p.clearComparison()
					}
				}
			})
		})
		for _, item := range p.latest.Proposals {
			if item.ID == p.selected {
				active = item
				break
			}
		}
		if active.ID == "" {
			active = p.latest.Proposals[0]
			p.selected = active.ID
			p.clearComparison()
		}
		if active.Summary != "" {
			ui.Text(c, active.Summary).Wrap()
		}
		if active.Source != "" {
			ui.Text(c, active.Source).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if !proposalTargets(p.win.Active(), active) {
			if ui.Button(c, "Open "+proposalLabel(active)).Clicked() {
				p.openTarget(active)
			}
		} else {
			p.drawComparison(c, active)
		}
		if p.proposalError != "" {
			ui.Text(c, p.proposalError).Role(ui.RoleAlert).Wrap()
		}
	})
}

func proposalLabel(p review.ChangeProposal) string {
	if p.Path == "" {
		return "Untitled document"
	}
	return filepath.Base(p.Path)
}

func proposalTargets(d workspace.DocumentInfo, p review.ChangeProposal) bool {
	if d.ID == "" || d.WorkspaceRoot != p.WorkspaceRoot {
		return false
	}
	if p.DocumentID != "" && d.ID == p.DocumentID {
		return true
	}
	return d.Path != "" && d.Path == p.Path
}

func (p *panel) openTarget(proposal review.ChangeProposal) {
	go func() {
		id := proposal.DocumentID
		if id == "" {
			id = proposal.Path
		}
		_, err := p.workspace.MCPOpenDocument(p.ctx, proposal.WorkspaceRoot, id, 0, false)
		if err != nil && id != proposal.Path && proposal.Path != "" {
			_, err = p.workspace.MCPOpenDocument(p.ctx, proposal.WorkspaceRoot, proposal.Path, 0, false)
		}
		if err != nil {
			p.win.Native().Update(func() {
				if p.ctx.Err() == nil && p.selected == proposal.ID && p.root == proposal.WorkspaceRoot {
					p.proposalError = err.Error()
				}
			})
		}
	}()
}

func (p *panel) drawComparison(c *ui.Context, active review.ChangeProposal) {
	available := p.compared.ID == active.ID && p.compared.Revision == active.Revision && p.base.current(p.win)
	if !available {
		if p.compared.ID != "" {
			ui.Text(c, review.ErrStaleComparison.Error()).Role(ui.RoleStatus).Wrap()
		}
		if ui.Button(c, "Compare current text").Clicked() {
			base := read(p.win)
			compared, err := p.proposals.SyncProposal(p.root, active.ID, base.text)
			p.clearComparison()
			if err == nil {
				p.base, p.compared = base, compared
			} else {
				p.proposalError = err.Error()
			}
		}
	} else {
		drawDiff(c, p.compared.Hunks, p.decisions)
		if p.compared.HasConflict {
			ui.Text(c, "Some hunks conflict with concurrent edits.").Role(ui.RoleStatus).Wrap()
		}
		ui.Row(c).Gap(6).Wrap().Children(func() {
			if ui.Button(c, "Accept all clean hunks").Clicked() {
				for _, h := range p.compared.Hunks {
					if !h.Conflict && p.decisions[h.ID] != review.HunkRejected {
						p.decisions[h.ID] = review.HunkAccepted
					}
				}
			}
			if ui.Button(c, "Apply decisions").Disabled(len(p.decisions) == 0 || p.win.Active().ReadOnly).Clicked() {
				err := p.proposals.ApplyReviewed(p.root, p.compared.ID, p.compared.Revision, p.base.text, p.decisions, func(next string) error {
					return p.base.apply(p.win, next)
				})
				if err == nil {
					p.clearComparison()
					err = p.win.Publish()
				}
				if err != nil {
					p.proposalError = err.Error()
				}
			}
		})
	}
	if ui.Button(c, "Dismiss proposal").Clicked() {
		if _, err := p.proposals.DismissProposal(p.root, active.ID); err != nil {
			p.proposalError = err.Error()
		} else {
			p.clearComparison()
		}
	}
}
