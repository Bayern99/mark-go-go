// Package nativeai is Settings / AI Integration: what an external agent can
// use, how to install the markgogo command, and how to connect a harness.
package nativeai

import (
	"os"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/agentcli"
	"github.com/Bayern99/mark-go-go/internal/agentguide"
	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

const (
	capabilitiesTitle    = "External Agent Capabilities"
	capabilitiesNote     = "An external agent reads and edits this window's Markdown through the command or the socket below. A GenUI backend is not one: it signs in in Settings / GenUI and only ever writes a document's reading view."
	capabilitiesKeywords = "external agent capabilities overview"

	agentTitle       = "Agent"
	agentDescription = "Pick the harness you are connecting. The prompt and the connection follow the card you choose."
	agentKeywords    = "agent harness codex pi other"

	promptTitle     = "Prompt"
	promptKeywords  = "prompt instructions copy"
	connectTitle    = "Connect"
	connectKeywords = "connect command copy"
	skillTitle      = "Skill"
	skillKeywords   = "skill md instructions install copy"

	commandTitle    = "Command"
	commandKeywords = "command cli markgogo install uninstall"
	pathTitle       = "Install path"
	pathDescription = "Where a harness finds the command. Mark Go Go writes nothing else: no sudo, no /usr/local/bin, no shell profile."
	pathKeywords    = "path folder bin local"

	externalTitle       = "Allow external agents"
	externalDescription = "Listen on this app's own socket for the markgogo command and the agents you connect. Off until you turn it on."
	externalKeywords    = "external agents ipc socket enable listen"

	socketTitle    = "Socket"
	socketKeywords = "socket listen status ipc"

	callsTitle    = "Last external call"
	callsKeywords = "last call client source time agent"

	mcpTitle    = "MCP (stdio)"
	mcpNote     = "The shim forwards a client's standard input and output to the same socket, so a client that only speaks MCP over stdio needs this one line. It carries no port and no token."
	mcpKeywords = "mcp stdio shim configuration server"
)

// Channel reports this app's local socket the way the page shows it.
type Channel struct {
	// Listening reports whether the app answers on its socket now.
	Listening func() bool
	// LastCall reports the most recent request the socket answered: the client
	// that sent it and when. ok is false when nothing has called yet.
	LastCall func() (source string, at time.Time, ok bool)
}

type panel struct {
	install  agentcli.Installer
	channel  Channel
	userData string
	settings workspace.Settings
	save     func(workspace.SettingsPatch) (workspace.Settings, error)

	harness string
	mcpOpen bool
	message string
}

// Install registers the page in every window of the app.
func Install(host *workspace.Service, channel Channel) func(*nativeui.Window) nativeui.Panels {
	return func(win *nativeui.Window) nativeui.Panels {
		p := &panel{
			install:  agentcli.Default(),
			channel:  channel,
			userData: host.UserDataDir(),
			harness:  agentguide.HarnessCodex,
		}
		p.settings, _ = host.GetSettings()
		p.save = host.SetSettings
		workspace.SettingsChanged.Listen(win.Native(), func(next workspace.Settings) {
			p.settings = next
		})
		return nativeui.Panels{Settings: []nativeui.SettingsPage{p.settingsPage()}}
	}
}

func (p *panel) settingsPage() nativeui.SettingsPage {
	return nativeui.SettingsPage{
		ID: "ai-integration", Section: "Workspace", Label: "AI Integration",
		Icon: portrait.IconPlugs, Order: 220,
		Keywords: []string{"ai integration external agent cli markgogo install ipc socket mcp skill prompt codex pi"},
		Dirty:    func() bool { return p.settings.ExternalIPC },
		Reset:    func() { p.setExternalIPC(false) },
		Draw:     p.drawSettingsPage,
	}
}

func (p *panel) drawSettingsPage(c *ui.Context, q nativeui.SettingsQuery) {
	if p.message != "" {
		portrait.Alert(c, portrait.Tones.Danger, "AI Integration", p.message)
	}
	p.drawCapabilities(c, q)
	p.drawCommand(c, q)
	p.drawConnect(c, q)
	p.drawMCP(c, q)
}

func (p *panel) drawCapabilities(c *ui.Context, q nativeui.SettingsQuery) {
	if !q.Match(capabilitiesTitle, capabilitiesNote, capabilitiesKeywords) {
		return
	}
	portrait.SettingsGroup(c, capabilitiesTitle, func(rows *portrait.Rows) {
		rows.Row(portrait.RowSpec{Title: "Ways an agent reaches this window", Stacked: true}, func() {
			grid := ui.Column(c).FillWidth().Gap(portrait.Form.PadX)
			grid.Children(func() {
				cards := p.capabilityCards()
				for start := 0; start < len(cards); start += 2 {
					ui.Row(c).FillWidth().Gap(portrait.Form.PadX).AlignItems(ui.Stretch).Children(func() {
						for column := 0; column < 2 && start+column < len(cards); column++ {
							p.capabilityCard(c, cards[start+column])
						}
					})
				}
			})
			portrait.Caption(c, capabilitiesNote).FillWidth().MinWidth(0).Wrap()
		})
	})
}

// capability is one of the four cards that summarize the page.
type capability struct {
	key, title, caption, status string
	tone                        portrait.Tone
}

func (p *panel) capabilityCards() []capability {
	command := p.command()
	skill := p.skill()
	return []capability{
		{key: "cli", title: "Command Line Tool", caption: "markgogo in your own bin folder.", status: command.status, tone: command.tone},
		{key: "prompt", title: "Agent Prompt", caption: "Instructions for the harness you pick.", status: agentguide.HarnessByID(p.harness).Name, tone: portrait.Tones.Neutral},
		{key: "skill", title: "Agent Skill", caption: "A SKILL.md that teaches an agent the command.", status: skill.status, tone: skill.tone},
		{key: "mcp", title: "MCP (stdio)", caption: "A command and its arguments, nothing else.", status: "No port, no token", tone: portrait.Tones.Neutral},
	}
}

func (p *panel) capabilityCard(c *ui.Context, card capability) {
	box := ui.Column(c.Key("capability-"+card.key)).Grow(1).MinWidth(0).AlignItems(ui.Start).
		Padding(portrait.Form.PadX).Radius(portrait.Radii.L).Border(portrait.Editor.Hairline, portrait.Colors.Line).
		Background(portrait.Colors.Fill).Role(ui.RoleGroup).Label(card.title)
	box.Children(func() {
		ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			portrait.Label(c, card.title).MaxLines(1).Ellipsis("…")
			ui.Row(c).Grow(1).MinWidth(0).Justify(ui.End).Children(func() {
				portrait.Badge(c, card.status, card.tone)
			})
		})
		portrait.Caption(c, card.caption).FillWidth().MinWidth(0).Wrap()
	})
}

