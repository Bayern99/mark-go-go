package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) alphaSettings(c *ui.Context) {
	s := p.state
	settingsSection(c, "alphaXiv", "Account", "Paper search and annotation account.", func() {
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
		ui.Row(c).Wrap().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if controlButton(c, "Sign in to alphaXiv in Terminal", "Sign in to alphaXiv using Terminal").Disabled(s.cliBusy).Clicked() {
				p.launchWizard("alpha_login", "")
			}
			if s.alphaStatus != nil && s.alphaStatus.LoggedIn && portrait.ButtonWith(c, "Log out", "Log out from alphaXiv", portrait.ButtonSpec{Look: portrait.DangerOutline, Size: portrait.Sizes.Regular}).Disabled(s.cliBusy).Clicked() {
				p.runUpdate(func() (func(), error) {
					result, err := p.host.CLIAlphaLogout(p.window.Context(), s.root)
					return func() { s.cliResult = firstNonEmpty(result.Summary, result.RawOutput) }, err
				})
				p.loadSettings(s.root)
			}
			if portrait.ButtonWith(c, "Open alphaXiv research", "Open alphaXiv search and paper tools", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).Clicked() {
				s.selectedTab = "alpha"
			}
		})
	})
}

func (p *panel) searchSettings(c *ui.Context) {
	s := p.state
	settingsSection(c, "Web search", "Provider & API key", "Provider routing and saved authentication.", func() {
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
					trigger := portrait.ButtonWith(c, "Clear "+provider+" API key", "Remove the saved "+provider+" API key", portrait.ButtonSpec{Look: portrait.DangerOutline, Size: portrait.Sizes.Regular}).Disabled(s.cliBusy)
					p.confirmOnClick("clear-search-key:"+provider, trigger, portrait.ConfirmSpec{
						Title:        "Clear API key?",
						Body:         "Remove the saved " + provider + " API key? This action cannot be undone.",
						ConfirmLabel: "Clear API key",
						OnConfirm:    func() { p.clearSearchKey(provider) },
					})
				}
			} else {
				resultText(c, s.searchStatus.Command, s.searchStatus.RawOutput, s.searchStatus.Error)
			}
		}
		portrait.FormRow(c, "Provider", "", func() {
			input(c, "feynman-search-provider", "Search provider", &p.searchProvider, "auto, exa, brave, tavily…")
		})
		portrait.FormRow(c, "API key", "", func() {
			input(c, "feynman-search-key", "Optional API key", &p.searchKey, "Leave blank to use existing authentication", func(entry ui.Element) {
				entry.Password()
			})
		})
		ui.Row(c).FillWidth().Justify(ui.End).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if portrait.ButtonWith(c, "Reset to Auto", "Reset search routing to automatic", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular}).Disabled(s.cliBusy).Clicked() {
				p.resetSearchSettings()
			}
			if portrait.ButtonWith(c, "Save", "Set the selected web search provider and API key", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Regular}).Disabled(s.cliBusy || strings.TrimSpace(p.searchProvider) == "").Clicked() {
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
	})
}
