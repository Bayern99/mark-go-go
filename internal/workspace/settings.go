package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/genui"
)

const settingsSchemaVersion = 2

// GenUIBackend is one backend's own configuration. A new backend starts from
// the registry defaults, so an untouched backend still knows how its CLI is
// launched.
type GenUIBackend struct {
	Command string `json:"command,omitempty"`
	// Args has no omitempty: an explicitly empty list is how a backend's
	// default arguments are removed, which nil still means "use the default".
	Args         []string `json:"args"`
	Model        string   `json:"model,omitempty"`
	Models       []string `json:"models,omitempty"`
	ThoughtLevel string   `json:"thoughtLevel,omitempty"`
}

type GenUISettings struct {
	Backend  string                  `json:"backend"`
	Backends map[string]GenUIBackend `json:"backends,omitempty"`
}

type Settings struct {
	SchemaVersion int           `json:"schemaVersion"`
	GenUI         GenUISettings `json:"genui"`
	// ExternalIPC opens the local socket external agents and the markgogo CLI
	// use. It is off until the user turns it on.
	ExternalIPC bool `json:"externalIPC,omitempty"`
}

type GenUIBackendPatch struct {
	Command      *string   `json:"command,omitempty"`
	Args         *[]string `json:"args,omitempty"`
	Model        *string   `json:"model,omitempty"`
	Models       *[]string `json:"models,omitempty"`
	ThoughtLevel *string   `json:"thoughtLevel,omitempty"`
}

type GenUIPatch struct {
	Backend  *string                      `json:"backend,omitempty"`
	Backends map[string]GenUIBackendPatch `json:"backends,omitempty"`
}

type SettingsPatch struct {
	GenUI       *GenUIPatch `json:"genui,omitempty"`
	ExternalIPC *bool       `json:"externalIPC,omitempty"`
}

var SettingsChanged = desktop.Event[Settings]{}

func defaultBackendSettings(id string) GenUIBackend {
	b, ok := genui.BackendByID(id)
	if !ok {
		return GenUIBackend{}
	}
	return GenUIBackend{
		Command: b.Command,
		Args:    slices.Clone(b.Args),
		Model:   b.DefaultModel,
		Models:  slices.Clone(b.DefaultModels),
	}
}

func defaultSettings() Settings {
	s := Settings{
		SchemaVersion: settingsSchemaVersion,
		GenUI:         GenUISettings{Backend: genui.BackendAntigravity, Backends: map[string]GenUIBackend{}},
	}
	for _, b := range genui.Backends {
		s.GenUI.Backends[b.ID] = defaultBackendSettings(b.ID)
	}
	return s
}

func DefaultSettings() Settings { return defaultSettings() }

// For returns a backend's settings, filling anything the file left out from the
// registry defaults so a partially written file still launches.
func (g GenUISettings) For(id string) GenUIBackend {
	id = genui.NormalizeBackendID(id)
	settings := defaultBackendSettings(id)
	stored, ok := g.Backends[id]
	if !ok {
		return settings
	}
	if command := strings.TrimSpace(stored.Command); command != "" {
		settings.Command = command
	}
	if stored.Args != nil {
		settings.Args = slices.Clone(stored.Args)
	}
	settings.Model = stored.Model
	settings.ThoughtLevel = stored.ThoughtLevel
	if len(stored.Models) > 0 {
		settings.Models = cleanModels(stored.Models)
	}
	return settings
}

// Active is the backend a generation runs on and its settings. The default
// backend is Antigravity, which is what settings.json v1 meant by "agy".
func (g GenUISettings) Active() (string, GenUIBackend) {
	id := genui.NormalizeBackendID(g.Backend)
	if id == "" {
		id = genui.BackendAntigravity
	}
	return id, g.For(id)
}

