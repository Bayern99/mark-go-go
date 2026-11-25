package nativegenui

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

//go:embed catalog.txt
var catalog string

type documentView struct {
	reading                            Reading
	preview                            *Reading
	stream                             *statementStream
	snapshot                           genui.ViewEvent
	loaded, settings, requestEdited    bool
	request, message, note, noteTarget string
	sourceText, imagesFor              string
	excerptSource                      func(string, workspace.Section) string
	excerpts                           map[sourceExcerptKey]string
	sections                           map[headingKey]workspace.Section
	notes                              []workspace.Note
	failure                            *genui.GenerateFailure
	lastIDs                            []string
	cancel                             context.CancelFunc
	previewSource                      sourcePreviewState
	previewTargets                     []sourcePreviewTarget
	previewAnchors                     map[string]*ui.Handle
}
type headingKey struct {
	name       string
	occurrence int
}

type sourcePreviewState struct {
	key                    string
	hoveredKey, focusedKey string
	open, dismissed        bool
	byPointer, overPreview bool
	hideAt                 time.Time
}

type sourceExcerptKey struct {
	source     string
	start, end int
}

type sourcePreviewTarget struct {
	anchor     *ui.Handle
	key, label string
	ref        Reference
	section    workspace.Section
	found      bool
}

const sourceExcerptLimit = 600

type panel struct {
	win             *nativeui.Window
	host            *workspace.Service
	watch, unwatch  func(context.Context, string) error
	views           map[string]*documentView
	closed          bool
	settings        workspace.Settings
	settingsMessage string

	// Seams for the settings page: production points them at the workspace
	// service, tests at fakes that never start an agent or a terminal.
	save             func(workspace.SettingsPatch) (workspace.Settings, error)
	probe            func(string) (genui.ProbeResult, error)
	refresh          func(string) (genui.ModelCatalog, error)
	confirmSelection func(string, workspace.GenUIBackendPatch) (genui.ModelCatalog, workspace.Settings, error)
	signIn           func(string) error
	runOnUI          func(func())

	edits    map[string]*backendEdit
	backends map[string]*backendStatus
}

func Install(host *workspace.Service) func(*nativeui.Window) nativeui.Panels {
	return func(win *nativeui.Window) nativeui.Panels {
		p := &panel{win: win, host: host, views: make(map[string]*documentView), watch: host.WatchView, unwatch: host.UnwatchView}
		p.settings, _ = host.GetSettings()
		p.save = host.SetSettings
		p.probe = host.ProbeGenUI
		p.refresh = host.RefreshGenUIModels
		p.confirmSelection = host.ConfirmGenUISelection
		p.signIn = host.OpenGenUILogin
		workspace.SettingsChanged.Listen(win.Native(), func(next workspace.Settings) {
			p.settings = next
		})
		win.Native().OnFocus(p.probeOnFocus)
		genui.ViewChanged.Listen(win.Native(), p.updated)
		return nativeui.Panels{GenUI: p.draw, Settings: []nativeui.SettingsPage{p.settingsPage()}, Changed: p.changed, Closed: p.close}
	}
}
func (p *panel) close() {
	p.closed = true
	for _, v := range p.views {
		if v.cancel != nil {
			v.cancel()
		}
	}
}
func (p *panel) changed() {
	if !p.win.PluginEnabled("genui") {
		p.suspend()
		return
	}
	open := map[string]bool{}
	for _, d := range p.win.Workspace().Documents {
		open[d.ID] = true
	}
	for id, v := range p.views {
		if !open[id] {
			if v.cancel != nil {
				v.cancel()
			}
			delete(p.views, id)
		}
	}
}

// suspend drops every watched view while GenUI is off: a generation in flight
// is cancelled before it can write, and its listener stops. The saved view
// stays on disk, so turning GenUI back on reads it again.
func (p *panel) suspend() {
	for id, v := range p.views {
		if v.cancel != nil {
			v.cancel()
		}
		if p.unwatch != nil {
			_ = p.unwatch(p.win.Context(), id)
		}
		delete(p.views, id)
	}
}

