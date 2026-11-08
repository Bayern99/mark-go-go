package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// Settings is the generation settings the page can read and change:
// which model the agent uses, what command starts it, and the models
// the agent offered on its last handshake.
type Settings struct {
	// Model is the model the agent is asked to use, "gemini-3.8-flash"
	// by default. Empty means: let the agent keep its own model.
	Model string `json:"model"`
	// AgentCommand is the command that starts the agent, "agy" by
	// default: a name looked up on PATH, or a path to the executable.
	AgentCommand string `json:"agentCommand"`
	// Models are the models the agent offered on its last handshake, or
	// the default fallback list before any handshake has succeeded.
	Models []string `json:"models,omitempty"`
}

// SettingsPatch is the reader's intent: only the fields that are present
// are written, so two windows changing different settings at once keep both
// changes. An empty model means the agent's own choice; an empty command
// means the default command. Cached models are never written this way:
// they belong to the agent's handshake.
type SettingsPatch struct {
	Model        *string `json:"model,omitempty"`
	AgentCommand *string `json:"agentCommand,omitempty"`
}

// settingsChanged tells every window's page the committed settings after a
// write, since the settings file is one process-wide store, not per document.
var settingsChanged = mygo.NewEvent[Settings]("settingsChanged")

var defaultModels = []string{"gemini-3.8-flash", "gemini-4-argon", "claude-opus-5-5"}

// defaultSettings are the settings when there is no settings file yet.
func defaultSettings() Settings {
	return Settings{
		Model:        "gemini-3.8-flash",
		AgentCommand: "agy",
		Models:       slices.Clone(defaultModels),
	}
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

// settingsFile is the name of the settings file in the store's
// directory.
const settingsFile = "settings.json"

// settingsStore keeps Settings in settings.json in one directory: the
// app's user data directory, or a temporary one in tests. Safe for
// concurrent use.
type settingsStore struct {
	mu  sync.Mutex
	dir string
	// onChange, when set, is called with the committed settings after every
	// successful write, outside the lock: the one store's writes reach every
	// window's page.
	onChange func(Settings)
}

func (st *settingsStore) path() string {
	return filepath.Join(st.dir, settingsFile)
}

func (st *settingsStore) getLocked() Settings {
	s := defaultSettings()
	if st.dir == "" {
		return s
	}
	data, err := os.ReadFile(st.path())
	if err != nil {
		return s
	}
	// Read the fields by name, so that a file without the model
	// keeps the default, and an empty model stays empty.
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return s
	}
	if raw, ok := fields["model"]; ok {
		_ = json.Unmarshal(raw, &s.Model)
	}
	if raw, ok := fields["agentCommand"]; ok {
		_ = json.Unmarshal(raw, &s.AgentCommand)
	}
	if s.AgentCommand == "" {
		s.AgentCommand = defaultSettings().AgentCommand
	}
	if raw, ok := fields["models"]; ok {
		var models []string
		if json.Unmarshal(raw, &models) == nil {
			if cleaned := cleanModels(models); len(cleaned) > 0 {
				s.Models = cleaned
			}
		}
	}
	return s
}

// get returns the settings: the file's values, or the defaults when
// there is no file, or the file cannot be read or parsed. A stored
// empty model stays empty: it tells Generate not to choose a model. An
// empty agent command comes back as the default.
func (st *settingsStore) get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.getLocked()
}

// apply writes the patch's fields, leaving every other setting and the
// cached models as they are, and returns the committed settings. An empty
// model resets the choice, an empty agent command restores the default. The
// write is atomic: two windows changing different fields both keep theirs.
// The change is announced under the lock, so windows hear writes in the
// order they were committed.
func (st *settingsStore) apply(p SettingsPatch) (Settings, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dir == "" {
		return Settings{}, errors.New("The settings folder is not available.")
	}
	s := st.getLocked()
	if p.Model != nil {
		s.Model = *p.Model
	}
	if p.AgentCommand != nil {
		s.AgentCommand = *p.AgentCommand
	}
	if s.AgentCommand == "" {
		s.AgentCommand = defaultSettings().AgentCommand
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return Settings{}, err
	}
	if err := writeFileAtomic(st.path(), append(data, '\n')); err != nil {
		return Settings{}, err
	}
	st.announceLocked(s)
	return s, nil
}

// announceLocked tells the store's follower the committed settings. The
// caller holds st.mu, and announcing does not call back into the store.
func (st *settingsStore) announceLocked(s Settings) {
	if st.onChange != nil {
		st.onChange(s)
	}
}

// setModels caches the models offered by the agent on its handshake, which
// is the only writer of the cached list.
func (st *settingsStore) setModels(models []string) error {
	cleaned := cleanModels(models)
	if st == nil || len(cleaned) == 0 {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dir == "" {
		return nil
	}
	s := st.getLocked()
	s.Models = cleaned
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(st.path(), append(data, '\n')); err != nil {
		return err
	}
	st.announceLocked(s)
	return nil
}
