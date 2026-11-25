//go:build !windows

package pirpc

import (
	"os/exec"
	"syscall"
)

// Configure puts the program in its own process group, so stopping it also
// stops whatever it started.
func Configure(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Stop kills the program's whole process group. A CLI that runs helpers as
// children leaves them holding the pipes open otherwise.
func Stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	_ = cmd.Process.Kill()
}
