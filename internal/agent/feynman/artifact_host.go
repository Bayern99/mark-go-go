package feynman

import (
	"context"
	"errors"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

func (h *ResearchManager) Artifacts(ctx context.Context, root string) ([]review.ResearchArtifact, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	session := h.sessions[fsutil.CanonicalPath(w.Root)]
	busy := session != nil && session.busy
	h.mu.Unlock()
	if busy {
		return nil, errors.New("Feynman is still writing research files.")
	}
	return review.CollectArtifacts(review.WorkDir(h.dataDir, w.Root), w.Root)
}

func (h *ResearchManager) ReadArtifact(ctx context.Context, root, relative string) (string, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return "", err
	}
	return review.ReadArtifactProposal(review.WorkDir(h.dataDir, w.Root), relative)
}

func (h *ResearchManager) RecoverArtifact(ctx context.Context, root, relative string) (string, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return "", err
	}
	return review.RecoverArtifact(review.WorkDir(h.dataDir, w.Root), w.Root, relative)
}

func (h *ResearchManager) ensureEditableDocument(ctx context.Context, root, documentID string) error {
	work := review.WorkDir(h.dataDir, root)
	if documentID == "" {
		return review.ClearStagedDocument(work)
	}
	if _, err := h.workspace(ctx, root); err != nil {
		return err
	}
	if h.workspaces == nil {
		return errors.New("The selected document is no longer editable in this workspace.")
	}
	if err := h.workspaces.EditableDocument(ctx, root, documentID); err != nil {
		return err
	}
	return review.ClearStagedDocument(work)
}
