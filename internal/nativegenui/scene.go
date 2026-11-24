package nativegenui

import (
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// SceneSettingsPage is the GenUI settings page the reader installs, drawn from
// the given settings by the same code, for the acceptance scenes.
func SceneSettingsPage(settings workspace.Settings) nativeui.SettingsPage {
	p := &panel{settings: settings, settingsCommand: settings.AgentCommand}
	return p.settingsPage()
}

// SceneRequest draws the reader's generation-request panel, for the scenes
// that show the request form on its own.
func SceneRequest(c *ui.Context, request string) {
	v := &documentView{request: request}
	p := &panel{}
	p.drawSettings(c, v, workspace.DocumentInfo{Path: "Field Notes.md"}, false)
}

// SceneReading builds a reading view from published statement JSON, so a scene
// draws the reader's own controls.
func SceneReading(data string) (*Reading, error) {
	surface, err := Open(data)
	if err != nil {
		return nil, err
	}
	r := &Reading{}
	r.Load(surface)
	return r, nil
}
