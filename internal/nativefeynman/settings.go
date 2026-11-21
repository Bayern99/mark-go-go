package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) loadSettings(root string) {
	if root == "" || p.state.cliBusy {
		return
	}
	p.settingsRoot = root
	p.state.cliBusy = true
	p.state.cliError = ""
	go func() {
		status, statusErr := p.host.CLIStatus(p.window.Context(), root)
		doctor, doctorErr := p.host.CLIDoctor(p.window.Context(), root)
		models, modelsErr := p.host.CLIModelList(p.window.Context(), root)
		alpha, alphaErr := p.host.CLIAlphaStatus(p.window.Context(), root)
		search, searchErr := p.host.CLISearchStatus(p.window.Context(), root)
		mcp, mcpErr := p.host.CLIMCPList(p.window.Context(), root)
		packages, packagesErr := p.host.CLIPackagesList(p.window.Context(), root)
		p.update(root, func() {
			p.state.cliBusy = false
			p.state.cliStatus = ptrIfOK(status, statusErr)
			p.state.doctorResult = ptrIfOK(doctor, doctorErr)
			p.state.cliModels = ptrIfOK(models, modelsErr)
			p.state.alphaStatus = ptrIfOK(alpha, alphaErr)
			p.state.searchStatus = ptrIfOK(search, searchErr)
			p.state.mcpServers = ptrIfOK(mcp, mcpErr)
			p.state.packages = ptrIfOK(packages, packagesErr)
			for _, err := range []error{statusErr, doctorErr, modelsErr, alphaErr, searchErr, mcpErr, packagesErr} {
				if err != nil {
					p.state.cliError = err.Error()
					break
				}
			}
		})
	}()
}

func ptrIfOK[T any](value T, err error) *T {
	if err != nil {
		return nil
	}
	return &value
}

func (p *panel) settings(c *ui.Context) {
	s := p.state
	ui.Row(c.Key("feynman-settings-header")).Gap(4).AlignItems(ui.Center).Children(func() {
		sectionTitle(c, "Feynman settings")
		ui.Row(c).Grow(1)
		if smallButton(c, "Refresh", "Refresh Feynman CLI settings").Disabled(s.cliBusy).Clicked() {
			p.settingsRoot = ""
			p.loadSettings(s.root)
		}
	})
	if s.cliError != "" {
		text(c, portrait.Type.Meta, s.cliError).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).FillWidth().Wrap()
	}
	if s.cliResult != "" {
		resultText(c, "Command result", s.cliResult, "")
	}
	ui.Scroll(c.Key("feynman-settings-scroll")).Grow(1).MinHeight(0).FillWidth().TrackScroll(&p.settingsScr).Label("Feynman settings and CLI controls").Children(func() {
		p.statusSettings(c)
		p.modelSettings(c)
		p.alphaSettings(c)
		p.searchSettings(c)
		p.mcpSettings(c)
		p.packageSettings(c)
	})
}

func (p *panel) statusSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-status-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		sectionTitle(c, "Status and diagnostics")
		ui.Row(c).Wrap().Gap(4).Children(func() {
			if smallButton(c, "Run Doctor", "Run feynman doctor").Disabled(s.cliBusy).Clicked() {
				p.runUpdate(func() (func(), error) {
					result, err := p.host.CLIDoctor(p.window.Context(), s.root)
					return func() { s.doctorResult = &result }, err
				})
			}
			if smallButton(c, "Verify Preview", "Run feynman setup preview").Disabled(s.cliBusy).Clicked() {
				p.runUpdate(func() (func(), error) {
					result, err := p.host.CLISetupPreview(p.window.Context(), s.root)
					return func() { s.setupPreview = &result }, err
				})
			}
			if smallButton(c, "Run Setup in Terminal", "Launch the Feynman setup wizard in Terminal").Disabled(s.cliBusy).Clicked() {
				p.launchWizard("setup", "")
			}
		})
		if s.cliStatus == nil {
			mutedText(c, "Feynman CLI status has not loaded.")
		} else if s.cliStatus.Parsed {
			for _, section := range s.cliStatus.Sections {
				ui.Collapsible(c.Key("cli-status-"+section.Title), section.Title, p.collapsibleState("cli-status-"+section.Title), func() {
					for _, entry := range section.Entries {
						ui.Row(c).Gap(5).Children(func() {
							text(c, portrait.Type.Meta, entry.Key).Grow(1).TextColor(portrait.Colors.Ink3)
							text(c, portrait.Type.Meta, entry.Value).TextColor(portrait.Colors.Ink).FillWidth()
						})
					}
				}).FillWidth()
			}
		} else {
			resultText(c, s.cliStatus.Command, s.cliStatus.RawOutput, s.cliStatus.Error)
		}
		if s.doctorResult != nil {
			if s.doctorResult.Parsed {
				status := "Needs attention"
				if s.doctorResult.Healthy {
					status = "Healthy"
				}
				mutedText(c, "Doctor · "+status)
				for _, issue := range s.doctorResult.Issues {
					ui.Row(c).Gap(6).AlignItems(ui.Start).FillWidth().Children(func() {
						portrait.Glyph(c, portrait.IconWarningCircle, 13, portrait.Colors.Warn).Margin(1, 0, 0, 0)
						text(c, portrait.Type.Meta, issue).TextColor(portrait.Colors.Warn).Grow(1).MinWidth(0).Wrap()
					})
				}
			} else {
				resultText(c, s.doctorResult.Command, s.doctorResult.RawOutput, s.doctorResult.Error)
			}
		}
		if s.setupPreview != nil {
			resultText(c, s.setupPreview.Command, s.setupPreview.RawOutput, s.setupPreview.Error)
		}
	})
}

