package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func (h *researchHost) Artifacts(ctx context.Context, root string) ([]ResearchArtifact, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return nil, err
	}
	researchArtifactsMu.Lock()
	defer researchArtifactsMu.Unlock()
	h.mu.Lock()
	session := h.sessions[canonicalPath(w.root)]
	busy := session != nil && session.busy
	h.mu.Unlock()
	if busy {
		return nil, errors.New("Feynman is still writing research files.")
	}
	return collectResearchArtifacts(researchWorkDir(h.dataDir, w.root), w.root)
}

func (h *researchHost) ReadArtifact(ctx context.Context, root, relative string) (string, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return "", err
	}
	if !allowedResearchPath(relative) {
		return "", errors.New("Choose a research Markdown file.")
	}
	work := researchWorkDir(h.dataDir, w.root)
	researchArtifactsMu.Lock()
	defer researchArtifactsMu.Unlock()
	history, err := readArtifactHistory(work)
	if err != nil {
		return "", err
	}
	result, ok := history[relative]
	if !ok || !result.File.Proposal || result.File.Error != "" {
		return "", errors.New("This file has no completed research proposal.")
	}
	data, err := readWorkingFile(work, relative)
	if err != nil {
		return "", err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != result.Digest {
		return "", errors.New("The working copy changed after the completed research proposal.")
	}
	return string(data), nil
}

func (h *researchHost) stageDocument(ctx context.Context, root, documentID, buffer string) error {
	work := researchWorkDir(h.dataDir, root)
	name := filepath.Join(work, researchDocumentCopy)
	if documentID == "" {
		err := os.Remove(name)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	h.documents.mu.Lock()
	s := h.documents.byID[documentID]
	valid := s != nil && !s.closed && s.ws == w && s.win == w.win
	if valid {
		state := s.infoLocked()
		valid = !state.ReadOnly && !state.Conflict && !state.Deleted
	}
	h.documents.mu.Unlock()
	if !valid {
		return errors.New("The selected document is no longer editable in this workspace.")
	}
	if len(buffer) > maxResearchArtifactBytes {
		return errors.New("The selected document exceeds 8 MiB.")
	}
	return writeFileAtomicMode(name, []byte(buffer), 0600)
}

func (h *researchHost) readStagedDocument(root string) (string, error) {
	data, err := readWorkingFile(researchWorkDir(h.dataDir, root), researchDocumentCopy)
	return string(data), err
}
