package nativereview

import (
	"context"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

// SceneState is the recorded state a Scene draws.
type SceneState struct {
	Proposals   review.ReviewUpdateEvent
	Compared    review.ChangeProposal
	Hunks       []review.DiffHunk
	Checkpoints []review.Checkpoint
}

// Scene draws the review surfaces from recorded state, so the acceptance
// images come from the same proposal, checkpoint and diff code a running
// window draws rather than a copy of it.
type Scene struct{ p *panel }

// NewScene builds a review panel around a window and recorded state.
func NewScene(win *nativeui.Window, state SceneState) *Scene {
	p := &panel{
		win:            win,
		ctx:            context.Background(),
		root:           win.Workspace().Root,
		latest:         state.Proposals,
		compared:       state.Compared,
		decisions:      map[string]review.HunkDecision{},
		checkpointDocs: map[string]*checkpointState{},
	}
	if len(state.Proposals.Proposals) > 0 {
		p.selected = state.Proposals.Proposals[0].ID
	}
	if source := win.Source(); source != nil && state.Compared.ID != "" {
		doc := win.Active()
		p.base = reading{document: source, id: doc.ID, path: doc.Path, text: source.Source(), revision: source.Version()}
	}
	if doc := win.Active(); doc.ID != "" {
		checkpoints := &checkpointState{id: doc.ID, path: doc.Path, loaded: true, items: state.Checkpoints}
		if len(checkpoints.items) > 0 {
			checkpoints.selected = checkpoints.items[0].ID
		}
		if len(state.Checkpoints) > 0 && len(state.Hunks) > 0 {
			// The comparison is current, as a just-compared checkpoint's is.
			checkpoints.comparing, checkpoints.diff, checkpoints.base = true, state.Hunks, p.base
			checkpoints.original = p.base.text
		}
		p.checkpointDocs[doc.ID] = checkpoints
	}
	return &Scene{p: p}
}

// OpenProposals opens the change-proposal popover.
func (s *Scene) OpenProposals() { s.p.open = true }

// Proposals draws the change-proposal button and its popover.
func (s *Scene) Proposals(c *ui.Context) { s.p.drawProposals(c) }

// Checkpoints draws the checkpoint panel of the active document.
func (s *Scene) Checkpoints(c *ui.Context) { s.p.drawCheckpoints(c) }

// ShowCheckpointList resets the checkpoint panel to its list state so the
// checkpoint-list scene is distinct from the comparison diff scene.
func (s *Scene) ShowCheckpointList() {
	for _, state := range s.p.checkpointDocs {
		state.comparing = false
	}
}

// Hunks draws a diff with no decisions, as a comparison preview does.
func Hunks(c *ui.Context, hunks []review.DiffHunk) { drawDiff(c, hunks, nil) }
