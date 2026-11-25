package workspace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/acp"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/hostterm"
)

const genuiProbeTimeout = 30 * time.Second

// UsableModel is the model a generation may ask for: the saved one only while
// the backend's list still declares it. A stale choice is left out rather than
// written, so the agent keeps its own current model instead of refusing.
func (b GenUIBackend) UsableModel() string {
	model := strings.TrimSpace(b.Model)
	if model == "" || len(b.Models) == 0 {
		return model
	}
	for _, offered := range b.Models {
		if offered == model {
			return model
		}
	}
	return ""
}

func (s *Service) genUISettings() Settings {
	if s == nil || s.settings == nil {
		return DefaultSettings()
	}
	return s.settings.get()
}

func (s *Service) genUIProbeConfig(backendID string) (acp.ProbeConfig, error) {
	b, ok := genui.BackendByID(backendID)
	if !ok {
		return acp.ProbeConfig{}, errors.New("Unknown GenUI backend.")
	}
	settings := s.genUISettings().GenUI.For(b.ID)
	command := strings.TrimSpace(settings.Command)
	if command == "" {
		return acp.ProbeConfig{}, errors.New(b.InstallHint)
	}
	return acp.ProbeConfig{
		Backend: b.ID,
		Command: command,
		Args:    settings.Args,
	}, nil
}

// ProbeGenUI confirms whether a backend can be started and what it answered.
// It never sends a generation request.
func (s *Service) ProbeGenUI(backendID string) (genui.ProbeResult, error) {
	b, ok := genui.BackendByID(backendID)
	if !ok {
		return genui.ProbeResult{Status: genui.AgentMissing, Detail: "Unknown GenUI backend."}, nil
	}
	if b.ID == genui.BackendPi {
		return s.probePi(s.genUISettings().GenUI.For(b.ID))
	}
	cfg, err := s.genUIProbeConfig(b.ID)
	if err != nil {
		return genui.ProbeResult{Status: genui.AgentMissing, Detail: err.Error(), Hint: b.InstallHint}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), genuiProbeTimeout)
	defer cancel()
	return acp.Probe(ctx, cfg)
}

// RefreshGenUIModels reads a backend's models through a short-lived session and
// records the list. It submits no generation request and no document text.
func (s *Service) RefreshGenUIModels(backendID string) (genui.ModelCatalog, error) {
	if b, ok := genui.BackendByID(backendID); ok && b.ID == genui.BackendPi {
		return s.refreshPi(b.ID, s.genUISettings().GenUI.For(b.ID))
	}
	cfg, err := s.genUIProbeConfig(backendID)
	if err != nil {
		return genui.ModelCatalog{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), genuiProbeTimeout)
	defer cancel()
	settings := s.genUISettings().GenUI.For(backendID)
	catalog, err := acp.ListModels(ctx, acp.SessionConfig{ProbeConfig: cfg, Model: settings.UsableModel()})
	if err != nil {
		return genui.ModelCatalog{}, err
	}
	if s.settings != nil {
		ids := make([]string, 0, len(catalog.Models))
		for _, model := range catalog.Models {
			ids = append(ids, model.ID)
		}
		if err := s.settings.setModels(backendID, ids); err != nil {
			return catalog, err
		}
	}
	return catalog, nil
}

// OpenGenUILogin starts a backend's own sign-in command in the system Terminal.
// The app never inherits another plugin's command, profile or environment.
func (s *Service) OpenGenUILogin(backendID string) error {
	b, ok := genui.BackendByID(backendID)
	if !ok {
		return errors.New("Unknown GenUI backend.")
	}
	if b.LoginCommand == "" {
		// A custom agent has no entry point this app could guess; the hint says
		// to sign in with the agent's own command instead of running it here.
		return errors.New(b.LoginHint)
	}
	command, args := b.LoginCommand, slices.Clone(b.LoginArgs)
	if b.ID == genui.BackendPi {
		// Pi signs a provider in, not a model, so the app names the provider it
		// recorded beside the model rather than asking Pi to guess.
		provider := strings.TrimSpace(s.genUISettings().GenUI.For(b.ID).Provider)
		if provider == "" {
			return errors.New(b.LoginHint)
		}
		args = append(args, provider)
	}
	ctx, cancel := context.WithTimeout(context.Background(), genuiProbeTimeout)
	defer cancel()
	launch := hostterm.CommandLine(command, args...)
	if s.terminal != nil {
		return s.terminal(ctx, launch)
	}
	return hostterm.Open(ctx, launch, nil)
}
