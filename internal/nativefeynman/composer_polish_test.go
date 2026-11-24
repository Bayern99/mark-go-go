package nativefeynman

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestResearchActionsButtonSpecUsesOnlyAnUpCaret(t *testing.T) {
	spec := researchActionsButtonSpec(false)
	if spec.Leading == portrait.IconDotsThree || spec.Trailing == portrait.IconDotsThree {
		t.Fatal("Actions still carries the dots glyph")
	}
	if spec.Leading != (portrait.Icon{}) {
		t.Fatalf("Actions leading icon = %+v, want none", spec.Leading)
	}
	if spec.Trailing != portrait.IconCaretUp {
		t.Fatalf("Actions trailing icon = %+v, want the upward caret", spec.Trailing)
	}
	if spec.Size != portrait.Sizes.Regular {
		t.Fatalf("Actions height = %v, want Regular %v", spec.Size, portrait.Sizes.Regular)
	}
}

func TestIncludeDocumentButtonTogglesAtRegularHeight(t *testing.T) {
	included := false
	app := ui.NewTester(func(c *ui.Context) {
		includeDocumentButton(c, &included)
	}, 240, 100)
	app.Frame()
	button, ok := app.AccessByRole(ui.RoleButton, "Include doc")
	if !ok {
		t.Fatal("Include doc is not a button")
	}
	if button.Bounds.H != portrait.Sizes.Regular {
		t.Fatalf("Include doc height = %v, want Regular %v", button.Bounds.H, portrait.Sizes.Regular)
	}
	if button.Checked {
		t.Fatal("Include doc starts pressed before it is selected")
	}
	if err := app.Click("Include doc"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !included {
		t.Fatal("Include doc did not toggle the attachment state")
	}
	button, _ = app.AccessByRole(ui.RoleButton, "Include doc")
	if !button.Checked {
		t.Fatal("Include doc does not expose its pressed state")
	}
}

func TestSessionUnavailableNoticeUsesAlertBodyStyling(t *testing.T) {
	const notice = "The Feynman session is unavailable. Reconnect to continue this research conversation."
	actual := ui.NewTester(func(c *ui.Context) {
		p := &panel{}
		p.message(c, 0, message{kind: "notice", notify: "warning", text: notice}, false)
	}, 420, 120)
	expected := ui.NewTester(func(c *ui.Context) {
		portrait.Alert(c.Key("message-0"), portrait.Tones.Warning, "", notice)
	}, 420, 120)
	actual.Frame()
	expected.Frame()
	body, ok := actual.Find(notice)
	if !ok {
		t.Fatal("session notice text is missing")
	}
	expectedBody, _ := expected.Find(notice)
	if body.H != expectedBody.H {
		t.Fatalf("notice height = %v, want alert body height %v", body.H, expectedBody.H)
	}
	if !bytes.Equal(actual.Image().Pix, expected.Image().Pix) {
		t.Fatal("notice does not render like calm regular-weight alert body text")
	}
}

func TestQuestionCardUsesCompactRadiosAndCancelFirstRegularActions(t *testing.T) {
	p := &panel{state: newPaneState("/workspace")}
	question := feynman.ResearchUpdate{
		Kind: "select", RequestID: "request-polish", Text: "Choose a source",
		Options: []feynman.ResearchOption{{ID: "local", Name: "Local notes"}, {ID: "remote", Name: "Published paper"}},
	}
	app := ui.NewTester(func(c *ui.Context) { p.question(c, question) }, 420, 360)
	app.Frame()
	radio, ok := app.AccessByRole(ui.RoleRadio, "Published paper")
	if !ok {
		t.Fatal("question option is not an accessible radio")
	}
	if radio.Bounds.H != portrait.Type.UI.Leading {
		t.Fatalf("question radio height = %v, want compact UI leading %v", radio.Bounds.H, portrait.Type.UI.Leading)
	}
	group, _ := app.AccessByRole(ui.RoleGroup, "Feynman confirmation")
	cancel, cancelOK := app.AccessByRole(ui.RoleButton, "Cancel this Feynman request")
	confirm, confirmOK := app.AccessByRole(ui.RoleButton, "Confirm selected choice")
	if !cancelOK || !confirmOK {
		t.Fatal("question is missing Cancel or Confirm")
	}
	for _, button := range []ui.AccessInfo{cancel, confirm} {
		if button.Bounds.H != portrait.Sizes.Regular {
			t.Errorf("%s height = %v, want Regular %v", button.Name, button.Bounds.H, portrait.Sizes.Regular)
		}
	}
	if cancel.Bounds.X >= confirm.Bounds.X {
		t.Fatalf("Cancel bounds %+v do not precede Confirm %+v", cancel.Bounds, confirm.Bounds)
	}
	if cancel.Bounds.X < group.Bounds.X+group.Bounds.W/2 {
		t.Fatalf("question actions are not right aligned: Cancel %+v in group %+v", cancel.Bounds, group.Bounds)
	}
	if err := app.Click("Published paper"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if p.questionPick != "remote" {
		t.Fatalf("selected option = %q, want remote", p.questionPick)
	}
}

func TestComposerFooterControlsShareRegularHeight(t *testing.T) {
	p := fixtureReadyPanel()
	p.state.status = statusWorking
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	for _, name := range []string{"Send a steering message", "Queue a follow-up message", "Stop the current research action", "Model", "Thinking"} {
		button, ok := app.AccessByRole(ui.RoleButton, name)
		if !ok {
			t.Errorf("missing footer button %q", name)
			continue
		}
		if button.Bounds.H != portrait.Sizes.Regular {
			t.Errorf("%s height = %v, want Regular %v", name, button.Bounds.H, portrait.Sizes.Regular)
		}
	}
}

func TestConversationActionsUseARegularQuietMenuButton(t *testing.T) {
	p := fixtureReadyPanel()
	app := ui.NewTester(func(c *ui.Context) { drawWidePane(c, p) }, 1280, 820)
	app.Frame()
	p.state.sessionsOpen = true
	app.Frame()
	trigger, ok := app.AccessByRole(ui.RoleMenuButton, "Conversation actions")
	if !ok {
		t.Fatal("conversation actions is not a menu button")
	}
	if trigger.Bounds.H <= 0 {
		t.Fatalf("conversation actions has invalid bounds %+v", trigger.Bounds)
	}
	if err := app.Click("Conversation actions"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if got := len(app.Menu()); got != 6 {
		t.Fatalf("conversation action count = %d, want 6: %v", got, app.Menu())
	}
}

func TestHistoryPopoverClearsTabsAndPanelEdgeWithOneListScroller(t *testing.T) {
	for _, width := range []float32{376, 240, 220} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			p := fixtureReadyPanel()
			p.state.sessionsOpen = true
			p.state.sessionRows = []feynman.ResearchConversation{{ID: "fixture", Title: "Fixture session", Current: true}}
			app := ui.NewTester(func(c *ui.Context) { drawFixturePane(c, p, width, 500) }, 480, 640)
			app.Frame()
			popover, ok := app.AccessByRole(ui.RolePopup, "Workspace conversation history")
			if !ok {
				t.Fatal("history popover is missing")
			}
			var tabs ui.AccessInfo
			for _, info := range app.AccessAll() {
				if info.Role == ui.RoleTabList {
					tabs = info
				}
			}
			if tabs.Role != ui.RoleTabList {
				t.Fatal("research tabs are missing")
			}
			if popover.Bounds.Y < tabs.Bounds.Y+tabs.Bounds.H {
				t.Errorf("history bounds %+v overlap tabs %+v", popover.Bounds, tabs.Bounds)
			}
			const drawerLeft = float32(96)
			if popover.Bounds.X < drawerLeft {
				t.Errorf("history left edge %v protrudes past the panel at %v", popover.Bounds.X, drawerLeft)
			}
			if _, ok := app.AccessByRole(ui.RoleScroll, "Workspace conversations"); ok {
				t.Error("conversation list adds a second nested scroller")
			}
			if _, ok := app.AccessByRole(ui.RoleButton, "Close conversation history"); ok {
				t.Error("history still has its redundant close button")
			}
		})
	}
}

