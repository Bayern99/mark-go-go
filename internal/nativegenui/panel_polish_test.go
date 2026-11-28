package nativegenui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

const polishSize = 640

const polishWidth = float32(polishSize)

func polishPanel() (*panel, *documentView, workspace.DocumentInfo) {
	v := &documentView{loaded: true}
	p := &panel{win: &nativeui.Window{}, views: map[string]*documentView{"doc": v}, settings: workspace.DefaultSettings()}
	return p, v, workspace.DocumentInfo{ID: "doc", Path: "doc.md"}
}

func drawPolishedPanel(c *ui.Context, p *panel, v *documentView, d workspace.DocumentInfo, busy bool) {
	p.drawReader(c, v, d, "", busy)
}

func polishTester(draw func(*ui.Context), width, height int) *ui.Tester {
	return ui.NewTesterWith(draw, width, height, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
}

func polishStatements(t *testing.T, p *panel, v *documentView, statements ...string) {
	t.Helper()
	p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: statementTestFile(t, statements)})
	v.index("# Overview\n\nAn overview.\n")
}

func TestCommandDockLeavesTheReadingCanvasAtTheTop(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()

	title, ok := app.Find("First card")
	if !ok {
		t.Fatalf("the reading view is missing: %v", app.Texts())
	}
	if want := float32(readingPadding) + portrait.GenUI.CardPadding; title.Y != want {
		t.Fatalf("the first title starts at y %v, want the reading canvas at y %v with no request bar", title.Y, want)
	}
	composer, ok := app.Access("Generation request")
	if !ok || composer.Bounds.Y <= title.Y || composer.Bounds.Y+composer.Bounds.H > 480-portrait.Shell.PageInset {
		t.Fatalf("floating composer bounds = %+v, want a dock at the bottom inside the pane", composer.Bounds)
	}
	if app.HasText("From") || app.HasText("Antigravity · gemini-4-hydrogen") {
		t.Errorf("persistent request metadata broke the reading flow: %v", app.Texts())
	}
}

func TestCommandDockAndOptionsStayInsideANarrowPane(t *testing.T) {
	const paneX, paneWidth, paneHeight = 600, 240, 480
	p, v, d := polishPanel()
	app := polishTester(func(c *ui.Context) {
		ui.Column(c).Absolute().Left(paneX).Top(0).Width(paneWidth).Height(paneHeight).Children(func() {
			drawPolishedPanel(c, p, v, d, false)
		})
	}, 900, paneHeight)
	app.Frame()

	for _, name := range []string{"Generation request", "Generate", "GenUI options"} {
		control, ok := app.Access(name)
		if !ok || control.Bounds.X < paneX || control.Bounds.X+control.Bounds.W > paneX+paneWidth {
			t.Errorf("%s bounds %+v escape the GenUI pane [%d, %d]", name, control.Bounds, paneX, paneX+paneWidth)
		}
	}
	if err := app.Click("GenUI options"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Antigravity · gemini-4-hydrogen") {
		t.Fatalf("options omit the backend summary: %v", app.Texts())
	}
	assertPopoversInside(t, app, ui.Rect{X: paneX, Y: 0, W: paneWidth, H: paneHeight})
}

func assertPopoversInside(t *testing.T, app *ui.Tester, pane ui.Rect) {
	t.Helper()
	found := false
	for _, info := range app.AccessAll() {
		if info.Role != ui.RolePopup {
			continue
		}
		found = true
		if info.Bounds.X < pane.X || info.Bounds.Y < pane.Y || info.Bounds.X+info.Bounds.W > pane.X+pane.W || info.Bounds.Y+info.Bounds.H > pane.Y+pane.H {
			t.Errorf("popover bounds %+v escape the GenUI pane %+v", info.Bounds, pane)
		}
	}
	if !found {
		t.Fatal("the open popover is missing from the accessibility tree")
	}
}

