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

const (
	hairlineHeight float32 = 1
	spaceDotButton float32 = 18
	spaceDotCircle float32 = 10
)

func (s *Shell) sidebar(c *ui.Context, m Model, a Actions, docked bool) {
	side := ui.Column(c).Fill().MinWidth(0).Role(ui.RoleNavigation).Label("Workspace navigation").Gap(6)
	edge := portrait.Shell.PageInset
	if docked {
		side.Padding(edge, edge/2, edge, edge)
	} else {
		side.Padding(edge/2, edge, edge, edge)
	}
	side.Children(func() {
		if docked {
			ui.Row(c).Height(portrait.Shell.PageBar).Padding(0, 0, 0, c.TitleBar().Left).AlignItems(ui.Center).DragWindow().Children(func() {
				ui.Row(c).Grow(1)
				if portrait.QuietIconButton(c, portrait.IconSidebar, "Hide sidebar").Clicked() {
					s.hideSidebar = true
				}
			})
		}
		s.space(c, m, a)
		s.commandField(c, a)
		s.documents(c, m, a)
		if len(m.Workspace.Documents) > 0 {
			ui.Box(c).FillWidth().Height(hairlineHeight).Background(portrait.Colors.Line).Margin(2, 4)
		}
		s.navHeader(c, m, a)
		if s.outline {
			s.outlineList(c, m, a)
		} else {
			s.files(c, m, a)
		}
		s.sidebarFooter(c, m, a)
	})
}

func (s *Shell) navHeader(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		portrait.Track(c, "navigation", func(t *portrait.Segments) {
			for _, title := range []string{"Files", "Outline"} {
				outline := title == "Outline"
				button := t.Segment(c, title, title, s.outline == outline).Grow(1).Role(ui.RoleTab).AccessSelected(s.outline == outline)
				if button.Clicked() {
					s.outline = outline
				}
				if button.Focused() && (button.Shortcut(0, ui.KeyLeft) || button.Shortcut(0, ui.KeyRight)) {
					s.outline = !outline
					s.navFocus = true
				}
				if s.navFocus && s.outline == outline {
					button.Focus()
					s.navFocus = false
				}
			}
		}).Label("Navigation views").Grow(1)
		if !s.outline && w.Kind == "folder" {
			active := s.filterOpen || strings.TrimSpace(s.query) != ""
			toggle := portrait.ButtonWith(c.Key("toggle-filter"), "", "Filter files", portrait.ButtonSpec{
				Look:     portrait.Quiet,
				Size:     portrait.Sizes.Regular,
				Leading:  portrait.IconFunnel,
				Selected: active,
			}).Expanded(active)
			if toggle.Clicked() {
				if active {
					s.filterOpen = false
					if s.query != "" {
						s.query = ""
						delete(s.queries, w.Root)
						if a.Filter != nil {
							a.Filter("")
						}
					}
				} else {
					s.filterOpen = true
					s.filterFocus = true
				}
			}
		}
	})
}

func (s *Shell) commandField(c *ui.Context, a Actions) {
	field := ui.ButtonBase(c.Key("command field")).Label("Search or run a command").FillWidth().Height(portrait.Shell.Row).Padding(0, 10).Gap(8).Radius(portrait.Radii.M).AlignItems(ui.Center).Expanded(s.command.open)
	portrait.Materials.Sunken.Apply(field)
	if field.Hovered() {
		field.Background(portrait.Materials.GlassHover)
	}
	field.Children(func() {
		portrait.Glyph(c, portrait.IconMagnifyingGlass, portrait.Type.Small.Size, portrait.Colors.Ink2)
		portrait.Type.UI.Apply(ui.Text(c, "Search or jump to…")).TextColor(portrait.Colors.Ink2).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
		portrait.Type.Meta.Apply(ui.Text(c, "⌘K")).TextColor(portrait.Colors.Ink2).AccessHidden(true)
	})
	if field.Clicked() {
		s.toggleCommand(a)
	}
}

