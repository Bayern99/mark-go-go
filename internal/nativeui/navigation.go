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

func (s *Shell) navigation(c *ui.Context, m Model, a Actions, docked bool) {
	w := m.Workspace
	panel(c, docked).Role(ui.RoleNavigation).Label("Workspace navigation").Children(func() {
		name := w.Name
		if name == "" {
			name = "Untitled"
		}
		switcher := ui.ButtonBase(c.Key("Switch workspace")).Label("Switch workspace").Value(name).FillWidth().MinHeight(26).Padding(2, 8).Gap(7).Radius(portrait.Radii.S).AlignItems(ui.Center)
		switcher.Children(func() {
			portrait.Shell.WorkspaceName.Apply(ui.Text(c, name)).FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…").MinWidth(0).Shrink(1)
			ui.Row(c).Grow(1)
			portrait.Glyph(c, portrait.IconCaretDown, 11, portrait.Colors.Ink3)
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
		})
		meta := w.Root
		if s.outline {
			meta = activeDocument(w).Name
		} else if w.Kind == "folder" {
			meta = fmt.Sprintf("%s · %d files", meta, m.Navigation.Total)
		} else {
			meta = "Untitled window"
		}
		portrait.Tooltip(c, portrait.Type.Meta.Apply(ui.Text(c, meta)).TextColor(portrait.Colors.Ink3).MaxLines(1).Ellipsis("…"), meta).Padding(0, 8)
		portrait.Track(c, "navigation", func(t *portrait.Segments) {
			for _, title := range []string{"Workspace", "Outline"} {
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
		}).Label("Navigation views").Margin(2, 0, 6, 0)
		if s.outline {
			scroll := s.scrollState("outline:" + w.ActiveID)
			ui.Scroll(c.Key("outline:" + w.ActiveID)).FillWidth().Grow(1).TrackScroll(scroll).Label("Document outline").MaskLinearGradient(scrollEdgeMask(180)).Children(func() {
				for _, section := range m.Sections {
					if navRow(c, navItem{key: fmt.Sprintf("heading-%d", section.Start), title: section.Heading, depth: section.Level - 1, icon: portrait.IconFileText}).Clicked() && a.Reveal != nil {
						s.leftOpen = false
						a.Reveal(section.Start)
					}
				}
				if len(m.Sections) == 0 {
					ui.Text(c, "No headings").TextColor(portrait.Colors.Ink3)
				}
			})
			return
		}
		if w.Kind == "folder" {
			portrait.InputFrame(c).Margin(4, 2, 8, 2).Children(func() {
				if portrait.Type.UI.Apply(ui.TextInputBase(c.Key("filter:"+w.Root), &s.query)).FocusRing(false).Grow(1).MinWidth(0).Placeholder("Filter files and headings").Label("Filter files and headings").Changed() && a.Filter != nil {
					s.queries[w.Root] = s.query
					a.Filter(s.query)
				}
				if s.query != "" && portrait.QuietIconButton(c, portrait.IconX, "Clear filter").Size(20, 20).Radius(portrait.Radii.Pill).Clicked() {
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
		ui.Scroll(c.Key(scrollKey)).Grow(1).FillWidth().Role(ui.RoleTree).FocusGroup(ui.Vertical).Label("Workspace files").TrackScroll(s.scrollState(scrollKey)).MaskLinearGradient(scrollEdgeMask(180)).Children(func() {
			if strings.TrimSpace(s.query) != "" {
				if strings.TrimSpace(s.query) != m.Search.Query {
					return
				}
				for _, hit := range m.Search.Hits {
					if navRow(c, navItem{key: hit.Path, title: hit.Name, description: hit.Excerpt, excerpt: hit.Excerpt, selected: hit.DocumentID == w.ActiveID, icon: portrait.IconFileText, group: capsuleGroup}).Clicked() && a.OpenPath != nil {
						s.leftOpen = false
						a.OpenPath(hit.Path, -1)
					}
					for _, heading := range hit.Headings {
						if navRow(c, navItem{key: fmt.Sprintf("%s-%d", hit.Path, heading.Offset), title: heading.Text, depth: 1, icon: portrait.IconFileText}).Clicked() && a.OpenPath != nil {
							s.leftOpen = false
							a.OpenPath(hit.Path, heading.Offset)
						}
					}
				}
				if len(m.Search.Hits) == 0 {
					ui.Text(c, "No matching files or headings").TextColor(portrait.Colors.Ink3)
				}
				return
			}
			if w.Kind != "folder" {
				for _, d := range w.Documents {
					if d.WorkspaceRoot != "" {
						continue
					}
					if navRow(c, navItem{key: d.ID, title: d.Name, description: d.Path, states: documentStates(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating), selected: d.ID == w.ActiveID, icon: portrait.IconFileText, group: capsuleGroup}).Clicked() && a.Activate != nil {
						s.leftOpen = false
						a.Activate(d.ID)
					}
				}
				return
			}
			for _, entry := range m.Navigation.Entries {
				folder := entry.Kind == "folder"
				item := navItem{key: entry.ID, title: entry.Name, description: entry.Path, depth: entry.Depth, selected: entry.DocumentID != "" && entry.DocumentID == w.ActiveID, icon: portrait.IconFileText, group: capsuleGroup,
					states: documentStates(entry.Edited, entry.Conflict, entry.Deleted, false, false)}
				if folder {
					item.icon, item.count, item.description = portrait.IconFolder, fmt.Sprint(entry.MarkdownCount), fmt.Sprintf("%d files", entry.MarkdownCount)
					if entry.FolderState == workspace.FolderOpen {
						item.icon = portrait.IconFolderOpen
					}
				}
				button := navRow(c, item)
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
				ui.Text(c, fmt.Sprintf("Indexing %d of %d files", m.Navigation.Indexed, m.Navigation.Total)).Role(ui.RoleStatus).TextColor(portrait.Colors.Ink3)
			} else if len(m.Navigation.Entries) == 0 {
				ui.Text(c, "No Markdown files").TextColor(portrait.Colors.Ink3)
			}
		})
	})
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
	key, title, description, excerpt, count string
	states                                  []documentState
	depth                                   int
	selected                                bool
	icon                                    portrait.Icon
	group                                   string
}

func navRow(c *ui.Context, item navItem) ui.Element {
	description := item.description
	if status := joinStates(item.states); status != "" {
		description = status
	}
	row := ui.ButtonBase(c.Key(item.key)).FillWidth().Role(ui.RoleTreeItem).Label(item.title).Description(description).AccessSelected(item.selected).Padding(4, 8, 4, float32(8+12*min(item.depth, 8))).Radius(portrait.Radii.M)
	portrait.RowHover(row, !item.selected && row.Hovered())
	ink := portrait.Colors.Ink2
	if item.selected || row.Hovered() {
		ink = portrait.Colors.Ink
	}
	row.Children(func() {
		portrait.RowCapsule(c, row, item.selected, item.group)
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Start).ZIndex(1).Children(func() {
			portrait.Glyph(c, item.icon, 14, portrait.Colors.Ink3).Margin(2, 0, 0, 0)
			ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Start).Children(func() {
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					portrait.Type.UI.Apply(ui.Text(c, item.title)).MaxLines(1).Ellipsis("…").TextColor(ink).MinWidth(0).Shrink(1)
					ui.Row(c).Grow(1)
					if len(item.states) > 0 {
						status := joinStates(item.states)
						portrait.Type.Meta.Apply(ui.Text(c, status)).MaxLines(1).TextColor(stateTone(item.states).Ink).Shrink(0)
					} else if item.count != "" {
						portrait.Type.Meta.Apply(ui.Text(c, item.count)).MaxLines(1).TextColor(portrait.Colors.Ink3).Shrink(0)
					}
				})
				if item.excerpt != "" {
					portrait.Type.Meta.Apply(ui.Text(c, item.excerpt)).MaxLines(1).Ellipsis("…").TextColor(portrait.Colors.Ink3)
				}
			})
		})
	})
	return row
}
