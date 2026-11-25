package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

func TestSettingsStore(t *testing.T) {
	dir := t.TempDir()
	st := &settingsStore{dir: dir}
	wantDefaults := GenUIBackend{
		Command: "agy",
		Args:    []string{"acp"},
		Model:   "gemini-4-hydrogen",
		Models:  []string{"gemini-4-hydrogen", "gemini-4-argon", "claude-opus-5-5"},
	}
	if id, settings := st.get().GenUI.Active(); id != genui.BackendAntigravity || !reflect.DeepEqual(settings, wantDefaults) {
		t.Errorf("defaults = %q %+v, want antigravity %+v", id, settings, wantDefaults)
	}
	backend := GenUIBackend{Model: "m-1", Command: "/usr/bin/agy", Args: []string{"acp"}, Models: wantDefaults.Models}
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &backend.Model, Command: &backend.Command},
	}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := (&settingsStore{dir: dir}).get()
	if id, settings := got.GenUI.Active(); id != genui.BackendAntigravity || !reflect.DeepEqual(settings, backend) {
		t.Errorf("round trip = %+v, want %+v", settings, backend)
	}
	if got.SchemaVersion != 2 {
		t.Errorf("stored schemaVersion = %d, want 2", got.SchemaVersion)
	}
	if err := st.SetModels(genui.BackendAntigravity, []string{"claude-opus-5-5", "gemini-3.8-flash", "claude-opus-5-5", ""}); err != nil {
		t.Fatalf("SetModels: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get().GenUI.For(genui.BackendAntigravity); !reflect.DeepEqual(got.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("cached models = %+v", got.Models)
	}
	empty := ""
	command := ""
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &empty, Command: &command},
	}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got = (&settingsStore{dir: dir}).get()
	if id, settings := got.GenUI.Active(); id != genui.BackendAntigravity || settings.Model != "" || settings.Command != "agy" || !reflect.DeepEqual(settings.Models, []string{"claude-opus-5-5", "gemini-3.8-flash"}) {
		t.Errorf("empty model and command with stale frontend models = %+v", settings)
	}

	noDir := &settingsStore{}
	if got := noDir.get(); !reflect.DeepEqual(got, defaultSettings()) {
		t.Errorf("store without a directory = %+v", got)
	}
	if _, err := noDir.apply(SettingsPatch{ExternalIPC: new(true)}); err == nil {
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

	model := "claude-opus-5-5"
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &model},
	}}}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	command := "/opt/homebrew/bin/agy"
	committed, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Command: &command},
	}}})
	if err != nil {
		t.Fatalf("apply command: %v", err)
	}
	want := GenUIBackend{Command: command, Args: []string{"acp"}, Model: model, Models: defaultBackendSettings(genui.BackendAntigravity).Models}
	if id, settings := committed.GenUI.Active(); id != genui.BackendAntigravity || !reflect.DeepEqual(settings, want) {
		t.Errorf("committed = %+v, want %+v", settings, want)
	}
	if got := (&settingsStore{dir: dir}).get(); !reflect.DeepEqual(got.GenUI.For(genui.BackendAntigravity), want) {
		t.Errorf("stored = %+v, want %+v", got.GenUI.For(genui.BackendAntigravity), want)
	}
	if len(announced) != 2 {
		t.Fatalf("announcements = %d, want 2: %+v", len(announced), announced)
	}

	if err := st.SetModels(genui.BackendAntigravity, []string{"model-alpha", "model-beta"}); err != nil {
		t.Fatalf("SetModels: %v", err)
	}
	beta := "model-beta"
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &beta},
	}}}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	last := announced[len(announced)-1].GenUI.For(genui.BackendAntigravity)
	if last.Model != "model-beta" || !reflect.DeepEqual(last.Models, []string{"model-alpha", "model-beta"}) {
		t.Errorf("after handshake and model change = %+v", last)
	}
	empty := ""
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &empty, Command: &empty},
	}}}); err != nil {
		t.Fatalf("apply empties: %v", err)
	}
	emptied := announced[len(announced)-1].GenUI.For(genui.BackendAntigravity)
	if emptied.Model != "" || emptied.Command != "agy" {
		t.Errorf("announced empties = %+v", emptied)
	}
}

func TestSettingsPatchesFromTwoWindowsBothSurvive(t *testing.T) {
	st := &settingsStore{dir: t.TempDir()}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			model := "claude-opus-5-5"
			if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
				genui.BackendAntigravity: {Model: &model},
			}}}); err != nil {
				t.Errorf("apply model: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			command := "/opt/bin/agy"
			if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
				genui.BackendAntigravity: {Command: &command},
			}}}); err != nil {
				t.Errorf("apply command: %v", err)
			}
		}()
	}
	wg.Wait()
	got := st.get().GenUI.For(genui.BackendAntigravity)
	if got.Model != "claude-opus-5-5" || got.Command != "/opt/bin/agy" {
		t.Errorf("after concurrent patches = %+v", got)
	}
}

func TestExternalIPCIsOffUntilItIsTurnedOn(t *testing.T) {
	dir := t.TempDir()
	st := &settingsStore{dir: dir}
	if st.get().ExternalIPC {
		t.Fatal("the external IPC channel is on by default")
	}
	if _, err := st.apply(SettingsPatch{ExternalIPC: new(true)}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !(&settingsStore{dir: dir}).get().ExternalIPC {
		t.Fatal("the external IPC channel did not stay enabled")
	}
	model := "m-2"
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &model},
	}}}); err != nil {
		t.Fatalf("apply model: %v", err)
	}
	if !(&settingsStore{dir: dir}).get().ExternalIPC {
		t.Fatal("a model change turned the external IPC channel off")
	}
	if _, err := st.apply(SettingsPatch{ExternalIPC: new(false)}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if (&settingsStore{dir: dir}).get().ExternalIPC {
		t.Fatal("the external IPC channel did not stay disabled")
	}
}

