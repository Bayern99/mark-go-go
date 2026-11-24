package nativereview

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/nativeeditor"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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
	dismissed      map[string]*dismissedProposal
	checkpointDocs map[string]*checkpointState
}

// dismissedProposal is what "Dismiss proposal" removed, so the toast it shows
// can put the same proposal back with the hunks and decisions it had.
type dismissedProposal struct {
	proposal  review.ChangeProposal
	decisions map[string]review.HunkDecision
	at        time.Time
}

// undoDismissTimeout keeps a dismissed proposal reachable for long enough to
// notice the mistake without holding the toast on screen.
const undoDismissTimeout = 8 * time.Second

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
	width, height := c.Size()
	ui.Row(c).Padding(3, 12).Children(func() {
		button := portrait.ButtonWith(c, fmt.Sprintf("Change proposals · %d", len(p.latest.Proposals)), "Change proposals", portrait.ButtonSpec{Look: portrait.Quiet, Leading: portrait.IconSparkle, Selected: p.open})
		if button.Clicked() {
			p.open = !p.open
		}
		popoverWidth := min(portrait.Form.PopoverWide, max(portrait.Form.MenuMin, width-2*portrait.Form.DialogPad))
		maxHeight := max(portrait.Form.MenuMin, height-2*portrait.Form.DialogPad)
		popover := portrait.PopoverWithSpec(c, button, &p.open, portrait.PopoverSpec{
			Width: popoverWidth, MaxHeight: maxHeight, Placement: ui.AnchorBottomStart, Opaque: true,
		}, func() { p.proposalPopover(c) })
		popover.AttachTo(button, ui.AnchorBottomStart, ui.AnchorTopStart)
	})
}

func (p *panel) proposalPopover(c *ui.Context) {
	tabs := proposalTabs(p.latest.Proposals)
	if p.selected == "" {
		p.selected = tabs[0].key
	}
	ui.Column(c).Gap(portrait.Form.Gap).Padding(0, portrait.Form.PadX).Children(func() {
		var active review.ChangeProposal
		p.proposalPicker(c, tabs)
		for _, item := range p.latest.Proposals {
			if item.ID == p.selected {
				active = item
				break
			}
		}
		if active.ID == "" {
			active = p.latest.Proposals[0]
			p.selected = tabs[0].key
			p.clearComparison()
		}
		if active.Summary != "" {
			portrait.Type.UI.Apply(ui.Text(c, active.Summary)).TextColor(portrait.Colors.Ink).Wrap()
		}
		if active.Source != "" {
			portrait.Type.Meta.Apply(ui.Text(c, active.Source)).TextColor(portrait.Colors.Ink3)
		}
		if !proposalTargets(p.win.Active(), active) {
			if portrait.Button(c, "Open "+proposalLabel(active), "Open "+proposalLabel(active)).AlignSelf(ui.Start).Clicked() {
				p.openTarget(active)
			}
		} else {
			p.drawComparison(c, active)
		}
		if p.proposalError != "" {
			portrait.Alert(c, portrait.Tones.Danger, "Change proposal", p.proposalError)
		}
	})
}

func proposalLabel(p review.ChangeProposal) string {
	if p.Path == "" {
		return "Untitled document"
	}
	return filepath.Base(p.Path)
}

// proposalTab is one document in the proposal popover. Its key is the
// proposal ID, so two proposals whose files share a basename still have two
// distinct tabs.
type proposalTab struct {
	key, title, tip string
}

// maxProposalTabs is how many tabs still fit a readable label; past it the
// documents move into a selector instead of squeezing every label.
const maxProposalTabs = 4

func proposalTabs(proposals []review.ChangeProposal) []proposalTab {
	bases := make([]string, len(proposals))
	shared := map[string]int{}
	for i, item := range proposals {
		bases[i] = proposalLabel(item)
		shared[bases[i]]++
	}
	tabs := make([]proposalTab, len(proposals))
	for i, item := range proposals {
		title := bases[i]
		if shared[title] > 1 {
			title = proposalDisambiguation(item, i)
		}
		tip := item.Path
		if tip == "" {
			tip = item.RelPath
		}
		tabs[i] = proposalTab{key: item.ID, title: title, tip: tip}
	}
	// A tab's key and label stay distinct even when records disagree about
	// an ID or two documents share a folder and a name.
	keys := map[string]int{}
	for i := range tabs {
		if tabs[i].key == "" {
			tabs[i].key = fmt.Sprintf("proposal:%d", i)
		}
		keys[tabs[i].key]++
		if keys[tabs[i].key] > 1 {
			tabs[i].key = fmt.Sprintf("%s:%d", tabs[i].key, keys[tabs[i].key])
		}
	}
	titles := map[string]int{}
	for _, tab := range tabs {
		titles[tab.title]++
	}
	seen := map[string]int{}
	for i := range tabs {
		if titles[tabs[i].title] > 1 {
			seen[tabs[i].title]++
			tabs[i].title = fmt.Sprintf("%s (%d)", tabs[i].title, seen[tabs[i].title])
		}
	}
	return tabs
}

// proposalDisambiguation names a document that shares its basename with
// another proposal by the folder that holds it.
func proposalDisambiguation(p review.ChangeProposal, index int) string {
	rel := ""
	if p.Path != "" {
		rel = p.Path
		if p.WorkspaceRoot != "" {
			if trimmed, err := filepath.Rel(p.WorkspaceRoot, p.Path); err == nil && filepath.IsLocal(trimmed) {
				rel = trimmed
			}
		}
	}
	if trimmed := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p.RelPath)), "./"); trimmed != "" {
		rel = trimmed
	}
	if rel == "" {
		return fmt.Sprintf("Untitled document %d", index+1)
	}
	rel = filepath.ToSlash(rel)
	if folder := filepath.ToSlash(filepath.Dir(rel)); folder != "" && folder != "." && folder != "/" {
		return folder + "/" + filepath.Base(rel)
	}
	return filepath.Base(rel)
}

