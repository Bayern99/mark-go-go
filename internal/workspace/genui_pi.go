package workspace

import (
	"context"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/acp"
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/pigenui"
)

// piConfig is what the Pi backend needs to launch: its own command, the provider
// recorded beside the model, and the model and effort the reader chose.
func (s *Service) piConfig(backend GenUIBackend) pigenui.Config {
	return pigenui.Config{
		Command:      strings.TrimSpace(backend.Command),
		Provider:     backend.Provider,
		Model:        backend.UsableModel(),
		ThoughtLevel: backend.ThoughtLevel,
	}
}

// generateOnPi runs one Pi generation. The job carries what the orchestrator
// needs — the request, the view, validation and the write guard — and the Pi
// config decides how the process is launched.
func generateOnPi(ctx context.Context, cfg pigenui.Config, job acp.GenerationJob, progress func(genui.GenerateProgress) error) error {
	cfg.Model = job.Model
	cfg.ThoughtLevel = job.ThoughtLevel
	cfg.Timeout = job.Timeout
	cfg.Now = job.Now
	cfg.Validate = job.Validate
	cfg.BeforeWrite = job.BeforeWrite
	if cfg.Env == nil {
		cfg.Env = job.Env
	}
	return pigenui.RunGeneration(ctx, cfg, job.Req, job.ViewPath, progress)
}

// probePi asks Pi about the selected provider's credentials. It starts no
// session, so it can neither generate nor read a document.
func (s *Service) probePi(backend GenUIBackend) (genui.ProbeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), genuiProbeTimeout)
	defer cancel()
	return pigenui.Probe(ctx, s.piConfig(backend))
}

// refreshPi reads the models Pi offers through a short-lived session that
// submits no prompt and no document text.
func (s *Service) refreshPi(backendID string, backend GenUIBackend) (genui.ModelCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), genuiProbeTimeout)
	defer cancel()
	catalog, err := pigenui.ListModels(ctx, s.piConfig(backend))
	if err != nil {
		return catalog, err
	}
	return catalog, s.rememberCatalog(backendID, catalog)
}

// rememberCatalog records what a refresh learned about a backend: its models,
// and the provider of its current model when none was recorded yet. The provider
// is what Pi's credential check, model write and Terminal login are named by.
func (s *Service) rememberCatalog(backendID string, catalog genui.ModelCatalog) error {
	if s.settings == nil {
		return nil
	}
	ids := make([]string, 0, len(catalog.Models))
	for _, model := range catalog.Models {
		ids = append(ids, model.ID)
	}
	patch := GenUIBackendPatch{Models: &ids, ModelOptions: &catalog.Models}
	backend := s.genUISettings().GenUI.For(backendID)
	if backend.Provider == "" && (backend.Model == "" || backend.Model == catalog.CurrentModel) {
		provider := catalog.CurrentProvider
		if provider == "" {
			for _, option := range catalog.Models {
				if option.ID != catalog.CurrentModel || option.Provider == "" {
					continue
				}
				if provider != "" && provider != option.Provider {
					provider = ""
					break
				}
				provider = option.Provider
			}
		}
		if provider != "" {
			patch.Provider = &provider
			patch.Model = &catalog.CurrentModel
		}
	}
	_, err := s.settings.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{backendID: patch}}})
	return err
}
