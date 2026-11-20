package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

const maxMCPDocumentBytes = 8 << 20

type WorkspaceFileEntry struct {
	RelPath    string `json:"relPath"`
	Name       string `json:"name"`
	Title      string `json:"title,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"`
	Notes      int    `json:"notes,omitempty"`
	Open       bool   `json:"open,omitempty"`
	DocumentID string `json:"documentId,omitempty"`
}

type WorkspaceMCPState struct {
	Root             string               `json:"root"`
	Name             string               `json:"name"`
	ActiveDocumentID string               `json:"activeDocumentId,omitempty"`
	OpenDocuments    []DocumentInfo       `json:"openDocuments"`
	Folders          []string             `json:"folders"`
	Files            []WorkspaceFileEntry `json:"files"`
}

type DocumentMCPState struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	RelPath       string `json:"relPath"`
	Open          bool   `json:"open"`
	Edited        bool   `json:"edited"`
	Conflict      bool   `json:"conflict"`
	Deleted       bool   `json:"deleted"`
	ReadOnly      bool   `json:"readOnly"`
	Version       int64  `json:"version"`
	EditorVersion int    `json:"-"`
	Text          string `json:"text"`
}

type DocReveal struct {
	ID     string `json:"id"`
	Path   string `json:"path,omitempty"`
	Offset int    `json:"offset"`
}

var DocumentRevealed = desktop.Event[DocReveal]{}

func (h *Service) workspaceByRoot(root string) *workspace {
	canon := fsutil.CanonicalPath(root)
	if canon == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, w := range h.allWorkspacesLocked() {
		if w != nil && w.root == canon {
			return w
		}
	}
	return nil
}

func (h *Service) openSessionsForWorkspace(root string) ([]*docSession, string) {
	canon := fsutil.CanonicalPath(root)
	h.mu.Lock()
	defer h.mu.Unlock()
	var sessions []*docSession
	var activeID string
	for _, w := range h.allWorkspacesLocked() {
		if w == nil || w.root == "" || w.root != canon {
			continue
		}
		if activeID == "" {
			activeID = w.active
		}
		for _, id := range w.tabs {
			if s := h.byID[id]; s != nil && !s.closed {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, activeID
}

func resolveWorkspaceRel(root, idOrPath string) (relSlash string, absPath string, err error) {
	trimmed := strings.TrimSpace(idOrPath)
	if trimmed == "" {
		return "", "", errors.New("document id or path is required")
	}
	canonRoot := fsutil.CanonicalPath(root)
	var candidate string
	if filepath.IsAbs(trimmed) {
		candidate = fsutil.CanonicalPath(trimmed)
	} else {
		cleaned := filepath.Clean(filepath.FromSlash(trimmed))
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return "", "", errors.New("path escapes workspace root")
		}
		candidate = fsutil.CanonicalPath(filepath.Join(canonRoot, cleaned))
	}
	if !fsutil.FolderContains(canonRoot, candidate) {
		return "", "", errors.New("path is outside workspace root")
	}
	rel, err := filepath.Rel(canonRoot, candidate)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", "", errors.New("invalid workspace-relative path")
	}
	relSlash = filepath.ToSlash(rel)
	for _, part := range strings.Split(relSlash, "/") {
		if part == "" || part == "." || part == ".." || fsutil.SkipDirName(strings.ToLower(part)) {
			return "", "", errors.New("path is in an excluded directory")
		}
	}
	return relSlash, candidate, nil
}

func readMarkdownFromRoot(canonRoot, relSlash string) (string, error) {
	r, err := os.OpenRoot(canonRoot)
	if err != nil {
		return "", err
	}
	defer r.Close()
	info, err := r.Lstat(relSlash)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("target is not a regular file")
	}
	if info.Size() > maxMCPDocumentBytes {
		return "", errors.New("document exceeds 8 MiB limit")
	}
	f, err := r.Open(relSlash)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMCPDocumentBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxMCPDocumentBytes || !utf8.Valid(data) {
		return "", errors.New("document is not valid UTF-8 or exceeds 8 MiB")
	}
	return string(data), nil
}

func (h *Service) MCPWorkspaceState(root string) (WorkspaceMCPState, error) {
	canonRoot := fsutil.CanonicalPath(root)
	if canonRoot == "" {
		return WorkspaceMCPState{}, errNoWorkspace
	}
	info, err := os.Stat(canonRoot)
	if err != nil || !info.IsDir() {
		return WorkspaceMCPState{}, errNoWorkspace
	}

	sessions, activeID := h.openSessionsForWorkspace(canonRoot)
	openDocs := make([]DocumentInfo, 0, len(sessions))
	openByRel := make(map[string]DocumentInfo, len(sessions))
	for _, s := range sessions {
		di := s.info()
		openDocs = append(openDocs, di)
		if di.Path != "" {
			if rel := relSlash(canonRoot, di.Path); rel != "" {
				openByRel[rel] = di
			}
		}
	}

	folderSet := map[string]bool{}
	fileEntries := make([]WorkspaceFileEntry, 0)

	osRoot, err := os.OpenRoot(canonRoot)
	if err != nil {
		return WorkspaceMCPState{}, err
	}
	defer osRoot.Close()

	err = fs.WalkDir(osRoot.FS(), ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if fsutil.SkipDirName(strings.ToLower(entry.Name())) {
				return fs.SkipDir
			}
			folderSet[filepath.ToSlash(rel)] = true
			return nil
		}
		if !entry.Type().IsRegular() || !fsutil.MarkdownName(entry.Name()) {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		fe := WorkspaceFileEntry{
			RelPath: relSlash,
			Name:    entry.Name(),
		}
		if di, ok := openByRel[relSlash]; ok {
			fe.Open = true
			fe.DocumentID = di.ID
		}
		fileEntries = append(fileEntries, fe)
		return nil
	})
	if err != nil {
		return WorkspaceMCPState{}, err
	}

	folders := make([]string, 0, len(folderSet))
	for f := range folderSet {
		folders = append(folders, f)
	}
	sort.Strings(folders)
	sort.Slice(fileEntries, func(i, j int) bool { return fileEntries[i].RelPath < fileEntries[j].RelPath })

	return WorkspaceMCPState{
		Root:             canonRoot,
		Name:             filepath.Base(canonRoot),
		ActiveDocumentID: activeID,
		OpenDocuments:    openDocs,
		Folders:          folders,
		Files:            fileEntries,
	}, nil
}

