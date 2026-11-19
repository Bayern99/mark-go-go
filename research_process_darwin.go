package main

import (
	"errors"
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
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dirs := append(filepath.SplitList(os.Getenv("PATH")), filepath.Join(home, ".local/bin"), filepath.Join(home, ".local/share/fnm/aliases/default/bin"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin")
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	find := func(name string) string {
		for _, dir := range paths {
			p := filepath.Join(dir, name)
			if fi, e := os.Stat(p); e == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
				return p
			}
		}
		return ""
	}
	feynman, adapter := find("feynman"), find("pi-acp")
	if feynman == "" || adapter == "" {
		return nil, errors.New("Install Feynman CLI and pi-acp, then run feynman setup in Terminal.")
	}
	if find("node") == "" {
		return nil, errors.New("Install Node.js for pi-acp, then reconnect.")
	}
	temp := filepath.Join(filepath.Dir(work), "tmp")
	stateHome := os.Getenv("FEYNMAN_HOME")
	if stateHome == "" {
		stateHome = home
	}
	if stateHome == "~" || strings.HasPrefix(stateHome, "~/") {
		stateHome = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(stateHome, "~"), "/"))
	}
	state := canonicalPath(filepath.Join(stateHome, ".feynman"))
	sessions := canonicalPath(filepath.Join(home, ".pi", "pi-acp"))
	for _, dir := range []string{work, temp, state, sessions} {
		if folderContains(root, dir) || folderContains(dir, root) {
			return nil, errors.New("Choose a project folder separate from the application and Feynman data folders.")
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command("/usr/bin/sandbox-exec", "-D", "WORK="+canonicalPath(work), "-D", "TEMP="+canonicalPath(temp), "-D", "FEYNMAN="+state, "-D", "MAP="+sessions, "-D", "WORKSPACE="+root, "-p", researchWriteProfile, adapter)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "PI_ACP_PI_COMMAND="+feynman, "TMPDIR="+temp, "PATH="+strings.Join(paths, string(os.PathListSeparator)))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

func stopResearchProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