// viewFor is the document's view, opened and watched on first use. It reports
// nil while GenUI is off, so a document never reaches the reader then.
func (p *panel) viewFor(d workspace.DocumentInfo) *documentView {
	if !p.win.PluginEnabled("genui") {
		return nil
	}
	v := p.views[d.ID]
	if v != nil {
		return v
	}
	v = &documentView{}
	p.views[d.ID] = v
	if err := p.watch(p.win.Context(), d.ID); err != nil {
		v.loaded = true
		v.snapshot.Error = err.Error()
	}
	return v
}

func (p *panel) updated(ev genui.ViewEvent) {
	v := p.views[ev.ID]
	if v == nil {
		return
	}
	v.loaded = true
	v.snapshot = ev
	v.discardPreview()
	v.reading.Load(nil)
	if !ev.Exists && !v.requestEdited {
		v.request = ""
	}
	if ev.Exists && ev.Error == "" {
		s, err := Open(ev.Text)
		if err != nil {
			v.snapshot.Error = err.Error()
		} else {
			v.reading.Load(s)
			if !v.requestEdited {
				v.request = s.Request
			}
		}
	}
}
func (p *panel) draw(c *ui.Context) {
	d := p.win.Active()
	v := p.viewFor(d)
	if v == nil {
		return
	}
	if p.win.Source() == nil {
		portrait.Empty(c, portrait.EmptySpec{Title: "Loading document…"})
		return
	}
	c = c.Key(d.ID)
	raw := p.win.Source().Source()
	v.index(raw)
	busy := v.cancel != nil || d.Generating
	// The left edge matches the page bar's breadcrumb, so the request form
	// lines up with the document it belongs to.
	ui.Column(c).Fill().MinHeight(0).Gap(8).Padding(8, 12, 12, 12).Children(func() {
		p.drawRequestToggle(c, v)
		if v.settings {
			p.drawSettings(c, v, d, busy)
		}
		p.drawGenerationStatus(c, v, d, busy)
		p.drawView(c, v, d, raw, busy)
	})
}

// drawRequestToggle is the panel's one control row. The caret and the
// selected look say that the button opens the request form.
func (p *panel) drawRequestToggle(c *ui.Context, v *documentView) {
	caret := portrait.IconCaretDown
	if v.settings {
		caret = portrait.IconCaretUp
	}
	ui.Row(c).FillWidth().Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
		portrait.Type.Meta.Apply(ui.Text(c, p.backendSummary())).TextColor(portrait.Colors.Ink3).
			Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
		toggle := portrait.ButtonWith(c, "Request", "Generation request", portrait.ButtonSpec{
			Look: portrait.Quiet, Selected: v.settings, Trailing: caret,
		}).Checked(v.settings).Expanded(v.settings)
		if toggle.Clicked() {
			v.settings = !v.settings
		}
		if portrait.QuietButton(c, "Configure…", "Configure GenUI").Clicked() {
			p.win.OpenSettings("genui")
		}
	})
}

// backendSummary is the panel's read-only line: which backend makes the view
// and which model it will ask for. The settings page is the only editor.
func (p *panel) backendSummary() string {
	id, backend := p.settings.GenUI.Active()
	name := id
	if b, ok := genui.BackendByID(id); ok {
		name = b.Name
	}
	model := backend.UsableModel()
	if model == "" {
		model = "Agent default"
	}
	return name + " · " + model
}

