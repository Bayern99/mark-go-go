package nativeui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func (s *Shell) navigation(c *ui.Context, m Model, a Actions) {
	w := m.Workspace
	panel(c).Role(ui.RoleNavigation).Label("Workspace navigation").Children(func() {
		name := w.Name
		if name == "" {
			name = "Untitled"
		}
		control(c, name, "Switch workspace").FillWidth().Menu(func(menu *ui.Menu) {
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
		ui.Text(c, meta).FontSize(11).TextColor(muted).MaxLines(1).Ellipsis("…").Tooltip(meta)
		ui.Row(c).Role(ui.RoleTabList).Label("Navigation views").Padding(2).Gap(2).Radius(9).Background(track).Children(func() {
			for _, title := range []string{"Workspace", "Outline"} {
				outline := title == "Outline"
				button := discrete(c, title, title, s.outline == outline, "navigation").Grow(1).Role(ui.RoleTab).AccessSelected(s.outline == outline)
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
		})
		if s.outline {
			scroll := s.scrollState("outline:" + w.ActiveID)
			ui.Scroll(c.Key("outline:" + w.ActiveID)).FillWidth().Grow(1).TrackScroll(scroll).Label("Document outline").Children(func() {
				for _, section := range m.Sections {
					if navRow(c, fmt.Sprintf("heading-%d", section.Start), section.Heading, "", section.Level-1, false, false).Clicked() && a.Reveal != nil {
						s.leftOpen = false
						a.Reveal(section.Start)
					}
				}
				if len(m.Sections) == 0 {
					ui.Text(c, "No headings").TextColor(muted)
				}
			})
			return
		}
		if w.Kind == "folder" {
			ui.Row(c).Padding(2).Radius(8).Background(track).Children(func() {
				if ui.TextInput(c.Key("filter:"+w.Root), &s.query).Grow(1).MinWidth(0).Placeholder("Filter files and headings").Label("Filter files and headings").Changed() && a.Filter != nil {
					s.queries[w.Root] = s.query
					a.Filter(s.query)
				}
				if s.query != "" && control(c, "×", "Clear filter").Clicked() {
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
		ui.Scroll(c.Key(scrollKey)).Grow(1).FillWidth().Role(ui.RoleTree).FocusGroup(ui.Vertical).Label("Workspace files").TrackScroll(s.scrollState(scrollKey)).Children(func() {
			if strings.TrimSpace(s.query) != "" {
				if strings.TrimSpace(s.query) != m.Search.Query {
					return
				}
				for _, hit := range m.Search.Hits {
					if navRow(c, hit.Path, hit.Name, hit.Excerpt, 0, hit.DocumentID == w.ActiveID, false).Clicked() && a.OpenPath != nil {
						s.leftOpen = false
						a.OpenPath(hit.Path, -1)
					}
					for _, heading := range hit.Headings {
						if navRow(c, fmt.Sprintf("%s-%d", hit.Path, heading.Offset), heading.Text, "", 1, false, false).Clicked() && a.OpenPath != nil {
							s.leftOpen = false
							a.OpenPath(hit.Path, heading.Offset)
						}
					}
				}
				if len(m.Search.Hits) == 0 {
					ui.Text(c, "No matching files or headings").TextColor(muted)
				}
				return
			}
			if w.Kind != "folder" {
				for _, d := range w.Documents {
					if d.WorkspaceRoot != "" {
						continue
					}
					if navRow(c, d.ID, d.Name, stateLabel(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating), 0, d.ID == w.ActiveID, false).Clicked() && a.Activate != nil {
						s.leftOpen = false
						a.Activate(d.ID)
					}
				}
				return
			}
			for _, entry := range m.Navigation.Entries {
				folder := entry.Kind == "folder"
				detail := entry.Parent
				if detail == "" {
					detail = entry.Path
				}
				if status := stateLabel(entry.Edited, entry.Conflict, entry.Deleted, false, false); status != "" {
					detail = status
				}
				if folder {
					detail = fmt.Sprintf("%d files", entry.MarkdownCount)
				}
				button := navRow(c, entry.ID, entry.Name, detail, entry.Depth, entry.DocumentID != "" && entry.DocumentID == w.ActiveID, folder)
				if folder {
					button.Expanded(entry.FolderState == workspace.FolderOpen)
				}
				if button.Clicked() {
					if folder {
						if rel, err := filepath.Rel(w.Root, entry.Path); err == nil && a.ToggleFolder != nil {
							a.ToggleFolder(filepath.ToSlash(rel))
						}
					} else if entry.DocumentID != "" && a.Activate != nil {
						s.leftOpen = false
						a.Activate(entry.DocumentID)
					} else if a.OpenPath != nil {
						s.leftOpen = false
						a.OpenPath(entry.Path, -1)
					}
				}
			}
			if m.Navigation.Error != "" {
				banner(c, m.Navigation.Error)
			} else if m.Navigation.Status == workspace.IndexScanning || m.Navigation.Status == workspace.IndexIndexing {
				ui.Text(c, fmt.Sprintf("Indexing %d of %d files", m.Navigation.Indexed, m.Navigation.Total)).Role(ui.RoleStatus).TextColor(muted)
			} else if len(m.Navigation.Entries) == 0 {
				ui.Text(c, "No Markdown files").TextColor(muted)
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

func navRow(c *ui.Context, key, title, detail string, depth int, selected, folder bool) ui.Element {
	row := ui.ButtonBase(c.Key(key)).FillWidth().Role(ui.RoleTreeItem).Label(title).Description(detail).AccessSelected(selected).Padding(4, 5, 4, float32(5+12*min(depth, 8))).Radius(7)
	if selected {
		row.Background(paper).Border(0.5, rim)
	} else if row.Hovered() {
		row.Background(track)
	}
	row.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			glyph := "\ued50"
			if folder {
				glyph = "\ue24a"
			}
			ui.Text(c, glyph).Font("Phosphor").FontSize(15).TextColor(muted).AccessHidden(true)
			ui.Column(c).MinWidth(0).Grow(1).Children(func() {
				ui.Text(c, title).Font(bodyFont(title)).MaxLines(1).Ellipsis("…").TextColor(ink)
				if detail != "" {
					ui.Text(c, detail).Font(bodyFont(detail)).MaxLines(1).Ellipsis("…").FontSize(11).TextColor(muted)
				}
			})
		})
	})
	return row
}
