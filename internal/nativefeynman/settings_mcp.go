package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) mcpSettings(c *ui.Context) {
	s := p.state
	settingsSection(c, "MCP Servers", "Configured servers", "Manage MCP servers and their tool exposure.", func() {
		if !p.mcpAddOpen {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				if portrait.Button(c, "Add server…", "Open the add MCP server form").Clicked() {
					p.mcpAddOpen = true
				}
			})
		}
		if s.mcpServers != nil {
			if !s.mcpServers.Parsed {
				resultText(c, s.mcpServers.Command, s.mcpServers.RawOutput, s.mcpServers.Error)
			} else if len(s.mcpServers.Servers) == 0 {
				mutedText(c, "No MCP servers configured.")
			} else {
				for _, server := range s.mcpServers.Servers {
					ui.Row(c.Key("mcp-server-" + server.Name)).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							text(c, portrait.Type.UI, server.Name).FontWeight(600).TextColor(portrait.Colors.Ink)
							mutedText(c, strings.Join([]string{server.Scope, server.Transport, server.Status}, " · "))
						})
						trigger := portrait.ButtonWith(c, "Remove", "Remove MCP server "+server.Name, portrait.ButtonSpec{Look: portrait.DangerOutline, Size: portrait.Sizes.Regular}).Disabled(s.cliBusy)
						p.confirmOnClick("remove-mcp:"+server.Scope+":"+server.Name, trigger, portrait.ConfirmSpec{
							Title:        "Remove MCP server?",
							Body:         "Remove “" + server.Name + "” from Feynman? This action cannot be undone.",
							ConfirmLabel: "Remove",
							OnConfirm:    func() { p.removeMCPServer(server) },
						})
					})
				}
			}
		}
		if p.mcpAddOpen {
			ui.Column(c.Key("feynman-mcp-add-form")).Gap(portrait.Form.Gap).Padding(portrait.Form.AreaPadY, 0, 0, 0).BorderWidth(portrait.Editor.Hairline, 0, 0, 0).BorderColor(portrait.Colors.Line).Children(func() {
				portrait.FormRow(c, "Name", "", func() { input(c, "feynman-mcp-name", "Server name", &p.serverName, "Name") })
				portrait.FormRow(c, "Transport", "", func() {
					activeTransport := p.serverTransport
					if activeTransport == "" {
						activeTransport = "http"
					}
					portrait.Track(c, "research-transport", func(track *portrait.Segments) {
						for _, transport := range []string{"http", "stdio"} {
							if track.Segment(c, transport, "Use "+transport+" transport", activeTransport == transport).Clicked() {
								p.serverTransport = transport
							}
						}
					}).Label("MCP transport").AlignSelf(ui.Start)
				})
				if p.serverTransport == "stdio" {
					portrait.FormRow(c, "Command", "", func() { input(c, "feynman-mcp-command", "Command", &p.serverCommand, "Command path") })
					portrait.FormRow(c, "Arguments", "", func() { input(c, "feynman-mcp-args", "Arguments", &p.serverArgs, "Space separated arguments") })
					portrait.FormRow(c, "Environment", "One KEY=value pair per line.", func() {
						area(c, "feynman-mcp-env", "Environment variables", &p.serverEnv, "KEY=value, one per line", portrait.Form.AreaMin)
					})
					portrait.FormRow(c, "Working folder", "", func() { input(c, "feynman-mcp-cwd", "Working folder", &p.serverCwd, "Optional working directory") })
				} else {
					portrait.FormRow(c, "URL", "", func() { input(c, "feynman-mcp-url", "HTTP URL", &p.serverURL, "https://…") })
					portrait.FormRow(c, "Headers", "One Header: value pair per line.", func() {
						area(c, "feynman-mcp-headers", "Headers", &p.serverHeaders, "Header: value, one per line", portrait.Form.AreaMin)
					})
				}
				ui.Row(c).Wrap().Gap(portrait.Form.Gap).Children(func() {
					portrait.FormRow(c, "Exposure", "How server tools appear to the model.", func() {
						portrait.Select(c, &p.serverExposure, []portrait.Option{{Value: "", Label: "Default"}, {Value: "direct", Label: "Direct"}}, portrait.SelectSpec{Label: "Exposure", Width: portrait.Form.SelectMin})
					})
					portrait.FormRow(c, "Description", "", func() {
						input(c, "feynman-mcp-description", "Description", &p.serverDescription, "Optional description")
					})
				})
				if p.serverTransport == "http" {
					portrait.FormRow(c, "Bearer token env var", "", func() {
						input(c, "feynman-mcp-bearer-env", "Bearer token environment variable", &p.serverBearerEnv, "Optional environment variable name")
					})
				}
				portrait.FormRow(c, "Project-local", "Save this server to the project instead of your user configuration.", func() {
					portrait.Switch(c.Key("feynman-mcp-local"), &p.serverLocal).Label("Project-local configuration")
				})
				ui.Row(c).Justify(ui.End).Gap(portrait.Form.Gap).Children(func() {
					if portrait.QuietButton(c, "Cancel", "Cancel adding MCP server").Clicked() {
						p.mcpAddOpen = false
					}
					if portrait.AccentButton(c, "Add server", "Add MCP server to Feynman").Disabled(s.cliBusy || strings.TrimSpace(p.serverName) == "").Clicked() {
						p.addMCPServer()
					}
				})
			})
		}
	})
}

func (p *panel) removeMCPServer(server feynman.CLIMCPServer) {
	s := p.state
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIMCPRemove(p.window.Context(), s.root, server.Name, server.Scope == "project")
		rows, rowsErr := p.host.CLIMCPList(p.window.Context(), s.root)
		return func() {
			s.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			s.mcpServers = &rows
		}, errorsJoin(err, rowsErr)
	})
}

func (p *panel) addMCPServer() {
	root := p.state.root
	inputValue := feynman.CLIMCPAddInput{
		Name:              strings.TrimSpace(p.serverName),
		Transport:         p.serverTransport,
		URL:               strings.TrimSpace(p.serverURL),
		Headers:           splitLines(p.serverHeaders),
		BearerTokenEnvVar: strings.TrimSpace(p.serverBearerEnv),
		Command:           strings.TrimSpace(p.serverCommand),
		Args:              strings.Fields(p.serverArgs),
		Env:               splitLines(p.serverEnv),
		Cwd:               strings.TrimSpace(p.serverCwd),
		Exposure:          strings.TrimSpace(p.serverExposure),
		Description:       strings.TrimSpace(p.serverDescription),
		Local:             p.serverLocal,
	}
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIMCPAdd(p.window.Context(), root, inputValue)
		rows, rowsErr := p.host.CLIMCPList(p.window.Context(), root)
		return func() {
			p.state.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			p.state.mcpServers = &rows
		}, errorsJoin(err, rowsErr)
	})
}
