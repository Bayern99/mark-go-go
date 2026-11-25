package genui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const fakeCandidate = `{"a2ui":[{"type":"Text"}],"sources":{}}`

const fakeStatementCandidate = `{"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","components":["root = Text(\"hi\")"]}}]}`

type fakeRunner struct {
	mu       sync.Mutex
	out      string
	write    bool
	hold     chan struct{}
	wrote    chan struct{}
	turn     Turn
	dirs     []string
	source   string
	base     string
	err      error
	result   TurnResult
	previews int
}

func (f *fakeRunner) Probe(context.Context) (ProbeResult, error) { return ProbeResult{}, nil }

func (f *fakeRunner) ListModels(context.Context) (ModelCatalog, error) {
	return ModelCatalog{}, nil
}

func (f *fakeRunner) Close() error { return nil }

func (f *fakeRunner) ExecuteTurn(ctx context.Context, turn Turn) (TurnResult, error) {
	f.mu.Lock()
	f.turn = turn
	f.dirs = append(f.dirs, turn.Dir)
	if source, err := os.ReadFile(filepath.Join(turn.Dir, "source.md")); err == nil {
		f.source = string(source)
	}
	if base, err := os.ReadFile(filepath.Join(turn.Dir, "view.json")); err == nil {
		f.base = string(base)
	}
	write, out, hold, wrote := f.write, f.out, f.hold, f.wrote
	f.mu.Unlock()
	if write {
		if err := os.WriteFile(filepath.Join(turn.Dir, "out.json"), []byte(out), 0o644); err != nil {
			return TurnResult{}, err
		}
		if wrote != nil {
			close(wrote)
		}
	}
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return TurnResult{}, ErrCancelled
		}
	}
	return f.result, f.err
}

func writeBaseView(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md.genui.json")
	const view = `{"version":2,"generatedAt":"2026-01-02T03:04:05Z","request":"older","sources":{},"a2ui":[{"createSurface":{"surfaceId":"surface-1","components":[{"id":"card","type":"Text"}]}}]}`
	if err := os.WriteFile(path, []byte(view), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func orchestratorFor(t *testing.T, runner Runner, req GenerateRequest, viewPath string, progress func(GenerateProgress) error) *Orchestrator {
	t.Helper()
	return &Orchestrator{
		Runner:          runner,
		Req:             req,
		ViewPath:        viewPath,
		Model:           "fake-model",
		ThoughtLevel:    "high",
		Now:             func() time.Time { return time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC) },
		Validate:        func(context.Context, []byte) error { return nil },
		PreviewInterval: time.Millisecond,
	}
}

func TestOrchestratorGivesEveryTurnAnIsolatedFolderAndCommitsTheCandidate(t *testing.T) {
	hold := make(chan struct{})
	wrote := make(chan struct{})
	runner := &fakeRunner{write: true, out: fakeStatementCandidate, hold: hold, wrote: wrote, result: TurnResult{Model: "fake-model"}}
	var previews int
	var mu sync.Mutex
	seen := make(chan struct{})
	var once sync.Once
	req := GenerateRequest{Text: "# Alpha\n\nBody.\n", Mode: "full", Language: StatementLanguage, Sections: []SectionRef{{Heading: "Alpha", Occurrence: 1, Start: 0, End: 14}}}
	viewPath := writeBaseView(t)
	orch := orchestratorFor(t, runner, req, viewPath, nil)
	orch.Artifact = func(string, string, string) {}
	done := make(chan error, 1)
	go func() {
		done <- orch.Run(context.Background(), func(p GenerateProgress) error {
			if p.Preview != nil {
				mu.Lock()
				previews++
				mu.Unlock()
				once.Do(func() { close(seen) })
			}
			return nil
		})
	}()
	<-wrote
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("no streaming preview reached the reader while the agent was still working")
	}
	close(hold)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	gotPreviews := previews
	mu.Unlock()
	if gotPreviews == 0 {
		t.Fatal("no streaming preview reached the reader")
	}
	if runner.source != req.Text {
		t.Fatalf("source.md = %q, want the request's text", runner.source)
	}
	if !strings.Contains(runner.turn.Prompt, "out.json") || !strings.Contains(runner.turn.Prompt, runner.turn.Dir) {
		t.Fatalf("prompt does not name the turn's out.json and folder: %q", runner.turn.Prompt)
	}
	if runner.turn.Model != "fake-model" || runner.turn.ThoughtLevel != "high" {
		t.Fatalf("turn model/effort = %q/%q, want what the orchestrator was configured with", runner.turn.Model, runner.turn.ThoughtLevel)
	}
	if _, err := os.Stat(runner.turn.Dir); !os.IsNotExist(err) {
		t.Fatalf("the turn's folder %q survived the run: %v", runner.turn.Dir, err)
	}
	view, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(view), `"model": "fake-model"`) {
		t.Fatalf("view = %s, want the model the runner confirmed", view)
	}
}

