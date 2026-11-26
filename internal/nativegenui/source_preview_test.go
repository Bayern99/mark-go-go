package nativegenui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
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

func openSourcePreview(t *testing.T, app *ui.Tester, name string) ui.AccessInfo {
	t.Helper()
	if err := app.Click(name); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Frame()
	popup, ok := app.AccessByRole(ui.RolePopup, "Source preview: "+name)
	if !ok {
		t.Fatalf("clicking the source did not pin its preview: %v", app.Texts())
	}
	return popup
}

func TestSourcePreviewOpensOnlyOnClickAndStaysPinned(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.Frame()

	heading, ok := app.Access("Overview")
	if !ok {
		t.Fatal("the GenUI view has no Overview source")
	}
	app.Move(heading.Bounds.X+heading.Bounds.W/2, heading.Bounds.Y+heading.Bounds.H/2)
	app.Frame()
	app.Key(0, ui.KeyTab)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("hover or keyboard focus opened a source preview without a click")
	}

	popup := openSourcePreview(t, app, "Overview")
	if !app.HasText("An overview.") || !app.HasText("Open in Markdown") {
		t.Fatalf("the pinned preview omits its source or explicit navigation action: %v", app.Texts())
	}
	app.Move(600, 340)
	app.Advance(400 * time.Millisecond)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok || !app.HasText("An overview.") {
		t.Fatal("moving the pointer away dismissed the pinned source preview")
	}
	app.Move(popup.Bounds.X+popup.Bounds.W/2, popup.Bounds.Y+popup.Bounds.H/2)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, "Source preview: Overview"); !ok {
		t.Fatal("moving through the preview dismissed its pinned state")
	}
}

