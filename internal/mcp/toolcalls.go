package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

// maxReadDocumentBytes bounds one read_document result, which keeps a whole-file
// read of a large document from flooding a client's context.
const maxReadDocumentBytes = 256 << 10

// resolveTargetDocument finds the document a tool call names, preferring the
// open document id over the path.
func (s *Server) resolveTargetDocument(root, documentID, docPath string) (workspace.DocumentMCPState, error) {
	if s.deps.Workspace == nil {
		return workspace.DocumentMCPState{}, errors.New("the workspace bridge is unavailable")
	}
	target := firstNonEmpty(documentID, docPath)
	if target == "" {
		return workspace.DocumentMCPState{}, fmt.Errorf("%w: pass documentId or path", workspace.ErrPathDenied)
	}
	doc, err := s.deps.Workspace.MCPDocument(root, target)
	if err != nil && documentID != "" && docPath != "" {
		doc, err = s.deps.Workspace.MCPDocument(root, docPath)
	}
	return doc, err
}

// workDir is the research work folder that records documents this channel
// creates. The external channel has no Feynman session, so it uses the folder
// the app keeps for the workspace itself.
func (s *Server) workDir(binding *sessionBinding) string {
	if binding.work != "" {
		return binding.work
	}
	if s.deps.Workspace == nil {
		return ""
	}
	userData := s.deps.Workspace.UserDataDir()
	if userData == "" {
		return ""
	}
	return review.WorkDir(userData, binding.root)
}

