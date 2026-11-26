package nativegenui

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func TestReaderNoteShortcutsFollowNoteFocus(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	card := v.reading.surface.Nodes["card"]
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, card, false)
			portrait.Button(c, "Elsewhere", "Elsewhere").Absolute().Left(20).Bottom(20)
		})
	}, 620, 420)
	app.Frame()

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("Draft note")
	app.Frame()
	if v.note != "Draft note" || v.noteTarget != card.ID {
		t.Fatalf("note draft = %q for %q, want %q for %q", v.note, v.noteTarget, "Draft note", card.ID)
	}

	if err := app.Click("Elsewhere"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if v.noteTarget != "" || v.note != "" || !app.Focused("Elsewhere") {
		t.Fatalf("outside dismissal did not preserve pointer focus: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if !app.Focused("Reader note") {
		t.Fatal("clicking the note did not focus it")
	}
	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != "" || v.note != "" {
		t.Fatalf("Escape focused on the note did not cancel it: target=%q note=%q", v.noteTarget, v.note)
	}

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("Second draft")
	app.Frame()
	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	// The window has no open document, so reaching addNote reports it.
	if v.message != "Document closed" || v.noteTarget != card.ID || v.note != "Second draft" {
		t.Fatalf("Cmd+Enter focused on the note did not save it: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}
}

func TestReaderNoteShortcutsIgnoreActiveIMEComposition(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	card := v.reading.surface.Nodes["card"]
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, card, false)
	}, 620, 420)
	app.Frame()

	if err := app.Click("Add note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	if err := app.Click("Reader note"); err != nil {
		t.Fatal(err)
	}
	app.Frame()
	app.Type("keep ")
	app.Frame()

	// Start IME composition with active marked text.
	app.Compose("ceshi", 5)
	app.Frame()

	app.Key(0, ui.KeyEscape)
	app.Frame()
	if v.noteTarget != card.ID || v.note != "keep " || v.message != "" {
		t.Fatalf("Escape during active IME composition changed note: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	if v.noteTarget != card.ID || v.note != "keep " || v.message != "" {
		t.Fatalf("Cmd+Enter during active IME composition prematurely submitted note: target=%q note=%q message=%q", v.noteTarget, v.note, v.message)
	}

	// End composition and press Cmd+Enter; now it submits cleanly.
	app.Compose("", 0)
	app.Frame()
	app.Key(ui.Cmd, ui.KeyEnter)
	app.Frame()
	if v.message != "Document closed" || v.note != "keep " {
		t.Fatalf("Cmd+Enter after IME composition ended did not reach save: note=%q message=%q", v.note, v.message)
	}
}

func TestGenerationRequestLinesClampExplicitAndWrappedText(t *testing.T) {
	tests := []struct {
		name, text string
		width      float32
		want       int
	}{
		{name: "empty", want: 1},
		{name: "two explicit lines", text: "First\nSecond", want: 2},
		{name: "explicit lines cap at three", text: "First\nSecond\nThird\nFourth", want: 3},
		{name: "wrapped text grows", text: strings.Repeat("wrapped request words ", 12), width: 80, want: 3},
		{name: "short wrapped text", text: "Short", width: 320, want: 1},
	}
	app := ui.NewTester(func(c *ui.Context) {
		for _, test := range tests {
			if got := generationRequestLines(c, test.text, test.width); got != test.want {
				t.Errorf("%s: lines = %d, want %d", test.name, got, test.want)
			}
		}
	}, 640, 400)
	app.Frame()
}

func TestChangedIDsUsesStoredIDsForRepeatedInstances(t *testing.T) {
	view := &documentView{
		reading: Reading{surface: &Surface{
			Sources: map[string]Reference{
				"changed": {Heading: "Changed", Hash: "old"},
				"current": {Heading: "Current"},
			},
			Nodes: map[string]Node{
				"row-[/items/0]":   {ID: "row-[/items/0]", ComponentID: "rowTemplate", Sources: []string{"changed"}},
				"row-[/items/1]":   {ID: "row-[/items/1]", ComponentID: "rowTemplate", Sources: []string{"changed"}},
				"current-instance": {ID: "current-instance", ComponentID: "currentCard", Sources: []string{"current"}},
				"stale-instance":   {ID: "stale-instance", ComponentID: "staleCard", Sources: []string{"changed"}},
				"missing-instance": {ID: "missing-instance", ComponentID: "missingCard", Sources: []string{"missing"}},
				"legacy":           {ID: "legacy", Sources: []string{"changed"}},
			},
		}},
	}
	view.index("# Changed\n\nNew section text.\n\n# Current\n\nUnchanged section text.\n")
	currentReference := view.reading.surface.Sources["current"]
	currentReference.Hash = view.sections[headingKey{name: workspace.NormalizeHeadingText("Current"), occurrence: 1}].Hash
	view.reading.surface.Sources["current"] = currentReference

	got := view.changedIDs()
	want := []string{"legacy", "missingCard", "rowTemplate", "staleCard"}
	if !slices.Equal(got, want) {
		t.Fatalf("changed IDs = %v, want stored component IDs %v", got, want)
	}
	for _, id := range got {
		if id == "row-[/items/0]" || id == "row-[/items/1]" {
			t.Errorf("changed IDs included a runtime row ID: %q", id)
		}
	}
}

func conditionalSourceView(t *testing.T, qdrantHash string) string {
	t.Helper()
	components, err := json.Marshal([]string{
		`$option = "qdrant"`,
		`root = Col([option_card])`,
		`option_card = Card([Title("Options", "md"), option_notes])`,
		`option_notes = $option == "pgvector" ? pgvector_details : $option == "qdrant" ? qdrant_details : milvus_details`,
		`pgvector_details = Source(["pgvector"], [Text("pgvector details")])`,
		`qdrant_details = Source(["qdrant"], [Text("Qdrant details")])`,
		`milvus_details = Source(["milvus"], [Text("Milvus details")])`,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(`{"version":3,"language":"openui-lang/1.0","sources":{"pgvector":{"heading":"pgvector"},"qdrant":{"heading":"Qdrant 1.12","hash":%q},"milvus":{"heading":"Milvus","hash":"old"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":%s}}]}`, qdrantHash, components)
}

func TestChangedConditionalSourcesNameTheirStatementsEvenWhenHidden(t *testing.T) {
	v := &documentView{}
	p := &panel{win: &nativeui.Window{}, views: map[string]*documentView{"doc": v}}
	p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: conditionalSourceView(t, "old")})
	document := "# pgvector\n\nCurrent.\n\n# Qdrant 1.12\n\nChanged.\n\n# Milvus\n\nAlso changed.\n"
	v.index(document)
	if got, want := v.changedIDs(), []string{"milvus_details", "qdrant_details"}; !slices.Equal(got, want) {
		t.Fatalf("changed statements = %v, want both visible and hidden branches %v", got, want)
	}

	d := workspace.DocumentInfo{Path: "doc.md"}
	settings := ui.NewTester(func(c *ui.Context) { p.drawOptions(c, v, d, false) }, 640, 400)
	settings.Frame()
	if !settings.HasText("Regenerate changed (2)") {
		t.Fatalf("the update action does not count both changed branches: %v", settings.Texts())
	}
	v.reading.Sources = func(c *ui.Context, n Node) { p.drawSources(c, v, d, n, false) }
	app := ui.NewTester(v.reading.View, 640, 600)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleStatus, "Source changed"); !ok {
		t.Fatal("the visible Qdrant source is not marked changed before its update")
	}

	current := v.sections[headingKey{name: workspace.NormalizeHeadingText("Qdrant 1.12"), occurrence: 1}].Hash
	p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: conditionalSourceView(t, current)})
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleStatus, "Source changed"); ok {
		t.Error("the visible Qdrant source is still marked changed after its statement was updated")
	}
	if got, want := v.changedIDs(), []string{"milvus_details"}; !slices.Equal(got, want) {
		t.Errorf("changed statements after the Qdrant update = %v, want only the hidden stale branch %v", got, want)
	}
	settings.Frame()
	if !settings.HasText("Regenerate changed (1)") || settings.HasText("Regenerate changed (2)") {
		t.Errorf("the update action did not reduce its changed count: %v", settings.Texts())
	}
}

