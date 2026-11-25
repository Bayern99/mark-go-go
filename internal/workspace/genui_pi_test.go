package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

// writePiScript puts an executable at dir/pi and returns its path. The scripts
// below stand in for the Pi CLI, which is the only system boundary these tests
// cannot run for real.
func writePiScript(t *testing.T, dir, script string) string {
	t.Helper()
	path := filepath.Join(dir, "pi")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// putPiOnPath makes dir's fake pi the first `pi` a lookup finds while keeping the
// rest of the PATH, so the script's own tools still work.
func putPiOnPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func piSettings(t *testing.T, command string) *settingsStore {
	t.Helper()
	store := &settingsStore{dir: t.TempDir()}
	backend := genui.BackendPi
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{
		Backend: &backend,
		Backends: map[string]GenUIBackendPatch{
			genui.BackendPi: {
				Command:  &command,
				Provider: new("anthropic"),
				Model:    new("claude-sonnet-4-5"),
			},
		},
	}}); err != nil {
		t.Fatalf("saving Pi settings: %v", err)
	}
	return store
}

func TestGenUILoginRunsPisSignInForTheRecordedProvider(t *testing.T) {
	var launched string
	s := &Service{
		settings: piSettings(t, "pi"),
		terminal: func(_ context.Context, line string) error {
			launched = line
			return nil
		},
	}
	if err := s.OpenGenUILogin(genui.BackendPi); err != nil {
		t.Fatalf("OpenGenUILogin: %v", err)
	}
	if launched != "pi auth login anthropic" {
		t.Fatalf("launched %q, want Pi signing in the recorded provider", launched)
	}
}

func TestGenUILoginWithoutAProviderAsksForAModelFirst(t *testing.T) {
	store := piSettings(t, "pi")
	empty := ""
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendPi: {Provider: &empty},
	}}}); err != nil {
		t.Fatalf("clearing the provider: %v", err)
	}
	s := &Service{settings: store, terminal: func(context.Context, string) error {
		t.Fatal("no sign-in may start without a provider")
		return nil
	}}
	err := s.OpenGenUILogin(genui.BackendPi)
	if err == nil || !strings.Contains(err.Error(), "pi auth login") {
		t.Fatalf("error = %v, want the hint that names the provider", err)
	}
}

func TestProbeGenUIMapsPisCredentialCheckExitCodes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLI is a shell script")
	}
	for _, tt := range []struct {
		name     string
		script   string
		want     genui.AgentStatus
		wantHint string
	}{
		{
			name:   "ready",
			script: "#!/bin/sh\necho '{\"status\":\"ready\",\"provider\":\"anthropic\"}'\nexit 0\n",
			want:   genui.AgentReady,
		},
		{
			name:     "needs a sign-in",
			script:   "#!/bin/sh\necho '{\"status\":\"not_ready\",\"provider\":\"anthropic\",\"reason\":\"credentials_not_configured\"}'\nexit 1\n",
			want:     genui.AgentLoginRequired,
			wantHint: "pi auth login anthropic",
		},
		{
			name:   "invalid",
			script: "#!/bin/sh\nexit 2\n",
			want:   genui.AgentFailed,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			writePiScript(t, bin, tt.script)
			putPiOnPath(t, bin)
			s := &Service{settings: piSettings(t, "pi")}
			result, err := s.ProbeGenUI(genui.BackendPi)
			if err != nil {
				t.Fatalf("ProbeGenUI: %v", err)
			}
			if result.Status != tt.want {
				t.Fatalf("status = %q (%s), want %q", result.Status, result.Detail, tt.want)
			}
			if tt.wantHint != "" && !strings.Contains(result.Hint, tt.wantHint) {
				t.Errorf("hint = %q, want it to name the sign-in command", result.Hint)
			}
		})
	}
}

