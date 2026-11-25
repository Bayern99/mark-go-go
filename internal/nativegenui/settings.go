package nativegenui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

const (
	backendDescription = "The agent that makes a document's reading view."
	refreshNote        = "Test & Refresh reads the agent's model list. It does not submit a generation request or your document text, though the agent may record the session in its own state."
	backendKeywords    = "backend codex antigravity custom pi"
	refreshKeywords    = "test refresh models"
	loginKeywords      = "sign in login terminal"
	customLoginNote    = "The agent signs in with its own command."
	modelKeywords      = "model"
	effortKeywords     = "reasoning effort thought level"
	commandKeywords    = "agent command arguments"
	argumentsKeywords  = "arguments agent command"
)

// backendEdit is the half-typed state of one backend's fields, so switching
// backends or redrawing does not lose what the user was writing.
func (p *panel) settingsPage() nativeui.SettingsPage {
	return nativeui.SettingsPage{
		ID: "genui", Section: "Workspace", Label: "GenUI", Icon: portrait.IconSparkle, Order: 210,
		Keywords: []string{"genui generation backend agent model reasoning effort codex antigravity custom"},
		Dirty: func() bool {
			id, backend := p.settings.GenUI.Active()
			defaults := workspace.DefaultSettings().GenUI.For(id)
			return backend.Model != defaults.Model || backend.Command != defaults.Command ||
				backend.ThoughtLevel != defaults.ThoughtLevel || !slices.Equal(backend.Args, defaults.Args)
		},
		Reset: func() {
			id, _ := p.settings.GenUI.Active()
			defaults := workspace.DefaultSettings().GenUI.For(id)
			p.saveBackend(id, workspace.GenUIBackendPatch{
				Model:        &defaults.Model,
				Command:      &defaults.Command,
				Args:         &defaults.Args,
				ThoughtLevel: &defaults.ThoughtLevel,
			})
			if edit := p.edits[id]; edit != nil {
				edit.commandEdited, edit.argumentsEdited = false, false
			}
		},
		Draw: p.drawSettingsPage,
	}
}

func (p *panel) drawSettingsPage(c *ui.Context, q nativeui.SettingsQuery) {
	id, backend := p.settings.GenUI.Active()
	if q.Active() && !q.Matches("Agent backend", backendDescription, backendKeywords) &&
		!q.Matches("Test & Refresh", refreshNote, refreshKeywords) &&
		!q.Matches("Sign in in Terminal", customLoginNote, loginKeywords) &&
		!q.Matches("Model", "The model the backend uses.", modelKeywords) &&
		!q.Matches("Reasoning effort", "How long the model may think before answering.", effortKeywords) &&
		!q.Matches("Agent command", "The command the backend runs.", commandKeywords) &&
		!q.Matches("Arguments", "The arguments that make the command speak ACP.", argumentsKeywords) {
		return
	}
	if p.settingsMessage != "" {
		portrait.Alert(c, portrait.Tones.Danger, "GenUI settings", p.settingsMessage)
	}
	p.drawBackendCards(c, q, id)
	p.drawConnection(c, q, id)
	p.drawGeneration(c, q, id, backend)
	p.drawAdvanced(c, q, id, backend)
}

func (p *panel) drawBackendCards(c *ui.Context, q nativeui.SettingsQuery, active string) {
	if q.Active() && !q.Match("Agent backend", backendDescription, backendKeywords) {
		return
	}
	portrait.SettingsGroup(c, "Backend", func(rows *portrait.Rows) {
		rows.Row(portrait.RowSpec{Title: "Agent backend", Description: backendDescription, Stacked: true}, func() {
			grid := ui.Column(c).FillWidth().Gap(portrait.Form.PadX)
			grid.Children(func() {
				for start := 0; start < len(genui.Backends); start += 2 {
					ui.Row(c).FillWidth().Gap(portrait.Form.PadX).AlignItems(ui.Stretch).Children(func() {
						for column := 0; column < 2 && start+column < len(genui.Backends); column++ {
							backend := genui.Backends[start+column]
							p.backendCard(c, backend, backend.ID == active)
						}
					})
				}
			})
		})
	})
}

