package review

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

const (
	checkpointPayloadFormat = "markgogo-checkpoint-v1"
	maxLocalCheckpoints     = 50
)

var (
	lastCheckpointMicro atomic.Int64
	localCheckpointsMu  sync.Mutex
)

type Checkpoint struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	Label     string `json:"label,omitempty"`
	Source    string `json:"source,omitempty"`
	Backend   string `json:"backend,omitempty"`
}

type checkpointPayload struct {
	Format    string `json:"markgogoCheckpoint"`
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	Label     string `json:"label,omitempty"`
	Source    string `json:"source,omitempty"`
	Content   string `json:"content"`
}

type CheckpointDoc struct {
	WorkspaceRoot string
	Path          string
	Content       string
	Edited        bool
	Conflict      bool
	Deleted       bool
}

type DocumentLookup interface {
	CheckpointDocument(ctx context.Context, documentID, path string) (CheckpointDoc, error)
}

type userDataDirLookup interface {
	UserDataDir() string
}

type CheckpointHost struct {
	docs DocumentLookup
}

func NewCheckpointHost(docs DocumentLookup) *CheckpointHost {
	return &CheckpointHost{docs: docs}
}

type checkpointFile struct {
	backend     string
	repo        string
	path        string
	prefix      string
	snapshotDir string
}

func nextCheckpointID() (string, int64) {
	now := time.Now().UnixMicro()
	for {
		prev := lastCheckpointMicro.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastCheckpointMicro.CompareAndSwap(prev, next) {
			return strconv.FormatInt(next, 10), next / 1000
		}
	}
}

func normalizeCheckpointSource(source string) string {
	s := strings.TrimSpace(source)
	if s == "" {
		return "user"
	}
	return s
}

func createdAtFromID(id string) int64 {
	micro, err := strconv.ParseInt(id, 10, 64)
	if err != nil || micro <= 0 {
		return 0
	}
	return micro / 1000
}

