package acp

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func setCalls(log []map[string]any) []map[string]any {
	return logRecv(log, "session/set_config_option")
}

func setModelCalls(log []map[string]any) []map[string]any {
	return logRecv(log, "session/set_model")
}

func TestCodexStyleBackendLaunchesWithoutArguments(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) {
		job.Args = nil
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	args, _ := logEntries(log, "args")[0]["args"].([]any)
	if len(args) != 1 {
		t.Fatalf("argv = %v, want only the executable for a backend that takes no arguments", args)
	}
}

func TestDeclaredModelOptionWinsOverTheLegacyList(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{Out: minimalOut, BothModelPaths: true}, nil, func(job *GenerationJob) {
		job.Model = "claude-opus-5-5"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	sets := setCalls(log)
	if len(sets) != 1 || sets[0]["configId"] != "model" || sets[0]["value"] != "claude-opus-5-5" {
		t.Fatalf("set_config_option = %v, want the model option", sets)
	}
	if legacy := setModelCalls(log); len(legacy) != 0 {
		t.Fatalf("session/set_model = %v; the declared option is the main path", legacy)
	}
}

func TestLegacyModelListIsUsedOnlyWithoutAModelOption(t *testing.T) {
	err, job, log := driveGeneration(t, fakeSpec{Out: minimalOut, LegacyModels: true}, nil, func(job *GenerationJob) {
		job.Model = "claude-opus-5-5"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	if sets := setCalls(log); len(sets) != 0 {
		t.Fatalf("set_config_option = %v, want none without a declared model option", sets)
	}
	legacy := setModelCalls(log)
	if len(legacy) != 1 || legacy[0]["modelId"] != "claude-opus-5-5" {
		t.Fatalf("session/set_model = %v, want the requested model", legacy)
	}
	if got := writtenView(t, job.ViewPath).Model; got != "claude-opus-5-5" {
		t.Fatalf("view model = %q, want the legacy-confirmed model", got)
	}
}

func TestLegacyPathKeepsACompoundModelID(t *testing.T) {
	err, job, log := driveGeneration(t, fakeSpec{Out: minimalOut, LegacyModels: true, Models: []string{"gpt-6-sol[high]", "gpt-6-sol[low]"}}, nil, func(job *GenerationJob) {
		job.Model = "gpt-6-sol[high]"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	legacy := setModelCalls(log)
	if len(legacy) != 1 || legacy[0]["modelId"] != "gpt-6-sol[high]" {
		t.Fatalf("session/set_model = %v, want the compound id the list declares", legacy)
	}
	if got := writtenView(t, job.ViewPath).Model; got != "gpt-6-sol[high]" {
		t.Fatalf("view model = %q, want the compound id", got)
	}
}

func TestStandardPathSplitsACompoundModelID(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{
		Out:           minimalOut,
		Models:        []string{"gpt-6-sol", "gpt-6-luna"},
		ThoughtValues: []string{"low", "high"},
	}, nil, func(job *GenerationJob) {
		job.Model = "gpt-6-sol[high]"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	sets := setCalls(log)
	if len(sets) != 2 {
		t.Fatalf("set_config_option = %v, want the model option and then the effort", sets)
	}
	if sets[0]["configId"] != "model" || sets[0]["value"] != "gpt-6-sol" {
		t.Fatalf("model write = %v, want the declared base id without the effort", sets[0])
	}
	if sets[1]["configId"] != "thought_level" || sets[1]["value"] != "high" {
		t.Fatalf("effort write = %v, want the effort the id carried", sets[1])
	}
	if legacy := setModelCalls(log); len(legacy) != 0 {
		t.Fatalf("session/set_model = %v; a compound id belongs to the legacy path", legacy)
	}
}

func TestThoughtLevelOptionIDVariants(t *testing.T) {
	for _, tt := range []struct {
		name     string
		id       string
		category string
	}{
		{name: "thought_level", id: "thought_level"},
		{name: "thoughtLevel", id: "thoughtLevel"},
		{name: "reasoning_effort", id: "reasoning_effort"},
		{name: "category only", id: "effort", category: "reasoning_effort"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err, _, log := driveGeneration(t, fakeSpec{
				Out:             minimalOut,
				ThoughtValues:   []string{"low", "high"},
				ThoughtID:       tt.id,
				ThoughtCategory: tt.category,
			}, nil, func(job *GenerationJob) { job.ThoughtLevel = "high" })
			if err != nil {
				t.Fatalf("RunGeneration: %v", err)
			}
			if got := selectedThought(t, log); got.ID != tt.id || got.Value != "high" {
				t.Fatalf("thought write = %+v, want %s=high", got, tt.id)
			}
		})
	}
}

func TestOptionsComeFromEachResponseNotTheFirstOne(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{
		Out:           minimalOut,
		Models:        []string{"gpt-6-sol"},
		InitialModel:  "gpt-6-astra",
		ThoughtValues: []string{"low", "high"},
		ThoughtID:     "reasoning_effort",
		ThoughtModels: []string{"gpt-6-sol"},
	}, nil, func(job *GenerationJob) {
		job.Model = "gpt-6-sol"
		job.ThoughtLevel = "high"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	writes := setCalls(log)
	if len(writes) != 2 {
		t.Fatalf("writes = %v, want the model and then the effort it declared", writes)
	}
	if writes[0]["configId"] != "model" || writes[0]["value"] != "gpt-6-sol" {
		t.Fatalf("first write = %v, want the model", writes[0])
	}
	if writes[1]["configId"] != "reasoning_effort" || writes[1]["value"] != "high" {
		t.Fatalf("second write = %v, want the effort the new model declared", writes[1])
	}
}

func TestAThoughtLevelTheCurrentModelDoesNotOfferIsSkipped(t *testing.T) {
	err, _, log := driveGeneration(t, fakeSpec{
		Out:           minimalOut,
		Models:        []string{"gpt-6-sol"},
		InitialModel:  "gpt-6-astra",
		ThoughtValues: []string{"low", "high"},
		ThoughtModels: []string{"gpt-6-sol"},
	}, nil, func(job *GenerationJob) {
		job.Model = ""
		job.ThoughtLevel = "high"
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	for _, write := range setCalls(log) {
		if write["configId"] != "model" {
			t.Fatalf("writes = %v; the initial model declares no effort option", setCalls(log))
		}
	}
}

type fakeSelection struct {
	backend, model, thought string
	calls                   int
}

func (f *fakeSelection) SetSelection(backend, model, thought string) error {
	f.backend, f.model, f.thought, f.calls = backend, model, thought, f.calls+1
	return nil
}

func TestConfirmedSelectionReachesTheSettings(t *testing.T) {
	selection := &fakeSelection{}
	err, _, _ := driveGeneration(t, fakeSpec{
		Out:           minimalOut,
		ThoughtValues: []string{"low", "high"},
	}, nil, func(job *GenerationJob) {
		job.Backend = genui.BackendCodex
		job.Model = "gemini-3.8-flash"
		job.ThoughtLevel = "high"
		job.Selection = selection
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	if selection.calls != 1 || selection.backend != genui.BackendCodex || selection.model != "gemini-3.8-flash" || selection.thought != "high" {
		t.Fatalf("selection = %+v, want the confirmed codex model and effort", selection)
	}
}

type thoughtWrite struct {
	ID    string
	Value string
}

func selectedThought(t *testing.T, log []map[string]any) thoughtWrite {
	t.Helper()
	for _, write := range setCalls(log) {
		if write["configId"] == "model" {
			continue
		}
		id, _ := write["configId"].(string)
		value, _ := write["value"].(string)
		return thoughtWrite{ID: id, Value: value}
	}
	t.Fatalf("no thought level write; writes = %v", setCalls(log))
	return thoughtWrite{}
}

func TestLaunchKeepsArgumentsOutOfTheExecutable(t *testing.T) {
	_, _, log := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, nil)
	args, _ := logEntries(log, "args")[0]["args"].([]any)
	if len(args) != 2 {
		t.Fatalf("argv = %v, want the executable and acp", args)
	}
	if args[1] != "acp" {
		t.Fatalf("argv = %v, want acp as the first argument", args)
	}
	if slices.Contains(args, "--version") {
		t.Fatalf("argv = %v, want no version entry point", args)
	}
}

func TestAnEffortTheAgentDoesNotApplyIsNotRecorded(t *testing.T) {
	selection := &fakeSelection{}
	err, _, _ := driveGeneration(t, fakeSpec{
		Out:            minimalOut,
		ThoughtValues:  []string{"low", "high"},
		ThoughtCurrent: "low",
		ThoughtStuck:   true,
	}, nil, func(job *GenerationJob) {
		job.Backend = genui.BackendCodex
		job.Model = "gemini-3.8-flash"
		job.ThoughtLevel = "high"
		job.Selection = selection
	})
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	if selection.model != "gemini-3.8-flash" || selection.thought != "" {
		t.Fatalf("selection = %+v, want the model but not the unapplied effort", selection)
	}
}
