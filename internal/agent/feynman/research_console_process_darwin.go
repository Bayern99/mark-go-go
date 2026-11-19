//go:build darwin

package feynman

import (
	"os/exec"
	"syscall"
)

func killResearchConsoleProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
