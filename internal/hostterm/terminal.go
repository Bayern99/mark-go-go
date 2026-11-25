// Package hostterm opens a command in the user's own terminal. It carries no
// profile and no environment of its own: the login an app asks for is the login
// the user already uses.
package hostterm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

// Command builds an external command; tests replace it.
type Command func(ctx context.Context, name string, args ...string) *exec.Cmd

var createTemp = func(pattern string) (*os.File, error) { return os.CreateTemp("", pattern) }

// CommandLine joins an executable and its arguments into one shell line.
func CommandLine(bin string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quote(bin))
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`;&|<>()*?[]{}#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Open starts commandLine in the system Terminal. commandLine is a single line
// the user can also run by hand; the error says so when no terminal is found.
func Open(ctx context.Context, commandLine string, command Command) error {
	if command == nil {
		command = exec.CommandContext
	}
	return open(ctx, runtime.GOOS, os.Getenv, commandLine, command)
}

func open(ctx context.Context, goos string, getenv func(string) string, commandLine string, command Command) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch goos {
	case "darwin":
		return openMacTerminal(ctx, commandLine, command)
	case "windows":
		cmd := command(ctx, "cmd.exe", "/c", "start", "", "cmd.exe", "/k", commandLine)
		return start(cmd)
	default:
		if term := strings.TrimSpace(getenv("TERMINAL")); term != "" {
			if err := start(command(ctx, term, "-e", "sh", "-lc", commandLine)); err == nil {
				return nil
			}
		}
		for _, candidate := range []struct {
			bin  string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", "sh", "-lc", commandLine}},
			{"gnome-terminal", []string{"--", "sh", "-lc", commandLine}},
			{"konsole", []string{"-e", "sh", "-lc", commandLine}},
			{"xterm", []string{"-e", "sh", "-lc", commandLine}},
		} {
			if path, ok := fsutil.ResolveExecutable(candidate.bin, nil, nil); ok {
				return start(command(ctx, path, candidate.args...))
			}
		}
		return fmt.Errorf("No terminal found. Run this in your terminal: %s", commandLine)
	}
}

func openMacTerminal(ctx context.Context, commandLine string, command Command) error {
	file, err := createTemp("mark-go-go-login-*.command")
	if err != nil {
		return err
	}
	path := file.Name()
	defer file.Close()
	opened := false
	defer func() {
		if !opened {
			_ = os.Remove(path)
		}
	}()
	if _, err := fmt.Fprintf(file, "#!/bin/sh\n/bin/rm -f -- %s\n%s\n", quote(path), commandLine); err != nil {
		return err
	}
	if err := file.Chmod(0o700); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	cmd := command(ctx, "open", "-b", "com.apple.Terminal", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("open Terminal: %s", msg)
		}
		return err
	}
	opened = true
	return nil
}

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