func TestLoadingShowsAShimmeringSkeletonUnderReadableStatus(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		app := ui.NewTesterWith(func(c *ui.Context) {
			loading(c, "Loading the GenUI view", "Reading the view file for this document.")
		}, 640, 420, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: reduce}})
		app.Frame()
		status, ok := app.AccessByRole(ui.RoleStatus, "Loading the GenUI view")
		if !ok || !status.Busy || !app.HasText("Reading the view file for this document.") {
			t.Fatalf("reduce %v: status %+v, texts %v", reduce, status, app.Texts())
		}
		before := app.Image()
		app.Advance(400 * time.Millisecond)
		moved := changedPixels(before, app.Image()) != 0
		if moved == reduce || app.FramePending() == reduce {
			t.Fatalf("reduce %v: skeleton shimmer moved %v, frames pending %v", reduce, moved, app.FramePending())
		}
	}
}

func TestSourceStatusSaysWhenTheSourceIsMissing(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"gone": {Heading: "Gone"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"gone"}}},
	}}}
	v.index("# Overview\n\nAn overview.\n")
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, v.reading.surface.Nodes["card"], false)
	}, 620, 200)
	app.Frame()
	if _, ok := app.AccessByRole(ui.RoleStatus, "Source missing"); !ok {
		t.Fatal("a missing source is not announced")
	}
	if heading, ok := app.Access("Gone"); !ok || !heading.Disabled {
		t.Fatalf("the missing source heading %+v, want disabled", heading)
	}
	if add, ok := app.Access("Add note"); !ok || !add.Disabled {
		t.Fatalf("Add note %+v, want disabled without a source", add)
	}
}

