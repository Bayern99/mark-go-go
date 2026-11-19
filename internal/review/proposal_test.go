package review

import (
	"testing"
)

func TestMyersDiffComputesMinimalHunksWithLineNumbersAndContext(t *testing.T) {
	if hunks := ComputeDiff("# Same\n\nBody.\n", "# Same\n\nBody.\n"); len(hunks) != 0 {
		t.Fatalf("expected 0 hunks for identical text, got %d", len(hunks))
	}

	before := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5 old\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10\nLine 11 old\nLine 12\n"
	after := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5 new\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10\nLine 11 new\nLine 11 extra\nLine 12\n"

	hunks := ComputeDiff(before, after)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 minimal hunks, got %d: %#v", len(hunks), hunks)
	}

	h1 := hunks[0]
	if h1.ID != "hunk-1" || h1.ChangeOldLine != 5 || h1.ChangeOldCount != 1 || h1.ChangeNewLine != 5 || h1.ChangeNewCount != 1 {
		t.Fatalf("unexpected hunk-1 coordinates: %+v", h1)
	}
	if h1.OldStart != 2 || h1.OldLines != 7 || h1.NewStart != 2 || h1.NewLines != 7 {
		t.Fatalf("expected 3 leading and 3 trailing context lines (lines 2..8), got OldStart=%d OldLines=%d NewStart=%d NewLines=%d",
			h1.OldStart, h1.OldLines, h1.NewStart, h1.NewLines)
	}
	if len(h1.Lines) != 8 {
		t.Fatalf("expected 8 diff lines in hunk-1 (3 pre + 1 del + 1 ins + 3 post), got %d: %+v", len(h1.Lines), h1.Lines)
	}
	if h1.Lines[0].Kind != DiffLineContext || h1.Lines[0].OldLine != 2 || h1.Lines[0].NewLine != 2 || h1.Lines[0].Text != "Line 2" {
		t.Fatalf("unexpected first context line in hunk-1: %+v", h1.Lines[0])
	}
	if h1.Lines[3].Kind != DiffLineDelete || h1.Lines[3].OldLine != 5 || h1.Lines[3].Text != "Line 5 old" {
		t.Fatalf("unexpected delete line in hunk-1: %+v", h1.Lines[3])
	}
	if h1.Lines[4].Kind != DiffLineInsert || h1.Lines[4].NewLine != 5 || h1.Lines[4].Text != "Line 5 new" {
		t.Fatalf("unexpected insert line in hunk-1: %+v", h1.Lines[4])
	}
	if h1.Lines[7].Kind != DiffLineContext || h1.Lines[7].OldLine != 8 || h1.Lines[7].NewLine != 8 || h1.Lines[7].Text != "Line 8" {
		t.Fatalf("unexpected last context line in hunk-1: %+v", h1.Lines[7])
	}

	h2 := hunks[1]
	if h2.ID != "hunk-2" || h2.ChangeOldLine != 11 || h2.ChangeOldCount != 1 || h2.ChangeNewLine != 11 || h2.ChangeNewCount != 2 {
		t.Fatalf("unexpected hunk-2 coordinates: %+v", h2)
	}
	if h2.OldStart != 8 || h2.OldLines != 5 || h2.NewStart != 8 || h2.NewLines != 6 {
		t.Fatalf("unexpected hunk-2 span: %+v", h2)
	}
}

func TestMyersDiffKeepsSeparateParagraphHunksSelectable(t *testing.T) {
	before := "# Title\n\nFirst paragraph old.\n\nSecond paragraph old.\n"
	after := "# Title\n\nFirst paragraph new.\n\nSecond paragraph new.\n"

	hunks := ComputeDiff(before, after)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 separate hunks for two distinct paragraphs, got %d: %+v", len(hunks), hunks)
	}
	if hunks[0].ChangeOldLine != 3 || hunks[1].ChangeOldLine != 5 {
		t.Fatalf("expected hunks at lines 3 and 5, got %d and %d", hunks[0].ChangeOldLine, hunks[1].ChangeOldLine)
	}
}