func parseCheckpointBlob(id, raw string) (Checkpoint, string) {
	if strings.HasPrefix(raw, "{") {
		var payload checkpointPayload
		if err := json.Unmarshal([]byte(raw), &payload); err == nil && payload.Format == checkpointPayloadFormat &&
			payload.ID == id && payload.CreatedAt == createdAtFromID(id) {
			return Checkpoint{
				ID:        id,
				CreatedAt: payload.CreatedAt,
				Label:     strings.TrimSpace(payload.Label),
				Source:    normalizeCheckpointSource(payload.Source),
				Backend:   "git",
			}, payload.Content
		}
	}
	return Checkpoint{
		ID:        id,
		CreatedAt: createdAtFromID(id),
		Source:    "user",
		Backend:   "git",
	}, raw
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

func hasGitSegment(rel string) bool {
	for part := range strings.SplitSeq(rel, "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}

func openCheckpointFile(ctx context.Context, workspace, path, userDataDir string) (checkpointFile, error) {
	if !fsutil.MarkdownName(path) || !fsutil.FolderContains(workspace, path) {
		return checkpointFile{}, errors.New("Open a saved Markdown document in its workspace.")
	}
	file, err := os.Lstat(path)
	if err != nil || !file.Mode().IsRegular() {
		return checkpointFile{}, errors.New("Checkpoints require a regular Markdown file.")
	}
	canonWorkspace := fsutil.CanonicalPath(workspace)
	canonPath := fsutil.CanonicalPath(path)
	if userDataDir == "" {
		if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
			userDataDir = filepath.Join(cfg, "markgogo")
		} else {
			userDataDir = filepath.Join(os.TempDir(), "markgogo")
		}
	}
	docKey := fmt.Sprintf("%x", sha256.Sum256([]byte(canonPath)))
	snapshotDir := filepath.Join(userDataDir, "checkpoints", docKey)
	if _, err := checkpointGit(ctx, filepath.Dir(path), "", "rev-parse", "--git-dir"); err == nil {
		top, topErr := checkpointGit(ctx, filepath.Dir(path), "", "rev-parse", "--show-toplevel")
		if topErr == nil {
			repo := strings.TrimSuffix(top, "\n")
			rel, relErr := filepath.Rel(fsutil.CanonicalPath(repo), canonPath)
			if relErr == nil && filepath.IsLocal(rel) {
				rel = filepath.ToSlash(rel)
				if hasGitSegment(rel) {
					return checkpointFile{}, errors.New("Git metadata cannot be checkpointed.")
				}
				_, err = checkpointGit(ctx, repo, "", "check-ignore", "--quiet", "--no-index", "--", "./"+rel)
				var exitErr *exec.ExitError
				if err == nil {
					return checkpointFile{}, errors.New("This document is ignored by Git.")
				}
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					return checkpointFile{}, errors.New("Git could not check the document's ignore rules.")
				}
				return checkpointFile{
					backend:     "git",
					repo:        repo,
					path:        rel,
					prefix:      fmt.Sprintf("refs/markgogo/checkpoints/%x/", sha256.Sum256([]byte(rel))),
					snapshotDir: snapshotDir,
				}, nil
			}
		}
	}
	rel, err := filepath.Rel(canonWorkspace, canonPath)
	if err != nil || !filepath.IsLocal(rel) {
		return checkpointFile{}, errors.New("Open a saved Markdown document in its workspace.")
	}
	relSlash := filepath.ToSlash(rel)
	if hasGitSegment(relSlash) {
		return checkpointFile{}, errors.New("Git metadata cannot be checkpointed.")
	}
	return checkpointFile{
		backend:     "local",
		repo:        canonWorkspace,
		path:        relSlash,
		snapshotDir: snapshotDir,
	}, nil
}

func (d checkpointFile) verifySavedText(text string) error {
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
	return nil
}

func (d checkpointFile) localIndexPath() string {
	return filepath.Join(d.snapshotDir, "index.json")
}

func (d checkpointFile) localBlobPath(id string) string {
	return filepath.Join(d.snapshotDir, id+".blob")
}

func (d checkpointFile) readLocalIndexLocked() ([]Checkpoint, error) {
	data, err := os.ReadFile(d.localIndexPath())
	if errors.Is(err, os.ErrNotExist) {
		return []Checkpoint{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Checkpoint
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	result := make([]Checkpoint, 0, len(entries))
	for _, cp := range entries {
		if !validCheckpointID(cp.ID) {
			continue
		}
		if cp.CreatedAt <= 0 {
			cp.CreatedAt = createdAtFromID(cp.ID)
		}
		cp.Label = strings.TrimSpace(cp.Label)
		cp.Source = normalizeCheckpointSource(cp.Source)
		cp.Backend = "local"
		result = append(result, cp)
	}
	return result, nil
}

func (d checkpointFile) createLocal(text, label, source string) (Checkpoint, error) {
	localCheckpointsMu.Lock()
	defer localCheckpointsMu.Unlock()
	if err := os.MkdirAll(d.snapshotDir, 0o700); err != nil {
		return Checkpoint{}, err
	}
	entries, err := d.readLocalIndexLocked()
	if err != nil {
		return Checkpoint{}, err
	}
	id, createdAt := nextCheckpointID()
	if err := fsutil.WriteFileAtomic(d.localBlobPath(id), []byte(text)); err != nil {
		return Checkpoint{}, fmt.Errorf("could not write the checkpoint: %w", err)
	}
	entry := Checkpoint{
		ID:        id,
		CreatedAt: createdAt,
		Label:     strings.TrimSpace(label),
		Source:    normalizeCheckpointSource(source),
		Backend:   "local",
	}
	next := make([]Checkpoint, 0, min(len(entries)+1, maxLocalCheckpoints))
	next = append(next, entry)
	next = append(next, entries...)
	var pruned []Checkpoint
	if len(next) > maxLocalCheckpoints {
		pruned = next[maxLocalCheckpoints:]
		next = next[:maxLocalCheckpoints]
	}
	indexBytes, err := json.Marshal(next)
	if err != nil {
		_ = os.Remove(d.localBlobPath(id))
		return Checkpoint{}, err
	}
	if err := fsutil.WriteFileAtomic(d.localIndexPath(), indexBytes); err != nil {
		_ = os.Remove(d.localBlobPath(id))
		return Checkpoint{}, fmt.Errorf("could not record the checkpoint: %w", err)
	}
	for _, old := range pruned {
		if validCheckpointID(old.ID) {
			_ = os.Remove(d.localBlobPath(old.ID))
		}
	}
	return entry, nil
}

func (d checkpointFile) create(ctx context.Context, text, label, source string) (Checkpoint, error) {
	if err := d.verifySavedText(text); err != nil {
		return Checkpoint{}, err
	}
	if d.backend == "local" {
		return d.createLocal(text, label, source)
	}
	id, createdAt := nextCheckpointID()
	payload := checkpointPayload{
		Format:    checkpointPayloadFormat,
		ID:        id,
		CreatedAt: createdAt,
		Label:     strings.TrimSpace(label),
		Source:    normalizeCheckpointSource(source),
		Content:   text,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Checkpoint{}, err
	}
	object, err := checkpointGit(ctx, d.repo, string(encoded), "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return Checkpoint{}, fmt.Errorf("could not write the checkpoint: %w", err)
	}
	_, err = checkpointGit(ctx, d.repo, "", "update-ref", "--no-deref", d.prefix+id, strings.TrimSpace(object), "")
	if err != nil {
		return Checkpoint{}, fmt.Errorf("could not record the checkpoint: %w", err)
	}
	return Checkpoint{ID: id, CreatedAt: createdAt, Label: payload.Label, Source: payload.Source, Backend: "git"}, nil
}

func (d checkpointFile) list(ctx context.Context) ([]Checkpoint, error) {
	localCheckpointsMu.Lock()
	local, err := d.readLocalIndexLocked()
	localCheckpointsMu.Unlock()
	if err != nil {
		return nil, err
	}
	if d.backend == "local" {
		return local, nil
	}
	out, err := checkpointGit(ctx, d.repo, "", "for-each-ref", "--sort=-refname", "--format=%(refname)", d.prefix)
	if err != nil {
		return nil, fmt.Errorf("could not list checkpoints: %w", err)
	}
	checkpoints := make([]Checkpoint, 0, len(local))
	gitIDs := make(map[string]bool)
	for ref := range strings.Lines(out) {
		id := strings.TrimPrefix(strings.TrimSuffix(ref, "\n"), d.prefix)
		if !validCheckpointID(id) {
			continue
		}
		raw, err := checkpointGit(ctx, d.repo, "", "cat-file", "blob", d.prefix+id)
		if err != nil {
			continue
		}
		cp, _ := parseCheckpointBlob(id, raw)
		checkpoints = append(checkpoints, cp)
		gitIDs[id] = true
	}
	for _, cp := range local {
		if !gitIDs[cp.ID] {
			checkpoints = append(checkpoints, cp)
		}
	}
	slices.SortFunc(checkpoints, func(a, b Checkpoint) int {
		if byLength := cmp.Compare(len(b.ID), len(a.ID)); byLength != 0 {
			return byLength
		}
		return cmp.Compare(b.ID, a.ID)
	})
	return checkpoints, nil
}

func validCheckpointID(id string) bool {
	timeID, err := strconv.ParseInt(id, 10, 64)
	return err == nil && timeID > 0 && strconv.FormatInt(timeID, 10) == id
}

func (d checkpointFile) read(ctx context.Context, id string) (string, error) {
	if !validCheckpointID(id) {
		return "", errors.New("Select a checkpoint of this document.")
	}
	if d.backend == "git" {
		raw, err := checkpointGit(ctx, d.repo, "", "cat-file", "blob", d.prefix+id)
		if err == nil {
			_, content := parseCheckpointBlob(id, raw)
			return content, nil
		}
	}
	localCheckpointsMu.Lock()
	defer localCheckpointsMu.Unlock()
	entries, err := d.readLocalIndexLocked()
	if err == nil && slices.ContainsFunc(entries, func(entry Checkpoint) bool { return entry.ID == id }) {
		if data, err := os.ReadFile(d.localBlobPath(id)); err == nil {
			return string(data), nil
		}
	}
	return "", errors.New("This checkpoint is no longer available.")
}

func CreateCheckpoint(ctx context.Context, workspaceRoot, path, expectedText, label, source, userDataDir string) (Checkpoint, int, error) {
	file, err := openCheckpointFile(ctx, workspaceRoot, path, userDataDir)
	if err != nil {
		return Checkpoint{}, 0, err
	}
	created, err := file.create(ctx, expectedText, label, source)
	if err != nil {
		return Checkpoint{}, 0, err
	}
	list, err := file.list(ctx)
	return created, len(list), err
}

func ListCheckpoints(ctx context.Context, workspaceRoot, path, userDataDir string) ([]Checkpoint, error) {
	file, err := openCheckpointFile(ctx, workspaceRoot, path, userDataDir)
	if err != nil {
		return nil, err
	}
	return file.list(ctx)
}

func ReadCheckpoint(ctx context.Context, workspaceRoot, path, checkpointID, userDataDir string) (string, error) {
	file, err := openCheckpointFile(ctx, workspaceRoot, path, userDataDir)
	if err != nil {
		return "", err
	}
	return file.read(ctx, checkpointID)
}

func (h *CheckpointHost) userDataDir() string {
	if u, ok := h.docs.(userDataDirLookup); ok {
		return u.UserDataDir()
	}
	return ""
}

func (h *CheckpointHost) document(ctx context.Context, id, path string) (CheckpointDoc, checkpointFile, error) {
	if h.docs == nil {
		return CheckpointDoc{}, checkpointFile{}, errors.New("Document lookup is unavailable.")
	}
	doc, err := h.docs.CheckpointDocument(ctx, id, path)
	if err != nil {
		return CheckpointDoc{}, checkpointFile{}, err
	}
	file, err := openCheckpointFile(ctx, doc.WorkspaceRoot, path, h.userDataDir())
	return doc, file, err
}

func (h *CheckpointHost) List(ctx context.Context, documentID, path string) ([]Checkpoint, error) {
	_, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return nil, err
	}
	return file.list(ctx)
}

func (h *CheckpointHost) Create(ctx context.Context, documentID, path, expectedText, label, source string) ([]Checkpoint, error) {
	doc, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return nil, err
	}
	if doc.Path != path || doc.Edited || doc.Conflict || doc.Deleted || doc.Content != expectedText {
		return nil, errors.New("Save the current document and resolve any conflicts before creating a checkpoint.")
	}
	if _, err := file.create(ctx, expectedText, label, source); err != nil {
		return nil, err
	}
	return file.list(ctx)
}

func (h *CheckpointHost) Read(ctx context.Context, documentID, path, checkpointID string) (string, error) {
	_, file, err := h.document(ctx, documentID, path)
	if err != nil {
		return "", err
	}
	return file.read(ctx, checkpointID)
}