func changedPixels(a, b *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
			n++
		}
	}
	return n
}

func TestTheReaderStartsOverOnlyWhenAWholeNewViewArrives(t *testing.T) {
	file := func(messages string) genui.ViewEvent {
		return genui.ViewEvent{ID: "doc", Exists: true, Text: `{"version":3,"language":"openui-lang/1.0","sources":{},"a2ui":[` + messages + `]}`}
	}
	const created = `{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["$months = 2","root = Col([plan, risks])","plan = Card([Slider($months, 0, 10, 1, \"Months\", \"mo\"), Text(\"Total \" + $months * 150)])","risks = Card([Text(\"Old risks\")])"]}}`
	const update = `,{"version":"1.0","updateComponents":{"surfaceId":"view","components":["risks = Card([Text(\"New risks\")])"]}}`
	const again = `{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["$months = 4","root = Col([Slider($months, 0, 10, 1, \"Months\", \"mo\"), Text(\"Total \" + $months * 150)])"]}}`

	v := &documentView{}
	p := &panel{views: map[string]*documentView{"doc": v}}
	p.updated(file(created))
	app := ui.NewTester(v.reading.View, 600, 600)
	total := func() string {
		app.Frame()
		app.Frame()
		for _, text := range app.Texts() {
			if strings.HasPrefix(text, "Total ") {
				return text
			}
		}
		return strings.Join(app.Texts(), "\n")
	}
	choose := func(share float32) {
		app.Frame()
		slider, ok := app.Access("Months")
		if !ok {
			t.Fatal("no slider named Months")
		}
		box := slider.Bounds
		app.Press(box.X+box.W*0.2, box.Y+box.H/2)
		app.Move(box.X+box.W*share, box.Y+box.H/2)
		app.Release(box.X+box.W*share, box.Y+box.H/2)
	}
	generating := func(ids ...string) {
		v.cancel, v.lastIDs, v.reading.Replacing = func() {}, ids, ids
	}

	choose(0.8)
	if got := total(); got != "Total 1200" {
		t.Fatalf("dragging to 8 months shows %q", got)
	}

	generating("risks")
	if got := total(); got != "Total 1200" || !app.HasText("Updating card…") || app.HasText("Old risks") {
		t.Errorf("while a card is updated the page shows %q and %v, want a placeholder in place of the card", got, app.Texts())
	}
	v.finished(errors.New("the agent stopped"))
	if got := total(); got != "Total 1200" || app.HasText("Updating card…") || !app.HasText("Old risks") || v.failure == nil {
		t.Errorf("after an update that failed the page shows %q and %v, failure %v, want the card as it was", got, app.Texts(), v.failure)
	}

	generating("risks")
	v.finished(nil)
	p.updated(file(created + update))
	if got := total(); got != "Total 1200" || !app.HasText("New risks") {
		t.Errorf("after an update of another card the page shows %q, want the reader's Total 1200 beside the new card", got)
	}

	generating()
	v.finished(errors.New("the agent stopped"))
	if got := total(); got != "Total 1200" || v.failure == nil {
		t.Errorf("after a full generation that failed the page shows %q, failure %v", got, v.failure)
	}

	generating()
	v.finished(nil)
	p.updated(file(again))
	if got := total(); got != "Total 600" {
		t.Errorf("after a full generation the page shows %q, want the new view's Total 600", got)
	}
	if v.cancel != nil || v.reading.Replacing != nil {
		t.Error("the finished generation still counts as running")
	}

	choose(0.8)
	if got := total(); got != "Total 1200" {
		t.Fatalf("dragging to 8 months in the new view shows %q", got)
	}
	p.updated(file(created))
	if got := total(); got != "Total 300" {
		t.Errorf("after the full generation is undone the page shows %q, want the earlier view's Total 300", got)
	}

	choose(0.8)
	p.updated(file(created + update))
	if got := total(); got != "Total 1200" || !app.HasText("New risks") {
		t.Errorf("after an update written outside the panel the page shows %q, want the reader's Total 1200 beside the new card", got)
	}
	p.updated(file(again))
	if got := total(); got != "Total 600" {
		t.Errorf("after a whole new view written outside the panel the page shows %q, want the new view's Total 600", got)
	}
}

