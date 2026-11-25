package nativeui

import (
	"slices"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type Model struct {
	ReturnToGenUI     bool
	DocumentScrollY   float32
	DocumentOffset    int
	HasDocumentOffset bool
	Crumbs            []workspace.Crumb
	Workspace         workspace.WorkspaceSnapshot
	Navigation        workspace.NavSnapshot
	Sections          []workspace.Section
	Recents           []workspace.RecentWorkspace
	Search            workspace.NavSearchResult
	Feynman, Git      bool
	Message           string
	Spaces            map[string]string
	Appearance        portrait.Appearance
	Preferences       map[string]string
	PreferenceFile    string
}

type Actions struct {
	Drop                                                       func([]string)
	RevealPath                                                 func(string)
	Activate, CloseDocument, ActivateWorkspace, CloseWorkspace func(string)
	OpenRecent, ToggleFolder, Filter, SetMode, SetSpace        func(string)
	OpenPath                                                   func(path string, offset int)
	Reveal                                                     func(int)
	NewDocument, OpenFolder                                    func()
	Save, SaveAs, Rename, Move, Revert                         func(string)
	OpenFiles, Find, Undo                                      func()
	CommandFocus                                               func() *ui.Handle
	SetFeynman, SetGit                                         func(bool)
	SetAppearance                                              func(portrait.Appearance)
	SetPreference                                              func(key, value string)
	ResetPreferences                                           func(keys ...string)
	ResetPreferencePrefixes                                    func(prefixes ...string)
	RevealPreferences                                          func()
	SaveConflict, RevertConflict                               func(string)
}

type Slots struct {
	Document, GenUI, Research, Checkpoints, Review func(*ui.Context)
	Settings                                       []SettingsPage
}

type Shell struct {
	ready, outline, navFocus                       bool
	leftOpen, rightOpen, hideResearch, hideSidebar bool
	filterOpen, filterFocus                        bool
	focusedTab, dark                               bool
	checkpoints                                    bool
	settingsOpen, workspaceMenu                    bool
	settingsTab, settingsQuery                     string
	settingsMatchedQuery                           string
	settingsMatchedPages                           []string
	settingsSubmitQuery                            string
	settingsFocus, settingsEditing                 bool
	settingsResetOpen                              bool
	settingsResetID                                string
	settingsReset, settingsResults                 ui.Handle
	settingsNav                                    map[string]*ui.Handle
	command                                        commandBar
	workspaceID, activeID, query, tabFocus         string
	tabScroll                                      string
	filterReturn                                   ui.Handle
	history                                        []string
	leftWidth, rightWidth                          float32
	leftToggle, rightToggle                        ui.Handle
	leftKeys, rightKeys                            portrait.KeyTrigger
	queries                                        map[string]string
	drills                                         map[string]string
	scrolls                                        map[string]*ui.ScrollState
	openCheckpoints                                map[string]bool
	selectedOutline                                map[string]int
	sidebarDocked, researchDocked                  bool
	sidebarShown, sidebarEdge                      float32
	sidebarInstant, wideReady, lastWide            bool
	sidebarAnimating                               bool
	settle                                         int
}

type sidebarMotionKey struct{}

const (
	sidebarDocksAt  = 641
	researchDocksAt = 1180
)

func (s *Shell) Draw(c *ui.Context, m Model, a Actions, slots Slots) {
	if !s.ready {
		s.queries = map[string]string{}
		s.drills = map[string]string{}
		s.openCheckpoints = map[string]bool{}
		s.selectedOutline = map[string]int{}
		s.leftWidth, s.ready = portrait.Shell.SidebarWidth, true
		if s.rightWidth == 0 {
			s.rightWidth = portrait.Shell.ResearchWidth
		}
	}
	if s.workspaceID != m.Workspace.Root {
		s.workspaceID, s.query = m.Workspace.Root, s.queries[m.Workspace.Root]
		s.filterOpen = s.query != ""
	}
	if s.activeID != m.Workspace.ActiveID {
		if s.activeID != "" {
			s.openCheckpoints[s.activeID] = s.checkpoints
		}
		s.activeID = m.Workspace.ActiveID
		s.tabScroll = s.activeID
		s.leftOpen, s.checkpoints = false, s.openCheckpoints[s.activeID]
	}
	s.history = slices.DeleteFunc(s.history, func(id string) bool {
		return !slices.ContainsFunc(m.Workspace.Documents, func(d workspace.DocumentInfo) bool { return d.ID == id })
	})
	if s.activeID != "" && (len(s.history) == 0 || s.history[0] != s.activeID) {
		s.history = append([]string{s.activeID}, slices.DeleteFunc(s.history, func(id string) bool { return id == s.activeID })...)
	}
	width, height := c.Size()
	if width >= sidebarDocksAt {
		s.leftOpen = false
	}
	if width >= researchDocksAt || !m.Feynman {
		s.rightOpen = false
	}
	if !m.Git {
		s.checkpoints = false
	}
	doc := activeDocument(m.Workspace)
	if c.Shortcut(ui.Ctrl, ui.KeyTab) || c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		if len(s.history) > 1 && a.Activate != nil {
			a.Activate(s.history[1])
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) && doc.ID != "" && a.SetMode != nil {
		if doc.ViewMode == "markdown" {
			a.SetMode("preview")
		} else {
			a.SetMode("markdown")
		}
	}
	if c.Shortcut(ui.Cmd|ui.Alt, ui.Key0) && m.Feynman {
		s.toggleResearch(width)
		if width >= researchDocksAt {
			s.persistResearch(a)
		}
		s.leftKeys.Flip()
		s.rightKeys.Flip()
	}
	if c.Shortcut(ui.Cmd, ui.Key0) {
		s.toggleSidebar(width)
		s.sidebarInstant = true
		if width >= sidebarDocksAt {
			s.persistSidebar(a)
		}
		s.leftKeys.Flip()
		s.rightKeys.Flip()
	}
	if c.Shortcut(ui.Cmd, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyT) {
		s.toggleCommand(c, a)
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		if s.settingsOpen {
			s.settingsOpen = false
		} else {
			s.OpenSettings("")
		}
	}
	wide := width >= sidebarDocksAt
	if s.wideReady && wide != s.lastWide {
		s.sidebarInstant = true
	}
	s.wideReady, s.lastWide = true, wide
	sidebarDocked, researchDocked := wide && !s.hideSidebar, m.Feynman && !s.hideResearch && width >= researchDocksAt
	if sidebarDocked != s.sidebarDocked || researchDocked != s.researchDocked {
		s.settle = 2
	}
	s.sidebarDocked, s.researchDocked = sidebarDocked, researchDocked
	if s.settle > 0 {
		s.settle--
		c.Invalidate()
	}
	systemDark := c.Theme().Dark
	s.dark = m.Appearance.IsDark(systemDark)
	portrait.Use(spaceOf(m), s.dark)
	t := portrait.Theme()
	translucent := c.Vibrancy() && !portrait.Opaque(c) && s.dark == systemDark
	if translucent {
		c.Root().Background(ui.Transparent)
	}
	root := themed(c, &t).Fill()
	if paths := root.DroppedFiles(); len(paths) > 0 && a.Drop != nil {
		a.Drop(slices.Clone(paths))
	}
	root.Children(func() {
		scenery := ui.Column(c).Fill().TextColor(portrait.Colors.Ink).Font(portrait.FontSans)
		spaceOf(m).Paint(scenery, translucent)
		scenery.Children(func() {
			target := float32(0)
			if s.sidebarDocked {
				target = 1
			}
			var progress float32
			if s.sidebarInstant || !portrait.Animates(c, false) {
				progress = scenery.Jump(sidebarMotionKey{}, target)
				s.sidebarInstant = false
			} else {
				progress = scenery.AnimateTo(sidebarMotionKey{}, target, ui.Animation{
					Duration: portrait.Motion.Switch,
					Ease:     portrait.Motion.EaseInOut.Easing(),
				})
			}
			settledHidden := !s.sidebarDocked && progress <= 0.001
			settledDocked := s.sidebarDocked && progress >= 0.999
			animating := !settledHidden && !settledDocked
			if animating != s.sidebarAnimating {
				s.sidebarAnimating = animating
				if !animating {
					s.settle = max(s.settle, 1)
				}
			}
			if settledHidden {
				progress = 0
			} else if settledDocked {
				progress = 1
			}
			room := width - 2*portrait.Shell.PageInset
			left := clamp(s.leftWidth, portrait.Shell.SidebarMin, min(portrait.Shell.SidebarMax, room-portrait.Shell.PageMin))
			right := clamp(s.rightWidth, portrait.Shell.ResearchMin, min(portrait.Shell.ResearchMax, width*0.56))
			if (s.sidebarDocked || progress > 0) && s.researchDocked {
				left = min(left, max(portrait.Shell.SidebarMin, room-portrait.Shell.PageMin-right-16))
				right = min(right, max(portrait.Shell.ResearchMin, room-portrait.Shell.PageMin-left-16))
			}
			shownWidth := (left + 1) * progress
			if settledHidden {
				shownWidth = 0
			} else if settledDocked {
				shownWidth = left + 1
			}
			s.sidebarShown = shownWidth
			inset := portrait.Shell.PageInset
			edge := inset - (inset/2)*progress
			s.sidebarEdge = edge
			stage := func() {
				themed(c, &t).Fill().MinWidth(0).Children(func() {
					if m.Message != "" || m.Workspace.RestoreError != "" {
						ui.Column(c).FillWidth().Padding(inset, inset, 0, edge).Gap(inset).Children(func() {
							if m.Message != "" {
								portrait.Alert(c, portrait.Tones.Danger, m.Message, "")
							}
							if m.Workspace.RestoreError != "" {
								portrait.Alert(c, portrait.Tones.Danger, m.Workspace.RestoreError, "")
							}
						})
					}
					page := func() { s.page(c, doc, m, a, slots) }
					if !s.researchDocked {
						ui.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(inset, inset, inset, edge).Children(page)
						return
					}
					remaining := max(0, width-shownWidth)
					first := remaining - right - 1
					quietSplit(c, &t).Grow(1).MinHeight(0).FillWidth().Children(func() {
						if ui.Split(c.Key("research-split"), &first,
							func() { themed(c, &t).Fill().Padding(inset, inset/2, inset, edge).Children(page) },
							func() {
								themed(c, &t).Fill().Padding(inset, inset, inset, inset/2).Children(func() { s.research(c, slots, true) })
							}).Fill().SplitReset(remaining - portrait.Shell.ResearchWidth - 1).Changed() {
							s.rightWidth = clamp(remaining-first-1, portrait.Shell.ResearchMin, min(portrait.Shell.ResearchMax, width*0.56))
						}
					})
				})
			}
			if settledHidden {
				stage()
				return
			}
			if animating {
				ui.Row(c.Key("library-anim")).Fill().AlignItems(ui.Stretch).Children(func() {
					slot := ui.Box(c.Key("sidebar-slot")).Width(shownWidth).FillHeight().Shrink(0).Clip()
					if !s.sidebarDocked {
						slot.AccessHidden(true).IgnoreInput(true)
					}
					slot.Children(func() {
						themed(c, &t).Width(left).FillHeight().Absolute().Top(0).Left(min(0, shownWidth-left)).Children(func() {
							s.sidebar(c, m, a, true)
						})
					})
					ui.Box(c.Key("stage-slot")).Grow(1).MinWidth(0).FillHeight().Clip().Children(stage)
				})
				return
			}
			actual := left
			quietSplit(c, &t).Fill().Children(func() {
				if ui.Split(c.Key("library-split"), &actual, func() { themed(c, &t).Fill().Children(func() { s.sidebar(c, m, a, true) }) }, stage).Fill().SplitReset(portrait.Shell.SidebarWidth).Changed() {
					s.leftWidth = clamp(actual, portrait.Shell.SidebarMin, portrait.Shell.SidebarMax)
				}
			})
		})
		s.drawer(c, true, width, height, m, a, slots)
		s.drawer(c, false, width, height, m, a, slots)
		s.commandBar(c, width, height, m, a, slots)
		s.settingsDialog(c, width, height, m, a, slots)
		portrait.Toasts(c)
	})
}

func (s *Shell) OpenSettings(tab string) {
	if tab == "tools" {
		tab = "mcp"
	}
	if tab == "" {
		tab = s.settingsTab
		if tab == "" || tab == "tools" {
			tab = "general"
		}
	}
	s.settingsTab = tab
	s.settingsQuery = ""
	s.settingsOpen = true
}

func (s *Shell) drillTo(root, dir string) {
	if s.drills == nil {
		s.drills = map[string]string{}
	}
	if root == "" || dir == "" || dir == root {
		delete(s.drills, root)
		return
	}
	s.drills[root] = dir
}

func themed(c *ui.Context, t *ui.Theme) ui.Element {
	return ui.Themed(c, t).Background(portrait.Clear)
}

func quietSplit(c *ui.Context, t *ui.Theme) ui.Element {
	quiet := *t
	quiet.Border = portrait.Clear
	return themed(c, &quiet)
}

func spaceOf(m Model) portrait.Space { return spaceNamed(m, m.Workspace.Root) }

func clamp(value, low, high float32) float32 { return max(low, min(value, high)) }

func activeDocument(w workspace.WorkspaceSnapshot) workspace.DocumentInfo {
	for _, d := range w.Documents {
		if d.ID == w.ActiveID {
			return d
		}
	}
	return workspace.DocumentInfo{}
}

func (s *Shell) persistSidebar(a Actions) {
	value := ""
	if s.hideSidebar {
		value = "on"
	}
	setPreference(a, sidebarHiddenPreference, value)
}

func (s *Shell) persistResearch(a Actions) {
	value := ""
	if s.hideResearch {
		value = "on"
	}
	setPreference(a, researchHiddenPreference, value)
}

func (s *Shell) toggleResearch(width float32) {
	if width < researchDocksAt {
		s.rightOpen = !s.rightOpen
		s.leftOpen = false
	} else {
		s.hideResearch = !s.hideResearch
	}
}

func (s *Shell) toggleSidebar(width float32) {
	if width < sidebarDocksAt {
		s.leftOpen = !s.leftOpen
		s.rightOpen = false
	} else {
		s.hideSidebar = !s.hideSidebar
	}
}

func (s *Shell) drawer(c *ui.Context, left bool, width, height float32, m Model, a Actions, slots Slots) {
	open, anchor, name, keys := &s.rightOpen, &s.rightToggle, "Research", &s.rightKeys
	if left {
		open, anchor, name, keys = &s.leftOpen, &s.leftToggle, "Navigation", &s.leftKeys
	}
	keyboard := keys.Live(c)
	slide, origin := portrait.Overlays.DrawerRight, float32(1)
	if left {
		slide, origin = portrait.Overlays.DrawerLeft, 0
	}
	edge := portrait.Shell.DrawerEdge
	w := min(width-3*edge, s.rightWidth)
	if left {
		w = min(width-3*edge, s.leftWidth)
	}
	tall := max(10*edge, height-2*edge)
	panel := portrait.DialogShell(c, name+" drawer", open, portrait.DialogSpec{
		Label: name, Width: w, Height: tall, Margin: edge, Top: edge, Radius: portrait.Radii.XL,
		PlainScrim: true, Drawer: true, DrawerRight: !left, DrawerMotion: slide, Keyboard: keyboard, OriginX: origin,
	}, func() {
		ui.Row(c).Padding(edge, portrait.Form.PadX, portrait.Form.PopoverGap, portrait.Form.PadX+portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
			if left {
				portrait.PanelTitle(c, name).Grow(1)
			} else {
				// The research panel's own header carries its title and tools.
				// Keep the dialog's accessible name without drawing that title twice.
				ui.Row(c).Grow(1)
			}
			close := portrait.QuietIconButton(c, portrait.IconX, "Close "+name)
			close.AutoFocus()
			if close.Clicked() {
				*open = false
				anchor.Focus(c)
			}
		})
		if left {
			s.sidebar(c, m, a, false)
		} else {
			s.research(c, slots, false)
		}
	})
	if panel.Valid() {
		panel.ObserveInput(keys.Observe)
		portrait.Materials.GlassOverlay.Apply(panel)
	}
	if panel.Dismissed() {
		anchor.Focus(c)
	}
}

func (s *Shell) research(c *ui.Context, slots Slots, docked bool) {
	panel(c, docked).Role(ui.RoleComplementary).Label("Feynman research").Children(func() {
		if slots.Research != nil {
			slots.Research(c)
		}
	})
}
