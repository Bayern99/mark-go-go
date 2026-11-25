package pigenui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func TestMain(m *testing.M) {
	if os.Getenv("MGO_FIXTURE_PI") == "1" {
		runPiFixture()
		os.Exit(0)
	}
	home, err := os.MkdirTemp("", "pi-test-home-")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"HOME", "USERPROFILE"} {
		if err := os.Setenv(name, home); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

// testConfig points the runner at the fake Pi in this test binary. The folder
// the guard is written to and the process the turn starts are real, so what the
// tests read is what the app would launch.
func testConfig(t *testing.T, logPath string) Config {
	t.Helper()
	return Config{
		Command:      os.Args[0],
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-5",
		ThoughtLevel: "high",
		Env: append(os.Environ(),
			"MGO_FIXTURE_PI=1",
			fixtureLogEnv+"="+logPath,
		),
		GuardGrace: 20 * time.Second,
		AbortGrace: 20 * time.Second,
	}
}

func turnDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve the turn folder: %v", err)
	}
	return resolved
}

func runTurn(t *testing.T, cfg Config, turn genui.Turn) (genui.TurnResult, []string, error) {
	t.Helper()
	var phases []string
	turn.Progress = func(progress genui.GenerateProgress) error {
		phases = append(phases, progress.Phase)
		return nil
	}
	runner := &Runner{cfg: cfg}
	result, err := runner.ExecuteTurn(context.Background(), turn)
	return result, phases, err
}

func TestATurnStartsPiInTheTurnsFolderWithOnlyTheFileTools(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	dir := turnDir(t)

	result, phases, err := runTurn(t, cfg, genui.Turn{Dir: dir, Prompt: "Write the view."})
	if err != nil {
		t.Fatalf("ExecuteTurn: %v", err)
	}
	if result.Reply != "Wrote the view." {
		t.Errorf("reply = %q, want Pi's last assistant message", result.Reply)
	}
	if result.Model != "claude-sonnet-4-5" {
		t.Errorf("model = %q, want the model Pi confirmed", result.Model)
	}

	records := fixtureLog(t, logPath)
	starts := fixtureStarts(records)
	if len(starts) != 1 {
		t.Fatalf("started %d processes, want one", len(starts))
	}
	args := fixtureStrings(starts[0]["args"])
	// The guard file lives in a folder of its own, so its path is the one
	// argument the test cannot know in advance.
	if len(args) != 11 {
		t.Fatalf("args = %v, want eleven arguments", args)
	}
	want := []string{
		"--mode", "rpc",
		"--no-session",
		"--no-extensions",
		"-e", args[5],
		"--no-approve",
		"--no-mcp",
		"--no-context-files",
		"--tools", "read,write,edit",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
	if filepath.Base(args[5]) != "guard.js" {
		t.Errorf("guard path = %q, want the guard extension", args[5])
	}
	if starts[0]["cwd"] != dir {
		t.Errorf("working folder = %v, want the turn's folder %q", starts[0]["cwd"], dir)
	}
	guard, _ := starts[0]["guard"].(map[string]any)
	if guard["root"] != dir {
		t.Errorf("guard root = %v, want %q", guard["root"], dir)
	}
	if want := filepath.Join(dir, "out.json"); guard["out"] != want {
		t.Errorf("guard out = %v, want %q", guard["out"], want)
	}
	if guard["version"] != GuardVersion {
		t.Errorf("guard version = %v, want %q", guard["version"], GuardVersion)
	}
	if token, _ := guard["token"].(string); token == "" {
		t.Error("the guard got no token, so the handshake could not be tied to this turn")
	}
	wantPhases := []string{"starting", "thinking", "reading", "writing"}
	if !slices.Equal(phases, wantPhases) {
		t.Errorf("phases = %v, want %v", phases, wantPhases)
	}
	if _, err := os.Stat(filepath.Dir(args[5])); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the guard folder outlived the turn: %v", err)
	}
}

