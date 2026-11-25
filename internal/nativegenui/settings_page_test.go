package nativegenui

import (
	"errors"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// settingsPanel is a panel whose saving behaves like the settings store, so a
// click can be observed through the page it redraws.
func settingsPanel(t *testing.T, settings workspace.Settings) (*panel, *[]workspace.SettingsPatch) {
	t.Helper()
	saved := &[]workspace.SettingsPatch{}
	p := &panel{settings: settings, settingsMessage: ""}
	p.save = func(patch workspace.SettingsPatch) (workspace.Settings, error) {
		*saved = append(*saved, patch)
		applyTestSettings(&p.settings, patch)
		return p.settings, nil
	}
	return p, saved
}

// applyTestSettings mirrors the store's patch rules for the fields the page
// writes. The store's own rules are covered by its tests.
func applyTestSettings(settings *workspace.Settings, patch workspace.SettingsPatch) {
	if patch.GenUI == nil {
		return
	}
	if patch.GenUI.Backend != nil {
		settings.GenUI.Backend = *patch.GenUI.Backend
	}
	for id, backendPatch := range patch.GenUI.Backends {
		backend := settings.GenUI.For(id)
		if backendPatch.Command != nil {
			backend.Command = *backendPatch.Command
		}
		if backendPatch.Args != nil {
			backend.Args = *backendPatch.Args
		}
		if backendPatch.Model != nil {
			backend.Model = *backendPatch.Model
		}
		if backendPatch.Models != nil {
			backend.Models = *backendPatch.Models
		}
		if backendPatch.ThoughtLevel != nil {
			backend.ThoughtLevel = *backendPatch.ThoughtLevel
		}
		if settings.GenUI.Backends == nil {
			settings.GenUI.Backends = map[string]workspace.GenUIBackend{}
		}
		settings.GenUI.Backends[id] = backend
	}
}

func pageTester(p *panel, query string) *ui.Tester {
	page := p.settingsPage()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() {
			page.Draw(c, nativeui.SettingsQuery{Text: query})
		})
	}, 640, 900)
	app.Frame()
	return app
}

func TestGenUISettingsOffersFourBackendsAndKeepsPiUnavailable(t *testing.T) {
	p, saved := settingsPanel(t, workspace.DefaultSettings())
	app := pageTester(p, "")

	for _, name := range []string{"Codex", "Antigravity", "Custom ACP", "Pi"} {
		if !app.HasText(name) {
			t.Fatalf("the backend cards are missing %q: %q", name, app.Texts())
		}
	}
	if !app.HasText("Coming soon") {
		t.Fatalf("Pi is offered as usable before it has a runner: %q", app.Texts())
	}
	if info, ok := app.Access("Pi backend"); !ok || !info.Disabled {
		t.Fatalf("Pi card = %+v, %t; want a disabled card", info, ok)
	}
	if err := app.Click("Codex backend"); err != nil {
		t.Fatalf("clicking the Codex card: %v", err)
	}
	app.Frame()
	if len(*saved) != 1 || (*saved)[0].GenUI == nil || (*saved)[0].GenUI.Backend == nil || *(*saved)[0].GenUI.Backend != genui.BackendCodex {
		t.Fatalf("saved patches = %+v, want the codex backend selected", *saved)
	}
	if id, _ := p.settings.GenUI.Active(); id != genui.BackendCodex {
		t.Fatalf("active backend = %q, want codex", id)
	}
	if err := app.Click("Pi backend"); err == nil && len(*saved) != 1 {
		t.Fatalf("clicking Pi saved a patch: %+v", *saved)
	}
}

