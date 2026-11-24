package nativefeynman

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestComposerEnterClearsDraftOnlyAfterAcceptedSubmission(t *testing.T) {
	readyCalled := make(chan struct{})
	tests := []struct {
		name       string
		status     connectionStatus
		prepare    func(*panel)
		wantDraft  string
		wantAccept bool
	}{
		{name: "connecting", status: statusConnecting, wantDraft: "draft before Enter"},
		{name: "offline", status: statusOffline, wantDraft: "draft before Enter"},
		{name: "document load fails", status: statusReady, wantDraft: "draft before Enter", prepare: func(p *panel) {
			p.state.includeDoc = true
			p.activeDocument = func() workspace.DocumentInfo {
				return workspace.DocumentInfo{ID: "doc", Name: "Notes.md", Path: "/workspace/notes/Notes.md", WorkspaceRoot: "/workspace/notes"}
			}
			p.loadResearchDocument = func(string) (workspace.Snapshot, error) { return workspace.Snapshot{}, errors.New("load failed") }
		}},
		{name: "ready", status: statusReady, wantAccept: true, prepare: func(p *panel) {
			p.submitPrompt = func(_, _, _, _ string, _ []string) (string, error) {
				close(readyCalled)
				return "", nil
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.status = tc.status
			p.state.draft = "draft before Enter"
			if tc.prepare != nil {
				tc.prepare(p)
			}
			app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
			app.Frame()
			if err := app.Click("Research prompt"); err != nil {
				t.Fatal(err)
			}
			app.Key(0, ui.KeyEnter)
			app.Frame()
			want := tc.wantDraft
			if want == "" && !tc.wantAccept {
				want = "draft before Enter"
			}
			if tc.wantAccept {
				want = ""
			}
			if p.state.draft != want {
				t.Fatalf("draft = %q, want %q", p.state.draft, want)
			}
			editor, ok := app.AccessByRole(ui.RoleTextField, "Research prompt")
			if !ok || editor.Value != want {
				t.Fatalf("editor = %+v, present=%t; want value %q", editor, ok, want)
			}
			if tc.wantAccept {
				select {
				case <-readyCalled:
				case <-time.After(time.Second):
					t.Fatal("accepted prompt did not reach the submission hook")
				}
			}
		})
	}
}

func TestWhitespaceOnlyEnterPreservesDraft(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = "  \n  "
	if p.send() {
		t.Fatal("whitespace-only prompt was accepted")
	}
	if p.state.draft != "  \n  " {
		t.Fatalf("whitespace draft = %q, want unchanged input", p.state.draft)
	}
}

func TestManyAttachmentChipsStayBoundedAndHideIneligibleDocument(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.includeDoc = true
	p.state.sources = []string{"/workspace/one/shared.md", "/workspace/two/shared.md"}
	for i := 0; i < 18; i++ {
		p.state.sources = append(p.state.sources, fmt.Sprintf("/workspace/sources/source-%02d.pdf", i))
	}
	app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, 376, 272) }, 480, 360)
	app.Frame()
	for _, target := range []struct {
		role ui.Role
		name string
	}{{ui.RoleTextField, "Research prompt"}, {ui.RoleButton, "Send research prompt"}} {
		info, ok := app.AccessByRole(target.role, target.name)
		if !ok || info.Offscreen || info.Bounds.Y < 48 || info.Bounds.Y+info.Bounds.H > 320 {
			t.Fatalf("%s is not on screen in the narrow drawer: %+v, present=%t", target.name, info, ok)
		}
	}
	attachments, ok := app.AccessByRole(ui.RoleScroll, "Attached research sources")
	if !ok || attachments.Bounds.H > 2*portrait.Sizes.Regular+portrait.Editor.Hairline {
		t.Fatalf("attachment area = %+v, present=%t; want at most two rows", attachments, ok)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Remove active document from the research prompt"); ok {
		t.Fatal("ineligible active document still claims it will be attached")
	}
	if !app.HasText("one/shared.md") || !app.HasText("two/shared.md") {
		t.Fatalf("colliding source basenames are not disambiguated: %v", app.Texts())
	}
}

