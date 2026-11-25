package acp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func probeConfig(t *testing.T, spec fakeSpec, mutate func(*ProbeConfig)) ProbeConfig {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	cfg := ProbeConfig{
		Backend: genui.BackendAntigravity,
		Command: os.Args[0],
		Args:    []string{"acp"},
		Env: append(os.Environ(),
			fakeAgentEnv+"=1",
			fakeSpecEnv+"="+string(data),
			fakeLogEnv+"="+filepath.Join(dir, "fake.log"),
		),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

func TestProbeReportsAMissingExecutable(t *testing.T) {
	result, err := Probe(context.Background(), ProbeConfig{Backend: genui.BackendAntigravity, Command: "definitely-not-an-agent-cmd"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentMissing || !strings.Contains(result.Hint, "Install the Antigravity CLI") {
		t.Fatalf("probe = %+v, want a missing agent with the install hint", result)
	}
}

func TestProbeRecordsTheVersionAndLeavesLoginUnknown(t *testing.T) {
	cfg := probeConfig(t, fakeSpec{}, nil)
	result, err := Probe(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentUnknown {
		t.Fatalf("status = %q, want unknown: authMethods lists ways to sign in, not that someone did", result.Status)
	}
	if !strings.Contains(result.Version, "fake-agent 1.2.3") || result.Executable == "" {
		t.Fatalf("probe = %+v, want the version and executable", result)
	}
}

func TestProbeSeesADeclaredLoginAndOutdatedAgent(t *testing.T) {
	result, err := Probe(context.Background(), probeConfig(t, fakeSpec{
		Errors: map[string]fakeRPCError{"initialize": {Code: -32000, Message: "Authentication required"}},
	}, nil))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentLoginRequired {
		t.Fatalf("status = %q, want login-required", result.Status)
	}

	result, err = Probe(context.Background(), probeConfig(t, fakeSpec{
		Errors: map[string]fakeRPCError{"initialize": {Code: -32603, Message: "this adapter needs a newer codex app-server"}},
	}, nil))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentOutdated || !strings.Contains(strings.ToLower(result.Hint), "update") {
		t.Fatalf("probe = %+v, want agent-outdated with an upgrade hint", result)
	}

	result, err = Probe(context.Background(), probeConfig(t, fakeSpec{
		ProtocolVersion: 2,
	}, nil))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentOutdated {
		t.Fatalf("status = %q, want agent-outdated for another ACP protocol version", result.Status)
	}
}

func TestProbeDoesNotCallAnInvalidParameterOutdated(t *testing.T) {
	result, err := Probe(context.Background(), probeConfig(t, fakeSpec{
		Errors: map[string]fakeRPCError{"initialize": {Code: -32602, Message: "Invalid params: cwd"}},
	}, nil))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentFailed {
		t.Fatalf("status = %q, want failed: a plain -32602 says nothing about the version", result.Status)
	}
}

func TestProbeRecordsTheCodexCLIBehindTheAdapter(t *testing.T) {
	cfg := probeConfig(t, fakeSpec{}, func(cfg *ProbeConfig) {
		cfg.Backend = genui.BackendCodex
		cfg.CodexCommand = os.Args[0]
	})
	result, err := Probe(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentUnknown {
		t.Fatalf("probe = %+v, want unknown", result)
	}
	if !strings.Contains(result.Detail, "codex fake-agent 1.2.3") {
		t.Fatalf("detail = %q, want the codex CLI version it actually calls", result.Detail)
	}

	cfg.CodexCommand = "definitely-not-codex"
	result, err = Probe(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Status != genui.AgentMissing || !strings.Contains(strings.ToUpper(result.Hint), "CODEX_PATH") {
		t.Fatalf("probe = %+v, want the missing codex CLI and its hint", result)
	}
}

func TestListModelsReadsTheStandardOptions(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "fake.log")
	cfg := probeConfig(t, fakeSpec{
		Models:         []string{"gpt-6-sol", "gpt-6-luna"},
		ThoughtValues:  []string{"low", "high"},
		ThoughtCurrent: "high",
		InitialModel:   "gpt-6-sol",
		SessionClose:   true,
	}, func(cfg *ProbeConfig) { cfg.Env = append(cfg.Env, fakeLogEnv+"="+logPath) })
	catalog, err := ListModels(context.Background(), SessionConfig{ProbeConfig: cfg})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(catalog.Models) != 2 || catalog.Models[0].ID != "gpt-6-sol" || catalog.CurrentModel != "gpt-6-sol" {
		t.Fatalf("catalog = %+v, want the declared models and current one", catalog)
	}
	if len(catalog.ThoughtLevels) != 2 || catalog.ThoughtLevel != "high" {
		t.Fatalf("catalog = %+v, want the current model's effort levels", catalog)
	}
	log := readFakeLog(t, logPath)
	if prompts := logRecv(log, "session/prompt"); len(prompts) != 0 {
		t.Fatalf("session/prompt = %v; refreshing models must not run a turn", prompts)
	}
	if closed := logEntries(log, "session-closed"); len(closed) != 1 {
		t.Fatalf("session/close = %v, want the short-lived session closed", closed)
	}
}

func TestListModelsFallsBackToTheLegacyList(t *testing.T) {
	cfg := probeConfig(t, fakeSpec{
		LegacyModels: true,
		Models:       []string{"gpt-6-sol", "gpt-6-luna"},
		InitialModel: "gpt-6-luna",
	}, nil)
	catalog, err := ListModels(context.Background(), SessionConfig{ProbeConfig: cfg})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(catalog.Models) != 2 || catalog.Models[1].ID != "gpt-6-luna" || catalog.CurrentModel != "gpt-6-luna" {
		t.Fatalf("catalog = %+v, want the legacy availableModels list", catalog)
	}
}
