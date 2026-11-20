package nativegenui

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

//go:embed catalog.txt
var catalog string

type documentView struct {
	reading                                        Reading
	snapshot                                       genui.ViewEvent
	loaded, settings, requestEdited, commandEdited bool
	request, command, message, note, noteTarget    string
	sourceText                                     string
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
	v.reading.Load(nil)
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
			if ui.Button(c, "Generation settings").Expanded(v.settings).Clicked() {
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
			ui.Text(c, v.message).Role(ui.RoleStatus)
		}
		if v.cancel != nil && ui.Button(c, "Cancel generation").Clicked() {
			v.cancel()
		}
		switch {
		case !v.loaded:
			ui.Text(c, "Loading view…")
		case v.snapshot.Error != "":
			ui.Text(c, v.snapshot.Error).Role(ui.RoleAlert)
		case v.cancel != nil && v.reading.surface == nil:
			ui.Column(c).Gap(8).Padding(20).Busy(true).Children(func() { ui.Text(c, "Generating view…").Role(ui.RoleStatus); skeleton(c) })
		case v.reading.surface == nil:
			ui.Text(c, "No view yet.")
			if ui.Button(c, "Set up generation").Clicked() {
				v.settings = true
			}
		default:
			v.reading.CanSelect = func(n Node) bool { return len(v.matches(n)) > 0 }
			v.reading.SelectItem = func(n Node, text string) {
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
				p.win.Reveal(start)
				_ = p.win.Source().SelectUTF16(start, end)
			}
			v.reading.Sources = func(c *ui.Context, n Node) { p.drawSources(c, v, d, n, busy) }
			v.reading.View(c)
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
}

func (v *documentView) source(id string) (Reference, workspace.Section, bool) {
	ref := v.reading.surface.Sources[id]
	s, ok := v.sections[headingKey{workspace.NormalizeHeadingText(ref.Heading), max(1, ref.Occurrence)}]
	return ref, s, ok
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
		for runtimeID, n := range v.reading.surface.Nodes {
			for _, source := range n.Sources {
				r, s, ok := v.source(source)
				if !ok || (r.Hash != "" && r.Hash != s.Hash) {
					id := n.ComponentID
					if id == "" {
						id = n.ID
					}
					if id == "" {
						id = runtimeID
					}
					if id != "" && !seen[id] {
						ids = append(ids, id)
						seen[id] = true
					}
					break
				}
			}
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
	for _, note := range v.notes {
		if slices.ContainsFunc(ss, func(s workspace.Section) bool { return note.Start >= s.Start && note.End <= s.End }) {
			ui.Text(c, "Note: "+note.Text).Selectable()
		}
	}
	ui.Row(c).Gap(6).Wrap().Children(func() {
		ui.Text(c, "From")
		for i, id := range n.Sources {
			ref, s, ok := v.source(id)
			label := ref.Heading
			if label == "" {
				label = id
			}
			if ref.Occurrence > 1 {
				label = fmt.Sprintf("%s (%d)", label, ref.Occurrence)
			}
			if ui.Button(c.Key(i), label).Disabled(!ok).Clicked() {
				p.win.Reveal(s.Start)
				_ = p.win.Source().SelectUTF16(s.Start, s.End)
			}
			if !ok {
				ui.Text(c, "Source missing")
			} else if ref.Hash != "" && ref.Hash != s.Hash {
				ui.Text(c, "Source changed")
			}
		}
	})
	ui.Row(c).Gap(6).Children(func() {
		if ui.Button(c, "Add note").Disabled(d.ReadOnly || len(ss) == 0).Clicked() {
			v.noteTarget = n.ID
			v.note = ""
		}
		if ui.Button(c, "Update card").Disabled(busy || d.Path == "").Clicked() {
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
	ui.TextArea(c, &v.note).Label("Reader note").Height(52).AutoFocus()
	enabled := len(ss) > 0 && !d.ReadOnly && strings.TrimSpace(v.note) != ""
	if (ui.Button(c, "Save note").Disabled(!enabled).Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)) && enabled {
		v.message = message(p.addNote(ss[0], v.note))
		if v.message == "" {
			v.noteTarget = ""
			v.note = ""
		}
	}
	if ui.Button(c, "Cancel note").Clicked() || c.Shortcut(0, ui.KeyEscape) {
		v.noteTarget = ""
		v.note = ""
	}
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
	v.reading.Replacing = v.lastIDs
	v.message = "Starting generation…"
	mode := "full"
	if len(ids) > 0 {
		mode = "update"
	}
	req := genui.GenerateRequest{Text: text, Mode: mode, Format: catalog, Request: v.request, ReplaceIDs: ids, Sections: workspace.GenerationSections(text)}
	done := p.host.StartGeneration(ctx, d.ID, req, func(progress genui.GenerateProgress) error {
		p.win.Native().Update(func() {
			if !p.closed && p.views[d.ID] == v {
				v.message = strings.TrimSpace(progress.Phase + " " + progress.Text)
			}
		})
		return ctx.Err()
	})
	go func() {
		defer cancel()
		err := <-done
		p.win.Native().Update(func() {
			if !p.closed && p.views[d.ID] == v {
				v.cancel = nil
				v.reading.Replacing = nil
				v.message = ""
				if err != nil {
					f := genui.FailureOf(err)
					v.failure = &f
				} else {
					v.requestEdited = false
				}
			}
		})
	}()
}
