//go:build windows

package pirpc

import "os/exec"

// Configure has nothing to prepare on Windows: a child process is not put in a
// group of its own here, so Stop ends the program itself.
func Configure(_ *exec.Cmd) {}

// Stop ends the program.
func Stop(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
