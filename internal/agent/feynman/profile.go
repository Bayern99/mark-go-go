package feynman

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

var profileMu sync.Mutex

func (h *ResearchManager) prepareProfile(work string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	source := filepath.Join(fsutil.CanonicalPath(home), ".feynman")
	profile := filepath.Join(h.profileHome(), ".feynman")
	agent := filepath.Join(profile, "agent")
	profileMu.Lock()
	defer profileMu.Unlock()
	if fsutil.FolderContains(source, profile) || fsutil.FolderContains(profile, source) {
		return errors.New("The private Feynman profile overlaps the user profile.")
	}
	if fsutil.CanonicalPath(agent) != agent {
		return errors.New("The private Feynman agent directory is redirected.")
	}
	if err := os.MkdirAll(agent, 0700); err != nil {
		return err
	}
	if err := h.writeProfileTrust(agent, home, work); err != nil {
		return err
	}
	if err := linkProfileEntries(source, profile, "agent"); err != nil {
		return err
	}
	return linkProfileEntries(filepath.Join(source, "agent"), agent, "trust.json", "trust.json.lock")
}

func linkProfileEntries(source, dest string, exclusions ...string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		skip := false
		for _, excluded := range exclusions {
			if strings.EqualFold(name, excluded) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		target, link := filepath.Join(source, name), filepath.Join(dest, name)
		if _, err := os.Lstat(link); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		info, err := os.Stat(target)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			continue
		}
		if err := os.Symlink(target, link); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("link Feynman profile entry %q: %w", name, err)
		}
	}
	return nil
}

func (h *ResearchManager) writeProfileTrust(agent, home, work string) error {
	path := filepath.Join(agent, "trust.json")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("The private Feynman trust file is redirected.")
	}
	research := filepath.Join(fsutil.CanonicalPath(h.dataDir), "research")
	prior := map[string]bool{}
	before, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(before, &prior) != nil || prior == nil {
			return errors.New("The private Feynman trust file is invalid.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	decisions := map[string]bool{}
	for _, dir := range []string{research, fsutil.CanonicalPath(home)} {
		for parent := filepath.Dir(dir); ; parent = filepath.Dir(parent) {
			if filepath.Dir(parent) == parent {
				decisions[parent] = false
				break
			}
		}
	}
	if runtime.GOOS == "windows" {
		for letter := 'A'; letter <= 'Z'; letter++ {
			decisions[string(letter)+`:\`] = false
		}
	}
	for dir, trusted := range prior {
		parent := filepath.Dir(dir)
		hash := filepath.Base(parent)
		if trusted && filepath.Base(dir) == "work" && filepath.Dir(parent) == research &&
			len(hash) == 64 && fsutil.CanonicalPath(dir) == dir {
			if _, err := hex.DecodeString(hash); err == nil {
				decisions[dir] = true
			}
		}
	}
	if work != "" {
		decisions[work] = true
	}
	after, err := json.MarshalIndent(decisions, "", "  ")
	if err != nil {
		return err
	}
	after = append(after, '\n')
	if bytes.Equal(before, after) {
		return nil
	}
	return fsutil.WriteFileAtomicMode(path, after, 0600)
}