func TestSettingsV1MigratesToTheAntigravityBackend(t *testing.T) {
	dir := t.TempDir()
	legacy := `{
  "model": "claude-opus-5-5",
  "agentCommand": "/opt/homebrew/bin/agy",
  "models": ["claude-opus-5-5", "gemini-3.8-flash"],
  "externalIPC": true
}
`
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (&settingsStore{dir: dir, detect: func() string { return genui.BackendCodex }}).get()
	id, settings := got.GenUI.Active()
	if id != genui.BackendAntigravity {
		t.Fatalf("v1 settings picked %q, want the agy backend", id)
	}
	want := GenUIBackend{
		Command: "/opt/homebrew/bin/agy",
		Args:    []string{"acp"},
		Model:   "claude-opus-5-5",
		Models:  []string{"claude-opus-5-5", "gemini-3.8-flash"},
	}
	if !reflect.DeepEqual(settings, want) {
		t.Fatalf("migrated = %+v, want %+v", settings, want)
	}
	if !got.ExternalIPC {
		t.Error("migration dropped the external IPC switch")
	}
}

func TestSettingsV1CommandKeepsItsArguments(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(`{"agentCommand": "agy acp --debug"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, settings := (&settingsStore{dir: dir}).get().GenUI.Active()
	if settings.Command != "agy" || !reflect.DeepEqual(settings.Args, []string{"acp", "--debug"}) {
		t.Fatalf("migrated command = %q %v", settings.Command, settings.Args)
	}
}

func TestFirstRunPicksTheFirstInstalledBackendWithARunner(t *testing.T) {
	for _, tt := range []struct {
		name   string
		detect func() string
		want   string
	}{
		{name: "codex", detect: func() string { return genui.BackendCodex }, want: genui.BackendCodex},
		{name: "nothing installed", detect: func() string { return "" }, want: genui.BackendAntigravity},
		{name: "no probe", detect: nil, want: genui.BackendAntigravity},
		{name: "pi", detect: func() string { return genui.BackendPi }, want: genui.BackendPi},
		{name: "legacy alias", detect: func() string { return "agy" }, want: genui.BackendAntigravity},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := (&settingsStore{dir: t.TempDir(), detect: tt.detect}).get()
			if id, _ := got.GenUI.Active(); id != tt.want {
				t.Fatalf("first run picked %q, want %q", id, tt.want)
			}
			if tt.want == genui.BackendCodex {
				if _, settings := got.GenUI.Active(); settings.Command != "codex-acp" || len(settings.Args) != 0 {
					t.Fatalf("codex defaults = %q %v, want codex-acp with no arguments", settings.Command, settings.Args)
				}
			}
		})
	}
}

func TestSettingsV2KeepsBackendsApart(t *testing.T) {
	dir := t.TempDir()
	stored := `{
  "schemaVersion": 2,
  "genui": {
    "backend": "codex",
    "backends": {
      "codex": {"command": "codex-acp", "model": "gpt-6-sol", "thoughtLevel": "high"},
      "agy": {"command": "/custom/agy", "args": ["acp"], "model": "gemini-4-argon"},
      "unknown": {"command": "/nope"}
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(stored), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (&settingsStore{dir: dir}).get()
	if got.GenUI.Backend != genui.BackendCodex {
		t.Fatalf("backend = %q, want codex", got.GenUI.Backend)
	}
	if _, settings := got.GenUI.Active(); settings.Model != "gpt-6-sol" || settings.ThoughtLevel != "high" || len(settings.Args) != 0 {
		t.Fatalf("codex settings = %+v", settings)
	}
	if agy := got.GenUI.For("agy"); agy.Command != "/custom/agy" || agy.Model != "gemini-4-argon" {
		t.Fatalf("agy alias = %+v", agy)
	}
	for id := range got.GenUI.Backends {
		if id == "unknown" {
			t.Fatal("an unknown backend id survived the read")
		}
	}
	store := &settingsStore{dir: dir}
	pi := genui.BackendPi
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{Backend: &pi}}); err != nil {
		t.Fatalf("selecting Pi: %v", err)
	}
	nonsense := "nonsense"
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{Backend: &nonsense}}); err == nil {
		t.Fatal("selecting an unknown backend succeeded")
	}
}

func TestClearingABackendsArgumentsSticks(t *testing.T) {
	dir := t.TempDir()
	st := &settingsStore{dir: dir}
	empty := []string{}
	if _, err := st.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Args: &empty},
	}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := (&settingsStore{dir: dir}).get().GenUI.For(genui.BackendAntigravity).Args; len(got) != 0 {
		t.Fatalf("args = %v, want the explicitly empty list to survive a reload", got)
	}
}

func TestNewServiceDetectsTheFirstInstalledBackendOnAnEmptyProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses Unix executable fixtures")
	}
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for _, command := range []string{"codex-acp", "agy", "pi"} {
		if err := os.WriteFile(filepath.Join(bin, command), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService()
	svc.settings.dir = t.TempDir()
	settings, err := svc.GetSettings()
	if err != nil || settings.GenUI.Backend != genui.BackendCodex {
		t.Fatalf("first backend = %q, %v, want Codex", settings.GenUI.Backend, err)
	}
}
