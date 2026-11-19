package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

type checkpointHost struct {
	documents *documentHost
}

type checkpointFile struct {
	repo   string
	path   string
	prefix string
}

func checkpointGit(ctx context.Context, dir, input string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)...)
	cmd.Dir = dir
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GIT_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	return string(out), err
}

func openCheckpointFile(ctx context.Context, workspace, path string) (checkpointFile, error) {
	if !markdownName(path) || !folderContains(workspace, path) {
		return checkpointFile{}, errors.New("Open a saved Markdown document in its workspace.")
	}
	file, err := os.Lstat(path)
	if err != nil || !file.Mode().IsRegular() {
		return checkpointFile{}, errors.New("Checkpoints require a regular Markdown file.")
	}
	top, err := checkpointGit(ctx, filepath.Dir(path), "", "rev-parse", "--show-toplevel")
	if err != nil {
		return checkpointFile{}, errors.New("Open a document in an existing Git repository.")
	}
	repo := strings.TrimSuffix(top, "\n")
	rel, err := filepath.Rel(canonicalPath(repo), canonicalPath(path))
	if err != nil || !filepath.IsLocal(rel) {
		return checkpointFile{}, errors.New("The document is outside the Git repository.")
	}
	rel = filepath.ToSlash(rel)
	for part := range strings.SplitSeq(rel, "/") {
		if strings.EqualFold(part, ".git") {
			return checkpointFile{}, errors.New("Git metadata cannot be checkpointed.")
		}
	}
	_, err = checkpointGit(ctx, repo, "", "check-ignore", "--quiet", "--no-index", "--", "./"+rel)
	var exitErr *exec.ExitError
	if err == nil {
		return checkpointFile{}, errors.New("This document is ignored by Git.")
	}
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return checkpointFile{}, errors.New("Git could not check the document's ignore rules.")
	}
	return checkpointFile{repo, rel, fmt.Sprintf("refs/markgogo/checkpoints/%x/", sha256.Sum256([]byte(rel)))}, nil
}

func (d checkpointFile) create(ctx context.Context, text string) error {
	root, err := os.OpenRoot(d.repo)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(d.path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("The document was moved or replaced.")
	}
	f, err := root.Open(d.path)
	if err != nil {
		return err
	}
	defer f.Close()
	current, err := f.Stat()
	if err != nil || !os.SameFile(info, current) {
		return errors.New("The document was moved or replaced.")
	}
	saved, err := io.ReadAll(io.LimitReader(f, int64(len(text))+1))
	if err != nil {
		return err
	}
	if string(saved) != text {
		return errors.New("The document changed. Save or resolve it before creating a checkpoint.")
	}
	object, err := checkpointGit(ctx, d.repo, text, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return fmt.Errorf("could not write the checkpoint: %w", err)
	}
	id := strconv.FormatInt(time.Now().UnixMicro(), 10)
	_, err = checkpointGit(ctx, d.repo, "", "update-ref", "--no-deref", d.prefix+id, strings.TrimSpace(object), "")
	if err != nil {
		return fmt.Errorf("could not record the checkpoint: %w", err)
	}
	return nil
}

func (d checkpointFile) list(ctx context.Context) ([]string, error) {
	out, err := checkpointGit(ctx, d.repo, "", "for-each-ref", "--sort=-refname", "--format=%(refname)", d.prefix)
	if err != nil {
		return nil, fmt.Errorf("could not list checkpoints: %w", err)
	}
	ids := []string{}
	for ref := range strings.Lines(out) {
		id := strings.TrimPrefix(strings.TrimSuffix(ref, "\n"), d.prefix)
		if validCheckpointID(id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func validCheckpointID(id string) bool {
	timeID, err := strconv.ParseInt(id, 10, 64)
	return err == nil && timeID > 0 && strconv.FormatInt(timeID, 10) == id
}

func (d checkpointFile) read(ctx context.Context, id string) (string, error) {
	if !validCheckpointID(id) {
		return "", errors.New("Select a checkpoint of this document.")
	}
	text, err := checkpointGit(ctx, d.repo, "", "cat-file", "blob", d.prefix+id)
	if err != nil {
		return "", errors.New("This checkpoint is no longer available.")
	}
	return text, nil
}

func (h *checkpointHost) document(ctx context.Context, id, path string) (*docSession, checkpointFile, error) {
	win := mygo.CallerWindow(ctx)
	h.documents.mu.Lock()
	s := h.documents.byID[id]
	w := h.documents.byWindow[win]
	if win == nil || s == nil || s.closed || s.doc == nil || w == nil || s.ws != w || w.active != id {
		h.documents.mu.Unlock()
		return nil, checkpointFile{}, errNoDocument
	}
	root := w.root
	h.documents.mu.Unlock()
	if path == "" || s.currentPath() != path {
		return nil, checkpointFile{}, errors.New("Select the saved document again.")
	}
	file, err := openCheckpointFile(ctx, root, path)
	return s, file, err
}

func (h *checkpointHost) List(ctx context.Context, documentID, path string) ([]string, error) {
	_, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return nil, err
	}
	return file.list(ctx)
}

func (h *checkpointHost) Create(ctx context.Context, documentID, path, expectedText string) ([]string, error) {
	doc, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return nil, err
	}
	state := doc.documentState()
	if state.Path != path || state.Edited || state.Conflict || state.Deleted || doc.doc.Content() != expectedText {
		return nil, errors.New("Save the current document and resolve any conflicts before creating a checkpoint.")
	}
	if err := file.create(ctx, expectedText); err != nil {
		return nil, err
	}
	return file.list(ctx)
}

func (h *checkpointHost) Read(ctx context.Context, documentID, path, checkpointID string) (string, error) {
	_, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return "", err
	}
	return file.read(ctx, checkpointID)
}
