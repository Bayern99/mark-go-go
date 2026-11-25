package nativeai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agentcli"
	"github.com/Bayern99/mark-go-go/internal/agentguide"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// testPanel is a page whose installer, socket and settings live in the test:
// the store's own rules are the store's tests, and this one is about the page.
func testPanel(t *testing.T) (*panel, *[]workspace.SettingsPatch) {
	t.Helper()
	source := filepath.Join(t.TempDir(), agentcli.Name)
	if err := os.WriteFile(source, []byte("#!/bin/sh\necho markgogo\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := &[]workspace.SettingsPatch{}
	p := &panel{
		install:  agentcli.Installer{Source: source, Dir: filepath.Join(t.TempDir(), ".local", "bin")},
		userData: t.TempDir(),
		settings: workspace.DefaultSettings(),
		harness:  agentguide.HarnessClaudeCode,
	}
	p.save = func(patch workspace.SettingsPatch) (workspace.Settings, error) {
		*saved = append(*saved, patch)
		if patch.ExternalIPC != nil {
			p.settings.ExternalIPC = *patch.ExternalIPC
		}
		return p.settings, nil
	}
	p.channel = Channel{
		Listening: func() bool { return p.settings.ExternalIPC },
		LastCall:  func() (string, time.Time, bool) { return "", time.Time{}, false },
	}
	return p, saved
}

func pageTester(p *panel, query string) *ui.Tester {
	page := p.settingsPage()
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(12).Children(func() {
			page.Draw(c, nativeui.SettingsQuery{Text: query})
		})
	}, 640, 1700)
	app.Frame()
	return app
}

func TestThePageShowsEveryWayAnAgentReachesTheWindow(t *testing.T) {
	p, _ := testPanel(t)
	app := pageTester(p, "")

	for _, want := range []string{
		"External Agent Capabilities", "Command Line Tool", "Agent Prompt",
		"Agent Skill", "MCP (stdio)", "Allow external agents", "Last external call",
		"Claude Code", "Codex", "Pi", "Other",
	} {
		if !app.HasText(want) {
			t.Errorf("the page lacks %q: %q", want, app.Texts())
		}
	}
	if !app.HasText("A GenUI backend is not one") {
		t.Errorf("the page does not tell an external agent from a GenUI backend: %q", app.Texts())
	}
}

// nativeuiPageLabel is the label Settings Center lists the page under.
const nativeuiPageLabel = "AI Integration"

func TestThePageIsRegisteredUnderTheWorkspaceSection(t *testing.T) {
	p, _ := testPanel(t)
	page := p.settingsPage()
	if page.ID != "ai-integration" || page.Label != nativeuiPageLabel {
		t.Fatalf("page %q is labelled %q", page.ID, page.Label)
	}
	if page.Section != "Workspace" {
		t.Fatalf("page section = %q, want Workspace", page.Section)
	}
	if page.Icon == (portrait.Icon{}) {
		t.Fatal("the page has no icon")
	}
}

func TestTheSocketSwitchSavesTheSettingTheAppStartsOn(t *testing.T) {
	p, saved := testPanel(t)
	app := pageTester(p, "")
	if !app.HasText("Not listening. The switch above opens it.") {
		t.Fatalf("the off socket is not described: %q", app.Texts())
	}
	info, ok := app.AccessByRole(ui.RoleSwitch, "Allow external agents")
	if !ok {
		t.Fatal("the page has no switch for external agents")
	}
	app.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	app.Frame()

	if len(*saved) != 1 || (*saved)[0].ExternalIPC == nil || !*(*saved)[0].ExternalIPC {
		t.Fatalf("saved patches = %+v, want externalIPC on", *saved)
	}
	if !p.settings.ExternalIPC {
		t.Fatal("the page did not take the saved setting")
	}
	if !app.HasText("Answering below.") {
		t.Fatalf("the socket still reads off after turning it on: %q", app.Texts())
	}
}

func TestASwitchThatCouldNotOpenTheSocketSaysSo(t *testing.T) {
	p, _ := testPanel(t)
	p.settings.ExternalIPC = true
	p.channel.Listening = func() bool { return false }
	app := pageTester(p, "")
	if !app.HasText("Not answering") {
		t.Fatalf("a switch with no socket behind it reads as listening or off: %q", app.Texts())
	}
	if !app.HasText("Another copy of Mark Go Go may already be listening") {
		t.Fatalf("the page does not say why nothing answers: %q", app.Texts())
	}
}

func TestResetTurnsTheSocketBackOff(t *testing.T) {
	p, saved := testPanel(t)
	p.settings.ExternalIPC = true
	page := p.settingsPage()
	if !page.Dirty() {
		t.Fatal("a page with the channel on does not report a change from the default")
	}
	page.Reset()

	if len(*saved) != 1 || (*saved)[0].ExternalIPC == nil || *(*saved)[0].ExternalIPC {
		t.Fatalf("saved patches = %+v, want externalIPC off", *saved)
	}
	if p.settings.ExternalIPC || page.Dirty() {
		t.Fatal("reset left the channel on")
	}
}

func TestInstallingAndUninstallingTheCommandFromThePage(t *testing.T) {
	p, _ := testPanel(t)
	app := pageTester(p, "")
	if !app.HasText("Not installed") {
		t.Fatalf("a missing command is not reported: %q", app.Texts())
	}
	if err := app.Click("Install the markgogo command"); err != nil {
		t.Fatalf("clicking Install: %v", err)
	}
	app.Frame()
	if _, err := os.Lstat(p.install.Path()); err != nil {
		t.Fatalf("Install left no command: %v", err)
	}
	if !app.HasText("Installed") {
		t.Fatalf("the page still reports the command missing: %q", app.Texts())
	}
	if err := app.Click("Uninstall the markgogo command"); err != nil {
		t.Fatalf("clicking Uninstall: %v", err)
	}
	app.Frame()
	if _, err := os.Lstat(p.install.Path()); !os.IsNotExist(err) {
		t.Fatalf("Uninstall left the command: %v", err)
	}
}