// command is the state of the markgogo command the page describes.
type commandState struct {
	status, description string
	tone                portrait.Tone
}

func (p *panel) command() commandState {
	state := p.install.State()
	switch {
	case !p.install.Shipped():
		return commandState{
			status: "Not in this build", tone: portrait.Tones.Warning,
			description: "This build does not carry the markgogo command. Build it from a checkout of github.com/Bayern99/mark-go-go with `go build -o ~/.local/bin/markgogo ./cmd/markgogo` and keep ~/.local/bin on your PATH.",
		}
	case state == agentcli.StateInstalled:
		return commandState{
			status: "Installed", tone: portrait.Tones.Success,
			description: "Linked from this app into your own bin folder.",
		}
	case state == agentcli.StateForeign:
		return commandState{
			status: "In the way", tone: portrait.Tones.Warning,
			description: "Another program's markgogo is at this path. Mark Go Go leaves it alone; remove it yourself to install this app's command.",
		}
	default:
		return commandState{
			status: "Not installed", tone: portrait.Tones.Neutral,
			description: "Link the markgogo command this app carries into your own bin folder, so a harness can run it.",
		}
	}
}

func (p *panel) drawCommand(c *ui.Context, q nativeui.SettingsQuery) {
	command := p.command()
	installed := p.install.State() == agentcli.StateInstalled
	socket := p.socket()
	showCommand := q.Match(commandTitle, command.description, commandKeywords)
	showPath := q.Match(pathTitle, pathDescription, pathKeywords)
	showExternal := q.Match(externalTitle, externalDescription, externalKeywords)
	showSocket := q.Match(socketTitle, socket.description, socketKeywords)
	calls := p.calls()
	showCalls := q.Match(callsTitle, calls, callsKeywords)
	if !showCommand && !showPath && !showExternal && !showSocket && !showCalls {
		return
	}
	portrait.SettingsGroup(c, "Command Line Tool", func(rows *portrait.Rows) {
		if showCommand {
			rows.Row(portrait.RowSpec{Title: commandTitle, Description: command.description}, func() {
				if installed {
					if portrait.Button(c, "Uninstall", "Uninstall the markgogo command").Clicked() {
						p.removeCommand()
					}
					return
				}
				install := portrait.AccentButton(c, "Install", "Install the markgogo command")
				install.Disabled(!p.install.Shipped())
				if install.Clicked() {
					p.addCommand()
				}
			})
		}
		if showPath {
			rows.Row(portrait.RowSpec{Title: pathTitle, Description: pathDescription, Stacked: true}, func() {
				portrait.Type.CodeSmall.Apply(ui.Text(c, shortPath(p.install.Path()))).TextColor(portrait.Colors.Ink2).Selectable().Wrap()
			})
		}
		if showExternal {
			rows.Row(portrait.RowSpec{Title: externalTitle, Description: externalDescription}, func() {
				on := p.settings.ExternalIPC
				if portrait.Switch(c.Key("ai-external-ipc"), &on).Label(externalTitle).Changed() {
					p.setExternalIPC(on)
				}
			})
		}
		if showSocket {
			rows.Row(portrait.RowSpec{Title: socketTitle, Description: socket.description, Stacked: true}, func() {
				ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					portrait.Badge(c, socket.status, socket.tone)
					portrait.Type.CodeSmall.Apply(ui.Text(c, shortPath(ipc.SocketPath(p.userData)))).TextColor(portrait.Colors.Ink2).Selectable()
				})
			})
		}
		if showCalls {
			rows.Row(portrait.RowSpec{Title: callsTitle, Description: calls}, nil)
		}
	})
}