func cleanModels(models []string) []string {
	var out []string
	seen := make(map[string]bool, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

const settingsFile = "settings.json"

type settingsStore struct {
	mu       sync.Mutex
	dir      string
	onChange func(Settings)
	// detect names the backend a first run should use. Nil keeps Antigravity,
	// so a store built without a probe stays deterministic.
	detect func() string
}

func (st *settingsStore) path() string {
	return filepath.Join(st.dir, settingsFile)
}

func (st *settingsStore) firstBackend() string {
	if st.detect != nil {
		if id := genui.NormalizeBackendID(st.detect()); id != "" {
			if b, ok := genui.BackendByID(id); ok && b.Ready {
				return b.ID
			}
		}
	}
	return genui.BackendAntigravity
}

func (st *settingsStore) getLocked() Settings {
	s := defaultSettings()
	if st.dir == "" {
		return s
	}
	data, err := os.ReadFile(st.path())
	if err != nil {
		s.GenUI.Backend = st.firstBackend()
		return s
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		s.GenUI.Backend = st.firstBackend()
		return s
	}
	if raw, ok := fields["externalIPC"]; ok {
		_ = json.Unmarshal(raw, &s.ExternalIPC)
	}
	var raw json.RawMessage
	if value, ok := fields["genui"]; ok {
		raw = value
	}
	if len(raw) == 0 {
		// settings.json v1 kept the agent in top-level fields; they belonged to
		// agy, so they move to the Antigravity backend unchanged.
		migrateV1(s.GenUI.Backends, fields)
		s.GenUI.Backend = genui.BackendAntigravity
		return s
	}
	var stored GenUISettings
	if json.Unmarshal(raw, &stored) == nil {
		if id := genui.NormalizeBackendID(stored.Backend); id != "" {
			s.GenUI.Backend = id
		}
		for id, backend := range stored.Backends {
			id = genui.NormalizeBackendID(id)
			if _, known := genui.BackendByID(id); !known {
				continue
			}
			s.GenUI.Backends[id] = backend
		}
	}
	if b, ok := genui.BackendByID(s.GenUI.Backend); !ok || !b.Ready {
		s.GenUI.Backend = st.firstBackend()
	} else {
		s.GenUI.Backend = b.ID
	}
	return s
}

func migrateV1(backends map[string]GenUIBackend, fields map[string]json.RawMessage) {
	agy := backends[genui.BackendAntigravity]
	var legacy struct {
		Model        string   `json:"model"`
		AgentCommand string   `json:"agentCommand"`
		Models       []string `json:"models"`
	}
	if raw, ok := fields["model"]; ok {
		_ = json.Unmarshal(raw, &legacy.Model)
	}
	if raw, ok := fields["agentCommand"]; ok {
		_ = json.Unmarshal(raw, &legacy.AgentCommand)
	}
	if raw, ok := fields["models"]; ok {
		_ = json.Unmarshal(raw, &legacy.Models)
	}
	agy.Model = legacy.Model
	if command := strings.TrimSpace(legacy.AgentCommand); command != "" {
		agy.Command, agy.Args = genui.SplitCommand(command)
		if len(agy.Args) == 0 {
			agy.Args = slices.Clone(defaultBackendSettings(genui.BackendAntigravity).Args)
		}
	}
	if models := cleanModels(legacy.Models); len(models) > 0 {
		agy.Models = models
	}
	backends[genui.BackendAntigravity] = agy
}

func (st *settingsStore) get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.getLocked()
}

func (st *settingsStore) apply(p SettingsPatch) (Settings, error) {
	st.mu.Lock()
	if st.dir == "" {
		st.mu.Unlock()
		return Settings{}, errors.New("The settings folder is not available.")
	}
	s := st.getLocked()
	if p.ExternalIPC != nil {
		s.ExternalIPC = *p.ExternalIPC
	}
	if p.GenUI != nil {
		if err := applyGenUIPatch(&s.GenUI, *p.GenUI); err != nil {
			st.mu.Unlock()
			return Settings{}, err
		}
	}
	onChange, err := st.writeLocked(s)
	st.mu.Unlock()
	if err != nil {
		return Settings{}, err
	}
	if onChange != nil {
		onChange(s)
	}
	return s, nil
}

func applyGenUIPatch(g *GenUISettings, p GenUIPatch) error {
	if p.Backend != nil {
		id := genui.NormalizeBackendID(*p.Backend)
		b, ok := genui.BackendByID(id)
		if !ok {
			return errors.New("Unknown GenUI backend.")
		}
		if !b.Ready {
			return errors.New(b.Name + " is not available in this release.")
		}
		g.Backend = b.ID
	}
	for id, patch := range p.Backends {
		id = genui.NormalizeBackendID(id)
		if _, ok := genui.BackendByID(id); !ok {
			return errors.New("Unknown GenUI backend.")
		}
		backend := g.For(id)
		if patch.Command != nil {
			backend.Command = strings.TrimSpace(*patch.Command)
		}
		if patch.Args != nil {
			backend.Args = slices.Clone(*patch.Args)
		}
		if patch.Model != nil {
			backend.Model = strings.TrimSpace(*patch.Model)
		}
		if patch.Models != nil {
			backend.Models = cleanModels(*patch.Models)
		}
		if patch.ThoughtLevel != nil {
			backend.ThoughtLevel = strings.TrimSpace(*patch.ThoughtLevel)
		}
		if g.Backends == nil {
			g.Backends = map[string]GenUIBackend{}
		}
		g.Backends[id] = backend
	}
	return nil
}

// SetSelection records the model and effort an agent confirmed. Empty values
// are left as they are, so an agent that applies only the model does not erase
// the effort the user chose.
func (st *settingsStore) SetSelection(backendID, model, thoughtLevel string) error {
	st.mu.Lock()
	if st.dir == "" {
		st.mu.Unlock()
		return nil
	}
	s := st.getLocked()
	id := genui.NormalizeBackendID(backendID)
	if _, ok := genui.BackendByID(id); !ok {
		st.mu.Unlock()
		return nil
	}
	current := s.GenUI.For(id)
	changed := false
	if model = strings.TrimSpace(model); model != "" && current.Model != model {
		current.Model = model
		changed = true
	}
	if thoughtLevel = strings.TrimSpace(thoughtLevel); thoughtLevel != "" && current.ThoughtLevel != thoughtLevel {
		current.ThoughtLevel = thoughtLevel
		changed = true
	}
	if !changed {
		st.mu.Unlock()
		return nil
	}
	if s.GenUI.Backends == nil {
		s.GenUI.Backends = map[string]GenUIBackend{}
	}
	s.GenUI.Backends[id] = current
	onChange, err := st.writeLocked(s)
	st.mu.Unlock()
	if err != nil {
		return err
	}
	if onChange != nil {
		onChange(s)
	}
	return nil
}

// writeLocked persists committed settings and returns the listener to call
// after the lock is released.
func (st *settingsStore) writeLocked(s Settings) (func(Settings), error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := fsutil.WriteFileAtomic(st.path(), append(data, '\n')); err != nil {
		return nil, err
	}
	return st.onChange, nil
}

// SetModels records the models an agent declared for one backend. An empty
// answer changes nothing, so a failed refresh never empties the menu.
func (st *settingsStore) setModels(backendID string, models []string) error {
	cleaned := cleanModels(models)
	if st == nil || len(cleaned) == 0 {
		return nil
	}
	st.mu.Lock()
	if st.dir == "" {
		st.mu.Unlock()
		return nil
	}
	s := st.getLocked()
	id := genui.NormalizeBackendID(backendID)
	if _, ok := genui.BackendByID(id); !ok {
		st.mu.Unlock()
		return nil
	}
	backend := s.GenUI.For(id)
	backend.Models = cleaned
	if s.GenUI.Backends == nil {
		s.GenUI.Backends = map[string]GenUIBackend{}
	}
	s.GenUI.Backends[id] = backend
	onChange, err := st.writeLocked(s)
	st.mu.Unlock()
	if err != nil {
		return err
	}
	if onChange != nil {
		onChange(s)
	}
	return nil
}