func TestBlankAtTheEndOfTheDocumentDoesNotMarkItsLastSectionChanged(t *testing.T) {
	v := &documentView{reading: Reading{surface: &Surface{
		Sources: map[string]Reference{"overview": {Heading: "Overview", Hash: "fnv1a32:57945685"}},
		Nodes:   map[string]Node{"card": {ID: "card", Sources: []string{"overview"}}},
	}}}
	p := &panel{win: &nativeui.Window{}}
	app := ui.NewTester(func(c *ui.Context) {
		p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, v.reading.surface.Nodes["card"], false)
	}, 620, 420)
	markedChanged := func(document string) bool {
		v.index(document)
		app.Frame()
		var names []string
		for _, el := range app.AccessAll() {
			names = append(names, el.Name)
		}
		return strings.Contains(strings.Join(names, "\n"), "Source changed")
	}

	for _, document := range []string{
		"# Overview\n\nAn overview.\n",
		"# Overview\n\nAn overview.",
		"# Overview\n\nAn overview.\n\n \n",
	} {
		if markedChanged(document) {
			t.Errorf("%q is marked as changed since the view was made", document)
		}
	}
	if !markedChanged("# Overview\n\nAnother overview.\n") {
		t.Error("a changed paragraph is not marked as changed")
	}
}

func TestAViewSavedBeforeTheEndOfDocumentRuleIsNotMarkedChangedUntilItsSectionChanges(t *testing.T) {
	for document, savedHash := range map[string]string{
		"# Overview\n\nAn overview.":                     "fnv1a32:afd7b68d",
		"# Overview\n\nAn overview.\n\n \n":              "fnv1a32:8dc4af77",
		"# Overview\r\n\r\nAn overview.":                 "fnv1a32:11644d29",
		"# Intro\n\nKept.\n\n# Overview\n\nAn overview.": "fnv1a32:afd7b68d",
	} {
		v := &documentView{reading: Reading{surface: &Surface{
			Sources: map[string]Reference{"overview": {Heading: "Overview", Hash: savedHash}},
			Nodes:   map[string]Node{"card": {ID: "card", ComponentID: "card", Sources: []string{"overview"}}},
		}}}
		p := &panel{win: &nativeui.Window{}}
		app := ui.NewTester(func(c *ui.Context) {
			p.drawSources(c, v, workspace.DocumentInfo{Path: "doc.md"}, v.reading.surface.Nodes["card"], false)
		}, 620, 420)
		markedChanged := func(document string) bool {
			v.index(document)
			app.Frame()
			var names []string
			for _, el := range app.AccessAll() {
				names = append(names, el.Name)
			}
			marked := strings.Contains(strings.Join(names, "\n"), "Source changed")
			if listed := slices.Contains(v.changedIDs(), "card"); listed != marked {
				t.Errorf("%q: the source is marked changed: %t, its statement is offered for update: %t", document, marked, listed)
			}
			return marked
		}
		if markedChanged(document) {
			t.Errorf("%q is marked as changed since the view was made", document)
		}
		if !markedChanged(strings.Replace(document, "An overview.", "Another overview.", 1)) {
			t.Errorf("%q with a changed paragraph is not marked as changed", document)
		}
	}
}