func TestTestAndRefreshReportsTheBackendAndLoadsItsModels(t *testing.T) {
	p, _ := settingsPanel(t, workspace.DefaultSettings())
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	p.runOnUI = func(fn func()) { fn(); close(done) }
	p.probe = func(id string) (genui.ProbeResult, error) {
		close(started)
		<-release
		return genui.ProbeResult{Status: genui.AgentUnknown, Version: "codex-acp 1.13.1"}, nil
	}
	p.refresh = func(id string) (genui.ModelCatalog, error) {
		return genui.ModelCatalog{
			Models:        []genui.ModelOption{{ID: "gpt-6-sol"}, {ID: "gpt-6-luna"}},
			CurrentModel:  "gpt-6-sol",
			ThoughtLevels: []genui.Choice{{Value: "low"}, {Value: "high"}},
			ThoughtLevel:  "high",
		}, nil
	}
	app := pageTester(p, "")
	clickButton(t, app, "Test & Refresh")
	<-started
	if !p.status(genui.BackendAntigravity).busy {
		t.Fatal("the card does not show that the test is running")
	}
	close(release)
	<-done
	app.Frame()

	state := p.status(genui.BackendAntigravity)
	if state.probe.Status != genui.AgentUnknown {
		t.Fatalf("status = %+v, want the unknown login state the probe reported", state)
	}
	if !app.HasText("Login unknown") {
		t.Fatalf("the card does not show the probe result: %q", app.Texts())
	}
	if got := state.catalog.Models; len(got) != 2 || got[0].ID != "gpt-6-sol" {
		t.Fatalf("refreshed models = %+v, want the two the agent declared", got)
	}
	if !app.HasText("The backend no longer offers") {
		t.Fatalf("a saved model outside the refreshed list is not reported: %q", app.Texts())
	}
	trigger, ok := app.AccessByRole(ui.RoleMenuButton, "Model")
	if !ok {
		t.Fatalf("the model row has no menu: %q", app.Texts())
	}
	app.ClickAt(trigger.Bounds.X+trigger.Bounds.W/2, trigger.Bounds.Y+trigger.Bounds.H/2)
	app.Frame()
	if !app.HasText("gpt-6-sol") || !app.HasText("gpt-6-luna") {
		t.Fatalf("the refreshed models are not on offer: %q", app.Texts())
	}
}

func TestReasoningEffortAppearsOnlyForAModelThatOffersIt(t *testing.T) {
	settings := workspace.DefaultSettings()
	backend := settings.GenUI.For(genui.BackendAntigravity)
	backend.Model = "gpt-6-astra"
	settings.GenUI.Backends[genui.BackendAntigravity] = backend
	p, _ := settingsPanel(t, settings)
	p.status(genui.BackendAntigravity).catalog = genui.ModelCatalog{
		Models:       []genui.ModelOption{{ID: "gpt-6-astra"}},
		CurrentModel: "gpt-6-astra",
	}
	if app := pageTester(p, ""); app.HasText("Reasoning effort") {
		t.Fatalf("a model without an effort option still shows the choice: %q", app.Texts())
	}

	backend.Model = "gpt-6-sol"
	settings.GenUI.Backends[genui.BackendAntigravity] = backend
	p, _ = settingsPanel(t, settings)
	p.status(genui.BackendAntigravity).catalog = genui.ModelCatalog{
		Models:        []genui.ModelOption{{ID: "gpt-6-sol"}},
		CurrentModel:  "gpt-6-sol",
		ThoughtLevels: []genui.Choice{{Value: "low"}, {Value: "high"}},
	}
	if app := pageTester(p, ""); !app.HasText("Reasoning effort") {
		t.Fatalf("a model with effort levels hides the choice: %q", app.Texts())
	}
}