func (s *Shell) space(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	name := w.Name
	if name == "" {
		name = "Untitled"
	}
	meta := "Untitled window"
	if w.Kind == "folder" {
		meta = fmt.Sprintf("%s · %d files", w.Root, m.Navigation.Total)
	}
	space := spaceOf(m)
	switcher := portrait.Tooltip(c, ui.ButtonBase(c.Key("Switch workspace")).Label("Switch workspace").Value(name), meta).FillWidth().Height(portrait.Shell.Row).Padding(0, 8).Gap(8).Radius(portrait.Radii.M).AlignItems(ui.Center)
	if switcher.Hovered() {
		switcher.Background(portrait.Materials.GlassHover)
	}
	switcher.Children(func() {
		space.Swatch(ui.Box(c).Size(portrait.Shell.Swatch, portrait.Shell.Swatch).Radius(portrait.Radii.XS).Shrink(0).AccessHidden(true))
		portrait.Shell.WorkspaceName.Apply(ui.Text(c, name)).FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…").MinWidth(0).Shrink(1)
		ui.Row(c).Grow(1)
		portrait.Glyph(c, portrait.IconCaretDown, portrait.Type.Meta.Size, portrait.Colors.Ink2)
	})
	switcher.Menu(func(menu *ui.Menu) {
		var current string
		for _, root := range w.Workspaces {
			selected := root.Root == w.Root
			if selected {
				current = root.ID
			}
			if menu.Item(root.Name).ID("switch-workspace:"+root.ID).Checked(selected).Chosen() && a.ActivateWorkspace != nil {
				a.ActivateWorkspace(root.ID)
			}
		}
		menu.Separator()
		if menu.Item("Open Folder…").Chosen() && a.OpenFolder != nil {
			a.OpenFolder()
		}
		if w.Kind == "folder" && current != "" && menu.Item("Close Workspace").ID("close-workspace:"+current).Chosen() && a.CloseWorkspace != nil {
			a.CloseWorkspace(current)
		}
		menu.Separator()
		menu.Submenu("Space", func(sub *ui.Menu) {
			for _, option := range portrait.Spaces {
				if sub.Item(option.Label).ID("space:"+option.Name).Radio(option.Name == space.Name).Chosen() && a.SetSpace != nil {
					a.SetSpace(option.Name)
				}
			}
		})
		menu.Submenu("Appearance", func(sub *ui.Menu) {
			for _, option := range portrait.Appearances {
				if sub.Item(string(option)).ID("appearance:"+string(option)).Radio(option == m.Appearance || option == portrait.System && m.Appearance == "").Chosen() && a.SetAppearance != nil {
					a.SetAppearance(option)
				}
			}
		})
		menu.Separator()
		if menu.Item("Settings…").ID("open-settings").Chosen() {
			s.OpenSettings("appearance")
		}
	})
}

func (s *Shell) outlineList(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	scroll := s.scrollState("outline:" + w.ActiveID)
	ui.Scroll(c.Key("outline:" + w.ActiveID)).FillWidth().Grow(1).MinHeight(collapsible).TrackScroll(scroll).Label("Document outline").MaskLinearGradient(scrollEdgeMask(180)).Children(func() {
		for _, section := range m.Sections {
			weight := 400
			ink := portrait.Colors.Ink2
			if section.Level <= 1 {
				weight, ink = 600, portrait.Colors.Ink
			} else if section.Level == 2 {
				weight, ink = 500, portrait.Colors.Ink
			}
			if navRow(c, navItem{
				key:    fmt.Sprintf("heading-%d", section.Start),
				title:  section.Heading,
				icon:   portrait.IconHash,
				weight: weight,
				ink:    ink,
			}).Clicked() && a.Reveal != nil {
				s.leftOpen = false
				a.Reveal(section.Start)
			}
		}
		if len(m.Sections) == 0 {
			sidebarNote(c, "No headings")
		}
	})
}

func (s *Shell) currentDir(root string) string {
	if root == "" || s.drills == nil {
		return ""
	}
	dir := s.drills[root]
	if dir == "" {
		return ""
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		delete(s.drills, root)
		return ""
	}
	return filepath.Clean(dir)
}

