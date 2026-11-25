package nativefeynman

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/egoist/mygo/ui"
)

func researchActionOption(t *testing.T, app *ui.Tester, name string) ui.AccessInfo {
	t.Helper()
	option, ok := app.AccessByRole(ui.RoleOption, "Choose research action "+name)
	if !ok {
		t.Fatalf("research action %q is not an option; visible text: %v", name, app.Texts())
	}
	return option
}

func openResearchActions(t *testing.T, app *ui.Tester, slash bool) {
	t.Helper()
	if slash {
		if err := app.Click("Research prompt"); err != nil {
			t.Fatal(err)
		}
		app.Type("/")
	} else {
		if err := app.Click("Browse workflows, skills and extensions"); err != nil {
			t.Fatal(err)
		}
	}
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Research actions"); !ok {
		t.Fatal("research actions did not open")
	}
}

func TestResearchActionsButtonSupportsImmediateKeyboardChoiceAndEscape(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = ""
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
	app.Frame()
	openResearchActions(t, app, false)

	app.Key(0, ui.KeyDown)
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if p.state.draft != "/summarize " || p.state.actionsOpen {
		t.Fatalf("immediate Down and Enter left draft %q and actions open = %v", p.state.draft, p.state.actionsOpen)
	}
	app.Type("abc")
	app.Frame()
	if p.state.draft != "/summarize abc" {
		t.Fatalf("typing an argument after button completion left draft %q, want %q", p.state.draft, "/summarize abc")
	}

	openResearchActions(t, app, false)
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if p.state.actionsOpen || !app.Focused("Browse workflows, skills and extensions") {
		t.Fatalf("Escape left actions open=%v without focus on the Actions button", p.state.actionsOpen)
	}
}

func TestResearchActionsButtonOutsideClickKeepsTheClickedFocus(t *testing.T) {
	for _, editFilter := range []bool{false, true} {
		p := fixtureReadyPanel()
		p.state.draft = ""
		p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
		app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
		app.Frame()
		openResearchActions(t, app, false)
		if editFilter {
			app.Type("rev")
			app.Frame()
		}
		if err := app.Click("Research prompt"); err != nil {
			t.Fatal(err)
		}
		app.Frame()
		app.Type("z")
		app.Frame()
		if p.state.actionsOpen || !app.Focused("Research prompt") || p.state.draft != "z" {
			t.Fatalf("editFilter=%v: clicking the prompt left actions open=%v, draft=%q, or focus outside the prompt", editFilter, p.state.actionsOpen, p.state.draft)
		}
	}
}

func TestResearchActionsButtonEscapeAfterTabbingOutReturnsToTheButton(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = ""
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
	app.Frame()
	openResearchActions(t, app, false)
	for i := 0; i < 20 && !app.Focused("Research prompt"); i++ {
		app.Key(ui.Shift, ui.KeyTab)
		app.Frame()
	}
	if !app.Focused("Research prompt") || !p.state.actionsOpen {
		t.Fatalf("Shift+Tab did not reach the prompt with actions open (open=%v)", p.state.actionsOpen)
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if p.state.actionsOpen || !app.Focused("Browse workflows, skills and extensions") {
		t.Fatalf("Escape after tabbing out left actions open=%v without focus on the Actions button", p.state.actionsOpen)
	}
}

func TestSlashUnknownTypedDraftSendsVerbatimAfterEscape(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = ""
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	sent := make(chan string, 1)
	p.submitPrompt = func(_, prompt, _, _ string, _ []string) (string, error) {
		sent <- prompt
		return "", nil
	}
	app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
	app.Frame()
	if err := app.Click("Research prompt"); err != nil {
		t.Fatal(err)
	}
	for _, r := range "/tmp" {
		app.Type(string(r))
	}
	app.Frame()
	if !p.state.actionsOpen || !app.HasText("No matching workflows, skills or extensions.") {
		t.Fatal("typing an unknown slash draft did not leave Actions open with its empty state")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if p.state.actionsOpen || p.state.draft != "/tmp" || !app.Focused("Research prompt") {
		t.Fatalf("Escape left actions open=%v, draft=%q, or focus outside the prompt", p.state.actionsOpen, p.state.draft)
	}
	app.Key(0, ui.KeyEnter)
	app.Frame()
	select {
	case got := <-sent:
		if got != "/tmp" {
			t.Fatalf("Enter sent %q, want the verbatim unknown slash draft", got)
		}
	case <-time.After(time.Minute):
		t.Fatal("Enter did not submit the unknown slash draft")
	}
}

func TestResearchActionsEmptyResultStaysOpenAndEnterDoesNotSend(t *testing.T) {
	for _, slash := range []bool{false, true} {
		name := "Actions button"
		if slash {
			name = "slash draft"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, slash)
			app.Type("no-match")
			app.Frame()
			if _, ok := app.AccessByRole(ui.RolePopup, "Research actions"); !ok || !app.HasText("No matching workflows, skills or extensions.") {
				t.Fatalf("empty research actions closed or omitted its empty state: %v", app.Texts())
			}
			app.Key(0, ui.KeyEnter)
			app.Frame()
			wantDraft := ""
			if slash {
				wantDraft = "/no-match"
			}
			if !p.state.actionsOpen || p.state.draft != wantDraft || p.state.error != "" || p.state.status != statusReady {
				t.Fatalf("empty Enter left open=%v draft=%q error=%q status=%q", p.state.actionsOpen, p.state.draft, p.state.error, p.state.status)
			}
		})
	}
}