func TestProbeGenUIWithoutAProviderClaimsNoLogin(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "ran")
	writePiScript(t, bin, "#!/bin/sh\ntouch "+marker+"\nexit 0\n")
	putPiOnPath(t, bin)
	store := piSettings(t, "pi")
	empty := ""
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendPi: {Provider: &empty},
	}}}); err != nil {
		t.Fatalf("clearing the provider: %v", err)
	}
	s := &Service{settings: store}
	result, err := s.ProbeGenUI(genui.BackendPi)
	if err != nil {
		t.Fatalf("ProbeGenUI: %v", err)
	}
	if result.Status != genui.AgentUnknown {
		t.Fatalf("status = %q, want an unknown login rather than a claim", result.Status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the probe ran Pi without a provider: %v", err)
	}
}

// piRPCScript answers a whole turn: the guard's status report, the model and
// level writes, and a prompt that writes a view before it settles.
const piRPCScript = `#!/bin/sh
printf '{"type":"extension_ui_request","id":"status","method":"setStatus","statusKey":"mgo-guard","statusText":"ready:%s:%s"}\n' "$MGO_GUARD_VERSION" "$MGO_GUARD_TOKEN"
while IFS= read -r line; do
  id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
  case "$line" in
    *'"type":"set_model"'*)
      printf '{"type":"response","id":"%s","command":"set_model","success":true,"data":{"id":"claude-sonnet-4-5"}}\n' "$id" ;;
    *'"type":"get_available_thinking_levels"'*)
      printf '{"type":"response","id":"%s","command":"get_available_thinking_levels","success":true,"data":{"levels":["off","low","high"]}}\n' "$id" ;;
    *'"type":"set_thinking_level"'*)
      printf '{"type":"response","id":"%s","command":"set_thinking_level","success":true}\n' "$id" ;;
    *'"type":"prompt"'*)
      printf '{"type":"response","id":"%s","command":"prompt","success":true,"data":{"disposition":"started"}}\n' "$id"
      cat > out.json <<'JSON'
{"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"s","catalogId":"markgogo.dev:genui/v3","components":["root = Text(\"Hi\")"]}}],"sources":{}}
JSON
      printf '{"type":"tool_execution_start","toolCallId":"c1","toolName":"read","args":{"path":"source.md"}}\n'
      printf '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Wrote it."}],"stopReason":"stop"}}\n'
      printf '{"type":"agent_settled","aborted":false}\n' ;;
  esac
done
`

