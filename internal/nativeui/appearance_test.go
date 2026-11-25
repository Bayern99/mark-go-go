package nativeui_test

import (
	"slices"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func pageFace(t *testing.T, app *ui.Tester) [3]int {
	t.Helper()
	page := mustAccess(t, app, ui.RoleMain, "Document").Bounds
	return sceneryAt(app, int(page.X+page.W/2), int(page.Y+page.H/2))
}

func rgb(c ui.Color) [3]int { return [3]int{int(c.R), int(c.G), int(c.B)} }

func TestAppearanceFollowsTheSystemUnlessChosen(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	model := libraryModel()
	model.Spaces = map[string]string{"/library": "Earth"}
	earth, _ := portrait.SpaceNamed("Earth")
	for _, tc := range []struct {
		name       string
		appearance portrait.Appearance
		systemDark bool
		want       portrait.Scheme
	}{
		{"system light", portrait.System, false, earth.Light},
		{"system dark", portrait.System, true, earth.Dark},
		{"forced dark over a light system", portrait.Dark, false, earth.Dark},
		{"forced light over a dark system", portrait.Light, true, earth.Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var shell nativeui.Shell
			model.Appearance = tc.appearance
			app := ui.NewTesterWith(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820, ui.TesterOptions{Dark: tc.systemDark})
			app.Frame()
			if got := pageFace(t, app); got != rgb(tc.want.Paper) {
				t.Fatalf("page is %v, want %v", got, rgb(tc.want.Paper))
			}
			if portrait.Colors.Accent != tc.want.Accent {
				t.Fatalf("accent %+v, want Earth's %+v", portrait.Colors.Accent, tc.want.Accent)
			}
		})
	}
}

func TestEverySpaceTakesItsOwnAccent(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	seen := map[[2]ui.Color]string{}
	for _, space := range portrait.Spaces {
		var shell nativeui.Shell
		model := libraryModel()
		model.Spaces = map[string]string{"/library": space.Name}
		model.Appearance = portrait.Dark
		app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, nativeui.Actions{}, nativeui.Slots{}) }, 1280, 820)
		app.Frame()
		if portrait.Colors.Accent != space.Dark.Accent || pageFace(t, app) != rgb(space.Dark.Paper) {
			t.Errorf("%s dark drew accent %+v on %v", space.Name, portrait.Colors.Accent, pageFace(t, app))
		}
		seen[[2]ui.Color{space.Light.Accent, space.Dark.Accent}] = space.Name
	}
	if len(seen) < len(portrait.Spaces)-1 {
		t.Errorf("only %d distinct accent pairs across %d Spaces", len(seen), len(portrait.Spaces))
	}
}

func TestAppearanceSettingsAndCommandBarStoreTheChoice(t *testing.T) {
	t.Cleanup(func() { portrait.Use(portrait.DefaultSpace, false) })
	var shell nativeui.Shell
	model := libraryModel()
	var chosen []portrait.Appearance
	actions := nativeui.Actions{SetAppearance: func(a portrait.Appearance) { chosen = append(chosen, a); model.Appearance = a }}
	app := ui.NewTester(func(c *ui.Context) { shell.Draw(c, model, actions, nativeui.Slots{}) }, 1280, 820)
	app.Frame()
	shell.OpenSettings("appearance")
	app.Frame()
	if err := app.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if err := app.Click("Close settings"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("toggle dark")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyK)
	app.Type("appearance: system")
	app.Key(0, ui.KeyEnter)
	app.Frame()
	if want := []portrait.Appearance{portrait.Dark, portrait.Light, portrait.System}; !slices.Equal(chosen, want) {
		t.Fatalf("appearance choices %v, want %v", chosen, want)
	}
}
