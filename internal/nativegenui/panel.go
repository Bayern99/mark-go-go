package nativegenui

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

//go:embed catalog.txt
var catalog string

type documentView struct {
	reading                                                        Reading
	preview                                                        *Reading
	stream                                                         *statementStream
	snapshot                                                       genui.ViewEvent
	loaded, requestEdited                                          bool
	options, failureOpen                                           bool
	focusCancel, focusRequest                                      bool
	request, message, note, noteTarget, noteError, focusNoteTarget string
	pane, requestInput                                             ui.Handle
	sourceText, imagesFor                                          string
	excerptSource                                                  func(string, workspace.Section) string
	openSource                                                     func(workspace.Section)
	excerpts                                                       map[sourceExcerptKey]string
	sections                                                       map[headingKey]workspace.Section
	notes                                                          []workspace.Note
	failure                                                        *genui.GenerateFailure
	lastIDs                                                        []string
	cancel                                                         context.CancelFunc
	previewSource                                                  sourcePreviewState
	previewTargets                                                 []sourcePreviewTarget
	previewAnchors                                                 map[string]*ui.Handle
}
type headingKey struct {
	name       string
	occurrence int
}

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
	p.drawReader(c, v, d, raw, busy)
}

func (p *panel) drawReader(c *ui.Context, v *documentView, d workspace.DocumentInfo, raw string, busy bool) {
	ui.Column(c).Fill().MinHeight(0).Bind(&v.pane).Children(func() {
		p.drawView(c, v, d, raw, busy)
		p.drawCommandDock(c, v, d, busy)
	})
}

func (p *panel) drawCommandDock(c *ui.Context, v *documentView, d workspace.DocumentInfo, busy bool) {
	ui.Row(c).Absolute().Left(portrait.Shell.PageInset).Right(portrait.Shell.PageInset).Bottom(portrait.Shell.PageInset).
		Justify(ui.Center).MinWidth(0).Children(func() {
		pill := ui.Row(c).FillWidth().MaxWidth(portrait.GenUI.CommandWidth).MinWidth(0).Gap(portrait.Form.Gap).
			Padding(portrait.Form.PopoverPad, portrait.Form.PadX).Radius(portrait.Radii.Pill).AlignItems(ui.End)
		portrait.Materials.Pop.Apply(pill)
		pill.Children(func() {
			switch {
			case busy || v.cancel != nil:
				p.drawBusyPill(c, v, busy)
			case v.failure != nil:
				p.drawFailurePill(c, v, d, pill)
			default:
				p.drawComposePill(c, v, d)
				p.drawCommandOptions(c, v, d, pill, false)
			}
		})
	})
}

func (p *panel) drawComposePill(c *ui.Context, v *documentView, d workspace.DocumentInfo) {
	width := v.requestInput.Bounds(c).W
	lines := generationRequestLines(c, v.request, width)
	height := float32(lines) * portrait.Type.UI.Leading
	generateShortcut := false
	ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(max(portrait.Sizes.Compact, height)).AlignItems(ui.Center).Children(func() {
		request := portrait.Type.UI.Apply(ui.TextAreaBase(c.Key("request"), &v.request)).
			FocusRing(false).Grow(1).Basis(0).MinWidth(0).Height(height).Bind(&v.requestInput).
			TextColor(portrait.Colors.Ink).Label("Generation request").Placeholder("Ask or reshape this view…")
		request.OnChange(func() { v.requestEdited = true })
		if v.focusRequest {
			request.Focus()
			v.focusRequest = false
		}
		generateShortcut = request.Shortcut(ui.Cmd, ui.KeyEnter) && !request.Composing()
	})
	canGenerate := d.Path != ""
	generate := portrait.ButtonWith(c, "Generate", "Generate", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Compact}).Disabled(!canGenerate)
	portrait.Tooltip(c, generate, "Generate", "⌘↩")
	if (generate.Clicked() || generateShortcut) && canGenerate {
		v.focusCancel = true
		p.generate(v, nil)
	}
}

func generationRequestLines(c *ui.Context, request string, width float32) int {
	lines := strings.Count(request, "\n") + 1
	if width > 0 {
		_, measured := c.MeasureText(width, ui.Span{
			Text: request, Font: portrait.Type.UI.Family, Size: portrait.Type.UI.RenderedSize(),
		})
		lines = max(lines, int(math.Round(float64(measured/portrait.Type.UI.Leading))))
	}
	return min(3, max(1, lines))
}

