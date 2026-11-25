package pigenui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/pirpc"
)

// authCheckTimeout bounds the read-only credential check.
const authCheckTimeout = 20 * time.Second

// authCheck is the part of `pi auth check --json` this app reads. It never
// carries a credential: the app runs the check without --credentials and keeps
// only the state and the reason.
type authCheck struct {
	Status   string `json:"status"`
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

// Probe answers what can be proven without starting a session that could
// generate: the CLI resolves, and the selected provider's credentials are
// ready. Pi names a provider rather than a model in `auth check`, so until a
// model has been refreshed the login state stays unknown: guessing it from the
// provider's own configuration would claim a login this app has not confirmed.
func Probe(ctx context.Context, cfg Config) (genui.ProbeResult, error) {
	backend, _ := genui.BackendByID(genui.BackendPi)
	result := genui.ProbeResult{Hint: backend.InstallHint}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	resolved, err := resolvePi(cfg.Command, cfg.Env)
	if err != nil {
		var ge *genui.GenError
		if errors.As(err, &ge) {
			result.Status = genui.AgentMissing
			result.Detail = ge.Message
			return result, nil
		}
		result.Status = genui.AgentMissing
		result.Detail = err.Error()
		return result, nil
	}
	result.Executable = resolved
	result.Hint = ""
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		result.Status = genui.AgentUnknown
		result.Detail = "Choose a model first: Pi checks the credentials of the provider that model belongs to."
		return result, nil
	}
	check, err := runAuthCheck(ctx, cfg, resolved, provider)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return result, err
	}
	if err != nil {
		result.Status = genui.AgentFailed
		result.Detail = "`pi auth check` did not answer: " + err.Error()
		result.Hint = "Update the Pi CLI, then choose Test & Refresh."
		return result, nil
	}
	switch check.Status {
	case "ready":
		result.Status = genui.AgentReady
	case "not_ready":
		result.Status = genui.AgentLoginRequired
		result.Detail = "Pi has no usable credentials for " + provider + "."
		result.Hint = "Run `pi auth login " + provider + "` in Terminal, then choose Test & Refresh."
	case "provider_not_found":
		result.Status = genui.AgentFailed
		result.Detail = "Pi does not know the provider " + provider + "."
		result.Hint = "Choose a model from a provider Pi knows, then choose Test & Refresh."
	default:
		result.Status = genui.AgentFailed
		result.Detail = "Pi could not check " + provider + "."
		if check.Reason != "" {
			result.Detail = "Pi could not check " + provider + " (" + strings.ReplaceAll(check.Reason, "_", " ") + ")."
		}
		result.Hint = "Update the Pi CLI, then choose Test & Refresh."
	}
	return result, nil
}

// runAuthCheck asks Pi whether the provider's credentials are ready. The exit
// code is the answer: 0 ready, 1 not_ready, 2 invalid. Credentials are never
// requested or printed.
func runAuthCheck(ctx context.Context, cfg Config, resolved, provider string) (authCheck, error) {
	timeout := authCheckTimeout
	if cfg.Timeout > 0 {
		timeout = min(timeout, cfg.Timeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := pirpc.CommandContext(ctx, resolved, "auth", "check", "--provider", provider, "--json", "--no-refresh")
	cmd.Env = fsutil.WithUserBinPath(cfg.Env)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var check authCheck
	if ctx.Err() != nil {
		return check, ctx.Err()
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &check)
	if err == nil {
		if check.Status == "" {
			check.Status = "ready"
		}
		return check, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitCode() {
		case 1:
			if check.Status == "" {
				check.Status = "not_ready"
			}
			// Pi reports an unknown provider as not_ready; there is nothing to
			// sign in to, so it is not a login-required answer.
			if check.Reason == "provider_not_found" {
				return authCheck{Status: "provider_not_found", Provider: provider, Reason: check.Reason}, nil
			}
			return check, nil
		case 2:
			check.Status = "invalid"
			if check.Reason == "" {
				check.Reason = strings.TrimSpace(stderr.String())
			}
			return check, nil
		}
		return check, fmt.Errorf("exit status %d: %s", exit.ExitCode(), strings.TrimSpace(stderr.String()))
	}
	return check, err
}
