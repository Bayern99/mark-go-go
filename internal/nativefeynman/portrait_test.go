package nativefeynman

import (
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

func fixtureBusyPanel() *panel {
	p := fixtureReadyPanel()
	s := p.state
	s.status = statusWorking
	s.progress = "Reading three sources"
	s.statuses = map[string]string{"search": "exa"}
	s.widgets = map[string]feynman.ResearchUpdate{"plan": {WidgetLines: []string{"Step one of three"}}}
	exit := 0
	s.messages = []message{
		{kind: "user", text: "Compare the two reports"},
		{kind: "assistant", messageID: "a", text: "## Findings\n\n1. First `code` point\n2. Second point\n\n```go\nfmt.Println(1)\n```"},
		{kind: "notice", notify: "warning", text: "Rate limited, retrying"},
		{kind: "plan", steps: []feynman.ResearchPlanEntry{{Content: "Read sources", Status: "completed"}, {Content: "Write summary", Status: "pending"}}},
		{kind: "tool", tool: feynman.ResearchTool{ToolCallID: "t1", Title: "Search", Status: "completed", Input: "query", Output: "result", ExitCode: &exit,
			Locations: []feynman.ResearchToolLocation{{Path: "/workspace/notes/a.md", Line: 3}},
			Diffs:     []feynman.ResearchToolDiff{{Path: "a.md", OldText: "old", NewText: "new"}}}},
	}
	p.collapsed["tool-t1"] = ptr(true)
	s.queue = feynman.ResearchQueueState{Steering: []string{"Focus on costs"}, FollowUp: []string{"Then cite"}}
	s.suggested = "Summarize the differences"
	s.sources = []string{"/workspace/notes/Attachment.md"}
	s.includeDoc = true
	s.sessionsOpen = true
	s.sessionRows = []feynman.ResearchConversation{{ID: "one", Title: "Current study", Current: true}, {ID: "two", Title: "Older study"}}
	s.artifacts = []review.ResearchArtifact{{Path: "research/summary.md", Target: "summary.md", Proposal: true}, {Path: "research/gone.md", Missing: true}}
	s.cliStatus = &feynman.CLIStatusResult{Parsed: true, Sections: []feynman.CLISection{{Title: "Runtime", Entries: []feynman.CLIKeyValue{{Key: "version", Value: "1.0"}}}}}
	p.collapsed["cli-status-Runtime"] = ptr(true)
	s.doctorResult = &feynman.CLIDoctorResult{Parsed: true, Issues: []string{"No search key"}}
	s.cliModels = &feynman.CLIModelListResult{Parsed: true, Models: []feynman.CLIModelEntry{{ID: "m", Name: "Model M", Provider: "local", Current: true}}}
	s.cliResult = "done"
	s.searchResult = &feynman.CLIAlphaSearchResult{Parsed: true, Mode: "semantic", Papers: []feynman.CLIAlphaPaper{{ID: "2401.1", Title: "A paper", Authors: "Ada"}}}
	return p
}

func ptr[T any](v T) *T { return &v }

func drawWidePane(c *ui.Context, p *panel) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c.Key("fixture-research-pane")).Width(420).Fill().Children(func() { p.draw(c) })
	})
}

var researchTabs = []string{"chat", "files", "settings", "alpha"}

func forEachTab(t *testing.T, prefs ui.Preferences, check func(t *testing.T, app *ui.Tester)) {
	for _, tab := range researchTabs {
		t.Run(tab, func(t *testing.T) {
			p := fixtureBusyPanel()
			p.state.selectedTab = tab
			app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 2400)
			app.SetPreferences(prefs)
			app.Frame()
			app.Advance(time.Second)
			check(t, app)
		})
	}
}

func TestResearchPanelTextIsNeverSmallerThanMeta(t *testing.T) {
	forEachTab(t, ui.Preferences{}, func(t *testing.T, app *ui.Tester) {
		texts := 0
		for _, a := range app.AccessAll() {
			if a.Role != ui.RoleText || a.Offscreen || strings.TrimSpace(a.Name) == "" {
				continue
			}
			texts++
			if a.Bounds.H < 14 {
				t.Errorf("text %q is %v DIP tall, smaller than the 12px meta step", a.Name, a.Bounds.H)
			}
		}
		if texts < 5 {
			t.Fatalf("only %d texts drawn; the fixture did not render", texts)
		}
	})
}

func isCharacterIcon(r rune) bool {
	return strings.ContainsRune("×✣✓⌁•▾▸☐☑", r) || unicode.In(r, unicode.Co)
}

func TestResearchPanelShowsNoCharacterIcons(t *testing.T) {
	forEachTab(t, ui.Preferences{}, func(t *testing.T, app *ui.Tester) {
		for _, a := range app.AccessAll() {
			for _, s := range []string{a.Name, a.Value, a.Description} {
				if strings.ContainsFunc(s, isCharacterIcon) {
					t.Errorf("%v %q shows a character icon in %q", a.Role, a.Name, s)
				}
			}
		}
	})
}