func (p *panel) backendCard(c *ui.Context, backend genui.Backend, selected bool) {
	card := ui.ButtonBase(c.Key("backend-"+backend.ID)).Label(backend.Name+" backend").
		Grow(1).MinWidth(0).AlignItems(ui.Start).Padding(portrait.Form.PadX).Radius(portrait.Radii.L).
		Border(portrait.Editor.Hairline, portrait.Colors.Line).Background(portrait.Colors.Fill)
	if selected {
		card.Border(portrait.Editor.Hairline, portrait.Colors.Accent).Background(portrait.Colors.AccentSoft)
	}
	card.Disabled(!backend.Ready).DimDisabled(true)
	card.Children(func() {
		ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			portrait.Label(c, backend.Name).MaxLines(1).Ellipsis("…")
			if !backend.Ready {
				portrait.Badge(c, "Coming soon", portrait.Tones.Neutral)
			}
			spacer := ui.Row(c).Grow(1).MinWidth(0)
			spacer.Children(func() {
				if backend.Ready {
					p.drawBackendStatus(c, backend.ID)
				}
			})
		})
		portrait.Caption(c, backendPrompt(backend))
	})
	if card.Clicked() && backend.Ready && !selected {
		p.selectBackend(backend.ID)
	}
}

// backendPrompt tells the reader what a card needs before it can generate.
func backendPrompt(backend genui.Backend) string {
	switch {
	case !backend.Ready:
		return backend.InstallHint
	case backend.ID == genui.BackendPi:
		return "pi --mode rpc, with reads and writes kept in a temporary folder."
	case backend.ID == genui.BackendCustom:
		return "An agent of your own that speaks ACP over stdio."
	case backend.Command == "":
		return backend.InstallHint
	default:
		return backend.Command + " " + strings.Join(backend.Args, " ")
	}
}

func statusLook(status genui.ProbeResult) (string, portrait.Tone) {
	switch status.Status {
	case genui.AgentReady:
		return "Ready", portrait.Tones.Success
	case genui.AgentLoginRequired:
		return "Sign in", portrait.Tones.Warning
	case genui.AgentUnknown:
		return "Login unknown", portrait.Tones.Neutral
	case genui.AgentOutdated:
		return "Update needed", portrait.Tones.Danger
	case genui.AgentMissing:
		return "Not installed", portrait.Tones.Danger
	case genui.AgentFailed:
		return "Not working", portrait.Tones.Danger
	default:
		return "Not tested", portrait.Tones.Neutral
	}
}

func (p *panel) drawBackendStatus(c *ui.Context, id string) {
	state := p.status(id)
	if state.busy {
		portrait.Type.Meta.Apply(ui.Text(c, "Testing…")).TextColor(portrait.Colors.Ink3)
		return
	}
	label, tone := statusLook(state.probe)
	ui.Row(c).Gap(portrait.Form.HelpGap).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(portrait.Shell.SpaceDotCircle, portrait.Shell.SpaceDotCircle).
			Radius(portrait.Radii.Pill).Background(tone.Solid)
		portrait.Type.Meta.Apply(ui.Text(c, label)).TextColor(portrait.Colors.Ink3).MaxLines(1)
	})
}

