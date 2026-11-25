package pirpc

import (
	"context"
	"os/exec"
	"time"
)

func CommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, executable, args...)
	Configure(cmd)
	cmd.Cancel = func() error {
		Stop(cmd)
		return nil
	}
	cmd.WaitDelay = 250 * time.Millisecond
	return cmd
}
