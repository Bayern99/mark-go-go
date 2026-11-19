package review

import "context"

type ReviewHost struct {
	s *Service
}

func (s *Service) Host() *ReviewHost {
	return &ReviewHost{s: s}
}

func (h *ReviewHost) Submit(_ context.Context, input ProposeInput) (ChangeProposal, error) {
	return h.s.SubmitProposal(input)
}

func (h *ReviewHost) List(_ context.Context, workspaceRoot string) ([]ChangeProposal, error) {
	return h.s.ListProposals(workspaceRoot), nil
}

func (h *ReviewHost) Get(_ context.Context, workspaceRoot, proposalID string) (ChangeProposal, error) {
	return h.s.GetProposal(workspaceRoot, proposalID)
}

func (h *ReviewHost) Sync(_ context.Context, workspaceRoot, proposalID, liveText string) (ChangeProposal, error) {
	return h.s.SyncProposal(workspaceRoot, proposalID, liveText)
}

func (h *ReviewHost) DecideHunk(_ context.Context, workspaceRoot, proposalID, hunkID string, decision HunkDecision, liveText string) (ChangeProposal, error) {
	return h.s.DecideHunk(workspaceRoot, proposalID, hunkID, decision, liveText)
}

func (h *ReviewHost) AcceptAll(_ context.Context, workspaceRoot, proposalID, liveText string) (ChangeProposal, error) {
	return h.s.AcceptAllHunks(workspaceRoot, proposalID, liveText)
}

func (h *ReviewHost) Complete(_ context.Context, workspaceRoot, proposalID string) ([]ChangeProposal, error) {
	return h.s.CompleteProposal(workspaceRoot, proposalID)
}

func (h *ReviewHost) Dismiss(_ context.Context, workspaceRoot, proposalID string) ([]ChangeProposal, error) {
	return h.s.DismissProposal(workspaceRoot, proposalID)
}

func (h *ReviewHost) Diff(_ context.Context, before, after string) ([]DiffHunk, error) {
	return ComputeDiff(before, after), nil
}