func TestEveryTurnStartsAndEndsItsOwnProcess(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	runner := &Runner{cfg: cfg}
	for i := 0; i < 2; i++ {
		if _, err := runner.ExecuteTurn(context.Background(), genui.Turn{Dir: turnDir(t), Prompt: "Write the view."}); err != nil {
			t.Fatalf("turn %d: %v", i+1, err)
		}
	}
	records := fixtureLog(t, logPath)
	starts := fixtureStarts(records)
	if len(starts) != 2 {
		t.Fatalf("started %d processes, want one per turn", len(starts))
	}
	if starts[0]["pid"] == starts[1]["pid"] {
		t.Fatalf("both turns ran in process %v, so a process was reused", starts[0]["pid"])
	}
	var exits int
	for _, record := range records {
		if record["event"] == "exit" {
			exits++
		}
	}
	if exits != 2 {
		t.Errorf("recorded %d exits, want each turn's process to end", exits)
	}
}

func TestATurnWithoutTheGuardNeverSendsAPrompt(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Env = append(cfg.Env, "MGO_FIXTURE_NO_GUARD=1")
	cfg.GuardGrace = 300 * time.Millisecond

	_, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."})
	if err == nil {
		t.Fatal("a turn whose guard never loaded must not generate")
	}
	if !strings.Contains(err.Error(), "guard") {
		t.Errorf("error = %q, want it to name the guard", err)
	}
	commands := fixtureCommands(fixtureLog(t, logPath))
	if slices.Contains(commands, "prompt") {
		t.Fatalf("commands = %s, want no prompt without a guard", describe(commands))
	}
}

func TestAGuardOfAnotherVersionStopsTheTurn(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Env = append(cfg.Env, "MGO_FIXTURE_GUARD_STATUS=mismatch:{version}:{token}")

	_, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."})
	if err == nil {
		t.Fatal("a guard of another version must stop the turn")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error = %q, want it to name the version", err)
	}
	commands := fixtureCommands(fixtureLog(t, logPath))
	if slices.Contains(commands, "prompt") {
		t.Fatalf("commands = %s, want no prompt from an unguarded run", describe(commands))
	}
}

func TestAGuardPiCouldNotLoadStopsTheTurnWithItsOwnMessage(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Env = append(cfg.Env, "MGO_FIXTURE_BROKEN_GUARD=1")

	_, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."})
	if err == nil {
		t.Fatal("a turn whose guard extension failed to load must not generate")
	}
	if !strings.Contains(err.Error(), "guard") {
		t.Errorf("error = %q, want it to name the guard", err)
	}
	failure := genui.FailureOf(err)
	if !strings.Contains(failure.Detail, "Failed to load extension") {
		t.Errorf("detail = %q, want Pi's own explanation", failure.Detail)
	}
	if commands := fixtureCommands(fixtureLog(t, logPath)); slices.Contains(commands, "prompt") {
		t.Fatalf("commands = %s, want no prompt", describe(commands))
	}
}

func TestADialogThisAppCannotShowIsDeclinedAndTheRunAborts(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Env = append(cfg.Env, "MGO_FIXTURE_DIALOG=1")

	if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."}); err != nil {
		t.Fatalf("ExecuteTurn: %v", err)
	}
	records := fixtureLog(t, logPath)
	var cancelled bool
	for _, record := range records {
		if record["event"] == "uiResponse" && record["id"] == "dialog-1" && record["cancelled"] == true {
			cancelled = true
		}
	}
	if !cancelled {
		t.Error("the app must answer the dialog by its id with cancelled")
	}
	commands := fixtureCommands(records)
	if !slices.Contains(commands, "abort") {
		t.Errorf("commands = %s, want an abort after the declined dialog", describe(commands))
	}
}