func TestConfigPickerKeepsFocusWhenSelectedModelChanges(t *testing.T) {
	selected := "anthropic/claude"
	p := &panel{collapsed: make(map[string]*bool)}
	app := ui.NewTester(func(c *ui.Context) {
		picker := fixtureResearchPicker(&selected)
		picker.current = selected
		for i := range picker.items {
			picker.items[i].current = picker.items[i].value == selected
		}
		ui.Column(c).Fill().Justify(ui.End).Padding(20).Children(func() {
			p.configPicker(c, picker)
		})
	}, 420, 500)
	app.Frame()
	if err := app.Click("Model"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("gpt")
	app.Frame()
	app.Key(0, ui.KeyDown)
	app.Frame()
	app.Key(0, ui.KeyEnter)
	app.Frame()
	app.Frame()
	if selected != "openai/gpt" {
		t.Fatalf("selected = %q, want openai/gpt", selected)
	}
	trigger, ok := app.AccessByRole(ui.RoleButton, "Model")
	if !ok {
		t.Fatal("Model trigger missing after selection")
	}
	if trigger.Value != "GPT Terra" {
		t.Fatalf("Model trigger value = %q, want GPT Terra", trigger.Value)
	}
	if !trigger.Focused {
		t.Fatalf("Model trigger lost focus after value changed to %q", trigger.Value)
	}
}
