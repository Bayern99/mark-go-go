//go:build !windows

package feynman

import (
	"os/exec"
	"syscall"
)

func configureResearchProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopResearchProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	_ = cmd.Process.Kill()
}
