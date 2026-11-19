package mcp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func (s *Server) handleToolsList() any {
	return map[string]any{
		"tools": []mcpTool{
			{
				Name:        "propose_edit",
				Title:       "Propose Document Edit",
				Description: "Queue a Change proposal for an existing Markdown document in the workspace without silently overwriting the open editor buffer or file on disk.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"documentId":   map[string]any{"type": "string", "description": "Open document ID from markgogo://workspace"},
						"path":         map[string]any{"type": "string", "description": "Workspace-relative or absolute path to the Markdown document"},
						"summary":      map[string]any{"type": "string", "description": "Short human-readable summary of the proposed change"},
						"source":       map[string]any{"type": "string", "description": "Proposal origin tag (defaults to \"mcp\")"},
						"proposedText": map[string]any{"type": "string", "description": "Complete proposed Markdown text"},
						"oldText":      map[string]any{"type": "string", "description": "Exact existing snippet to replace with newText"},
						"newText":      map[string]any{"type": "string", "description": "Replacement text for oldText or startLine..endLine"},
						"startLine":    map[string]any{"type": "integer", "description": "1-based start line for targeted replacement"},
						"endLine":      map[string]any{"type": "integer", "description": "1-based inclusive end line for targeted replacement"},
						"baseVersion":  map[string]any{"type": "integer", "description": "Optional base buffer version observed when reading the document"},
					},
				},
			},
			{
				Name:        "create_document",
				Title:       "Create Markdown Document",
				Description: "Atomically create a new Markdown document inside the workspace using O_EXCL semantics, refusing to overwrite existing files.",
				InputSchema: map[string]any{
					"type":     "object",
					"required": []string{"path", "content"},
					"properties": map[string]any{
						"path":    map[string]any{"type": "string", "description": "Workspace-relative Markdown file path (e.g. outputs/summary.md)"},
						"content": map[string]any{"type": "string", "description": "UTF-8 Markdown content to write"},
						"open":    map[string]any{"type": "boolean", "description": "Whether to open the created document in the workspace window"},
					},
				},
			},
			{
				Name:        "open_document",
				Title:       "Open Document",
				Description: "Open or focus a workspace Markdown document and optionally jump to a heading or section.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"documentId": map[string]any{"type": "string", "description": "Open document ID"},
						"path":       map[string]any{"type": "string", "description": "Workspace-relative or absolute path to the Markdown document"},
						"heading":    map[string]any{"type": "string", "description": "Optional heading text to focus"},
						"occurrence": map[string]any{"type": "integer", "description": "1-based heading occurrence (default 1)"},
						"sectionId":  map[string]any{"type": "string", "description": "Optional section ID/hash from sections resource"},
					},
				},
			},
			{
				Name:        "write_genui_view",
				Title:       "Write GenUI View",
				Description: "Validate an A2UI 1.0 + markgogo.dev:genui/v1 candidate view via the pure-Go headless validator and atomically write its .genui.json file.",
				InputSchema: map[string]any{
					"type":     "object",
					"required": []string{"a2ui"},
					"properties": map[string]any{
						"documentId": map[string]any{"type": "string", "description": "Open document ID"},
						"path":       map[string]any{"type": "string", "description": "Workspace-relative or absolute path to the source Markdown document"},
						"a2ui":       map[string]any{"type": "array", "description": "Non-empty array of A2UI 1.0 messages"},
						"sources":    map[string]any{"type": "object", "description": "Source citation map keyed by source ID -> {heading, occurrence}"},
						"request":    map[string]any{"type": "string", "description": "Optional generation request description"},
						"model":      map[string]any{"type": "string", "description": "Optional model identifier"},
					},
				},
			},
			{
				Name:        "create_checkpoint",
				Title:       "Create Checkpoint",
				Description: "Record a labeled provenance checkpoint for a saved Markdown document.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"documentId": map[string]any{"type": "string", "description": "Open document ID"},
						"path":       map[string]any{"type": "string", "description": "Workspace-relative or absolute path to the saved Markdown document"},
						"label":      map[string]any{"type": "string", "description": "Human-readable checkpoint label"},
						"source":     map[string]any{"type": "string", "description": "Provenance source tag (defaults to \"mcp\")"},
					},
				},
			},
		},
	}
}