func TestCommandOptionsMenuIsCompactRightAlignedAndTextAligned(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	ref := v.reading.surface.Sources["overview"]
	ref.Hash = "stale"
	v.reading.surface.Sources["overview"] = ref
	v.snapshot.CanUndo = true
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	if err := app.Click("GenUI options"); err != nil {
		t.Fatal(err)
	}
	app.Frame()

	trigger, _ := app.Access("GenUI options")
	popup, ok := app.AccessByRole(ui.RolePopup, "")
	if !ok {
		t.Fatal("the options popover is missing")
	}
	if popup.Bounds.W != portrait.GenUI.CommandMenuWidth {
		t.Errorf("options width = %v, want compact %v", popup.Bounds.W, portrait.GenUI.CommandMenuWidth)
	}
	if right, want := popup.Bounds.X+popup.Bounds.W, trigger.Bounds.X+trigger.Bounds.W+portrait.Form.PadX; right < want-0.5 || right > want+0.5 {
		t.Errorf("options right edge = %v, want pill right edge %v", right, want)
	}
	if popup.Bounds.Y+popup.Bounds.H > trigger.Bounds.Y {
		t.Errorf("options popup %+v is not above its trigger %+v", popup.Bounds, trigger.Bounds)
	}

	summary, ok := app.Find(p.backendSummary())
	if !ok {
		t.Fatal("the backend summary text is missing")
	}
	configureText, ok := app.Find("Configure…")
	if !ok {
		t.Fatal("Configure text is missing")
	}
	configure, _ := app.Access("Configure GenUI")
	restore, _ := app.Access("Restore previous view")
	regenerate, _ := app.Access("Regenerate changed (1)")
	inset := portrait.Form.PadX - 2
	if wantTextX := configure.Bounds.X + inset + portrait.IconSizes.M + portrait.Form.Gap; configureText.X != wantTextX {
		t.Errorf("Configure text x = %v, want icon+gap aligned %v", configureText.X, wantTextX)
	}
	for name, button := range map[string]ui.AccessInfo{"Regenerate": regenerate, "Restore": restore, "Configure": configure} {
		if contentX := button.Bounds.X + inset; contentX != summary.X {
			t.Errorf("%s icon/content starts at x %v, want shared header inset %v", name, contentX, summary.X)
		}
	}
	if configure.Bounds.X != restore.Bounds.X || configure.Bounds.W != restore.Bounds.W ||
		configure.Bounds.X != regenerate.Bounds.X || configure.Bounds.W != regenerate.Bounds.W {
		t.Errorf("menu actions do not share full-width row geometry: regenerate=%+v restore=%+v configure=%+v", regenerate.Bounds, restore.Bounds, configure.Bounds)
	}
}

func TestComposerGrowsFromOneToThreeLinesWithoutClipping(t *testing.T) {
	p, v, d := polishPanel()
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	app.Frame()

	request, ok := app.Access("Generation request")
	if !ok || !request.Multiline {
		t.Fatalf("composer is not a multiline text area: %+v", request)
	}
	generate, _ := app.Access("Generate")
	options, _ := app.Access("GenUI options")
	if request.Bounds.H != portrait.Type.UI.Leading {
		t.Fatalf("empty composer height = %v, want one %vpt line", request.Bounds.H, portrait.Type.UI.Leading)
	}
	center, generateCenter := request.Bounds.Y+request.Bounds.H/2, generate.Bounds.Y+generate.Bounds.H/2
	if center != generateCenter {
		t.Errorf("one-line composer center = %v, Generate center = %v", center, generateCenter)
	}
	bottom := generate.Bounds.Y + generate.Bounds.H

	for lines, text := range map[int]string{
		2: "First line\nSecond line",
		3: "First line\nSecond line\nThird line\nFourth line scrolls",
	} {
		v.request = text
		app.Frame()
		app.Frame()
		request, _ = app.Access("Generation request")
		generate, _ = app.Access("Generate")
		options, _ = app.Access("GenUI options")
		wantHeight := float32(lines) * portrait.Type.UI.Leading
		if request.Bounds.H != wantHeight {
			t.Errorf("composer with %d visible lines has height %v, want %v", lines, request.Bounds.H, wantHeight)
		}
		if request.Bounds.Y+request.Bounds.H != generate.Bounds.Y+generate.Bounds.H ||
			generate.Bounds.Y+generate.Bounds.H != options.Bounds.Y+options.Bounds.H {
			t.Errorf("%d-line composer and actions are not bottom aligned: request=%+v generate=%+v options=%+v", lines, request.Bounds, generate.Bounds, options.Bounds)
		}
		if generate.Bounds.Y+generate.Bounds.H != bottom {
			t.Errorf("%d-line composer moved the pill's bottom edge from %v to %v", lines, bottom, generate.Bounds.Y+generate.Bounds.H)
		}
	}

	v.request = strings.Repeat("wrapped request words ", 40)
	app.Frame()
	app.Frame()
	request, _ = app.Access("Generation request")
	if request.Bounds.H <= portrait.Type.UI.Leading || request.Bounds.H > 3*portrait.Type.UI.Leading {
		t.Errorf("wrapped composer height = %v, want measured growth capped at three lines", request.Bounds.H)
	}
}