// drawGenerationStatus reports the running or failed generation above the
// view, so the reader never has to look for what happened.
func (p *panel) drawGenerationStatus(c *ui.Context, v *documentView, d workspace.DocumentInfo, busy bool) {
	if v.failure != nil {
		var retry []portrait.AlertAction
		if !busy && d.Path != "" {
			retry = append(retry, portrait.AlertAction{Label: "Retry", OnAction: func() { p.generate(v, v.lastIDs) }})
		}
		portrait.Alert(c, portrait.Tones.Danger, "Generation failed", strings.TrimSpace(v.failure.Message+" "+v.failure.Hint), retry...)
		if v.failure.Detail != "" {
			portrait.OutputBlock(c, "Generation details", v.failure.Detail)
		}
	}
	if v.message != "" {
		portrait.Type.UI.Apply(ui.Text(c, v.message)).TextColor(portrait.Colors.Ink2).Role(ui.RoleStatus)
	}
	if v.cancel != nil {
		cancel := portrait.Button(c, "Cancel generation", "Cancel generation").AlignSelf(ui.Start)
		if cancel.Clicked() {
			v.cancel()
		}
	}
}

// drawView draws the reader, or what stands in for it before one exists. A
// failure is already drawn above, so it never shares the page with the empty
// state, whose next step would contradict it.
func (p *panel) drawView(c *ui.Context, v *documentView, d workspace.DocumentInfo, raw string, busy bool) {
	switch {
	case !v.loaded:
		loading(c, "Loading the GenUI view", "Reading the view file for this document.")
	case v.snapshot.Error != "":
		portrait.Alert(c, portrait.Tones.Danger, "GenUI view", v.snapshot.Error)
	case v.cancel != nil && v.reader().surface == nil:
		loading(c, "Generating view…", "Your agent is laying out this document.")
	case v.failure != nil && v.reader().surface == nil:
		return
	case v.reader().surface == nil:
		ui.Column(c).FillWidth().Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			spec := portrait.EmptySpec{
				Icon:        portrait.IconSparkle,
				Title:       "No view yet",
				Description: "Generate a reading view from this document. The Markdown stays the source.",
			}
			if !v.settings {
				spec.Action = func(c *ui.Context) ui.Element {
					b := portrait.AccentButton(c, "Write a request", "Write a request").Margin(6, 0, 0, 0)
					if b.Clicked() {
						v.settings = true
					}
					return b
				}
			}
			portrait.Empty(c, spec).MaxWidth(portrait.Shell.SidebarWidth + portrait.Shell.Documents/4)
		})
	default:
		reader := v.reader()
		reader.CanSelect = func(n Node) bool { return len(v.matches(n)) > 0 }
		if v.imagesFor != d.Path || reader.LoadImage == nil {
			directory := ""
			if d.Path != "" {
				directory = filepath.Dir(d.Path)
			}
			v.imagesFor, reader.LoadImage = d.Path, DocumentImages(raw, directory)
			reader.images.Clear()
		}
		reader.SelectItem = func(n Node, text string) {
			ss := v.matches(n)
			if len(ss) == 0 {
				return
			}
			start, end := ss[0].Start, ss[0].End
			if needle := strings.TrimSpace(text); needle != "" {
				for _, s := range ss {
					a, b := byteAt(raw, s.Start), byteAt(raw, s.End)
					if i := strings.Index(raw[a:b], needle); i >= 0 {
						start = s.Start + utf16Length(raw[a:a+i])
						end = start + utf16Length(needle)
						break
					}
				}
			}
			highlightCard(reader, n)
			p.win.Reveal(start)
			_ = p.win.Source().SelectUTF16(start, end)
		}
		reader.Sources = func(c *ui.Context, n Node) { p.drawSources(c, v, d, n, busy) }
		v.drawReading(c)
	}
}

func (v *documentView) index(raw string) {
	if v.sections != nil && raw == v.sourceText {
		return
	}
	v.sourceText = raw
	v.excerpts = nil
	v.sections = map[headingKey]workspace.Section{}
	for _, s := range workspace.ExtractSections(raw) {
		v.sections[headingKey{workspace.NormalizeHeadingText(s.Heading), s.Occurrence}] = s
	}
	v.notes = workspace.ExtractNotes(raw)
	v.imagesFor = ""
}

func (v *documentView) reader() *Reading {
	if v.preview != nil && v.preview.surface != nil {
		return v.preview
	}
	return &v.reading
}

