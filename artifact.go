package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"unicode/utf8"
)

type ResearchArtifact struct {
	Path     string `json:"path"`
	Target   string `json:"target"`
	Proposal bool   `json:"proposal"`
	Error    string `json:"error,omitempty"`
}

const researchDocumentCopy = "current-document.md"
const maxResearchArtifactBytes = 8 << 20

var researchArtifactsMu sync.Mutex

func allowedResearchPath(rel string) bool {
	if rel == "." || !fs.ValidPath(rel) || strings.Contains(rel, `\`) || !markdownName(rel) || strings.EqualFold(rel, researchDocumentCopy) {
		return false
	}
	for dir := rel; dir != "."; dir = path.Dir(dir) {
		name := strings.ToLower(path.Base(dir))
		if skipDirName(name) && dir != "outputs/.plans" && dir != "outputs/.drafts" {
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
		if entry.IsDir() && skipDirName(strings.ToLower(entry.Name())) && rel != "outputs/.plans" && rel != "outputs/.drafts" {
			return fs.SkipDir
		}
		if entry.Type().IsRegular() && allowedResearchPath(rel) {
			return fn(rel)
		}
		return nil
	})
}

type artifactRecord struct {
	File   ResearchArtifact `json:"file"`
	Digest string           `json:"digest,omitempty"`
}
type artifactHistory map[string]artifactRecord

type researchBaseline struct {
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
	return writeFileAtomicMode(artifactHistoryPath(work), data, 0600)
}

func prepareResearchArtifacts(work, workspace string) (researchBaseline, error) {
	researchArtifactsMu.Lock()
	defer researchArtifactsMu.Unlock()
	before := researchBaseline{existing: map[string]bool{}, staged: map[string]fs.FileInfo{}}
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
	if stat.Size() > maxResearchArtifactBytes {
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
	data, err := io.ReadAll(io.LimitReader(f, maxResearchArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResearchArtifactBytes || !utf8.Valid(data) {
		return nil, fmt.Errorf("Research file %q exceeds 8 MiB or is not UTF-8 Markdown.", name)
	}
	return data, nil
}

func readWorkingFile(work, rel string) ([]byte, error) {
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

func publishResearchFile(parent *os.Root, name string, data []byte) error {
	tmp := ".markgogo-" + rand.Text()
	f, err := parent.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
	return parent.Link(tmp, name)
}

func finishResearchArtifacts(work, workspace string, before researchBaseline) error {
	researchArtifactsMu.Lock()
	defer researchArtifactsMu.Unlock()
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

func collectResearchArtifacts(work, workspace string) ([]ResearchArtifact, error) {
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
	for rel, record := range history {
		if !allowedResearchPath(rel) {
			continue
		}
		file := record.File
		if file.Error == "" {
			parent, err := artifactParent(root, rel, false)
			if err == nil {
				var st fs.FileInfo
				st, err = parent.Lstat(path.Base(rel))
				parent.Close()
				if err == nil && !st.Mode().IsRegular() {
					err = fmt.Errorf("Research target %q is not a regular file.", rel)
				}
			}
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				file.Error = err.Error()
				file.Target = ""
			} else {
				file.Target = filepath.Join(workspace, filepath.FromSlash(rel))
			}
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func collectResearchFile(work string, dest *os.Root, rel string, known bool) (ResearchArtifact, string, error) {
	var none ResearchArtifact
	data, err := readWorkingFile(work, rel)
	if err != nil {
		return none, "", err
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	parent, err := artifactParent(dest, rel, !known)
	if known && errors.Is(err, fs.ErrNotExist) {
		return none, "", nil
	}
	if err != nil {
		return none, "", err
	}
	defer parent.Close()
	file := ResearchArtifact{Path: rel, Target: filepath.Join(dest.Name(), filepath.FromSlash(rel))}
	name := path.Base(rel)
	stat, err := parent.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		if known {
			return none, "", nil
		}
		err = publishResearchFile(parent, name, data)
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