func directNavEntries(root, dir string, entries []workspace.NavEntry) []workspace.NavEntry {
	target := filepath.Clean(root)
	if dir != "" {
		target = filepath.Clean(dir)
	}
	var out []workspace.NavEntry
	for _, entry := range entries {
		if entry.Path != "" && filepath.Clean(filepath.Dir(entry.Path)) == target {
			out = append(out, entry)
		} else if dir == "" && entry.Depth == 0 && (entry.Path == "" || root == "") {
			out = append(out, entry)
		}
	}
	return out
}

func (s *Shell) drillAncestors(c *ui.Context, root, name, dir string) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	rootName := name
	if rootName == "" {
		rootName = filepath.Base(root)
	}
	ui.Column(c.Key("drill-ancestors")).FillWidth().Gap(1).Padding(0, 0, 4, 0).BorderWidth(0, 0, 1, 0).BorderColor(portrait.Colors.Line).Children(func() {
		ancestorButton := func(key, label, target string) {
			b := ui.ButtonBase(c.Key(key)).FillWidth().Height(portrait.Sizes.Compact).Padding(0, 8).Gap(4).Radius(portrait.Radii.S).AlignItems(ui.Center).Justify(ui.Start).Label("Go to " + label)
			if b.Hovered() {
				b.Background(portrait.Materials.GlassHover)
			}
			ink := portrait.Colors.Ink2
			if b.Hovered() {
				ink = portrait.Colors.Ink
			}
			b.Children(func() {
				portrait.Glyph(c, portrait.IconCaretLeft, portrait.Type.Meta.Size, ink).Shrink(0)
				portrait.Type.Meta.Apply(ui.Text(c, label)).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
			})
			if b.Clicked() {
				s.drillTo(root, target)
			}
		}
		ancestorButton("anc-root", rootName, "")
		if len(parts) > 2 {
			parentRel := strings.Join(parts[:len(parts)-1], "/")
			parentPath := filepath.Join(root, filepath.FromSlash(parentRel))
			ancestorButton("anc-parent", "… / "+parts[len(parts)-2], parentPath)
		} else if len(parts) == 2 {
			ancestorButton("anc-parent", parts[0], filepath.Join(root, parts[0]))
		}
		current := parts[len(parts)-1]
		ui.Row(c.Key("anc-current")).FillWidth().Height(portrait.Sizes.Compact).Padding(0, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			portrait.Glyph(c, portrait.IconFolderOpen, portrait.Type.Meta.Size, portrait.Colors.Accent).Shrink(0)
			portrait.Type.Meta.Apply(ui.Text(c, current)).FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…").MinWidth(0)
		})
	})
}

