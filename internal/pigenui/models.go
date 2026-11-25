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
	if r.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.cfg.Timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return catalog, err
	}
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
			ID       string `json:"id"`
			Provider string `json:"provider"`
		} `json:"model"`
		ThinkingLevel string `json:"thinkingLevel"`
	}
	if live.call(ctx, "get_state", nil, &state) == nil {
		catalog.CurrentModel = state.Model.ID
		catalog.CurrentProvider = state.Model.Provider
		catalog.ThoughtLevel = state.ThinkingLevel
	}
	if catalog.CurrentModel == "" {
		catalog.CurrentModel = live.modelStamp()
		if catalog.CurrentModel == r.cfg.Model {
			catalog.CurrentProvider = r.cfg.Provider
		}
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
	if err := ctx.Err(); err != nil {
		return catalog, err
	}
	return catalog, nil
}
