package nativeui

import (
	"fmt"
	"slices"
	"strings"

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
	ui.Row(c).Height(42).Padding(5, 9).Gap(5).AlignItems(ui.Center).Children(func() {
		if width < 641 {
			toggle := iconButton(c, "\uec24", "Show navigation").Width(33).Expanded(s.leftOpen).Bind(&s.leftToggle)
			if toggle.Clicked() {
				s.leftOpen = !s.leftOpen
				s.rightOpen = false
			}
		}
		ui.Row(c).Role(ui.RoleTabList).Label("Documents").Padding(2).Radius(9).Background(track).Grow(1).MinWidth(0).Gap(3).AlignItems(ui.Center).Children(func() {
			for i := start; i < end; i++ {
				d := docs[i]
				title := labels[d.ID]
				status := stateLabel(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating)
				if status != "" {
					title += " · " + status
				}
				ui.Row(c.Key(d.ID)).Radius(7).MaxWidth(cell).MinWidth(0).Height(29).AlignItems(ui.Center).Background(tabBackground(d.ID == s.activeID)).Children(func() {
					tab := discrete(c, title, labels[d.ID], d.ID == s.activeID, "document tabs").MinWidth(0).Shrink(1).Role(ui.RoleTab).TabStop(d.ID == s.activeID).AccessSelected(d.ID == s.activeID).Description(status).Tooltip(d.Path)
					if s.tabFocus == d.ID && d.ID == s.activeID {
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
					if iconButton(c, "\ue4f6", "Close "+labels[d.ID]).Size(23, 23).Padding(0).Clicked() && a.CloseDocument != nil {
						a.CloseDocument(d.ID)
					}
				})
			}
			if count < len(docs) {
				button := control(c, fmt.Sprintf("+%d", len(docs)-count), "More documents").Expanded(s.overflow)
				if button.Clicked() {
					s.overflow = !s.overflow
					s.tabQuery = ""
					s.focusFilter = s.overflow
				}
				pop := glassPopover(c, button, &s.overflow, func() {
					ui.Column(c).Width(min(340, width-40)).Gap(5).Padding(6).Children(func() {
						filter := ui.TextInput(c, &s.tabQuery).Label("Filter open tabs").Placeholder("Filter open tabs")
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
								item := discrete(c, labels[d.ID], labels[d.ID], false)
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
								ui.Text(c, "No matching documents").TextColor(muted)
							}
						})
					})
				})
				if pop.Dismissed() {
					button.Focus()
				}
			}
		})
		iconButton(c, "\ueb56", "Plugins").Menu(func(menu *ui.Menu) {
			if menu.Item("Feynman").Checked(m.Feynman).Chosen() && a.SetFeynman != nil {
				a.SetFeynman(!m.Feynman)
			}
			if menu.Item("Git").Checked(m.Git).Chosen() && a.SetGit != nil {
				a.SetGit(!m.Git)
			}
		})
		if m.Feynman {
			visible := s.rightOpen || width >= 1180 && !s.hideResearch
			toggle := iconButton(c, "\uec24", "Toggle research").Expanded(visible).KeyShortcuts(ui.Cmd|ui.Alt, ui.Key0).Bind(&s.rightToggle)
			if toggle.Clicked() {
				s.toggleResearch(width)
			}
		}
	})
}

func tabBackground(selected bool) ui.Color {
	if selected {
		return paper
	}
	return track
}
