//go:build !darwin

package main

import "os/exec"

func killResearchConsoleProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
