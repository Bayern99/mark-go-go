package workspace

import (
	"context"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func TestUsableModelLeavesOutAChoiceTheBackendNoLongerOffers(t *testing.T) {
	for _, tt := range []struct {
		name    string
		backend GenUIBackend
		want    string
	}{
		{name: "declared", backend: GenUIBackend{Model: "gpt-6-sol", Models: []string{"gpt-6-sol", "gpt-6-luna"}}, want: "gpt-6-sol"},
		{name: "stale", backend: GenUIBackend{Model: "gpt-6-astra", Models: []string{"gpt-6-sol"}}, want: ""},
		{name: "no list yet", backend: GenUIBackend{Model: "gpt-6-astra"}, want: "gpt-6-astra"},
		{name: "agent default", backend: GenUIBackend{Models: []string{"gpt-6-sol"}}, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.backend.UsableModel(); got != tt.want {
				t.Fatalf("UsableModel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenUILoginRunsTheBackendsOwnSignInCommand(t *testing.T) {
	for _, tt := range []struct {
		backend string
		want    string
	}{
		{backend: genui.BackendCodex, want: "codex-acp login"},
		{backend: genui.BackendAntigravity, want: "agy acp --login"},
	} {
		t.Run(tt.backend, func(t *testing.T) {
			var launched string
			s := &Service{
				settings: &settingsStore{dir: t.TempDir()},
				terminal: func(_ context.Context, line string) error {
					launched = line
					return nil
				},
			}
			if err := s.OpenGenUILogin(tt.backend); err != nil {
				t.Fatalf("OpenGenUILogin: %v", err)
			}
			if launched != tt.want {
				t.Fatalf("launched %q, want %q", launched, tt.want)
			}
		})
	}
}

func TestCustomACPHasNoLoginCommandOfItsOwn(t *testing.T) {
	s := &Service{settings: &settingsStore{dir: t.TempDir()}}
	err := s.OpenGenUILogin(genui.BackendCustom)
	if err == nil || !strings.Contains(err.Error(), "Sign in with the agent's own command") {
		t.Fatalf("error = %v, want the custom sign-in hint", err)
	}
}

func TestProbeGenUIWithoutACommandReportsACustomAgentThatIsNotSetUp(t *testing.T) {
	s := &Service{settings: &settingsStore{dir: t.TempDir()}}
	result, err := s.ProbeGenUI(genui.BackendCustom)
	if err != nil {
		t.Fatalf("ProbeGenUI: %v", err)
	}
	if result.Status != genui.AgentMissing || result.Detail == "" {
		t.Fatalf("probe = %+v, want a missing command and its detail", result)
	}
}

func TestGenUIProbeConfigCarriesTheBackendCommandAndArguments(t *testing.T) {
	s := &Service{settings: &settingsStore{dir: t.TempDir()}}
	cfg, err := s.genUIProbeConfig(genui.BackendAntigravity)
	if err != nil {
		t.Fatalf("genUIProbeConfig: %v", err)
	}
	if cfg.Backend != genui.BackendAntigravity || cfg.Command != "agy" || len(cfg.Args) != 1 || cfg.Args[0] != "acp" {
		t.Fatalf("config = %+v, want agy with its acp argument", cfg)
	}
	cfg, err = s.genUIProbeConfig(genui.BackendCodex)
	if err != nil {
		t.Fatalf("genUIProbeConfig: %v", err)
	}
	if cfg.Command != "codex-acp" || len(cfg.Args) != 0 {
		t.Fatalf("codex config = %+v, want codex-acp without arguments", cfg)
	}
}
