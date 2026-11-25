package pigenui

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestListModelsReadsTheModelsAndTheCurrentModelsLevels(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	catalog, err := ListModels(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(catalog.Models) != 2 {
		t.Fatalf("models = %+v, want the two Pi offers", catalog.Models)
	}
	if catalog.Models[0].ID != "claude-sonnet-4-5" || catalog.Models[0].Name != "Claude Sonnet 4.5" || catalog.Models[0].Provider != "anthropic" {
		t.Errorf("first model = %+v, want Pi's own id, name and provider", catalog.Models[0])
	}
	if catalog.CurrentModel != "claude-sonnet-4-5" {
		t.Errorf("current model = %q, want the model that was applied", catalog.CurrentModel)
	}
	if catalog.ThoughtLevel != "low" {
		t.Errorf("thinking level = %q, want Pi's current one", catalog.ThoughtLevel)
	}
	var levels []string
	for _, level := range catalog.ThoughtLevels {
		levels = append(levels, level.Value)
	}
	if !slices.Equal(levels, []string{"off", "low", "high", "max"}) {
		t.Errorf("levels = %v, want the current model's levels", levels)
	}

	records := fixtureLog(t, logPath)
	commands := fixtureCommands(records)
	if slices.Contains(commands, "prompt") {
		t.Fatalf("commands = %s, want no prompt while listing models", describe(commands))
	}
	for _, command := range []string{"get_available_models", "set_model", "get_available_thinking_levels"} {
		if !slices.Contains(commands, command) {
			t.Errorf("commands = %s, want %s", describe(commands), command)
		}
	}
	var exits int
	for _, record := range records {
		if record["event"] == "exit" {
			exits++
		}
	}
	if exits != 1 {
		t.Errorf("recording %d exits, want the model list's process to end", exits)
	}
}

func TestListModelsWithoutASavedModelKeepsPisOwnChoice(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Model, cfg.Provider = "", ""
	catalog, err := ListModels(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if catalog.CurrentModel != "claude-sonnet-4-5" {
		t.Errorf("current model = %q, want the model Pi reports", catalog.CurrentModel)
	}
	if slices.Contains(fixtureCommands(fixtureLog(t, logPath)), "set_model") {
		t.Error("the model list must not write a model the reader did not save")
	}
}
