package nativegenui

import (
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
	return ui.NewTesterWith(v.reading.View, 620, 360, ui.TesterOptions{Preferences: preferences})
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

	v.index("# Another section\n\nOther text.\n")
	app.Frame()
	if !app.HasText("This section is no longer in the document.") || app.HasText("Old paragraph.") || app.HasText("New paragraph.") {
		t.Fatalf("a missing source did not replace its excerpt with the missing state: %v", app.Texts())
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
