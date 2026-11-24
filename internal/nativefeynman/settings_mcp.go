package nativefeynman

import (
	"net/url"
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
			transport := p.serverTransport
			if transport == "" {
				transport = "http"
			}
			transportError := mcpTransportValidation(transport, p.serverURL, p.serverCommand)
			validation := mcpServerValidation(p.serverName, transportError)
			showTransportError := transportError != "" && ((transport == "stdio" && strings.TrimSpace(p.serverCommand) != "") || (transport != "stdio" && strings.TrimSpace(p.serverURL) != ""))
			displayedTransportError := ""
			if showTransportError {
				displayedTransportError = transportError
			}
			ui.Column(c.Key("feynman-mcp-add-form")).Gap(portrait.Form.Gap).Padding(portrait.Form.AreaPadY, 0, 0, 0).BorderWidth(portrait.Editor.Hairline, 0, 0, 0).BorderColor(portrait.Colors.Line).Children(func() {
				portrait.FormRow(c, "Name", "", func() { input(c, "feynman-mcp-name", "Server name", &p.serverName, "Name") })
				portrait.FormRow(c, "Transport", "", func() {
					selected := 0
					if transport == "stdio" {
						selected = 1
					}
					portrait.Segmented(c.Key("research-transport"), &selected, []string{"HTTP", "Stdio"}, "MCP transport")
					if selected == 0 {
						p.serverTransport = "http"
					} else {
						p.serverTransport = "stdio"
					}
				})
				if transport == "stdio" {
					portrait.FormRow(c, "Command", displayedTransportError, func() {
						validatedInput(c, "feynman-mcp-command", "Command", &p.serverCommand, "Command path", showTransportError)
					})
					portrait.FormRow(c, "Arguments", "", func() { input(c, "feynman-mcp-args", "Arguments", &p.serverArgs, "Space separated arguments") })
					portrait.FormRow(c, "Environment", "One KEY=value pair per line.", func() {
						area(c, "feynman-mcp-env", "Environment variables", &p.serverEnv, "KEY=value, one per line", portrait.Form.AreaMin)
					})
					portrait.FormRow(c, "Working folder", "", func() { input(c, "feynman-mcp-cwd", "Working folder", &p.serverCwd, "Optional working directory") })
				} else {
					portrait.FormRow(c, "URL", displayedTransportError, func() {
						validatedInput(c, "feynman-mcp-url", "HTTP URL", &p.serverURL, "https://…", showTransportError)
					})
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
				if transport == "http" {
					portrait.FormRow(c, "Bearer token env var", "", func() {
						input(c, "feynman-mcp-bearer-env", "Bearer token environment variable", &p.serverBearerEnv, "Optional environment variable name")
					})
				}
				portrait.FormRow(c, "Project-local", "Save this server to the project instead of your user configuration.", func() {
					portrait.Checkbox(c.Key("feynman-mcp-local"), &p.serverLocal, "Project-local configuration")
				})
				ui.Row(c).Justify(ui.End).Gap(portrait.Form.Gap).Children(func() {
					if portrait.QuietButton(c, "Cancel", "Cancel adding MCP server").Clicked() {
						p.mcpAddOpen = false
					}
					if portrait.ButtonWith(c, "Add server", "Add MCP server to Feynman", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular, Disabled: s.cliBusy || validation != ""}).Clicked() {
						p.addMCPServer()
					}
				})
			})
		}
	})
}

func validatedInput(c *ui.Context, key, label string, value *string, placeholder string, invalid bool) ui.Element {
	field := portrait.TextField(c.Key(key), value, portrait.FieldSpec{Label: label, Placeholder: placeholder, Size: portrait.Form.Field, Invalid: invalid})
	field.Frame().FillWidth().MinWidth(0)
	return field.Frame()
}

func mcpTransportValidation(transport, serverURL, command string) string {
	if transport == "stdio" {
		if strings.TrimSpace(command) == "" {
			return "Enter a stdio command."
		}
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "Enter a valid http:// or https:// URL with a host."
	}
	return ""
}

func mcpServerValidation(name, transportError string) string {
	if strings.TrimSpace(name) == "" {
		return "Enter a server name."
	}
	return transportError
}

func (p *panel) removeMCPServer(server feynman.CLIMCPServer) {
	s := p.state
	p.runSettingsUpdate(func() (func(), error) {
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
	transport := p.serverTransport
	if transport == "" {
		transport = "http"
	}
	if mcpServerValidation(p.serverName, mcpTransportValidation(transport, p.serverURL, p.serverCommand)) != "" {
		return
	}
	inputValue := feynman.CLIMCPAddInput{
		Name:              strings.TrimSpace(p.serverName),
		Transport:         transport,
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
	p.runSettingsUpdate(func() (func(), error) {
		result, err := p.host.CLIMCPAdd(p.window.Context(), root, inputValue)
		rows, rowsErr := p.host.CLIMCPList(p.window.Context(), root)
		return func() {
			p.state.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			p.state.mcpServers = &rows
		}, errorsJoin(err, rowsErr)
	})
}