func (p *panel) drawBusyPill(c *ui.Context, v *documentView, busy bool) {
	message := v.message
	if message == "" {
		message = "Generating…"
	}
	ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(portrait.Form.Gap).AlignItems(ui.Center).Role(ui.RoleStatus).Label(message).Busy(true).Children(func() {
		portrait.Spinner(c, message).Shrink(0)
		portrait.Type.Small.Apply(ui.Text(c, message)).TextColor(portrait.Colors.Ink2).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
	})
	cancel := portrait.ButtonWith(c, "Cancel", "Cancel generation", portrait.ButtonSpec{
		Look: portrait.Quiet, Size: portrait.Sizes.Compact, Disabled: busy && v.cancel == nil,
	})
	if cancel.Clicked() && v.cancel != nil {
		v.cancel()
	}
	if v.focusCancel {
		cancel.Focus()
		v.focusCancel = false
	}
}

func (p *panel) drawFailurePill(c *ui.Context, v *documentView, d workspace.DocumentInfo, pill ui.Element) {
	status := "Generation failed"
	if message := strings.TrimSpace(v.failure.Message); message != "" {
		status += " · " + message
	}
	ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(portrait.Form.Gap).AlignItems(ui.Center).Role(ui.RoleStatus).Label(status).Children(func() {
		portrait.Tooltip(c, ui.Box(c).Size(portrait.GenUI.StatusDot, portrait.GenUI.StatusDot).Radius(portrait.Radii.Pill).
			Background(portrait.Tones.Danger.Solid).AccessHidden(true), status)
		portrait.Type.Small.Apply(ui.Text(c, "Generation failed")).TextColor(portrait.Tones.Danger.Ink).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
	})
	details := portrait.ButtonWith(c, "Details", "Generation failure details", portrait.ButtonSpec{
		Look: portrait.Link, Size: portrait.Sizes.Compact, Ink: portrait.Tones.Danger.Ink,
	}).Expanded(v.failureOpen)
	if details.Clicked() {
		v.failureOpen = !v.failureOpen
		v.options = false
	}
	width, maxHeight := upwardPopoverRoom(v.paneBounds(c), pill)
	panel := portrait.PopoverWithSpec(c, details, &v.failureOpen, portrait.PopoverSpec{
		Width: width, MaxHeight: maxHeight, Title: "Generation failure details", Opaque: true, Placement: ui.AnchorTopStart,
	}, func() {
		p.drawGenerationStatus(c, v, d, false)
	})
	placePopoverAbove(panel, details, v.paneBounds(c), width, maxHeight)
	if panel.Dismissed() && c.KeyboardInput() {
		details.Focus()
	}
	if d.Path != "" && portrait.ButtonWith(c, "Retry", "Retry", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Compact}).Clicked() {
		p.generate(v, v.lastIDs)
	}
	p.drawCommandOptions(c, v, d, pill, false)
}

func (p *panel) drawCommandOptions(c *ui.Context, v *documentView, d workspace.DocumentInfo, pill ui.Element, busy bool) {
	options := portrait.ButtonWith(c, "", "GenUI options", portrait.ButtonSpec{
		Look: portrait.Quiet, Size: portrait.Sizes.Compact, Leading: portrait.IconDotsThree,
	}).Expanded(v.options)
	if options.Clicked() {
		v.options = !v.options
		v.failureOpen = false
	}
	pane := v.paneBounds(c)
	_, maxHeight := upwardPopoverRoom(pane, pill)
	width := min(portrait.GenUI.CommandMenuWidth, max(0, pane.W-2*portrait.Shell.PageInset))
	panel := portrait.PopoverWithSpec(c, options, &v.options, portrait.PopoverSpec{
		Width: width, MaxHeight: maxHeight, Placement: ui.AnchorTopStart,
	}, func() {
		p.drawOptions(c, v, d, busy)
	})
	portrait.Materials.GlassOverlay.Apply(panel)
	panel.Radius(portrait.Radii.L)
	placePopoverAbove(panel, pill, pane, width, maxHeight)
	panel.Bottom(portrait.Form.Gap)
	if panel.Dismissed() && c.KeyboardInput() {
		options.Focus()
	}
}

