package pigenui

import (
	"context"
	"os"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
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

	live, cleanupGuard, err := startTurn(r.cfg, dir, nil)
	if err != nil {
		return catalog, err
	}
	defer cleanupGuard()
	defer live.shutdown(r.cfg.abortGrace())
	if err := live.awaitGuard(ctx, r.cfg.guardGrace()); err != nil {
		return catalog, err
	}

	var listed struct {
		Models []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Provider string `json:"provider"`
		} `json:"models"`
	}
	if err := live.call(ctx, "get_available_models", nil, &listed); err != nil {
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
	// thinking levels of the current model only. A saved model Pi refuses is
	// left out: the catalog still says what Pi actually uses.
	_ = live.applyModel(ctx, r.cfg.Provider, r.cfg.Model)
	var state struct {
		Model struct {
			ID string `json:"id"`
		} `json:"model"`
		ThinkingLevel string `json:"thinkingLevel"`
	}
	if live.call(ctx, "get_state", nil, &state) == nil {
		catalog.CurrentModel = state.Model.ID
		catalog.ThoughtLevel = state.ThinkingLevel
	}
	if applied := live.modelStamp(); applied != "" {
		catalog.CurrentModel = applied
	}

	var levels struct {
		Levels []string `json:"levels"`
	}
	if live.call(ctx, "get_available_thinking_levels", nil, &levels) == nil {
		for _, level := range levels.Levels {
			if level != "" {
				catalog.ThoughtLevels = append(catalog.ThoughtLevels, genui.Choice{Value: level})
			}
		}
	}
	return catalog, nil
}
