package nativegenui

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/egoist/mygo/ui"
)

// TestGenUISettingsSaveErrorSurvivesSettingsSearch covers a save from inside
// Settings search: the page draws only the rows the query matched, and the
// error the save saved has to show above them.
func TestGenUISettingsSaveErrorSurvivesSettingsSearch(t *testing.T) {
	const message = "The agent command must not be empty."
	draw := func(query string) *ui.Tester {
		p := &panel{settingsMessage: message}
		page := p.settingsPage()
		app := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(12).Children(func() {
				page.Draw(c, nativeui.SettingsQuery{Text: query})
			})
		}, 640, 420)
		app.Frame()
		return app
	}

	app := draw("agent command")
	if !app.HasText("Agent command") {
		t.Fatalf("the search result is missing its field: %q", app.Texts())
	}
	if !app.HasText("GenUI settings") || !app.HasText(message) {
		t.Fatalf("a save error is hidden while the matched row is on screen: %q", app.Texts())
	}

	app = draw("nothing matches this")
	if app.HasText(message) || app.HasText("GenUI settings") {
		t.Fatalf("a section the search does not match still draws its error: %q", app.Texts())
	}
}