func TestStreamingChatFollowsOnlyWhileReaderIsAtBottom(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = ""
	p.state.status = statusWorking
	p.state.messages = nil
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { p.chat(c) })
	}, 480, 360)
	app.Frame()
	paragraph := func(i int) string {
		return fmt.Sprintf("Paragraph %d: streamed research content with enough words to occupy a full line.", i)
	}
	for i := 1; i <= 30; i++ {
		p.state.messages = append(p.state.messages, message{kind: "assistant", messageID: fmt.Sprintf("stream-%d", i), text: paragraph(i)})
	}
	for range 30 {
		app.Advance(time.Second)
		app.Frame()
	}
	app.Frame()
	if !p.chatFollow || p.chatScroll.Y < p.chatScroll.MaxY-portrait.Editor.Hairline {
		t.Fatalf("stream did not follow the bottom: Y=%v MaxY=%v follow=%t", p.chatScroll.Y, p.chatScroll.MaxY, p.chatFollow)
	}
	if !app.HasText("Paragraph 30") {
		t.Fatalf("last streamed paragraph was not rendered at the followed bottom (Y=%v MaxY=%v): %v", p.chatScroll.Y, p.chatScroll.MaxY, app.Texts())
	}
	scroll, ok := app.AccessByRole(ui.RoleScroll, "Feynman conversation")
	if !ok {
		t.Fatal("conversation scroll is missing")
	}
	app.Wheel(scroll.Bounds.X+scroll.Bounds.W/2, scroll.Bounds.Y+scroll.Bounds.H/2, 0, -240)
	app.Advance(time.Second)
	app.Frame()
	if p.chatFollow {
		t.Fatalf("wheel-up did not release streaming auto-follow: Y=%v lastY=%v MaxY=%v", p.chatScroll.Y, p.chatLastY, p.chatScroll.MaxY)
	}
	for i := 31; i <= 35; i++ {
		p.state.messages = append(p.state.messages, message{kind: "assistant", messageID: fmt.Sprintf("stream-%d", i), text: paragraph(i)})
	}
	app.Frame()
	if p.chatFollow {
		t.Fatal("new content resumed follow while the reader was above the bottom")
	}
	app.Wheel(scroll.Bounds.X+scroll.Bounds.W/2, scroll.Bounds.Y+scroll.Bounds.H/2, 0, 4000)
	app.Advance(time.Second)
	app.Frame()
	app.Frame()
	if !p.chatFollow || p.chatScroll.Y < p.chatScroll.MaxY-portrait.Editor.Hairline {
		t.Fatalf("scrolling back to the bottom did not resume follow: Y=%v MaxY=%v", p.chatScroll.Y, p.chatScroll.MaxY)
	}
}

func TestFilterableQuestionCannotConfirmInvisibleSelection(t *testing.T) {
	p := fixtureReadyPanel()
	question := feynman.ResearchUpdate{Kind: "select", RequestID: "filtered", Text: "Choose a source"}
	for i := 0; i < 7; i++ {
		question.Options = append(question.Options, feynman.ResearchOption{ID: fmt.Sprintf("option-%d", i), Name: fmt.Sprintf("Option %d", i)})
	}
	p.permissionID = question.RequestID
	p.questionPick = "option-0"
	p.questionFind = "does-not-match"
	app := ui.NewTester(func(c *ui.Context) { p.question(c, question) }, 420, 500)
	app.Frame()
	confirm, ok := app.AccessByRole(ui.RoleButton, "Confirm selected choice")
	if !ok || !confirm.Disabled {
		t.Fatalf("Confirm = %+v, present=%t; want disabled for an invisible selection", confirm, ok)
	}
	if !app.HasText("No matching options") {
		t.Fatal("zero-match question does not explain its empty list")
	}
}

