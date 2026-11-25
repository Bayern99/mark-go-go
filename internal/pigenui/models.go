package pigenui

import (
	"context"
	"os"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/pirpc"
)

// ListModels reads the models Pi offers and the thinking levels the current one
// supports. It runs in a folder of its own, submits no prompt and no document
// text, and ends the process before it returns.
func ListModels(ctx context.Context, cfg Config) (genui.ModelCatalog, error) {
	r := &Runner{cfg: cfg}
	return r.listModels(ctx)
}

func (r *Runner) listModels(ctx context.Context) (genui.ModelCatalog, error) {
	var catalog genui.ModelCatalog
	dir, err := os.MkdirTemp("", "markgogo-pi-models-")
	if err != nil {
		return catalog, genui.AgentErrorf("Could not make a folder for the model list: %v.", err)
	}
	defer os.RemoveAll(dir)

	guardPath, cleanupGuard, err := writeGuard()
	if err != nil {
		return catalog, err
	}
	defer cleanupGuard()

	token := newToken()
	cmd, err := command(r.cfg, dir, guardPath, token)
	if err != nil {
		return catalog, err
	}
	client, err := pirpc.New(pirpc.Config{Command: cmd, Name: "Pi"})
	if err != nil {
		return catalog, newStartFailure(r.cfg.Command, err)
	}
	r.begin(client, nil)
	client.SetHandlers(r.handleEvent, r.handleUIRequest)
	client.Start()
	defer func() {
		client.SetHandlers(nil, nil)
		r.end(client)
		client.Shutdown(r.cfg.abortGrace())
	}()
	if err := r.awaitGuard(ctx, client, token); err != nil {
		return catalog, err
	}

	var listed struct {
		Models []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Provider string `json:"provider"`
		} `json:"models"`
	}
	if err := r.call(ctx, "get_available_models", nil, &listed); err != nil {
		return catalog, err
	}
	for _, model := range listed.Models {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		catalog.Models = append(catalog.Models, genui.ModelOption{
			ID:       model.ID,
			Name:     model.Name,
			Provider: model.Provider,
		})
	}

	// A saved model becomes the current one first, because Pi declares the
	// thinking levels of the current model only.
	if model := strings.TrimSpace(r.cfg.Model); model != "" && strings.TrimSpace(r.cfg.Provider) != "" {
		var applied struct {
			ID string `json:"id"`
		}
		if r.call(ctx, "set_model", map[string]any{"provider": r.cfg.Provider, "modelId": model}, &applied) == nil {
			catalog.CurrentModel = applied.ID
		}
	}
	var state struct {
		Model struct {
			ID string `json:"id"`
		} `json:"model"`
		ThinkingLevel string `json:"thinkingLevel"`
	}
	if r.call(ctx, "get_state", nil, &state) == nil {
		if catalog.CurrentModel == "" {
			catalog.CurrentModel = state.Model.ID
		}
		catalog.ThoughtLevel = state.ThinkingLevel
	}

	var levels struct {
		Levels []string `json:"levels"`
	}
	if r.call(ctx, "get_available_thinking_levels", nil, &levels) == nil {
		for _, level := range levels.Levels {
			if level != "" {
				catalog.ThoughtLevels = append(catalog.ThoughtLevels, genui.Choice{Value: level})
			}
		}
	}
	return catalog, nil
}
