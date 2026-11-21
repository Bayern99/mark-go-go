package nativefeynman

import (
	"path/filepath"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func TestFeynmanProposalUsesTheOpenWorkspaceDocument(t *testing.T) {
	root := t.TempDir()
	root = fsutil.CanonicalPath(root)
	path := filepath.Join(root, "notes.md")
	service := review.NewService()
	doc := workspace.DocumentInfo{WorkspaceRoot: root, ID: "document-7", Path: path}
	snapshot := workspace.Snapshot{
		ID:      doc.ID,
		Text:    "base text\n",
		Version: 12,
		State:   doc,
	}

	if err := submitProposal(service, root, doc, snapshot, "revised text\n"); err != nil {
		t.Fatalf("submit proposal: %v", err)
	}
	proposal := service.Snapshot(root).Proposals
	if len(proposal) != 1 {
		t.Fatalf("got %d proposals, want one", len(proposal))
	}
	got := proposal[0]
	if got.WorkspaceRoot != root || got.DocumentID != doc.ID || got.Path != path {
		t.Fatalf("proposal target = (%q, %q, %q), want (%q, %q, %q)", got.WorkspaceRoot, got.DocumentID, got.Path, root, doc.ID, path)
	}
	if got.BaseText != snapshot.Text || got.BaseVersion != snapshot.Version || got.ProposedText != "revised text\n" {
		t.Fatalf("proposal body = (%q, %d, %q)", got.BaseText, got.BaseVersion, got.ProposedText)
	}
}
