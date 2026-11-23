package nativefeynman

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const alphaAuthenticationMessage = "alphaXiv authentication is required. Run alphaxiv auth login, then return here and retry the request."

func fixtureAlphaPanel() *panel {
	p := fixtureReadyPanel()
	p.state.selectedTab = "alpha"
	p.state.alphaOutput = "Authentication is required before paper details can be inspected."
	p.state.cliError = alphaAuthenticationMessage
	p.alphaQuery = "acoustic scene understanding"
	p.alphaPaperID = "2401.00001"
	p.alphaQuestion = "What method does this paper use?"
	p.alphaRepoURL = "https://github.com/example/repository"
	p.alphaFilePath = "src/model.go"
	p.alphaNoteInput = "Reproduce the evaluation with the workspace corpus."
	return p
}

func drawFixtureAlphaDrawer(c *ui.Context, p *panel, width, height float32) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c.Key("fixture-alpha-drawer")).Absolute().Left(96).Top(8).Width(width).Height(height).Padding(8).Gap(6).Clip().Children(func() {
			p.header(c)
			p.tabs(c)
			p.alphaXiv(c)
		})
	})
}

func TestAlphaXivControlsFitNarrowDrawerAndStandardWindow(t *testing.T) {
	naturalWidths := make(map[string]float32)
	standardHeights := make(map[string]float32)
	for _, test := range []struct {
		name                     string
		windowWidth              int
		drawerWidth, drawerRight float32
		keepRows                 bool
	}{
		{name: "1280 by 820 window", windowWidth: 1280, drawerWidth: 640, drawerRight: 736, keepRows: true},
		{name: "480 by 820 drawer", windowWidth: 480, drawerWidth: 376, drawerRight: 472},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := fixtureAlphaPanel()
			app := ui.NewTester(func(c *ui.Context) {
				drawFixtureAlphaDrawer(c, p, test.drawerWidth, 804)
			}, test.windowWidth, 820)
			app.Frame()

			targets := []struct {
				role ui.Role
				name string
			}{
				{ui.RoleTextField, "Search papers"},
				{ui.RoleButton, "Search alphaXiv"},
				{ui.RoleTextField, "Paper ID"},
				{ui.RoleCheckBox, "Full text"},
				{ui.RoleButton, "Get alphaXiv paper details"},
				{ui.RoleTextField, "Ask about paper"},
				{ui.RoleButton, "Ask alphaXiv about this paper"},
				{ui.RoleTextField, "Repository URL"},
				{ui.RoleTextField, "Repository file"},
				{ui.RoleButton, "Inspect repository code for this paper"},
				{ui.RoleTextField, "Paper note"},
				{ui.RoleButton, "alphaXiv annotation list"},
				{ui.RoleButton, "alphaXiv annotation get"},
				{ui.RoleButton, "alphaXiv annotation set"},
				{ui.RoleButton, "alphaXiv annotation clear"},
				{ui.RoleAlert, alphaAuthenticationMessage},
			}
			bounds := make(map[string]ui.Rect, len(targets))
			for _, target := range targets {
				info, ok := app.AccessByRole(target.role, target.name)
				if !ok {
					t.Errorf("missing accessible %v %q", target.role, target.name)
					continue
				}
				bounds[target.name] = info.Bounds
				if info.Offscreen || info.Bounds.X < 104 || info.Bounds.Y < 16 || info.Bounds.X+info.Bounds.W > test.drawerRight-8 || info.Bounds.Y+info.Bounds.H > 804 {
					t.Errorf("%q bounds %+v offscreen=%t, want inside drawer content x=104..%v y=16..804", target.name, info.Bounds, info.Offscreen, test.drawerRight-8)
				}
				if target.role != ui.RoleAlert && !info.Focusable {
					t.Errorf("%q is not keyboard focusable", target.name)
				}
				if test.keepRows {
					naturalWidths[target.name] = info.Bounds.W
					standardHeights[target.name] = info.Bounds.H
				} else if target.role == ui.RoleButton || target.role == ui.RoleCheckBox {
					if width := naturalWidths[target.name]; width > 0 && info.Bounds.W < width-0.01 {
						t.Errorf("%q is clipped from its natural width %v to %v", target.name, width, info.Bounds.W)
					}
				}
			}

			if !test.keepRows {
				for _, name := range []string{"Get alphaXiv paper details", "Inspect repository code for this paper"} {
					inputName := "Paper ID"
					if name == "Inspect repository code for this paper" {
						inputName = "Repository URL"
					}
					if bounds[name].Y < bounds[inputName].Y+bounds[inputName].H {
						t.Errorf("narrow drawer kept %q on the cramped input row: input bounds=%+v, control bounds=%+v", name, bounds[inputName], bounds[name])
					}
				}
				if bounds[alphaAuthenticationMessage].H <= standardHeights[alphaAuthenticationMessage] {
					t.Errorf("narrow authentication message did not wrap to additional lines: bounds=%+v", bounds[alphaAuthenticationMessage])
				}
			}

			if test.keepRows {
				for _, row := range [][]string{
					{"Search papers", "Search alphaXiv"},
					{"Paper ID", "Full text", "Get alphaXiv paper details"},
					{"Ask about paper", "Ask alphaXiv about this paper"},
					{"Repository URL", "Repository file", "Inspect repository code for this paper"},
				} {
					first := bounds[row[0]]
					for _, name := range row[1:] {
						if bounds[name].Y >= first.Y+first.H {
							t.Errorf("standard window split %q onto another row: first bounds=%+v, control bounds=%+v", name, first, bounds[name])
						}
					}
				}
			}
		})
	}
}