func TestFirstGenerationFailureKeepsOneCanvasRecoveryAndNoPillRetry(t *testing.T) {
	p, v, d := polishPanel()
	v.failure = &genui.GenerateFailure{Message: "The agent exited", Hint: "Check the agent command.", Detail: "stderr: unknown flag"}
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()

	status, ok := app.AccessByRole(ui.RoleStatus, "Generation failed · The agent exited")
	if !ok {
		t.Fatalf("the failure has no calm status in the pill: %v", app.Texts())
	}
	retry, ok := app.AccessByRole(ui.RoleButton, "Retry")
	if !ok || !app.HasText("Generation failed") || !app.HasText("Generation details") {
		t.Fatalf("the first-generation failure left no canvas recovery: %v", app.Texts())
	}
	if got := accessButtonCount(app, "Retry"); got != 1 {
		t.Fatalf("Retry button count = %d, want one canvas action and no pill duplicate: %+v", got, app.AccessAll())
	}
	if retry.Bounds.Y >= status.Bounds.Y {
		t.Errorf("Retry bounds %+v are not in the content above the command status %+v", retry.Bounds, status.Bounds)
	}
	details, ok := app.AccessByRole(ui.RoleButton, "Generation failure details")
	if !ok || details.Bounds.Y >= status.Bounds.Y {
		t.Errorf("Details bounds %+v, present %t; want the secondary action in the content", details.Bounds, ok)
	}
	if app.HasText("No view yet") {
		t.Error("the failure recovery is contradicted by the empty state")
	}
	if err := app.Click("Generation failure details"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !v.failureOpen || !app.HasText("stderr: unknown flag") {
		t.Fatal("Details did not open the full failure in a popover")
	}
	if got := accessButtonCount(app, "Retry"); got != 1 {
		t.Errorf("Details added a duplicate Retry; button count = %d", got)
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.failureOpen || !app.Focused("Generation failure details") {
		t.Error("Escape did not close failure details and return focus to Details")
	}
}

func TestRegenerationFailureKeepsOldViewAndOnePillRecovery(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	v.failure = &genui.GenerateFailure{Kind: genui.FailTimeout, Message: "Timed out", Detail: "deadline exceeded"}
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()

	title, ok := app.Find("First card")
	if !ok {
		t.Fatalf("the old view disappeared after regeneration failed: %v", app.Texts())
	}
	if _, ok := app.AccessByRole(ui.RoleAlert, "Generation failed"); ok {
		t.Error("a regeneration failure drew a second recovery alert over the old view")
	}
	retry, ok := app.AccessByRole(ui.RoleButton, "Retry")
	if !ok || retry.Bounds.Y <= title.Y {
		t.Fatalf("the old view has no command-pill recovery: %+v, present %t", retry.Bounds, ok)
	}
	if got := accessButtonCount(app, "Retry"); got != 1 {
		t.Fatalf("Retry button count = %d, want the single command-pill action", got)
	}
	if err := app.Click("Generation failure details"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !v.failureOpen || !app.HasText("deadline exceeded") {
		t.Fatal("the command-pill Details action did not show diagnostics")
	}
	if got := accessButtonCount(app, "Retry"); got != 1 {
		t.Errorf("Details added a duplicate Retry; button count = %d", got)
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.failureOpen || !app.Focused("Generation failure details") {
		t.Error("Escape did not return from diagnostics to the pill Details action")
	}
}

func TestFailureRecoveryDispatchesByKind(t *testing.T) {
	tests := []struct {
		name, kind, action string
		settings           string
		signIn, retry      bool
	}{
		{name: "missing agent", kind: genui.FailAgentNotFound, action: "Configure agent", settings: "genui"},
		{name: "invalid model", kind: genui.FailModelInvalid, action: "Choose model", settings: "genui"},
		{name: "authentication", kind: genui.FailAuthRequired, action: "Sign in in Terminal", signIn: true},
		{name: "temporary", kind: genui.FailTimeout, action: "Retry", retry: true},
		{name: "empty kind fallback", action: "Retry", retry: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, v, d := polishPanel()
			v.failure = &genui.GenerateFailure{Kind: test.kind, Message: "Generation did not finish"}
			v.lastIDs = []string{"risks", "summary"}
			var openedSettings string
			p.openSettings = func(tab string) { openedSettings = tab }
			signed := make(chan string, 1)
			p.signIn = func(id string) error { signed <- id; return nil }
			var retried []string
			p.startGeneration = func(_ *documentView, ids []string) { retried = slices.Clone(ids) }
			app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
			app.Frame()

			if got := accessButtonCount(app, test.action); got != 1 {
				t.Fatalf("%q button count = %d, want one kind-specific primary action: %+v", test.action, got, app.AccessAll())
			}
			if test.action != "Retry" {
				if got := accessButtonCount(app, "Retry"); got != 0 {
					t.Fatalf("%s incorrectly fell back to Retry", test.kind)
				}
			}
			if err := app.Click(test.action); err != nil {
				t.Fatal(err)
			}
			if openedSettings != test.settings {
				t.Errorf("opened settings = %q, want %q", openedSettings, test.settings)
			}
			if test.signIn {
				select {
				case id := <-signed:
					if want, _ := p.settings.GenUI.Active(); id != want {
						t.Errorf("signed in backend %q, want active backend %q", id, want)
					}
				case <-time.After(time.Second):
					t.Fatal("the authentication action did not open the active backend's login")
				}
			}
			if test.retry && !slices.Equal(retried, v.lastIDs) {
				t.Errorf("Retry IDs = %v, want the last generation IDs %v", retried, v.lastIDs)
			}
			if !test.retry && retried != nil {
				t.Errorf("%s unexpectedly retried with IDs %v", test.kind, retried)
			}
		})
	}
}

func TestGenUIAuthFailureTargetsFailedBackendAfterActiveSwitch(t *testing.T) {
	p, v, d := polishPanel()
	v.failure = &genui.GenerateFailure{Kind: genui.FailAuthRequired, Message: "Session expired"}
	v.failureBackend = "codex"
	p.settings.GenUI.Backend = "antigravity"
	signed := make(chan string, 1)
	p.signIn = func(id string) error { signed <- id; return nil }
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	if err := app.Click("Sign in in Terminal"); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-signed:
		if id != "codex" {
			t.Fatalf("signed in backend %q, want failed backend %q", id, "codex")
		}
	case <-time.After(time.Second):
		t.Fatal("authentication recovery did not sign in failed backend")
	}
}

func accessButtonCount(app *ui.Tester, name string) int {
	count := 0
	for _, info := range app.AccessAll() {
		if info.Role == ui.RoleButton && info.Name == name {
			count++
		}
	}
	return count
}

func TestUpwardPopoverPointerDismissalDoesNotStealFocus(t *testing.T) {
	p, v, d := polishPanel()
	v.options = true
	outside := ""
	app := polishTester(func(c *ui.Context) {
		drawPolishedPanel(c, p, v, d, false)
		ui.Column(c).Absolute().Left(20).Top(300).Width(300).Children(func() {
			portrait.TextField(c, &outside, portrait.FieldSpec{Label: "Outside field"})
		})
	}, polishSize, 480)
	app.Frame()

	clickField(t, app, "Outside field")
	if v.options {
		t.Fatal("clicking outside did not dismiss GenUI options")
	}
	if !app.Focused("Outside field") {
		t.Error("pointer dismissal stole focus back to GenUI options")
	}
	if err := app.Click("GenUI options"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.options || !app.Focused("GenUI options") {
		t.Error("Escape did not close options and return focus to their trigger")
	}
}

func TestGenerationStartHandsComposerFocusToCancel(t *testing.T) {
	p, v, d := polishPanel()
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	clickField(t, app, "Generation request")

	cancelled := false
	v.cancel = func() { cancelled = true }
	v.focusCancel = true
	v.message = "Thinking"
	app.Frame()
	if !app.Focused("Cancel generation") {
		t.Fatalf("starting generation left focus nowhere: %+v", app.AccessAll())
	}
	if !app.HasText("Thinking") {
		t.Error("the generation step is missing from the progress pill")
	}
	if err := app.Click("Cancel generation"); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Error("the progress pill did not cancel generation")
	}
}

func TestTheEmptyStateFocusesTheFloatingComposer(t *testing.T) {
	p, v, d := polishPanel()
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	if err := app.Click("Write a request"); err != nil {
		t.Fatalf("the empty state offers no request action: %v", app.Texts())
	}
	app.Frame()
	if !app.Focused("Generation request") {
		t.Error("the empty state did not move focus to the floating composer")
	}
}

func TestNotePopoverDoesNotMoveCardsAndRestoresFocus(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first, second])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
		`second = Source(["overview"], [Card([Title("Second card", "md"), Text("Beta")])])`,
	)
	outside := ""
	app := polishTester(func(c *ui.Context) {
		drawPolishedPanel(c, p, v, d, false)
		ui.Column(c).Absolute().Left(20).Top(410).Width(220).Children(func() {
			portrait.TextField(c, &outside, portrait.FieldSpec{Label: "Outside field"})
		})
	}, polishSize, 520)
	app.Frame()
	firstBefore, _ := app.Find("First card")
	secondBefore, _ := app.Find("Second card")
	add := firstAccess(t, app, "Add note")
	app.ClickAt(add.Bounds.X+add.Bounds.W/2, add.Bounds.Y+add.Bounds.H/2)
	app.Frame()

	if v.noteTarget == "" || !app.Focused("Reader note") {
		t.Fatalf("Add note did not open and focus its popover: target=%q texts=%v", v.noteTarget, app.Texts())
	}
	assertPopoversInside(t, app, ui.Rect{W: polishSize, H: 520})
	firstAfter, _ := app.Find("First card")
	secondAfter, _ := app.Find("Second card")
	if firstAfter != firstBefore || secondAfter != secondBefore {
		t.Errorf("opening Add note moved cards from %+v/%+v to %+v/%+v", firstBefore, secondBefore, firstAfter, secondAfter)
	}
	app.Type("Draft note")
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != "" || v.note != "" || !app.Focused("Add note") {
		t.Errorf("Escape did not dismiss the note and restore its trigger: target=%q note=%q", v.noteTarget, v.note)
	}

	add = firstAccess(t, app, "Add note")
	app.ClickAt(add.Bounds.X+add.Bounds.W/2, add.Bounds.Y+add.Bounds.H/2)
	app.Frame()
	clickField(t, app, "Outside field")
	if v.noteTarget != "" || !app.Focused("Outside field") {
		t.Error("outside pointer dismissal did not preserve the clicked field's focus")
	}
}