func TestOrchestratorKeepsTheOldViewWhenTheRunnerFails(t *testing.T) {
	before := writeBaseView(t)
	original, err := os.ReadFile(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		runner *fakeRunner
	}{
		{name: "agent failure", runner: &fakeRunner{err: NewFailure(FailAgentMessage, "The agent declined to make the view.", HintAgentMessage)}},
		{name: "invalid output", runner: &fakeRunner{write: true, out: `not json`}},
		{name: "cancelled", runner: &fakeRunner{err: ErrCancelled}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			orch := orchestratorFor(t, tt.runner, GenerateRequest{Text: "# A\n", Mode: "full"}, before, nil)
			if err := orch.Run(context.Background(), nil); err == nil {
				t.Fatal("Run succeeded, want a failure")
			}
			kept, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			if string(kept) != string(original) {
				t.Fatalf("the view changed after %s", tt.name)
			}
		})
	}
}

func TestOrchestratorRefusesAnUnknownModeBeforeStartingTheAgent(t *testing.T) {
	runner := &fakeRunner{}
	orch := orchestratorFor(t, runner, GenerateRequest{Text: "# A\n", Mode: "rewrite"}, filepath.Join(t.TempDir(), "view.json"), nil)
	err := orch.Run(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "Unknown generation mode") {
		t.Fatalf("Run error = %v, want the unknown mode", err)
	}
	if len(runner.dirs) != 0 {
		t.Fatalf("the agent ran for an unknown mode: %v", runner.dirs)
	}
}

func TestOrchestratorHandsAnUpdateTheBaseView(t *testing.T) {
	viewPath := writeBaseView(t)
	base, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{result: TurnResult{Model: "fake-model"}}
	orch := orchestratorFor(t, runner, GenerateRequest{Text: "# A\n", Mode: "update", ReplaceIDs: []string{"card"}}, viewPath, nil)
	if err := orch.Run(context.Background(), nil); err == nil {
		t.Fatal("Run succeeded without out.json")
	}
	if runner.base != string(base) {
		t.Fatalf("view.json in the agent's folder = %q, want the current view", runner.base)
	}
}

func TestOrchestratorSplitsEveryTurnIntoItsOwnFolder(t *testing.T) {
	runner := &fakeRunner{write: true, out: fakeCandidate}
	var dirs []string
	runner.result = TurnResult{Model: "fake-model"}
	orch := orchestratorFor(t, runner, GenerateRequest{Text: "# A\n", Mode: "full"}, filepath.Join(t.TempDir(), "view.json"), nil)
	orch.Artifact = func(name, path, content string) {
		if name == "source.md" {
			dirs = append(dirs, filepath.Dir(path))
		}
	}
	if err := orch.Run(context.Background(), nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := orch.Run(context.Background(), nil); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(dirs) != 2 || dirs[0] == dirs[1] {
		t.Fatalf("turn folders = %v, want a fresh one per turn", dirs)
	}
}

func TestOrchestratorStopsWhenTheReaderCancels(t *testing.T) {
	hold := make(chan struct{})
	wrote := make(chan struct{})
	runner := &fakeRunner{write: true, out: fakeCandidate, hold: hold, wrote: wrote}
	ctx, cancel := context.WithCancel(context.Background())
	orch := orchestratorFor(t, runner, GenerateRequest{Text: "# A\n", Mode: "full"}, filepath.Join(t.TempDir(), "view.json"), nil)
	errCh := make(chan error, 1)
	go func() { errCh <- orch.Run(ctx, nil) }()
	<-wrote
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrCancelled) && FailureOf(err).Kind != FailCancelled {
			t.Fatalf("Run error = %v, want a cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run ignored the cancellation")
	}
	close(hold)
}