func toolError(msg string) mcpCallToolResult {
	return mcpCallToolResult{
		Content: []mcpTextContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func toolJSON(payload any) (mcpCallToolResult, *rpcError) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return mcpCallToolResult{}, &rpcError{Code: errCodeInternalError, Message: err.Error()}
	}
	return mcpCallToolResult{
		Content:           []mcpTextContent{{Type: "text", Text: string(raw)}},
		StructuredContent: payload,
	}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, binding *sessionBinding, rawParams json.RawMessage) (any, *rpcError) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(rawParams) == 0 || json.Unmarshal(rawParams, &params) != nil || strings.TrimSpace(params.Name) == "" {
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "tool name is required"}
	}
	args := params.Arguments
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

	switch params.Name {
	case "propose_edit":
		return s.toolProposeEdit(binding, args)
	case "create_document":
		return s.toolCreateDocument(ctx, binding, args)
	case "open_document":
		return s.toolOpenDocument(ctx, binding, args)
	case "write_genui_view":
		return s.toolWriteGenUIView(ctx, binding, args)
	case "create_checkpoint":
		return s.toolCreateCheckpoint(ctx, binding, args)
	default:
		return nil, &rpcError{Code: errCodeMethodNotFound, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func (s *Server) resolveTargetDocument(root, documentID, docPath string) (workspace.DocumentMCPState, error) {
	if s.deps.Workspace == nil {
		return workspace.DocumentMCPState{}, errors.New("workspace bridge is unavailable")
	}
	target := firstNonEmpty(documentID, docPath)
	if target == "" {
		return workspace.DocumentMCPState{}, errors.New("documentId or path is required")
	}
	doc, err := s.deps.Workspace.MCPDocument(root, target)
	if err != nil && documentID != "" && docPath != "" {
		doc, err = s.deps.Workspace.MCPDocument(root, docPath)
	}
	return doc, err
}

func (s *Server) toolProposeEdit(binding *sessionBinding, rawArgs json.RawMessage) (any, *rpcError) {
	if s.deps.Review == nil {
		return nil, &rpcError{Code: errCodeInternalError, Message: "review service is unavailable"}
	}
	var args struct {
		DocumentID   string `json:"documentId"`
		Path         string `json:"path"`
		Summary      string `json:"summary"`
		Source       string `json:"source"`
		ProposedText string `json:"proposedText"`
		OldText      string `json:"oldText"`
		NewText      string `json:"newText"`
		StartLine    int    `json:"startLine"`
		EndLine      int    `json:"endLine"`
		BaseVersion  int    `json:"baseVersion"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return toolError("invalid arguments: " + err.Error()), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolError(err.Error()), nil
	}
	if doc.ReadOnly {
		return toolError("document is read-only"), nil
	}
	if doc.Deleted {
		return toolError("document has been deleted"), nil
	}

	source := strings.TrimSpace(args.Source)
	if source == "" {
		source = "mcp"
	}
	baseVersion := args.BaseVersion
	if baseVersion == 0 {
		baseVersion = doc.Version
	}

	proposal, err := s.deps.Review.SubmitProposal(review.ProposeInput{
		WorkspaceRoot: binding.root,
		DocumentID:    doc.ID,
		Path:          doc.Path,
		Source:        source,
		Summary:       strings.TrimSpace(args.Summary),
		BaseVersion:   baseVersion,
		BaseText:      doc.Text,
		ProposedText:  args.ProposedText,
		OldText:       args.OldText,
		NewText:       args.NewText,
		StartLine:     args.StartLine,
		EndLine:       args.EndLine,
	})
	if err != nil {
		return toolError(err.Error()), nil
	}
	return toolJSON(proposal)
}

func validateCreateRelPath(root, rawPath string) (string, string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" || strings.Contains(trimmed, `\`) {
		return "", "", errors.New("a workspace-relative Markdown path is required")
	}
	var rel string
	if filepath.IsAbs(trimmed) {
		canonRoot := fsutil.CanonicalPath(root)
		canonTarget := fsutil.CanonicalPath(trimmed)
		if !fsutil.FolderContains(canonRoot, canonTarget) {
			return "", "", errors.New("path escapes workspace root")
		}
		r, err := filepath.Rel(canonRoot, canonTarget)
		if err != nil || !filepath.IsLocal(r) {
			return "", "", errors.New("path escapes workspace root")
		}
		rel = filepath.ToSlash(r)
	} else {
		rel = path.Clean(trimmed)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") || !fs.ValidPath(rel) {
		return "", "", errors.New("path escapes workspace root")
	}
	if !fsutil.MarkdownName(rel) {
		return "", "", errors.New("new document must have a .md or .markdown extension")
	}
	for dir := rel; dir != "."; dir = path.Dir(dir) {
		name := strings.ToLower(path.Base(dir))
		if fsutil.SkipDirName(name) && dir != "outputs/.plans" && dir != "outputs/.drafts" {
			return "", "", errors.New("cannot create document inside excluded or hidden directory")
		}
	}
	return rel, filepath.Join(fsutil.CanonicalPath(root), filepath.FromSlash(rel)), nil
}

func openWorkspaceParent(root *os.Root, rel string) (*os.Root, error) {
	parent, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	dir := path.Dir(rel)
	if dir == "." {
		return parent, nil
	}
	for _, name := range strings.Split(dir, "/") {
		if name == "." || name == "" {
			continue
		}
		if err := parent.Mkdir(name, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			parent.Close()
			return nil, err
		}
		stat, err := parent.Lstat(name)
		if err != nil || !stat.IsDir() {
			parent.Close()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("path %q crosses a symbolic link or non-directory", rel)
		}
		next, err := parent.OpenRoot(name)
		parent.Close()
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(stat, opened) {
			next.Close()
			return nil, fmt.Errorf("directory changed while opening %q", rel)
		}
		parent = next
	}
	return parent, nil
}

func createFileExclusive(parent *os.Root, name string, data []byte) error {
	if _, err := parent.Lstat(name); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp := ".markgogo-" + rand.Text()
	f, err := parent.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer parent.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return parent.Link(tmp, name)
}

func (s *Server) toolCreateDocument(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (any, *rpcError) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Open    bool   `json:"open"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return toolError("invalid arguments: " + err.Error()), nil
	}
	rel, absPath, err := validateCreateRelPath(binding.root, args.Path)
	if err != nil {
		return toolError(err.Error()), nil
	}
	if len(args.Content) > review.MaxResearchArtifactBytes {
		return toolError("document exceeds 8 MiB limit"), nil
	}
	if !utf8.ValidString(args.Content) {
		return toolError("document content must be valid UTF-8"), nil
	}

	rootHandle, err := os.OpenRoot(binding.root)
	if err != nil {
		return toolError(err.Error()), nil
	}
	defer rootHandle.Close()

	parent, err := openWorkspaceParent(rootHandle, rel)
	if err != nil {
		return toolError(err.Error()), nil
	}
	defer parent.Close()

	if err := createFileExclusive(parent, path.Base(rel), []byte(args.Content)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return toolError(fmt.Sprintf("file %q already exists; use propose_edit to modify existing documents", rel)), nil
		}
		return toolError(err.Error()), nil
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

func (s *Server) toolOpenDocument(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (any, *rpcError) {
	var args struct {
		DocumentID string `json:"documentId"`
		Path       string `json:"path"`
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		SectionID  string `json:"sectionId"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return toolError("invalid arguments: " + err.Error()), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolError(err.Error()), nil
	}

	revealOffset := 0
	hasReveal := false
	matchedHeading := ""
	if strings.TrimSpace(args.Heading) != "" || strings.TrimSpace(args.SectionID) != "" {
		sections := workspace.ExtractSections(doc.Text)
		sec, ok := findMatchingSection(sections, strings.TrimSpace(args.SectionID), args.Heading, args.Occurrence)
		if !ok {
			return toolError("requested heading or section was not found in the document"), nil
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
		return toolError(err.Error()), nil
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

func (s *Server) toolWriteGenUIView(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (any, *rpcError) {
	var args struct {
		DocumentID string          `json:"documentId"`
		Path       string          `json:"path"`
		A2UI       json.RawMessage `json:"a2ui"`
		Sources    json.RawMessage `json:"sources"`
		Request    string          `json:"request"`
		Model      string          `json:"model"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return toolError("invalid arguments: " + err.Error()), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolError(err.Error()), nil
	}
	if doc.Path == "" || !fsutil.MarkdownName(doc.Path) || !fsutil.FolderContains(binding.root, doc.Path) {
		return toolError("target document must be a saved Markdown file inside the workspace"), nil
	}
	var messages []json.RawMessage
	if len(args.A2UI) == 0 || json.Unmarshal(args.A2UI, &messages) != nil || len(messages) == 0 {
		return toolError(`"a2ui" must be a non-empty array of A2UI messages`), nil
	}

	parsedSources, err := genui.ParseSources(args.Sources)
	if err != nil {
		return toolError(err.Error()), nil
	}
	sections := workspace.ExtractSections(doc.Text)

	type viewSourceOut struct {
		Heading    string `json:"heading"`
		Occurrence int    `json:"occurrence"`
		Hash       string `json:"hash,omitempty"`
	}
	sourcesOut := make(map[string]viewSourceOut, len(parsedSources))
	for key, src := range parsedSources {
		entry := viewSourceOut{Heading: src.Heading, Occurrence: src.Occurrence}
		if sec, ok := findMatchingSection(sections, "", src.Heading, src.Occurrence); ok {
			entry.Hash = sec.Hash
		}
		sourcesOut[key] = entry
	}

	envelope := struct {
		Version     int                      `json:"version"`
		GeneratedAt string                   `json:"generatedAt"`
		Model       string                   `json:"model,omitempty"`
		Request     string                   `json:"request,omitempty"`
		Sources     map[string]viewSourceOut `json:"sources"`
		A2UI        json.RawMessage          `json:"a2ui"`
	}{
		Version:     genui.ViewVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Model:       strings.TrimSpace(args.Model),
		Request:     strings.TrimSpace(args.Request),
		Sources:     sourcesOut,
		A2UI:        args.A2UI,
	}
	content, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return toolError("failed to encode GenUI view: " + err.Error()), nil
	}
	content = append(content, '\n')
	if len(content) > genui.MaxViewBytes {
		return toolError(fmt.Sprintf("GenUI view exceeds %d byte limit", genui.MaxViewBytes)), nil
	}

	if err := genui.ValidateCandidate(ctx, content); err != nil {
		return toolError(err.Error()), nil
	}

	viewPath := genui.ViewPath(doc.Path)
	if err := genui.ReplaceView(viewPath, content, nil); err != nil {
		return toolError(err.Error()), nil
	}
	relView, _ := filepath.Rel(fsutil.CanonicalPath(binding.root), fsutil.CanonicalPath(viewPath))
	return toolJSON(map[string]any{
		"written":      true,
		"viewPath":     viewPath,
		"relViewPath":  filepath.ToSlash(relView),
		"documentPath": doc.Path,
		"version":      genui.ViewVersion,
	})
}

func (s *Server) toolCreateCheckpoint(ctx context.Context, binding *sessionBinding, rawArgs json.RawMessage) (any, *rpcError) {
	var args struct {
		DocumentID string `json:"documentId"`
		Path       string `json:"path"`
		Label      string `json:"label"`
		Source     string `json:"source"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return toolError("invalid arguments: " + err.Error()), nil
	}
	doc, err := s.resolveTargetDocument(binding.root, args.DocumentID, args.Path)
	if err != nil {
		return toolError(err.Error()), nil
	}
	if doc.Path == "" || !fsutil.MarkdownName(doc.Path) || !fsutil.FolderContains(binding.root, doc.Path) {
		return toolError("checkpoints require a saved Markdown document inside the workspace"), nil
	}
	if doc.Edited || doc.Conflict || doc.Deleted {
		return toolError("save the document and resolve any conflicts before creating a checkpoint"), nil
	}
	source := strings.TrimSpace(args.Source)
	if source == "" {
		source = "mcp"
	}
	var userDataDir string
	if s.deps.Workspace != nil {
		userDataDir = s.deps.Workspace.UserDataDir()
	}
	list, err := review.CreateCheckpoint(ctx, binding.root, doc.Path, doc.Text, strings.TrimSpace(args.Label), source, userDataDir)
	if err != nil {
		return toolError(err.Error()), nil
	}
	var newest review.Checkpoint
	if len(list) > 0 {
		newest = list[0]
	}
	return toolJSON(map[string]any{
		"created":          true,
		"checkpoint":       newest,
		"checkpointsCount": len(list),
		"path":             doc.Path,
		"relPath":          doc.RelPath,
	})
}
