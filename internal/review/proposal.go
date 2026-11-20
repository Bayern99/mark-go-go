package review

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

type ChangeProposal struct {
	ID            string     `json:"id"`
	WorkspaceRoot string     `json:"workspaceRoot"`
	DocumentID    string     `json:"documentId,omitempty"`
	Path          string     `json:"path"`
	RelPath       string     `json:"relPath,omitempty"`
	Source        string     `json:"source,omitempty"`
	Summary       string     `json:"summary,omitempty"`
	BaseVersion   int        `json:"baseVersion,omitempty"`
	BaseText      string     `json:"baseText"`
	ProposedText  string     `json:"proposedText"`
	LiveText      string     `json:"liveText"`
	MergedText    string     `json:"mergedText"`
	Hunks         []DiffHunk `json:"hunks"`
	HasConflict   bool       `json:"hasConflict,omitempty"`
	CreatedAt     int64      `json:"createdAt"`
}

type ProposeInput struct {
	ID            string `json:"id,omitempty"`
	WorkspaceRoot string `json:"workspaceRoot"`
	DocumentID    string `json:"documentId,omitempty"`
	Path          string `json:"path"`
	Source        string `json:"source,omitempty"`
	Summary       string `json:"summary,omitempty"`
	BaseVersion   int    `json:"baseVersion,omitempty"`
	BaseText      string `json:"baseText"`
	ProposedText  string `json:"proposedText,omitempty"`
	OldText       string `json:"oldText,omitempty"`
	NewText       string `json:"newText,omitempty"`
	StartLine     int    `json:"startLine,omitempty"`
	EndLine       int    `json:"endLine,omitempty"`
}

type ReviewUpdateEvent struct {
	WorkspaceRoot string           `json:"workspaceRoot"`
	Revision      int64            `json:"revision"`
	Proposals     []ChangeProposal `json:"proposals"`
}

var ReviewUpdated = mygo.NewEvent[ReviewUpdateEvent]("reviewUpdated")

type storedProposal struct {
	proposal     ChangeProposal
	baseClusters []changeCluster
	decisions    map[string]HunkDecision
}

type Service struct {
	mu       sync.Mutex
	seq      int64
	revision int64
	byRoot   map[string][]*storedProposal
	onEvent  func(ReviewUpdateEvent)
}

func NewService() *Service {
	return &Service{
		byRoot: make(map[string][]*storedProposal),
	}
}

