package nativeui

import (
	"sort"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (s *Shell) settingsDialog(c *ui.Context, width, height float32, m Model, a Actions, slots Slots) {
	pages := mergeSettingsPages(append(s.nativeSettingsPages(m, a), settingsPages(slots)...))
	visible := make([]SettingsPage, 0, len(pages))
	for _, page := range pages {
		if page.Visible == nil || page.Visible() {
			visible = append(visible, page)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool { return visible[i].Order < visible[j].Order })
	if len(visible) == 0 {
		s.settingsOpen = false
		return
	}
	if !settingsPageVisible(visible, s.settingsTab) {
		s.settingsTab = visible[0].ID
	}
	if s.settingsNav == nil {
		s.settingsNav = map[string]*ui.Handle{}
	}
	if s.settingsOpen && !s.settingsEditing && (c.Shortcut(0, ui.KeySlash) || c.Shortcut(ui.Cmd, ui.KeyF)) {
		s.settingsFocus = true
	}
	searchState := &settingsSearchState{}
	query := SettingsQuery{Text: s.settingsQuery, state: searchState}
	current := settingsPageByID(visible, s.settingsTab)
	panel := portrait.DialogShell(c, "settings dialog", &s.settingsOpen, portrait.DialogSpec{
		Label: "Settings", Width: portrait.Settings.Width, Height: portrait.Settings.Height,
		Margin: portrait.Settings.Margin, Radius: portrait.Settings.Radius, Glass: true,
	}, func() {
		sheetWidth := min(portrait.Settings.Width, max(0, width-2*portrait.Settings.Margin))
		root := ui.Row(c).Fill().MinWidth(0)
		root.Children(func() {
			if sheetWidth >= portrait.Settings.Collapse {
				s.settingsRail(c, visible, query)
				s.settingsPane(c, current, visible, query, searchState, true)
			} else {
				ui.Column(c).Fill().MinWidth(0).Children(func() {
					ui.Row(c).FillWidth().Height(portrait.Settings.Header).Padding(0, portrait.Form.PadX).Gap(portrait.Form.Gap).
						BorderWidth(0, 0, portrait.Editor.Hairline, 0).BorderColor(portrait.Colors.Line).AlignItems(ui.Center).Children(func() {
						selected := s.settingsTab
						options := make([]portrait.Option, 0, len(visible))
						for _, page := range visible {
							options = append(options, portrait.Option{Value: page.ID, Label: page.Label})
						}
						before := selected
						portrait.Select(c, &selected, options, portrait.SelectSpec{Label: "Settings category", Width: portrait.Form.MenuMin})
						if selected != before {
							s.settingsTab, s.settingsQuery = selected, ""
						}
						s.settingsSearch(c, visible).Frame().Grow(1).MinWidth(0)
					})
					s.settingsPane(c, current, visible, query, searchState, false)
				})
			}
		})
	})
	_ = panel
	if s.settingsResetOpen {
		page := settingsPageByID(visible, s.settingsResetID)
		label := page.Label
		if s.settingsResetID == "all" {
			label = "all settings"
		}
		portrait.Confirm(c, &s.settingsResetOpen, portrait.ConfirmSpec{
			Title: "Reset " + label + "?", Body: "Restore the defaults for " + label + ".",
			ConfirmLabel: "Reset to defaults", Return: &s.settingsReset,
			OnConfirm: func() {
				if s.settingsResetID == "all" && a.ResetPreferencePrefixes != nil {
					a.ResetPreferencePrefixes("mgo.", "feynman.")
				} else if page.Reset != nil {
					page.Reset()
				}
			},
		})
	}
}

func settingsPageVisible(pages []SettingsPage, id string) bool {
	for _, page := range pages {
		if page.ID == id {
			return true
		}
	}
	return false
}

func settingsPageByID(pages []SettingsPage, id string) SettingsPage {
	for _, page := range pages {
		if page.ID == id {
			return page
		}
	}
	return SettingsPage{}
}

func (s *Shell) settingsSearch(c *ui.Context, pages []SettingsPage) portrait.Field {
	field := portrait.SearchField(c, &s.settingsQuery, portrait.SearchSpec{Label: "Search settings", Placeholder: "Search", Shortcut: []string{"/"}, Size: portrait.Sizes.Regular})
	field.Frame().Margin(portrait.Form.MenuPad, portrait.Form.PadX)
	if s.settingsFocus {
		field.Input().Focus()
		s.settingsFocus = false
	}
	field.Input().OnTextKey(func(key ui.TextKey) bool {
		switch key.Key {
		case ui.KeyEscape:
			if key.Text() != "" {
				key.SetText("")
				return true
			}
		case ui.KeyDown:
			s.settingsResults.Focus(c)
			return true
		case ui.KeyEnter:
			for _, page := range pages {
				if settingsPageMatchesText(page, key.Text()) {
					s.settingsTab = page.ID
					key.SetText("")
					return true
				}
			}
		}
		return false
	})
	s.settingsEditing = field.Frame().FocusWithin()
	return field
}

func settingsPageMatchesText(page SettingsPage, text string) bool {
	if strings.TrimSpace(text) == "" {
		return true
	}
	q := SettingsQuery{Text: text}
	return q.PageMatch(page)
}

func (s *Shell) settingsRail(c *ui.Context, pages []SettingsPage, query SettingsQuery) {
	rail := ui.Column(c).Width(portrait.Settings.Rail).FillHeight().Background(portrait.Colors.Fill).
		BorderWidth(0, 1, 0, 0).BorderColor(portrait.Colors.Line)
	rail.Children(func() {
		ui.Row(c).FillWidth().Height(portrait.Settings.Header).AlignItems(ui.Center).Children(func() { s.settingsSearch(c, pages) })
		nav := ui.Column(c).FillWidth().Grow(1).Padding(0, portrait.Form.PadX, portrait.Form.PadX, portrait.Form.PadX).Gap(portrait.Form.PopoverGap).
			Role(ui.RoleTabList).Label("Settings categories").FocusGroup(ui.Vertical)
		nav.Children(func() {
			lastSection := ""
			for i, page := range pages {
				if page.Section == "Footer" {
					continue
				}
				if page.Section != "" && page.Section != lastSection {
					portrait.SectionLabel(c, strings.ToUpper(page.Section)).Padding(portrait.Settings.NavSectionGap, portrait.Form.PadX, portrait.Form.HelpGap, portrait.Form.PadX)
				}
				lastSection = page.Section
				s.settingsNavItem(c, pages, i, page)
			}
			ui.Spacer(c)
			for i, page := range pages {
				if page.Section == "Footer" {
					s.settingsNavItem(c, pages, i, page)
				}
			}
		})
	})
	_ = query
}

func (s *Shell) settingsNavItem(c *ui.Context, pages []SettingsPage, index int, page SettingsPage) {
	selected := s.settingsTab == page.ID && s.settingsQuery == ""
	button := ui.ButtonBase(c.Key("settings-page-"+page.ID)).Role(ui.RoleTab).Label(page.Label).AccessSelected(selected).
		FillWidth().Height(portrait.Settings.NavRow).Padding(0, portrait.Form.PadX).Gap(portrait.Form.Gap).Radius(portrait.Radii.M).AlignItems(ui.Center).
		Bind(settingsHandle(s.settingsNav, page.ID))
	ink := portrait.Colors.Ink2
	if selected {
		button.Background(portrait.Colors.AccentSoft)
		ink = portrait.Colors.Accent
	} else if button.Hovered() {
		button.Background(portrait.Colors.FillHover)
		ink = portrait.Colors.Ink
	}
	button.Children(func() {
		portrait.Glyph(c, page.Icon, portrait.IconSizes.M, ink).Shrink(0)
		text := portrait.Type.UI.Apply(ui.Text(c, page.Label)).TextColor(ink).MaxLines(1).Ellipsis("…")
		if selected {
			text.FontWeight(500)
		}
	})
	if button.Clicked() {
		s.settingsTab, s.settingsQuery = page.ID, ""
	}
	if button.Focused() {
		next := -1
		switch {
		case button.Shortcut(0, ui.KeyDown):
			next = s.nextSettingsNav(pages, index, 1)
		case button.Shortcut(0, ui.KeyUp):
			next = s.nextSettingsNav(pages, index, -1)
		case button.Shortcut(0, ui.KeyHome):
			next = 0
		case button.Shortcut(0, ui.KeyEnd):
			next = len(pages) - 1
		}
		if next >= 0 && next < len(pages) {
			handle := s.settingsNav[pages[next].ID]
			if handle != nil {
				handle.Focus(c)
			}
		}
	}
}

func settingsHandle(handles map[string]*ui.Handle, id string) *ui.Handle {
	if handles[id] == nil {
		handles[id] = new(ui.Handle)
	}
	return handles[id]
}

func (s *Shell) nextSettingsNav(pages []SettingsPage, index, delta int) int {
	for next := index + delta; next >= 0 && next < len(pages); next += delta {
		if pages[next].Section != "Footer" || delta > 0 {
			return next
		}
	}
	return -1
}

func (s *Shell) settingsPane(c *ui.Context, current SettingsPage, pages []SettingsPage, query SettingsQuery, searchState *settingsSearchState, withRail bool) {
	pane := ui.Column(c).Grow(1).MinWidth(0).FillHeight()
	if !withRail {
		pane.FillWidth()
	}
	pane.Children(func() {
		scroll := s.scrollState("settings:" + s.settingsTab)
		header := ui.Row(c).FillWidth().Height(portrait.Settings.Header).Padding(0, portrait.Settings.ColumnPadX/2).Gap(portrait.Form.Gap).AlignItems(ui.Center)
		if scroll.Y > 0 {
			header.BorderWidth(0, 0, portrait.Editor.Hairline, 0).BorderColor(portrait.Colors.Line)
		}
		header.Children(func() {
			portrait.Type.UI.Apply(ui.Text(c, "Settings")).TextColor(portrait.Colors.Ink3)
			portrait.Type.UI.Apply(ui.Text(c, "/")).TextColor(portrait.Colors.Ink4)
			label := current.Label
			if query.Active() {
				label = "Search"
			}
			portrait.Type.UI.Apply(ui.Text(c, label)).FontWeight(600).TextColor(portrait.Colors.Ink).Role(ui.RoleHeading).Grow(1)
			if !query.Active() && current.Reset != nil && current.Dirty != nil && current.Dirty() {
				reset := portrait.ButtonWith(c, "Reset to defaults", "Reset "+current.Label+" to defaults", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Regular, Leading: portrait.IconArrowCounterClockwise}).Bind(&s.settingsReset)
				if reset.Clicked() {
					s.settingsResetID, s.settingsResetOpen = current.ID, true
				}
			}
			if portrait.QuietIconButton(c, portrait.IconX, "Close settings").Clicked() {
				s.settingsOpen = false
			}
		})
		ui.Scroll(c.Key("settings-body-" + s.settingsTab)).Grow(1).MinHeight(collapsible).FillWidth().TrackScroll(scroll).
			Label("Settings content").Bind(&s.settingsResults).Focusable().Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(portrait.Settings.Column).Padding(portrait.Settings.ColumnPadTop, portrait.Settings.ColumnPadX, portrait.Settings.ColumnPadBottom, portrait.Settings.ColumnPadX).
					Gap(portrait.Settings.GroupGap).Children(func() {
					if !query.Active() {
						current.Draw(c, query)
						return
					}
					for _, page := range pages {
						if !query.PageMatch(page) {
							continue
						}
						ui.Column(c.Key("search-page-" + page.ID)).FillWidth().Gap(portrait.Settings.HeadingGap).Children(func() {
							link := portrait.ButtonWith(c, page.Label, "Open "+page.Label, portrait.ButtonSpec{Look: portrait.Link, Size: portrait.Sizes.Regular})
							if link.Clicked() {
								s.settingsTab, s.settingsQuery = page.ID, ""
							}
							page.Draw(c, query)
						})
					}
					if searchState.matches == 0 {
						portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconMagnifyingGlass, Title: "No settings match “" + query.Text + "”", Description: "Try a different setting name or keyword."})
					}
				})
			})
		})
	})
}