func (v *documentView) discardPreview() {
	v.preview, v.stream = nil, nil
}

func (v *documentView) progressed(step genui.GenerateProgress) {
	v.message = step.Text
	if step.Preview == nil || len(v.lastIDs) != 0 {
		return
	}
	if v.stream == nil {
		stream, err := newStatementStream()
		if err != nil {
			return
		}
		v.stream = stream
	}
	surface, err := v.stream.Push(*step.Preview)
	if err != nil || surface.tree == nil {
		return
	}
	if v.preview == nil {
		v.preview = &Reading{}
	}
	v.preview.Load(surface)
}

func statementSource(surface *Surface, sections map[headingKey]workspace.Section, id string) (Reference, workspace.Section, bool) {
	ref := surface.Sources[id]
	s, ok := sections[headingKey{workspace.NormalizeHeadingText(ref.Heading), max(1, ref.Occurrence)}]
	return ref, s, ok
}

func (v *documentView) source(id string) (Reference, workspace.Section, bool) {
	return statementSource(v.reader().surface, v.sections, id)
}

func (v *documentView) storedSource(id string) (Reference, workspace.Section, bool) {
	return statementSource(v.reading.surface, v.sections, id)
}
func (v *documentView) matches(n Node) []workspace.Section {
	var matches []workspace.Section
	for _, id := range n.Sources {
		if _, s, ok := v.source(id); ok {
			matches = append(matches, s)
		}
	}
	return matches
}
func (v *documentView) changedIDs() []string {
	var ids []string
	if v.reading.surface != nil {
		seen := map[string]bool{}
		add := func(id string, sources []string) {
			if id == "" || seen[id] {
				return
			}
			for _, source := range sources {
				r, s, ok := v.storedSource(source)
				if !ok || (r.Hash != "" && !s.UnchangedSince(r.Hash)) {
					ids = append(ids, id)
					seen[id] = true
					break
				}
			}
		}
		for runtimeID, n := range v.reading.surface.Nodes {
			id := n.ComponentID
			if id == "" {
				id = n.ID
			}
			if id == "" {
				id = runtimeID
			}
			add(id, n.Sources)
		}
		for _, use := range v.reading.surface.sourceUses {
			add(use.Statement, use.Strings)
		}
	}
	sort.Strings(ids)
	return ids
}

func (p *panel) drawSettings(c *ui.Context, v *documentView, d workspace.DocumentInfo, busy bool) {
	portrait.TextArea(c, &v.request, portrait.AreaSpec{Label: "Generation request", Placeholder: "Generation request", Rows: 2, Disabled: busy}).OnChange(func() { v.requestEdited = true })
	if d.Path == "" {
		portrait.Caption(c, "Save the document first to generate a view.")
	}
	ui.Row(c).Gap(portrait.Form.Gap).Wrap().Children(func() {
		if portrait.AccentButton(c, "Generate", "Generate").Disabled(busy || d.Path == "").Clicked() {
			p.generate(v, nil)
		}
		if ids := v.changedIDs(); len(ids) > 0 && portrait.Button(c, fmt.Sprintf("Regenerate changed (%d)", len(ids)), fmt.Sprintf("Regenerate changed (%d)", len(ids))).Disabled(busy || d.Path == "").Clicked() {
			p.generate(v, ids)
		}
		if v.snapshot.CanUndo && portrait.QuietButton(c, "Restore previous view", "Restore previous view").Disabled(busy).Clicked() {
			v.message = message(p.host.UndoView(p.win.Context(), d.ID))
			if v.message == "" {
				v.requestEdited = false
			}
		}
	})
}

