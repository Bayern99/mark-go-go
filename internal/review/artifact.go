package review

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

type ResearchArtifact struct {
	Path     string `json:"path"`
	Target   string `json:"target"`
	Proposal bool   `json:"proposal"`
	Missing  bool   `json:"missing,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ResearchFilePublishedEvent struct {
	WorkspaceRoot string           `json:"workspaceRoot"`
	File          ResearchArtifact `json:"file"`
}

const (
	ResearchDocumentCopy     = "current-document.md"
	MaxResearchArtifactBytes = 8 << 20
)

var artifactsMu sync.Mutex
var ResearchFilePublished = mygo.NewEvent[ResearchFilePublishedEvent]("researchFilePublished")

func WorkDir(dataDir, root string) string {
	return filepath.Join(dataDir, "research", fmt.Sprintf("%x", sha256.Sum256([]byte(fsutil.CanonicalPath(root)))), "work")
}

func AllowedPath(rel string) bool {
	if rel == "." || !fs.ValidPath(rel) || strings.Contains(rel, `\`) || !fsutil.MarkdownName(rel) || strings.EqualFold(rel, ResearchDocumentCopy) {
		return false
	}
	for dir := rel; dir != "."; dir = path.Dir(dir) {
		name := strings.ToLower(path.Base(dir))
		if fsutil.SkipDirName(name) && dir != "outputs/.plans" && dir != "outputs/.drafts" {
			return false
		}
	}
	return true
}

func walkResearchMarkdown(dir *os.Root, fn func(string) error) error {
	return fs.WalkDir(dir.FS(), ".", func(rel string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && fsutil.SkipDirName(strings.ToLower(entry.Name())) && rel != "outputs/.plans" && rel != "outputs/.drafts" {
			return fs.SkipDir
		}
		if entry.Type().IsRegular() && AllowedPath(rel) {
			return fn(rel)
		}
		return nil
	})
}

type artifactRecord struct {
	File   ResearchArtifact `json:"file"`
	Digest string           `json:"digest,omitempty"`
	MCP    bool             `json:"mcp,omitempty"`
}
type artifactHistory map[string]artifactRecord

type Baseline struct {
	existing map[string]bool
	staged   map[string]fs.FileInfo
}

func artifactHistoryPath(work string) string {
	return filepath.Join(filepath.Dir(work), "artifacts.json")
}

func readArtifactHistory(work string) (artifactHistory, error) {
	history := artifactHistory{}
	data, err := os.ReadFile(artifactHistoryPath(work))
	if errors.Is(err, fs.ErrNotExist) {
		return history, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	if history == nil {
		return nil, errors.New("Invalid research artifact history.")
	}
	return history, nil
}

func (h artifactHistory) save(work string) error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomicMode(artifactHistoryPath(work), data, 0600)
}

func PrepareArtifacts(work, workspace string) (Baseline, error) {
	artifactsMu.Lock()
	defer artifactsMu.Unlock()
	before := Baseline{existing: map[string]bool{}, staged: map[string]fs.FileInfo{}}
	if err := os.MkdirAll(work, 0700); err != nil {
		return before, err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return before, err
	}
	defer root.Close()
	if err := walkResearchMarkdown(root, func(rel string) error { before.existing[rel] = true; return nil }); err != nil {
		return before, err
	}
	src, err := os.OpenRoot(work)
	if err != nil {
		return before, err
	}
	defer src.Close()
	err = walkResearchMarkdown(src, func(rel string) error {
		info, err := src.Lstat(rel)
		if err == nil {
			before.staged[rel] = info
		}
		return err
	})
	return before, err
}

func artifactParent(root *os.Root, rel string, create bool) (*os.Root, error) {
	parent, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(path.Dir(rel), "/") {
		if name == "." {
			continue
		}
		if create {
			if err := parent.Mkdir(name, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
				parent.Close()
				return nil, err
			}
		}
		stat, err := parent.Lstat(name)
		if err != nil || !stat.IsDir() {
			parent.Close()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("Research path %q crosses a link or a non-directory.", rel)
		}
		next, err := parent.OpenRoot(name)
		parent.Close()
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(stat, opened) {
			next.Close()
			return nil, fmt.Errorf("Research directory changed while opening %q.", rel)
		}
		parent = next
	}
	return parent, nil
}

func readResearchFile(parent *os.Root, name string) ([]byte, error) {
	stat, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("Research file %q is not a regular file.", name)
	}
	if stat.Size() > MaxResearchArtifactBytes {
		return nil, fmt.Errorf("Research file %q exceeds 8 MiB.", name)
	}
	f, err := parent.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(stat, opened) {
		return nil, fmt.Errorf("Research file %q changed while opening.", name)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxResearchArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxResearchArtifactBytes || !utf8.Valid(data) {
		return nil, fmt.Errorf("Research file %q exceeds 8 MiB or is not UTF-8 Markdown.", name)
	}
	return data, nil
}

func ReadWorkingFile(work, rel string) ([]byte, error) {
	root, err := os.OpenRoot(work)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	parent, err := artifactParent(root, rel, false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return readResearchFile(parent, path.Base(rel))
}

func publishResearchFile(parent *os.Root, name string, data []byte, mode fs.FileMode) error {
	tmp := ".markgogo-" + rand.Text()
	f, err := parent.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer parent.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	err = parent.Link(tmp, name)
	if !errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, syscall.EOPNOTSUPP) &&
		!errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.ENOSYS) {
		return err
	}
	target, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	created, statErr := target.Stat()
	_, err = target.Write(data)
	if err == nil {
		err = target.Sync()
	}
	err = errors.Join(err, target.Close())
	if err != nil && statErr == nil {
		if current, statErr := parent.Lstat(name); statErr == nil && os.SameFile(created, current) {
			_ = parent.Remove(name)
		}
	}
	return err
}

func mcpArtifactCopyDir(work string) string {
	return filepath.Join(filepath.Dir(work), "mcp-artifacts")
}

func CreateDocumentArtifact(work, workspace, rel string, data []byte) error {
	if !AllowedPath(rel) || len(data) > MaxResearchArtifactBytes || !utf8.Valid(data) {
		return errors.New("Choose a workspace Markdown path and valid UTF-8 content within 8 MiB.")
	}
	artifactsMu.Lock()
	err := createDocumentArtifactLocked(work, workspace, rel, data)
	artifactsMu.Unlock()
	if err == nil {
		_ = ResearchFilePublished.Broadcast(ResearchFilePublishedEvent{
			WorkspaceRoot: workspace,
			File:          ResearchArtifact{Path: rel, Target: filepath.Join(workspace, filepath.FromSlash(rel))},
		})
	}
	return err
}

func createDocumentArtifactLocked(work, workspace, rel string, data []byte) error {
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	parent, err := artifactParent(root, rel, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	if _, err := parent.Lstat(path.Base(rel)); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	history, err := readArtifactHistory(work)
	if err != nil {
		return err
	}
	copies := mcpArtifactCopyDir(work)
	if err := os.MkdirAll(copies, 0700); err != nil {
		return err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := fsutil.WriteFileAtomicMode(filepath.Join(copies, hash), data, 0600); err != nil {
		return err
	}
	if err := publishResearchFile(parent, path.Base(rel), data, 0644); err != nil {
		return err
	}
	history[rel] = artifactRecord{File: ResearchArtifact{Path: rel}, Digest: hash, MCP: true}
	if err := history.save(work); err != nil {
		return fmt.Errorf("created %q, but could not record it in Review: %w", rel, err)
	}
	return nil
}

func FinishArtifacts(work, workspace string, before Baseline) error {
	artifactsMu.Lock()
	defer artifactsMu.Unlock()
	source, err := os.OpenRoot(work)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	defer dest.Close()
	history, err := readArtifactHistory(work)
	if err != nil {
		return err
	}
	changed := false
	err = walkResearchMarkdown(source, func(rel string) error {
		prior := before.staged[rel]
		info, err := source.Lstat(rel)
		if err != nil {
			return err
		}
		if prior != nil && prior.ModTime() == info.ModTime() && prior.Size() == info.Size() && os.SameFile(prior, info) {
			return nil
		}
		_, known := history[rel]
		file, hash, err := collectResearchFile(work, dest, rel, known || before.existing[rel])
		if err != nil {
			file = ResearchArtifact{Path: rel, Error: err.Error()}
		}
		if file.Path != "" {
			history[rel] = artifactRecord{File: file, Digest: hash}
			changed = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if changed {
		return history.save(work)
	}
	return nil
}

func (r artifactRecord) completedBytes(work, rel string) ([]byte, error) {
	if r.File.Error != "" || r.Digest == "" {
		return nil, errors.New("There is no completed research copy for this file.")
	}
	var data []byte
	var err error
	if r.MCP {
		dir, openErr := os.OpenRoot(mcpArtifactCopyDir(work))
		if openErr != nil {
			return nil, openErr
		}
		defer dir.Close()
		if len(r.Digest) != 64 || strings.ContainsAny(r.Digest, `/\`) {
			return nil, errors.New("The completed research copy has an invalid digest.")
		}
		data, err = readResearchFile(dir, r.Digest)
	} else {
		data, err = ReadWorkingFile(work, rel)
	}
	if err != nil {
		return nil, fmt.Errorf("The completed research copy is unavailable: %w", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != r.Digest {
		return nil, errors.New("The working copy has changed since the completed research turn.")
	}
	return data, nil
}

func inspectResearchArtifact(work string, root *os.Root, rel string, record artifactRecord) ResearchArtifact {
	file := ResearchArtifact{Path: rel, Error: record.File.Error}
	target := filepath.Join(root.Name(), filepath.FromSlash(rel))
	name := path.Base(rel)
	parent, err := artifactParent(root, rel, false)
	if err == nil {
		defer parent.Close()
		var st fs.FileInfo
		st, err = parent.Lstat(name)
		if err == nil && !st.Mode().IsRegular() {
			err = fmt.Errorf("Research target %q is not a regular file.", rel)
		}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		file.Error = err.Error()
		return file
	}
	file.Missing = errors.Is(err, fs.ErrNotExist)
	if !file.Missing {
		file.Target = target
	}
	if file.Error != "" {
		return file
	}
	if !file.Missing {
		if !record.File.Proposal {
			return file
		}
		text, err := readResearchFile(parent, name)
		if err != nil {
			file.Error = err.Error()
			return file
		}
		if fmt.Sprintf("%x", sha256.Sum256(text)) == record.Digest {
			return file
		}
	}
	if _, err := record.completedBytes(work, rel); err != nil {
		file.Error = err.Error()
		return file
	}
	file.Target = target
	file.Proposal = !file.Missing
	return file
}

func CollectArtifacts(work, workspace string) ([]ResearchArtifact, error) {
	artifactsMu.Lock()
	defer artifactsMu.Unlock()
	return collectArtifactsLocked(work, workspace)
}

func collectArtifactsLocked(work, workspace string) ([]ResearchArtifact, error) {
	history, err := readArtifactHistory(work)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := []ResearchArtifact{}
	accepted := false
	for rel, record := range history {
		if !AllowedPath(rel) {
			continue
		}
		file := inspectResearchArtifact(work, root, rel, record)
		if record.File.Proposal && !file.Proposal && !file.Missing && file.Error == "" {
			record.File = file
			history[rel] = record
			accepted = true
		}
		files = append(files, file)
	}
	if accepted {
		if err := history.save(work); err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func ReadArtifactProposal(work, relative string) (string, error) {
	if !AllowedPath(relative) {
		return "", errors.New("Choose a research Markdown file.")
	}
	artifactsMu.Lock()
	defer artifactsMu.Unlock()
	history, err := readArtifactHistory(work)
	if err != nil {
		return "", err
	}
	result, ok := history[relative]
	if !ok || !result.File.Proposal || result.File.Error != "" {
		return "", errors.New("This file has no completed research proposal.")
	}
	data, err := result.completedBytes(work, relative)
	return string(data), err
}

func RecoverArtifact(work, workspace, rel string) (string, error) {
	if !AllowedPath(rel) {
		return "", errors.New("Choose a research Markdown file.")
	}
	artifactsMu.Lock()
	defer artifactsMu.Unlock()
	history, err := readArtifactHistory(work)
	if err != nil {
		return "", err
	}
	record := history[rel]
	data, err := record.completedBytes(work, rel)
	if err != nil {
		return "", err
	}
	dest, err := os.OpenRoot(workspace)
	if err != nil {
		return "", err
	}
	defer dest.Close()
	parent, err := artifactParent(dest, rel, true)
	if err != nil {
		return "", err
	}
	defer parent.Close()
	if err := publishResearchFile(parent, path.Base(rel), data, 0600); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", errors.New("A workspace file already exists at this path.")
		}
		return "", err
	}
	target := filepath.Join(workspace, filepath.FromSlash(rel))
	if record.File.Proposal {
		record.File = ResearchArtifact{Path: rel, Target: target}
		history[rel] = record
		if err := history.save(work); err != nil {
			return "", fmt.Errorf("File restored, but its research record could not be updated: %w", err)
		}
	}
	return target, nil
}

func StageDocument(work, buffer string) error {
	if len(buffer) > MaxResearchArtifactBytes {
		return errors.New("The selected document exceeds 8 MiB.")
	}
	return fsutil.WriteFileAtomicMode(filepath.Join(work, ResearchDocumentCopy), []byte(buffer), 0600)
}

func ClearStagedDocument(work string) error {
	err := os.Remove(filepath.Join(work, ResearchDocumentCopy))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func ReadStagedDocument(work string) (string, error) {
	data, err := ReadWorkingFile(work, ResearchDocumentCopy)
	return string(data), err
}

func collectResearchFile(work string, dest *os.Root, rel string, known bool) (ResearchArtifact, string, error) {
	var none ResearchArtifact
	data, err := ReadWorkingFile(work, rel)
	if err != nil {
		return none, "", err
	}
	digest := sha256.Sum256(data)
	hash := fmt.Sprintf("%x", digest)
	file := ResearchArtifact{Path: rel, Target: filepath.Join(dest.Name(), filepath.FromSlash(rel))}
	parent, err := artifactParent(dest, rel, !known)
	if known && errors.Is(err, fs.ErrNotExist) {
		file.Proposal = true
		return file, hash, nil
	}
	if err != nil {
		return none, "", err
	}
	defer parent.Close()
	name := path.Base(rel)
	stat, err := parent.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		if known {
			file.Proposal = true
			return file, hash, nil
		}
		err = publishResearchFile(parent, name, data, 0600)
		if err == nil {
			return file, hash, nil
		}
		if errors.Is(err, fs.ErrExist) {
			stat, err = parent.Lstat(name)
		}
	}
	if err != nil {
		return none, "", err
	}
	if !stat.Mode().IsRegular() {
		return none, "", fmt.Errorf("Research target %q is not a regular file.", rel)
	}
	current, err := readResearchFile(parent, name)
	if err != nil {
		return none, "", err
	}
	file.Proposal = sha256.Sum256(current) != digest
	return file, hash, nil
}
