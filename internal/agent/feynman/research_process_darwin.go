package feynman

import (
	"encoding/json"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const researchWriteProfile = `(version 1)
(allow default)
(deny file-write*)
(allow file-write* (subpath (param "WORK")) (subpath (param "TEMP"))
    (subpath (param "FEYNMAN")) (subpath (param "MAP")) (literal "/dev/null"))
(deny file-write* (subpath (param "WORKSPACE")))
`

func researchCommand(root, work string) (*exec.Cmd, error) {
	paths, temp, state, sessions, err := researchRuntime(root, work)
	if err != nil {
		return nil, err
	}
	find := func(name string) string {
		return findExecutable([]string{"PATH=" + strings.Join(paths, string(os.PathListSeparator))}, name)
	}
	feynman, adapter := find("feynman"), find("pi-acp")
	if feynman == "" || adapter == "" {
		return nil, errors.New("Install Feynman CLI and pi-acp, then run feynman setup in Terminal.")
	}
	if find("node") == "" {
		return nil, errors.New("Install Node.js for pi-acp, then reconnect.")
	}
	return researchSandboxCommand(root, work, temp, state, sessions, paths, []string{adapter}, []string{"PI_ACP_PI_COMMAND=" + feynman, "MGO_FEYNMAN_EXECUTABLE=" + feynman})
}

func researchConsoleCommand(root, work string, args []string) (*exec.Cmd, error) {
	paths, temp, state, sessions, err := researchRuntime(root, work)
	if err != nil {
		return nil, err
	}
	feynman := findExecutable([]string{"PATH=" + strings.Join(paths, string(os.PathListSeparator))}, "feynman")
	if feynman == "" {
		return nil, errors.New("Install Feynman CLI, then reconnect the console.")
	}
	if len(args) == 0 {
		args = currentConsoleConversation(work, state, sessions)
	}
	argv := make([]string, 0, len(args)+2)
	argv = append(argv, feynman, "--cwd", fsutil.CanonicalPath(work))
	argv = append(argv, args...)
	return researchSandboxCommand(root, work, temp, state, sessions, paths, argv, nil)
}

func currentConsoleConversation(work, state, adapterDir string) []string {
	record, err := os.ReadFile(filepath.Join(filepath.Dir(work), "session"))
	if err != nil || len(record) == 0 {
		return nil
	}
	var index struct {
		Sessions map[string]struct{ Cwd, SessionFile string }
	}
	data, err := os.ReadFile(filepath.Join(adapterDir, "session-map.json"))
	if err == nil && json.Unmarshal(data, &index) == nil {
		entry, ok := index.Sessions[string(record)]
		if ok && fsutil.SameRoot(entry.Cwd, work) && filepath.Ext(entry.SessionFile) == ".jsonl" && fsutil.FolderContains(filepath.Join(state, "sessions"), entry.SessionFile) {
			if info, err := os.Lstat(entry.SessionFile); err == nil && info.Mode().IsRegular() {
				return []string{"--session", entry.SessionFile}
			}
		}
	}
	return []string{"--new-session"}
}

func researchRuntime(root, work string) (paths []string, temp, state, sessions string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", "", "", err
	}
	dirs := append(filepath.SplitList(os.Getenv("PATH")), filepath.Join(home, ".local/bin"), filepath.Join(home, ".local/share/fnm/aliases/default/bin"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin")
	paths = make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	temp = filepath.Join(filepath.Dir(work), "tmp")
	stateHome := os.Getenv("FEYNMAN_HOME")
	if stateHome == "" {
		stateHome = home
	}
	if stateHome == "~" || strings.HasPrefix(stateHome, "~/") {
		stateHome = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(stateHome, "~"), "/"))
	}
	state = fsutil.CanonicalPath(filepath.Join(stateHome, ".feynman"))
	sessions = fsutil.CanonicalPath(filepath.Join(home, ".pi", "pi-acp"))
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

func researchSandboxCommand(root, work, temp, state, sessions string, paths []string, argv, extraEnv []string) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, errors.New("The Feynman command is empty.")
	}
	sandboxArgs := []string{"-D", "WORK=" + fsutil.CanonicalPath(work), "-D", "TEMP=" + fsutil.CanonicalPath(temp), "-D", "FEYNMAN=" + state, "-D", "MAP=" + sessions, "-D", "WORKSPACE=" + root, "-p", researchWriteProfile}
	sandboxArgs = append(sandboxArgs, argv...)
	cmd := exec.Command("/usr/bin/sandbox-exec", sandboxArgs...)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+state, "TMPDIR="+temp, "PATH="+strings.Join(paths, string(os.PathListSeparator)))
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

func stopResearchProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
