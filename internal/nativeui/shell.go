package nativeui

import (
	"slices"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type Model struct {
	ReturnToGenUI bool
	Crumbs        []workspace.Crumb
	Version       int
	Workspace     workspace.WorkspaceSnapshot
	Navigation    workspace.NavSnapshot
	Sections      []workspace.Section
	Recents       []workspace.RecentWorkspace
	Search        workspace.NavSearchResult
	Feynman, Git  bool
	Message       string
}

type Actions struct {
	Drop                                                       func([]string)
	RevealPath                                                 func(string)
	Activate, CloseDocument, ActivateWorkspace, CloseWorkspace func(string)
	OpenRecent, ToggleFolder, Filter, SetMode                  func(string)
	OpenPath                                                   func(path string, offset int)
	Reveal                                                     func(int)
	NewDocument, OpenFolder                                    func()
	SetFeynman, SetGit                                         func(bool)
}

type Slots struct {
	Document, GenUI, Research, Checkpoints, Review func(*ui.Context)
}

type Shell struct {
	ready, outline, navFocus                         bool
	leftOpen, rightOpen, hideResearch                bool
	overflow, focusFilter, focusedTab                bool
	checkpoints                                      bool
	workspaceID, activeID, query, tabQuery, tabFocus string
	history                                          []string
	leftWidth, rightWidth                            float32
	leftToggle, rightToggle                          ui.Handle
	queries                                          map[string]string
	scrolls                                          map[string]*ui.ScrollState
	openCheckpoints                                  map[string]bool
}

func (s *Shell) Draw(c *ui.Context, m Model, a Actions, slots Slots) {
	if !s.ready {
		s.queries = map[string]string{}
		s.openCheckpoints = map[string]bool{}
		s.leftWidth, s.ready = 248, true
		if s.rightWidth == 0 {
			s.rightWidth = 376
		}
	}
	if s.workspaceID != m.Workspace.Root {
		s.workspaceID, s.query = m.Workspace.Root, s.queries[m.Workspace.Root]
	}
	if s.activeID != m.Workspace.ActiveID {
		if s.activeID != "" {
			s.openCheckpoints[s.activeID] = s.checkpoints
		}
		s.activeID = m.Workspace.ActiveID
		s.leftOpen, s.checkpoints = false, s.openCheckpoints[s.activeID]
	}
	s.history = slices.DeleteFunc(s.history, func(id string) bool {
		return !slices.ContainsFunc(m.Workspace.Documents, func(d workspace.DocumentInfo) bool { return d.ID == id })
	})
	if s.activeID != "" && (len(s.history) == 0 || s.history[0] != s.activeID) {
		s.history = append([]string{s.activeID}, slices.DeleteFunc(s.history, func(id string) bool { return id == s.activeID })...)
	}
	width, height := c.Size()
	if width >= 641 {
		s.leftOpen = false
	}
	if width >= 1180 || !m.Feynman {
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
	if c.Shortcut(ui.Cmd, ui.KeySlash) && doc.ID != "" && a.SetMode != nil {
		if doc.ViewMode == "markdown" {
			a.SetMode("preview")
		} else {
			a.SetMode("markdown")
		}
	}
	if c.Shortcut(ui.Cmd|ui.Alt, ui.Key0) && m.Feynman {
		s.toggleResearch(width)
	}
	t := portraitTheme()
	root := ui.Themed(c, &t).Fill()
	if paths := root.DroppedFiles(); len(paths) > 0 && a.Drop != nil {
		a.Drop(slices.Clone(paths))
	}
	root.Children(func() {
		ui.Column(c).Fill().Background(paper).TextColor(ink).Font("IBM Plex Sans").Children(func() {

			if m.Message != "" {
				banner(c, m.Message)
			}
			if m.Workspace.RestoreError != "" {
				banner(c, m.Workspace.RestoreError)
			}
			ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Padding(8).Gap(0).Children(func() {
				leftDocked := width >= 641
				rightDocked := m.Feynman && !s.hideResearch && width >= 1180
				room := width - 16
				left := clamp(s.leftWidth, 140, min(420, room-412))
				right := clamp(s.rightWidth, 310, min(800, width*0.56))
				if leftDocked && rightDocked {
					left = min(left, max(140, room-412-right-16))
					right = min(right, max(310, room-412-left-16))
				}
				centerWidth := room
				if leftDocked {
					centerWidth -= left + 1
				}
				if rightDocked {
					centerWidth -= right + 1
				}
				center := func() {
					ui.Column(c).Fill().MinWidth(0).Children(func() {
						s.tabs(c, width, centerWidth, m, a)
						ui.Column(c).Grow(1).MinHeight(0).Children(func() { s.document(c, doc, m, a, slots) })
					})
				}
				inside := func() {
					if !rightDocked {
						center()
						return
					}
					remaining := room
					if leftDocked {
						remaining -= left + 1
					}
					first := remaining - right - 1
					if ui.Split(c.Key("research-split"), &first, center, func() { s.research(c, slots) }).Fill().SplitReset(remaining - 377).Changed() {
						s.rightWidth = clamp(remaining-first-1, 310, min(800, width*0.56))
					}
				}
				if leftDocked {
					actual := left
					if ui.Split(c.Key("library-split"), &actual, func() { s.navigation(c, m, a) }, inside).Fill().SplitReset(248).Changed() {
						s.leftWidth = clamp(actual, 190, 420)
					}
				} else {
					inside()
				}
			})
		})
		s.drawer(c, true, width, height, m, a, slots)
		s.drawer(c, false, width, height, m, a, slots)
	})
}

func clamp(value, low, high float32) float32 { return max(low, min(value, high)) }

func activeDocument(w workspace.WorkspaceSnapshot) workspace.DocumentInfo {
	for _, d := range w.Documents {
		if d.ID == w.ActiveID {
			return d
		}
	}
	return workspace.DocumentInfo{}
}

func (s *Shell) toggleResearch(width float32) {
	if width < 1180 {
		s.rightOpen = !s.rightOpen
		s.leftOpen = false
	} else {
		s.hideResearch = !s.hideResearch
	}
}

func (s *Shell) drawer(c *ui.Context, left bool, width, height float32, m Model, a Actions, slots Slots) {
	open, anchor, name := &s.rightOpen, &s.rightToggle, "Research"
	if left {
		open, anchor, name = &s.leftOpen, &s.leftToggle, "Navigation"
	}
	panel := ui.DialogBase(c.Key(name+" drawer"), open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(29, 29, 31, 0.08)).AlignItems(ui.Start).Justify(ui.Start)
		w := min(width-24, s.rightWidth)
		if left {
			w = min(width-24, s.leftWidth)
		}
		panel.Absolute().Top(8).Height(max(80, height-16)).Width(w).Radius(14).Material(glass(c, true)).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.14)).Label(name)
		if left {
			panel.Left(8)
		} else {
			panel.Right(8)
		}
		ui.Row(c).Padding(6, 10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, name).Bold().Grow(1)
			close := portrait.QuietIconButton(c, portrait.IconX, "Close "+name)
			close.AutoFocus()
			if close.Clicked() {
				*open = false
				anchor.Focus(c)
			}
		})
		if left {
			s.navigation(c, m, a)
		} else {
			s.research(c, slots)
		}
	})
	if panel.Dismissed() {
		anchor.Focus(c)
	}
}

func (s *Shell) research(c *ui.Context, slots Slots) {
	panel(c).Role(ui.RoleComplementary).Label("Feynman research").Children(func() {
		if slots.Research != nil {
			slots.Research(c)
		}
	})
}
