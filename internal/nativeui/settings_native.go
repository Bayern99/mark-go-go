package nativeui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	sidebarHiddenPreference  = "mgo.sidebar.hidden"
	sidebarViewPreference    = "mgo.sidebar.view"
	researchHiddenPreference = "mgo.research.hidden"
	defaultViewPreference    = "mgo.viewMode.default"
	readingSizePreference    = "mgo.editor.readingSize"
	readingWidthPreference   = "mgo.editor.readingWidth"
	codeSizePreference       = "mgo.editor.codeSize"
	lineNumbersPreference    = "mgo.editor.lineNumbers"
)

func preference(m Model, key, fallback string) string {
	if value := m.Preferences[key]; value != "" {
		return value
	}
	return fallback
}

func setPreference(a Actions, key, value string) {
	if a.SetPreference != nil {
		a.SetPreference(key, value)
	}
}

func resetPreferences(a Actions, keys ...string) {
	if a.ResetPreferences != nil {
		a.ResetPreferences(keys...)
	}
}

func (s *Shell) nativeSettingsPages(m Model, a Actions) []SettingsPage {
	generalKeys := []string{sidebarHiddenPreference, sidebarViewPreference, researchHiddenPreference, "mgo.researchWidth", defaultViewPreference}
	editorKeys := []string{readingSizePreference, readingWidthPreference, codeSizePreference, lineNumbersPreference}
	dirtyKeys := func(keys ...string) func() bool {
		return func() bool {
			for _, key := range keys {
				if m.Preferences[key] != "" {
					return true
				}
			}
			return false
		}
	}
	pages := []SettingsPage{
		{
			ID: "general", Label: "General", Icon: portrait.IconSlidersHorizontal, Order: 10,
			Keywords: []string{"sidebar navigation files outline research panel width documents default preview markdown"},
			Dirty:    dirtyKeys(generalKeys...), Reset: func() {
				resetPreferences(a, generalKeys...)
				s.hideSidebar, s.hideResearch, s.outline = false, false, false
				s.rightWidth = portrait.Shell.ResearchWidth
			},
			Draw: func(c *ui.Context, q SettingsQuery) {
				if !q.Active() || q.Match("Sidebar", "Navigation and document browser", "sidebar files outline") {
					portrait.SettingsGroup(c, "Sidebar", func(rows *portrait.Rows) {
						if q.Match("Show sidebar", "Keep the workspace navigation visible.", "sidebar navigation") {
							rows.Row(portrait.RowSpec{Title: "Show sidebar", Description: "Keep the workspace navigation visible."}, func() {
								visible := preference(m, sidebarHiddenPreference, "") != "on"
								if portrait.Switch(c.Key("settings-show-sidebar"), &visible).Label("Show sidebar").Changed() {
									s.hideSidebar = !visible
									if visible {
										setPreference(a, sidebarHiddenPreference, "")
									} else {
										setPreference(a, sidebarHiddenPreference, "on")
									}
								}
							})
						}
						if q.Match("Default navigation view", "Choose Files or Outline for a new window.", "files outline navigation") {
							rows.Row(portrait.RowSpec{Title: "Default navigation view", Description: "Choose Files or Outline for a new window."}, func() {
								view := preference(m, sidebarViewPreference, "files")
								before := view
								portrait.Select(c, &view, []portrait.Option{{Value: "files", Label: "Files"}, {Value: "outline", Label: "Outline"}}, portrait.SelectSpec{Label: "Default navigation view"})
								if view != before {
									s.outline = view == "outline"
									setPreference(a, sidebarViewPreference, view)
								}
							})
						}
					})
				}
				if m.Feynman && (!q.Active() || q.Match("Research panel", "Research visibility and width", "research feynman width")) {
					portrait.SettingsGroup(c, "Research panel", func(rows *portrait.Rows) {
						if q.Match("Show research panel", "Keep Feynman beside the document when there is room.", "research feynman") {
							rows.Row(portrait.RowSpec{Title: "Show research panel", Description: "Keep Feynman beside the document when there is room."}, func() {
								visible := preference(m, researchHiddenPreference, "") != "on"
								if portrait.Switch(c.Key("settings-show-research"), &visible).Label("Show research panel").Changed() {
									s.hideResearch = !visible
									if visible {
										setPreference(a, researchHiddenPreference, "")
									} else {
										setPreference(a, researchHiddenPreference, "on")
									}
								}
							})
						}
						if q.Match("Panel width", "The saved research panel width.", "research width") {
							spec := portrait.RowSpec{Title: "Panel width", Description: "Drag the divider to resize. The default is 376 points."}
							if m.Preferences["mgo.researchWidth"] != "" {
								spec.Reset = func() { resetPreferences(a, "mgo.researchWidth"); s.rightWidth = portrait.Shell.ResearchWidth }
							}
							rows.Row(spec, func() {
								width := preference(m, "mgo.researchWidth", strconv.Itoa(int(portrait.Shell.ResearchWidth)))
								portrait.Type.UI.Apply(ui.Text(c, width+" pt")).TextColor(portrait.Colors.Ink2)
							})
						}
					})
				}
				if !q.Active() || q.Match("Documents", "Default document view", "documents preview markdown") {
					portrait.SettingsGroup(c, "Documents", func(rows *portrait.Rows) {
						if q.Match("Default view", "Used when a document has no saved view.", "documents preview markdown") {
							rows.Row(portrait.RowSpec{Title: "Default view", Description: "Used when a document has no saved view."}, func() {
								mode := preference(m, defaultViewPreference, "preview")
								portrait.Track(c, "settings-default-view", func(track *portrait.Segments) {
									for _, option := range []struct{ value, label string }{{"preview", "Preview"}, {"markdown", "Markdown"}} {
										if track.Segment(c, option.label, "Default view "+option.label, mode == option.value).Clicked() {
											setPreference(a, defaultViewPreference, option.value)
										}
									}
								}).Label("Default document view")
							})
						}
					})
				}
			},
		},
		{
			ID: "appearance", Label: "Appearance", Icon: portrait.IconPalette, Order: 20,
			Keywords: []string{"appearance theme system light dark wu xing space pigment scenery accent workspace"},
			Dirty:    func() bool { return m.Appearance != portrait.System || m.Spaces[m.Workspace.Root] != "" },
			Reset:    func() { resetPreferences(a, appearancePreference, spacePreference+m.Workspace.Root) },
			Draw:     func(c *ui.Context, q SettingsQuery) { s.appearanceSettingsPage(c, m, a, q) },
		},
		{
			ID: "editor", Label: "Editor", Icon: portrait.IconFileText, Order: 30,
			Keywords: []string{"editor reading text size line width narrow default wide markdown source code size line numbers folded headings table widths reset"},
			Dirty:    dirtyKeys(editorKeys...), Reset: func() { resetPreferences(a, editorKeys...) },
			Draw: func(c *ui.Context, q SettingsQuery) { s.editorSettingsPage(c, m, a, q) },
		},
		{
			ID: "shortcuts", Label: "Shortcuts", Icon: portrait.IconCommand, Order: 40,
			Keywords: []string{"shortcuts keyboard keys command bar settings sidebar research preview markdown previous document close open folder save cancel search"},
			Draw:     func(c *ui.Context, q SettingsQuery) { shortcutSettings(c, q) },
		},
		{
			ID: "plugins", Section: "Workspace", Label: "Plugins", Icon: portrait.IconPuzzlePiece, Order: 200,
			Keywords: []string{"plugins feynman research git checkpoints enable disable"},
			Draw: func(c *ui.Context, q SettingsQuery) {
				if q.Active() && !q.Match("Optional plugins", "Feynman Research and Git Checkpoints", "plugins feynman git") {
					return
				}
				portrait.SettingsGroup(c, "Optional plugins", func(rows *portrait.Rows) {
					if q.Match("Feynman Research", "Side-by-side AI research assistant, models, and MCP tools.", "feynman plugin") {
						rows.Row(portrait.RowSpec{Title: "Feynman Research", Description: "Side-by-side AI research assistant, models, and MCP tools."}, func() {
							on := m.Feynman
							if portrait.Switch(c.Key("settings-plugin-feynman"), &on).Label("Enable Feynman").Changed() && a.SetFeynman != nil {
								a.SetFeynman(on)
							}
						})
					}
					if q.Match("Git Checkpoints", "Save, diff, and restore local document snapshots.", "git plugin checkpoints") {
						rows.Row(portrait.RowSpec{Title: "Git Checkpoints", Description: "Save, diff, and restore local document snapshots."}, func() {
							on := m.Git
							if portrait.Switch(c.Key("settings-plugin-git"), &on).Label("Enable Git").Changed() && a.SetGit != nil {
								a.SetGit(on)
							}
						})
					}
				})
			},
		},
		{
			ID: "diagnostics", Section: "Footer", Label: "Diagnostics", Icon: portrait.IconWrench, Order: 900,
			Keywords: []string{"diagnostics application preferences file reveal finder reset all settings"},
			Draw: func(c *ui.Context, q SettingsQuery) {
				if q.Active() && !q.Match("Application", "Preferences and maintenance", "diagnostics preferences reset") {
					return
				}
				portrait.SettingsGroup(c, "Application", func(rows *portrait.Rows) {
					if q.Match("Preferences file", m.PreferenceFile, "preferences path finder") {
						rows.Row(portrait.RowSpec{Title: "Preferences file", Description: m.PreferenceFile}, func() {
							if portrait.QuietButton(c, "Reveal in Finder", "Reveal preferences in Finder").Clicked() && a.RevealPreferences != nil {
								a.RevealPreferences()
							}
						})
					}
					if q.Match("Reset all settings", "Restore every application and research preference.", "reset preferences") {
						rows.Row(portrait.RowSpec{Title: "Reset all settings", Description: "Restore every application and research preference."}, func() {
							if portrait.ButtonWith(c, "Reset all settings", "Reset all settings", portrait.ButtonSpec{Look: portrait.Danger, Size: portrait.Sizes.Regular}).Clicked() {
								s.settingsResetID, s.settingsResetOpen = "all", true
							}
						})
					}
				})
			},
		},
	}
	return pages
}