func (s *docSession) mcpDocument(root string) DocumentMCPState {
	s.host.mu.Lock()
	text := s.currentText()
	switch {
	case s.mcpVersion == 0:
		s.mcpVersion = max(s.version, 1)
	case text != s.mcpText:
		s.mcpVersion++
	}
	s.mcpText = text
	version := s.mcpVersion
	editorVersion := s.version
	s.host.mu.Unlock()
	info := s.info()
	return DocumentMCPState{
		ID: info.ID, Name: info.Name, Path: info.Path, RelPath: relSlash(root, info.Path),
		Open: true, Edited: info.Edited, Conflict: info.Conflict, Deleted: info.Deleted,
		ReadOnly: info.ReadOnly, Version: int64(version), EditorVersion: editorVersion, Text: text,
	}
}

func (h *Service) MCPDocument(root, idOrPath string) (DocumentMCPState, error) {
	canonRoot := fsutil.CanonicalPath(root)
	if canonRoot == "" {
		return DocumentMCPState{}, errNoWorkspace
	}
	sessions, _ := h.openSessionsForWorkspace(canonRoot)
	for _, s := range sessions {
		if s.id == idOrPath {
			return s.mcpDocument(canonRoot), nil
		}
	}

	rel, absPath, err := resolveWorkspaceRel(canonRoot, idOrPath)
	if err != nil {
		return DocumentMCPState{}, err
	}
	for _, s := range sessions {
		if p := s.currentPath(); p != "" && fsutil.SameFile(p, absPath) {
			return s.mcpDocument(canonRoot), nil
		}
	}

	if !fsutil.MarkdownName(rel) {
		return DocumentMCPState{}, errors.New("target is not a Markdown document")
	}
	text, err := readMarkdownFromRoot(canonRoot, rel)
	if err != nil {
		return DocumentMCPState{}, err
	}
	digest := sha256.Sum256([]byte(text))
	const diskVersionHashBits = 52
	diskVersion := int64(1<<diskVersionHashBits | binary.BigEndian.Uint64(digest[:8])>>(64-diskVersionHashBits))
	return DocumentMCPState{
		Name:    filepath.Base(absPath),
		Path:    absPath,
		RelPath: rel,
		Open:    false,
		Version: diskVersion,
		Text:    text,
	}, nil
}

func (h *Service) MCPOpenDocument(ctx context.Context, root, idOrPath string, revealOffset int, hasReveal bool) (DocumentInfo, error) {
	canonRoot := fsutil.CanonicalPath(root)
	if canonRoot == "" {
		return DocumentInfo{}, errNoWorkspace
	}
	sessions, _ := h.openSessionsForWorkspace(canonRoot)
	var targetSession *docSession
	for _, s := range sessions {
		if s.id == idOrPath {
			targetSession = s
			break
		}
	}
	if targetSession == nil {
		rel, absPath, err := resolveWorkspaceRel(canonRoot, idOrPath)
		if err != nil {
			return DocumentInfo{}, err
		}
		if !fsutil.MarkdownName(rel) {
			return DocumentInfo{}, errors.New("target is not a Markdown document")
		}
		for _, s := range sessions {
			if p := s.currentPath(); p != "" && fsutil.SameFile(p, absPath) {
				targetSession = s
				break
			}
		}
		if targetSession == nil {
			if _, err := readMarkdownFromRoot(canonRoot, rel); err != nil {
				return DocumentInfo{}, err
			}
			if h.docs == nil {
				return DocumentInfo{}, errors.New("document host is unavailable")
			}
			info, err := h.Open(ctx, absPath)
			if err != nil {
				return DocumentInfo{}, err
			}
			if hasReveal {
				if s, sErr := h.docSession(info.ID); sErr == nil {
					if win := h.sessionWindow(s); win != nil && !win.IsDestroyed() {
						_ = DocumentRevealed.Emit(win, DocReveal{ID: info.ID, Path: info.Path, Offset: revealOffset})
					}
				}
			}
			return info, nil
		}
	}

	h.activate(targetSession)
	if win := h.sessionWindow(targetSession); win != nil && !win.IsDestroyed() {
		win.Show()
		win.Focus()
		if hasReveal {
			_ = DocumentRevealed.Emit(win, DocReveal{ID: targetSession.id, Path: targetSession.currentPath(), Offset: revealOffset})
		}
	}
	return targetSession.info(), nil
}
