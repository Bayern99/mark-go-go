package pigenui

import (
	"context"
	"os"
	"slices"
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
	defer func() {
		if ctx.Err() != nil {
			live.client.Kill()
		}
		live.shutdown(r.cfg.abortGrace())
	}()
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
	catalog.Models = make([]genui.ModelOption, 0, len(listed.Models))
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

	if slices.ContainsFunc(catalog.Models, func(option genui.ModelOption) bool {
		return option.ID == r.cfg.Model && option.Provider == r.cfg.Provider
	}) {
		if err := live.applyModel(ctx, r.cfg.Provider, r.cfg.Model); err != nil {
			return catalog, err
		}
	}
	if err := live.applyThoughtLevel(ctx, r.cfg.ThoughtLevel); err != nil {
		return catalog, err
	}
	var state struct {
		Model struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		} `json:"model"`
		ThinkingLevel string `json:"thinkingLevel"`
	}
	if err := live.call(ctx, "get_state", nil, &state); err != nil {
		return catalog, err
	}
	catalog.CurrentModel, catalog.CurrentProvider, catalog.ThoughtLevel = state.Model.ID, state.Model.Provider, state.ThinkingLevel

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
