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
		s.commandField(c, a)
		s.documents(c, m, a)
		s.space(c, m, a)
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
		}).Label("Navigation views")
		if s.outline {
			s.outlineList(c, m, a)
		} else {
			s.files(c, m, a)
		}
		s.sidebarFooter(c, m, a)
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
	switcher := portrait.Tooltip(c, ui.ButtonBase(c.Key("Switch workspace")).Label("Switch workspace").Value(name), meta).FillWidth().Height(portrait.Shell.Row).Margin(4, 0, 0, 0).Padding(0, 8).Gap(8).Radius(portrait.Radii.M).AlignItems(ui.Center)
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
		menu.Submenu("Scenery", func(sub *ui.Menu) {
			for _, option := range portrait.Spaces {
				if sub.Item(option.Name).ID("scenery:"+option.Name).Radio(option.Name == space.Name).Chosen() && a.SetSpace != nil {
					a.SetSpace(option.Name)
				}
			}
		})
	})
}

func (s *Shell) outlineList(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	scroll := s.scrollState("outline:" + w.ActiveID)
	ui.Scroll(c.Key("outline:" + w.ActiveID)).FillWidth().Grow(1).MinHeight(collapsible).TrackScroll(scroll).Label("Document outline").MaskLinearGradient(scrollEdgeMask(180)).Children(func() {
		for _, section := range m.Sections {
			if navRow(c, navItem{key: fmt.Sprintf("heading-%d", section.Start), title: section.Heading, depth: section.Level - 1, icon: portrait.IconHash}).Clicked() && a.Reveal != nil {
				s.leftOpen = false
				a.Reveal(section.Start)
			}
		}
		if len(m.Sections) == 0 {
			sidebarNote(c, "No headings")
		}
	})
}

func (s *Shell) files(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	if w.Kind == "folder" {
		portrait.InputFrame(c).Children(func() {
			portrait.Glyph(c, portrait.IconFunnel, portrait.Type.UI.Size, portrait.Colors.Ink2)
			if portrait.Type.UI.Apply(ui.TextInputBase(c.Key("filter:"+w.Root), &s.query)).FocusRing(false).Grow(1).MinWidth(0).Placeholder("Filter files and headings").Label("Filter files and headings").Changed() && a.Filter != nil {
				s.queries[w.Root] = s.query
				a.Filter(s.query)
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
					if navRow(c, navItem{key: fmt.Sprintf("%s-%d", hit.Path, heading.Offset), title: heading.Text, depth: 1, icon: portrait.IconHash}).Clicked() && a.OpenPath != nil {
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
		for _, entry := range m.Navigation.Entries {
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
			if folder {
				detail = fmt.Sprintf("%d files", entry.MarkdownCount)
				trailing = fmt.Sprint(entry.MarkdownCount)
			}
			icon := portrait.IconFileText
			if folder && entry.FolderState == workspace.FolderOpen {
				icon = portrait.IconFolderOpen
			} else if folder {
				icon = portrait.IconFolder
			}
			button := navRow(c, navItem{key: entry.ID, title: entry.Name, detail: detail, trailing: trailing, depth: entry.Depth, selected: entry.DocumentID != "" && entry.DocumentID == w.ActiveID, icon: icon, group: capsuleGroup})
			if folder {
				button.Expanded(entry.FolderState == workspace.FolderOpen)
			}
			if button.Clicked() {
				if folder {
					if rel, err := filepath.Rel(w.Root, entry.Path); err == nil && a.ToggleFolder != nil {
						a.ToggleFolder(filepath.ToSlash(rel))
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
		} else if len(m.Navigation.Entries) == 0 {
			sidebarNote(c, "No Markdown files")
		}
	})
}

func (s *Shell) sidebarFooter(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	ui.Row(c).Height(portrait.Sizes.Regular).Gap(2).AlignItems(ui.Center).Children(func() {
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
						mark := spaceNamed(m, root.Root).Swatch(ui.Box(c).Size(portrait.Type.Meta.Size, portrait.Type.Meta.Size).Radius(portrait.Radii.Pill).AccessHidden(true))
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
		if portrait.QuietIconButton(c, portrait.IconPlus, "New document").Clicked() && a.NewDocument != nil {
			a.NewDocument()
		}
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
	depth                                 int
	selected                              bool
	icon                                  portrait.Icon
	group                                 string
}

func navRow(c *ui.Context, item navItem) ui.Element {
	indent := portrait.Shell.PageInset + portrait.Type.Small.Size*float32(min(item.depth, 8))
	row := ui.ButtonBase(c.Key(item.key)).FillWidth().Role(ui.RoleTreeItem).Label(item.title).Description(item.detail).AccessSelected(item.selected).MinHeight(portrait.Sizes.Regular).Padding(4, 8, 4, indent).Radius(portrait.Radii.M).AlignItems(ui.Center)
	if !item.selected && row.Hovered() {
		row.Background(portrait.Materials.GlassHover)
	}
	ink := portrait.Colors.Ink2
	if item.selected || row.Hovered() {
		ink = portrait.Colors.Ink
	}
	row.Children(func() {
		portrait.RowWash(c, row, item.selected, item.group)
		ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Start).ZIndex(1).Children(func() {
			portrait.Glyph(c, item.icon, portrait.Type.Small.Size, portrait.Colors.Ink2).Margin(2, 0, 0, 0)
			ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Start).Children(func() {
				portrait.Type.UI.Apply(ui.Text(c, item.title)).MaxLines(1).Ellipsis("…").TextColor(ink)
				if item.excerpt != "" {
					portrait.Type.Meta.Apply(ui.Text(c, item.excerpt)).MaxLines(2).Ellipsis("…").TextColor(portrait.Colors.Ink2)
				}
			})
			if item.trailing != "" {
				portrait.Type.Meta.Apply(ui.Text(c, item.trailing)).TextColor(portrait.Colors.Ink2).Shrink(0).Margin(1, 0, 0, 0).AccessHidden(true)
			}
		})
	})
	return row
}
