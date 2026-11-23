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
	reading                                        Reading
	preview                                        *Reading
	stream                                         *statementStream
	snapshot                                       genui.ViewEvent
	loaded, settings, requestEdited, commandEdited bool
	request, command, message, note, noteTarget    string
	sourceText, imagesFor                          string
	sections                                       map[headingKey]workspace.Section
	notes                                          []workspace.Note
	failure                                        *genui.GenerateFailure
	lastIDs                                        []string
	cancel                                         context.CancelFunc
}
type headingKey struct {
	name       string
	occurrence int
}
type panel struct {
	win      *nativeui.Window
	host     *workspace.Service
	views    map[string]*documentView
	closed   bool
	settings workspace.Settings
}

func Install(host *workspace.Service) func(*nativeui.Window) nativeui.Panels {
	return func(win *nativeui.Window) nativeui.Panels {
		p := &panel{win: win, host: host, views: make(map[string]*documentView)}
		p.settings, _ = host.GetSettings()
		workspace.SettingsChanged.Listen(win.Native(), func(next workspace.Settings) {
			p.settings = next
			for _, v := range p.views {
				if !v.commandEdited {
					v.command = next.AgentCommand
				}
			}
		})
		genui.ViewChanged.Listen(win.Native(), p.updated)
		return nativeui.Panels{GenUI: p.draw, Changed: p.changed, Closed: p.close}
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
	v := p.views[d.ID]
	if v == nil {
		v = &documentView{command: p.settings.AgentCommand}
		p.views[d.ID] = v
		if err := p.host.WatchView(p.win.Context(), d.ID); err != nil {
			v.loaded = true
			v.snapshot.Error = err.Error()
		}
	}
	if p.win.Source() == nil {
		ui.Text(c, "Loading document…")
		return
	}
	c = c.Key(d.ID)
	raw := p.win.Source().Source()
	v.index(raw)
	busy := v.cancel != nil || d.Generating
	ui.Column(c).Fill().MinHeight(0).Gap(8).Padding(12).Children(func() {
		ui.Row(c).Justify(ui.End).Children(func() {
			if portrait.QuietButton(c, "Generation settings", "Generation settings").Expanded(v.settings).Clicked() {
				v.settings = !v.settings
			}
		})
		if v.settings {
			p.drawSettings(c, v, d, busy)
		}
		if v.failure != nil {
			ui.Text(c, strings.TrimSpace(v.failure.Message+" "+v.failure.Hint)).Role(ui.RoleAlert)
			if v.failure.Detail != "" {
				ui.Text(c, v.failure.Detail).Selectable()
			}
			if ui.Button(c, "Retry generation").Disabled(busy || d.Path == "").Clicked() {
				p.generate(v, v.lastIDs)
			}
		}
		if v.message != "" {
			portrait.Type.UI.Apply(ui.Text(c, v.message)).TextColor(portrait.Colors.Ink2).Role(ui.RoleStatus)
		}
		if v.cancel != nil && ui.Button(c, "Cancel generation").Clicked() {
			v.cancel()
		}
		switch {
		case !v.loaded:
			loading(c, "Loading the GenUI view", "Reading the view file for this document.")
		case v.snapshot.Error != "":
			ui.Text(c, v.snapshot.Error).Role(ui.RoleAlert)
		case v.cancel != nil && v.reader().surface == nil:
			loading(c, "Generating view…", "Your agent is laying out this document.")
		case v.reader().surface == nil:
			ui.Text(c, "No view yet.")
			if ui.Button(c, "Set up generation").Clicked() {
				v.settings = true
			}
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
			reader.View(c)
		}
	})
}

func (v *documentView) index(raw string) {
	if v.sections != nil && raw == v.sourceText {
		return
	}
	v.sourceText = raw
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
	ui.TextArea(c, &v.request).Height(52).Label("Generation request").Placeholder("Generation request").Disabled(busy).OnChange(func() { v.requestEdited = true })
	model := p.settings.Model
	models := p.settings.Models
	if !slices.Contains(models, model) {
		models = append(slices.Clone(models), model)
	}
	ui.Row(c).Gap(8).Wrap().Children(func() {
		ui.Text(c, "Model")
		ui.Select(c, &model, models).Label("Model").Disabled(busy).OnChange(func() {
			_, err := p.host.SetSettings(workspace.SettingsPatch{Model: &model})
			v.message = message(err)
		})
		ui.TextInput(c, &v.command).Label("Agent command").Placeholder("Agent command").Disabled(busy).OnChange(func() { v.commandEdited = true })
		if ui.Button(c, "Save command").Disabled(busy || !v.commandEdited).Clicked() {
			s, err := p.host.SetSettings(workspace.SettingsPatch{AgentCommand: &v.command})
			v.message = message(err)
			if err == nil {
				v.commandEdited = false
				v.command = s.AgentCommand
			}
		}
	})
	if d.Path == "" {
		ui.Text(c, "Save the document first to generate a view.")
	}
	ui.Row(c).Gap(6).Wrap().Children(func() {
		if ui.Button(c, "Generate").Disabled(busy || d.Path == "").Clicked() {
			p.generate(v, nil)
		}
		if ids := v.changedIDs(); len(ids) > 0 && ui.Button(c, fmt.Sprintf("Regenerate changed (%d)", len(ids))).Disabled(busy || d.Path == "").Clicked() {
			p.generate(v, ids)
		}
		if v.snapshot.CanUndo && ui.Button(c, "Restore previous view").Disabled(busy).Clicked() {
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
				portrait.Type.Meta.Apply(ui.Text(c.Key(i), note)).Padding(4, 8, 4, 11).Radius(4).Background(portrait.Colors.WarningSoft).
					InsetShadow(3, 0, 0, 0, portrait.Colors.Warning).Selectable()
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
				if sourceHeading(c.Key(i), label, !ok).Clicked() {
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
	note := ui.TextArea(c, &v.note).Label("Reader note").Height(52).AutoFocus()
	composing := note.Composing()
	saveShortcut := note.Shortcut(ui.Cmd, ui.KeyEnter) && !composing
	cancelShortcut := note.Shortcut(0, ui.KeyEscape) && !composing
	enabled := len(ss) > 0 && !d.ReadOnly && strings.TrimSpace(v.note) != ""
	if (portrait.AccentButton(c, "Save note", "Save note").Disabled(!enabled).Clicked() || saveShortcut) && enabled {
		v.message = message(p.addNote(ss[0], v.note))
		if v.message == "" {
			v.noteTarget = ""
			v.note = ""
		}
	}
	if portrait.QuietButton(c, "Cancel note", "Cancel note").Clicked() || cancelShortcut {
		v.noteTarget = ""
		v.note = ""
	}
}

func highlightCard(reader *Reading, n Node) {
	id := n.ComponentID
	if id == "" {
		id = n.ID
	}
	reader.Highlight(id)
}

func sourceHeading(c *ui.Context, label string, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).DimDisabled(false).Radius(3)
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
	fill, ink, icon := portrait.Colors.WarnSoft, portrait.Colors.Warn, portrait.IconWarning
	if missing {
		fill, ink, icon = portrait.Colors.DangerSoft, portrait.Colors.Danger, portrait.IconWarningCircle
	}
	ui.Row(c.Key("status")).Gap(4).AlignItems(ui.Center).Radius(999).Padding(1, 8).Background(fill).Role(ui.RoleStatus).Label(text).Children(func() {
		portrait.Glyph(c, icon, 12, ink)
		portrait.Type.Meta.Apply(ui.Text(c, text)).TextColor(ink)
	})
}

type sourceHover struct{}
type sourcePress struct{}

func sourceButton(c *ui.Context, label string, disabled bool) ui.Element {
	g := portrait.GenUI
	b := ui.ButtonBase(c.Key(label)).Label(label).Disabled(disabled).DimDisabled(false).Padding(2, 8).Radius(6).AlignItems(ui.Center)
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
			if !p.closed && p.views[d.ID] == v {
				v.finished(err)
			}
		})
	}()
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