func (s *Shell) files(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	if w.Kind == "folder" && (s.filterOpen || strings.TrimSpace(s.query) != "") {
		portrait.InputFrame(c).Children(func() {
			portrait.Glyph(c, portrait.IconFunnel, portrait.Type.UI.Size, portrait.Colors.Ink2)
			filter := portrait.Type.UI.Apply(ui.TextInputBase(c.Key("filter:"+w.Root), &s.query)).FocusRing(false).Grow(1).MinWidth(0).Placeholder("Filter files and headings").Label("Filter files and headings")
			if s.filterFocus {
				filter.Focus()
				s.filterFocus = false
			}
			if filter.Changed() && a.Filter != nil {
				s.queries[w.Root] = s.query
				a.Filter(s.query)
			}
			if filter.Shortcut(0, ui.KeyEscape) {
				s.filterOpen = false
				if s.query != "" {
					s.query = ""
					delete(s.queries, w.Root)
					if a.Filter != nil {
						a.Filter("")
					}
				}
			}
			if s.query != "" && portrait.QuietIconButton(c, portrait.IconX, "Clear filter").Size(portrait.Sizes.Compact, portrait.Sizes.Compact).Radius(portrait.Radii.Pill).Clicked() {
				s.query = ""
				delete(s.queries, w.Root)
				if a.Filter != nil {
					a.Filter("")
				}
			}
		})
	}
	scrollKey := "workspace:" + w.Root
	if strings.TrimSpace(s.query) != "" {
		scrollKey = "filter:" + w.Root
	}
	capsuleGroup := "workspace:" + scrollKey
	ui.Scroll(c.Key(scrollKey)).Grow(1).MinHeight(collapsible).FillWidth().Role(ui.RoleTree).FocusGroup(ui.Vertical).Label("Workspace files").TrackScroll(s.scrollState(scrollKey)).MaskLinearGradient(scrollEdgeMask(180)).Children(func() {
		if strings.TrimSpace(s.query) != "" {
			if strings.TrimSpace(s.query) != m.Search.Query {
				return
			}
			for _, hit := range m.Search.Hits {
				if navRow(c, navItem{key: hit.Path, title: hit.Name, detail: hit.Excerpt, excerpt: hit.Excerpt, selected: hit.DocumentID == w.ActiveID, icon: portrait.IconFileText, group: capsuleGroup}).Clicked() && a.OpenPath != nil {
					s.leftOpen = false
					a.OpenPath(hit.Path, -1)
				}
				for _, heading := range hit.Headings {
					if navRow(c, navItem{key: fmt.Sprintf("%s-%d", hit.Path, heading.Offset), title: heading.Text, icon: portrait.IconHash}).Clicked() && a.OpenPath != nil {
						s.leftOpen = false
						a.OpenPath(hit.Path, heading.Offset)
					}
				}
			}
			if len(m.Search.Hits) == 0 {
				sidebarNote(c, "No matching files or headings")
			}
			return
		}
		if w.Kind != "folder" {
			sidebarNote(c, "Open a folder to browse its Markdown files here.")
			if portrait.ButtonLeadingIcon(c, portrait.IconFolderPlus, "Open Folder…", "Open a folder").Margin(2, 8).Clicked() && a.OpenFolder != nil {
				a.OpenFolder()
			}
			return
		}
		dir := s.currentDir(w.Root)
		if dir != "" {
			s.drillAncestors(c, w.Root, w.Name, dir)
		}
		visible := directNavEntries(w.Root, dir, m.Navigation.Entries)
		if dir != "" && len(visible) == 0 && len(m.Navigation.Entries) > 0 {
			hasDir := slices.ContainsFunc(m.Navigation.Entries, func(e workspace.NavEntry) bool { return e.Kind == "folder" && filepath.Clean(e.Path) == dir })
			if !hasDir {
				s.drillTo(w.Root, "")
				visible = directNavEntries(w.Root, "", m.Navigation.Entries)
			}
		}
		for _, entry := range visible {
			folder := entry.Kind == "folder"
			detail := entry.Parent
			if detail == "" {
				detail = entry.Path
			}
			status := stateLabel(entry.Edited, entry.Conflict, entry.Deleted, false, false)
			if status != "" {
				detail = status
			}
			trailing := ""
			var trailingColor ui.Color
			if folder {
				detail = fmt.Sprintf("%d files", entry.MarkdownCount)
				trailing = fmt.Sprint(entry.MarkdownCount)
			} else if status != "" {
				trailing = status
				if entry.Conflict || entry.Deleted {
					trailingColor = portrait.Colors.Danger
				} else if entry.Edited {
					trailingColor = portrait.Colors.Warn
				}
			}
			icon := portrait.IconFileText
			if folder {
				icon = portrait.IconFolder
			}
			button := navRow(c, navItem{
				key:           entry.ID,
				title:         entry.Name,
				detail:        detail,
				trailing:      trailing,
				trailingColor: trailingColor,
				chevron:       folder,
				selected:      entry.DocumentID != "" && entry.DocumentID == w.ActiveID,
				icon:          icon,
				group:         capsuleGroup,
			})
			if folder {
				button.Expanded(entry.FolderState == workspace.FolderOpen)
			}
			if button.Clicked() {
				if folder {
					s.drillTo(w.Root, entry.Path)
					if entry.FolderState != workspace.FolderOpen {
						if rel, err := filepath.Rel(w.Root, entry.Path); err == nil && a.ToggleFolder != nil {
							a.ToggleFolder(filepath.ToSlash(rel))
						}
					}
				} else if entry.DocumentID != "" && a.Activate != nil && slices.ContainsFunc(w.Documents, func(doc workspace.DocumentInfo) bool {
					return doc.ID == entry.DocumentID
				}) {
					s.leftOpen = false
					a.Activate(entry.DocumentID)
				} else if a.OpenPath != nil {
					s.leftOpen = false
					a.OpenPath(entry.Path, -1)
				}
			}
		}
		if m.Navigation.Error != "" {
			portrait.Alert(c, portrait.Tones.Danger, m.Navigation.Error, "")
		} else if m.Navigation.Status == workspace.IndexScanning || m.Navigation.Status == workspace.IndexIndexing {
			sidebarNote(c, fmt.Sprintf("Indexing %d of %d files", m.Navigation.Indexed, m.Navigation.Total)).Role(ui.RoleStatus)
		} else if len(visible) == 0 {
			sidebarNote(c, "No Markdown files")
		}
	})
}

