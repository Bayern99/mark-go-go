package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

const (
	toolReadDocument     = "read_document"
	toolGetWorkspace     = "get_workspace"
	toolProposeEdit      = "propose_edit"
	toolCreateDocument   = "create_document"
	toolOpenDocument     = "open_document"
	toolWriteGenUIView   = "write_genui_view"
	toolCreateCheckpoint = "create_checkpoint"
)

// auditTools are the tools that change what the user sees or has on disk.
var auditTools = map[string]bool{
	toolProposeEdit:      true,
	toolCreateDocument:   true,
	toolWriteGenUIView:   true,
	toolCreateCheckpoint: true,
}

func readOnlyAnnotations() map[string]any {
	return map[string]any{
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
}

func writeAnnotations(idempotent bool) map[string]any {
	return map[string]any{
		"readOnlyHint":    false,
		"destructiveHint": false,
		"idempotentHint":  idempotent,
		"openWorldHint":   false,
	}
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func arrayOf(items map[string]any, description string) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": items}
}

func documentTargetProps() map[string]any {
	return map[string]any{
		"documentId": stringProp("Open document id from the workspace resource"),
		"path":       stringProp("Workspace-relative or absolute path of a Markdown document"),
	}
}

func documentOutputSchema() map[string]any {
	return objectSchema(map[string]any{
		"documentId": stringProp("Open document id, empty for a document that is only on disk"),
		"path":       stringProp("Absolute path of the document"),
		"relPath":    stringProp("Workspace-relative path of the document"),
		"version":    intProp("Document version, to pass back as baseVersion"),
		"open":       boolProp("Whether the editor has the document open"),
		"edited":     boolProp("Whether the open buffer has unsaved edits"),
		"conflict":   boolProp("Whether the open buffer conflicts with the file on disk"),
		"readOnly":   boolProp("Whether the document cannot be edited"),
		"startLine":  intProp("1-based line the returned text starts on"),
		"endLine":    intProp("1-based line the returned text ends on"),
		"text":       stringProp("The requested Markdown text"),
		"truncated":  boolProp("Whether the request was capped before the whole range"),
		"heading":    stringProp("Heading of the section that was read, when one was selected"),
		"sectionId":  stringProp("Id of the section that was read, when one was selected"),
		"hash":       stringProp("Hash of the section that was read, when one was selected"),
	}, "documentId", "path", "relPath", "version", "open", "edited", "conflict", "readOnly", "startLine", "endLine", "text", "truncated")
}

func (s *Server) handleToolsList() any {
	return map[string]any{"tools": []mcpTool{
		{
			Name:        toolReadDocument,
			Title:       "Read Document",
			Description: "Read a workspace Markdown document by id or path, wholly or as one heading section or line range. Returns the version a line-range propose_edit needs.",
			InputSchema: objectSchema(map[string]any{
				"documentId": stringProp("Open document id from the workspace resource"),
				"path":       stringProp("Workspace-relative or absolute path of a saved Markdown document"),
				"sectionId":  stringProp("Section id or hash from the sections resource"),
				"heading":    stringProp("Heading text of the section to read"),
				"occurrence": intProp("1-based occurrence of the heading (default 1)"),
				"startLine":  intProp("1-based first line of the range to read"),
				"endLine":    intProp("1-based last line of the range to read, inclusive"),
			}),
			OutputSchema: documentOutputSchema(),
			Annotations:  readOnlyAnnotations(),
		},
		{
			Name:        toolGetWorkspace,
			Title:       "Get Workspace",
			Description: "List the workspace root, its open documents, and up to " + fmt.Sprint(workspace.MaxWorkspaceListingFiles) + " Markdown files with a truncated flag.",
			InputSchema: objectSchema(map[string]any{}),
			OutputSchema: objectSchema(map[string]any{
				"root":             stringProp("Absolute workspace root"),
				"name":             stringProp("Folder name of the workspace"),
				"activeDocumentId": stringProp("Id of the document the editor shows, when one is active"),
				"openDocuments": arrayOf(objectSchema(map[string]any{
					"documentId": stringProp("Open document id"),
					"name":       stringProp("File name"),
					"path":       stringProp("Absolute path"),
					"relPath":    stringProp("Workspace-relative path"),
					"edited":     boolProp("Whether the buffer has unsaved edits"),
					"conflict":   boolProp("Whether the buffer conflicts with the file on disk"),
					"readOnly":   boolProp("Whether the document cannot be edited"),
				}, "documentId", "name", "path", "relPath", "edited", "conflict", "readOnly"), "Open documents"),
				"folders": arrayOf(map[string]any{"type": "string"}, "Workspace-relative folder paths"),
				"files": arrayOf(objectSchema(map[string]any{
					"relPath":    stringProp("Workspace-relative path"),
					"name":       stringProp("File name"),
					"open":       boolProp("Whether the editor has the file open"),
					"documentId": stringProp("Open document id, empty when the file is closed"),
				}, "relPath", "name", "open"), "Markdown files"),
				"truncated":  boolProp("Whether more files exist than the listing carries"),
				"filesLimit": intProp("Number of files a full listing may carry"),
			}, "root", "name", "openDocuments", "folders", "files", "truncated", "filesLimit"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        toolProposeEdit,
			Title:       "Propose Document Edit",
			Description: "Queue a Change proposal for an existing Markdown document. The user reviews it in Mark Go Go before any text changes; pass exactly one of proposedText, oldText with newText, or startLine with endLine and newText.",
			InputSchema: objectSchema(map[string]any{
				"documentId":   stringProp("Open document id from the workspace resource"),
				"path":         stringProp("Workspace-relative or absolute path of the Markdown document"),
				"summary":      stringProp("One sentence about the change, for the reviewer"),
				"proposedText": stringProp("Complete proposed Markdown text"),
				"oldText":      stringProp("Exact existing snippet to replace with newText"),
				"newText":      stringProp("Replacement text for oldText or for startLine..endLine"),
				"startLine":    intProp("1-based first line of the range to replace"),
				"endLine":      intProp("1-based last line of the range to replace, inclusive"),
				"baseVersion":  intProp("Document version from read_document, required for a line-range edit"),
			}),
			OutputSchema: objectSchema(map[string]any{
				"proposalId": stringProp("Id of the queued proposal"),
				"revision":   intProp("Revision of the workspace's proposal list"),
				"hunksCount": intProp("Number of change hunks the user reviews"),
				"conflicted": boolProp("Whether the proposal conflicts with the current text"),
				"summary":    stringProp("The summary the reviewer sees"),
			}, "proposalId", "revision", "hunksCount", "conflicted", "summary"),
			Annotations: writeAnnotations(false),
		},
		{
			Name:        toolCreateDocument,
			Title:       "Create Markdown Document",
			Description: "Create a new Markdown document inside the workspace, refusing to overwrite an existing file.",
			InputSchema: objectSchema(map[string]any{
				"path":    stringProp("Workspace-relative Markdown file path (e.g. outputs/summary.md)"),
				"content": stringProp("UTF-8 Markdown content to write"),
				"open":    boolProp("Whether to open the created document in the workspace window"),
			}, "path", "content"),
			OutputSchema: objectSchema(map[string]any{
				"created":    boolProp("Whether the document was created"),
				"path":       stringProp("Absolute path of the created document"),
				"relPath":    stringProp("Workspace-relative path of the created document"),
				"bytes":      intProp("Number of bytes written"),
				"documentId": stringProp("Open document id, empty when the document was not opened"),
			}, "created", "path", "relPath", "bytes", "documentId"),
			Annotations: writeAnnotations(false),
		},
		{
			Name:        toolOpenDocument,
			Title:       "Open Document",
			Description: "Open or focus a workspace Markdown document and optionally scroll to a heading or section.",
			InputSchema: objectSchema(map[string]any{
				"documentId": stringProp("Open document id"),
				"path":       stringProp("Workspace-relative or absolute path of the Markdown document"),
				"heading":    stringProp("Heading text to scroll to"),
				"occurrence": intProp("1-based heading occurrence (default 1)"),
				"sectionId":  stringProp("Section id or hash from the sections resource"),
			}),
			OutputSchema: objectSchema(map[string]any{
				"opened":     boolProp("Whether the document is open and focused"),
				"documentId": stringProp("Open document id"),
				"name":       stringProp("File name"),
				"path":       stringProp("Absolute path"),
				"relPath":    stringProp("Workspace-relative path"),
				"heading":    stringProp("Heading that was scrolled to, when one was requested"),
				"offset":     intProp("Character offset that was revealed"),
			}, "opened", "documentId", "name", "path", "relPath", "heading", "offset"),
			Annotations: writeAnnotations(true),
		},
		{
			Name:        toolWriteGenUIView,
			Title:       "Write GenUI View",
			Description: "Validate an A2UI 1.0 or openui-lang candidate against the GenUI catalog and atomically write the document's .genui.json view file. Returns the version actually written.",
			InputSchema: objectSchema(map[string]any{
				"documentId": stringProp("Open document id"),
				"path":       stringProp("Workspace-relative or absolute path of the source Markdown document"),
				"a2ui":       arrayOf(map[string]any{}, "Non-empty array of A2UI 1.0 messages"),
				"sources":    map[string]any{"type": "object", "description": "Source citation map keyed by source id -> {heading, occurrence}"},
				"request":    stringProp("Optional generation request description"),
				"model":      stringProp("Optional model identifier"),
			}, "a2ui"),
			OutputSchema: objectSchema(map[string]any{
				"written":      boolProp("Whether the view file was written"),
				"documentPath": stringProp("Absolute path of the source document"),
				"viewPath":     stringProp("Absolute path of the view file"),
				"relViewPath":  stringProp("Workspace-relative path of the view file"),
				"version":      intProp("View version that was written, 2 or 3"),
			}, "written", "documentPath", "viewPath", "relViewPath", "version"),
			Annotations: writeAnnotations(false),
		},
		{
			Name:        toolCreateCheckpoint,
			Title:       "Create Checkpoint",
			Description: "Record a labeled provenance checkpoint of a saved Markdown document, so the user can compare or restore it later.",
			InputSchema: objectSchema(map[string]any{
				"documentId": stringProp("Open document id"),
				"path":       stringProp("Workspace-relative or absolute path of the saved Markdown document"),
				"label":      stringProp("Human-readable checkpoint label"),
			}),
			OutputSchema: objectSchema(map[string]any{
				"created": boolProp("Whether the checkpoint was created"),
				"checkpoint": objectSchema(map[string]any{
					"id":        stringProp("Id of the checkpoint"),
					"createdAt": intProp("Unix seconds when the checkpoint was created"),
					"label":     stringProp("Label of the checkpoint"),
					"source":    stringProp("Channel the checkpoint came from"),
					"backend":   stringProp("Storage backend the checkpoint uses"),
				}, "id", "createdAt"),
				"checkpointsCount": intProp("Number of checkpoints the document now has"),
				"path":             stringProp("Absolute path of the document"),
				"relPath":          stringProp("Workspace-relative path of the document"),
			}, "created", "checkpoint", "checkpointsCount", "path", "relPath"),
			Annotations: writeAnnotations(false),
		},
	}}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func (s *Server) handleToolsCall(ctx context.Context, binding *sessionBinding, rawParams json.RawMessage) (any, *rpcError) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(rawParams) == 0 || json.Unmarshal(rawParams, &params) != nil || strings.TrimSpace(params.Name) == "" {
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "tool name is required"}
	}

	var result mcpCallToolResult
	var rpcErr *rpcError
	target := ""
	switch params.Name {
	case toolReadDocument:
		result, rpcErr = s.toolReadDocument(binding, params.Arguments)
	case toolGetWorkspace:
		result, rpcErr = s.toolGetWorkspace(binding)
	case toolProposeEdit:
		result, rpcErr = s.toolProposeEdit(binding, params.Arguments)
		target = targetPathOf(params.Arguments)
	case toolCreateDocument:
		result, rpcErr = s.toolCreateDocument(ctx, binding, params.Arguments)
		target = targetPathOf(params.Arguments)
	case toolOpenDocument:
		result, rpcErr = s.toolOpenDocument(ctx, binding, params.Arguments)
		target = targetPathOf(params.Arguments)
	case toolWriteGenUIView:
		result, rpcErr = s.toolWriteGenUIView(ctx, binding, params.Arguments)
		target = targetPathOf(params.Arguments)
	case toolCreateCheckpoint:
		result, rpcErr = s.toolCreateCheckpoint(ctx, binding, params.Arguments)
		target = targetPathOf(params.Arguments)
	default:
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "unknown tool: " + params.Name}
	}
	if rpcErr != nil {
		return nil, rpcErr
	}
	if auditTools[params.Name] {
		s.audit(binding, params.Name, target, result)
	}
	return result, nil
}

// targetPathOf reports what a tool call acted on, for the audit record.
func targetPathOf(raw json.RawMessage) string {
	var args struct {
		Path       string `json:"path"`
		DocumentID string `json:"documentId"`
	}
	_ = json.Unmarshal(raw, &args)
	return firstNonEmpty(args.Path, args.DocumentID)
}

// appErrorOf reports the app error a tool result carries, if any.
func appErrorOf(result mcpCallToolResult) *appError {
	if !result.IsError {
		return nil
	}
	if failure, ok := result.StructuredContent.(*appError); ok && failure != nil {
		return failure
	}
	return &appError{Code: ipc.CodeInvalidArgs, Message: "The tool failed."}
}
