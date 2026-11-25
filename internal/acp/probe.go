package acp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

const probeTimeout = 20 * time.Second

const versionTimeout = 10 * time.Second

// ProbeConfig names one backend's executables. Args are what makes the command
// speak ACP over stdio; VersionArgs are its separate version entry point.
type ProbeConfig struct {
	Backend string
	Command string
	Args    []string
	// CodexCommand is the CLI behind the Codex ACP adapter; CODEX_PATH names it
	// and otherwise codex is looked up like any other tool.
	CodexCommand string
	Env          []string
	Timeout      time.Duration
}

type SessionConfig struct {
	ProbeConfig
	// Dir is the short-lived session's working directory. It is never a
	// document's folder.
	Dir string
}

func (c ProbeConfig) backend() (genui.Backend, bool) {
	if c.Backend == "" {
		return genui.Backend{}, false
	}
	return genui.BackendByID(c.Backend)
}

func (c ProbeConfig) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return probeTimeout
}

// Probe proves what can be proven without submitting a generation: the
// executable resolves, it answers --version, and its ACP initialize handshake
// is one this app speaks. authMethods only lists the ways an agent can sign in,
// so without further evidence the login state stays unknown.
func Probe(ctx context.Context, cfg ProbeConfig) (genui.ProbeResult, error) {
	backend, _ := cfg.backend()
	result := genui.ProbeResult{Hint: backend.InstallHint}
	resolved, err := ResolveAgentCommand(cfg.Command)
	if err != nil {
		result.Status = genui.AgentMissing
		result.Detail = err.Error()
		return result, nil
	}
	result.Executable = resolved
	env := cfg.Env
	if cfg.Backend == genui.BackendCodex {
		codex := cfg.CodexCommand
		if codex == "" {
			codex = strings.TrimSpace(os.Getenv("CODEX_PATH"))
		}
		if codex == "" {
			codex = "codex"
		}
		path, version, err := versionOf(ctx, codex, env)
		switch {
		case err != nil:
			result.Status = genui.AgentMissing
			result.Detail = "The codex CLI behind codex-acp was not found."
			result.Hint = "Install the Codex CLI, or set CODEX_PATH to its executable, then try again."
			return result, nil
		default:
			result.Detail = "codex " + version + " at " + path
		}
	}

	version, err := runVersion(ctx, resolved, env)
	if err != nil {
		result.Status = genui.AgentFailed
		result.Detail = "The agent's version check failed: " + err.Error()
		result.Hint = genui.HintAgentError
		return result, nil
	}
	result.Version = version
	if result.Detail != "" {
		result.Detail = result.Version + " · " + result.Detail
	} else {
		result.Detail = result.Version
	}

	initialized, err := initialize(ctx, cfg, resolved, env)
	if err != nil {
		classifyProbeFailure(&result, backend, err)
		return result, nil
	}
	result.AuthMethods = initialized.authMethods()
	if initialized.ProtocolVersion != ProtocolVersion {
		result.Status = genui.AgentOutdated
		result.Detail = strings.TrimSpace(result.Detail + " · speaks ACP protocol version " + strconv.Itoa(initialized.ProtocolVersion) + ", not " + strconv.Itoa(ProtocolVersion))
		result.Hint = outdatedHint(backend)
		return result, nil
	}
	result.Status = genui.AgentUnknown
	return result, nil
}

func outdatedHint(backend genui.Backend) string {
	if backend.Name != "" {
		return "Update " + backend.Name + ", then choose Test & Refresh."
	}
	return "Update the agent, then choose Test & Refresh."
}

func classifyProbeFailure(result *genui.ProbeResult, backend genui.Backend, err error) {
	message := strings.ToLower(err.Error())
	switch {
	case genui.IsAuthError(0, message):
		result.Status = genui.AgentLoginRequired
		result.Hint = backend.LoginHint
	case mentionsOutdated(message):
		result.Status = genui.AgentOutdated
		result.Hint = outdatedHint(backend)
	default:
		// A plain -32602 means the agent did not like a parameter, which says
		// nothing about its version.
		result.Status = genui.AgentFailed
		result.Detail = strings.TrimSpace(result.Detail + " · " + err.Error())
		if backend.LoginHint != "" {
			result.Hint = genui.HintAgentError
		}
	}
}