func TestNotePopoverStaysInsideANarrowPane(t *testing.T) {
	const paneX, paneWidth, paneHeight = 600, 240, 480
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	v.noteTarget = "root/0"
	app := polishTester(func(c *ui.Context) {
		ui.Column(c).Absolute().Left(paneX).Top(0).Width(paneWidth).Height(paneHeight).Children(func() {
			drawPolishedPanel(c, p, v, d, false)
		})
	}, 900, paneHeight)
	app.Frame()
	if !app.Focused("Reader note") {
		t.Fatal("the narrow-pane note popover did not focus its editor")
	}
	assertPopoversInside(t, app, ui.Rect{X: paneX, Y: 0, W: paneWidth, H: paneHeight})
}

func firstAccess(t *testing.T, app *ui.Tester, name string) ui.AccessInfo {
	t.Helper()
	for _, info := range app.AccessAll() {
		if info.Name == name {
			return info
		}
	}
	t.Fatalf("no accessible control %q: %+v", name, app.AccessAll())
	return ui.AccessInfo{}
}

func TestCornerActionsRevealOnHoverAndKeyboardFocusWithoutReflow(t *testing.T) {
	for _, keyboard := range []bool{false, true} {
		t.Run(map[bool]string{false: "hover", true: "focus"}[keyboard], func(t *testing.T) {
			p, v, d := polishPanel()
			polishStatements(t, p, v,
				`root = Col([first, second])`,
				`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
				`second = Source(["overview"], [Card([Title("Second card", "md"), Text("Beta")])])`,
			)
			app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 520)
			app.Frame()
			firstBefore, _ := app.Find("First card")
			secondBefore, _ := app.Find("Second card")
			rest := app.Image()

			if keyboard {
				for range 12 {
					app.Key(0, ui.KeyTab)
					app.Frame()
					if app.Focused("Add note") {
						break
					}
				}
				if !app.Focused("Add note") {
					t.Fatal("keyboard navigation did not reach the hidden-at-rest Add note action")
				}
			} else {
				app.Move(firstBefore.X+firstBefore.W/2, firstBefore.Y+firstBefore.H/2)
				app.Frame()
			}
			if changedPixels(rest, app.Image()) == 0 {
				t.Error("hover/focus did not reveal the corner micro-toolbar")
			}
			firstAfter, _ := app.Find("First card")
			secondAfter, _ := app.Find("Second card")
			if firstAfter != firstBefore || secondAfter != secondBefore {
				t.Error("revealing the corner micro-toolbar moved card content")
			}
			add := firstAccess(t, app, "Add note")
			if add.Bounds.Y >= secondBefore.Y || add.Bounds.H != portrait.Sizes.Compact {
				t.Errorf("Add note bounds %+v are not a compact top-corner overlay", add.Bounds)
			}
		})
	}
}

func TestSavedNotesAreFramelessLeftRuleAnnotations(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	v.index("# Overview\n\nAn overview.\n\n<!-- note: Remember this -->\n")
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 480)
	app.Frame()
	note, ok := app.Find("Remember this")
	if !ok {
		t.Fatalf("saved note is not shown: %v", app.Texts())
	}
	pixel := app.Image().RGBAAt(int(note.X+1), int(note.Y+1))
	paper := portrait.Colors.Paper
	if pixel.R != paper.R || pixel.G != paper.G || pixel.B != paper.B {
		t.Errorf("saved note padding is %+v, want frameless paper %+v", pixel, paper)
	}
	if app.HasText("From") {
		t.Error("the removed provenance footer is still present")
	}
}

func TestReaderNoteActionsRunCancelThenSaveFromTheRight(t *testing.T) {
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	v.noteTarget, v.note = "root/0", "Draft note"
	app := polishTester(func(c *ui.Context) { drawPolishedPanel(c, p, v, d, false) }, polishSize, 420)
	app.Frame()

	save, ok := app.Access("Save note")
	if !ok {
		t.Fatalf("the note composer offers no save: %v", app.Texts())
	}
	cancel, ok := app.Access("Cancel note")
	if !ok {
		t.Fatalf("the note composer offers no cancel: %v", app.Texts())
	}
	if cancel.Bounds.Y > save.Bounds.Y+save.Bounds.H || save.Bounds.Y > cancel.Bounds.Y+cancel.Bounds.H {
		t.Errorf("cancel sits at y %.0f and save at y %.0f, want them on one row", cancel.Bounds.Y, save.Bounds.Y)
	}
	if cancel.Bounds.X >= save.Bounds.X {
		t.Errorf("cancel starts at x %.0f and save at x %.0f, want cancel on the left", cancel.Bounds.X, save.Bounds.X)
	}
	if right := save.Bounds.X + save.Bounds.W; right < polishWidth*0.75 {
		t.Errorf("save ends at x %.0f in a %.0f panel, want the actions along the right edge", right, polishWidth)
	}
	assertPopoversInside(t, app, ui.Rect{W: polishSize, H: 420})
}

func TestShowAllKeepsItsOwnWidthInsideTheCard(t *testing.T) {
	view, err := Open(`{"version":2,"sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Report","items":["One","Two","Three","Four","Five","Six"],"sources":["overview"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	reader := &Reading{}
	reader.Load(view)
	app := ui.NewTester(reader.View, polishSize, 480)
	app.Frame()

	show, ok := app.Access("Show all 6")
	if !ok {
		t.Fatalf("a long list offers no way to show every point: %v", app.Texts())
	}
	if show.Bounds.W > polishWidth/2 || show.Bounds.X > polishWidth/4 {
		t.Errorf("Show all 6 is %.0f wide at x %.0f, want a button at the card's left edge", show.Bounds.W, show.Bounds.X)
	}
	if err := app.Click("Show all 6"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.HasText("Six") || !app.HasText("Show fewer") {
		t.Errorf("expanding the list shows %v, want every point and a way back", app.Texts())
	}
}
