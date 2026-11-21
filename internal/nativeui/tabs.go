package nativeui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func tabLabels(docs []workspace.DocumentInfo) map[string]string {
	groups := make(map[string][]workspace.DocumentInfo)
	for _, d := range docs {
		groups[d.Name] = append(groups[d.Name], d)
	}
	out := make(map[string]string, len(docs))
	dirs := func(path string) []string {
		parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
		if len(parts) == 0 {
			return nil
		}
		return parts[:len(parts)-1]
	}
	for _, group := range groups {
		for _, doc := range group {
			out[doc.ID] = doc.Name
			if len(group) == 1 {
				continue
			}
			if doc.Path != "" {
				out[doc.ID] = doc.Path
			}
			parts := dirs(doc.Path)
			for n := 1; n <= len(parts); n++ {
				suffix := strings.Join(parts[len(parts)-n:], "/")
				if slices.ContainsFunc(group, func(other workspace.DocumentInfo) bool {
					if other.ID == doc.ID {
						return false
					}
					p := dirs(other.Path)
					return strings.Join(p[max(0, len(p)-n):], "/") == suffix
				}) {
					continue
				}
				out[doc.ID] = doc.Name + " — " + suffix
				break
			}
		}
	}
	return out
}

func stateLabel(edited, conflict, deleted, readOnly, generating bool) string {
	var tags []string
	if edited {
		tags = append(tags, "Unsaved")
	}
	if conflict {
		tags = append(tags, "Conflict")
	}
	if deleted {
		tags = append(tags, "Deleted")
	}
	if readOnly {
		tags = append(tags, "Read only")
	}
	if generating {
		tags = append(tags, "Generating")
	}
	return strings.Join(tags, " · ")
}