func (p *panel) modelSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-model-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		sectionTitle(c, "Models and service tier")
		ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
			input(c, "feynman-provider-login", "Model provider", &p.loginProvider, "Optional provider, such as anthropic")
			if smallButton(c, "Sign in in Terminal", "Sign in to a model provider in Terminal").Disabled(s.cliBusy).Clicked() {
				p.launchWizard("model_login", strings.TrimSpace(p.loginProvider))
			}
		})
		ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
			input(c, "feynman-service-tier", "Service tier", &p.tierValue, "Tier value")
			if smallButton(c, "Apply tier", "Set the Feynman service tier").Disabled(s.cliBusy || strings.TrimSpace(p.tierValue) == "").Clicked() {
				p.modelAction("tier", strings.TrimSpace(p.tierValue))
			}
		})
		if s.cliModels == nil {
			mutedText(c, "No authenticated models loaded.")
		} else if !s.cliModels.Parsed {
			resultText(c, s.cliModels.Command, s.cliModels.RawOutput, s.cliModels.Error)
		} else if len(s.cliModels.Models) == 0 {
			mutedText(c, "No authenticated models. Use Sign in in Terminal.")
		} else {
			for _, model := range s.cliModels.Models {
				p.cliModelRow(c, model)
			}
		}
		if s.session.Model != nil {
			mutedText(c, "Active research model · "+s.session.Model.Provider+"/"+s.session.Model.ID)
		}
	})
}

func (p *panel) cliModelRow(c *ui.Context, model feynman.CLIModelEntry) {
	label := model.Name
	if label == "" {
		label = model.ID
	}
	ui.Row(c.Key("feynman-model-"+model.ID)).Wrap().Gap(4).AlignItems(ui.Center).Padding(3, 0).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			text(c, portrait.Type.UI, label).FontWeight(600).TextColor(portrait.Colors.Ink)
			meta := model.Provider
			if model.Current {
				meta += " · current"
			}
			if model.Recommended {
				meta += " · recommended"
			}
			mutedText(c, meta)
		})
		if smallButton(c, "Set default", "Set "+label+" as the default model").Disabled(model.Current || p.state.cliBusy).Clicked() {
			p.modelAction("set", model.Name)
		}
		if smallButton(c, "Log out", "Log out of "+model.Provider).Disabled(p.state.cliBusy).Clicked() {
			p.modelAction("logout", model.Provider)
		}
	})
}

func (p *panel) modelAction(action, value string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIModelAction(p.window.Context(), root, action, value)
		return func() {
			p.state.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			p.loadSettings(root)
			if p.state.status == statusReady {
				p.run(func() error { _, err := p.host.RefreshModels(p.window.Context(), root); return err })
			}
		}, err
	})
}

func (p *panel) alphaSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-alpha-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		sectionTitle(c, "alphaXiv account")
		if s.alphaStatus == nil {
			mutedText(c, "alphaXiv status has not loaded.")
		} else if s.alphaStatus.Parsed {
			label := s.alphaStatus.StatusText
			if s.alphaStatus.LoggedIn {
				label = "Signed in"
				if s.alphaStatus.Account != "" {
					label += " as " + s.alphaStatus.Account
				}
			}
			mutedText(c, label)
		} else {
			resultText(c, s.alphaStatus.Command, s.alphaStatus.RawOutput, s.alphaStatus.Error)
		}
		ui.Row(c).Wrap().Gap(4).Children(func() {
			if smallButton(c, "Sign in to alphaXiv in Terminal", "Sign in to alphaXiv using Terminal").Disabled(s.cliBusy).Clicked() {
				p.launchWizard("alpha_login", "")
			}
			if s.alphaStatus != nil && s.alphaStatus.LoggedIn && smallButton(c, "Log out", "Log out from alphaXiv").Disabled(s.cliBusy).Clicked() {
				p.runUpdate(func() (func(), error) {
					result, err := p.host.CLIAlphaLogout(p.window.Context(), s.root)
					return func() { s.cliResult = firstNonEmpty(result.Summary, result.RawOutput) }, err
				})
				p.loadSettings(s.root)
			}
		})
		if smallButton(c, "Open alphaXiv research", "Open alphaXiv search and paper tools").Clicked() {
			s.selectedTab = "alpha"
		}
	})
}

