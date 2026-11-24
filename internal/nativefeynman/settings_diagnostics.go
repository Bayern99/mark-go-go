package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) statusSettings(c *ui.Context) {
	s := p.state
	settingsSection(c, "Feynman CLI", "Feynman CLI status, doctor and setup output.", func() {
		mutedText(c, "Feynman CLI status, doctor and setup output.")
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
			if smallButton(c, "Refresh", "Refresh Feynman CLI settings").Disabled(s.cliBusy).Clicked() {
				p.settingsRoot = ""
				p.loadSettings(s.root)
			}
		})
		if s.cliStatus == nil {
			mutedText(c, "Feynman CLI status has not loaded.")
		} else if s.cliStatus.Parsed {
			for _, section := range s.cliStatus.Sections {
				portrait.Disclosure(c.Key("cli-status-"+section.Title), section.Title, p.collapsibleState("cli-status-"+section.Title), portrait.DisclosureSpec{}, func() {
					for _, entry := range section.Entries {
						ui.Row(c).Gap(portrait.Form.Gap).Children(func() {
							portrait.Caption(c, entry.Key).Grow(1)
							text(c, portrait.Type.Meta, entry.Value).TextColor(portrait.Colors.Ink).FillWidth()
						})
					}
				})
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
					portrait.Alert(c, portrait.Tones.Warning, "Doctor issue", issue)
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