func (p *panel) drawGeneration(c *ui.Context, q nativeui.SettingsQuery, id string, backend workspace.GenUIBackend) {
	if q.Active() && !q.Match("Model", "The model the backend uses.", modelKeywords) &&
		!q.Match("Reasoning effort", "How long the model may think before answering.", effortKeywords) {
		return
	}
	state := p.status(id)
	saved := genui.ModelOption{ID: backend.Model, Provider: backend.Provider}
	levels := effortLevels(state.catalog, saved.Key())
	portrait.SettingsGroup(c, "Generation", func(rows *portrait.Rows) {
		if q.Match("Model", "The model the backend uses.", modelKeywords) {
			rows.Row(portrait.RowSpec{Title: "Model", Description: modelDescription(state.catalog, backend)}, func() {
				models := offeredModels(backend, state.catalog)
				options := make([]portrait.Option, 0, len(models))
				for _, model := range models {
					label := model.Name
					if strings.TrimSpace(label) == "" {
						label = model.ID
					}
					if model.Provider != "" {
						label += " · " + model.Provider
					}
					options = append(options, portrait.Option{Value: model.Key(), Label: label})
				}
				if len(options) == 0 {
					portrait.Caption(c, "Test & Refresh to load the models the backend offers.")
					return
				}
				selected := saved.Key()
				if saved.ID == "" {
					selected = state.catalog.CurrentModelKey()
				}
				before := selected
				portrait.Select(c, &selected, options, portrait.SelectSpec{Label: "Model"}).Disabled(state.busy)
				if selected != before {
					for _, option := range models {
						if option.Key() == selected {
							empty := ""
							p.selectBackendOption(id, workspace.GenUIBackendPatch{Model: &option.ID, Provider: &option.Provider, ThoughtLevel: &empty})
							break
						}
					}
				}
			})
		}
		if len(levels) > 0 && q.Match("Reasoning effort", "How long the model may think before answering.", effortKeywords) {
			rows.Row(portrait.RowSpec{Title: "Reasoning effort", Description: "How long the model may think before answering."}, func() {
				options := make([]portrait.Option, 0, len(levels))
				for _, level := range levels {
					label := level.Name
					if label == "" {
						label = level.Value
					}
					options = append(options, portrait.Option{Value: level.Value, Label: label})
				}
				selected := backend.ThoughtLevel
				if selected == "" {
					selected = state.catalog.ThoughtLevel
				}
				before := selected
				portrait.Select(c, &selected, options, portrait.SelectSpec{Label: "Reasoning effort"}).Disabled(state.busy)
				if selected != before {
					p.selectBackendOption(id, workspace.GenUIBackendPatch{ThoughtLevel: &selected})
				}
			})
		}
	})
}

// offeredModels is the agent's latest word on its models: what the last refresh
// declared, and the saved list until a refresh declares any.
func offeredModels(backend workspace.GenUIBackend, catalog genui.ModelCatalog) []genui.ModelOption {
	if catalog.Models != nil {
		return catalog.Models
	}
	if backend.ModelOptions != nil {
		return backend.ModelOptions
	}
	options := make([]genui.ModelOption, 0, len(backend.Models))
	for _, id := range backend.Models {
		option := genui.ModelOption{ID: id}
		if id == backend.Model {
			option.Provider = backend.Provider
		}
		options = append(options, option)
	}
	return options
}

func modelDescription(catalog genui.ModelCatalog, backend workspace.GenUIBackend) string {
	models := offeredModels(backend, catalog)
	if len(models) == 0 {
		return "Test & Refresh to load the models the backend offers."
	}
	saved := genui.ModelOption{ID: backend.Model, Provider: backend.Provider}
	if saved.ID != "" && !slices.ContainsFunc(models, func(model genui.ModelOption) bool { return model.Key() == saved.Key() }) {
		return fmt.Sprintf("The backend no longer offers %q; it will use its own current model.", saved.ID)
	}
	return "The model the backend uses."
}

// effortLevels are the levels the current model offers. An ACP agent declares
// the option only for models that have one, so a model without levels shows no
// choice at all.
func effortLevels(catalog genui.ModelCatalog, model string) []genui.Choice {
	if len(catalog.ThoughtLevels) == 0 {
		return nil
	}
	if model != "" && catalog.CurrentModel != "" && model != catalog.CurrentModelKey() {
		return nil
	}
	return catalog.ThoughtLevels
}