func (v *documentView) paneBounds(c *ui.Context) ui.Rect {
	if bounds := v.pane.Bounds(c); bounds.W > 0 && bounds.H > 0 {
		return bounds
	}
	width, height := c.Size()
	return ui.Rect{W: width, H: height}
}

func upwardPopoverRoom(pane ui.Rect, anchor ui.Element) (width, maxHeight float32) {
	available := max(0, pane.W-2*portrait.Shell.PageInset)
	if bounds := anchor.Bounds(); bounds.W > 0 {
		available = min(available, bounds.W)
	}
	width = min(portrait.Form.PopoverWide, available)
	if bounds := anchor.Bounds(); bounds.H > 0 {
		maxHeight = max(0, bounds.Y-pane.Y-portrait.Shell.PageInset-portrait.Form.PopoverPad)
	}
	return width, maxHeight
}

func placePopoverAbove(panel, anchor ui.Element, pane ui.Rect, width, maxHeight float32) {
	panel.AttachTo(anchor, ui.AnchorTopEnd, ui.AnchorBottomEnd).AttachEdgeMargin(portrait.Shell.PageInset).MaxHeight(maxHeight)
	placePopoverHorizontal(panel, anchor, pane, width)
}

func placePopoverBelow(panel, anchor ui.Element, pane ui.Rect, width, maxHeight float32) {
	panel.AttachTo(anchor, ui.AnchorBottomEnd, ui.AnchorTopEnd).AttachEdgeMargin(portrait.Shell.PageInset).MaxHeight(maxHeight)
	placePopoverHorizontal(panel, anchor, pane, width)
}

func placePopoverHorizontal(panel, anchor ui.Element, pane ui.Rect, width float32) {
	bounds := anchor.Bounds()
	offset := min(0, pane.X+pane.W-portrait.Shell.PageInset-(bounds.X+bounds.W))
	left := bounds.X + bounds.W - width + offset
	// MyGo clamps an overflowing attachment to the window before applying
	// Left. Correct only a base position that survives that clamp, or the
	// same left inset would be applied twice on narrow screens.
	if left < pane.X+portrait.Shell.PageInset && left >= portrait.Shell.PageInset {
		offset += pane.X + portrait.Shell.PageInset - left
	}
	panel.Left(offset)
}

