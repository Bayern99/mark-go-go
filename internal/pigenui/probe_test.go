package pigenui

import (
	"context"
	"errors"
	"os"

	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func TestProbeReadsTheExitCodeOfTheCredentialCheck(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		exit       string
		reason     string
		want       genui.AgentStatus
		wantDetail string
	}{
		{name: "ready", status: "ready", exit: "0", want: genui.AgentReady},
		{name: "needs a sign-in", status: "not_ready", exit: "1", reason: "credentials_not_configured", want: genui.AgentLoginRequired, wantDetail: "no usable credentials"},
		{name: "invalid", exit: "2", want: genui.AgentFailed, wantDetail: "could not check"},
		{name: "an unknown provider", status: "not_ready", exit: "1", reason: "provider_not_found", want: genui.AgentFailed, wantDetail: "does not know the provider"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "fixture.log")
			cfg := testConfig(t, logPath)
			cfg.Env = append(cfg.Env, "MGO_FIXTURE_AUTH_STATUS="+tt.status, "MGO_FIXTURE_AUTH_EXIT="+tt.exit)
			if tt.reason != "" {
				cfg.Env = append(cfg.Env, "MGO_FIXTURE_AUTH_REASON="+tt.reason)
			}
			result, err := Probe(context.Background(), cfg)
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if result.Status != tt.want {
				t.Fatalf("status = %q (%s), want %q", result.Status, result.Detail, tt.want)
			}
			if tt.want == genui.AgentReady && result.Hint != "" {
				t.Errorf("installed and ready Pi still displays %q", result.Hint)
			}
			if tt.wantDetail != "" && !strings.Contains(result.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to mention %q", result.Detail, tt.wantDetail)
			}
			if tt.want == genui.AgentLoginRequired && !strings.Contains(result.Hint, "pi auth login anthropic") {
				t.Errorf("hint = %q, want the sign-in command for the provider", result.Hint)
			}
			starts := fixtureStarts(fixtureLog(t, logPath))
			if len(starts) != 1 {
				t.Fatalf("ran %d commands, want one credential check", len(starts))
			}
			args := fixtureStrings(starts[0]["args"])
			if !slices.Contains(args, "--no-refresh") {
				t.Errorf("args = %v, want the check not to refresh credentials", args)
			}
			if slices.Contains(args, "--credentials") {
				t.Errorf("args = %v, want no request for a credential", args)
			}
			if !slices.Contains(args, "anthropic") {
				t.Errorf("args = %v, want the selected provider", args)
			}
		})
	}
}

func TestProbeWithoutAProviderReportsAnUnknownLogin(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Provider = ""
	result, err := Probe(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentUnknown {
		t.Fatalf("status = %q, want an unknown login state rather than a claim", result.Status)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("the probe ran a command without a provider: %v", err)
	}
}

func TestProbeDoesNotStartAnAuthCheckAfterCancellation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Probe(ctx, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("probe with cancelled context = %v", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("a cancelled probe started Pi: %v", err)
	}
}