// socket is what the page says about the local channel now.
type socketState struct {
	status, description string
	tone                portrait.Tone
}

func (p *panel) socket() socketState {
	if p.listening() {
		return socketState{
			status: "Listening", tone: portrait.Tones.Success,
			description: "Answering below. The folder and the socket belong to your user alone.",
		}
	}
	// A switch that is on while nothing answers means the app could not take
	// the socket, usually because another copy of it is serving that path.
	if p.settings.ExternalIPC {
		return socketState{
			status: "Not answering", tone: portrait.Tones.Warning,
			description: "The switch is on, but nothing answers on the socket. Another copy of Mark Go Go may already be listening; quit it, then switch this off and on again.",
		}
	}
	return socketState{
		status: "Off", tone: portrait.Tones.Neutral,
		description: "Not listening. The switch above opens it.",
	}
}

// calls describes the last external request, or says nobody has called yet.
func (p *panel) calls() string {
	if p.channel.LastCall != nil {
		if source, at, ok := p.channel.LastCall(); ok {
			return source + " · " + at.UTC().Format("2006-01-02 15:04 UTC")
		}
	}
	return "No external agent has used the socket since Mark Go Go started."
}

func (p *panel) listening() bool {
	return p.channel.Listening != nil && p.channel.Listening()
}

func (p *panel) drawConnect(c *ui.Context, q nativeui.SettingsQuery) {
	harness := agentguide.HarnessByID(p.harness)
	skill := p.skill()
	showAgent := q.Match(agentTitle, agentDescription, agentKeywords)
	showPrompt := q.Match(promptTitle, harness.Summary, promptKeywords)
	showConnect := q.Match(connectTitle, harness.Connect, connectKeywords)
	showSkill := q.Match(skillTitle, p.skill().description, skillKeywords)
	if !showAgent && !showPrompt && !showConnect && !showSkill {
		return
	}
	portrait.SettingsGroup(c, "Connect an Agent", func(rows *portrait.Rows) {
		if showAgent {
			rows.Row(portrait.RowSpec{Title: agentTitle, Description: agentDescription, Stacked: true}, func() {
				p.drawHarnessCards(c)
			})
		}
		if showPrompt {
			rows.Row(portrait.RowSpec{Title: promptTitle, Description: harness.Summary}, func() {
				if portrait.AccentButton(c, "Copy prompt", "Copy the prompt for "+harness.Name).Clicked() {
					p.copy(c, "Copied prompt", agentguide.Prompt(harness.ID, ipc.SocketPath(p.userData)))
				}
			})
		}
		if showConnect {
			rows.Row(portrait.RowSpec{Title: connectTitle, Description: harness.Connect}, func() {
				if portrait.ButtonWith(c, "Copy", "Copy what connects "+harness.Name, portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconCopy}).Clicked() {
					p.copy(c, "Copied connection", harness.Connect)
				}
			})
		}
		if showSkill {
			rows.Row(portrait.RowSpec{Title: skillTitle, Description: skill.description}, func() {
				ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					install := portrait.Button(c, "Install SKILL.md", "Install SKILL.md")
					if install.Clicked() {
						p.installSkill()
					}
					if portrait.ButtonWith(c, "Copy", "Copy SKILL.md", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconCopy}).Clicked() {
						p.copy(c, "Copied SKILL.md", agentguide.Skill())
					}
				})
			})
		}
	})
}