func (p *panel) drawSources(c *ui.Context, v *documentView, d workspace.DocumentInfo, n Node, busy bool) {
	ss := v.matches(n)
	var notes []string
	for _, note := range v.notes {
		if slices.ContainsFunc(ss, func(s workspace.Section) bool { return note.Start >= s.Start && note.End <= s.End }) {
			notes = append(notes, note.Text)
		}
	}
	if len(notes) > 0 {
		ui.Column(c).Gap(6).Margin(2, 0, 0, 0).Role(ui.RoleList).Label("Notes").Children(func() {
			for i, note := range notes {
				portrait.Type.Meta.Apply(ui.Text(c.Key(i), note)).Padding(4, 8).Radius(portrait.Radii.S).Background(portrait.Colors.WarningSoft).
					Border(0.5, portrait.Colors.Line).Selectable()
			}
		})
	}
	status, missing := "", false
	ui.Row(c).Wrap().GapX(10).GapY(6).AlignItems(ui.Center).Margin(2, 0, 0, 0).Padding(8, 0, 0, 0).Shadow(0, -0.5, 0, 0, portrait.Colors.LineStrong).Children(func() {
		ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
			portrait.Type.Meta.Apply(ui.Text(c, "From")).TextColor(portrait.Colors.Ink3)
			for i, id := range n.Sources {
				ref, s, ok := v.source(id)
				label := ref.Heading
				if label == "" {
					label = id
				}
				if ref.Occurrence > 1 {
					label = fmt.Sprintf("%s (%d)", label, ref.Occurrence)
				}
				var heading ui.Element
				anchor := ui.Box(c.Key(i)).Children(func() {
					heading = sourceHeading(c, label, !ok)
				})
				if !ok {
					anchor.Focusable().Role(ui.RoleStatus).Label(label + ", source missing").Radius(portrait.Radii.S)
				}
				key := fmt.Sprintf("%s/%d/%s", n.ID, i, id)
				if v.previewAnchors == nil {
					v.previewAnchors = map[string]*ui.Handle{}
				}
				handle := v.previewAnchors[key]
				if handle == nil {
					handle = new(ui.Handle)
					v.previewAnchors[key] = handle
				}
				anchor.Bind(handle)
				v.previewTargets = append(v.previewTargets, sourcePreviewTarget{
					anchor: handle, key: key, label: label,
					ref: ref, section: s, found: ok,
				})
				if heading.Clicked() {
					highlightCard(v.reader(), n)
					p.win.Reveal(s.Start)
					_ = p.win.Source().SelectUTF16(s.Start, s.End)
				}
				if !ok {
					status, missing = "Source missing", true
				} else if ref.Hash != "" && !s.UnchangedSince(ref.Hash) && status == "" {
					status = "Source changed"
				}
			}
		})
		if status != "" {
			sourceStatus(c, status, missing)
		}
		if sourceButton(c, "Add note", d.ReadOnly || len(ss) == 0).Clicked() {
			v.noteTarget = n.ID
			v.note = ""
		}
		if sourceButton(c, "Update card", busy || d.Path == "" || n.ComponentID == "" && v.reader().surface.Session != nil).Clicked() {
			componentID := n.ComponentID
			if componentID == "" {
				componentID = n.ID
			}
			p.generate(v, []string{componentID})
		}
	})
	if v.noteTarget != n.ID {
		return
	}
	note := portrait.TextArea(c, &v.note, portrait.AreaSpec{Label: "Reader note", Placeholder: "Reader note", Rows: 2}).AutoFocus()
	composing := note.Composing()
	saveShortcut := note.Shortcut(ui.Cmd, ui.KeyEnter) && !composing
	cancelShortcut := note.Shortcut(0, ui.KeyEscape) && !composing
	enabled := len(ss) > 0 && !d.ReadOnly && strings.TrimSpace(v.note) != ""
	ui.Row(c).FillWidth().Justify(ui.End).Gap(portrait.Form.Gap).Children(func() {
		if portrait.QuietButton(c, "Cancel note", "Cancel note").Clicked() || cancelShortcut {
			v.noteTarget = ""
			v.note = ""
		}
		if (portrait.AccentButton(c, "Save note", "Save note").Disabled(!enabled).Clicked() || saveShortcut) && enabled {
			v.message = message(p.addNote(ss[0], v.note))
			if v.message == "" {
				v.noteTarget = ""
				v.note = ""
			}
		}
	})
}

