//go:build windows

package feynman

import "os/exec"

func configureResearchProcess(_ *exec.Cmd) {}

func stopResearchProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
