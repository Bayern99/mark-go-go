package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/egoist/mygo/ui"
)

type shellScene struct {
	name          string
	width, height int
	feynman       bool
	empty         bool
	mode          string
	act           func(*ui.Tester)
}

func sceneActions(model *nativeui.Model) nativeui.Actions {
	return nativeui.Actions{
		Activate: func(id string) { model.Workspace.ActiveID = id },
		SetMode: func(mode string) {
			for i := range model.Workspace.Documents {
				if model.Workspace.Documents[i].ID == model.Workspace.ActiveID {
					model.Workspace.Documents[i].ViewMode = mode
				}
			}
		},
	}
}

func sceneList() []shellScene {
	return []shellScene{
		{name: "main", width: 1280, height: 820, mode: "preview"},
		{name: "agent", width: 1440, height: 900, feynman: true, mode: "preview"},
		{name: "markdown", width: 1280, height: 820, mode: "markdown"},
		{name: "outline", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { _ = app.Click("Outline") }},
		{name: "sidebar_hidden", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.Key0) }},
		{name: "command_bar", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) { app.Key(ui.Cmd, ui.KeyK) }},
		{name: "command_search", width: 1280, height: 820, mode: "preview", act: func(app *ui.Tester) {
			app.Key(ui.Cmd, ui.KeyK)
			app.Type("sh")
			app.Key(0, ui.KeyDown)
		}},
		{name: "narrow", width: 560, height: 720, mode: "preview"},
		{name: "narrow_drawer", width: 560, height: 720, mode: "preview", act: func(app *ui.Tester) { _ = app.Click("Show navigation") }},
		{name: "welcome", width: 1280, height: 820, empty: true, mode: "preview"},
	}
}
