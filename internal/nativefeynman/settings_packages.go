package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) packageSettings(c *ui.Context) {
	s := p.state
	settingsSection(c, "Packages", "Workflows, skills and extensions", "Install and update Feynman packages.", func() {
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
						look := portrait.Raised
						if item.Installed {
							action, label, look = "remove", "Remove", portrait.DangerOutline
						}
						if portrait.ButtonWith(c, label, label+" "+item.Name, portrait.ButtonSpec{Look: look, Size: portrait.Sizes.Compact}).Disabled(s.cliBusy).Clicked() {
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