func highlightCard(reader *Reading, n Node) {
	id := n.ComponentID
	if id == "" {
		id = n.ID
	}
	reader.Highlight(id)
}

func (v *documentView) drawReading(c *ui.Context) {
	v.previewTargets = v.previewTargets[:0]
	v.reader().View(c)
	v.drawSourcePreview(c, v.previewTargets)
}

func (v *documentView) drawSourcePreview(c *ui.Context, targets []sourcePreviewTarget) {
	state := &v.previewSource
	var hovered, focused *sourcePreviewTarget
	for i := range targets {
		target := &targets[i]
		anchor := c.Resolve(*target.anchor)
		if !anchor.Valid() {
			continue
		}
		if hovered == nil && anchor.Hovered() && c.Preferences().Hover() {
			hovered = target
		}
		if focused == nil && anchor.FocusWithin() {
			focused = target
		}
	}
	hoveredKey, focusedKey := "", ""
	if hovered != nil {
		hoveredKey = hovered.key
	}
	if focused != nil {
		focusedKey = focused.key
	}
	newHover := hoveredKey != "" && hoveredKey != state.hoveredKey
	newFocus := focusedKey != "" && focusedKey != state.focusedKey
	state.hoveredKey, state.focusedKey = hoveredKey, focusedKey
	if state.dismissed && !newHover && !newFocus {
		state.open, state.overPreview = false, false
		state.hideAt = time.Time{}
		return
	}
	state.dismissed = false

	target := hovered
	if target == nil {
		target = focused
	}
	if target != nil {
		if state.key != target.key {
			state.byPointer = target == hovered && target != focused
		}
		state.key, state.open = target.key, true
		state.hideAt = time.Time{}
	} else if state.open && state.overPreview {
		target = sourcePreviewTargetForKey(targets, state.key)
		state.hideAt = time.Time{}
	} else if state.open && state.byPointer {
		target = sourcePreviewTargetForKey(targets, state.key)
		if state.hideAt.IsZero() {
			state.hideAt = c.Now().Add(300 * time.Millisecond)
		}
		if wait := state.hideAt.Sub(c.Now()); wait > 0 {
			c.After(wait)
		} else {
			target = nil
		}
	}
	if target == nil {
		state.key, state.open, state.overPreview = "", false, false
		state.hideAt = time.Time{}
		return
	}

	windowWidth, windowHeight := c.Size()
	anchor := c.Resolve(*target.anchor)
	panel := portrait.PopoverWithSpec(c, anchor, &state.open, portrait.PopoverSpec{
		Width:     min(portrait.Form.PopoverWide, max(0, windowWidth-2*portrait.Shell.PageInset)),
		MaxHeight: max(0, windowHeight-2*portrait.Shell.PageInset),
		Title:     "Source preview: " + target.label,
	}, func() {
		switch {
		case !target.found:
			portrait.Alert(c, portrait.Tones.Danger, "Source missing", "This section is no longer in the document.")
		case target.ref.Hash != "" && !target.section.UnchangedSince(target.ref.Hash):
			portrait.Alert(c, portrait.Tones.Warning, "Source changed", "This section changed after the view was generated. Open the source to read its current text.")
		default:
			if target.ref.Hash == "" {
				portrait.Type.Meta.Apply(ui.Text(c, "Source version at generation time cannot be verified.")).TextColor(portrait.Colors.Ink3)
			}
			portrait.Type.Small.Apply(ui.Text(c, v.excerpt(target.key, target.section))).TextColor(portrait.Colors.Ink2).Selectable()
		}
	})
	state.overPreview = panel.Hovered()
	panel.KeepFocus().Label("Source preview: " + target.label)
	if !state.open {
		state.dismissed = true
		state.overPreview = false
	}
}