func TestABuildWithoutTheCommandSaysHowToGetOne(t *testing.T) {
	p, _ := testPanel(t)
	p.install.Source = ""
	app := pageTester(p, "")
	if !app.HasText("Not in this build") {
		t.Fatalf("the page does not report a build without markgogo: %q", app.Texts())
	}
	if !app.HasText("go build -o ~/.local/bin/markgogo ./cmd/markgogo") {
		t.Fatalf("the page does not say how to install the command: %q", app.Texts())
	}
	if info, ok := app.Access("Install the markgogo command"); !ok || !info.Disabled {
		t.Fatalf("Install = %+v, %t; want a disabled button", info, ok)
	}
}

func TestCopyingAPromptCopiesTheSelectedHarness(t *testing.T) {
	p, _ := testPanel(t)
	app := pageTester(p, "")
	if err := app.Click("Claude Code harness"); err != nil {
		t.Fatalf("selecting Claude Code: %v", err)
	}
	app.Frame()
	if err := app.Click("Copy the prompt for Claude Code"); err != nil {
		t.Fatalf("copying the prompt: %v", err)
	}
	if got := app.Clipboard(); !strings.Contains(got, "claude mcp add markgogo -- markgogo mcp") || !strings.Contains(got, "ipc.sock") {
		t.Fatalf("copied %q, want the Claude Code prompt", got)
	}

	if err := app.Click("Codex harness"); err != nil {
		t.Fatalf("selecting Codex: %v", err)
	}
	app.Frame()
	if err := app.Click("Copy the prompt for Codex"); err != nil {
		t.Fatalf("copying the Codex prompt: %v", err)
	}
	if got := app.Clipboard(); !strings.Contains(got, "sandbox") {
		t.Fatalf("copied %q, want what Codex is told about its sandbox", got)
	}
}

func TestTheSkillCanBeCopiedAndInstalled(t *testing.T) {
	p, _ := testPanel(t)
	app := pageTester(p, "")
	if err := app.Click("Copy SKILL.md"); err != nil {
		t.Fatalf("copying the skill: %v", err)
	}
	if got := app.Clipboard(); got != agentguide.Skill() {
		t.Fatalf("copied %q, want the static SKILL.md", got)
	}
	if err := app.Click("Install SKILL.md"); err != nil {
		t.Fatalf("installing the skill: %v", err)
	}
	app.Frame()
	path := agentguide.SkillPath(p.userData)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Install SKILL.md wrote nothing: %v", err)
	}
	if !app.HasText("Written to") {
		t.Fatalf("the page does not say where the skill went: %q", app.Texts())
	}
}

func TestTheMCPConfigurationStartsCollapsedAndCanBeCopied(t *testing.T) {
	p, _ := testPanel(t)
	app := pageTester(p, "")
	if app.HasText(agentguide.MCPConfig) {
		t.Fatal("the MCP configuration is on screen before the reader opens it")
	}
	if err := app.Click("Server configuration"); err != nil {
		t.Fatalf("opening the MCP configuration: %v", err)
	}
	// The disclosure opens over Motion.Expand; the tester's clock moves it.
	app.Advance(portrait.Motion.Expand)
	app.Frame()
	if !app.HasText(agentguide.MCPConfig) {
		t.Fatalf("the opened configuration is not on screen: %q", app.Texts())
	}
	if err := app.Click("Copy MCP configuration"); err != nil {
		t.Fatalf("copying the configuration: %v", err)
	}
	if got := app.Clipboard(); got != agentguide.MCPConfig {
		t.Fatalf("copied %q, want the MCP configuration", got)
	}
}

func TestTheLastExternalCallNamesTheClientAndTheTime(t *testing.T) {
	p, _ := testPanel(t)
	at := time.Date(2026, 11, 25, 9, 11, 0, 0, time.UTC)
	p.channel.LastCall = func() (string, time.Time, bool) { return "markgogo-mcp 0.6.0", at, true }
	app := pageTester(p, "")
	if !app.HasText("markgogo-mcp 0.6.0 · 2026-11-25 09:11 UTC") {
		t.Fatalf("the last call is not on screen: %q", app.Texts())
	}

	p.channel.LastCall = func() (string, time.Time, bool) { return "", time.Time{}, false }
	app = pageTester(p, "")
	if !app.HasText("No external agent has used the socket since Mark Go Go started.") {
		t.Fatalf("a socket nobody has used reads oddly: %q", app.Texts())
	}
}

func TestSearchFindsOnlyTheRowsThatMatch(t *testing.T) {
	p, _ := testPanel(t)
	at := time.Date(2026, 11, 25, 9, 11, 0, 0, time.UTC)
	p.channel.LastCall = func() (string, time.Time, bool) { return "markgogo-mcp 0.6.0", at, true }
	app := pageTester(p, "markgogo-mcp")
	if !app.HasText("Last external call") || !app.HasText("markgogo-mcp 0.6.0 · 2026-11-25 09:11 UTC") {
		t.Fatalf("search lost the row that matched: %q", app.Texts())
	}
	if app.HasText("Agent Skill") || app.HasText("Not listening") {
		t.Fatalf("search left unrelated rows: %q", app.Texts())
	}
}