func (p *panel) drawConnection(c *ui.Context, q nativeui.SettingsQuery, id string) {
	backend, _ := genui.BackendByID(id)
	state := p.status(id)
	login := loginHint(backend)
	if q.Active() && !q.Match("Test & Refresh", refreshNote, refreshKeywords) &&
		!q.Match("Sign in in Terminal", login, loginKeywords) {
		return
	}
	portrait.SettingsGroup(c, "Connection", func(rows *portrait.Rows) {
		if q.Match("Test & Refresh", refreshNote, refreshKeywords) {
			rows.Row(portrait.RowSpec{Title: "Test & Refresh", Description: connectionDescription(state)}, func() {
				button := portrait.AccentButton(c, "Test & Refresh", "Test & Refresh")
				button.Disabled(state.busy)
				if button.Clicked() {
					p.testConnection(id, true)
				}
			})
		}
		if q.Match("Sign in in Terminal", login, loginKeywords) {
			rows.Row(portrait.RowSpec{Title: "Sign in in Terminal", Description: login}, func() {
				button := portrait.Button(c, "Sign in in Terminal", "Sign in in Terminal")
				button.Disabled(backend.LoginCommand == "")
				if button.Clicked() {
					p.signInInTerminal(id)
				}
			})
		}
	})
}

func loginHint(backend genui.Backend) string {
	if backend.LoginCommand == "" {
		return customLoginNote
	}
	return backend.LoginHint
}

// connectionDescription is what the last test learned, or what the test does.
func connectionDescription(state *backendStatus) string {
	if state.busy {
		return "Testing the command and asking the agent for its models…"
	}
	if state.probe.Detail != "" {
		return state.probe.Detail
	}
	if state.probe.Hint != "" {
		return state.probe.Hint
	}
	return refreshNote
}

func (p *panel) drawAdvanced(c *ui.Context, q nativeui.SettingsQuery, id string, backend workspace.GenUIBackend) {
	showCommand := q.Match("Agent command", "The command the backend runs.", commandKeywords)
	showArgs := id != genui.BackendPi && q.Match("Arguments", "The arguments that make the command speak ACP.", argumentsKeywords)
	if !showCommand && !showArgs {
		return
	}
	edit := p.editFor(id)
	if q.Active() {
		// A matched row has to be on screen, so search opens the disclosure.
		edit.advancedOpen = true
	}
	portrait.SettingsGroup(c, "Advanced", func(rows *portrait.Rows) {
		portrait.Disclosure(c.Key("genui-advanced-"+id), "Advanced", &edit.advancedOpen, portrait.DisclosureSpec{}, func() {
			ui.Column(c).FillWidth().Gap(portrait.Form.Gap).Children(func() {
				if showCommand {
					ui.Column(c).FillWidth().Gap(portrait.Form.HelpGap).Children(func() {
						portrait.Label(c, "Agent command")
						ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
							field := portrait.TextField(c, &edit.command, portrait.FieldSpec{Label: "Agent command", Placeholder: "Agent command", Size: portrait.Form.FieldCompact})
							field.Frame().Grow(1).MinWidth(0)
							field.Input().OnChange(func() { edit.commandEdited = true })
							p.drawSaveCommand(c, id, edit)
						})
					})
				}
				if showArgs {
					ui.Column(c).FillWidth().Gap(portrait.Form.HelpGap).Children(func() {
						portrait.Label(c, "Arguments")
						ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
							field := portrait.TextField(c, &edit.arguments, portrait.FieldSpec{Label: "Arguments", Placeholder: `["acp"]`, Size: portrait.Form.FieldCompact})
							field.Frame().Grow(1).MinWidth(0)
							field.Input().OnChange(func() { edit.argumentsEdited = true })
							save := portrait.AccentButton(c, "Save arguments", "Save arguments")
							save.Disabled(!edit.argumentsEdited)
							if save.Clicked() {
								args, err := parseArguments(edit.arguments)
								if err != nil {
									p.settingsMessage = err.Error()
								} else {
									p.saveBackend(id, workspace.GenUIBackendPatch{Args: &args})
									edit.argumentsEdited = false
								}
							}
						})
					})
				}
				if backend.Command == "" && id == genui.BackendCustom {
					portrait.Caption(c, "Set the command to the ACP agent you run.")
				}
			})
		})
	})
}

func (p *panel) drawSaveCommand(c *ui.Context, id string, edit *backendEdit) {
	save := portrait.AccentButton(c, "Save command", "Save command")
	save.Disabled(!edit.commandEdited || strings.TrimSpace(edit.command) == "")
	if save.Clicked() {
		p.saveBackend(id, workspace.GenUIBackendPatch{Command: &edit.command})
		edit.commandEdited = false
	}
}