func (p *panel) searchSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-search-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, "Web search")
			ui.Row(c).Grow(1)
			if smallButton(c, "Reset to Auto", "Reset search routing to automatic").Disabled(s.cliBusy).Clicked() {
				p.runUpdate(func() (func(), error) {
					result, err := p.host.CLISearchAction(p.window.Context(), s.root, "clear", "", "")
					status, statusErr := p.host.CLISearchStatus(p.window.Context(), s.root)
					return func() {
						s.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
						s.searchStatus = &status
						if status.RequestRoute != "" {
							p.searchProvider = status.RequestRoute
						}
					}, errorsJoin(err, statusErr)
				})
			}
		})
		if s.searchStatus != nil {
			if s.searchStatus.Parsed {
				for _, entry := range s.searchStatus.Entries {
					ui.Row(c).Gap(5).Children(func() {
						text(c, portrait.Type.Meta, entry.Key).Grow(1).TextColor(portrait.Colors.Ink3)
						text(c, portrait.Type.Meta, entry.Value).TextColor(portrait.Colors.Ink)
					})
				}
				for _, provider := range strings.Split(s.searchStatus.SavedAPIKeys, ",") {
					provider = strings.TrimSpace(provider)
					if provider == "" || provider == "none" {
						continue
					}
					provider := provider
					if smallButton(c, "Clear "+provider+" API key", "Remove the saved "+provider+" API key").Disabled(s.cliBusy).Clicked() {
						p.runUpdate(func() (func(), error) {
							result, err := p.host.CLISearchAction(p.window.Context(), s.root, "clear-key", provider, "")
							status, statusErr := p.host.CLISearchStatus(p.window.Context(), s.root)
							return func() { s.cliResult = firstNonEmpty(result.Summary, result.RawOutput); s.searchStatus = &status }, errorsJoin(err, statusErr)
						})
					}
				}
			} else {
				resultText(c, s.searchStatus.Command, s.searchStatus.RawOutput, s.searchStatus.Error)
			}
		}
		input(c, "feynman-search-provider", "Search provider", &p.searchProvider, "auto, exa, brave, tavily…")
		key := input(c, "feynman-search-key", "Optional API key", &p.searchKey, "Leave blank to use existing authentication").Password()
		_ = key
		if smallButton(c, "Save search settings", "Set the selected web search provider and API key").Disabled(s.cliBusy || strings.TrimSpace(p.searchProvider) == "").Clicked() {
			p.runUpdate(func() (func(), error) {
				result, err := p.host.CLISearchAction(p.window.Context(), s.root, "set", strings.TrimSpace(p.searchProvider), strings.TrimSpace(p.searchKey))
				status, statusErr := p.host.CLISearchStatus(p.window.Context(), s.root)
				return func() {
					s.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
					s.searchStatus = &status
					p.searchKey = ""
				}, errorsJoin(err, statusErr)
			})
		}
	})
}