func TestMultiFileProposalQueueManagesProposalsIndependentlyOfAgent(t *testing.T) {
	svc := NewService()
	root := t.TempDir()

	p1, err := svc.SubmitProposal(ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-1",
		Path:          root + "/draft.md",
		Source:        "mcp",
		Summary:       "Refine introduction",
		BaseText:      "# Draft\n\nIntro old.\n\nConclusion old.\n",
		ProposedText:  "# Draft\n\nIntro new.\n\nConclusion new.\n",
	})
	if err != nil {
		t.Fatalf("SubmitProposal 1 failed: %v", err)
	}
	if p1.ID == "" || p1.RelPath != "draft.md" || len(p1.Hunks) != 2 {
		t.Fatalf("unexpected proposal 1: %+v", p1)
	}

	p2, err := svc.SubmitProposal(ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-2",
		Path:          root + "/papers/methods.md",
		Source:        "custom-agent",
		BaseText:      "# Methods\n\nStep 1.\n\nStep 2.\n",
		StartLine:     3,
		EndLine:       3,
		NewText:       "Step 1 revised.",
	})
	if err != nil {
		t.Fatalf("SubmitProposal 2 failed: %v", err)
	}
	if p2.RelPath != "papers/methods.md" || len(p2.Hunks) != 1 || p2.ProposedText != "# Methods\n\nStep 1 revised.\n\nStep 2.\n" {
		t.Fatalf("unexpected proposal 2: %+v", p2)
	}

	p3, err := svc.SubmitProposal(ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-3",
		Path:          root + "/notes.md",
		BaseText:      "# Notes\n\nTarget section.\n",
		OldText:       "Target section.",
		NewText:       "Updated section.",
	})
	if err != nil {
		t.Fatalf("SubmitProposal 3 failed: %v", err)
	}
	if p3.ProposedText != "# Notes\n\nUpdated section.\n" {
		t.Fatalf("unexpected proposal 3 text: %q", p3.ProposedText)
	}

	queue := svc.ListProposals(root)
	if len(queue) != 3 {
		t.Fatalf("expected 3 proposals in multi-file queue, got %d: %+v", len(queue), queue)
	}
	if queue[0].ID != p1.ID || queue[1].ID != p2.ID || queue[2].ID != p3.ID {
		t.Fatalf("unexpected queue order: %+v", queue)
	}

	remaining, err := svc.DismissProposal(root, p2.ID)
	if err != nil {
		t.Fatalf("DismissProposal failed: %v", err)
	}
	if len(remaining) != 2 || remaining[0].ID != p1.ID || remaining[1].ID != p3.ID {
		t.Fatalf("expected p1 and p3 remaining after dismissing p2, got: %+v", remaining)
	}
}

func TestSelectiveHunkAcceptRejectAndAcceptAll(t *testing.T) {
	svc := NewService()
	root := t.TempDir()

	base := "# Document\n\nParagraph 1 old.\n\nParagraph 2.\n\nParagraph 3 old.\n"
	proposed := "# Document\n\nParagraph 1 new.\n\nParagraph 2.\n\nParagraph 3 new.\n"

	prop, err := svc.SubmitProposal(ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-1",
		Path:          root + "/doc.md",
		BaseText:      base,
		ProposedText:  proposed,
	})
	if err != nil {
		t.Fatalf("SubmitProposal failed: %v", err)
	}
	if len(prop.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(prop.Hunks))
	}

	// Accept hunk-1, reject hunk-2 -> only Paragraph 1 changes in MergedText.
	prop, err = svc.DecideHunk(root, prop.ID, "hunk-1", HunkAccepted, "")
	if err != nil {
		t.Fatalf("DecideHunk hunk-1 failed: %v", err)
	}
	prop, err = svc.DecideHunk(root, prop.ID, "hunk-2", HunkRejected, "")
	if err != nil {
		t.Fatalf("DecideHunk hunk-2 failed: %v", err)
	}
	wantPartial := "# Document\n\nParagraph 1 new.\n\nParagraph 2.\n\nParagraph 3 old.\n"
	if prop.MergedText != wantPartial {
		t.Fatalf("selective MergedText = %q, want %q", prop.MergedText, wantPartial)
	}

	// Now flip: reject hunk-1, accept hunk-2 -> only Paragraph 3 changes in MergedText.
	prop, err = svc.DecideHunk(root, prop.ID, "hunk-1", HunkRejected, "")
	if err != nil {
		t.Fatalf("DecideHunk hunk-1 reject failed: %v", err)
	}
	prop, err = svc.DecideHunk(root, prop.ID, "hunk-2", HunkAccepted, "")
	if err != nil {
		t.Fatalf("DecideHunk hunk-2 accept failed: %v", err)
	}
	wantSecondOnly := "# Document\n\nParagraph 1 old.\n\nParagraph 2.\n\nParagraph 3 new.\n"
	if prop.MergedText != wantSecondOnly {
		t.Fatalf("selective second-only MergedText = %q, want %q", prop.MergedText, wantSecondOnly)
	}

	// AcceptAllHunks -> both Paragraph 1 and Paragraph 3 change.
	prop, err = svc.AcceptAllHunks(root, prop.ID, "")
	if err != nil {
		t.Fatalf("AcceptAllHunks failed: %v", err)
	}
	if prop.MergedText != proposed {
		t.Fatalf("AcceptAll MergedText = %q, want %q", prop.MergedText, proposed)
	}
	for _, h := range prop.Hunks {
		if h.Decision != HunkAccepted {
			t.Fatalf("expected hunk %s to be accepted, got %s", h.ID, h.Decision)
		}
	}
}