func (s *Shell) appearanceSettingsPage(c *ui.Context, m Model, a Actions, q SettingsQuery) {
	if !q.Active() || q.Match("Theme", "Appearance System Light Dark", "theme appearance dark light") {
		portrait.SettingsGroup(c, "Theme", func(rows *portrait.Rows) {
			if q.Match("Appearance", "Choose Light, Dark, or follow macOS automatically.", "appearance theme dark light system") {
				rows.Row(portrait.RowSpec{Title: "Appearance", Description: "Choose Light, Dark, or follow macOS automatically."}, func() {
					portrait.Track(c, "settings-appearance", func(track *portrait.Segments) {
						for _, option := range portrait.Appearances {
							selected := option == m.Appearance || (option == portrait.System && m.Appearance == "")
							if track.Segment(c, string(option), "Appearance "+string(option), selected).Role(ui.RoleTab).AccessSelected(selected).Clicked() && a.SetAppearance != nil {
								a.SetAppearance(option)
							}
						}
					}).Label("Appearance mode")
				})
			}
		})
	}
	if q.Active() && !q.Match("Wu Xing Space", "Workspace pigment scenery and accent", "space wu xing pigment") {
		return
	}
	portrait.SettingsGroup(c, "Wu Xing Space", func(rows *portrait.Rows) {
		if !q.Match("Space for this workspace", m.Workspace.Name+" · each folder keeps its own pigment scenery and accent", "space wu xing") {
			return
		}
		spec := portrait.RowSpec{Title: "Space for this workspace", Description: m.Workspace.Name + " · each folder keeps its own pigment scenery and accent", Stacked: true}
		if m.Spaces[m.Workspace.Root] != "" {
			spec.Reset = func() { setPreference(a, spacePreference+m.Workspace.Root, "") }
		}
		rows.Row(spec, func() {
			activeSpace := spaceOf(m)
			columns := 4
			width, _ := c.Size()
			if width < portrait.Settings.Collapse {
				columns = 2
			}
			for start := 0; start < len(portrait.Spaces); start += columns {
				ui.Row(c.Key("space-row-" + portrait.Spaces[start].Name)).FillWidth().Gap(portrait.Form.PadX).Children(func() {
					for column := 0; column < columns && start+column < len(portrait.Spaces); column++ {
						space := portrait.Spaces[start+column]
						card := portrait.SpaceCard(c, space, space.Name == activeSpace.Name, s.dark)
						if card.Clicked() && a.SetSpace != nil {
							a.SetSpace(space.Name)
						}
					}
				})
			}
		})
	})
}

