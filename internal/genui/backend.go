package genui

import "strings"

const (
	BackendCodex       = "codex"
	BackendAntigravity = "antigravity"
	BackendCustom      = "custom"
	BackendPi          = "pi"
)

// Backend describes one GenUI agent backend: the executable it starts, the
// arguments that make it speak ACP over stdio, and the separate entry points
// for its version and its Terminal login. Ready is false while the backend has
// no Runner yet; its card stays in Settings without being selectable.
type Backend struct {
	ID            string
	Name          string
	Command       string
	Args          []string
	LoginCommand  string
	LoginArgs     []string
	InstallHint   string
	LoginHint     string
	Ready         bool
	DefaultModel  string
	DefaultModels []string
}

var Backends = []Backend{
	{
		ID:           BackendCodex,
		Name:         "Codex",
		Command:      "codex-acp",
		LoginCommand: "codex-acp",
		LoginArgs:    []string{"login"},
		InstallHint:  "Install the Codex CLI and codex-acp, then try again.",
		LoginHint:    "Run `codex-acp login` in Terminal, then choose Test & Refresh.",
		Ready:        true,
	},
	{
		ID:            BackendAntigravity,
		Name:          "Antigravity",
		Command:       "agy",
		Args:          []string{"acp"},
		LoginCommand:  "agy",
		LoginArgs:     []string{"acp", "--login"},
		InstallHint:   "Install the Antigravity CLI, which provides agy, or set the agent command to its full path.",
		LoginHint:     "Run `agy acp --login` in Terminal, then choose Test & Refresh.",
		Ready:         true,
		DefaultModel:  "gemini-4-hydrogen",
		DefaultModels: []string{"gemini-4-hydrogen", "gemini-4-argon", "claude-opus-5-5"},
	},
	{
		ID:          BackendCustom,
		Name:        "Custom ACP",
		InstallHint: "Set the command and arguments of an agent that speaks the Agent Client Protocol over stdio.",
		LoginHint:   "Sign in with the agent's own command in Terminal, then choose Test & Refresh.",
		Ready:       true,
	},
	{
		ID:           BackendPi,
		Name:         "Pi",
		Command:      "pi",
		LoginCommand: "pi",
		LoginArgs:    []string{"auth", "login"},
		InstallHint:  "Install the Pi CLI so `pi` runs in Terminal, or set its full path in Agent settings.",
		LoginHint:    "Choose a model so this app knows its provider, then run `pi auth login <provider>` in Terminal.",
		Ready:        true,
	},
}

// ProbeOrder is the order a first run looks for an installed backend.
var ProbeOrder = []string{BackendCodex, BackendPi, BackendAntigravity}

func BackendByID(id string) (Backend, bool) {
	id = NormalizeBackendID(id)
	for _, b := range Backends {
		if b.ID == id {
			return b, true
		}
	}
	return Backend{}, false
}

// NormalizeBackendID maps the names a settings file may hold onto the ids this
// app writes. "agy" was the Antigravity CLI's name in settings.json v1.
func NormalizeBackendID(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "":
		return ""
	case "agy", BackendAntigravity:
		return BackendAntigravity
	default:
		return strings.ToLower(strings.TrimSpace(id))
	}
}

// DetectBackend is the first-run probe: the first backend in ProbeOrder whose
// executable resolves and which has a Runner in this release.
func DetectBackend(resolve func(command string) bool) string {
	if resolve == nil {
		return ""
	}
	for _, id := range ProbeOrder {
		b, ok := BackendByID(id)
		if !ok || !b.Ready || b.Command == "" {
			continue
		}
		if resolve(b.Command) {
			return b.ID
		}
	}
	return ""
}

// SplitCommand turns a command the user typed into the executable and its
// arguments, so a launch never hands exec a command line it cannot resolve.
func SplitCommand(line string) (string, []string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// SplitModelID separates the effort a compound model id carries, such as
// "gpt-6-sol[high]", from the model's own id. ACP agents declare the model and
// its reasoning effort as separate options; only the legacy model path accepts
// the compound form.
func SplitModelID(id string) (model, effort string) {
	open := strings.LastIndex(id, "[")
	if open <= 0 || !strings.HasSuffix(id, "]") {
		return id, ""
	}
	effort = strings.TrimSpace(id[open+1 : len(id)-1])
	if effort == "" {
		return id, ""
	}
	return id[:open], effort
}
