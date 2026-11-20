package nativefeynman

import (
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/egoist/mygo/ui"
)

func TestNativeResearchMessagesRenderMarkdownToolOutputAndQuestions(t *testing.T) {
	p := &panel{state: newPaneState("/workspace"), collapsed: make(map[string]*bool)}
	assistant := message{kind: "assistant", messageID: "response", text: "## Research summary\n\nA **grounded** result."}
	tool := message{kind: "tool", tool: feynman.ResearchTool{
		ToolCallID:    "tool-1",
		Title:         "Search",
		Status:        "completed",
		TerminalDelta: "found three sources",
		Output:        "source-a.md",
	}}
	question := feynman.ResearchUpdate{
		Kind:      "select",
		RequestID: "request-1",
		Text:      "Choose a source",
		Options:   []feynman.ResearchOption{{ID: "one", Name: "Source one"}},
	}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			p.message(c, 0, assistant, true)
			p.message(c, 1, tool, false)
			p.question(c, question)
		})
	}, 420, 500)
	app.Frame()
	app.Advance(time.Second)
	if !app.HasText("Research summary") || !app.HasText("grounded") {
		t.Fatal("streaming Markdown response did not render its heading and emphasized text")
	}
	if _, ok := app.AccessByRole(ui.RoleGroup, "Feynman confirmation"); !ok {
		t.Fatal("interactive Feynman question has no accessible group name")
	}
	if err := app.Click("Search · completed"); err != nil {
		t.Fatalf("open tool output: %v", err)
	}
	app.Frame()
	if !app.HasText("found three sources") || !app.HasText("source-a.md") {
		t.Fatal("expanded research tool did not render its terminal output and result")
	}
}

func TestNativeResearchQuestionSelectsAnAccessibleOption(t *testing.T) {
	p := &panel{state: newPaneState("/workspace")}
	question := feynman.ResearchUpdate{
		Kind:      "select",
		RequestID: "request-select",
		Text:      "Choose a source",
		Options: []feynman.ResearchOption{
			{ID: "local", Name: "Local notes"},
			{ID: "remote", Name: "Published paper"},
		},
	}
	app := ui.NewTester(func(c *ui.Context) { p.question(c, question) }, 420, 360)
	app.Frame()
	if err := app.Click("Published paper"); err != nil {
		t.Fatalf("select research question option: %v", err)
	}
	app.Frame()
	if p.questionPick != "remote" {
		t.Fatalf("selected option = %q, want remote", p.questionPick)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Confirm selected choice"); !ok {
		t.Fatal("selected option has no accessible confirmation action")
	}
}

func TestNativeResearchConfirmationShowsAnswerAndCancel(t *testing.T) {
	p := &panel{state: newPaneState("/workspace")}
	question := feynman.ResearchUpdate{
		Kind:      "confirm",
		RequestID: "request-confirm",
		Text:      "Continue with the research?",
		Options: []feynman.ResearchOption{
			{ID: "yes", Name: "Continue"},
			{ID: "no", Name: "Stop"},
		},
	}
	app := ui.NewTester(func(c *ui.Context) { p.question(c, question) }, 420, 360)
	app.Frame()
	for _, name := range []string{"Answer Continue", "Answer Stop", "Cancel confirmation"} {
		if _, ok := app.AccessByRole(ui.RoleButton, name); !ok {
			t.Fatalf("confirmation question has no accessible %q action", name)
		}
	}
}
