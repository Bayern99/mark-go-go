package feynman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

const (
	defaultCLITimeout = 60 * time.Second
	longCLITimeout    = 120 * time.Second
)

func feynmanCLICommand(ctx context.Context, work string, args ...string) (*exec.Cmd, error) {
	feynmanBin, pathEnv, err := resolveFeynmanExecutable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, feynmanBin, args...)
	if work != "" {
		if err := os.MkdirAll(work, 0700); err == nil {
			cmd.Dir = fsutil.CanonicalPath(work)
		}
	}
	cmd.Env = append(os.Environ(),
		pathEnv,
		"NO_COLOR=1",
		"TERM=dumb",
	)
	configureResearchProcess(cmd)
	cmd.Cancel = func() error {
		stopResearchProcess(cmd)
		return nil
	}
	return cmd, nil
}

func cleanCLIOutput(raw []byte) string {
	s := ansiEscapePattern.ReplaceAllString(string(raw), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

func formatCLICommand(args ...string) string {
	if len(args) == 0 {
		return "feynman"
	}
	return "feynman " + strings.Join(args, " ")
}

func (h *ResearchManager) cliWorkDir(root string) string {
	if h.dataDir == "" || root == "" || !filepath.IsAbs(root) {
		return ""
	}
	work := review.WorkDir(h.dataDir, fsutil.CanonicalPath(root))
	if err := os.MkdirAll(work, 0700); err != nil {
		return ""
	}
	return work
}

func (h *ResearchManager) runCLI(ctx context.Context, root string, timeout time.Duration, args ...string) (raw string, exitCode int, err error) {
	if _, wsErr := h.workspace(ctx, root); wsErr != nil {
		return "", 0, wsErr
	}
	if err := h.checkPrivateWorkspace(root, review.WorkDir(h.dataDir, root)); err != nil {
		return "", 0, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	h.mu.Lock()
	factory := h.cliCommand
	h.mu.Unlock()
	if factory == nil {
		factory = feynmanCLICommand
	}
	if err := h.prepareProfile(""); err != nil {
		return "", 0, err
	}
	cmd, err := factory(ctx, h.cliWorkDir(root), args...)
	if err != nil {
		return "", 0, err
	}
	cmd.Env = h.feynmanEnv(cmd)
	out, runErr := cmd.CombinedOutput()
	raw = cleanCLIOutput(out)
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return raw, exitErr.ExitCode(), nil
		}
		return raw, -1, runErr
	}
	return raw, 0, nil
}

func firstNonZeroError(exitCode int, raw string) string {
	if exitCode == 0 {
		return ""
	}
	if raw != "" {
		return raw
	}
	return fmt.Sprintf("Command exited with status %d", exitCode)
}

func commandResult(command, raw string, exitCode int) CLICommandResult {
	return CLICommandResult{
		Command:   command,
		Parsed:    exitCode == 0 && raw != "",
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		Summary:   firstLine(raw),
	}
}

func (h *ResearchManager) runCLICommand(ctx context.Context, root string, timeout time.Duration, args ...string) (CLICommandResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, timeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}