func TestCancellingATurnAbortsAndThenEndsTheProcess(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	cfg.Env = append(cfg.Env, "MGO_FIXTURE_IGNORE_ABORT=1")
	cfg.AbortGrace = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &Runner{cfg: cfg}
	_, err := runner.ExecuteTurn(ctx, genui.Turn{
		Dir:    turnDir(t),
		Prompt: "Write the view.",
		Progress: func(progress genui.GenerateProgress) error {
			if progress.Phase == "thinking" {
				cancel()
			}
			return nil
		},
	})
	if !errors.Is(err, genui.ErrCancelled) {
		t.Fatalf("error = %v, want a cancelled run", err)
	}
	commands := fixtureCommands(fixtureLog(t, logPath))
	if !slices.Contains(commands, "abort") {
		t.Errorf("commands = %s, want an abort before the process was killed", describe(commands))
	}
	assertProcessGone(t, fixtureStarts(fixtureLog(t, logPath)))
}

func TestTheModelIsAppliedWithItsProvider(t *testing.T) {
	cases := []struct {
		name      string
		provider  string
		model     string
		wantModel bool
	}{
		{name: "a saved model and its provider", provider: "anthropic", model: "claude-sonnet-4-5", wantModel: true},
		{name: "no model saved", provider: "anthropic", model: "", wantModel: false},
		{name: "no provider recorded", provider: "", model: "claude-sonnet-4-5", wantModel: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "fixture.log")
			cfg := testConfig(t, logPath)
			cfg.Provider, cfg.Model = tt.provider, tt.model
			if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."}); err != nil {
				t.Fatalf("ExecuteTurn: %v", err)
			}
			records := fixtureLog(t, logPath)
			var applied bool
			for _, record := range records {
				if record["event"] != "setModel" {
					continue
				}
				applied = true
				if record["provider"] != tt.provider || record["model"] != tt.model {
					t.Errorf("set_model = %v/%v, want %q/%q", record["provider"], record["model"], tt.provider, tt.model)
				}
			}
			if applied != tt.wantModel {
				t.Errorf("set_model sent = %v, want %v (%s)", applied, tt.wantModel, describe(fixtureCommands(records)))
			}
		})
	}
}

func TestAThinkingLevelTheModelDoesNotOfferIsNotWritten(t *testing.T) {
	cases := []struct {
		name    string
		levels  string
		wantSet bool
	}{
		{name: "offered", levels: "off,low,high", wantSet: true},
		{name: "not offered", levels: "off,low", wantSet: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "fixture.log")
			cfg := testConfig(t, logPath)
			cfg.Env = append(cfg.Env, "MGO_FIXTURE_LEVELS="+tt.levels)
			if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view.", ThoughtLevel: "high"}); err != nil {
				t.Fatalf("ExecuteTurn: %v", err)
			}
			commands := fixtureCommands(fixtureLog(t, logPath))
			if got := slices.Contains(commands, "set_thinking_level"); got != tt.wantSet {
				t.Errorf("set_thinking_level sent = %v, want %v (%s)", got, tt.wantSet, describe(commands))
			}
		})
	}
}

func TestTheTurnAsksForTheModelBeforeItSendsThePrompt(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fixture.log")
	cfg := testConfig(t, logPath)
	if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."}); err != nil {
		t.Fatalf("ExecuteTurn: %v", err)
	}
	cfg.ThoughtLevel = "high"
	if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view.", ThoughtLevel: "high"}); err != nil {
		t.Fatalf("ExecuteTurn with a thinking level: %v", err)
	}
	commands := fixtureCommands(fixtureLog(t, logPath))
	order := []string{"set_model", "get_available_thinking_levels", "set_thinking_level", "prompt"}
	position := 0
	for _, command := range commands {
		if position < len(order) && command == order[position] {
			position++
		}
	}
	if position != len(order) {
		t.Fatalf("commands = %s, want %v in that order", describe(commands), order)
	}
}

func TestAMissingCommandIsAnInstallProblem(t *testing.T) {
	cfg := testConfig(t, filepath.Join(t.TempDir(), "fixture.log"))
	cfg.Command = "mark-go-go-pi-that-is-not-installed"
	result, err := Probe(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentMissing {
		t.Fatalf("status = %q, want the missing status", result.Status)
	}
	if _, _, err := runTurn(t, cfg, genui.Turn{Dir: turnDir(t), Prompt: "Write the view."}); err == nil {
		t.Fatal("a turn with no Pi command must fail")
	}
}