func TestHostConfirmationHasTwoDistinctActionsAndNoDuplicateMessage(t *testing.T) {
	p := fixtureReadyPanel()
	question := feynman.ResearchUpdate{
		Kind: "confirm", RequestID: "host-confirm", Title: "Apply changes?", Text: "Apply changes?", Message: "Apply changes?",
		Options: []feynman.ResearchOption{{ID: "yes", Name: "Confirm", Kind: "confirm"}, {ID: "no", Name: "Cancel", Kind: "cancel"}},
	}
	app := ui.NewTester(func(c *ui.Context) { p.question(c, question) }, 420, 360)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleButton, "Answer Confirm"); !ok {
		t.Fatal("host confirmation has no affirmative action")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Answer Cancel"); !ok {
		t.Fatal("host confirmation has no decline action")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Cancel confirmation"); ok {
		t.Fatal("host confirmation renders a redundant third Cancel action")
	}
	count := 0
	for _, value := range app.Texts() {
		if value == question.Message {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("confirmation message rendered %d times, want once: %v", count, app.Texts())
	}
}

func TestSuggestedPromptPreservesExistingDraftWhitespace(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = "  indented  \ntrailing spaces  "
	p.state.suggested = "Next prompt"
	app := ui.NewTester(func(c *ui.Context) { p.composer(c) }, 640, 600)
	app.Frame()
	if err := app.Click("Insert suggested prompt"); err != nil {
		t.Fatal(err)
	}
	want := "  indented  \ntrailing spaces  \nNext prompt"
	if p.state.draft != want {
		t.Fatalf("draft = %q, want %q", p.state.draft, want)
	}
}

func TestThoughtDisclosureUsesStableMessageIdentity(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) {
		p.message(c, 0, message{kind: "thought", messageID: "stable-message", text: "reasoning"}, true)
	}, 420, 200)
	app.Frame()
	if !app.HasText("Thinking…") {
		t.Fatal("active reasoning is not labelled Thinking")
	}
	if _, ok := p.collapsed["stable-message-thought"]; !ok {
		t.Fatalf("thought keys = %v, want stable message ID", p.collapsed)
	}
	p.state.status = statusReady
	app = ui.NewTester(func(c *ui.Context) {
		p.message(c, 0, message{kind: "thought", messageID: "stable-message", text: "reasoning"}, false)
	}, 420, 200)
	app.Frame()
	if !app.HasText("Thought") || app.HasText("Thinking…") {
		t.Fatal("settled reasoning keeps the streaming title")
	}
}

func TestSlashPopoverStaysAboveComposerAndRemembersEscape(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = "/"
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Justify(ui.End).Children(func() { p.composer(c) })
	}, 640, 600)
	app.Frame()
	if err := app.Click("Research prompt"); err != nil {
		t.Fatal(err)
	}
	app.Type("rev")
	app.Frame()
	popover, ok := app.AccessByRole(ui.RolePopup, "Research actions")
	if !ok {
		t.Fatal("slash command did not open Research actions")
	}
	prompt, _ := app.AccessByRole(ui.RoleTextField, "Research prompt")
	if popover.Bounds.Y+popover.Bounds.H > prompt.Bounds.Y {
		t.Fatalf("actions popover %+v covers the composer prompt %+v", popover.Bounds, prompt.Bounds)
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if p.state.actionsOpen || !p.actionsDismissed {
		t.Fatal("Escape did not dismiss and remember the slash popover")
	}
	app.Type("iew")
	app.Frame()
	if p.state.actionsOpen {
		t.Fatal("continuing the dismissed slash draft reopened the popover")
	}
	if err := app.Click("Browse workflows, skills and extensions"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !p.state.actionsOpen || p.actionsDismissed {
		t.Fatal("manual Actions click did not reopen the dismissed popover")
	}
	if err := app.Click("Filter research actions"); err != nil {
		t.Fatal(err)
	}
	app.Type("iew")
	app.Frame()
	p.state.draft = "/summarize"
	app.Frame()
	if p.commandQuery != "review" {
		t.Fatalf("explicit action filter = %q, want review", p.commandQuery)
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Choose research action review"); !ok {
		t.Fatal("redraw discarded the explicitly filtered review action")
	}
	if _, ok := app.AccessByRole(ui.RoleButton, "Choose research action summarize"); ok {
		t.Fatal("redraw replaced the explicit action filter with the draft")
	}
}

func TestSlashCommandsRejectPathsAndMissingCommands(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	for _, draft := range []string{"/Users/ada/notes.md", "/tmp/input.md", "/unknown"} {
		if p.slashCommandMatches(draft) {
			t.Errorf("%q unexpectedly opens research actions", draft)
		}
	}
	for _, draft := range []string{"/", "/rev"} {
		if !p.slashCommandMatches(draft) {
			t.Errorf("%q should open matching research actions", draft)
		}
	}
}

func TestSlashPopoverClosesWhenDraftStopsMatching(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = "/"
	p.state.commands = []feynman.ResearchCommand{{Name: "review"}, {Name: "summarize"}}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Justify(ui.End).Children(func() { p.composer(c) })
	}, 640, 600)
	app.Frame()
	if err := app.Click("Research prompt"); err != nil {
		t.Fatal(err)
	}
	app.Type("rev")
	app.Frame()
	if !p.state.actionsOpen {
		t.Fatal("expected /rev to open Research actions")
	}
	app.Type("zzz")
	app.Frame()
	if p.state.actionsOpen {
		t.Fatal("slash popover stayed open with stale match after typing /revzzz")
	}
}