func (s *Shell) editorSettingsPage(c *ui.Context, m Model, a Actions, q SettingsQuery) {
	if !q.Active() || q.Match("Reading", "Text size and line width", "reading text size width") {
		portrait.SettingsGroup(c, "Reading", func(rows *portrait.Rows) {
			if q.Match("Text size", "Reading text size in points.", "editor reading text size") {
				rows.Row(portrait.RowSpec{Title: "Text size", Description: "Reading text size in points."}, func() {
					value := float32(preferenceInt(m, readingSizePreference, 16))
					before := value
					portrait.Stepper(c, &value, 15, 18, 1, func(v float32) string { return fmt.Sprintf("%.0f pt", v) })
					if value != before {
						setPreference(a, readingSizePreference, strconv.Itoa(int(value)))
					}
				})
			}
			if q.Match("Line width", "Maximum reading column width.", "line width narrow default wide") {
				rows.Row(portrait.RowSpec{Title: "Line width", Description: "Maximum reading column width."}, func() {
					width := preferenceInt(m, readingWidthPreference, 624)
					portrait.Track(c, "settings-reading-width", func(track *portrait.Segments) {
						for _, option := range []struct {
							width int
							label string
						}{{560, "Narrow"}, {624, "Default"}, {720, "Wide"}} {
							if track.Segment(c, option.label, fmt.Sprintf("Reading width %d", option.width), width == option.width).Clicked() {
								setPreference(a, readingWidthPreference, strconv.Itoa(option.width))
							}
						}
					}).Label("Reading line width")
				})
			}
		})
	}
	if !q.Active() || q.Match("Markdown source", "Code size and line numbers", "markdown code source") {
		portrait.SettingsGroup(c, "Markdown source", func(rows *portrait.Rows) {
			if q.Match("Code size", "Markdown source text size in points.", "code size markdown") {
				rows.Row(portrait.RowSpec{Title: "Code size", Description: "Markdown source text size in points."}, func() {
					value := float32(preferenceInt(m, codeSizePreference, 13))
					before := value
					portrait.Stepper(c, &value, 12, 14, 1, func(v float32) string { return fmt.Sprintf("%.0f pt", v) })
					if value != before {
						setPreference(a, codeSizePreference, strconv.Itoa(int(value)))
					}
				})
			}
			if q.Match("Line numbers", "Show line numbers in Markdown source.", "line numbers source") {
				rows.Row(portrait.RowSpec{Title: "Line numbers", Description: "Show line numbers in Markdown source."}, func() {
					on := preference(m, lineNumbersPreference, "on") != "off"
					if portrait.Switch(c.Key("settings-line-numbers"), &on).Label("Line numbers").Changed() {
						if on {
							setPreference(a, lineNumbersPreference, "")
						} else {
							setPreference(a, lineNumbersPreference, "off")
						}
					}
				})
			}
		})
	}
	if q.Active() && !q.Match("Maintenance", "Reset reading state", "folded headings table widths") {
		return
	}
	portrait.SettingsGroup(c, "Maintenance", func(rows *portrait.Rows) {
		if !q.Match("Reset reading state", "Reset folded headings and table widths for this document", "folded headings table widths") {
			return
		}
		rows.Row(portrait.RowSpec{Title: "Reset reading state", Description: "Reset folded headings and table widths for this document."}, func() {
			button := portrait.Button(c, "Reset reading state", "Reset folded headings and table widths")
			button.Disabled(m.Workspace.ActiveID == "")
			if button.Clicked() && a.ResetPreferencePrefixes != nil {
				key := activeDocument(m.Workspace).Path
				if key == "" {
					key = m.Workspace.ActiveID
				}
				a.ResetPreferencePrefixes("mgo.folds:"+key, "mgo.table-widths:"+key)
			}
		})
	})
}