func TestAdvancedHoldsTheCommandAndArgumentsAndIsOpenForCustomACP(t *testing.T) {
	p, saved := settingsPanel(t, workspace.DefaultSettings())
	if app := pageTester(p, ""); app.HasText("Agent command") {
		t.Fatalf("a known backend's advanced rows start open: %q", app.Texts())
	}
	custom := genui.BackendCustom
	if _, err := p.save(workspace.SettingsPatch{GenUI: &workspace.GenUIPatch{Backend: &custom}}); err != nil {
		t.Fatal(err)
	}
	app := pageTester(p, "")
	if !app.HasText("Agent command") || !app.HasText("Arguments") {
		t.Fatalf("Custom ACP does not show its command and arguments: %q", app.Texts())
	}

	clickField(t, app, "Agent command")
	app.Type("pi-acp --json")
	app.Frame()
	if err := app.Click("Save command"); err != nil {
		t.Fatalf("saving the command: %v", err)
	}
	app.Frame()
	last := (*saved)[len(*saved)-1]
	if last.GenUI == nil || last.GenUI.Backends[genui.BackendCustom].Command == nil || *last.GenUI.Backends[genui.BackendCustom].Command != "pi-acp --json" {
		t.Fatalf("saved patches = %+v, want the custom command", *saved)
	}
}

func TestSignInInTerminalRunsTheChosenBackendAndIsUnavailableForCustomACP(t *testing.T) {
	p, _ := settingsPanel(t, workspace.DefaultSettings())
	signed := make(chan string, 1)
	done := make(chan struct{})
	p.runOnUI = func(fn func()) { fn(); close(done) }
	p.signIn = func(id string) error {
		signed <- id
		return nil
	}
	app := pageTester(p, "")
	clickButton(t, app, "Sign in in Terminal")
	<-done
	if got := <-signed; got != genui.BackendAntigravity {
		t.Fatalf("signed in %q, want the active backend", got)
	}

	custom := genui.BackendCustom
	if _, err := p.save(workspace.SettingsPatch{GenUI: &workspace.GenUIPatch{Backend: &custom}}); err != nil {
		t.Fatal(err)
	}
	app = pageTester(p, "")
	if info, ok := app.AccessByRole(ui.RoleButton, "Sign in in Terminal"); !ok || !info.Disabled {
		t.Fatalf("Custom ACP offers a sign-in this app cannot run: %+v, %t", info, ok)
	}
}

func TestASaveErrorShowsAboveTheBackendCards(t *testing.T) {
	p, _ := settingsPanel(t, workspace.DefaultSettings())
	p.settingsMessage = "The agent command must not be empty."
	app := pageTester(p, "")
	if !app.HasText("The agent command must not be empty.") {
		t.Fatalf("the page hides its save error: %q", app.Texts())
	}
}

func TestTheRefreshFailureKeepsThePreviousModels(t *testing.T) {
	p, _ := settingsPanel(t, workspace.DefaultSettings())
	done := make(chan struct{})
	p.runOnUI = func(fn func()) { fn(); close(done) }
	p.status(genui.BackendAntigravity).catalog = genui.ModelCatalog{
		Models:       []genui.ModelOption{{ID: "gpt-6-sol"}},
		CurrentModel: "gpt-6-sol",
	}
	p.probe = func(string) (genui.ProbeResult, error) {
		return genui.ProbeResult{Status: genui.AgentUnknown}, nil
	}
	p.refresh = func(string) (genui.ModelCatalog, error) {
		return genui.ModelCatalog{}, errors.New("The agent did not answer.")
	}
	app := pageTester(p, "")
	clickButton(t, app, "Test & Refresh")
	<-done
	app.Frame()
	if !app.HasText("The agent did not answer.") {
		t.Fatalf("the refresh error is hidden: %q", app.Texts())
	}
	if got := p.status(genui.BackendAntigravity).catalog.Models; len(got) != 1 || got[0].ID != "gpt-6-sol" {
		t.Fatalf("a failed refresh replaced the models: %+v", got)
	}
}