func TestConcurrentEditMergesNonOverlappingHunksAndFlagsOverlappingConflicts(t *testing.T) {
	svc := NewService()
	root := t.TempDir()

	base := "# Paper\n\nParagraph 1 original.\n\nParagraph 2 original.\n\nParagraph 3 original.\n"
	proposed := "# Paper\n\nParagraph 1 by agent.\n\nParagraph 2 original.\n\nParagraph 3 by agent.\n"

	prop, err := svc.SubmitProposal(ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-1",
		Path:          root + "/paper.md",
		BaseText:      base,
		ProposedText:  proposed,
	})
	if err != nil {
		t.Fatalf("SubmitProposal failed: %v", err)
	}

	// Case 1: User edits unrelated Paragraph 2 (and adds a subtitle line at top) while proposal is pending.
	liveUnrelated := "# Paper\nSubtitle by user.\n\nParagraph 1 original.\n\nParagraph 2 edited by user.\n\nParagraph 3 original.\n"
	synced, err := svc.SyncProposal(root, prop.ID, liveUnrelated)
	if err != nil {
		t.Fatalf("SyncProposal unrelated edit failed: %v", err)
	}
	if synced.HasConflict {
		t.Fatalf("expected HasConflict=false for non-overlapping concurrent edit, got true: %+v", synced.Hunks)
	}
	if synced.Hunks[0].Conflict || synced.Hunks[1].Conflict {
		t.Fatalf("expected neither hunk to conflict, got: %+v", synced.Hunks)
	}
	// Because 1 extra line was inserted at the top, Paragraph 1 moved from line 3 to line 4, and Paragraph 3 moved from line 7 to line 8.
	if synced.Hunks[0].ChangeOldLine != 4 || synced.Hunks[1].ChangeOldLine != 8 {
		t.Fatalf("expected shifted ChangeOldLine 4 and 8, got %d and %d", synced.Hunks[0].ChangeOldLine, synced.Hunks[1].ChangeOldLine)
	}
	wantCleanMerge := "# Paper\nSubtitle by user.\n\nParagraph 1 by agent.\n\nParagraph 2 edited by user.\n\nParagraph 3 by agent.\n"
	if synced.MergedText != wantCleanMerge {
		t.Fatalf("non-overlapping MergedText = %q, want %q", synced.MergedText, wantCleanMerge)
	}

	// Case 2: User also edits Paragraph 1 in the live buffer (colliding with hunk-1), while leaving Paragraph 3 untouched.
	liveColliding := "# Paper\nSubtitle by user.\n\nParagraph 1 edited by user.\n\nParagraph 2 edited by user.\n\nParagraph 3 original.\n"
	collided, err := svc.SyncProposal(root, prop.ID, liveColliding)
	if err != nil {
		t.Fatalf("SyncProposal colliding edit failed: %v", err)
	}
	if !collided.HasConflict {
		t.Fatalf("expected HasConflict=true when user edited Paragraph 1")
	}
	if !collided.Hunks[0].Conflict || collided.Hunks[0].ConflictReason == "" {
		t.Fatalf("expected hunk-1 to be marked conflict=true with reason, got: %+v", collided.Hunks[0])
	}
	if collided.Hunks[1].Conflict {
		t.Fatalf("expected hunk-2 (Paragraph 3) to remain conflict=false, got: %+v", collided.Hunks[1])
	}
	// Attempting to accept the conflicting hunk-1 must return an error.
	if _, err := svc.DecideHunk(root, prop.ID, "hunk-1", HunkAccepted, liveColliding); err == nil {
		t.Fatalf("expected error when attempting to accept conflicting hunk-1")
	}
	// Accepting non-conflicting hunk-2 (or AcceptAllHunks) merges hunk-2 while preserving user's Paragraph 1 and Paragraph 2 edits.
	accepted, err := svc.AcceptAllHunks(root, prop.ID, liveColliding)
	if err != nil {
		t.Fatalf("AcceptAllHunks on partially conflicting proposal failed: %v", err)
	}
	if accepted.Hunks[0].Decision == HunkAccepted {
		t.Fatalf("conflicting hunk-1 must not be marked accepted by AcceptAllHunks")
	}
	if accepted.Hunks[1].Decision != HunkAccepted {
		t.Fatalf("non-conflicting hunk-2 should be marked accepted by AcceptAllHunks")
	}
	wantPartialConflictMerge := "# Paper\nSubtitle by user.\n\nParagraph 1 edited by user.\n\nParagraph 2 edited by user.\n\nParagraph 3 by agent.\n"
	if accepted.MergedText != wantPartialConflictMerge {
		t.Fatalf("partial conflict MergedText = %q, want %q", accepted.MergedText, wantPartialConflictMerge)
	}
}