func (s *Shell) sidebarFooter(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	activeSpace := spaceOf(m)
	ui.Column(c).FillWidth().Gap(4).Padding(6, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(portrait.Colors.Line).Children(func() {
		ui.Row(c.Key("wuxing-palette-strip")).FillWidth().Height(portrait.Sizes.Compact).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Role(ui.RoleGroup).Label("Wu Xing Spaces").Gap(2).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
				for _, opt := range portrait.Spaces {
					selected := opt.Name == activeSpace.Name
					tip := opt.Label + " · " + opt.Element
					btn := portrait.Tooltip(c, ui.ButtonBase(c.Key("palette:"+opt.Name)).Label("Space: "+opt.Label).AccessSelected(selected), tip).
						Size(spaceDotButton, spaceDotButton).Radius(portrait.Radii.Pill).AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
					if selected {
						btn.Border(1.5, portrait.Colors.Accent)
					} else if btn.Hovered() {
						btn.Background(portrait.Materials.GlassHover)
					}
					btn.Children(func() {
						dot := opt.Dot(ui.Box(c).Size(spaceDotCircle, spaceDotCircle).Radius(portrait.Radii.Pill).AccessHidden(true))
						if !selected && !btn.Hovered() {
							dot.Opacity(0.84)
						}
					})
					if btn.Clicked() && !selected && a.SetSpace != nil {
						a.SetSpace(opt.Name)
					}
				}
			})
			if a.SetAppearance != nil {
				icon, label, next := portrait.IconMoon, "Switch to Dark Mode", portrait.Dark
				if s.dark {
					icon, label, next = portrait.IconSun, "Switch to Light Mode", portrait.Light
				}
				if portrait.QuietIconButton(c, icon, label).Size(portrait.Sizes.Compact, portrait.Sizes.Compact).Clicked() {
					a.SetAppearance(next)
				}
			}
		})
		ui.Row(c).FillWidth().Height(portrait.Sizes.Regular).Gap(2).AlignItems(ui.Center).Children(func() {
			if len(w.Workspaces) > 1 {
				ui.Row(c).Role(ui.RoleTabList).Label("Workspaces").Gap(2).AlignItems(ui.Center).MinWidth(0).Shrink(1).Clip().Children(func() {
					for _, root := range w.Workspaces {
						selected := root.Root == w.Root
						dot := portrait.Tooltip(c, ui.ButtonBase(c.Key("space:"+root.ID)).Role(ui.RoleTab).Label("Show "+root.Name), root.Root).Size(portrait.Sizes.Compact, portrait.Sizes.Compact).Radius(portrait.Radii.S).AlignItems(ui.Center).Justify(ui.Center).AccessSelected(selected).Shrink(0)
						if selected {
							dot.Background(portrait.Materials.GlassSelected)
						} else if dot.Hovered() {
							dot.Background(portrait.Materials.GlassHover)
						}
						dot.Children(func() {
							mark := spaceNamed(m, root.Root).Swatch(ui.Box(c).Size(portrait.Type.Meta.Size, portrait.Type.Meta.Size).Radius(portrait.Radii.XS).AccessHidden(true))
							if !selected {
								mark.Opacity(0.72)
							}
						})
						if dot.Clicked() && !selected && a.ActivateWorkspace != nil {
							a.ActivateWorkspace(root.ID)
						}
					}
				})
			}
			ui.Row(c).Grow(1)
			portrait.QuietIconButton(c, portrait.IconPlugs, "Plugins").Menu(func(menu *ui.Menu) {
				if menu.Item("Feynman").Checked(m.Feynman).Chosen() && a.SetFeynman != nil {
					a.SetFeynman(!m.Feynman)
				}
				if menu.Item("Git").Checked(m.Git).Chosen() && a.SetGit != nil {
					a.SetGit(!m.Git)
				}
			})
			if portrait.QuietIconButton(c, portrait.IconGear, "Settings").Clicked() {
				s.OpenSettings("appearance")
			}
			if portrait.QuietIconButton(c, portrait.IconPlus, "New document").Clicked() && a.NewDocument != nil {
				a.NewDocument()
			}
		})
	})
}