func TestDeletingAViewClearsOnlyAnUneditedGenerationRequest(t *testing.T) {
	file := func(request string) genui.ViewEvent {
		const created = `{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":["root = Col([card])","card = Card([Text(\"View\")])"]}}`
		return genui.ViewEvent{ID: "doc", Exists: true, Text: `{"version":3,"request":` + strconv.Quote(request) + `,"language":"openui-lang/1.0","sources":{},"a2ui":[` + created + `]}`}
	}
	for _, test := range []struct {
		name, draft, want string
	}{
		{name: "an unedited request is cleared", want: ""},
		{name: "a reader draft is preserved", draft: "Keep this draft", want: "Keep this draft"},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := &documentView{}
			p := &panel{views: map[string]*documentView{"doc": v}}
			p.updated(file("Old request"))
			if v.request != "Old request" {
				t.Fatalf("loaded request = %q, want the sidecar request", v.request)
			}
			if test.draft != "" {
				v.request = test.draft
				v.requestEdited = true
			}

			p.updated(genui.ViewEvent{ID: "doc"})
			if v.request != test.want {
				t.Errorf("request after deletion = %q, want %q", v.request, test.want)
			}
			if got := v.generationRequest("# Overview", nil).Request; got != test.want {
				t.Errorf("next generation request = %q, want %q", got, test.want)
			}

			if test.draft == "" {
				p.updated(file("New request"))
				if v.request != "New request" {
					t.Errorf("request after a new sidecar arrives = %q, want %q", v.request, "New request")
				}
			}
		})
	}
}

func statementTestFile(t *testing.T, statements []string) string {
	t.Helper()
	components, err := json.Marshal(statements)
	if err != nil {
		t.Fatal(err)
	}
	return `{"version":3,"language":"openui-lang/1.0","sources":{"overview":{"heading":"Overview"}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":` + string(components) + `}}]}`
}

func statementTestPreview(t *testing.T, statements []string) *genui.StatementPreview {
	t.Helper()
	preview := genui.ReadStatementPreview([]byte(statementTestFile(t, statements)))
	if len(preview.Statements) != len(statements) {
		t.Fatalf("preview statements = %v, want %v", preview.Statements, statements)
	}
	return &preview
}

func TestAFullStatementGenerationAppearsProgressivelyAndMatchesTheReopenedSidecar(t *testing.T) {
	v := &documentView{cancel: func() {}}
	p := &panel{views: map[string]*documentView{"doc": v}}
	app := ui.NewTesterWith(func(c *ui.Context) { v.reader().View(c) }, 640, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})

	first := []string{`root = Col([Title("First view", "xl")])`}
	v.progressed(genui.GenerateProgress{Preview: statementTestPreview(t, first)})
	app.Frame()
	if !app.HasText("First view") {
		t.Fatalf("the first complete statement was not drawn: %v", app.Texts())
	}

	final := []string{
		`root = Col([head, body])`,
		`head = Title("First view", "xl")`,
		`body = Text("Streamed body", "xl")`,
	}
	v.progressed(genui.GenerateProgress{Preview: statementTestPreview(t, final)})
	app.Frame()
	if !app.HasText("Streamed body") {
		t.Fatalf("the next complete statement was not added: %v", app.Texts())
	}
	streamed := app.Image()

	p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: statementTestFile(t, final)})
	app.Frame()
	if !app.HasText("First view") || !app.HasText("Streamed body") {
		t.Fatalf("the reopened sidecar has different content: %v", app.Texts())
	}
	if changed := changedPixels(streamed, app.Image()); changed != 0 {
		t.Errorf("reopening the sidecar changed %d pixels; optional-property degradation differed during streaming", changed)
	}
}