func documentLines(text string) []string {
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

func lineRangeText(text string, startLine, endLine int) (string, error) {
	lines := documentLines(text)
	if startLine < 1 || endLine < startLine {
		return "", fmt.Errorf("startLine must be 1 or more and endLine must not be before it")
	}
	if endLine > len(lines) {
		return "", fmt.Errorf("endLine %d is past the document's last line %d", endLine, len(lines))
	}
	return strings.Join(lines[startLine-1:endLine], "\n"), nil
}

// truncateAtLine cuts a read at a line boundary without breaking a rune, and
// reports whether anything was dropped.
func truncateAtLine(text string) (string, bool) {
	if len(text) <= maxReadDocumentBytes {
		return text, false
	}
	cut := text[:maxReadDocumentBytes]
	if next := strings.LastIndexByte(cut, '\n'); next >= 0 {
		cut = cut[:next]
	} else {
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
	}
	return cut, true
}

func (s *Server) toolReadDocument(binding *sessionBinding, rawArgs json.RawMessage) (mcpCallToolResult, *rpcError) {
	var args struct {
		DocumentID string `json:"documentId"`
		Path       string `json:"path"`
		SectionID  string `json:"sectionId"`
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		StartLine  int    `json:"startLine"`
		EndLine    int    `json:"endLine"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	if firstNonEmpty(args.DocumentID, args.Path) == "" {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "Pass documentId or path.")), nil
	}
	if args.SectionID != "" && args.Heading != "" {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "Pass sectionId or heading, not both.")), nil
	}
	if (args.StartLine == 0) != (args.EndLine == 0) {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "Pass startLine and endLine together.")), nil
	}
	if args.StartLine != 0 && (args.SectionID != "" || args.Heading != "") {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "Pass a section or a line range, not both.")), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	lines := documentLines(doc.Text)

	out := map[string]any{
		"documentId": doc.ID,
		"path":       doc.Path,
		"relPath":    doc.RelPath,
		"version":    doc.Version,
		"open":       doc.Open,
		"edited":     doc.Edited,
		"conflict":   doc.Conflict,
		"readOnly":   doc.ReadOnly,
		"text":       doc.Text,
		"startLine":  1,
		"endLine":    len(lines),
		"truncated":  false,
	}
	switch {
	case args.SectionID != "" || args.Heading != "":
		sections := workspace.ExtractSections(doc.Text)
		section, ok := findMatchingSection(sections, args.SectionID, args.Heading, args.Occurrence)
		if !ok {
			return toolFailure(newAppError(ipc.CodeNotFound, "That heading or section is not in the document.")), nil
		}
		endLine := min(section.EndLine, len(lines))
		text, err := lineRangeText(doc.Text, section.StartLine, endLine)
		if err != nil {
			return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
		}
		out["text"] = text
		out["startLine"] = section.StartLine
		out["endLine"] = endLine
		out["heading"] = section.Heading
		out["sectionId"] = section.ID
		out["hash"] = section.Hash
	case args.StartLine != 0:
		text, err := lineRangeText(doc.Text, args.StartLine, args.EndLine)
		if err != nil {
			return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
		}
		out["text"] = text
		out["startLine"] = args.StartLine
		out["endLine"] = args.EndLine
	}
	text, truncated := truncateAtLine(out["text"].(string))
	if truncated {
		out["text"] = text
		out["truncated"] = true
		out["endLine"] = out["startLine"].(int) + max(len(documentLines(text)), 1) - 1
	}
	return toolJSON(out)
}

func (s *Server) toolGetWorkspace(binding *sessionBinding, rawArgs json.RawMessage) (mcpCallToolResult, *rpcError) {
	var args struct{}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	if s.deps.Workspace == nil {
		return toolFailure(newAppError(ipc.CodeWorkspaceNotOpen, "The workspace bridge is unavailable.")), nil
	}
	state, err := s.deps.Workspace.MCPWorkspaceState(binding.root)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	openDocuments := make([]map[string]any, 0, len(state.OpenDocuments))
	for _, doc := range state.OpenDocuments {
		rel := ""
		if doc.Path != "" && fsutil.FolderContains(state.Root, doc.Path) {
			if r, err := filepath.Rel(state.Root, doc.Path); err == nil {
				rel = filepath.ToSlash(r)
			}
		}
		openDocuments = append(openDocuments, map[string]any{
			"documentId": doc.ID,
			"name":       doc.Name,
			"path":       doc.Path,
			"relPath":    rel,
			"edited":     doc.Edited,
			"conflict":   doc.Conflict,
			"readOnly":   doc.ReadOnly,
		})
	}
	files := make([]map[string]any, 0, len(state.Files))
	for _, file := range state.Files {
		files = append(files, map[string]any{
			"relPath":    file.RelPath,
			"name":       file.Name,
			"open":       file.Open,
			"documentId": file.DocumentID,
		})
	}
	limit := state.FilesLimit
	if limit == 0 {
		limit = workspace.MaxWorkspaceListingFiles
	}
	return toolJSON(map[string]any{
		"root":             state.Root,
		"name":             state.Name,
		"activeDocumentId": state.ActiveDocumentID,
		"openDocuments":    openDocuments,
		"folders":          state.Folders,
		"files":            files,
		"truncated":        state.Truncated,
		"filesLimit":       limit,
	})
}

func (s *Server) toolProposeEdit(binding *sessionBinding, rawArgs json.RawMessage) (result mcpCallToolResult, rpcErr *rpcError) {
	if s.deps.Review == nil {
		return toolFailure(newAppError(ipc.CodeAppNotRunning, "The review service is unavailable.")), nil
	}
	var args struct {
		DocumentID   string            `json:"documentId"`
		Path         string            `json:"path"`
		Summary      string            `json:"summary"`
		ProposedText editField[string] `json:"proposedText"`
		OldText      editField[string] `json:"oldText"`
		NewText      editField[string] `json:"newText"`
		StartLine    editField[int]    `json:"startLine"`
		EndLine      editField[int]    `json:"endLine"`
		BaseVersion  *int64            `json:"baseVersion"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	if failure := validateEditModes(args.ProposedText, args.OldText, args.NewText, args.StartLine, args.EndLine); failure != nil {
		return toolFailure(failure), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	defer func() { result.auditTarget = firstNonEmpty(doc.Path, doc.ID) }()
	if doc.ReadOnly {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "That document is read-only.")), nil
	}
	if doc.Deleted {
		return toolFailure(newAppError(ipc.CodeNotFound, "That document has been deleted.")), nil
	}
	if args.StartLine.value != 0 {
		if args.BaseVersion == nil {
			return toolFailure(newAppError(ipc.CodeVersionConflict, "baseVersion is required for a line-range edit; read the document first.")), nil
		}
		if *args.BaseVersion != doc.Version {
			return toolFailure(newAppError(ipc.CodeVersionConflict, "The document version changed; read it again before proposing a line-range edit.")), nil
		}
	}

	proposal, err := s.deps.Review.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: binding.root,
		DocumentID:    doc.ID,
		Path:          doc.Path,
		Source:        binding.source(),
		Summary:       strings.TrimSpace(args.Summary),
		BaseVersion:   doc.EditorVersion,
		BaseText:      doc.Text,
		ProposedText:  args.ProposedText.value,
		OldText:       args.OldText.value,
		NewText:       args.NewText.value,
		StartLine:     args.StartLine.value,
		EndLine:       args.EndLine.value,
	})
	if err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	return toolJSON(map[string]any{
		"proposalId": proposal.ID,
		"revision":   proposal.Revision,
		"hunksCount": len(proposal.Hunks),
		"conflicted": proposal.HasConflict,
		"summary":    proposal.Summary,
	})
}