func TestSourcePreviewSameLinkEscapeAndCloseControlDismiss(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.Frame()

	openSourcePreview(t, app, "Overview")
	if err := app.Click("Overview"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if v.previewSource.open {
		t.Fatal("clicking the same source did not toggle its preview closed")
	}

	openSourcePreview(t, app, "Overview")
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.previewSource.open || !app.Focused("Overview") {
		t.Fatal("Escape did not close the preview and return focus to its source")
	}

	openSourcePreview(t, app, "Overview")
	if err := app.Click("Close"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if v.previewSource.open || !app.Focused("Overview") {
		t.Fatal("Close did not dismiss the preview and restore the source focus")
	}
}

func TestSourcePreviewOutsideDismissalPreservesClickedFocus(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	p := &panel{win: &nativeui.Window{}}
	v.reading.Sources = func(c *ui.Context, n Node) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, n, false)
	}
	outside := ""
	app := ui.NewTesterWith(func(c *ui.Context) {
		v.drawReading(c)
		portrait.TextField(c, &outside, portrait.FieldSpec{Label: "Outside field"}).Frame().Absolute().Left(20).Bottom(20)
	}, 620, 360, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	app.Frame()
	openSourcePreview(t, app, "Overview")

	clickField(t, app, "Outside field")
	app.Frame()
	if v.previewSource.open || !app.Focused("Outside field") {
		t.Fatal("outside dismissal stole focus from the clicked control")
	}
}

func TestSourcePreviewNavigationRequiresOpenInMarkdown(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	var opened []workspace.Section
	v.openSource = func(section workspace.Section) { opened = append(opened, section) }
	app := sourcePreviewTester(v)
	app.Frame()

	openSourcePreview(t, app, "Overview")
	if len(opened) != 0 {
		t.Fatal("clicking a source link navigated away before Open in Markdown")
	}
	if err := app.Click("Open in Markdown"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if len(opened) != 1 || opened[0].Heading != "Overview" {
		t.Fatalf("Open in Markdown revealed %+v, want the Overview section once", opened)
	}
	if v.previewSource.open {
		t.Fatal("Open in Markdown left the source preview pinned")
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

func TestSourcePreviewClickingAnotherSourceMovesThePin(t *testing.T) {
	for _, separateCards := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate cards %t", separateCards), func(t *testing.T) {
			v := newTwoSourcePreviewView(t, separateCards)
			app := sourcePreviewTester(v)
			app.Frame()
			openSourcePreview(t, app, "Overview")
			if separateCards {
				app.Key(0, ui.KeyEscape)
				app.Frame()
			}
			openSourcePreview(t, app, "Other")

			var popups []ui.AccessInfo
			for _, access := range app.AccessAll() {
				if access.Role == ui.RolePopup {
					popups = append(popups, access)
				}
			}
			if len(popups) != 1 || popups[0].Name != "Source preview: Other" || !app.HasText("Another excerpt.") || app.HasText("An overview.") {
				t.Fatalf("clicking Other did not move the pinned preview: %+v %v", popups, app.Texts())
			}
		})
	}
}

func TestSourcePreviewClickWorksWithoutHoverCapability(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTesterWith(v, ui.Preferences{NoHover: true})
	app.Frame()
	app.Key(0, ui.KeyTab)
	app.Frame()
	if !app.Focused("Overview") || v.previewSource.open {
		t.Fatal("focus alone opened the source preview")
	}
	openSourcePreview(t, app, "Overview")
	if !app.HasText("An overview.") {
		t.Fatalf("click did not open the excerpt without hover capability: %v", app.Texts())
	}
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
	if v.previewSource.open || app.HasText("This section is no longer in the document.") {
		t.Fatal("focusing a missing source opened a preview without a click")
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
	openSourcePreview(t, app, "Overview")
	if !app.HasText("Old paragraph.") {
		t.Fatal("the unchanged source did not show its excerpt")
	}

	v.index("# Overview\n\nNew paragraph.\n")
	app.Frame()
	if !app.HasText("This section changed after the view was generated.") || app.HasText("Old paragraph.") || app.HasText("New paragraph.") {
		t.Fatalf("a changed source did not replace its excerpt with the changed state: %v", app.Texts())
	}

	v.index("# Another section\n\nOther text.\n")
	app.Frame()
	if !app.HasText("This section is no longer in the document.") || app.HasText("Old paragraph.") || app.HasText("New paragraph.") {
		t.Fatalf("a missing source did not replace its excerpt with the missing state: %v", app.Texts())
	}
	if open, ok := app.Access("Open in Markdown"); !ok || !open.Disabled {
		t.Fatalf("a missing source still offers navigation: %+v", open)
	}
}

func TestSourcePreviewFitsANarrowReader(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n")
	app := sourcePreviewTester(v)
	app.SetSize(320, 360)
	app.Frame()
	popup := openSourcePreview(t, app, "Overview")
	if popup.Bounds.X < 0 || popup.Bounds.X+popup.Bounds.W > 320 {
		t.Errorf("the preview extends outside the narrow reader: %+v", popup.Bounds)
	}
}

func TestSourcePreviewStaysInsideANarrowGenUIPane(t *testing.T) {
	const paneX, paneWidth, paneHeight = 600, 240, 480
	p, v, d := polishPanel()
	polishStatements(t, p, v,
		`root = Col([first])`,
		`first = Source(["overview"], [Card([Title("First card", "md"), Text("Alpha")])])`,
	)
	app := polishTester(func(c *ui.Context) {
		ui.Column(c).Absolute().Left(paneX).Top(0).Width(paneWidth).Height(paneHeight).Children(func() {
			drawPolishedPanel(c, p, v, d, false)
		})
	}, 900, paneHeight)
	app.Frame()
	openSourcePreview(t, app, "Overview")
	assertPopoversInside(t, app, ui.Rect{X: paneX, Y: 0, W: paneWidth, H: paneHeight})
}

func TestSourcePreviewScrollsToTheExcerptEndInANarrowReader(t *testing.T) {
	excerpt := "# Overview\n\n" + strings.Repeat("摘", 569) + "末"
	v := newSourcePreviewView(t)
	v.index(excerpt + "\n")
	app := sourcePreviewTester(v)
	app.SetSize(320, 360)
	app.Frame()
	popup := openSourcePreview(t, app, "Overview")
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
	openSourcePreview(t, app, "Overview")
	if !app.HasText("Legacy excerpt.") || !app.HasText("Source version at generation time cannot be verified.") {
		t.Fatalf("a legacy source omitted its excerpt or verification notice: %v", app.Texts())
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
	openSourcePreview(t, app, "Overview")
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
	openSourcePreview(t, app, "Overview")

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

func TestSourcePreviewClosesWhenItsSourceLeavesTheView(t *testing.T) {
	v := newSourcePreviewView(t)
	v.index("# Overview\n\nAn overview.\n\n# Other\n\nAnother excerpt.\n")
	app := sourcePreviewTester(v)
	app.Frame()
	openSourcePreview(t, app, "Overview")

	view, err := Open(`{"version":2,"sources":{"other":{"heading":"Other"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","title":"Details","items":["Another derived point"],"sources":["other"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	v.reading.Load(view)
	app.Frame()
	if v.previewSource.open {
		t.Fatal("the pinned preview outlived its source target")
	}
	other, ok := app.Access("Other")
	if !ok {
		t.Fatalf("the updated view has no Other source: %v", app.Texts())
	}
	app.Move(other.Bounds.X+other.Bounds.W/2, other.Bounds.Y+other.Bounds.H/2)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RolePopup, ""); ok {
		t.Fatal("hovering the new source reopened a preview")
	}
	openSourcePreview(t, app, "Other")
}

func TestSourcePreviewDoesNotOpenWhenTheViewScrollsUnderAStillPointer(t *testing.T) {
	var ids, cards []string
	for i := 0; i < 12; i++ {
		id, source := fmt.Sprintf("card%d", i), "other"
		if i == 0 {
			source = "overview"
		}
		ids = append(ids, fmt.Sprintf("%q", id))
		cards = append(cards, fmt.Sprintf(`{"id":%q,"component":"KeyPoints","title":"Report","items":["A derived point"],"sources":[%q]}`, id, source))
	}
	view, err := Open(fmt.Sprintf(`{"version":2,"sources":{"overview":{"heading":"Overview"},"other":{"heading":"Other"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"v","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Column","children":[%s]},%s]}}]}`, strings.Join(ids, ","), strings.Join(cards, ",")))
	if err != nil {
		t.Fatal(err)
	}
	v := &documentView{}
	v.reading.Load(view)
	v.index("# Overview\n\nText.\n\n# Other\n\nOther text.\n")
	app := sourcePreviewTester(v)
	app.Frame()
	overview, _ := app.Access("Overview")
	other, ok := app.Access("Other")
	if !ok {
		t.Fatal("the GenUI view has no Other source")
	}
	x, y := overview.Bounds.X+overview.Bounds.W/3, overview.Bounds.Y+overview.Bounds.H/2
	app.Move(x, y)
	app.Frame()
	step := other.Bounds.Y - overview.Bounds.Y + 0.12345
	for i := 0; i < 3; i++ {
		app.Scroll(x, y, 0, step)
		app.Frame()
		app.Frame()
		if popup, ok := app.AccessByRole(ui.RolePopup, ""); ok {
			t.Fatalf("scroll %d under the still pointer opened %q", i+1, popup.Name)
		}
	}
}
