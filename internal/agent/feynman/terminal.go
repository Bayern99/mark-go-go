package feynman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

const terminalLaunchTimeout = 15 * time.Second

func (h *ResearchManager) LaunchTerminalWizard(ctx context.Context, root, wizard, target string) (CLITerminalLaunchResult, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return CLITerminalLaunchResult{}, err
	}
	if err := h.checkPrivateWorkspace(root, review.WorkDir(h.dataDir, root)); err != nil {
		return CLITerminalLaunchResult{}, err
	}
	h.ensureFocusWatch(w)

	wizard = strings.TrimSpace(wizard)
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "-") {
		return CLITerminalLaunchResult{}, errors.New("Invalid provider or target identifier.")
	}

	var args []string
	switch wizard {
	case "model_login":
		args = []string{"model", "login"}
		if target != "" {
			args = append(args, target)
		}
	case "alpha_login":
		args = []string{"alpha", "login"}
	case "setup":
		args = []string{"setup"}
	default:
		return CLITerminalLaunchResult{}, fmt.Errorf("Unsupported interactive wizard %q.", wizard)
	}

	if err := h.prepareProfile(""); err != nil {
		return CLITerminalLaunchResult{}, err
	}
	command := shellJoinCommand("feynman", args...)
	h.mu.Lock()
	launcher := h.openTerminal
	h.mu.Unlock()
	if launcher == nil {
		launcher = openSystemTerminal
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, terminalLaunchTimeout)
		defer cancel()
	}
	if err := launcher(ctx, command, h.profileHome()); err != nil {
		return CLITerminalLaunchResult{Wizard: wizard, Command: command, Launched: false}, err
	}
	return CLITerminalLaunchResult{Wizard: wizard, Command: command, Launched: true}, nil
}

func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' || r == ':') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func shellJoinCommand(bin string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuoteArg(bin))
	for _, a := range args {
		parts = append(parts, shellQuoteArg(a))
	}
	return strings.Join(parts, " ")
}

func openSystemTerminal(ctx context.Context, command, profile string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sessions := filepath.Join(profile, ".feynman", "sessions")
	launchCmd := "FEYNMAN_HOME=" + shellQuoteArg(profile) +
		" PI_CODING_AGENT_DIR=" + shellQuoteArg(filepath.Join(profile, ".feynman", "agent")) +
		" PI_CODING_AGENT_SESSION_DIR=" + shellQuoteArg(sessions) + " " + command
	if runtime.GOOS != "windows" && strings.HasPrefix(command, "feynman") {
		if bin, _, err := resolveFeynmanExecutable(); err == nil {
			launchCmd = fmt.Sprintf("PATH=%s:\"$PATH\" %s", shellQuoteArg(filepath.Dir(bin)), launchCmd)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		file, err := os.CreateTemp("", "mark-go-go-feynman-*.command")
		if err != nil {
			return err
		}
		defer file.Close()
		path := file.Name()
		opened := false
		defer func() {
			if !opened {
				_ = os.Remove(path)
			}
		}()
		if _, err := fmt.Fprintf(file, "#!/bin/sh\n/bin/rm -f -- %s\n%s\n", shellQuoteArg(path), launchCmd); err != nil {
			return err
		}
		if err := file.Chmod(0700); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "open", "-b", "com.apple.Terminal", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			msg := strings.TrimSpace(string(out))
			if msg != "" {
				return fmt.Errorf("open Terminal: %s", msg)
			}
			return err
		}
		opened = true
		return nil
	case "windows":
		cmd := exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", command)
		cmd.Env = feynmanProfileEnv(cmd.Environ(), profile)
		return startTerminal(cmd)
	default:
		if term := strings.TrimSpace(os.Getenv("TERMINAL")); term != "" {
			if err := startTerminal(exec.Command(term, "-e", "sh", "-lc", launchCmd)); err == nil {
				return nil
			}
		}
		for _, candidate := range []struct {
			bin  string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", "sh", "-lc", launchCmd}},
			{"gnome-terminal", []string{"--", "sh", "-lc", launchCmd}},
			{"konsole", []string{"-e", "sh", "-lc", launchCmd}},
			{"xterm", []string{"-e", "sh", "-lc", launchCmd}},
		} {
			if path, ok := fsutil.ResolveExecutable(candidate.bin, nil, nil); ok {
				return startTerminal(exec.Command(path, candidate.args...))
			}
		}
		return errors.New("No supported terminal emulator found. Run the command in your terminal: " + launchCmd)
	}
}

func startTerminal(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