type editField[T string | int] struct {
	value   T
	present bool
}

func (f *editField[T]) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		return errors.New("edit fields cannot be null")
	}
	f.present = true
	return json.Unmarshal(raw, &f.value)
}

func validateEditModes(full, old, replacement editField[string], start, end editField[int]) *appError {
	invalid := func(message string) *appError { return newAppError(ipc.CodeInvalidArgs, message) }
	switch {
	case full.present && (old.present || replacement.present || start.present || end.present):
		return invalid("Pass proposedText on its own, without oldText, newText, startLine or endLine.")
	case full.present:
		return nil
	case old.present && (start.present || end.present):
		return invalid("Pass an oldText snippet or a line range, not both.")
	case old.present && old.value != "" && replacement.present:
		return nil
	case start.present && end.present && replacement.present:
		if start.value < 1 || end.value < start.value {
			return invalid("startLine must be 1 or more and endLine must not be before it.")
		}
		return nil
	default:
		return invalid("Pass proposedText, non-empty oldText with newText, or startLine and endLine with newText.")
	}
}

func validateCreateRelPath(root, rawPath string) (string, string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" || strings.Contains(trimmed, `\`) {
		return "", "", fmt.Errorf("%w: a workspace-relative Markdown path is required", workspace.ErrPathDenied)
	}
	var rel string
	if filepath.IsAbs(trimmed) {
		canonRoot := fsutil.CanonicalPath(root)
		canonTarget := fsutil.CanonicalPath(trimmed)
		if !fsutil.FolderContains(canonRoot, canonTarget) {
			return "", "", fmt.Errorf("%w: it escapes the workspace root", workspace.ErrPathDenied)
		}
		r, err := filepath.Rel(canonRoot, canonTarget)
		if err != nil || !filepath.IsLocal(r) {
			return "", "", fmt.Errorf("%w: it escapes the workspace root", workspace.ErrPathDenied)
		}
		rel = filepath.ToSlash(r)
	} else {
		rel = path.Clean(trimmed)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") || !fs.ValidPath(rel) {
		return "", "", fmt.Errorf("%w: it escapes the workspace root", workspace.ErrPathDenied)
	}
	if !fsutil.MarkdownName(rel) {
		return "", "", fmt.Errorf("%w: a new document needs a .md or .markdown name", workspace.ErrPathDenied)
	}
	for dir := rel; dir != "."; dir = path.Dir(dir) {
		name := strings.ToLower(path.Base(dir))
		if fsutil.SkipDirName(name) && dir != "outputs/.plans" && dir != "outputs/.drafts" {
			return "", "", fmt.Errorf("%w: it is in an excluded or hidden directory", workspace.ErrPathDenied)
		}
	}
	return rel, filepath.Join(fsutil.CanonicalPath(root), filepath.FromSlash(rel)), nil
}