func mentionsOutdated(message string) bool {
	for _, hint := range []string{
		"app-server", "app server", "incompatible", "unsupported protocol", "protocol version",
		"upgrade", "too old", "requires codex",
	} {
		if strings.Contains(message, hint) {
			return true
		}
	}
	return false
}

type initializedHandshake struct {
	ProtocolVersion   int
	AgentCapabilities []byte
	auth              []byte
}

func (h initializedHandshake) authMethods() []string {
	var methods []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if len(h.auth) == 0 {
		return nil
	}
	if json.Unmarshal(h.auth, &methods) != nil {
		return nil
	}
	ids := make([]string, 0, len(methods))
	for _, m := range methods {
		if id := firstNonEmpty(m.ID, m.Kind); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func initialize(ctx context.Context, cfg ProbeConfig, resolved string, env []string) (initializedHandshake, error) {
	var handshake initializedHandshake
	dir, err := os.MkdirTemp("", "markgogo-probe-")
	if err != nil {
		return handshake, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command(resolved, cfg.Args...)
	cmd.Dir = dir
	cmd.Env = env
	agent, err := NewAgent(cmd, nil)
	if err != nil {
		return handshake, err
	}
	agent.Start()
	defer agent.Kill()
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	var answer struct {
		ProtocolVersion   int             `json:"protocolVersion"`
		AgentCapabilities json.RawMessage `json:"agentCapabilities"`
		AuthMethods       json.RawMessage `json:"authMethods"`
	}
	if err := agent.Call("initialize", acpInitializeParams, &answer); err != nil {
		return handshake, err
	}
	handshake.ProtocolVersion = answer.ProtocolVersion
	handshake.AgentCapabilities = answer.AgentCapabilities
	handshake.auth = answer.AuthMethods
	return handshake, nil
}

// ListModels refreshes a backend's models through a short-lived ACP session:
// initialize, session/new, read what it declared, session/close. No prompt is
// ever sent and no generation runs.
func ListModels(ctx context.Context, cfg SessionConfig) (genui.ModelCatalog, error) {
	var catalog genui.ModelCatalog
	resolved, err := ResolveAgentCommand(cfg.Command)
	if err != nil {
		return catalog, err
	}
	dir := cfg.Dir
	if dir == "" {
		dir, err = os.MkdirTemp("", "markgogo-models-")
		if err != nil {
			return catalog, err
		}
		defer os.RemoveAll(dir)
	}
	env := cfg.Env
	cmd := exec.Command(resolved, cfg.Args...)
	cmd.Dir = dir
	cmd.Env = env
	agent, err := NewAgent(cmd, nil)
	if err != nil {
		return catalog, genui.AgentErrorf("Could not start the agent command %q: %v.", cfg.Command, err)
	}
	agent.Start()
	defer agent.Kill()
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	handshake, err := callBounded(ctx, func() (initializedHandshake, error) {
		var answer struct {
			ProtocolVersion   int             `json:"protocolVersion"`
			AgentCapabilities json.RawMessage `json:"agentCapabilities"`
			AuthMethods       json.RawMessage `json:"authMethods"`
		}
		if err := agent.Call("initialize", acpInitializeParams, &answer); err != nil {
			return initializedHandshake{}, err
		}
		return initializedHandshake{
			ProtocolVersion:   answer.ProtocolVersion,
			AgentCapabilities: answer.AgentCapabilities,
			auth:              answer.AuthMethods,
		}, nil
	})
	if err != nil {
		return catalog, modelError(err)
	}
	if handshake.ProtocolVersion != ProtocolVersion {
		return catalog, genui.AgentErrorf("The agent speaks ACP protocol version %d, but this app speaks version %d.", handshake.ProtocolVersion, ProtocolVersion)
	}
	var opened struct {
		SessionID     string          `json:"sessionId"`
		ConfigOptions json.RawMessage `json:"configOptions"`
		Models        json.RawMessage `json:"models"`
	}
	if _, err := callBounded(ctx, func() (struct{}, error) {
		return struct{}{}, agent.Call("session/new", map[string]any{
			"cwd":        dir,
			"mcpServers": []any{},
		}, &opened)
	}); err != nil {
		return catalog, modelError(err)
	}
	if opened.SessionID != "" && agentSupportsSessionClose(handshake.AgentCapabilities) {
		_ = closeSession(ctx, agent, opened.SessionID)
	}

	catalog.Models, catalog.CurrentModel, catalog.ThoughtLevels, catalog.ThoughtLevel = readCatalog(opened.ConfigOptions, opened.Models)
	return catalog, nil
}

func readCatalog(options, legacy json.RawMessage) (models []genui.ModelOption, current string, thoughtLevels []genui.Choice, thoughtLevel string) {
	if opt, ok := findConfigOptionIn(options, []string{traceModelOption}, []string{traceModelOption}); ok {
		for _, value := range opt.Options {
			id, _ := configString(value.Value)
			if id == "" {
				continue
			}
			models = append(models, genui.ModelOption{ID: id, Name: value.Name})
		}
		if value, ok := opt.current(); ok {
			current, _ = configString(value)
		}
	}
	if len(models) == 0 {
		for _, id := range legacyModelValues(legacy) {
			models = append(models, genui.ModelOption{ID: id, Name: id})
		}
	}
	if value, ok := legacyCurrent(legacy); ok {
		current = value
	}
	if opt, ok := findThoughtOption(options); ok {
		for _, value := range opt.Options {
			id, _ := configString(value.Value)
			if id == "" {
				continue
			}
			thoughtLevels = append(thoughtLevels, genui.Choice{Value: id, Name: value.Name})
		}
		if value, ok := opt.current(); ok {
			thoughtLevel, _ = configString(value)
		}
	}
	return models, current, thoughtLevels, thoughtLevel
}

func legacyCurrent(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var listed struct {
		CurrentModelID string `json:"currentModelId"`
	}
	if json.Unmarshal(raw, &listed) != nil || listed.CurrentModelID == "" {
		return "", false
	}
	return listed.CurrentModelID, true
}

func closeSession(ctx context.Context, agent *Agent, sessionID string) error {
	_, err := callBounded(ctx, func() (struct{}, error) {
		return struct{}{}, agent.Call("session/close", map[string]any{"sessionId": sessionID}, nil)
	})
	return err
}

func callBounded[T any](ctx context.Context, call func() (T, error)) (T, error) {
	type outcome struct {
		value T
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		value, err := call()
		done <- outcome{value, err}
	}()
	select {
	case out := <-done:
		return out.value, out.err
	case <-ctx.Done():
		var zero T
		return zero, genui.ErrCancelled
	}
}

func modelError(err error) error {
	if errors.Is(err, genui.ErrCancelled) {
		return err
	}
	var rpc *RPCError
	if errors.As(err, &rpc) {
		if genui.IsAuthError(rpc.Code, rpc.Message) {
			return genui.AuthRequired("", rpc.Message)
		}
		return genui.AgentErrorf("The agent answered with an error: %s", rpc.Message)
	}
	return genui.AgentError(err.Error())
}

func versionOf(ctx context.Context, command string, env []string) (string, string, error) {
	resolved, err := ResolveAgentCommand(command)
	if err != nil {
		return "", "", err
	}
	version, err := runVersion(ctx, resolved, env)
	return resolved, version, err
}

func runVersion(ctx context.Context, resolved string, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, resolved, "--version")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text != "" {
			return "", errors.New(text)
		}
		return "", err
	}
	if text == "" {
		return "", errors.New("it printed no version")
	}
	if line, _, ok := strings.Cut(text, "\n"); ok {
		text = strings.TrimSpace(line)
	}
	return text, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
