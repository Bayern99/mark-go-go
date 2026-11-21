package nativefeynman

import (
	"errors"
	"path/filepath"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func submitProposal(service *review.Service, root string, document workspace.DocumentInfo, snapshot workspace.Snapshot, proposed string) error {
	if service == nil {
		return errors.New("Document review is unavailable.")
	}
	root = fsutil.CanonicalPath(root)
	if root == "" || fsutil.CanonicalPath(document.WorkspaceRoot) != root {
		return errors.New("The selected document belongs to a different workspace.")
	}
	if document.ID == "" || snapshot.ID != document.ID || snapshot.State.ID != document.ID {
		return errors.New("The selected document changed before the proposal was submitted.")
	}
	path := fsutil.CanonicalPath(document.Path)
	if !filepath.IsAbs(path) || path != fsutil.CanonicalPath(snapshot.State.Path) {
		return errors.New("The selected document moved before the proposal was submitted.")
	}
	_, err := service.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    document.ID,
		Path:          path,
		Source:        "feynman",
		BaseVersion:   snapshot.Version,
		BaseText:      snapshot.Text,
		ProposedText:  proposed,
	})
	return err
}