func TestAFailedOrCancelledFullStatementPreviewRestoresTheOriginalReader(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "cancelled", err: genui.ErrCancelled},
		{name: "failed validation", err: errors.New("invalid candidate")},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := []string{
				`$months = 2`,
				`root = Col([plan])`,
				`plan = Card([Slider($months, 0, 10, 1, "Months", "mo"), Text("Total " + $months * 150)])`,
			}
			v := &documentView{}
			p := &panel{views: map[string]*documentView{"doc": v}}
			p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: statementTestFile(t, original)})
			app := ui.NewTesterWith(func(c *ui.Context) { v.reader().View(c) }, 600, 500, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
			app.Frame()
			slider, ok := app.Access("Months")
			if !ok {
				t.Fatal("the original view has no Months slider")
			}
			box := slider.Bounds
			app.Press(box.X+box.W*0.2, box.Y+box.H/2)
			app.Move(box.X+box.W*0.8, box.Y+box.H/2)
			app.Release(box.X+box.W*0.8, box.Y+box.H/2)
			app.Frame()
			if !app.HasText("Total 1200") {
				t.Fatalf("the original reader did not change to 8 months: %v", app.Texts())
			}

			v.cancel = func() {}
			v.lastIDs = nil
			v.progressed(genui.GenerateProgress{Preview: statementTestPreview(t, []string{`root = Col([Text("Provisional replacement")])`})})
			app.Frame()
			if !app.HasText("Provisional replacement") || app.HasText("Total 1200") {
				t.Fatalf("the provisional view was not shown: %v", app.Texts())
			}

			v.finished(test.err)
			app.Frame()
			if app.HasText("Provisional replacement") || !app.HasText("Total 1200") {
				t.Errorf("the original reader was not restored: %v", app.Texts())
			}
			if slider, _ := app.Access("Months"); slider.Now != 8 {
				t.Errorf("the original reader's slider = %v, want the preserved value 8", slider.Now)
			}
			if v.preview != nil || v.stream != nil {
				t.Error("the abandoned preview is still retained")
			}
		})
	}
}

func TestAStatementPreviewDoesNotReplaceTheSingleCardUpdatePlaceholder(t *testing.T) {
	v := &documentView{lastIDs: []string{"risks"}}
	p := &panel{views: map[string]*documentView{"doc": v}}
	p.updated(genui.ViewEvent{ID: "doc", Exists: true, Text: statementTestFile(t, []string{
		`root = Col([plan, risks])`,
		`plan = Card([Text("Plan stays readable")])`,
		`risks = Card([Text("Old risks")])`,
	})})
	v.reading.Replacing = v.lastIDs
	v.progressed(genui.GenerateProgress{Preview: statementTestPreview(t, []string{`root = Col([Text("Wrong full preview")])`})})
	app := ui.NewTester(v.reading.View, 600, 500)
	app.Frame()
	if v.preview != nil || !app.HasText("Updating card…") || !app.HasText("Plan stays readable") || app.HasText("Wrong full preview") {
		t.Fatalf("single-card update was replaced by a full preview: %v", app.Texts())
	}
	v.finished(errors.New("cancelled"))
	app.Frame()
	if app.HasText("Updating card…") || !app.HasText("Old risks") {
		t.Errorf("the updated card was not restored: %v", app.Texts())
	}
}

func TestGenerationProgressSaysEachStepOnce(t *testing.T) {
	v := &documentView{}
	for _, step := range []genui.GenerateProgress{
		{Phase: "starting", Text: "Starting the agent"},
		{Phase: "thinking", Text: "Thinking"},
		{Phase: "writing", Text: "Writing the view"},
		{Phase: "checking", Text: "Checking the view"},
	} {
		v.progressed(step)
		if v.message != step.Text {
			t.Errorf("status during %q = %q, want %q", step.Phase, v.message, step.Text)
		}
	}
}