// proposalPicker draws one tab per document until there are more documents
// than fit readable tabs, and a selector from then on.
func (p *panel) proposalPicker(c *ui.Context, tabs []proposalTab) {
	if len(tabs) > maxProposalTabs {
		selected := p.selected
		options := make([]portrait.Option, 0, len(tabs))
		for _, tab := range tabs {
			options = append(options, portrait.Option{Value: tab.key, Label: tab.title})
		}
		trigger := portrait.Select(c, &selected, options, portrait.SelectSpec{Label: "Change proposal documents", Width: portrait.Form.SelectMax})
		for _, tab := range tabs {
			if tab.key == selected {
				portrait.Tooltip(c, trigger, tab.tip)
			}
		}
		if selected != p.selected {
			p.selected = selected
			p.clearComparison()
		}
		return
	}
	portrait.Track(c, "change proposals", func(track *portrait.Segments) {
		for _, tab := range tabs {
			segment := track.SegmentKey(c, tab.key, tab.title, tab.title, p.selected == tab.key)
			segment.Grow(1).MinWidth(0)
			portrait.Tooltip(c, segment, tab.tip)
			if segment.Clicked() {
				p.selected = tab.key
				p.clearComparison()
			}
		}
	}).Label("Change proposal documents")
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
			portrait.Alert(c, portrait.Tones.Warning, "Stale comparison", review.ErrStaleComparison.Error())
		}
		if portrait.Button(c, "Compare current text", "Compare current text").AlignSelf(ui.Start).Clicked() {
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
		drawDiffSides(c, p.compared.Hunks, p.decisions, diffSides{before: p.base.text, after: p.compared.ProposedText})
		if p.compared.HasConflict {
			portrait.Alert(c, portrait.Tones.Warning, "Conflicting hunks", "Some hunks conflict with concurrent edits.")
		}
	}
	ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
		if portrait.ButtonWith(c, "Dismiss proposal", "Dismiss proposal", portrait.ButtonSpec{Look: portrait.DangerOutline}).Clicked() {
			p.dismissProposal(c, active)
		}
		ui.Spacer(c)
		if available {
			if portrait.Button(c, "Accept all clean hunks", "Accept all clean hunks").Clicked() {
				for _, h := range p.compared.Hunks {
					if !h.Conflict && p.decisions[h.ID] != review.HunkRejected {
						p.decisions[h.ID] = review.HunkAccepted
					}
				}
			}
			if portrait.AccentButton(c, "Apply decisions", "Apply decisions").Disabled(len(p.decisions) == 0 || p.win.Active().ReadOnly).Clicked() {
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
		}
	})
}

// dismissProposal removes a proposal but keeps enough of it to put it back:
// the popover shows an Undo toast instead of deleting the work silently.
func (p *panel) dismissProposal(c *ui.Context, active review.ChangeProposal) {
	if _, err := p.proposals.DismissProposal(p.root, active.ID); err != nil {
		p.proposalError = err.Error()
		return
	}
	decisions := make(map[string]review.HunkDecision, len(p.decisions))
	for id, decision := range p.decisions {
		decisions[id] = decision
	}
	if p.dismissed == nil {
		p.dismissed = map[string]*dismissedProposal{}
	}
	for id, kept := range p.dismissed {
		// A toast that timed out can no longer undo its proposal.
		if time.Since(kept.at) > undoDismissTimeout {
			delete(p.dismissed, id)
		}
	}
	p.dismissed[active.ID] = &dismissedProposal{proposal: active, decisions: decisions, at: time.Now()}
	p.selected = ""
	p.clearComparison()
	portrait.Notify(c, portrait.ToastSpec{
		ID:       "review:dismiss:" + active.ID,
		Tone:     portrait.Tones.Neutral,
		Title:    "Proposal dismissed",
		Action:   "Undo",
		Timeout:  undoDismissTimeout,
		OnAction: func() { p.restoreDismissed(active.ID) },
	})
}

// restoreDismissed resubmits the dismissed proposal under its old ID, so the
// same hunks return, then replays the decisions it had.
func (p *panel) restoreDismissed(id string) {
	dismissed := p.dismissed[id]
	if dismissed == nil {
		return
	}
	delete(p.dismissed, id)
	proposal := dismissed.proposal
	restored, err := p.proposals.SubmitProposal(review.ProposeInput{
		ID:            proposal.ID,
		WorkspaceRoot: proposal.WorkspaceRoot,
		DocumentID:    proposal.DocumentID,
		Path:          proposal.Path,
		Source:        proposal.Source,
		Summary:       proposal.Summary,
		BaseVersion:   proposal.BaseVersion,
		BaseText:      proposal.BaseText,
		ProposedText:  proposal.ProposedText,
	})
	if err != nil {
		p.proposalError = err.Error()
		return
	}
	for id, decision := range dismissed.decisions {
		if decision == "" || decision == review.HunkPending {
			continue
		}
		if next, err := p.proposals.DecideHunk(p.root, restored.ID, id, decision, proposal.LiveText); err == nil {
			restored = next
		}
	}
	p.selected = restored.ID
	p.latest = p.proposals.Snapshot(p.root)
	p.clearComparison()
}
