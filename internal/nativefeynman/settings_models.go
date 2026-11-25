package nativefeynman

import (
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

type providerModels struct {
	name     string
	signedIn bool
	models   []feynman.CLIModelEntry
}

func (p *panel) providerGroups() []providerModels {
	s := p.state
	var groups []providerModels
	group := func(name string) *providerModels {
		for i := range groups {
			if groups[i].name == name {
				return &groups[i]
			}
		}
		groups = append(groups, providerModels{name: name})
		return &groups[len(groups)-1]
	}
	if s.cliModels != nil {
		for _, model := range s.cliModels.Models {
			g := group(model.Provider)
			g.signedIn = true
			g.models = append(g.models, model)
		}
	}
	for _, model := range s.models {
		g := group(model.Provider)
		if !slices.ContainsFunc(g.models, func(m feynman.CLIModelEntry) bool { return m.ID == model.ID }) {
			g.models = append(g.models, feynman.CLIModelEntry{ID: model.ID, Provider: model.Provider, Name: researchModelLabel(model)})
		}
	}
	return groups
}

func (p *panel) modelSettings(c *ui.Context) {
	s := p.state
	if !p.tierLoaded {
		if tier := firstNonEmpty(tierOf(s.cliModels), serviceTierOf(s.cliStatus)); tier != "" {
			p.tierValue, p.tierLoaded = tier, true
		}
	}
	portrait.SettingsGroup(c, "Defaults", func(rows *portrait.Rows) {
		rows.Row(portrait.RowSpec{Title: "Service tier", Description: "The service tier used for research requests."}, func() {
			ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
				portrait.Select(c.Key("feynman-service-tier"), &p.tierValue, serviceTierOptions(p.tierValue), portrait.SelectSpec{Label: "Service tier", Placeholder: "Choose a tier", Width: portrait.Form.SelectMin})
				if portrait.Button(c, "Apply", "Set the Feynman service tier").Disabled(s.cliBusy || strings.TrimSpace(p.tierValue) == "" || p.tierValue == firstNonEmpty(tierOf(s.cliModels), serviceTierOf(s.cliStatus))).Clicked() {
					p.modelAction("tier", strings.TrimSpace(p.tierValue))
				}
			})
		})
	})
	groups := p.providerGroups()
	loadingModels := s.cliBusy && (s.cliModels == nil || len(s.cliModels.Models) == 0)
	if loadingModels {
		portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconCube, Title: "Loading models…"}).Busy(true)
	} else if s.cliModels != nil && !s.cliModels.Parsed {
		resultText(c, s.cliModels.Command, s.cliModels.RawOutput, s.cliModels.Error)
	} else if len(groups) == 0 {
		portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconCube, Title: "No models yet", Description: "Sign in to a provider in Terminal."})
	}
	for _, group := range groups {
		p.providerGroup(c, group)
	}
	portrait.SettingsGroup(c, "Add provider", func(rows *portrait.Rows) {
		rows.Row(portrait.RowSpec{Title: "Provider id", Description: "Another provider, such as anthropic."}, func() {
			ui.Row(c).Gap(portrait.Form.Gap).Children(func() {
				input(c, "feynman-provider-login", "Model provider", &p.loginProvider, "Provider id").Width(portrait.Form.SelectMin)
				if portrait.Button(c, "Sign in in Terminal", "Sign in to another model provider in Terminal").Disabled(s.cliBusy || strings.TrimSpace(p.loginProvider) == "").Clicked() {
					p.launchWizard("model_login", strings.TrimSpace(p.loginProvider))
				}
			})
		})
	})
}

func serviceTierOptions(current string) []portrait.Option {
	options := []portrait.Option{{Value: "default", Label: "Default"}, {Value: "priority", Label: "Priority"}, {Value: "flex", Label: "Flex"}}
	for _, option := range options {
		if option.Value == current {
			return options
		}
	}
	if strings.TrimSpace(current) != "" {
		options = append(options, portrait.Option{Value: current, Label: current})
	}
	return options
}

func tierOf(r *feynman.CLIModelListResult) string {
	if r == nil {
		return ""
	}
	return r.Tier
}

func serviceTierOf(r *feynman.CLIStatusResult) string {
	if r == nil {
		return ""
	}
	return r.ServiceTier
}

func (p *panel) providerGroup(c *ui.Context, g providerModels) {
	name := providerLabel(g.name)
	group := portrait.SettingsGroup(c, name, func(rows *portrait.Rows) {
		status, tone := "Not signed in", portrait.Tones.Neutral
		if g.signedIn {
			status, tone = "Signed in", portrait.Tones.Success
		}
		rows.Row(portrait.RowSpec{Title: "Account", Description: status}, func() {
			ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
				portrait.Badge(c, status, tone)
				if g.signedIn {
					if portrait.ButtonWith(c, "Log out", "Log out of "+name, portrait.ButtonSpec{Look: portrait.DangerOutline, Size: portrait.Sizes.Regular}).Disabled(p.state.cliBusy).Clicked() {
						p.modelAction("logout", g.name)
					}
				} else if portrait.Button(c, "Sign in in Terminal", "Sign in to "+name+" in Terminal").Disabled(p.state.cliBusy).Clicked() {
					p.launchWizard("model_login", g.name)
				}
			})
		})
		for _, model := range g.models {
			p.cliModelRow(c, rows, model)
		}
	})
	group.Role(ui.RoleGroup).Label(name + " models")
}

func (p *panel) cliModelRow(c *ui.Context, rows *portrait.Rows, model feynman.CLIModelEntry) {
	label := firstNonEmpty(model.Name, model.ID)
	key := model.Provider + "/" + model.ID
	rows.Row(portrait.RowSpec{Title: label, Description: providerLabel(model.Provider)}, func() {
		ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if model.Current {
				portrait.Badge(c, "Default", portrait.Tones.Accent)
			} else if portrait.QuietButton(c, "Set default", "Set "+label+" as the default model").Disabled(p.state.cliBusy).Clicked() {
				p.modelAction("set", firstNonEmpty(model.Name, model.ID))
			}
			pinned := p.modelPinned(key)
			verb := "Pin "
			if pinned {
				verb = "Unpin "
			}
			pin := portrait.ButtonWith(c.Key("pin"), "", verb+label, portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconPushPin, Selected: pinned}).Checked(pinned)
			if pin.Clicked() {
				p.toggleModelPinned(key)
			}
			ui.Row(c).Gap(portrait.Form.HelpGap).AlignItems(ui.Center).Children(func() {
				portrait.Caption(c, "In picker")
				shown := p.modelShown(key)
				if portrait.Switch(c.Key("shown"), &shown).Label("Show " + label + " in Picker").Changed() {
					p.setModelShown(key, shown)
				}
			})
		})
	})
}

func (p *panel) modelAction(action, value string) {
	root := p.state.root
	p.runSettingsUpdate(func() (func(), error) {
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