func notePopoverRoom(pane ui.Rect, anchor ui.Element) (above bool, maxHeight float32) {
	bounds := anchor.Bounds()
	if bounds.H <= 0 {
		return false, max(0, pane.H-2*portrait.Shell.PageInset)
	}
	roomAbove := max(0, bounds.Y-pane.Y-portrait.Shell.PageInset-portrait.Form.PopoverPad)
	roomBelow := max(0, pane.Y+pane.H-(bounds.Y+bounds.H)-portrait.Shell.PageInset-portrait.Form.PopoverPad)
	if roomAbove >= 180 || roomAbove >= roomBelow {
		return true, roomAbove
	}
	return false, roomBelow
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

func (p *panel) drawGenerationStatus(c *ui.Context, v *documentView, d workspace.DocumentInfo, busy bool) {
	if v.failure == nil {
		return
	}
	var retry []portrait.AlertAction
	if !busy && d.Path != "" {
		retry = append(retry, portrait.AlertAction{Label: "Retry", OnAction: func() { p.generate(v, v.lastIDs) }})
	}
	portrait.Alert(c, portrait.Tones.Danger, "Generation failed", strings.TrimSpace(v.failure.Message+" "+v.failure.Hint), retry...)
	if v.failure.Detail != "" {
		portrait.OutputBlock(c, "Generation details", v.failure.Detail)
	}
}

// drawView draws the reader, or what stands in for it before one exists. A
// failure is already drawn above, so it never shares the page with the empty
// state, whose next step would contradict it.
func (p *panel) drawView(c *ui.Context, v *documentView, d workspace.DocumentInfo, raw string, busy bool) {
	v.openSource = func(section workspace.Section) {
		p.win.SetMode("markdown")
		p.win.Reveal(section.Start)
		if source := p.win.Source(); source != nil {
			_ = source.SelectUTF16(section.Start, section.End)
		}
	}
	switch {
	case !v.loaded:
		loading(c, "Loading the GenUI view", "Reading the view file for this document.")
	case v.snapshot.Error != "":
		portrait.Alert(c, portrait.Tones.Danger, "GenUI view", v.snapshot.Error)
	case v.cancel != nil && v.reader().surface == nil:
		loading(c, "Generating view…", "Your agent is laying out this document.")
	case v.failure != nil && v.reader().surface == nil:
		p.drawGenerationStatus(c, v, d, busy)
	case v.reader().surface == nil:
		ui.Column(c).FillWidth().Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			spec := portrait.EmptySpec{
				Icon:        portrait.IconSparkle,
				Title:       "No view yet",
				Description: "Generate a reading view from this document. The Markdown stays the source.",
			}
			spec.Action = func(c *ui.Context) ui.Element {
				b := portrait.AccentButton(c, "Write a request", "Write a request").Margin(6, 0, 0, 0)
				if b.Clicked() {
					v.focusRequest = true
				}
				return b
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

func (p *panel) drawOptions(c *ui.Context, v *documentView, d workspace.DocumentInfo, busy bool) {
	inset := portrait.Form.PadX - 2
	ui.Column(c).FillWidth().Padding(portrait.Form.MenuPad).Gap(portrait.Form.PopoverGap).Children(func() {
		ui.Row(c).FillWidth().Padding(portrait.Form.MenuPad, inset).AlignItems(ui.Center).Children(func() {
			portrait.Type.Meta.Apply(ui.Text(c, p.backendSummary())).TextColor(portrait.Colors.Ink3).
				Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
		})
		ui.Box(c).FillWidth().Height(portrait.Editor.Hairline).Margin(portrait.Form.PopoverGap, inset).Background(portrait.Colors.Line).AccessHidden(true)
		canGenerate := !busy && d.Path != ""
		if ids := v.changedIDs(); len(ids) > 0 {
			label := fmt.Sprintf("Regenerate changed (%d)", len(ids))
			if optionMenuRow(c, label, label, portrait.IconSparkle, !canGenerate).Clicked() {
				v.options = false
				p.generate(v, ids)
			}
		}
		if v.snapshot.CanUndo && optionMenuRow(c, "Restore previous view", "Restore previous view", portrait.IconClockCounterClockwise, busy).Clicked() {
			v.options = false
			v.message = message(p.host.UndoView(p.win.Context(), d.ID))
			if v.message == "" {
				v.requestEdited = false
			}
		}
		if optionMenuRow(c, "Configure…", "Configure GenUI", portrait.IconSlidersHorizontal, false).Clicked() {
			v.options = false
			p.win.OpenSettings("genui")
		}
	})
}

func optionMenuRow(c *ui.Context, title, label string, icon portrait.Icon, disabled bool) ui.Element {
	inset := portrait.Form.PadX - 2
	row := ui.ButtonBase(c.Key(label)).Label(label).FillWidth().Height(portrait.Form.MenuRow).
		Padding(0, inset).Gap(portrait.Form.Gap).Radius(portrait.Form.MenuRadius).
		AlignItems(ui.Center).Justify(ui.Start).Disabled(disabled)
	if row.Hovered() && !disabled {
		row.Background(portrait.Colors.Fill)
	}
	ink := portrait.Colors.Ink
	iconInk := portrait.Colors.Ink2
	if disabled {
		ink = portrait.Colors.Ink3
		iconInk = portrait.Colors.Ink3
	}
	row.Children(func() {
		portrait.Glyph(c, icon, portrait.IconSizes.M, iconInk).Shrink(0)
		portrait.Type.UI.Apply(ui.Text(c, title)).TextColor(ink).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
	})
	return row
}

func (p *panel) drawSources(c *ui.Context, v *documentView, d workspace.DocumentInfo, n Node, busy bool) {
	ss := v.matches(n)
	var notes []string
	for _, note := range v.notes {
		if slices.ContainsFunc(ss, func(s workspace.Section) bool { return note.Start >= s.Start && note.End <= s.End }) {
			notes = append(notes, note.Text)
		}
	}
	owner := v.reader().cardOwner
	if v.noteTarget == n.ID && owner.Valid() {
		owner.InsetShadow(portrait.GenUI.SelectionBorder, 0, 0, 0, portrait.Colors.Accent)
	}
	if len(notes) > 0 {
		ui.Column(c).Gap(portrait.Form.Gap).Margin(portrait.Form.HelpGap/2, 0, 0, 0).Role(ui.RoleList).Label("Notes").Children(func() {
			for i, note := range notes {
				ui.Row(c.Key(i)).AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Width(portrait.GenUI.NoteRule).Shrink(0).Background(portrait.Tones.Warning.Border).AccessHidden(true)
					portrait.Type.Meta.Apply(ui.Text(c, note)).TextColor(portrait.Colors.Ink2).Padding(0, 0, 0, portrait.Form.PadX).Selectable()
				})
			}
		})
	}

	corner := ui.Row(c.Key("source-corner-" + n.ID)).Gap(portrait.Form.PopoverGap).AlignItems(ui.Center)
	var cornerMaxWidth float32
	if owner.Valid() {
		corner.AttachTo(owner, ui.AnchorTopRight, ui.AnchorTopRight).Top(-14).Right(innerGap).ZIndex(1)
		if w := owner.Bounds().W; w > 0 {
			cornerMaxWidth = max(0, w-innerGap)
			corner.MaxWidth(cornerMaxWidth)
		}
	} else {
		corner.AlignSelf(ui.End)
	}
	var addNote ui.Element
	status, missing := "", false
	corner.Children(func() {
		actions := ui.Row(c).Gap(0).AlignItems(ui.Center).MinWidth(0).Padding(portrait.Form.PopoverGap).Radius(portrait.Radii.Pill)
		if cornerMaxWidth > 0 {
			actions.MaxWidth(cornerMaxWidth)
		}
		portrait.Materials.Pop.Apply(actions)
		reveal := !owner.Valid() || owner.Hovered() || owner.FocusWithin() || corner.Hovered() || corner.FocusWithin() ||
			v.noteTarget == n.ID || v.previewSource.open && v.previewSource.cardID == n.ID
		opacity := flag(reveal)
		if portrait.Animates(c, c.KeyboardInput()) {
			opacity = actions.AnimateTo(sourceActionsReveal{}, opacity, ui.Animation{Duration: portrait.Motion.Press, Ease: portrait.Motion.EaseOut.Easing(), NonMotion: true})
		}
		actions.Opacity(opacity)
		actions.Children(func() {
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
				anchor := ui.Box(c.Key(i)).MinWidth(0).Shrink(1).Children(func() {
					heading = sourceHeading(c, label, !ok).MinWidth(0)
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
				if ok {
					heading.Bind(handle)
				} else {
					anchor.Bind(handle)
				}
				v.previewTargets = append(v.previewTargets, sourcePreviewTarget{
					anchor: handle, key: key, label: label, cardID: n.ID,
					ref: ref, section: s, node: n, found: ok,
				})
				if heading.Clicked() {
					v.toggleSourcePreview(key, n.ID)
				}
				if !ok {
					status, missing = "Source missing", true
				} else if ref.Hash != "" && !s.UnchangedSince(ref.Hash) && status == "" {
					status = "Source changed"
				}
			}
			addNote = sourceButton(c, "Add note", d.ReadOnly || len(ss) == 0).Shrink(0)
			if addNote.Clicked() {
				v.noteTarget = n.ID
				v.note = ""
				v.noteError = ""
			}
			if v.focusNoteTarget == n.ID {
				addNote.Focus()
				v.focusNoteTarget = ""
			}
			update := sourceButton(c, "Update card", busy || d.Path == "" || n.ComponentID == "" && v.reader().surface.Session != nil).Shrink(0)
			if update.Clicked() {
				componentID := n.ComponentID
				if componentID == "" {
					componentID = n.ID
				}
				p.generate(v, []string{componentID})
			}
		})
		if status != "" {
			tone := portrait.Tones.Warning
			if missing {
				tone = portrait.Tones.Danger
			}
			dot := ui.Box(c).Size(portrait.GenUI.StatusDot, portrait.GenUI.StatusDot).Radius(portrait.Radii.Pill).
				Background(tone.Solid).Role(ui.RoleStatus).Label(status).Shrink(0)
			portrait.Tooltip(c, dot, status)
		}
	})
	if v.noteTarget != n.ID {
		return
	}
	closeNote := func(restoreFocus bool) {
		v.noteTarget = ""
		v.note = ""
		v.noteError = ""
		if restoreFocus {
			addNote.Focus()
			v.focusNoteTarget = n.ID
		}
	}
	noteOpen := true
	pane := v.paneBounds(c)
	width := min(portrait.Form.PopoverWide, max(0, pane.W-2*portrait.Shell.PageInset))
	above, maxHeight := notePopoverRoom(pane, addNote)
	placement := ui.AnchorBottomStart
	if above {
		placement = ui.AnchorTopStart
	}
	panel := portrait.PopoverWithSpec(c, addNote, &noteOpen, portrait.PopoverSpec{
		Width: width, MaxHeight: maxHeight, Opaque: true, Placement: placement,
	}, func() {
		portrait.Frame(c, func() {
			ui.Column(c).FillWidth().Gap(portrait.Form.Gap).Padding(portrait.Form.PadX).Children(func() {
				portrait.Type.Meta.Apply(ui.Text(c, "Reader note")).FontWeight(500).TextColor(portrait.Colors.Ink2).AccessHidden(true).IgnoreInput(true)
				note := portrait.TextArea(c, &v.note, portrait.AreaSpec{Label: "Reader note", Placeholder: "Reader note", Rows: 3, Bare: true}).AutoFocus()
				note.OnChange(func() { v.noteError = "" })
				if v.noteError != "" {
					portrait.Type.Meta.Apply(ui.Text(c, v.noteError)).TextColor(portrait.Tones.Danger.Ink).Role(ui.RoleAlert).Wrap()
				}
				composing := note.Composing()
				saveShortcut := note.Shortcut(ui.Cmd, ui.KeyEnter) && !composing
				cancelShortcut := note.Shortcut(0, ui.KeyEscape) && !composing
				enabled := len(ss) > 0 && !d.ReadOnly && strings.TrimSpace(v.note) != ""
				ui.Row(c).FillWidth().Justify(ui.End).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					portrait.Type.Meta.Apply(ui.Text(c, "⌘↩")).TextColor(portrait.Colors.Ink3).Grow(1).MinWidth(0)
					cancel := portrait.ButtonWith(c, "Cancel note", "Cancel note", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact})
					if cancel.Clicked() || cancelShortcut {
						closeNote(true)
					}
					save := portrait.ButtonWith(c, "Save note", "Save note", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Compact}).Disabled(!enabled)
					if (save.Clicked() || saveShortcut) && enabled {
						v.noteError = message(p.addNote(ss[0], v.note))
						v.message = v.noteError
						if v.noteError == "" {
							closeNote(true)
						}
					}
				})
			})
		})
	})
	if above {
		placePopoverAbove(panel, addNote, pane, width, maxHeight)
	} else {
		placePopoverBelow(panel, addNote, pane, width, maxHeight)
	}
	if panel.Dismissed() {
		closeNote(!panel.PressedOutside())
	}
}