func spaceNamed(m Model, root string) portrait.Space {
	if space, ok := portrait.SpaceNamed(m.Spaces[root]); ok {
		return space
	}
	return portrait.SpaceFor(root)
}

func sidebarNote(c *ui.Context, text string) ui.Element {
	return portrait.Type.UI.Apply(ui.Text(c, text)).TextColor(portrait.Colors.Ink2).Padding(6, 8).Wrap()
}

func (s *Shell) scrollState(key string) *ui.ScrollState {
	if s.scrolls == nil {
		s.scrolls = map[string]*ui.ScrollState{}
	}
	if s.scrolls[key] == nil {
		s.scrolls[key] = &ui.ScrollState{}
	}
	return s.scrolls[key]
}

type navItem struct {
	key, title, detail, trailing, excerpt string
	trailingColor                         ui.Color
	chevron                               bool
	selected                              bool
	icon                                  portrait.Icon
	group                                 string
	weight                                int
	ink                                   ui.Color
}

func navRow(c *ui.Context, item navItem) ui.Element {
	row := ui.ButtonBase(c.Key(item.key)).FillWidth().Role(ui.RoleTreeItem).Label(item.title).Description(item.detail).AccessSelected(item.selected).MinHeight(portrait.Sizes.Regular).Padding(4, 8).Radius(portrait.Radii.M).AlignItems(ui.Center)
	if !item.selected && row.Hovered() {
		row.Background(portrait.Materials.GlassHover)
	}
	ink := item.ink
	if ink == (ui.Color{}) {
		ink = portrait.Colors.Ink2
	}
	if item.selected || row.Hovered() {
		ink = portrait.Colors.Ink
	}
	iconColor := portrait.Colors.Ink2
	if item.selected {
		iconColor = portrait.Colors.Accent
	}
	row.Children(func() {
		portrait.RowWash(c, row, item.selected, item.group)
		ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Center).ZIndex(1).Children(func() {
			portrait.Glyph(c, item.icon, portrait.Type.Small.Size, iconColor).Shrink(0)
			ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Start).Children(func() {
				title := portrait.Type.UI.Apply(ui.Text(c, item.title)).MaxLines(1).Ellipsis("…").TextColor(ink)
				if item.weight != 0 {
					title.FontWeight(item.weight)
				}
				if item.excerpt != "" {
					portrait.Type.Meta.Apply(ui.Text(c, item.excerpt)).MaxLines(2).Ellipsis("…").TextColor(portrait.Colors.Ink2)
				}
			})
			if item.trailing != "" {
				tColor := item.trailingColor
				if tColor == (ui.Color{}) {
					tColor = portrait.Colors.Ink2
				}
				portrait.Type.Meta.Apply(ui.Text(c, item.trailing)).TextColor(tColor).Shrink(0).AccessHidden(true)
			}
			if item.chevron {
				portrait.Glyph(c, portrait.IconCaretRight, portrait.Type.Meta.Size-2, portrait.Colors.Ink3).Shrink(0)
			}
		})
	})
	return row
}