func TestProposalQueueConcurrentAccessIsRaceFree(t *testing.T) {
	svc := NewService()
	root := t.TempDir()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := range 25 {
			p, err := svc.SubmitProposal(ProposeInput{
				WorkspaceRoot: root,
				Path:          root + "/concurrent.md",
				BaseText:      "# Doc\n\nPara 1.\n\nPara 2.\n",
				ProposedText:  "# Doc\n\nPara 1 updated.\n\nPara 2 updated.\n",
				BaseVersion:   i,
			})
			if err != nil {
				t.Errorf("SubmitProposal failed: %v", err)
				return
			}
			_, _ = svc.SyncProposal(root, p.ID, "# Doc\n\nPara 1.\n\nPara 2.\n")
			_, _ = svc.DecideHunk(root, p.ID, "hunk-1", HunkAccepted, "")
			_, _ = svc.AcceptAllHunks(root, p.ID, "")
		}
	}()

	for range 25 {
		_ = svc.ListProposals(root)
	}
	<-done
}

func TestReviewHostAndComposingMultipleTargetedEditsOnSameFile(t *testing.T) {
	svc := NewService()
	host := svc.Host()
	root := t.TempDir()
	base := "# Guide\n\nSection A old.\n\nSection B old.\n"

	_, err := host.Submit(t.Context(), ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-1",
		Path:          root + "/guide.md",
		Source:        "mcp",
		BaseText:      base,
		OldText:       "Section A old.",
		NewText:       "Section A new.",
	})
	if err != nil {
		t.Fatalf("first targeted Submit failed: %v", err)
	}
	combined, err := host.Submit(t.Context(), ProposeInput{
		WorkspaceRoot: root,
		DocumentID:    "doc-1",
		Path:          root + "/guide.md",
		Source:        "mcp",
		BaseText:      base,
		OldText:       "Section B old.",
		NewText:       "Section B new.",
	})
	if err != nil {
		t.Fatalf("second targeted Submit failed: %v", err)
	}
	if len(combined.Hunks) != 2 {
		t.Fatalf("expected 2 composed hunks on same file, got %d: %+v", len(combined.Hunks), combined.Hunks)
	}
	wantProposed := "# Guide\n\nSection A new.\n\nSection B new.\n"
	if combined.ProposedText != wantProposed {
		t.Fatalf("combined ProposedText = %q, want %q", combined.ProposedText, wantProposed)
	}

	list, err := host.List(t.Context(), root)
	if err != nil || len(list) != 1 {
		t.Fatalf("host.List = %+v, %v", list, err)
	}
	decided, err := host.DecideHunk(t.Context(), root, combined.ID, "hunk-1", HunkAccepted, base)
	if err != nil || decided.Hunks[0].Decision != HunkAccepted {
		t.Fatalf("host.DecideHunk = %+v, %v", decided, err)
	}
	diffHunks, err := host.Diff(t.Context(), "a\nb\n", "a\nc\n")
	if err != nil || len(diffHunks) != 1 {
		t.Fatalf("host.Diff = %+v, %v", diffHunks, err)
	}
	afterComplete, err := host.Complete(t.Context(), root, combined.ID)
	if err != nil || len(afterComplete) != 0 {
		t.Fatalf("host.Complete = %+v, %v", afterComplete, err)
	}
}



