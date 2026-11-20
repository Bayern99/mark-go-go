package feynman

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

var profileTrustMu sync.Mutex

func (h *ResearchManager) profileHome() string {
	return filepath.Join(fsutil.CanonicalPath(h.dataDir), "feynman")
}

func (h *ResearchManager) profileSessionsDir() string {
	return filepath.Join(h.profileHome(), ".feynman", "sessions")
}

func (h *ResearchManager) checkPrivateWorkspace(root, work string) error {
	if h.dataDir == "" {
		return errors.New("The research data folder is unavailable.")
	}
	dataDir := fsutil.CanonicalPath(h.dataDir)
	if fsutil.FolderContains(root, dataDir) || fsutil.FolderContains(dataDir, root) {
		return errors.New("Choose a project folder separate from the application data folder.")
	}
	expectedWork := review.WorkDir(dataDir, root)
	for _, dir := range []string{expectedWork, filepath.Join(filepath.Dir(expectedWork), "tmp"), filepath.Join(h.profileHome(), ".feynman", "agent"), h.profileSessionsDir()} {
		if fsutil.CanonicalPath(dir) != dir {
			return errors.New("The research data folder contains a redirected path.")
		}
	}
	if fsutil.CanonicalPath(work) != expectedWork {
		return errors.New("Research must run in the application's managed project directory.")
	}
	return nil
}

func (h *ResearchManager) trustManagedProject(work string) error {
	dir := filepath.Join(h.profileHome(), ".feynman", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	trustPath := filepath.Join(dir, "trust.json")
	if fsutil.CanonicalPath(trustPath) != trustPath {
		return errors.New("The private Feynman trust file is redirected.")
	}
	profileTrustMu.Lock()
	defer profileTrustMu.Unlock()
	trust := map[string]*bool{}
	data, err := os.ReadFile(trustPath)
	if err == nil {
		if json.Unmarshal(data, &trust) != nil || trust == nil {
			return errors.New("The private Feynman trust file is invalid.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if decision := trust[work]; decision != nil && *decision {
		return nil
	}
	approved := true
	trust[work] = &approved
	data, err = json.MarshalIndent(trust, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomicMode(trustPath, append(data, '\n'), 0600)
}

func (h *ResearchManager) feynmanEnv(cmd *exec.Cmd) []string {
	return feynmanProfileEnv(cmd.Environ(), h.profileHome())
}

func feynmanProfileEnv(env []string, profile string) []string {
	return append(env, "HOME="+profile, "USERPROFILE="+profile, "FEYNMAN_HOME="+profile,
		"PI_CODING_AGENT_SESSION_DIR="+filepath.Join(profile, ".feynman", "sessions"))
}

func (h *ResearchManager) researchCommand(root, work string) (*exec.Cmd, error) {
	if err := h.checkPrivateWorkspace(root, work); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dirs := append(filepath.SplitList(os.Getenv("PATH")), fsutil.DefaultUserBinDirs()...)
	dirs = append(dirs, filepath.Join(home, ".local", "share", "fnm", "aliases", "default", "bin"), "/usr/bin", "/bin")
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	pathEnv := "PATH=" + strings.Join(paths, string(os.PathListSeparator))
	feynmanBin, ok := fsutil.ResolveExecutable("feynman", []string{pathEnv}, nil)
	if !ok {
		return nil, errors.New("Install Feynman CLI, then open Feynman settings to set up the app profile.")
	}
	canonWork := fsutil.CanonicalPath(work)
	temp := filepath.Join(filepath.Dir(canonWork), "tmp")
	sessions := h.profileSessionsDir()
	for _, dir := range []string{canonWork, temp, sessions} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	if err := h.trustManagedProject(canonWork); err != nil {
		return nil, err
	}
	cmd := exec.Command(feynmanBin, "--mode", "rpc", "--session-dir", sessions, "--cwd", canonWork)
	cmd.Dir = canonWork
	cmd.Env = append(h.feynmanEnv(cmd),
		"TMPDIR="+temp,
		pathEnv,
	)
	configureResearchProcess(cmd)
	return cmd, nil
}