func TestAsyncSendFailurePreservesRawIndentationAndSeparatesInterveningDraft(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.draft = ""
	p.state.restoreDraft("    code with indentation\n", []string{"/workspace/notes/a.md"})
	if got := p.state.draft; got != "    code with indentation\n" {
		t.Fatalf("empty composer restored %q, want raw indented draft", got)
	}

	p2 := fixtureReadyPanel()
	p2.state.draft = "new draft typed while waiting"
	p2.state.sources = []string{"/workspace/notes/new.md"}
	p2.state.restoreDraft("    old failed prompt\n", []string{"/workspace/notes/old.md"})
	if p2.state.draft != "new draft typed while waiting" {
		t.Fatalf("intervening draft overwritten: %q", p2.state.draft)
	}
	if len(p2.state.sources) != 1 || p2.state.sources[0] != "/workspace/notes/new.md" {
		t.Fatalf("old sources polluted intervening draft: %v", p2.state.sources)
	}
	if p2.state.failedDraft != "    old failed prompt\n" || len(p2.state.failedSources) != 1 {
		t.Fatalf("failed submission not preserved: %q %v", p2.state.failedDraft, p2.state.failedSources)
	}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { p2.composer(c) })
	}, 520, 480)
	app.Frame()
	if err := app.Click("Restore unsent prompt and sources"); err != nil {
		t.Fatalf("missing restore failed draft control: %v", err)
	}
	app.Frame()
	if p2.state.draft != "    old failed prompt\n" {
		t.Fatalf("restored draft = %q, want exact failed draft", p2.state.draft)
	}
	if len(p2.state.sources) != 2 {
		t.Fatalf("restored sources = %v, want both old and current sources", p2.state.sources)
	}
}

func TestFeynmanSettingsErrorRendersDuringSearchAndRejectsEmptyHostnameURL(t *testing.T) {
	if err := mcpTransportValidation("http", "https://:443/mcp", ""); err == "" {
		t.Fatal("https://:443/mcp with empty hostname should be rejected")
	}
	if err := mcpTransportValidation("http", "https://mcp.example.com:443/mcp", ""); err != "" {
		t.Fatalf("valid https URL rejected: %q", err)
	}
	p := fixtureReadyPanel()
	p.settingsRoot = p.state.root
	p.state.cliError = "MCP server failed to add"
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSettingsPage(c, "mcp", nativeui.SettingsQuery{Text: "mcp"})
	}, 720, 600)
	app.Frame()
	if !app.HasText("MCP server failed to add") {
		t.Fatalf("cliError hidden while Settings search is active; texts: %v", app.Texts())
	}
}
