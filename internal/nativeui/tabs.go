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

const tabStatusDotSize = 6

type documentState struct {
	label string
	tone  portrait.Tone
}

func documentStates(edited, conflict, deleted, readOnly, generating bool) []documentState {
	var states []documentState
	if edited {
		states = append(states, documentState{"Unsaved", portrait.Tones.Warning})
	}
	if conflict {
		states = append(states, documentState{"Conflict", portrait.Tones.Danger})
	}
	if deleted {
		states = append(states, documentState{"Deleted", portrait.Tones.Danger})
	}
	if readOnly {
		states = append(states, documentState{"Read only", portrait.Tones.Neutral})
	}
	if generating {
		states = append(states, documentState{"Generating", portrait.Tones.Accent})
	}
	return states
}

func joinStates(states []documentState) string {
	tags := make([]string, len(states))
	for i, state := range states {
		tags[i] = state.label
	}
	return strings.Join(tags, " · ")
}

func stateTone(states []documentState) portrait.Tone {
	for _, state := range states {
		if state.tone == portrait.Tones.Danger {
			return state.tone
		}
	}
	if len(states) == 0 {
		return portrait.Tones.Neutral
	}
	return states[0].tone
}

type closeRevealKey struct{}

func toolbarToggle(c *ui.Context, icon portrait.Icon, label string, on bool) ui.Element {
	return portrait.ButtonWith(c, "", label, portrait.ButtonSpec{Look: portrait.Quiet, Leading: icon, Selected: on}).Expanded(on)
}

func (s *Shell) tabs(c *ui.Context, width, available float32, m Model, a Actions) {
	docs := m.Workspace.Documents
	labels := tabLabels(docs)
	active := slices.IndexFunc(docs, func(d workspace.DocumentInfo) bool { return d.ID == s.activeID })
	room := available - 48 - 42
	if m.Feynman {
		room -= 38
	}
	const minCell, maxCell, overflowRoom = 136, 220, 53
	count := min(len(docs), max(1, int(room/minCell)))
	extra := float32(0)
	if count < len(docs) {
		count = max(1, int((room-overflowRoom)/minCell))
		extra = overflowRoom
	}
	cell := clamp((room-extra)/float32(count)-3, minCell, maxCell)
	start := max(0, active-count+1)
	end := min(len(docs), start+count)
	ui.Row(c).Height(42).Padding(8, 8, 2, 8).Gap(6).AlignItems(ui.Center).Children(func() {
		navLabel, navVisible := "Toggle navigation", !s.hideNav
		if width < 641 {
			navLabel, navVisible = "Show navigation", s.leftOpen
		}
		toggle := toolbarToggle(c, portrait.IconSidebar, navLabel, navVisible).KeyShortcuts(ui.Cmd|ui.Ctrl, ui.KeyS).Bind(&s.leftToggle)
		toggle.ObserveInput(func(ev ui.InputEvent) {
			if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
				s.leftKeys.Flip()
				s.rightKeys.Flip()
			}
		})
		if toggle.Clicked() {
			s.toggleNavigation(width)
		}
		portrait.Track(c, "document tabs", func(t *portrait.Segments) {
			for i := start; i < end; i++ {
				d := docs[i]
				selected := d.ID == s.activeID
				states := documentStates(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating)
				status := joinStates(states)
				cellRow := ui.Row(c.Key(d.ID)).Radius(portrait.Radii.Segment).MaxWidth(cell).MinWidth(0).Height(portrait.Sizes.Regular).AlignItems(ui.Center).Padding(0, 4, 0, 0)
				hovered := cellRow.Hovered()
				portrait.RowHover(cellRow, hovered && !selected)
				cellRow.Children(func() {
					tab := portrait.Tooltip(c, ui.ButtonBase(c.Key("tab")).Label(labels[d.ID]).Height(portrait.Sizes.Regular).Padding(0, 4, 0, 8).Gap(6).AlignItems(ui.Center).MinWidth(0).Shrink(1).ZIndex(1).Role(ui.RoleTab).TabStop(selected).AccessSelected(selected), d.Path)
					if status != "" {
						tab.Description(status)
					}
					ink := portrait.Colors.Ink2
					if selected || hovered {
						ink = portrait.Colors.Ink
					}
					tab.Children(func() {
						portrait.Glyph(c, portrait.IconFileText, 12, portrait.Colors.Ink3)
						portrait.Type.UI.Apply(ui.Text(c, labels[d.ID])).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
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
					for _, state := range states {
						dot := ui.Box(c.Key("status-"+state.label)).Size(tabStatusDotSize, tabStatusDotSize).
							Radius(portrait.Radii.Pill).Background(state.tone.Solid).
							Role(ui.RoleText).Label(state.label).Shrink(0).ZIndex(1)
						portrait.Tooltip(c, dot, state.label)
					}
					closeTab := portrait.QuietIconButton(c, portrait.IconX, "Close "+labels[d.ID]).Size(20, 20).Radius(portrait.Radii.S).ZIndex(1)
					reveal := float32(0)
					if hovered || closeTab.Hovered() || closeTab.FocusVisible() {
						reveal = 1
					} else if selected {
						reveal = 0.55
					}
					if eased := closeTab.AnimateTo(closeRevealKey{}, reveal, ui.Animation{Duration: portrait.Motion.Expand, Ease: portrait.Motion.EaseOut.Easing()}); portrait.Animates(c, c.KeyboardInput()) {
						reveal = eased
					}
					closeTab.Opacity(reveal)
					if closeTab.Clicked() && a.CloseDocument != nil {
						a.CloseDocument(d.ID)
					}
					t.Capsule(c, tab, selected)
				})
			}
			if count < len(docs) {
				button := portrait.ButtonWith(c, fmt.Sprintf("+%d", len(docs)-count), "More documents", portrait.ButtonSpec{Look: portrait.Quiet, Selected: s.overflow}).Padding(0, 8).MinWidth(32).Expanded(s.overflow)
				if button.Clicked() {
					s.overflow = !s.overflow
					s.tabQuery = ""
					s.focusFilter = s.overflow
				}
				pop := portrait.Popover(c, button, &s.overflow, func() {
					ui.Column(c).Width(min(340, width-40)).Gap(5).Padding(1).Children(func() {
						var filter ui.Element
						portrait.InputFrame(c).Children(func() {
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
		}).Label("Documents").Height(32).Gap(3).Shrink(1).MinWidth(0).Clip().MaskLinearGradient(scrollEdgeMask(90))
		ui.Row(c).Grow(1)
		portrait.QuietIconButton(c, portrait.IconPlugs, "Plugins").Menu(func(menu *ui.Menu) {
			if menu.Item("Feynman").Checked(m.Feynman).Chosen() && a.SetFeynman != nil {
				a.SetFeynman(!m.Feynman)
			}
			if menu.Item("Git").Checked(m.Git).Chosen() && a.SetGit != nil {
				a.SetGit(!m.Git)
			}
		})
		if m.Feynman {
			visible := s.rightOpen || width >= 1180 && !s.hideResearch
			toggle := toolbarToggle(c, portrait.IconSidebar.Mirrored(), "Toggle research", visible).KeyShortcuts(ui.Cmd|ui.Alt, ui.Key0).Bind(&s.rightToggle)
			toggle.ObserveInput(func(ev ui.InputEvent) {
				if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
					s.leftKeys.Flip()
					s.rightKeys.Flip()
				}
			})
			if toggle.Clicked() {
				s.toggleResearch(width)
			}
		}
	})
}
