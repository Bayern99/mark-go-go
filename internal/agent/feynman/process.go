package feynman

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/pirpc"
	"github.com/Bayern99/mark-go-go/internal/review"
)

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
	for _, dir := range []string{expectedWork, filepath.Join(filepath.Dir(expectedWork), "tmp"), filepath.Join(h.profileHome(), ".feynman", "agent")} {
		if fsutil.CanonicalPath(dir) != dir {
			return errors.New("The research data folder contains a redirected path.")
		}
	}
	if fsutil.CanonicalPath(work) != expectedWork {
		return errors.New("Research must run in the application's managed project directory.")
	}
	return nil
}

func (h *ResearchManager) feynmanEnv(cmd *exec.Cmd) []string {
	return feynmanProfileEnv(cmd.Environ(), h.profileHome())
}

func feynmanProfileEnv(env []string, profile string) []string {
	return append(env, "FEYNMAN_HOME="+profile, "PI_CODING_AGENT_DIR="+filepath.Join(profile, ".feynman", "agent"),
		"PI_CODING_AGENT_SESSION_DIR="+filepath.Join(profile, ".feynman", "sessions"))
}

func resolveFeynmanExecutable() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
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
		return "", "", errors.New("Install Feynman CLI, then open Feynman settings to set up the app profile.")
	}
	return feynmanBin, pathEnv, nil
}

func (h *ResearchManager) researchCommand(root, work string) (*exec.Cmd, error) {
	if err := h.checkPrivateWorkspace(root, work); err != nil {
		return nil, err
	}
	feynmanBin, pathEnv, err := resolveFeynmanExecutable()
	if err != nil {
		return nil, err
	}
	canonWork := fsutil.CanonicalPath(work)
	temp := filepath.Join(filepath.Dir(canonWork), "tmp")
	sessions := h.profileSessionsDir()
	if err := h.prepareProfile(canonWork); err != nil {
		return nil, err
	}
	for _, dir := range []string{canonWork, temp, sessions} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command(feynmanBin, "--mode", "rpc", "--session-dir", sessions, "--cwd", canonWork)
	cmd.Dir = canonWork
	cmd.Env = append(h.feynmanEnv(cmd),
		"TMPDIR="+temp,
		pathEnv,
	)
	pirpc.Configure(cmd)
	return cmd, nil
}