func TestResearchIconButtonsKeepTheirNames(t *testing.T) {
	forEachTab(t, ui.Preferences{}, func(t *testing.T, app *ui.Tester) {
		if app.HasText("Compare the two reports") {
			for _, name := range []string{"Dismiss suggested prompt", "Remove active document from the research prompt", "Remove attached source /workspace/notes/Attachment.md"} {
				info, ok := app.AccessByRole(ui.RoleButton, name)
				if !ok {
					t.Errorf("no button named %q", name)
				} else if info.Description == "" {
					t.Errorf("button %q has no tooltip", name)
				}
			}
		}
	})
}

func TestResearchWorkingShowsASpinnerThatStopsUnderReducedMotion(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		p := fixtureBusyPanel()
		p.state.sessionsOpen = false
		app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
		app.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		app.Frame()
		spinner, ok := app.AccessByRole(ui.RoleProgress, "Feynman is working")
		if !ok {
			t.Fatalf("reduced=%t: working panel shows no spinner", reduced)
		}
		if !app.HasText("working") || !app.HasText("Reading three sources") {
			t.Fatalf("reduced=%t: working state lost its text", reduced)
		}
		before := app.Image()
		app.Advance(225 * time.Millisecond)
		after := app.Image()
		moved := 0
		for y := int(spinner.Bounds.Y); y < int(spinner.Bounds.Y+spinner.Bounds.H); y++ {
			for x := int(spinner.Bounds.X); x < int(spinner.Bounds.X+spinner.Bounds.W); x++ {
				if before.At(x, y) != after.At(x, y) {
					moved++
				}
			}
		}
		if reduced && moved != 0 {
			t.Errorf("spinner turned %d pixels under reduced motion", moved)
		}
		if !reduced && moved == 0 {
			t.Error("spinner did not turn while working")
		}
	}
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleProgress, "Feynman is working"); ok {
		t.Error("ready panel shows a spinner")
	}
}

func TestResearchSendButtonSinksWhilePressed(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	info, ok := app.AccessByRole(ui.RoleButton, "Send research prompt")
	if !ok {
		t.Fatal("no Send button")
	}
	b := info.Bounds
	region := func() []uint32 {
		img := app.Image()
		var px []uint32
		for y := int(b.Y); y < int(b.Y+b.H); y++ {
			for x := int(b.X); x < int(b.X+b.W); x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				px = append(px, r+g+bl)
			}
		}
		return px
	}
	rest := region()
	app.Press(b.X+b.W/2, b.Y+b.H/2)
	app.Advance(300 * time.Millisecond)
	pressed := region()
	changed := 0
	for i := range rest {
		if rest[i] != pressed[i] {
			changed++
		}
	}
	if changed < len(rest)/4 {
		t.Fatalf("pressing Send changed %d of %d pixels, want press feedback", changed, len(rest))
	}
}

func TestResearchToolbarActionsAreQuietAndShowWhatIsOpen(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	edge := func(name string) (inside, outside uint32) {
		t.Helper()
		info, ok := app.AccessByRole(ui.RoleButton, name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		img := app.Image()
		at := func(x, y float32) uint32 {
			r, g, b, _ := img.At(int(x), int(y)).RGBA()
			return r + g + b
		}
		y := info.Bounds.Y + info.Bounds.H/2
		return at(info.Bounds.X+3, y), at(info.Bounds.X-2, y)
	}
	for _, name := range []string{"Workspace conversation history", "Attach source files", "Browse workflows, skills and extensions"} {
		if inside, outside := edge(name); inside != outside {
			t.Errorf("%s paints a raised fill at rest: %d vs surrounding %d", name, inside, outside)
		}
	}
	p.state.sessionsOpen = true
	app.Frame()
	if inside, outside := edge("Workspace conversation history"); inside == outside {
		t.Error("History looks the same while the conversation list is open")
	}
}

func TestResponseCopyShowsOnHoverOrFocus(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	copyButton, ok := app.AccessByRole(ui.RoleButton, "Copy Feynman response")
	if !ok {
		t.Fatal("missing Copy")
	}
	ink := func() int {
		img := app.Image()
		b := copyButton.Bounds
		n := 0
		for y := int(b.Y); y < int(b.Y+b.H); y++ {
			for x := int(b.X); x < int(b.X+b.W); x++ {
				if r, _, _, _ := img.At(x, y).RGBA(); r>>8 < 200 {
					n++
				}
			}
		}
		return n
	}
	if n := ink(); n != 0 {
		t.Fatalf("Copy shows %d ink pixels at rest, want it hidden until hover", n)
	}
	app.Move(copyButton.Bounds.X+copyButton.Bounds.W/2, copyButton.Bounds.Y-20)
	app.Advance(time.Second)
	app.Frame()
	if ink() == 0 {
		t.Fatal("hovering the response did not reveal Copy")
	}
}