func preferenceInt(m Model, key string, fallback int) int {
	value, err := strconv.Atoi(m.Preferences[key])
	if err != nil {
		return fallback
	}
	return value
}

func shortcutSettings(c *ui.Context, q SettingsQuery) {
	type shortcutRow struct {
		title, detail string
		keys          []string
	}
	shortcuts := []shortcutRow{
		{"Command Bar", "Search files and run commands", []string{"⌘K"}},
		{"Command Bar", "Open a new command", []string{"⌘T"}},
		{"Settings", "Open the Settings Center", []string{"⌘,"}},
		{"Sidebar", "Show or hide navigation", []string{"⌘0"}},
		{"Research", "Show or hide research", []string{"⌥⌘0"}},
		{"Preview ↔ Markdown", "Switch the document mode", []string{"⌘/"}},
		{"Previous document", "Return to the previous tab", []string{"⌃Tab"}},
		{"Close document", "Close the current document", []string{"⌘W"}},
		{"Open folder", "Open a workspace folder", []string{"⇧⌘O"}},
		{"Save note or block", "Commit the active draft", []string{"⌘↩"}},
		{"Cancel", "Cancel or close the active surface", []string{"Esc"}},
		{"Search settings", "Focus Settings search", []string{"/"}},
	}
	matched := make([]shortcutRow, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		if q.Match(shortcut.title, shortcut.detail, strings.Join(shortcut.keys, " ")) {
			matched = append(matched, shortcut)
		}
	}
	if len(matched) == 0 {
		return
	}
	portrait.SettingsGroup(c, "Keyboard shortcuts", func(rows *portrait.Rows) {
		for _, shortcut := range matched {
			shortcut := shortcut
			rows.Row(portrait.RowSpec{Title: shortcut.title, Description: shortcut.detail}, func() { portrait.Kbd(c, shortcut.keys...) })
		}
	})
}
