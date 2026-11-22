package workspace

import (
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestSettingsStore(t *testing.T) {
	dir := t.TempDir()
	st := &settingsStore{dir: dir}
	wantDefaults := Settings{
		Model:        "gemini-4-hydrogen",
		AgentCommand: "agy",
		Models:       []string{"gemini-4-hydrogen", "gemini-4-argon", "claude-opus-5-5"},
	}
	if got := st.get(); !reflect.DeepEqual(got, wantDefaults) {
		t.Errorf("defaults = %+v, want %+v", got, wantDefaults)
	}
	if _, err := st.apply(SettingsPatch{Model: new("m-1"), AgentCommand: new("/usr/bin/agy")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got, Settings{
		Model:        "m-1",
		AgentCommand: "/usr/bin/agy",
		Models:       wantDefaults.Models,
	}) {
		t.Errorf("round trip = %+v", got)
	}
	if err := st.SetModels([]string{"claude-opus-5-5", "gemini-3.8-flash", "claude-opus-5-5", ""}); err != nil {
		t.Fatalf("SetModels: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("cached models = %+v", got.Models)
	}
	if _, err := st.apply(SettingsPatch{Model: new(""), AgentCommand: new("")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get(); got.Model != "" || got.AgentCommand != "agy" || !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("empty model and command with stale frontend models = %+v", got)
	}

	empty := &settingsStore{}
	if got := empty.get(); !reflect.DeepEqual(got, defaultSettings()) {
		t.Errorf("store without a directory = %+v", got)
	}
	if _, err := empty.apply(SettingsPatch{Model: new("m-1")}); err == nil {
		t.Errorf("apply without a directory succeeded")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != settingsFile {
			t.Errorf("stray file %q in the settings directory", e.Name())
		}
	}
}

func TestSettingsPatchKeepsUntouchedFields(t *testing.T) {
	dir := t.TempDir()
	var announced []Settings
	st := &settingsStore{dir: dir, onChange: func(s Settings) { announced = append(announced, s) }}

	if _, err := st.apply(SettingsPatch{Model: new("claude-opus-5-5")}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	committed, err := st.apply(SettingsPatch{AgentCommand: new("/opt/homebrew/bin/agy")})
	if err != nil {
		t.Fatalf("apply command: %v", err)
	}
	want := Settings{
		Model:        "claude-opus-5-5",
		AgentCommand: "/opt/homebrew/bin/agy",
		Models:       defaultModels,
	}
	if !reflect.DeepEqual(committed, want) {
		t.Errorf("committed = %+v, want %+v", committed, want)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got, want) {
		t.Errorf("stored = %+v, want %+v", got, want)
	}
	if len(announced) != 2 {
		t.Fatalf("announcements = %d, want 2: %+v", len(announced), announced)
	}
	if !reflect.DeepEqual(announced[1], want) {
		t.Errorf("last announcement = %+v, want %+v", announced[1], want)
	}

	if err := st.SetModels([]string{"model-alpha", "model-beta"}); err != nil {
		t.Fatalf("SetModels: %v", err)
	}
	if _, err := st.apply(SettingsPatch{Model: new("model-beta")}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	last := announced[len(announced)-1]
	if last.Model != "model-beta" || !reflect.DeepEqual(last.Models, []string{"model-alpha", "model-beta"}) {
		t.Errorf("after handshake and model change = %+v", last)
	}
	empty, err := st.apply(SettingsPatch{Model: new(""), AgentCommand: new("")})
	if err != nil {
		t.Fatalf("apply empties: %v", err)
	}
	if empty.Model != "" || empty.AgentCommand != "agy" {
		t.Errorf("empties = %+v", empty)
	}
	if last := announced[len(announced)-1]; last.Model != "" || last.AgentCommand != "agy" {
		t.Errorf("announced empties = %+v", last)
	}
}

func TestSettingsPatchesFromTwoWindowsBothSurvive(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := st.apply(SettingsPatch{Model: new("claude-opus-5-5")}); err != nil {
				t.Errorf("apply model: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := st.apply(SettingsPatch{AgentCommand: new("/opt/bin/agy")}); err != nil {
				t.Errorf("apply command: %v", err)
			}
		}()
	}
	wg.Wait()
	got := st.get()
	if got.Model != "claude-opus-5-5" || got.AgentCommand != "/opt/bin/agy" {
		t.Errorf("after concurrent patches = %+v", got)
	}
}