func sourcePreviewTargetForKey(targets []sourcePreviewTarget, key string) *sourcePreviewTarget {
	for i := range targets {
		if targets[i].key == key {
			return &targets[i]
		}
	}
	return nil
}

func (v *documentView) excerpt(source string, section workspace.Section) string {
	key := sourceExcerptKey{source: source, start: section.Start, end: section.End}
	if excerpt, ok := v.excerpts[key]; ok {
		return excerpt
	}
	excerpt := ""
	if v.excerptSource != nil {
		excerpt = v.excerptSource(v.sourceText, section)
	} else {
		excerpt = sourceExcerpt(v.sourceText, section)
	}
	if v.excerpts == nil {
		v.excerpts = map[sourceExcerptKey]string{}
	}
	v.excerpts[key] = excerpt
	return excerpt
}

func sourceExcerpt(raw string, section workspace.Section) string {
	excerpt := strings.TrimSpace(raw[byteAt(raw, section.Start):byteAt(raw, section.End)])
	runes := 0
	for i := range excerpt {
		if runes == sourceExcerptLimit {
			return strings.TrimSpace(excerpt[:i]) + "…"
		}
		runes++
	}
	return excerpt
}

func sourceHeading(c *ui.Context, label string, disabled bool) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Disabled(disabled).DimDisabled(false).Radius(portrait.Radii.S)
	ink, line := portrait.Colors.Accent, portrait.Colors.LineStrong
	if disabled {
		ink = portrait.Colors.Ink3
	} else if b.Hovered() {
		b.Background(portrait.Colors.AccentSoft)
		line = portrait.Colors.Accent
	}
	b.Children(func() {
		portrait.Type.Meta.Apply(ui.Text(c, label)).TextColor(ink).Underline().UnderlineOffset(2).DecorationColor(line)
	})
	return b
}

func sourceStatus(c *ui.Context, text string, missing bool) {
	tone := portrait.Tones.Warning
	if missing {
		tone = portrait.Tones.Danger
	}
	portrait.Badge(c, text, tone).Role(ui.RoleStatus)
}

type sourceHover struct{}
type sourcePress struct{}

func sourceButton(c *ui.Context, label string, disabled bool) ui.Element {
	g := portrait.GenUI
	b := ui.ButtonBase(c.Key(label)).Label(label).Disabled(disabled).DimDisabled(false).Padding(2, 8).Radius(portrait.Radii.M).AlignItems(ui.Center)
	ease := portrait.Motion.EaseOut.Easing()
	hovered, pressed := b.Hovered() && !disabled, b.Pressed() && !disabled
	h := b.AnimateTo(sourceHover{}, flag(hovered && !pressed), ui.Animation{Duration: portrait.Motion.Press, Ease: ease, NonMotion: true})
	pr := b.AnimateTo(sourcePress{}, flag(pressed), ui.Animation{Duration: portrait.Motion.Press, Ease: ease})
	g.Source.Mix(g.SourceHover, h).Mix(g.SourcePress, pr).Apply(b)
	scale := 1 - 0.03*pr
	b.TransformOrigin(0.5, 0.5).Translate(0, 0.5*pr).Scale(scale, scale)
	ink := portrait.Colors.Accent
	if disabled {
		ink = portrait.Colors.Ink3
	}
	b.Children(func() { portrait.Type.Meta.Apply(ui.Text(c, label)).TextColor(ink) })
	return b
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func loading(c *ui.Context, title, detail string) {
	ui.Column(c.Key("loading")).Gap(12).Padding(72, 0, 24, 0).FillWidth().MaxWidth(520).AlignSelf(ui.Center).Busy(true).Role(ui.RoleStatus).Label(title).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			portrait.Type.Title.In(portrait.FontDisplay).Apply(ui.Text(c, title)).FontWeight(600).TextColor(portrait.Colors.Ink)
			portrait.Type.Small.Apply(ui.Text(c, detail)).TextColor(portrait.Colors.Ink2)
		})
		viewSkeleton(c)
	})
}

