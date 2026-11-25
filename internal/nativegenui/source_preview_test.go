package nativegenui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func newSourcePreviewView(t *testing.T) *documentView {
	t.Helper()
	view, err := Open(`{"version":2,"sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Report","items":["A derived point"],"sources":["overview"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	v := &documentView{}
	v.reading.Load(view)
	return v
}

func sourcePreviewTesterWith(v *documentView, preferences ui.Preferences) *ui.Tester {
	p := &panel{win: &nativeui.Window{}}
	v.reading.Sources = func(c *ui.Context, n Node) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, n, false)
	}
	preferences.ReduceMotion = true
	return ui.NewTesterWith(v.drawReading, 620, 360, ui.TesterOptions{Preferences: preferences})
}

func sourcePreviewTester(v *documentView) *ui.Tester {
	return sourcePreviewTesterWith(v, ui.Preferences{})
}

func hoverSourcePreview(t *testing.T, app *ui.Tester) ui.AccessInfo {
	t.Helper()
	heading, ok := app.Access("Overview")
	if !ok {
		t.Fatal("the GenUI view has no Overview source")
	}
	app.Move(heading.Bounds.X+heading.Bounds.W/2, heading.Bounds.Y+heading.Bounds.H/2)
	app.Frame()
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview")
	if !ok {
		t.Fatalf("hovering the source did not open its preview: %v", app.Texts())
	}
	return popup
}

func TestSourcePreviewOpensForFocusAndHoverAndEscapeKeepsFocus(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.Frame()

	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Overview") {
		t.Fatal("Tab did not focus the source")
	}
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok || !app.HasText("An overview.") {
		t.Fatalf("focusing the source did not show its excerpt: %v", app.Texts())
	}

	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); ok || app.HasText("An overview.") {
		t.Fatal("Escape did not close the source preview")
	}
	if !app.Focused("Overview") {
		t.Fatal("Escape moved focus away from the source")
	}

	app.Key(0, ui.KeyTab)
	app.Move(600, 340)
	hoverSourcePreview(t, app)
	app.Move(600, 340)
	app.Advance(400 * time.Millisecond)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); ok {
		t.Fatal("moving away did not close the source preview")
	}
}

func TestSourcePreviewHasOneActiveSourceWhenFocusAndHoverDiffer(t *testing.T) {
	for _, separateCards := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate cards %t", separateCards), func(t *testing.T) {
			v := newTwoSourcePreviewView(t, separateCards)
			app := sourcePreviewTester(v)
			app.Frame()

			app.Key(0, ui.KeyTab)
			app.Frame()
			if separateCards {
				app.Key(0, ui.KeyEscape)
				app.Frame()
			}
			other, ok := app.Access("Other")
			if !ok {
				t.Fatal("the GenUI view has no Other source")
			}
			app.Move(other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2)
			app.Frame()

			var popups []ui.AccessInfo
			for _, access := range app.AccessAll() {
				if access.Role == ui.RolePopup {
					popups = append(popups, access)
				}
			}
			if len(popups) != 1 || popups[0].Name != "Source preview: Other" {
				t.Fatalf("focus on Overview and hover on Other opened %v, want only the hovered Other preview", popups)
			}
			app.Key(0, ui.KeyEscape)
			app.Frame()
			for _, access := range app.AccessAll() {
				if access.Role == ui.RolePopup {
					t.Fatalf("Escape left a source preview open: %v", access)
				}
			}
			if !app.Focused("Overview") {
				t.Fatal("Escape moved focus away from Overview")
			}
			app.Move(600, 340)
			app.Frame()
			if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
				t.Fatal("leaving the hovered source reopened the focused source after Escape")
			}
			app.Move(other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2)
			app.Frame()
			if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other"); !ok {
				t.Fatal("a new hover did not reopen the preview after Escape")
			}
		})
	}
}

func TestSourcePreviewEscapeDoesNotRevealSourceUnderStationaryPointer(t *testing.T) {
	v := newTwoSourcePreviewView(t, true)
	app := sourcePreviewTester(v)
	app.Frame()

	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Overview") {
		t.Fatal("Tab did not focus Overview")
	}
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview")
	if !ok {
		t.Fatalf("focusing Overview did not open its preview: %v", app.Texts())
	}
	other, ok := app.Access("Other")
	if !ok {
		t.Fatal("the GenUI view has no Other source")
	}
	otherX, otherY := other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2
	if otherX < popup.Bounds.X || otherX > popup.Bounds.X+popup.Bounds.W || otherY < popup.Bounds.Y || otherY > popup.Bounds.Y+popup.Bounds.H {
		t.Fatalf("Other at (%v, %v) is not covered by Overview's popup %+v", otherX, otherY, popup.Bounds)
	}

	app.Move(otherX, otherY)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok {
		t.Fatal("moving into Overview's popup closed it before Escape")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if popup, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatalf("Escape revealed %q under the stationary pointer", popup.Name)
	}
	if !app.Focused("Overview") {
		t.Fatal("Escape moved focus away from Overview")
	}

	app.Move(600, 340)
	app.Frame()
	app.Move(otherX, otherY)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other"); !ok {
		t.Fatal("moving the pointer onto Other did not reopen its preview after Escape")
	}
}

func TestSourcePreviewKeepsHoveredSourceWhenPointerEntersPopup(t *testing.T) {
	v := newTwoSourcePreviewView(t, false)
	app := sourcePreviewTester(v)
	app.Frame()

	app.Key(0, ui.KeyTab)
	app.Frame()
	other, ok := app.Access("Other")
	if !ok {
		t.Fatal("the GenUI view has no Other source")
	}
	app.Move(other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2)
	app.Frame()
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other")
	if !ok || !app.HasText("Another excerpt.") {
		t.Fatalf("hovering Other with Overview focused did not show Other's excerpt: %v", app.Texts())
	}

	app.Move(popup.Bounds.X+popup.Bounds.W/2, popup.Bounds.Y+popup.Bounds.H/2)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other"); !ok || !app.HasText("Another excerpt.") || app.HasText("An overview.") {
		t.Fatalf("entering Other's popup restored the focused Overview preview: %v", app.Texts())
	}
}

func TestSourcePreviewLatestFocusReplacesStationaryHover(t *testing.T) {
	v := newTwoSourcePreviewView(t, false)
	app := sourcePreviewTester(v)
	app.Frame()

	app.Move(0, 0)
	app.Frame()
	overview, ok := app.Access("Overview")
	if !ok {
		t.Fatal("the GenUI view has no Overview source")
	}
	app.Move(overview.Bounds.X+overview.Bounds.W/2, overview.Bounds.Y+overview.Bounds.H/2)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok {
		t.Fatalf("hovering Overview did not open its preview: %v", app.Texts())
	}

	app.Key(0, ui.KeyTab)
	app.Frame()
	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Other") {
		t.Fatal("two Tab presses did not focus Other")
	}
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other"); !ok || !app.HasText("Another excerpt.") || app.HasText("An overview.") {
		t.Fatalf("the newer Other focus did not replace the stationary Overview hover: %v", app.Texts())
	}
}

func newTwoSourcePreviewView(t *testing.T, separateCards bool) *documentView {
	t.Helper()
	components := `[{"id":"root","component":"KeyPoints","title":"Report","items":["A derived point"],"sources":["overview","other"]}]`
	if separateCards {
		components = `[{"id":"root","component":"Column","children":["first","second"]},{"id":"first","component":"KeyPoints","title":"Report","items":["A derived point"],"sources":["overview"]},{"id":"second","component":"KeyPoints","title":"Details","items":["Another derived point"],"sources":["other"]}]`
	}
	view, err := Open(`{"version":2,"sources":{"overview":{"heading":"Overview"},"other":{"heading":"Other"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":` + components + `}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	v := &documentView{}
	v.reading.Load(view)
	v.index("# Overview\n\nAn overview.\n\n# Other\n\nAnother excerpt.\n")
	return v
}

func TestSourcePreviewOpensForFocusWithoutHoverCapability(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTesterWith(v, ui.Preferences{NoHover: true})
	app.Frame()
	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Overview") || !app.HasText("An overview.") {
		t.Fatalf("focus did not open the preview without hover capability: %v", app.Texts())
	}
}

func TestSourcePreviewStaysOpenWhilePointerMovesIntoIt(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.Frame()
	heading, _ := app.Access("Overview")
	popup := hoverSourcePreview(t, app)

	app.Move(popup.Bounds.X+popup.Bounds.W/2, heading.Bounds.Y+heading.Bounds.H+2)
	app.Advance(100 * time.Millisecond)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok {
		t.Fatal("the preview closed in the gap before the pointer could enter it")
	}
	app.Move(popup.Bounds.X+popup.Bounds.W/2, popup.Bounds.Y+popup.Bounds.H/2)
	app.Advance(400 * time.Millisecond)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok {
		t.Fatal("the preview closed while the pointer rested inside it")
	}

	app.Key(0, ui.KeyEscape)
	app.Frame()
	app.Move(600, 340)
	app.Frame()
	hoverSourcePreview(t, app)
}

func TestSourcePreviewMissingSourceHasAKeyboardStatusEntry(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Another section\n\nOther text.\n")
	app := sourcePreviewTesterWith(v, ui.Preferences{NoHover: true})
	app.Frame()

	const statusName = "Overview, source missing"
	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused(statusName) {
		t.Fatalf("Tab did not focus the missing source status: %v", app.Texts())
	}
	status, ok := app.AccessByRole(ui.RoleStatus, statusName)
	if !ok || !status.Focusable {
		t.Fatalf("the missing source has no named, focusable status entry: %+v", status)
	}
	if jump, ok := app.AccessByRole(ui.RoleButton, "Overview"); !ok || !jump.Disabled {
		t.Fatalf("the missing source jump is not disabled: %+v", jump)
	}
	if !app.HasText("This section is no longer in the document.") {
		t.Fatalf("focusing the missing source did not open its preview: %v", app.Texts())
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); ok {
		t.Fatal("Escape did not close the missing source preview")
	}
	if !app.Focused(statusName) {
		t.Fatal("Escape moved focus away from the missing source status")
	}
}

func TestSourcePreviewReportsChangedAndMissingSectionsInsteadOfStaleExcerpts(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nOld paragraph.\n")
	ref := v.reading.surface.Sources["overview"]
	ref.Hash = v.sections[headingKey{name: "overview", occurrence: 1}].Hash
	v.reading.surface.Sources["overview"] = ref
	app := sourcePreviewTester(v)
	app.Frame()
	hoverSourcePreview(t, app)
	if !app.HasText("Old paragraph.") {
		t.Fatal("the unchanged source did not show its excerpt")
	}

	v.index("# Overview\n\nNew paragraph.\n")
	app.Frame()
	if !app.HasText("This section changed after the view was generated.") || app.HasText("Old paragraph.") || app.HasText("New paragraph.") {
		t.Fatalf("a changed source did not replace its excerpt with the changed state: %v", app.Texts())
	}
	app.Move(600, 340)
	app.Advance(400 * time.Millisecond)
	hoverSourcePreview(t, app)
	if !app.HasText("This section changed after the view was generated.") || app.HasText("New paragraph.") {
		t.Fatalf("reopening a changed source did not show only its changed state: %v", app.Texts())
	}

	v.index("# Another section\n\nOther text.\n")
	app.Frame()
	if !app.HasText("This section is no longer in the document.") || app.HasText("Old paragraph.") || app.HasText("New paragraph.") {
		t.Fatalf("a missing source did not replace its excerpt with the missing state: %v", app.Texts())
	}
	app.Move(600, 340)
	app.Advance(400 * time.Millisecond)
	hoverSourcePreview(t, app)
	if !app.HasText("This section is no longer in the document.") || app.HasText("Old paragraph.") {
		t.Fatalf("reopening a missing source did not show only its missing state: %v", app.Texts())
	}
}

func TestSourcePreviewFitsANarrowReader(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.SetSize(320, 360)
	app.Key(0, ui.KeyTab)
	app.Frame()
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview")
	if !ok {
		t.Fatal("the narrow reader did not open its source preview")
	}
	if popup.Bounds.X < 0 || popup.Bounds.X+popup.Bounds.W > 320 {
		t.Errorf("the preview extends outside the narrow reader: %+v", popup.Bounds)
	}
}

func TestSourcePreviewScrollsToTheExcerptEndInANarrowReader(t *testing.T) {
	excerpt := "# Overview\n\n" + strings.Repeat("摘", 569) + "末"
	v := newSourcePreviewView(t)
	v.index(excerpt + "\n")
	app := sourcePreviewTester(v)
	app.SetSize(320, 360)
	app.Key(0, ui.KeyTab)
	app.Frame()
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview")
	if !ok {
		t.Fatal("the narrow reader did not open its source preview")
	}
	before, ok := app.Find(excerpt)
	if !ok || before.H == 0 {
		t.Fatalf("the long excerpt is not visible in its popup: excerpt=%+v popup=%+v", before, popup.Bounds)
	}

	app.Scroll(popup.Bounds.X+popup.Bounds.W/2, popup.Bounds.Y+popup.Bounds.H/2, 0, 2000)
	app.Frame()
	after, ok := app.Find(excerpt)
	if !ok || after.Y >= before.Y || after.Y+after.H > popup.Bounds.Y+popup.Bounds.H+1 {
		t.Fatalf("scrolling did not reveal the excerpt ending: before=%+v after=%+v popup=%+v", before, after, popup.Bounds)
	}
}

func TestSourcePreviewWithoutAHashShowsExcerptAndVersionNotice(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nLegacy excerpt.\n")
	app := sourcePreviewTester(v)
	app.Frame()
	hoverSourcePreview(t, app)
	if !app.HasText("Legacy excerpt.") {
		t.Fatalf("a legacy source without a hash did not show its excerpt: %v", app.Texts())
	}
	if !app.HasText("Source version at generation time cannot be verified.") {
		t.Fatalf("a legacy source without a hash did not explain its verification limit: %v", app.Texts())
	}
}

func TestSourcePreviewCachesExcerptsUntilTheSourceChanges(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nOld paragraph.\n")
	computes := 0
	v.excerptSource = func(raw string, section workspace.Section) string {
		computes++
		return sourceExcerpt(raw, section)
	}
	app := sourcePreviewTester(v)
	app.Frame()
	hoverSourcePreview(t, app)
	before := computes
	if before == 0 {
		t.Fatal("opening the preview did not compute its excerpt")
	}
	app.Frame()
	app.Frame()
	if computes != before {
		t.Fatalf("redrawing an unchanged source computed the excerpt %d more times, want none", computes-before)
	}

	v.index("# Overview\n\nNew paragraph.\n")
	app.Frame()
	if computes != before+1 {
		t.Fatalf("changing the source computed the excerpt %d times, want once", computes-before)
	}
	if !app.HasText("New paragraph.") || app.HasText("Old paragraph.") {
		t.Fatalf("the changed source did not replace its cached excerpt: %v", app.Texts())
	}
	before = computes
	app.Frame()
	if computes != before {
		t.Fatalf("redrawing the changed source recomputed its excerpt %d times", computes-before)
	}
}

func TestSourcePreviewCapsTheExcerptLength(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\n" + strings.Repeat("Original paragraph ", 50) + "ending sentinel\n")
	app := sourcePreviewTester(v)
	app.Frame()
	hoverSourcePreview(t, app)

	var excerpt string
	for _, text := range app.Texts() {
		if strings.HasPrefix(text, "# Overview") {
			excerpt = text
			break
		}
	}
	if excerpt == "" {
		t.Fatalf("the source preview has no excerpt: %v", app.Texts())
	}
	if got := utf8.RuneCountInString(excerpt); got > 601 {
		t.Errorf("the excerpt has %d runes, want at most 600 plus an ellipsis", got)
	}
	if !strings.HasSuffix(excerpt, "…") || strings.Contains(excerpt, "ending sentinel") {
		t.Errorf("the long excerpt is not visibly truncated before its ending: %q", excerpt)
	}
}

func TestSourcePreviewReopensOnHoverAfterTheDismissedSourceLeavesTheView(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n\n# Other\n\nAnother excerpt.\n")
	app := sourcePreviewTester(v)
	app.Frame()
	hoverSourcePreview(t, app)
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("Escape did not close the preview")
	}

	view, err := Open(`{"version":2,"sources":{"other":{"heading":"Other"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Details","items":["Another derived point"],"sources":["other"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	v.reading.Load(view)
	app.Frame()
	app.Move(600, 340)
	app.Frame()
	other, ok := app.Access("Other")
	if !ok {
		t.Fatalf("the updated view has no Other source: %v", app.Texts())
	}
	app.Move(other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Other"); !ok {
		t.Fatalf("hovering Other after the dismissed source left the view did not open its preview: %v", app.Texts())
	}
}
