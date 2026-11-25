package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/workspace"
)

const (
	uriWorkspace        = "markgogo://workspace"
	uriDocPrefix        = "markgogo://document/"
	uriDocTemplate      = "markgogo://document/{id}"
	uriSectionsTemplate = "markgogo://document/{id}/sections"
	uriNotesTemplate    = "markgogo://document/{id}/notes"
)

func (s *Server) handleResourcesList(binding *sessionBinding) (any, *rpcError) {
	resources := []mcpResource{
		{
			URI:         uriWorkspace,
			Name:        "workspace",
			Title:       "Workspace",
			Description: "Active workspace root, open document tabs, active document ID, and folder/file listing",
			MimeType:    "application/json",
		},
	}
	if s.deps.Workspace != nil {
		if openDocs, err := s.deps.Workspace.MCPOpenDocuments(binding.root); err == nil {
			for _, doc := range openDocs {
				escaped := url.PathEscape(doc.ID)
				resources = append(resources,
					mcpResource{
						URI:         uriDocPrefix + escaped,
						Name:        doc.Name,
						Title:       doc.Name,
						Description: "Live Markdown document buffer (falls back to disk when closed)",
						MimeType:    "application/json",
					},
					mcpResource{
						URI:         uriDocPrefix + escaped + "/sections",
						Name:        doc.Name + " sections",
						Title:       doc.Name + " sections",
						Description: "Heading outline, line ranges, and FNV-1a section hashes",
						MimeType:    "application/json",
					},
					mcpResource{
						URI:         uriDocPrefix + escaped + "/notes",
						Name:        doc.Name + " notes",
						Title:       doc.Name + " notes",
						Description: "Reader <!-- note: ... --> annotations in the document",
						MimeType:    "application/json",
					},
				)
			}
		}
	}
	return map[string]any{"resources": resources}, nil
}

func (s *Server) handleResourceTemplatesList() any {
	return map[string]any{
		"resourceTemplates": []mcpResourceTemplate{
			{
				URITemplate: uriDocTemplate,
				Name:        "document",
				Title:       "Document",
				Description: "Read live unsaved buffer or saved Markdown document by ID or workspace-relative path",
				MimeType:    "application/json",
			},
			{
				URITemplate: uriSectionsTemplate,
				Name:        "document-sections",
				Title:       "Document Sections",
				Description: "Read Markdown headings and stable FNV-1a section hashes for a document",
				MimeType:    "application/json",
			},
			{
				URITemplate: uriNotesTemplate,
				Name:        "document-notes",
				Title:       "Document Notes",
				Description: "Read reader annotations (<!-- note: ... -->) for a document",
				MimeType:    "application/json",
			},
		},
	}
}

func parseDocumentResourceURI(uri string) (idOrPath string, sub string, ok bool) {
	if !strings.HasPrefix(uri, uriDocPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(uri, uriDocPrefix)
	if rest == "" {
		return "", "", false
	}
	switch {
	case strings.HasSuffix(rest, "/sections"):
		sub = "sections"
		rest = strings.TrimSuffix(rest, "/sections")
	case strings.HasSuffix(rest, "/notes"):
		sub = "notes"
		rest = strings.TrimSuffix(rest, "/notes")
	}
	if rest == "" {
		return "", "", false
	}
	unescaped, err := url.PathUnescape(rest)
	if err != nil || strings.TrimSpace(unescaped) == "" {
		return "", "", false
	}
	return unescaped, sub, true
}

func marshalResourceJSON(uri string, payload any) (any, *rpcError) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: errCodeInternalError, Message: fmt.Sprintf("marshal resource %s: %v", uri, err)}
	}
	return map[string]any{
		"contents": []mcpTextResourceContents{
			{
				URI:      uri,
				MimeType: "application/json",
				Text:     string(raw),
			},
		},
	}, nil
}

func (s *Server) handleResourcesRead(binding *sessionBinding, rawParams json.RawMessage) (any, *rpcError) {
	var params struct {
		URI string `json:"uri"`
	}
	if len(rawParams) == 0 || json.Unmarshal(rawParams, &params) != nil || strings.TrimSpace(params.URI) == "" {
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "uri parameter is required"}
	}
	if s.deps.Workspace == nil {
		return nil, &rpcError{Code: errCodeInternalError, Message: "workspace bridge is unavailable"}
	}

	uri := strings.TrimSpace(params.URI)
	if uri == uriWorkspace {
		state, err := s.deps.Workspace.MCPWorkspaceState(binding.root)
		if err != nil {
			return nil, &rpcError{Code: errCodeResourceNotFound, Message: err.Error()}
		}
		return marshalResourceJSON(uri, state)
	}

	idOrPath, sub, ok := parseDocumentResourceURI(uri)
	if !ok {
		return nil, &rpcError{Code: errCodeResourceNotFound, Message: fmt.Sprintf("unknown resource URI: %s", uri)}
	}
	doc, err := s.deps.Workspace.MCPDocument(binding.root, idOrPath)
	if err != nil {
		return nil, &rpcError{Code: errCodeResourceNotFound, Message: err.Error()}
	}

	switch sub {
	case "":
		return marshalResourceJSON(uri, doc)
	case "sections":
		return marshalResourceJSON(uri, map[string]any{
			"documentId": doc.ID,
			"path":       doc.Path,
			"relPath":    doc.RelPath,
			"open":       doc.Open,
			"version":    doc.Version,
			"sections":   workspace.ExtractSections(doc.Text),
		})
	case "notes":
		return marshalResourceJSON(uri, map[string]any{
			"documentId": doc.ID,
			"path":       doc.Path,
			"relPath":    doc.RelPath,
			"open":       doc.Open,
			"version":    doc.Version,
			"notes":      workspace.ExtractNotes(doc.Text),
		})
	default:
		return nil, &rpcError{Code: errCodeResourceNotFound, Message: fmt.Sprintf("unknown resource URI: %s", uri)}
	}
}