type sourceActionsReveal struct{}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
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

func sourceHeading(c *ui.Context, label string, disabled bool) ui.Element {
	ink := portrait.Colors.Accent
	if disabled {
		ink = portrait.Colors.Ink3
	}
	return portrait.ButtonWith(c, "§ "+label, label, portrait.ButtonSpec{
		Look: portrait.Link, Size: portrait.Sizes.Compact, Ink: ink, Disabled: disabled,
	})
}

func sourceButton(c *ui.Context, label string, disabled bool) ui.Element {
	return portrait.ButtonWith(c, label, label, portrait.ButtonSpec{
		Look: portrait.Link, Size: portrait.Sizes.Compact, Ink: portrait.Colors.Ink3, Disabled: disabled,
	})
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
	v.options = false
	v.failureOpen = false
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
	case generationCancelled(err):
		return portrait.ToastSpec{ID: id, Tone: portrait.Tones.Neutral, Title: "Generation canceled"}
	default:
		return portrait.ToastSpec{ID: id, Tone: portrait.Tones.Danger, Title: "Generation failed"}
	}
}
func generationCancelled(err error) bool {
	return err != nil && (errors.Is(err, context.Canceled) || genui.FailureOf(err).Kind == genui.FailCancelled)
}

func (v *documentView) finished(err error) {
	v.cancel = nil
	v.reading.Replacing = nil
	v.message = ""
	if err != nil {
		v.discardPreview()
		if generationCancelled(err) {
			return
		}
		f := genui.FailureOf(err)
		v.failure = &f
		return
	}
	v.requestEdited = false
	v.reader().Halo()
}