func (s *Server) toolCreateDocument(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (result mcpCallToolResult, rpcErr *rpcError) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Open    bool   `json:"open"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "A workspace-relative Markdown path is required.")), nil
	}
	rel, absPath, err := validateCreateRelPath(binding.root, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	defer func() { result.auditTarget = absPath }()
	if len(args.Content) > review.MaxResearchArtifactBytes {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "The document exceeds the 8 MiB limit.")), nil
	}
	if !utf8.ValidString(args.Content) {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, "The document content must be valid UTF-8.")), nil
	}
	work := s.workDir(binding)
	if work == "" {
		return toolFailure(newAppError(ipc.CodeAppNotRunning, "The app has no user data folder to record created documents in.")), nil
	}
	if err := review.CreateDocumentArtifact(work, binding.root, rel, []byte(args.Content)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return toolFailure(newAppError(ipc.CodeInvalidArgs, fmt.Sprintf("The file %q already exists; use propose_edit to change an existing document.", rel))), nil
		}
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}

	var docID string
	if args.Open && s.deps.Workspace != nil {
		if info, err := s.deps.Workspace.MCPOpenDocument(ctx, binding.root, rel, 0, false); err == nil {
			docID = info.ID
		}
	}
	return toolJSON(map[string]any{
		"created":    true,
		"path":       absPath,
		"relPath":    rel,
		"bytes":      len(args.Content),
		"documentId": docID,
	})
}

func findMatchingSection(sections []workspace.Section, sectionID, heading string, occurrence int) (workspace.Section, bool) {
	if sectionID != "" {
		for _, sec := range sections {
			if sec.ID == sectionID || sec.Hash == sectionID {
				return sec, true
			}
		}
		return workspace.Section{}, false
	}
	if occurrence < 1 {
		occurrence = 1
	}
	key := workspace.NormalizeHeadingText(heading)
	if key == "" {
		return workspace.Section{}, false
	}
	for _, sec := range sections {
		if sec.Occurrence == occurrence && workspace.NormalizeHeadingText(sec.Heading) == key {
			return sec, true
		}
	}
	return workspace.Section{}, false
}

func (s *Server) toolOpenDocument(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (mcpCallToolResult, *rpcError) {
	var args struct {
		DocumentID string `json:"documentId"`
		Path       string `json:"path"`
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		SectionID  string `json:"sectionId"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	if s.deps.Workspace == nil {
		return toolFailure(newAppError(ipc.CodeAppNotRunning, "The workspace bridge is unavailable.")), nil
	}

	revealOffset := 0
	hasReveal := false
	matchedHeading := ""
	if strings.TrimSpace(args.Heading) != "" || strings.TrimSpace(args.SectionID) != "" {
		sections := workspace.ExtractSections(doc.Text)
		sec, ok := findMatchingSection(sections, strings.TrimSpace(args.SectionID), args.Heading, args.Occurrence)
		if !ok {
			return toolFailure(newAppError(ipc.CodeNotFound, "That heading or section is not in the document.")), nil
		}
		revealOffset = sec.Start
		hasReveal = true
		matchedHeading = sec.Heading
	}

	target := firstNonEmpty(doc.ID, doc.RelPath)
	if target == "" {
		target = doc.Path
	}
	info, err := s.deps.Workspace.MCPOpenDocument(ctx, binding.root, target, revealOffset, hasReveal)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	return toolJSON(map[string]any{
		"opened":     true,
		"documentId": info.ID,
		"name":       info.Name,
		"path":       info.Path,
		"relPath":    doc.RelPath,
		"heading":    matchedHeading,
		"offset":     revealOffset,
	})
}

