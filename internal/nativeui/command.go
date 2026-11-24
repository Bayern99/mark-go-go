package nativeui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type commandBar struct {
	open, focus, moved bool
	toggled            bool
	query, sent        string
	index              int
}

type commandItem struct {
	icon          portrait.Icon
	title, detail string
	keys          []string
	run           func()
}

const commandSearchLimit = 24

func (s *Shell) toggleCommand(a Actions) {
	if s.command.toggled {
		return
	}
	if s.command.open {
		s.closeCommand(a)
	} else {
		s.command = commandBar{open: true, focus: true}
	}
	s.command.toggled = true
}

func (s *Shell) closeCommand(a Actions) {
	if s.command.sent != "" && a.Filter != nil {
		a.Filter(s.query)
	}
	s.command = commandBar{}
}

func (s *Shell) searchQuery(root string) string {
	if s.command.open && s.command.sent != "" {
		return s.command.sent
	}
	return s.queries[root]
}

func (s *Shell) wantsSections() bool { return s.outline || s.command.open }

func (s *Shell) commandItems(m Model, a Actions, slots Slots, width float32) []commandItem {
	w := m.Workspace
	doc := activeDocument(w)
	query := strings.TrimSpace(s.command.query)
	needle := strings.ToLower(query)
	var items []commandItem
	add := func(item commandItem) {
		if needle == "" || strings.Contains(strings.ToLower(item.title+" "+item.detail), needle) {
			items = append(items, item)
		}
	}

	labels := tabLabels(w.Documents)
	docs := slices.Clone(w.Documents)
	rank := func(id string) int {
		if id == s.activeID {
			return len(s.history) + len(docs)
		}
		if i := slices.Index(s.history, id); i >= 0 {
			return i
		}
		return len(s.history)
	}
	slices.SortStableFunc(docs, func(a, b workspace.DocumentInfo) int { return rank(a.ID) - rank(b.ID) })
	for _, d := range docs {
		detail := "Open document"
		if d.ID == s.activeID {
			detail = "Current document"
		}
		if status := stateLabel(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating); status != "" {
			detail += " · " + status
		}
		add(commandItem{icon: portrait.IconFileText, title: labels[d.ID], detail: detail, run: func() {
			if a.Activate != nil {
				a.Activate(d.ID)
			}
		}})
	}

	if needle != "" {
		for _, section := range m.Sections {
			add(commandItem{icon: portrait.IconHash, title: section.Heading, detail: "Heading in " + doc.Name, run: func() {
				if a.Reveal != nil {
					a.Reveal(section.Start)
				}
			}})
		}
		if w.Kind == "folder" && m.Search.Query == query && a.OpenPath != nil {
			found := 0
			for _, hit := range m.Search.Hits {
				if found >= commandSearchLimit {
					break
				}
				open := slices.ContainsFunc(w.Documents, func(d workspace.DocumentInfo) bool { return d.ID == hit.DocumentID && hit.DocumentID != "" })
				if !open {
					folder := "File"
					if rel, err := filepath.Rel(w.Root, filepath.Dir(hit.Path)); err == nil && rel != "." {
						folder = filepath.ToSlash(rel)
					}
					items = append(items, commandItem{icon: portrait.IconFileText, title: hit.Name, detail: folder, run: func() { a.OpenPath(hit.Path, -1) }})
					found++
				}
				for _, heading := range hit.Headings {
					if hit.DocumentID == s.activeID && hit.DocumentID != "" {
						continue
					}
					items = append(items, commandItem{icon: portrait.IconHash, title: heading.Text, detail: "Heading in " + hit.Name, run: func() { a.OpenPath(hit.Path, heading.Offset) }})
					found++
				}
			}
		}
	}

	if a.NewDocument != nil {
		add(commandItem{icon: portrait.IconPlus, title: "New Document", detail: "Start an untitled document", run: a.NewDocument})
	}
	if a.OpenFolder != nil {
		add(commandItem{icon: portrait.IconFolderPlus, title: "Open Folder…", detail: "Add a workspace to this window", run: a.OpenFolder})
	}
	if doc.ID != "" && a.SetMode != nil {
		for _, mode := range []struct {
			id, title string
			icon      portrait.Icon
			keys      []string
		}{{"preview", "Show Preview", portrait.IconEye, []string{"⌘", "/"}}, {"markdown", "Show Markdown", portrait.IconCode, []string{"⌘", "/"}}, {"genui", "Show GenUI", portrait.IconSparkle, nil}} {
			current := doc.ViewMode == mode.id || mode.id == "preview" && doc.ViewMode != "markdown" && doc.ViewMode != "genui"
			if current || mode.id == "genui" && slots.GenUI == nil {
				continue
			}
			add(commandItem{icon: mode.icon, title: mode.title, detail: "View mode", keys: mode.keys, run: func() { a.SetMode(mode.id) }})
		}
	}
	if doc.ID != "" && m.Git && slots.Checkpoints != nil {
		add(commandItem{icon: portrait.IconClockCounterClockwise, title: "Checkpoints", detail: "Compare or restore local versions", run: func() { s.checkpoints = true }})
	}
	sidebar := "Hide Sidebar"
	if !s.sidebarDocked && !s.leftOpen {
		sidebar = "Show Sidebar"
	}
	add(commandItem{icon: portrait.IconSidebar, title: sidebar, detail: "Navigation", keys: []string{"⌘", "0"}, run: func() { s.toggleSidebar(width); s.persistSidebar(a) }})
	show := func(outline bool) func() {
		return func() {
			s.outline = outline
			if width < sidebarDocksAt {
				s.leftOpen, s.rightOpen = true, false
			} else {
				s.hideSidebar = false
			}
		}
	}
	add(commandItem{icon: portrait.IconHash, title: "Show Outline", detail: "Navigation", run: show(true)})
	add(commandItem{icon: portrait.IconFolder, title: "Show Files", detail: "Navigation", run: show(false)})
	if m.Feynman {
		add(commandItem{icon: portrait.IconAtom, title: "Toggle Research Panel", detail: "Feynman", keys: []string{"⌥", "⌘", "0"}, run: func() { s.toggleResearch(width); s.persistResearch(a) }})
	}
	for _, plugin := range []struct {
		name string
		on   bool
		set  func(bool)
	}{{"Feynman", m.Feynman, a.SetFeynman}, {"Git", m.Git, a.SetGit}} {
		if plugin.set == nil {
			continue
		}
		title := "Enable " + plugin.name
		if plugin.on {
			title = "Disable " + plugin.name
		}
		add(commandItem{icon: portrait.IconPlugs, title: title, detail: "Plugin", run: func() { plugin.set(!plugin.on) }})
	}
	if a.SetAppearance != nil {
		next, icon := portrait.Dark, portrait.IconMoon
		if s.dark {
			next, icon = portrait.Light, portrait.IconSun
		}
		add(commandItem{icon: icon, title: "Toggle Dark Mode", detail: "Appearance", run: func() { a.SetAppearance(next) }})
	}
	add(commandItem{icon: portrait.IconGear, title: "Open Settings…", detail: "General, Appearance, Editor, Research", keys: []string{"⌘", ","}, run: func() { s.OpenSettings("") }})
	if doc.ID != "" && a.CloseDocument != nil {
		add(commandItem{icon: portrait.IconX, title: "Close Document", detail: labels[doc.ID], keys: []string{"⌘", "W"}, run: func() { a.CloseDocument(doc.ID) }})
	}
	if a.ActivateWorkspace != nil {
		for _, root := range w.Workspaces {
			if root.Root != w.Root {
				add(commandItem{icon: portrait.IconFolderOpen, title: "Switch to " + root.Name, detail: "Workspace", run: func() { a.ActivateWorkspace(root.ID) }})
			}
		}
	}
	if needle != "" {
		pages := mergeSettingsPages(append(s.nativeSettingsPages(m, a), settingsPages(slots)...))
		for _, page := range pages {
			page := page
			if page.Visible == nil || page.Visible() {
				add(commandItem{icon: page.Icon, title: "Settings: " + page.Label, detail: "Settings Center", run: func() { s.OpenSettings(page.ID) }})
			}
		}
		if a.OpenRecent != nil {
			for _, recent := range m.Recents {
				add(commandItem{icon: portrait.IconFolder, title: "Open " + recent.Name, detail: recent.Root, run: func() { a.OpenRecent(recent.Root) }})
			}
		}
		if a.SetAppearance != nil {
			for _, appearance := range portrait.Appearances {
				add(commandItem{icon: portrait.IconMoon, title: "Appearance: " + string(appearance), detail: "Light, dark or follow the system", run: func() { a.SetAppearance(appearance) }})
			}
		}
		if a.SetSpace != nil {
			for _, space := range portrait.Spaces {
				add(commandItem{icon: portrait.IconPalette, title: "Space: " + space.Label, detail: "Colours of this workspace", run: func() { a.SetSpace(space.Name) }})
			}
		}
	}
	return items
}