func (s *Shell) tabs(c *ui.Context, width, available float32, m Model, a Actions) {
	docs := m.Workspace.Documents
	labels := tabLabels(docs)
	active := slices.IndexFunc(docs, func(d workspace.DocumentInfo) bool { return d.ID == s.activeID })
	room := available - 48
	if width < 641 {
		room -= 42
	}
	if m.Feynman {
		room -= 38
	}
	const cell = 136
	count := min(len(docs), max(1, int(room/cell)))
	if count < len(docs) {
		count = max(1, int((room-53)/cell))
	}
	start := max(0, active-count+1)
	end := min(len(docs), start+count)
	ui.Row(c).Height(42).Padding(8, 8, 2, 8).Gap(6).AlignItems(ui.Center).Children(func() {
		if width < 641 {
			toggle := portrait.IconButton(c, portrait.IconSidebar, "Show navigation").Expanded(s.leftOpen).Bind(&s.leftToggle)
			if toggle.Clicked() {
				s.leftOpen = !s.leftOpen
				s.rightOpen = false
			}
		}
		portrait.Track(c, "document tabs", func(t *portrait.Segments) {
			for i := start; i < end; i++ {
				d := docs[i]
				selected := d.ID == s.activeID
				status := stateLabel(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating)
				title := labels[d.ID]
				if status != "" {
					title += " · " + status
				}
				cellRow := ui.Row(c.Key(d.ID)).Radius(7).MaxWidth(cell).MinWidth(0).Height(28).AlignItems(ui.Center).Padding(0, 4, 0, 0)
				hovered := cellRow.Hovered()
				if hovered && !selected {
					portrait.Materials.Row.Apply(cellRow)
				}
				cellRow.Children(func() {
					tab := ui.ButtonBase(c.Key("tab")).Label(labels[d.ID]).Height(28).Padding(0, 4, 0, 8).Gap(6).AlignItems(ui.Center).MinWidth(0).Shrink(1).ZIndex(1).Role(ui.RoleTab).TabStop(selected).AccessSelected(selected).Description(status).Tooltip(d.Path)
					ink := portrait.Colors.Ink2
					if selected || hovered {
						ink = portrait.Colors.Ink
					}
					tab.Children(func() {
						portrait.Glyph(c, portrait.IconFileText, 12, portrait.Colors.Ink3)
						portrait.Type.UI.Apply(ui.Text(c, title)).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
					})
					if s.tabFocus == d.ID && selected {
						tab.Focus()
						s.tabFocus = ""
						s.focusedTab = true
					}
					if tab.Clicked() && a.Activate != nil {
						a.Activate(d.ID)
					}
					if tab.Focused() && len(docs) > 1 && a.Activate != nil {
						switch {
						case tab.Shortcut(0, ui.KeyRight):
							s.tabFocus = docs[(i+1)%len(docs)].ID
							a.Activate(s.tabFocus)
						case tab.Shortcut(0, ui.KeyLeft):
							s.tabFocus = docs[(i+len(docs)-1)%len(docs)].ID
							a.Activate(s.tabFocus)
						case tab.Shortcut(0, ui.KeyHome):
							s.tabFocus = docs[0].ID
							a.Activate(s.tabFocus)
						case tab.Shortcut(0, ui.KeyEnd):
							s.tabFocus = docs[len(docs)-1].ID
							a.Activate(s.tabFocus)
						}
					}
					closeTab := portrait.QuietIconButton(c, portrait.IconX, "Close "+labels[d.ID]).Size(20, 20).Radius(6).ZIndex(1)
					if !hovered && !closeTab.Hovered() {
						closeTab.Opacity(0.55)
					}
					if closeTab.Clicked() && a.CloseDocument != nil {
						a.CloseDocument(d.ID)
					}
					t.Capsule(tab, selected)
				})
			}
			if count < len(docs) {
				button := portrait.Button(c, fmt.Sprintf("+%d", len(docs)-count), "More documents").Height(26).Padding(0, 8).MinWidth(32).Expanded(s.overflow)
				if button.Clicked() {
					s.overflow = !s.overflow
					s.tabQuery = ""
					s.focusFilter = s.overflow
				}
				pop := portrait.Popover(c, button, &s.overflow, func() {
					ui.Column(c).Width(min(340, width-40)).Gap(5).Padding(1).Children(func() {
						var filter ui.Element
						portrait.Field(c).Children(func() {
							portrait.Glyph(c, portrait.IconMagnifyingGlass, 13, portrait.Colors.Ink3)
							filter = portrait.Type.UI.Apply(ui.TextInputBase(c, &s.tabQuery)).FocusRing(false).Label("Filter open tabs").Placeholder("Filter open tabs").Grow(1).MinWidth(0)
						})
						if s.focusFilter {
							filter.Focus()
							s.focusFilter = false
						}
						ui.Scroll(c).MaxHeight(300).Children(func() {
							hidden := make([]workspace.DocumentInfo, 0, len(docs)-count)
							for i, d := range docs {
								if (i < start || i >= end) && strings.Contains(strings.ToLower(labels[d.ID]+" "+d.Path), strings.ToLower(strings.TrimSpace(s.tabQuery))) {
									hidden = append(hidden, d)
								}
							}
							rank := func(id string) int {
								if i := slices.Index(s.history, id); i >= 0 {
									return i
								}
								return len(s.history)
							}
							slices.SortStableFunc(hidden, func(a, b workspace.DocumentInfo) int { return rank(a.ID) - rank(b.ID) })
							next := filter.Shortcut(0, ui.KeyDown)
							for i, d := range hidden {
								item := portrait.QuietButton(c, labels[d.ID], labels[d.ID]).FillWidth().Justify(ui.Start).Padding(0, 8)
								if i == 0 && next {
									item.Focus()
								}
								if item.Clicked() || (i == 0 && filter.Submitted()) {
									s.overflow = false
									s.tabFocus = d.ID
									if a.Activate != nil {
										a.Activate(d.ID)
									}
								}
							}
							if len(hidden) == 0 {
								portrait.Type.UI.Apply(ui.Text(c, "No matching documents")).TextColor(portrait.Colors.Ink3).Padding(4, 8)
							}
						})
					})
				})
				if pop.Dismissed() {
					button.Focus()
				}
			}
		}).Label("Documents").Height(32).Gap(3).Shrink(1).MinWidth(0).Clip()
		ui.Row(c).Grow(1)
		portrait.IconButton(c, portrait.IconPlugs, "Plugins").Menu(func(menu *ui.Menu) {
			if menu.Item("Feynman").Checked(m.Feynman).Chosen() && a.SetFeynman != nil {
				a.SetFeynman(!m.Feynman)
			}
			if menu.Item("Git").Checked(m.Git).Chosen() && a.SetGit != nil {
				a.SetGit(!m.Git)
			}
		})
		if m.Feynman {
			visible := s.rightOpen || width >= 1180 && !s.hideResearch
			toggle := portrait.IconButton(c, portrait.IconSidebar.Mirrored(), "Toggle research").Expanded(visible).KeyShortcuts(ui.Cmd|ui.Alt, ui.Key0).Bind(&s.rightToggle)
			if toggle.Clicked() {
				s.toggleResearch(width)
			}
		}
	})
}
