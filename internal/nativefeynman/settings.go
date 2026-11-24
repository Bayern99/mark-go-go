package nativefeynman

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) loadSettings(root string) {
	if root == "" || p.state.cliBusy || p.host == nil {
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

func (p *panel) settingsPages() []nativeui.SettingsPage {
	visible := func() bool { return p.window != nil && (p.window.Native() == nil || p.window.PluginEnabled("feynman")) }
	return []nativeui.SettingsPage{
		{ID: "models", Section: "Research", Label: "Models & Providers", Icon: portrait.IconCube, Order: 100, Visible: visible,
			Keywords: []string{"models providers authentication service tier default pin show picker sign in log out"},
			Dirty: func() bool {
				return p.window.Preference("feynman.models.pinned") != "" || p.window.Preference("feynman.models.hidden") != ""
			},
			Reset: func() {
				p.window.SetPreference("feynman.models.pinned", "")
				p.window.SetPreference("feynman.models.hidden", "")
			},
			Draw: func(c *ui.Context, q nativeui.SettingsQuery) { p.drawSettingsPage(c, "models", q) }},
		{ID: "search", Section: "Research", Label: "Search & Papers", Icon: portrait.IconMagnifyingGlass, Order: 110, Visible: visible,
			Keywords: []string{"web search provider api key exa brave tavily alphaxiv account papers"},
			Dirty:    func() bool { return p.searchProvider != "" && p.searchProvider != "auto" }, Reset: p.resetSearchSettings,
			Draw: func(c *ui.Context, q nativeui.SettingsQuery) { p.drawSettingsPage(c, "search", q) }},
		{ID: "mcp", Section: "Research", Label: "MCP Servers", Icon: portrait.IconPlugs, Order: 120, Visible: visible,
			Keywords: []string{"mcp servers transport http stdio headers bearer command arguments environment exposure project local add remove"},
			Draw:     func(c *ui.Context, q nativeui.SettingsQuery) { p.drawSettingsPage(c, "mcp", q) }},
		{ID: "packages", Section: "Research", Label: "Packages", Icon: portrait.IconSparkle, Order: 130, Visible: visible,
			Keywords: []string{"packages workflows skills extensions install remove update target"},
			Draw:     func(c *ui.Context, q nativeui.SettingsQuery) { p.drawSettingsPage(c, "packages", q) }},
		{ID: "diagnostics", Section: "Footer", Label: "Diagnostics", Icon: portrait.IconWrench, Order: 900, Visible: visible,
			Keywords: []string{"diagnostics feynman cli doctor setup preview status refresh"},
			Draw:     func(c *ui.Context, q nativeui.SettingsQuery) { p.drawSettingsPage(c, "diagnostics", q) }},
	}
}

func (p *panel) drawSettingsPage(c *ui.Context, tab string, q nativeui.SettingsQuery) {
	p.syncConnection()
	s := p.state
	if s.root == "" {
		portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconFolderOpen, Title: "Open a workspace folder", Description: "Configure Feynman models, MCP servers, and diagnostics after opening a folder."})
		return
	}
	if p.settingsRoot != s.root && !s.cliBusy {
		p.loadSettings(s.root)
	}
	if s.cliError != "" {
		portrait.Alert(c, portrait.Tones.Danger, "Feynman CLI", s.cliError)
	}
	if s.cliResult != "" {
		portrait.OutputBlock(c, "Command result", s.cliResult)
	}
	switch tab {
	case "search":
		if q.Match("Web search", "Provider & API key routing and saved authentication.", "web search provider api key exa brave tavily") {
			p.searchSettings(c)
		}
		if q.Match("alphaXiv", "Paper search and annotation account.", "alphaxiv account papers") {
			p.alphaSettings(c)
		}
	case "mcp":
		if q.Match("MCP Servers", "Configured servers and tool exposure.", "mcp servers transport http stdio headers bearer command arguments environment exposure project local add remove") {
			p.mcpSettings(c)
		}
	case "packages":
		if q.Match("Packages", "Workflows, skills, and extensions.", "packages workflows skills extensions install remove update target") {
			p.packageSettings(c)
		}
	case "diagnostics":
		if q.Match("Diagnostics", "Feynman CLI doctor, setup, and preview status.", "diagnostics feynman cli doctor setup preview status refresh") {
			p.statusSettings(c)
		}
	default:
		if q.Match("Models & Providers", "Authentication, service tier, default model, and picker visibility.", "models providers authentication service tier default pin show picker sign in log out") {
			p.modelSettings(c)
		}
	}
	p.drawConfirmation(c)
}

func (p *panel) resetSearchSettings() {
	s := p.state
	if s.root == "" || s.cliBusy {
		return
	}
	p.runUpdate(func() (func(), error) {
		result, err := p.host.CLISearchAction(p.window.Context(), s.root, "clear", "", "")
		status, statusErr := p.host.CLISearchStatus(p.window.Context(), s.root)
		return func() {
			s.cliResult = firstNonEmpty(result.Summary, result.RawOutput)
			s.searchStatus = &status
			p.searchProvider = status.RequestRoute
		}, errorsJoin(err, statusErr)
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