func (s *Service) SetEventSink(fn func(ReviewUpdateEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEvent = fn
}

func canonicalRootKey(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return fsutil.CanonicalPath(root)
}

func relativeProposalPath(workspaceRoot, targetPath string) string {
	if targetPath == "" {
		return ""
	}
	if workspaceRoot != "" {
		cRoot := fsutil.CanonicalPath(workspaceRoot)
		cTarget := fsutil.CanonicalPath(targetPath)
		if rel, err := filepath.Rel(cRoot, cTarget); err == nil && filepath.IsLocal(rel) {
			return filepath.ToSlash(rel)
		}
		if rel, err := filepath.Rel(workspaceRoot, targetPath); err == nil && filepath.IsLocal(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(targetPath)
}

func resolveProposedText(base string, input ProposeInput) (string, error) {
	if input.ProposedText != "" || (input.StartLine == 0 && input.EndLine == 0 && input.OldText == "") {
		return input.ProposedText, nil
	}
	if input.StartLine > 0 {
		lines := splitTextLines(base)
		endLine := input.EndLine
		if endLine < input.StartLine {
			endLine = input.StartLine
		}
		if input.StartLine > len(lines)+1 || endLine > len(lines) {
			return "", fmt.Errorf("line range %d-%d is out of bounds for document with %d lines", input.StartLine, endLine, len(lines))
		}
		startIdx := input.StartLine - 1
		endIdx := endLine
		nl := defaultLineEnding(base)
		var b strings.Builder
		for i := 0; i < startIdx; i++ {
			b.WriteString(lines[i].raw)
		}
		if input.NewText != "" {
			b.WriteString(input.NewText)
			if !endsLine(input.NewText) && (endIdx < len(lines) || endsLine(base)) {
				b.WriteString(nl)
			}
		}
		for i := endIdx; i < len(lines); i++ {
			b.WriteString(lines[i].raw)
		}
		return b.String(), nil
	}
	if input.OldText != "" {
		idx := strings.Index(base, input.OldText)
		if idx < 0 {
			return "", errors.New("Target text to replace was not found in the document.")
		}
		return base[:idx] + input.NewText + base[idx+len(input.OldText):], nil
	}
	return input.ProposedText, nil
}

func sameProposalTarget(p ChangeProposal, input ProposeInput) bool {
	if p.Path != "" && input.Path != "" {
		return p.Path == input.Path
	}
	return p.DocumentID != "" && p.DocumentID == input.DocumentID
}

func (s *Service) SubmitProposal(input ProposeInput) (ChangeProposal, error) {
	return s.submitProposal(input, nil, input.BaseText)
}

func (s *Service) ReviseProposal(input ProposeInput, expectedText, liveText string) (ChangeProposal, error) {
	return s.submitProposal(input, &expectedText, liveText)
}

func (s *Service) submitProposal(input ProposeInput, expectedText *string, liveText string) (ChangeProposal, error) {
	if strings.TrimSpace(input.Path) == "" && strings.TrimSpace(input.DocumentID) == "" {
		return ChangeProposal{}, errors.New("A target document path or ID is required.")
	}
	rootKey := canonicalRootKey(input.WorkspaceRoot)
	relPath := relativeProposalPath(input.WorkspaceRoot, input.Path)

	s.mu.Lock()
	queue := s.byRoot[rootKey]
	match := -1
	var existingMatch *storedProposal
	for i, existing := range queue {
		if (expectedText == nil && sameProposalTarget(existing.proposal, input)) ||
			(expectedText != nil && existing.proposal.ID == input.ID) {
			match = i
			existingMatch = existing
			break
		}
	}
	if expectedText != nil && (existingMatch == nil || existingMatch.proposal.ProposedText != *expectedText) {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("The proposal changed during review.")
	}

	if existingMatch != nil && existingMatch.proposal.BaseText == input.BaseText && input.ProposedText == "" && input.OldText != "" {
		if combined, err := resolveProposedText(existingMatch.proposal.ProposedText, input); err == nil {
			input.ProposedText = combined
		}
	}

	proposed, err := resolveProposedText(input.BaseText, input)
	if err != nil {
		s.mu.Unlock()
		return ChangeProposal{}, err
	}
	if expectedText != nil && proposed == *expectedText && input.BaseText == liveText &&
		input.BaseText != existingMatch.proposal.BaseText {
		proposed = compareProposalText(existingMatch, liveText)
	}
	if proposed == input.BaseText {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("The proposed edit makes no changes to the document.")
	}
	_, _, _, clusters := extractClusters(input.BaseText, proposed)
	if len(clusters) == 0 {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("The proposed edit makes no changes to the document.")
	}

	id := input.ID
	if existingMatch != nil {
		id = existingMatch.proposal.ID
	} else {
		for id == "" || s.findLocked(rootKey, id) != nil {
			s.seq++
			id = fmt.Sprintf("proposal-%d", s.seq)
		}
	}

	decisions := make(map[string]HunkDecision, len(clusters))
	for i := range clusters {
		decisions[fmt.Sprintf("hunk-%d", i+1)] = HunkPending
	}

	sp := &storedProposal{
		proposal: ChangeProposal{
			ID:            id,
			WorkspaceRoot: input.WorkspaceRoot,
			DocumentID:    input.DocumentID,
			Path:          input.Path,
			RelPath:       relPath,
			Source:        input.Source,
			Summary:       input.Summary,
			BaseVersion:   input.BaseVersion,
			BaseText:      input.BaseText,
			ProposedText:  proposed,
			LiveText:      input.BaseText,
			CreatedAt:     time.Now().UnixMilli(),
		},
		baseClusters: clusters,
		decisions:    decisions,
	}
	recomputeProposalLocked(sp, liveText)

	if match >= 0 {
		queue[match] = sp
	} else {
		queue = append(queue, sp)
	}
	s.byRoot[rootKey] = queue
	result := cloneProposal(sp.proposal)
	snap := s.changedLocked(rootKey, input.WorkspaceRoot)
	sink := s.onEvent
	s.mu.Unlock()

	emitReviewUpdate(sink, snap)
	return result, nil
}

func (s *Service) ListProposals(workspaceRoot string) []ChangeProposal {
	return s.Snapshot(workspaceRoot).Proposals
}

func (s *Service) Snapshot(workspaceRoot string) ReviewUpdateEvent {
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked(rootKey, workspaceRoot)
}

func (s *Service) GetProposal(workspaceRoot, proposalID string) (ChangeProposal, error) {
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := s.findLocked(rootKey, proposalID)
	if sp == nil {
		return ChangeProposal{}, errors.New("Change proposal not found.")
	}
	return cloneProposal(sp.proposal), nil
}

func (s *Service) SyncProposal(workspaceRoot, proposalID, liveText string) (ChangeProposal, error) {
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	sp := s.findLocked(rootKey, proposalID)
	if sp == nil {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("Change proposal not found.")
	}
	recomputeProposalLocked(sp, liveText)
	result := cloneProposal(sp.proposal)
	snap := s.changedLocked(rootKey, workspaceRoot)
	sink := s.onEvent
	s.mu.Unlock()

	emitReviewUpdate(sink, snap)
	return result, nil
}

func (s *Service) DecideHunk(workspaceRoot, proposalID, hunkID string, decision HunkDecision, liveText string) (ChangeProposal, error) {
	if decision != HunkPending && decision != HunkAccepted && decision != HunkRejected {
		return ChangeProposal{}, fmt.Errorf("invalid hunk decision %q", decision)
	}
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	sp := s.findLocked(rootKey, proposalID)
	if sp == nil {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("Change proposal not found.")
	}
	if liveText != "" {
		recomputeProposalLocked(sp, liveText)
	}
	var target *DiffHunk
	for i := range sp.proposal.Hunks {
		if sp.proposal.Hunks[i].ID == hunkID {
			target = &sp.proposal.Hunks[i]
			break
		}
	}
	if target == nil {
		s.mu.Unlock()
		return ChangeProposal{}, fmt.Errorf("hunk %q not found in proposal", hunkID)
	}
	if decision == HunkAccepted && target.Conflict {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("Cannot accept a hunk that conflicts with concurrent document edits.")
	}
	sp.decisions[hunkID] = decision
	recomputeProposalLocked(sp, sp.proposal.LiveText)
	result := cloneProposal(sp.proposal)
	snap := s.changedLocked(rootKey, workspaceRoot)
	sink := s.onEvent
	s.mu.Unlock()

	emitReviewUpdate(sink, snap)
	return result, nil
}

func (s *Service) AcceptAllHunks(workspaceRoot, proposalID, liveText string) (ChangeProposal, error) {
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	sp := s.findLocked(rootKey, proposalID)
	if sp == nil {
		s.mu.Unlock()
		return ChangeProposal{}, errors.New("Change proposal not found.")
	}
	if liveText != "" {
		recomputeProposalLocked(sp, liveText)
	}
	for _, h := range sp.proposal.Hunks {
		if !h.Conflict {
			sp.decisions[h.ID] = HunkAccepted
		}
	}
	recomputeProposalLocked(sp, sp.proposal.LiveText)
	result := cloneProposal(sp.proposal)
	snap := s.changedLocked(rootKey, workspaceRoot)
	sink := s.onEvent
	s.mu.Unlock()

	emitReviewUpdate(sink, snap)
	return result, nil
}

func (s *Service) DismissProposal(workspaceRoot, proposalID string) ([]ChangeProposal, error) {
	return s.removeProposal(workspaceRoot, proposalID)
}

func (s *Service) CompleteProposal(workspaceRoot, proposalID string) ([]ChangeProposal, error) {
	return s.removeProposal(workspaceRoot, proposalID)
}

func (s *Service) removeProposal(workspaceRoot, proposalID string) ([]ChangeProposal, error) {
	rootKey := canonicalRootKey(workspaceRoot)
	s.mu.Lock()
	queue := s.byRoot[rootKey]
	idx := -1
	for i, sp := range queue {
		if sp.proposal.ID == proposalID {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return nil, errors.New("Change proposal not found.")
	}
	queue = append(queue[:idx], queue[idx+1:]...)
	s.byRoot[rootKey] = queue
	snap := s.changedLocked(rootKey, workspaceRoot)
	sink := s.onEvent
	s.mu.Unlock()

	emitReviewUpdate(sink, snap)
	return snap.Proposals, nil
}

func (s *Service) findLocked(rootKey, proposalID string) *storedProposal {
	for _, sp := range s.byRoot[rootKey] {
		if sp.proposal.ID == proposalID {
			return sp
		}
	}
	return nil
}

func (s *Service) changedLocked(rootKey, workspaceRoot string) ReviewUpdateEvent {
	s.revision++
	return s.snapshotLocked(rootKey, workspaceRoot)
}

func (s *Service) snapshotLocked(rootKey, workspaceRoot string) ReviewUpdateEvent {
	queue := s.byRoot[rootKey]
	proposals := make([]ChangeProposal, 0, len(queue))
	for _, sp := range queue {
		proposals = append(proposals, cloneProposal(sp.proposal))
	}
	return ReviewUpdateEvent{
		WorkspaceRoot: workspaceRoot,
		Revision:      s.revision,
		Proposals:     proposals,
	}
}

func emitReviewUpdate(sink func(ReviewUpdateEvent), ev ReviewUpdateEvent) {
	if sink != nil {
		sink(ev)
	}
	_ = ReviewUpdated.Broadcast(ev)
}

func cloneProposal(p ChangeProposal) ChangeProposal {
	out := p
	if len(p.Hunks) > 0 {
		out.Hunks = make([]DiffHunk, len(p.Hunks))
		for i, h := range p.Hunks {
			ch := h
			if len(h.Lines) > 0 {
				ch.Lines = make([]DiffLine, len(h.Lines))
				copy(ch.Lines, h.Lines)
			}
			out.Hunks[i] = ch
		}
	}
	return out
}

func clustersConflict(prop changeCluster, user changeCluster) (bool, bool) {
	if prop.oldStart == user.oldStart && prop.oldEnd == user.oldEnd && len(prop.inserts) == len(user.inserts) {
		same := true
		for i := range prop.inserts {
			if prop.inserts[i].raw != user.inserts[i].raw {
				same = false
				break
			}
		}
		if same {
			return false, true
		}
	}
	pEmpty := prop.oldStart == prop.oldEnd
	uEmpty := user.oldStart == user.oldEnd
	if pEmpty && uEmpty {
		return prop.oldStart == user.oldStart, false
	}
	if pEmpty {
		return user.oldStart < prop.oldStart && prop.oldStart < user.oldEnd, false
	}
	if uEmpty {
		return prop.oldStart < user.oldStart && user.oldStart < prop.oldEnd, false
	}
	start := max(prop.oldStart, user.oldStart)
	end := min(prop.oldEnd, user.oldEnd)
	return start < end, false
}

func compareProposalText(sp *storedProposal, liveText string) string {
	base, _, _, live := extractClusters(sp.proposal.BaseText, liveText)
	changes := slices.Clone(sp.baseClusters)
	for _, change := range live {
		if !slices.ContainsFunc(sp.baseClusters, func(proposed changeCluster) bool {
			overlaps, identical := clustersConflict(proposed, change)
			return overlaps || identical
		}) {
			changes = append(changes, change)
		}
	}
	slices.SortFunc(changes, func(a, b changeCluster) int {
		if byStart := cmp.Compare(a.oldStart, b.oldStart); byStart != 0 {
			return byStart
		}
		return cmp.Compare(a.oldEnd, b.oldEnd)
	})
	var out []string
	cursor := 0
	for _, change := range changes {
		for ; cursor < change.oldStart; cursor++ {
			out = append(out, base[cursor].raw)
		}
		for _, ins := range change.inserts {
			out = append(out, ins.raw)
		}
		cursor = change.oldEnd
	}
	for _, line := range base[cursor:] {
		out = append(out, line.raw)
	}
	return joinRawLines(out, defaultLineEnding(liveText))
}

func recomputeProposalLocked(sp *storedProposal, liveText string) {
	sp.proposal.LiveText = liveText
	baseText := sp.proposal.BaseText
	liveLines := splitTextLines(liveText)
	_, _, _, userClusters := extractClusters(baseText, liveText)

	baseToLive := func(baseIdx int) int {
		delta := 0
		for _, u := range userClusters {
			if u.oldEnd <= baseIdx {
				delta += len(u.inserts) - (u.oldEnd - u.oldStart)
			}
		}
		res := baseIdx + delta
		if res < 0 {
			return 0
		}
		if res > len(liveLines) {
			return len(liveLines)
		}
		return res
	}

	hunks := make([]DiffHunk, 0, len(sp.baseClusters))
	hasConflict := false

	type resolvedCluster struct {
		hunkID    string
		liveStart int
		liveEnd   int
		inserts   []editOp
		conflict  bool
		decision  HunkDecision
	}
	resolved := make([]resolvedCluster, 0, len(sp.baseClusters))

	for idx, c := range sp.baseClusters {
		hunkID := fmt.Sprintf("hunk-%d", idx+1)
		conflict := false
		conflictReason := ""
		alreadyApplied := false
		for _, u := range userClusters {
			collides, identical := clustersConflict(c, u)
			if identical {
				alreadyApplied = true
				break
			}
			if collides {
				conflict = true
				conflictReason = fmt.Sprintf("Concurrent edit overlaps base line %d", c.oldStart+1)
				break
			}
		}

		decision := sp.decisions[hunkID]
		if decision == "" {
			decision = HunkPending
		}
		if conflict {
			hasConflict = true
		}

		liveStart := baseToLive(c.oldStart)
		liveEnd := liveStart + (c.oldEnd - c.oldStart)
		if conflict || alreadyApplied {
			liveEnd = baseToLive(c.oldEnd)
			if liveEnd < liveStart {
				liveEnd = liveStart
			}
		}
		if liveStart > len(liveLines) {
			liveStart = len(liveLines)
		}
		if liveEnd > len(liveLines) {
			liveEnd = len(liveLines)
		}

		preStart := max(0, liveStart-DefaultContextLines)
		if idx > 0 && preStart < resolved[idx-1].liveEnd {
			preStart = resolved[idx-1].liveEnd
		}
		postEnd := min(len(liveLines), liveEnd+DefaultContextLines)
		if idx+1 < len(sp.baseClusters) {
			nextLiveStart := baseToLive(sp.baseClusters[idx+1].oldStart)
			if postEnd > nextLiveStart {
				postEnd = nextLiveStart
			}
		}

		var lines []DiffLine
		newLineCursor := c.newStart + 1 - (liveStart - preStart)
		if newLineCursor < 1 {
			newLineCursor = 1
		}
		for lIdx := preStart; lIdx < liveStart; lIdx++ {
			lines = append(lines, DiffLine{
				Kind:    DiffLineContext,
				Text:    liveLines[lIdx].content,
				OldLine: lIdx + 1,
				NewLine: newLineCursor,
			})
			newLineCursor++
		}
		for dIdx, del := range c.deletes {
			lines = append(lines, DiffLine{
				Kind:    DiffLineDelete,
				Text:    del.text,
				OldLine: liveStart + dIdx + 1,
			})
		}
		for _, ins := range c.inserts {
			lines = append(lines, DiffLine{
				Kind:    DiffLineInsert,
				Text:    ins.text,
				NewLine: newLineCursor,
			})
			newLineCursor++
		}
		for lIdx := liveEnd; lIdx < postEnd; lIdx++ {
			lines = append(lines, DiffLine{
				Kind:    DiffLineContext,
				Text:    liveLines[lIdx].content,
				OldLine: lIdx + 1,
				NewLine: newLineCursor,
			})
			newLineCursor++
		}

		oldStart := liveStart + 1
		oldCount := 0
		newStart := c.newStart + 1
		newCount := 0
		foundOld := false
		foundNew := false
		for _, l := range lines {
			if l.Kind != DiffLineInsert {
				if !foundOld && l.OldLine > 0 {
					oldStart = l.OldLine
					foundOld = true
				}
				oldCount++
			}
			if l.Kind != DiffLineDelete {
				if !foundNew && l.NewLine > 0 {
					newStart = l.NewLine
					foundNew = true
				}
				newCount++
			}
		}

		hunks = append(hunks, DiffHunk{
			ID:             hunkID,
			Header:         fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount),
			OldStart:       oldStart,
			OldLines:       oldCount,
			NewStart:       newStart,
			NewLines:       newCount,
			ChangeOldLine:  liveStart + 1,
			ChangeOldCount: len(c.deletes),
			ChangeNewLine:  c.newStart + 1,
			ChangeNewCount: len(c.inserts),
			Lines:          lines,
			Decision:       decision,
			Conflict:       conflict,
			ConflictReason: conflictReason,
		})

		resolved = append(resolved, resolvedCluster{
			hunkID:    hunkID,
			liveStart: liveStart,
			liveEnd:   liveEnd,
			inserts:   c.inserts,
			conflict:  conflict || alreadyApplied,
			decision:  decision,
		})
	}

	sp.proposal.Hunks = hunks
	sp.proposal.HasConflict = hasConflict

	anyAccepted := false
	anyRejected := false
	for _, rc := range resolved {
		if rc.decision == HunkAccepted {
			anyAccepted = true
		}
		if rc.decision == HunkRejected {
			anyRejected = true
		}
	}

	shouldApply := func(rc resolvedCluster) bool {
		if rc.conflict {
			return false
		}
		if anyAccepted {
			return rc.decision == HunkAccepted
		}
		if anyRejected {
			return rc.decision != HunkRejected
		}
		return true
	}

	nl := defaultLineEnding(liveText)
	if !strings.ContainsAny(liveText, "\r\n") {
		nl = defaultLineEnding(sp.proposal.ProposedText)
	}
	var outRaw []string
	cursor := 0
	for _, rc := range resolved {
		if !shouldApply(rc) {
			continue
		}
		for cursor < rc.liveStart && cursor < len(liveLines) {
			outRaw = append(outRaw, liveLines[cursor].raw)
			cursor++
		}
		for _, ins := range rc.inserts {
			outRaw = append(outRaw, ins.raw)
		}
		cursor = rc.liveEnd
	}
	for cursor < len(liveLines) {
		outRaw = append(outRaw, liveLines[cursor].raw)
		cursor++
	}

	sp.proposal.MergedText = joinRawLines(outRaw, nl)
}

func joinRawLines(lines []string, ending string) string {
	var b strings.Builder
	for i, raw := range lines {
		if i+1 < len(lines) && !endsLine(raw) {
			b.WriteString(raw)
			b.WriteString(ending)
		} else {
			b.WriteString(raw)
		}
	}
	return b.String()
}