func TestResearchActionsFilterIsCaseInsensitiveAndIgnoresOuterSpace(t *testing.T) {
	for _, slash := range []bool{false, true} {
		name := "Actions button"
		if slash {
			name = "slash draft"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{
				{Name: "review", Source: "Workflow"},
				{Name: "revise", Source: "Workflow"},
				{Name: "summarize", Source: "Skill"},
			}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, slash)
			app.Key(0, ui.KeyDown)
			app.Type(" WORKFLOW ")
			app.Frame()
			if !researchActionOption(t, app, "review").Selected {
				t.Fatal("filtering did not select the first visible research action")
			}
			if _, ok := app.AccessByRole(ui.RoleOption, "Choose research action summarize"); ok {
				t.Fatal("a research action that does not match the filter stayed listed")
			}
		})
	}
}

func TestResearchActionsEscapeReturnsFocusToItsOrigin(t *testing.T) {
	for _, slash := range []bool{false, true} {
		name := "Actions button"
		origin := "Browse workflows, skills and extensions"
		if slash {
			name = "slash draft"
			origin = "Research prompt"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, slash)
			app.Key(0, ui.KeyEscape)
			app.Frame()
			if p.state.actionsOpen || !app.Focused(origin) {
				t.Fatalf("Escape left actions open=%v without focus on %q", p.state.actionsOpen, origin)
			}
		})
	}
}

func TestResearchActionsEscapeReturnsFocusToItsOriginAfterFiltering(t *testing.T) {
	for _, slash := range []bool{false, true} {
		name := "Actions button"
		origin := "Browse workflows, skills and extensions"
		if slash {
			name = "slash draft"
			origin = "Research prompt"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, slash)
			if err := app.Click("Filter research actions"); err != nil {
				t.Fatal(err)
			}
			app.Type("rev")
			app.Frame()
			if _, ok := app.AccessByRole(ui.RoleOption, "Choose research action summarize"); ok {
				t.Fatal("the filter edit did not narrow the research actions")
			}
			app.Key(0, ui.KeyEscape)
			app.Frame()
			if p.state.actionsOpen || !app.Focused(origin) {
				t.Fatalf("Escape after filtering left actions open=%v without focus on %q", p.state.actionsOpen, origin)
			}
		})
	}
}

func TestResearchActionsCompletionPlacesCaretAfterCommand(t *testing.T) {
	for _, moveDown := range []bool{false, true} {
		name := "first action"
		want := "/review abc"
		if moveDown {
			name = "second action"
			want = "/summarize abc"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, true)
			if moveDown {
				app.Key(0, ui.KeyDown)
			}
			app.Key(0, ui.KeyEnter)
			app.Frame()
			app.Type("abc")
			app.Frame()
			if p.state.draft != want {
				t.Fatalf("typing an argument after completion left draft %q, want %q", p.state.draft, want)
			}
		})
	}
}

func TestResearchActionsChooseWithTheSameArrowsAndEnterAsCommandBar(t *testing.T) {
	for _, slash := range []bool{false, true} {
		name := "Actions button"
		if slash {
			name = "slash draft"
		}
		t.Run(name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.draft = ""
			p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			openResearchActions(t, app, slash)

			if !researchActionOption(t, app, "review").Selected {
				t.Fatal("the first research action is not selected")
			}
			app.Key(0, ui.KeyUp)
			if !researchActionOption(t, app, "summarize").Selected {
				t.Fatal("Up did not wrap the first research action to the last")
			}
			app.Key(0, ui.KeyDown)
			if !researchActionOption(t, app, "review").Selected {
				t.Fatal("Down did not wrap the last research action to the first")
			}
			app.Key(0, ui.KeyDown)
			if !researchActionOption(t, app, "summarize").Selected || researchActionOption(t, app, "review").Selected {
				t.Fatal("Down did not move the research action selection")
			}
			app.Key(0, ui.KeyUp)
			if !researchActionOption(t, app, "review").Selected {
				t.Fatal("Up did not move the research action selection back")
			}
			app.Key(0, ui.KeyDown)
			app.Key(0, ui.KeyEnter)
			app.Frame()
			if p.state.draft != "/summarize " || p.state.actionsOpen {
				t.Fatalf("Enter left draft %q and actions open = %v, want the selected action inserted and closed", p.state.draft, p.state.actionsOpen)
			}
		})
	}
}
