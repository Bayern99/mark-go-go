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

type Settings struct {
	Model        string   `json:"model"`
	AgentCommand string   `json:"agentCommand"`
	Models       []string `json:"models,omitempty"`
}

type SettingsPatch struct {
	Model        *string `json:"model,omitempty"`
	AgentCommand *string `json:"agentCommand,omitempty"`
}

var settingsChanged = mygo.NewEvent[Settings]("settingsChanged")

var defaultModels = []string{"gemini-3.8-flash", "gemini-4-argon", "claude-opus-5-5"}

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

const settingsFile = "settings.json"

type settingsStore struct {
	mu       sync.Mutex
	dir      string
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

func (st *settingsStore) get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.getLocked()
}

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

func (st *settingsStore) announceLocked(s Settings) {
	if st.onChange != nil {
		st.onChange(s)
	}
}

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
