package nativegenui

import (
	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// SceneSettingsPage is the GenUI settings page the reader installs, drawn from
// the given settings by the same code, for the acceptance scenes.
func SceneSettingsPage(settings workspace.Settings) nativeui.SettingsPage {
	p := &panel{settings: settings}
	return p.settingsPage()
}

// SceneRequest draws the reader's generation-request panel inside the same
// padded layout column that panel.draw uses.
func SceneRequest(c *ui.Context, request string) {
	v := &documentView{request: request, settings: true}
	p := &panel{}
	ui.Column(c).Fill().Gap(portrait.Form.Gap).Padding(portrait.Form.AreaPadY, portrait.Form.DialogGap, portrait.Form.DialogGap, portrait.Form.DialogGap).Children(func() {
		p.drawRequestToggle(c, v)
		p.drawSettings(c, v, workspace.DocumentInfo{Path: "Field Notes.md"}, false)
	})
}

// SceneFailure draws the reader's real production failure state (request form,
// failure alert with inline Retry action, details block, and suppressed empty
// state) for acceptance scenes.
func SceneFailure(c *ui.Context, message, hint, detail string) {
	v := &documentView{
		loaded:   true,
		settings: true,
		failure:  &genui.GenerateFailure{Message: message, Hint: hint, Detail: detail},
	}
	p := &panel{}
	d := workspace.DocumentInfo{Path: "ch05-s05-jade-hybrid-ceramics.md"}
	ui.Column(c).Fill().Gap(portrait.Form.Gap).Padding(portrait.Form.AreaPadY, portrait.Form.DialogGap, portrait.Form.DialogGap, portrait.Form.DialogGap).Children(func() {
		p.drawRequestToggle(c, v)
		p.drawSettings(c, v, d, false)
		p.drawGenerationStatus(c, v, d, false)
		p.drawView(c, v, d, "", false)
	})
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