func (p *panel) drawHarnessCards(c *ui.Context) {
	grid := ui.Column(c).FillWidth().Gap(portrait.Form.PadX)
	grid.Children(func() {
		harnesses := agentguide.Harnesses
		for start := 0; start < len(harnesses); start += 2 {
			ui.Row(c).FillWidth().Gap(portrait.Form.PadX).AlignItems(ui.Stretch).Children(func() {
				for column := 0; column < 2 && start+column < len(harnesses); column++ {
					p.harnessCard(c, harnesses[start+column])
				}
			})
		}
	})
}

func (p *panel) harnessCard(c *ui.Context, harness agentguide.Harness) {
	selected := harness.ID == p.harness
	card := ui.ButtonBase(c.Key("harness-"+harness.ID)).Label(harness.Name+" harness").
		Grow(1).MinWidth(0).AlignItems(ui.Start).Padding(portrait.Form.PadX).Radius(portrait.Radii.L).
		Border(portrait.Editor.Hairline, portrait.Colors.Line).Background(portrait.Colors.Fill).
		AccessSelected(selected)
	if selected {
		card.Border(portrait.Editor.Hairline, portrait.Colors.Accent).Background(portrait.Colors.AccentSoft)
	}
	card.Children(func() {
		ui.Column(c).FillWidth().Gap(portrait.Form.HelpGap).AlignItems(ui.Start).Children(func() {
			portrait.Label(c, harness.Name).MaxLines(1).Ellipsis("…")
			portrait.Caption(c, harness.Summary).FillWidth().MinWidth(0).Wrap()
		})
	})
	if card.Clicked() && !selected {
		p.harness = harness.ID
	}
}

// skill is what the page says about the SKILL.md in the app's own folder.
type skillState struct {
	status, description string
	tone                portrait.Tone
}

func (p *panel) skill() skillState {
	path := agentguide.SkillPath(p.userData)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return skillState{
			status: "Installed", tone: portrait.Tones.Success,
			description: "Written to " + shortPath(path) + ". Copy it into the place your harness reads skills from.",
		}
	}
	return skillState{
		status: "Not installed", tone: portrait.Tones.Neutral,
		description: "Writes SKILL.md into Mark Go Go's own folder; Mark Go Go does not write into another program's configuration.",
	}
}

func (p *panel) drawMCP(c *ui.Context, q nativeui.SettingsQuery) {
	if !q.Match(mcpTitle, mcpNote, mcpKeywords) {
		return
	}
	if q.Active() {
		// A matched row has to be on screen, so search opens the disclosure.
		p.mcpOpen = true
	}
	ui.Column(c).FillWidth().Gap(portrait.Settings.HeadingGap).Children(func() {
		portrait.GroupHeading(c, mcpTitle)
		portrait.Disclosure(c.Key("ai-mcp"), "Server configuration", &p.mcpOpen, portrait.DisclosureSpec{}, func() {
			ui.Column(c).FillWidth().Gap(portrait.Form.Gap).Children(func() {
				portrait.Caption(c, mcpNote).FillWidth().MinWidth(0).Wrap()
				portrait.OutputBlock(c, "MCP configuration", agentguide.MCPConfig)
			})
		})
	})
}

func (p *panel) addCommand() {
	if err := p.install.Install(); err != nil {
		p.message = err.Error()
		return
	}
	p.message = ""
}

func (p *panel) removeCommand() {
	if err := p.install.Uninstall(); err != nil {
		p.message = err.Error()
		return
	}
	p.message = ""
}

func (p *panel) setExternalIPC(on bool) {
	if p.save == nil {
		p.message = "Mark Go Go cannot save this setting."
		return
	}
	next, err := p.save(workspace.SettingsPatch{ExternalIPC: &on})
	if err != nil {
		p.message = err.Error()
		return
	}
	p.settings, p.message = next, ""
}

func (p *panel) installSkill() {
	if _, err := agentguide.InstallSkill(p.userData); err != nil {
		p.message = err.Error()
		return
	}
	p.message = ""
}

// copy puts text on the clipboard and says so; an empty harness line copies
// nothing.
func (p *panel) copy(c *ui.Context, title, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	c.WriteClipboard(text)
	portrait.Notify(c, portrait.ToastSpec{ID: "ai-integration-" + title, Tone: portrait.Tones.Success, Title: title})
}

// shortPath names a path under the home folder with a ~, so a long library
// path stays readable in a row.
func shortPath(path string) string {
	if path == "" {
		return "Not set"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	return strings.Replace(path, home+string(os.PathSeparator), "~"+string(os.PathSeparator), 1)
}
