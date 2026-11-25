package genui

import "context"

// AgentStatus is what a Probe found out about one backend. AuthMethods in an
// ACP initialize answer lists the ways an agent can authenticate; it says
// nothing about whether that already happened, so only evidence moves a
// backend off AgentStatusUnknown.
type AgentStatus string

const (
	AgentReady         AgentStatus = "ready"
	AgentLoginRequired AgentStatus = "login-required"
	AgentUnknown       AgentStatus = "unknown"
	AgentOutdated      AgentStatus = "agent-outdated"
	AgentMissing       AgentStatus = "not-installed"
	AgentFailed        AgentStatus = "failed"
)

type ProbeResult struct {
	Status      AgentStatus `json:"status"`
	Executable  string      `json:"executable,omitempty"`
	Version     string      `json:"version,omitempty"`
	AuthMethods []string    `json:"authMethods,omitempty"`
	Detail      string      `json:"detail,omitempty"`
	Hint        string      `json:"hint,omitempty"`
}

// ModelOption is one model an agent offers. Provider is set by backends that
// name a provider beside the model, such as Pi, whose credential checks, model
// writes and Terminal login all take a provider rather than a model.
type ModelOption struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type Choice struct {
	Value string `json:"value"`
	Name  string `json:"name,omitempty"`
}

// ModelCatalog is what ListModels learned without submitting a generation
// request. ThoughtLevels are the levels the current model offers, because an
// ACP agent only declares that option once a model is applied.
type ModelCatalog struct {
	Models        []ModelOption `json:"models"`
	CurrentModel  string        `json:"currentModel,omitempty"`
	ThoughtLevels []Choice      `json:"thoughtLevels,omitempty"`
	ThoughtLevel  string        `json:"thoughtLevel,omitempty"`
}

// ProviderOf is the provider a model belongs to, as the catalog last reported
// it. Backends that name a provider beside the model use it for their
// credential check, their model write and their Terminal login.
func (c ModelCatalog) ProviderOf(model string) string {
	for _, option := range c.Models {
		if option.ID == model {
			return option.Provider
		}
	}
	return ""
}

type Turn struct {
	Dir          string
	Prompt       string
	Model        string
	ThoughtLevel string
	Progress     func(GenerateProgress) error
}

type TurnResult struct {
	Model string
	Reply string
}

// Runner is one agent backend's protocol adapter: everything the orchestrator
// needs to know about an agent, and nothing about ACP.
type Runner interface {
	Probe(ctx context.Context) (ProbeResult, error)
	ListModels(ctx context.Context) (ModelCatalog, error)
	ExecuteTurn(ctx context.Context, turn Turn) (TurnResult, error)
	Close() error
}
