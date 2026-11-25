// Package pigenui is the GenUI backend that drives Pi's own RPC mode, the way
// Feynman is driven over RPC: no ACP adapter in between. It hands the shared
// JSONL transport a process it prepared itself and turns Pi's events into the
// progress the reader sees.
//
// Pi has no sandbox and does not ask before a tool call, so every generation
// loads the guard extension in this package and refuses to run without proof
// that the guard is active. Each turn starts its own process in its own folder
// and ends it again: a Pi session cannot change its working folder, and the
// guard's allowed paths are fixed for the life of the process, so reuse would
// trade isolation for one process start.
package pigenui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
)

const (
	// GuardVersion is the guard protocol this app knows. It must match
	// GUARD_VERSION in guard/guard.js, and a guard that reports another version
	// stops the run instead of generating unguarded.
	GuardVersion = "1"

	// guardStatusKey is the status entry the guard extension reports itself
	// through before any prompt is sent.
	guardStatusKey = "mgo-guard"

	defaultGuardGrace = 15 * time.Second
	defaultAbortGrace = 3 * time.Second

	// toolAllowlist limits Pi to the three file tools. Plain names make this a
	// complete allowlist rather than an addition to the defaults, so no shell
	// is ever declared to the model.
	toolAllowlist = "read,write,edit"
)

// Config is everything one backend launch needs. The provider is recorded
// beside the model because Pi names a provider in its auth check, its
// set_model and its Terminal login.
type Config struct {
	// Command is the executable that speaks Pi's RPC mode, "pi" unless the
	// reader set another path.
	Command  string
	Provider string
	Model    string
	// ThoughtLevel is the reasoning level to ask for. Pi keeps the model's own
	// default when the level is empty or the model does not offer it.
	ThoughtLevel string

	// Env replaces the process environment. Tests set it; production leaves it
	// nil and the program inherits this app's environment.
	Env []string

	Timeout     time.Duration
	Now         func() time.Time
	Validate    func(context.Context, []byte) error
	BeforeWrite func() error
	GuardGrace  time.Duration
	AbortGrace  time.Duration
}

func (c Config) guardGrace() time.Duration {
	if c.GuardGrace > 0 {
		return c.GuardGrace
	}
	return defaultGuardGrace
}

func (c Config) abortGrace() time.Duration {
	if c.AbortGrace > 0 {
		return c.AbortGrace
	}
	return defaultAbortGrace
}

// RunGeneration runs one GenUI generation on Pi. It is the Pi counterpart of an
// ACP generation: the orchestrator owns the folder, the prompt, the preview,
// validation and the commit, and this only supplies the runner.
func RunGeneration(ctx context.Context, cfg Config, req genui.GenerateRequest, viewPath string, progress func(genui.GenerateProgress) error) error {
	runner := &Runner{cfg: cfg}
	orchestrator := &genui.Orchestrator{
		Runner:       runner,
		Req:          req,
		ViewPath:     viewPath,
		Model:        cfg.Model,
		ThoughtLevel: cfg.ThoughtLevel,
		Now:          cfg.Now,
		Timeout:      cfg.Timeout,
		Validate:     cfg.Validate,
		BeforeWrite:  cfg.BeforeWrite,
	}
	err := orchestrator.Run(ctx, progress)
	if closeErr := runner.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

// launchArgs are the arguments this app chooses for a generation. Pi 1.6.0 has
// no --cwd, so the turn's folder is the process's working folder instead.
//
// --no-extensions still loads an explicit -e, so the guard is the only
// extension; --no-approve leaves project-local resources unloaded; --no-mcp and
// --no-context-files keep servers and instruction files out; --no-session keeps
// the run out of the reader's Pi history.
func launchArgs(guardPath string) []string {
	return []string{
		"--mode", "rpc",
		"--no-session",
		"--no-extensions",
		"-e", guardPath,
		"--no-approve",
		"--no-mcp",
		"--no-context-files",
		"--tools", toolAllowlist,
	}
}

// guardEnv tells the guard extension which paths this turn may touch. The paths
// never change while the process lives, and the token proves the handshake came
// from the guard this app loaded for this turn.
func guardEnv(root, token string) []string {
	return []string{
		"MGO_GUARD_ROOT=" + root,
		"MGO_GUARD_OUT=" + filepath.Join(root, "out.json"),
		"MGO_GUARD_TOKEN=" + token,
		"MGO_GUARD_VERSION=" + GuardVersion,
	}
}

func resolvePi(command string, env []string) (string, error) {
	if resolved, ok := fsutil.ResolveExecutable(command, fsutil.WithUserBinPath(env), nil); ok {
		return resolved, nil
	}
	return "", newStartFailure(command, errors.New("the command was not found"))
}

func newStartFailure(command string, err error) *genui.GenError {
	failure := genui.NewFailure(genui.FailAgentNotFound,
		fmt.Sprintf("Could not start Pi with the command %q: %v.", command, err),
		"Install the Pi CLI so `pi` runs in Terminal, or set its full path in Agent settings.")
	return failure
}

func newToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("token-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// piCommand prepares the process for one turn: this app's own arguments, the
// turn's folder, and the guard's environment.
func piCommand(cfg Config, dir, guardPath, token string) (*exec.Cmd, error) {
	resolved, err := resolvePi(cfg.Command, cfg.Env)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(resolved, launchArgs(guardPath)...)
	cmd.Dir = dir
	cmd.Env = append(fsutil.WithUserBinPath(cfg.Env), guardEnv(dir, token)...)
	return cmd, nil
}

// The Pi backend is one of the GenUI runners the orchestrator drives.
var _ genui.Runner = (*Runner)(nil)