func (p *panel) mcpSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-mcp-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		sectionTitle(c, "MCP servers")
		if s.mcpServers != nil {
			if !s.mcpServers.Parsed {
				resultText(c, s.mcpServers.Command, s.mcpServers.RawOutput, s.mcpServers.Error)
			} else {
				for _, server := range s.mcpServers.Servers {
					ui.Row(c.Key("mcp-server-" + server.Name)).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							text(c, portrait.Type.UI, server.Name).FontWeight(600).TextColor(portrait.Colors.Ink)
							mutedText(c, strings.Join([]string{server.Scope, server.Transport, server.Status}, " · "))
						})
						if smallButton(c, "Remove", "Remove MCP server "+server.Name).Disabled(s.cliBusy).Clicked() {
							p.runUpdate(func() (func(), error) {
								result, err := p.host.CLIMCPRemove(p.window.Context(), s.root, server.Name, server.Scope == "project")
								rows, rowsErr := p.host.CLIMCPList(p.window.Context(), s.root)
								return func() { s.cliResult = firstNonEmpty(result.Summary, result.RawOutput); s.mcpServers = &rows }, errorsJoin(err, rowsErr)
							})
						}
					})
				}
			}
		}
		input(c, "feynman-mcp-name", "Server name", &p.serverName, "Name")
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for _, transport := range []string{"http", "stdio"} {
				if choiceButton(c, "research-transport-"+transport, transport, "Use "+transport+" transport", p.serverTransport == transport, true).Clicked() {
					p.serverTransport = transport
				}
			}
		})
		if p.serverTransport == "stdio" {
			input(c, "feynman-mcp-command", "Command", &p.serverCommand, "Command path")
			input(c, "feynman-mcp-args", "Arguments", &p.serverArgs, "Space separated arguments")
			area(c, "feynman-mcp-env", "Environment variables", &p.serverEnv, "KEY=value, one per line", 55)
			input(c, "feynman-mcp-cwd", "Working folder", &p.serverCwd, "Optional working directory")
		} else {
			input(c, "feynman-mcp-url", "HTTP URL", &p.serverURL, "https://…")
			area(c, "feynman-mcp-headers", "Headers", &p.serverHeaders, "Header: value, one per line", 55)
		}
		ui.Row(c).Wrap().Gap(4).Children(func() {
			input(c, "feynman-mcp-exposure", "Exposure", &p.serverExposure, "direct")
			input(c, "feynman-mcp-description", "Description", &p.serverDescription, "Optional description")
		})
		if p.serverTransport == "http" {
			input(c, "feynman-mcp-bearer-env", "Bearer token environment variable", &p.serverBearerEnv, "Optional environment variable name")
		}
		ui.Checkbox(c.Key("feynman-mcp-local"), &p.serverLocal, "Project-local configuration")
		if controlButton(c, "Add MCP server", "Add MCP server to Feynman").Disabled(s.cliBusy || strings.TrimSpace(p.serverName) == "").Clicked() {
			p.addMCPServer()
		}
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

func (p *panel) packageSettings(c *ui.Context) {
	s := p.state
	ui.Column(c.Key("feynman-package-settings")).Gap(4).Padding(7).Radius(9).Background(portrait.Colors.Fill).Children(func() {
		sectionTitle(c, "Workflows, skills and extensions")
		if s.packages != nil {
			if !s.packages.Parsed {
				resultText(c, s.packages.Command, s.packages.RawOutput, s.packages.Error)
			} else {
				for _, item := range s.packages.Packages {
					ui.Row(c.Key("feynman-package-" + item.Name)).Wrap().Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							text(c, portrait.Type.UI, item.Name).FontWeight(600).TextColor(portrait.Colors.Ink)
							mutedText(c, item.Kind+" · "+item.Description)
						})
						action := "install"
						label := "Install"
						if item.Installed {
							action, label = "remove", "Remove"
						}
						if smallButton(c, label, label+" "+item.Name).Disabled(s.cliBusy).Clicked() {
							p.packageAction(action, item.Name)
						}
						if item.Installed && smallButton(c, "Update", "Update "+item.Name).Disabled(s.cliBusy).Clicked() {
							p.packageAction("update", item.Name)
						}
					})
				}
			}
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			input(c, "feynman-package-name", "Package name", &p.packageTarget, "Optional target")
			if smallButton(c, "Update all", "Update all installed Feynman packages").Disabled(s.cliBusy).Clicked() {
				p.packageAction("update", strings.TrimSpace(p.packageTarget))
			}
		})
	})
}

func (p *panel) packageAction(action, target string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLIPackageAction(p.window.Context(), root, action, target)
		rows, rowsErr := p.host.CLIPackagesList(p.window.Context(), root)
		return func() { p.state.cliResult = firstNonEmpty(result.Summary, result.RawOutput); p.state.packages = &rows }, errorsJoin(err, rowsErr)
	})
}

func (p *panel) launchWizard(wizard, target string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.LaunchTerminalWizard(p.window.Context(), root, wizard, target)
		message := result.Command
		if result.Launched {
			message = "Launched " + result.Command + " in Terminal. Status refreshes when this window regains focus."
		}
		return func() { p.state.cliResult = message }, err
	})
}

func (p *panel) clearSearchKey(provider string) {
	root := p.state.root
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLISearchAction(p.window.Context(), root, "clear-key", provider, "")
		status, statusErr := p.host.CLISearchStatus(p.window.Context(), root)
		return func() {
			p.state.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			p.state.searchStatus = &status
		}, errorsJoin(err, statusErr)
	})
}

func splitLines(value string) []string {
	var result []string
	for _, line := range strings.Split(strings.ReplaceAll(value, ",", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

func errorsJoin(first, second error) error {
	if first != nil && second != nil {
		return first
	}
	if first != nil {
		return first
	}
	return second
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
