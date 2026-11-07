package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// set stores s, writing the file atomically. An empty agent command is
// stored as the default, and existing cached models are preserved when
// s.Models is empty.
func (st *settingsStore) set(s Settings) error {
	if s.AgentCommand == "" {
		s.AgentCommand = defaultSettings().AgentCommand
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dir == "" {
		return errors.New("The settings folder is not available.")
	}
	if cleaned := cleanModels(s.Models); len(cleaned) > 0 {
		s.Models = cleaned
	} else {
		s.Models = st.getLocked().Models
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(st.path(), append(data, '\n'))
}

// setModels caches the models offered by the agent on its handshake.
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
	return writeFileAtomic(st.path(), append(data, '\n'))
}