func TestThePanelShowsAReadOnlyBackendSummaryBesideConfigure(t *testing.T) {
	settings := workspace.DefaultSettings()
	p, _ := settingsPanel(t, settings)
	v := &documentView{}
	app := ui.NewTester(func(c *ui.Context) { p.drawRequestToggle(c, v) }, 520, 120)
	app.Frame()
	if !app.HasText("Antigravity · gemini-4-hydrogen") {
		t.Fatalf("the panel does not summarise the backend and model: %q", app.Texts())
	}
	if _, ok := app.Access("Configure GenUI"); !ok {
		t.Fatalf("the panel offers no way into the settings: %q", app.Texts())
	}

	settings.GenUI.Backends[genui.BackendAntigravity] = workspace.GenUIBackend{Command: "agy", Args: []string{"acp"}}
	p, _ = settingsPanel(t, settings)
	app = ui.NewTester(func(c *ui.Context) { p.drawRequestToggle(c, v) }, 520, 120)
	app.Frame()
	if !app.HasText("Antigravity · Agent default") {
		t.Fatalf("an empty model is not reported as the agent's own: %q", app.Texts())
	}
}

func TestTheRefreshNotePromisesOnlyWhatTheAppCanKeep(t *testing.T) {
	if strings.Contains(refreshNote, "no local state") || !strings.Contains(refreshNote, "does not submit a generation request") {
		t.Fatalf("refresh note = %q, want a promise the app can keep", refreshNote)
	}
}

// clickButton clicks a button whose name is also a row title.
func clickButton(t *testing.T, app *ui.Tester, name string) {
	t.Helper()
	info, ok := app.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("no button %q; texts: %q", name, app.Texts())
	}
	app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	app.Frame()
}

// clickField focuses a text field, whose accessible name also labels its row.
func clickField(t *testing.T, app *ui.Tester, name string) {
	t.Helper()
	for _, info := range app.AccessAll() {
		if info.Name != name || !info.Focusable {
			continue
		}
		app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		app.Frame()
		return
	}
	t.Fatalf("no focusable field %q; texts: %q", name, app.Texts())
}

// TestAFocusRecheckKeepsTheModelsAndOnlyRetestsTheLogin covers the window's
// OnFocus refresh: a sign-in finished in Terminal shows up, but coming back to
// the window never opens a model session on its own.
func TestAFocusRecheckKeepsTheModelsAndOnlyRetestsTheLogin(t *testing.T) {
	p, _ := settingsPanel(t, workspace.DefaultSettings())
	probes, refreshes := 0, 0
	done := make(chan struct{}, 2)
	p.runOnUI = func(fn func()) { fn(); done <- struct{}{} }
	p.probe = func(string) (genui.ProbeResult, error) {
		probes++
		return genui.ProbeResult{Status: genui.AgentReady, Version: "codex-acp 1.13.1"}, nil
	}
	p.refresh = func(string) (genui.ModelCatalog, error) {
		refreshes++
		return genui.ModelCatalog{Models: []genui.ModelOption{{ID: "gpt-6-sol"}}, CurrentModel: "gpt-6-sol"}, nil
	}

	// A backend nobody tested is not probed on focus.
	p.probeOnFocus()
	if probes != 0 || refreshes != 0 {
		t.Fatalf("focus probed an untested backend: probes=%d refreshes=%d", probes, refreshes)
	}

	p.status(genui.BackendAntigravity).tested = true
	p.status(genui.BackendAntigravity).catalog = genui.ModelCatalog{Models: []genui.ModelOption{{ID: "gpt-6-sol"}}}
	p.probeOnFocus()
	<-done
	if probes != 1 || refreshes != 0 {
		t.Fatalf("focus probes=%d refreshes=%d, want one probe and no model session", probes, refreshes)
	}
	if got := p.status(genui.BackendAntigravity).probe.Status; got != genui.AgentReady {
		t.Fatalf("status after focus = %q, want the sign-in it found", got)
	}
	if got := p.status(genui.BackendAntigravity).catalog.Models; len(got) != 1 {
		t.Fatalf("focus dropped the known models: %+v", got)
	}
}