func activeDefaults(settings workspace.Settings) workspace.GenUIBackend {
	defaults := workspace.DefaultSettings()
	id, _ := settings.GenUI.Active()
	return defaults.GenUI.For(id)
}

func message(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
func utf16Length(s string) int {
	length := 0
	for _, r := range s {
		length += utf16.RuneLen(r)
	}
	return length
}
func byteAt(s string, offset int) int {
	for i, r := range s {
		if offset <= 0 {
			return i
		}
		offset -= utf16.RuneLen(r)
	}
	return len(s)
}

var noteClose = regexp.MustCompile(`--(\\*)(!?)>`)

func (p *panel) addNote(s workspace.Section, note string) error {
	d := p.win.Source()
	if d == nil {
		return errors.New("Document closed")
	}
	raw := d.Source()
	at := byteAt(raw, s.End)
	at = len(strings.TrimRight(raw[:at], "\r\n"))
	newline := "\n"
	if strings.Contains(raw, "\r\n") {
		newline = "\r\n"
	}
	insert := newline + newline + "<!-- note: " + noteClose.ReplaceAllString(strings.TrimSpace(note), `--\${1}${2}>`) + " -->"
	if err := d.Apply(d.Version(), at, at, "", insert); err != nil {
		return err
	}
	return p.win.Publish()
}
func (v *documentView) generationRequest(text string, ids []string) genui.GenerateRequest {
	mode := "full"
	if len(ids) > 0 {
		mode = "update"
	}
	req := genui.GenerateRequest{Text: text, Mode: mode, Format: genuikit.Format(), Language: genui.StatementLanguage, Request: v.request, ReplaceIDs: ids, Sections: workspace.GenerationSections(text)}
	if surface := v.reading.surface; mode == "update" && surface != nil && surface.Session == nil {
		req.Format, req.Language = catalog, ""
	}
	return req
}

func (p *panel) generate(v *documentView, ids []string) {
	if v.cancel != nil {
		return
	}
	d := p.win.Active()
	text := p.win.Source().Source()
	ctx, cancel := context.WithCancel(p.win.Context())
	v.cancel = cancel
	v.failure = nil
	v.lastIDs = slices.Clone(ids)
	v.discardPreview()
	v.reading.Replacing = v.lastIDs
	v.message = "Starting generation…"
	req := v.generationRequest(text, ids)
	p.win.Notify(portrait.ToastSpec{ID: "generation:" + d.ID, Tone: portrait.Tones.Info, Title: "Generating…", Timeout: -1})
	done := p.host.StartGeneration(ctx, d.ID, req, func(progress genui.GenerateProgress) error {
		p.win.Native().Update(func() {
			if !p.closed && p.views[d.ID] == v {
				v.progressed(progress)
			}
		})
		return ctx.Err()
	})
	go func() {
		defer cancel()
		err := <-done
		p.win.Native().Update(func() {
			if p.closed {
				return
			}
			if p.views[d.ID] == v {
				v.finished(err)
			}
			p.win.Notify(generationOutcomeToast(d.ID, err))
		})
	}()
}

func generationOutcomeToast(docID string, err error) portrait.ToastSpec {
	id := "generation:" + docID
	switch {
	case err == nil:
		return portrait.ToastSpec{ID: id, Tone: portrait.Tones.Success, Title: "Generated"}
	case errors.Is(err, context.Canceled) || genui.FailureOf(err).Kind == genui.FailCancelled:
		return portrait.ToastSpec{ID: id, Tone: portrait.Tones.Neutral, Title: "Generation canceled"}
	default:
		return portrait.ToastSpec{ID: id, Tone: portrait.Tones.Danger, Title: "Generation failed"}
	}
}
func (v *documentView) finished(err error) {
	v.cancel = nil
	v.reading.Replacing = nil
	v.message = ""
	if err != nil {
		v.discardPreview()
		f := genui.FailureOf(err)
		v.failure = &f
		return
	}
	v.requestEdited = false
	v.message = "View generated."
	v.reader().Halo()
}