func TestPiGenerationWritesTheViewInItsOwnTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLI is a shell script")
	}
	bin := t.TempDir()
	writePiScript(t, bin, piRPCScript)
	putPiOnPath(t, bin)
	dir := t.TempDir()
	docPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(docPath, []byte("# A\n\nThe source.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Service{settings: piSettings(t, "pi")}
	session := hostSession(t, h, "doc-1", docPath)

	var phases []string
	done := <-h.StartGeneration(context.Background(), session.id, genui.GenerateRequest{
		Text:     "# A\n\nThe source.\n",
		Mode:     "full",
		Format:   "a2ui",
		Language: genui.StatementLanguage,
	}, func(progress genui.GenerateProgress) error {
		phases = append(phases, progress.Phase)
		return nil
	})
	if done != nil {
		t.Fatalf("generation failed: %v", done)
	}
	view := genui.ViewPath(docPath)
	data, err := os.ReadFile(view)
	if err != nil {
		t.Fatalf("the view was not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("the view is empty")
	}
	for _, want := range []string{"starting", "reading", "checking"} {
		if !containsPhase(phases, want) {
			t.Errorf("phases = %v, want %q", phases, want)
		}
	}
}

func TestPiGenerationWithoutAGuardIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLI is a shell script")
	}
	bin := t.TempDir()
	writePiScript(t, bin, "#!/bin/sh\nwhile IFS= read -r line; do :; done\n")
	putPiOnPath(t, bin)
	dir := t.TempDir()
	docPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(docPath, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Service{settings: piSettings(t, "pi")}
	session := hostSession(t, h, "doc-1", docPath)
	start := time.Now()
	done := <-h.StartGeneration(context.Background(), session.id, genui.GenerateRequest{
		Text: "# A\n", Mode: "full", Format: "a2ui", Language: genui.StatementLanguage,
	}, func(genui.GenerateProgress) error { return nil })
	if done == nil {
		t.Fatal("a turn whose guard never reported in must not generate")
	}
	failure := genui.FailureOf(done)
	if !strings.Contains(failure.Message, "guard") {
		t.Fatalf("failure = %+v, want it to name the guard", failure)
	}
	if _, err := os.Stat(genui.ViewPath(docPath)); !os.IsNotExist(err) {
		t.Fatalf("a refused turn wrote a view: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Errorf("the refusal took %s, want it to fail fast", elapsed)
	}
}

func containsPhase(phases []string, want string) bool {
	for _, phase := range phases {
		if phase == want {
			return true
		}
	}
	return false
}

func TestPiKeepsItsOwnProviderAndModelInSettingsV2(t *testing.T) {
	dir := t.TempDir()
	store := &settingsStore{dir: dir}
	antigravityModel := "gemini-4-argon"
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{Backends: map[string]GenUIBackendPatch{
		genui.BackendAntigravity: {Model: &antigravityModel},
	}}}); err != nil {
		t.Fatalf("saving the Antigravity model: %v", err)
	}
	pi := genui.BackendPi
	if _, err := store.apply(SettingsPatch{GenUI: &GenUIPatch{
		Backend: &pi,
		Backends: map[string]GenUIBackendPatch{
			genui.BackendPi: {Provider: new("anthropic"), Model: new("claude-sonnet-4-5")},
		},
	}}); err != nil {
		t.Fatalf("saving Pi's provider and model: %v", err)
	}
	reopened := (&settingsStore{dir: dir}).get()
	id, backend := reopened.GenUI.Active()
	if id != genui.BackendPi {
		t.Fatalf("active backend = %q, want pi", id)
	}
	if backend.Provider != "anthropic" || backend.Model != "claude-sonnet-4-5" {
		t.Fatalf("Pi backend = %+v, want its own provider and model", backend)
	}
	if got := reopened.GenUI.For(genui.BackendAntigravity).Model; got != antigravityModel {
		t.Fatalf("Antigravity model = %q, want its own model untouched by Pi", got)
	}
}

func TestRefreshingPiModelsRecordsTheCurrentModelsProvider(t *testing.T) {
	dir := t.TempDir()
	s := &Service{settings: &settingsStore{dir: dir}}
	s.rememberCatalog(genui.BackendPi, genui.ModelCatalog{
		Models: []genui.ModelOption{
			{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "anthropic"},
			{ID: "gpt-5.4", Name: "GPT-5.4", Provider: "openai"},
		},
		CurrentModel: "gpt-5.4",
	})
	backend := (&settingsStore{dir: dir}).get().GenUI.For(genui.BackendPi)
	if backend.Provider != "openai" {
		t.Fatalf("provider = %q, want the current model's provider", backend.Provider)
	}
	if len(backend.Models) != 2 || backend.Models[1] != "gpt-5.4" {
		t.Fatalf("models = %v, want the refreshed list", backend.Models)
	}
}

func TestRefreshingPiKeepsBothProvidersForTheSameModelAfterRestart(t *testing.T) {
	dir := t.TempDir()
	s := &Service{settings: &settingsStore{dir: dir}}
	options := []genui.ModelOption{{ID: "shared", Provider: "a"}, {ID: "shared", Provider: "b"}}
	if err := s.rememberCatalog(genui.BackendPi, genui.ModelCatalog{
		Models: options, CurrentModel: "shared", CurrentProvider: "b",
	}); err != nil {
		t.Fatal(err)
	}
	reopened := (&settingsStore{dir: dir}).get().GenUI.For(genui.BackendPi)
	if reopened.Model != "shared" || reopened.Provider != "b" || reopened.UsableModel() != "shared" ||
		!slices.Equal(reopened.ModelOptions, options) {
		t.Fatalf("reopened Pi settings = %+v", reopened)
	}
}