func (s *Shell) commandBar(c *ui.Context, width, height float32, m Model, a Actions, slots Slots) {
	bar := &s.command
	var items []commandItem
	if bar.open {
		items = s.commandItems(m, a, slots, width)
		bar.index = max(0, min(bar.index, len(items)-1))
	}
	var run func()
	panel := portrait.DialogShell(c, "command bar", &bar.open, portrait.DialogSpec{
		Label: "Command Bar", Width: portrait.Shell.CommandWidth, Margin: 2 * portrait.Shell.PageInset,
		Radius: portrait.Radii.XL, Glass: true, Top: min(portrait.Shell.CommandTop, height/8),
	}, func() {
		ui.Row(c).Height(portrait.Shell.CommandInput).Padding(0, 14, 0, 18).Gap(10).AlignItems(ui.Center).Children(func() {
			portrait.Glyph(c, portrait.IconMagnifyingGlass, portrait.IconSizes.L, portrait.Colors.Ink3)
			input := portrait.Type.Reading.Apply(ui.TextInputBase(c.Key("command query"), &bar.query)).FocusRing(false).Grow(1).MinWidth(0).
				Placeholder("Search documents, headings and commands").Label("Search documents, headings and commands")
			if bar.focus {
				input.Focus()
				bar.focus = false
			}
			if input.Changed() {
				bar.index = 0
				if query := strings.TrimSpace(bar.query); a.Filter != nil && m.Workspace.Kind == "folder" && (query != "" || bar.sent != "") {
					bar.sent = query
					a.Filter(query)
				}
			}
			if n := len(items); n > 0 {
				if input.Shortcut(0, ui.KeyDown) {
					bar.index, bar.moved = (bar.index+1)%n, true
				}
				if input.Shortcut(0, ui.KeyUp) {
					bar.index, bar.moved = (bar.index+n-1)%n, true
				}
				if input.Submitted() {
					run = items[bar.index].run
				}
			}
			if input.Shortcut(ui.Cmd, ui.KeyK) || input.Shortcut(ui.Cmd, ui.KeyT) {
				s.toggleCommand(a)
			}
			portrait.Kbd(c, "esc").Opacity(0.7)
		})
		ui.Scroll(c.Key("command results")).FillWidth().MaxHeight(min(portrait.Shell.Results, height-portrait.Shell.CommandTop-portrait.Shell.CommandInput-portrait.Shell.Strip)).
			Padding(6).Gap(1).BorderWidth(1, 0, 0, 0).BorderColor(portrait.Colors.Line).Role(ui.RoleListBox).Label("Results").Children(func() {
			for i, item := range items {
				selected := i == bar.index
				row := ui.ButtonBase(c.Key(fmt.Sprintf("result:%d", i))).Role(ui.RoleOption).Label(item.title).Description(item.detail).AccessSelected(selected).TabStop(false).
					FillWidth().Height(portrait.Shell.CommandRow).Padding(0, 10).Gap(10).Radius(portrait.Radii.M).AlignItems(ui.Center)
				ink, muted := portrait.Colors.Ink, portrait.Colors.Ink3
				switch {
				case selected:
					row.Background(portrait.Colors.AccentSoft)
					ink, muted = portrait.Colors.Accent, portrait.Colors.Ink2
				case row.Hovered():
					row.Background(portrait.Colors.Fill)
				}
				if selected && bar.moved {
					row.ScrollIntoView()
				}
				row.Children(func() {
					portrait.Glyph(c, item.icon, portrait.IconSizes.M, muted).Shrink(0)
					portrait.Type.UI.Apply(ui.Text(c, item.title)).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0).Shrink(1)
					portrait.Type.Meta.Apply(ui.Text(c, item.detail)).TextColor(muted).MaxLines(1).Ellipsis("…").Grow(1).MinWidth(0).AccessHidden(true)
					if selected {
						portrait.Glyph(c, portrait.IconArrowElbowDownLeft, portrait.IconSizes.S, muted).Shrink(0)
					} else if len(item.keys) > 0 {
						portrait.KbdTint(c, muted, portrait.Clear, item.keys...).Shrink(0).AccessHidden(true)
					}
				})
				if row.Clicked() {
					run = item.run
				}
			}
			bar.moved = false
			if len(items) == 0 {
				portrait.Type.UI.Apply(ui.Text(c, "No matching documents or commands")).TextColor(portrait.Colors.Ink3).Padding(10, 10)
			}
		})
	})
	if panel.Valid() {
		portrait.Materials.GlassOverlay.Apply(panel).TransformOrigin(0.5, 0)
	}
	switch {
	case run != nil:
		s.closeCommand(a)
		run()
	case panel.Dismissed():
		s.closeCommand(a)
	}
	s.command.toggled = false
}