func (s *Server) toolWriteGenUIView(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (result mcpCallToolResult, rpcErr *rpcError) {
	var args struct {
		DocumentID string          `json:"documentId"`
		Path       string          `json:"path"`
		A2UI       json.RawMessage `json:"a2ui"`
		Sources    json.RawMessage `json:"sources"`
		Request    string          `json:"request"`
		Model      string          `json:"model"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	defer func() { result.auditTarget = firstNonEmpty(doc.Path, doc.ID) }()
	if doc.Path == "" || !fsutil.MarkdownName(doc.Path) || !fsutil.FolderContains(binding.root, doc.Path) {
		return toolFailure(newAppError(ipc.CodePathDenied, "The target must be a saved Markdown file inside the workspace.")), nil
	}
	if len(args.A2UI) == 0 {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, `"a2ui" must be a non-empty array of A2UI messages.`)), nil
	}
	parsedSources, err := genui.ParseSources(args.Sources)
	if err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	sections := workspace.ExtractSections(doc.Text)
	content, version, err := genui.AssembleView(genui.ViewOptions{
		A2UI:        args.A2UI,
		Sources:     parsedSources,
		Model:       strings.TrimSpace(args.Model),
		Request:     strings.TrimSpace(args.Request),
		GeneratedAt: time.Now(),
		Hash: func(_ string, src genui.AgentSource) string {
			if sec, ok := findMatchingSection(sections, "", src.Heading, src.Occurrence); ok {
				return sec.Hash
			}
			return ""
		},
	})
	if err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	if len(content) > genui.MaxViewBytes {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, fmt.Sprintf("The GenUI view exceeds the %d byte limit.", genui.MaxViewBytes))), nil
	}
	if err := genui.ValidateCandidate(ctx, content); err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	viewPath := genui.ViewPath(doc.Path)
	if err := genui.ReplaceView(viewPath, content, nil); err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	relView, _ := filepath.Rel(fsutil.CanonicalPath(binding.root), fsutil.CanonicalPath(viewPath))
	return toolJSON(map[string]any{
		"written":      true,
		"documentPath": doc.Path,
		"viewPath":     viewPath,
		"relViewPath":  filepath.ToSlash(relView),
		"version":      version,
	})
}

func (s *Server) toolCreateCheckpoint(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (result mcpCallToolResult, rpcErr *rpcError) {
	var args struct {
		DocumentID string `json:"documentId"`
		Path       string `json:"path"`
		Label      string `json:"label"`
	}
	if failure := decodeArgs(rawArgs, &args); failure != nil {
		return toolFailure(failure), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolFailure(classifyFailure(err)), nil
	}
	defer func() { result.auditTarget = firstNonEmpty(doc.Path, doc.ID) }()
	if doc.Path == "" || !fsutil.MarkdownName(doc.Path) || !fsutil.FolderContains(binding.root, doc.Path) {
		return toolFailure(newAppError(ipc.CodePathDenied, "Checkpoints need a saved Markdown document inside the workspace.")), nil
	}
	if doc.Edited || doc.Conflict || doc.Deleted {
		return toolFailure(newAppError(ipc.CodeVersionConflict, "Save the document and resolve any conflicts before creating a checkpoint.")), nil
	}
	var userDataDir string
	if s.deps.Workspace != nil {
		userDataDir = s.deps.Workspace.UserDataDir()
	}
	created, count, err := review.CreateCheckpoint(ctx, binding.root, doc.Path, doc.Text, strings.TrimSpace(args.Label), binding.source(), userDataDir)
	if err != nil {
		return toolFailure(newAppError(ipc.CodeInvalidArgs, err.Error())), nil
	}
	return toolJSON(map[string]any{
		"created":          true,
		"checkpoint":       created,
		"checkpointsCount": count,
		"path":             doc.Path,
		"relPath":          doc.RelPath,
	})
}
