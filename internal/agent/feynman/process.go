package feynman

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

func expandHomePath(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(home, p[2:])
	}
	return p
}

func resolveFeynmanDirs() (stateDir, sessionsDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	stateHome := strings.TrimSpace(os.Getenv("FEYNMAN_HOME"))
	if stateHome == "" {
		stateHome = home
	} else {
		stateHome = expandHomePath(stateHome, home)
	}
	stateDir = fsutil.CanonicalPath(filepath.Join(stateHome, ".feynman"))
	if envSessions := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_SESSION_DIR")); envSessions != "" {
		sessionsDir = fsutil.CanonicalPath(expandHomePath(envSessions, home))
	} else {
		sessionsDir = fsutil.CanonicalPath(filepath.Join(stateDir, "sessions"))
	}
	return stateDir, sessionsDir, nil
}

func researchRuntime(root, work string) (paths []string, temp, state, sessions string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", "", "", err
	}
	state, sessions, err = resolveFeynmanDirs()
	if err != nil {
		return nil, "", "", "", err
	}
	dirs := append(filepath.SplitList(os.Getenv("PATH")), fsutil.DefaultUserBinDirs()...)
	dirs = append(dirs, filepath.Join(home, ".local", "share", "fnm", "aliases", "default", "bin"), "/usr/bin", "/bin")
	paths = make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	temp = filepath.Join(filepath.Dir(work), "tmp")
	for _, dir := range []string{work, temp, state, sessions} {
		if fsutil.FolderContains(root, dir) || fsutil.FolderContains(dir, root) {
			return nil, "", "", "", errors.New("Choose a project folder separate from the application and Feynman data folders.")
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, "", "", "", err
		}
	}
	return paths, temp, state, sessions, nil
}

func researchCommand(root, work string) (*exec.Cmd, error) {
	paths, temp, _, sessions, err := researchRuntime(root, work)
	if err != nil {
		return nil, err
	}
	pathEnv := "PATH=" + strings.Join(paths, string(os.PathListSeparator))
	feynmanBin := findExecutable([]string{pathEnv}, "feynman")
	if feynmanBin == "" {
		return nil, errors.New("Install Feynman CLI, then run feynman setup in Terminal.")
	}
	canonWork := fsutil.CanonicalPath(work)
	cmd := exec.Command(feynmanBin, "--mode", "rpc", "--session-dir", sessions, "--cwd", canonWork)
	cmd.Dir = canonWork
	cmd.Env = append(os.Environ(),
		"TMPDIR="+fsutil.CanonicalPath(temp),
		pathEnv,
	)
	configureResearchProcess(cmd)
	return cmd, nil
}
