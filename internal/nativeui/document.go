package nativeui

import (
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func (s *Shell) page(c *ui.Context, doc workspace.DocumentInfo, m Model, a Actions, slots Slots) {
	sheet(c).Role(ui.RoleMain).Label("Document").Children(func() {
		s.pageBar(c, doc, m, a, slots)
		if doc.ID == "" {
			s.welcome(c, m, a)
			return
		}
		ui.Column(c).FillWidth().Padding(0, 12).Gap(6).Children(func() {
			if doc.ActionError != "" {
				portrait.Alert(c, portrait.Tones.Danger, doc.ActionError, "")
			}
			if doc.RestoreError != "" {
				portrait.Alert(c, portrait.Tones.Danger, doc.RestoreError, "")
			}
			if doc.Conflict {
				portrait.Alert(c, portrait.Tones.Warning, "The file changed on disk.", "Choose which version to keep before saving.")
				ui.Row(c).Gap(portrait.Form.Gap).Children(func() {
					if portrait.AccentButton(c, "Save Anyway", "Save over the disk version").Clicked() && a.SaveConflict != nil {
						a.SaveConflict(doc.ID)
					}
					if portrait.ButtonWith(c, "Revert to Disk", "Revert to the disk version", portrait.ButtonSpec{Look: portrait.DangerOutline}).Clicked() && a.RevertConflict != nil {
						a.RevertConflict(doc.ID)
					}
				})
			}
			if doc.Deleted {
				portrait.Alert(c, portrait.Tones.Warning, "The file was deleted on disk. Edits are still available in this window.", "")
			}
			if m.ReturnToGenUI && slots.GenUI != nil {
				ui.Row(c).Children(func() {
					if portrait.ButtonLeadingIcon(c, portrait.IconArrowLeft, "Back to GenUI", "Back to GenUI").Clicked() && a.SetMode != nil {
						a.SetMode("genui")
					}
				})
			}
		})
		if slots.Review != nil {
			slots.Review(c)
		}
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			if doc.ViewMode == "genui" && slots.GenUI != nil {
				slots.GenUI(c)
			} else if slots.Document != nil {
				slots.Document(c)
			}
		})
	})
}

func (s *Shell) pageBar(c *ui.Context, doc workspace.DocumentInfo, m Model, a Actions, slots Slots) {
	width, _ := c.Size()
	bar := c.TitleBar()
	lead := float32(8)
	if !s.sidebarDocked {
		lead = max(lead, bar.Left-portrait.Shell.PageInset/4)
	}
	barRow := ui.Row(c).Height(portrait.Shell.PageBar).Padding(0, 8, 0, lead).Gap(6).AlignItems(ui.Center).DragWindow()
	if m.DocumentScrollY > 0 {
		barRow.BorderWidth(0, 0, portrait.Editor.Hairline, 0).BorderColor(portrait.Colors.Line)
	}
	barRow.Children(func() {
		if !s.sidebarDocked {
			toggle := portrait.QuietIconButton(c, portrait.IconSidebar, "Show navigation").Expanded(s.leftOpen).Bind(&s.leftToggle)
			toggle.ObserveInput(func(ev ui.InputEvent) {
				if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
					s.leftKeys.Flip()
					s.rightKeys.Flip()
				}
			})
			if toggle.Clicked() {
				s.toggleSidebar(width)
			}
		}
		ui.Row(c).Grow(1).MinWidth(0).Clip().AlignItems(ui.Center).Gap(2).Padding(0, 0, 0, 4).Children(func() {
			if doc.ID == "" {
				return
			}
			if len(m.Crumbs) == 0 {
				portrait.Type.UI.Apply(ui.Text(c, doc.Name)).FontWeight(500).TextColor(portrait.Colors.Ink).SingleLine().TruncateMiddle().MinWidth(0).Ellipsis("…")
				return
			}
			showAncestors := width >= sidebarDocksAt+portrait.Shell.SidebarMin
			for _, crumb := range m.Crumbs {
				if !crumb.IsFile && !showAncestors {
					continue
				}
				if crumb.IsFile {
					name := portrait.Type.UI.Apply(ui.Text(c, crumb.Name)).FontWeight(500).TextColor(portrait.Colors.Ink).SingleLine().TruncateMiddle().Ellipsis("…").MinWidth(0).Grow(1).Shrink(1)
					if showAncestors {
						name.MinWidth(portrait.Form.SelectMin)
					}
					portrait.Tooltip(c, name, doc.Path)
					continue
				}
				b := portrait.Tooltip(c, ui.ButtonBase(c.Key(crumb.Path)).Label("Reveal "+crumb.Name), crumb.Path).Padding(2, 4).Radius(portrait.Radii.S).MinWidth(0).Shrink(1)
				ink := portrait.Press(b, portrait.Quiet)
				if !b.Hovered() {
					ink = portrait.Colors.Ink3
				}
				b.Children(func() {
					portrait.Type.UI.Apply(ui.Text(c, crumb.Name)).TextColor(ink).SingleLine().TruncateMiddle().MinWidth(0).Ellipsis("…")
				})
				if b.Clicked() && a.RevealPath != nil {
					a.RevealPath(crumb.Path)
				}
				portrait.Glyph(c, portrait.IconCaretRight, portrait.IconSizes.XS, portrait.Colors.Ink4).Shrink(0)
			}
		})
		if doc.ID != "" {
			if state := documentStateText(doc); state != "" {
				portrait.Type.Meta.Apply(ui.Text(c, state)).TextColor(portrait.Colors.Ink3).Role(ui.RoleStatus).MaxLines(1).Shrink(0)
			}
			portrait.Track(c, "view modes", func(t *portrait.Segments) {
				for i, title := range []string{"Preview", "Markdown", "GenUI"} {
					mode := []string{"preview", "markdown", "genui"}[i]
					if mode == "genui" && slots.GenUI == nil {
						continue
					}
					selected := doc.ViewMode == mode || (mode == "preview" && doc.ViewMode != "genui" && doc.ViewMode != "markdown")
					if t.Segment(c, title, title, selected).Role(ui.RoleTab).AccessSelected(selected).Clicked() && a.SetMode != nil {
						a.SetMode(mode)
					}
				}
			}).Label("Document views").Shrink(0)
			if m.Git && slots.Checkpoints != nil {
				b := portrait.QuietIconButton(c, portrait.IconClockCounterClockwise, "Checkpoints").Expanded(s.checkpoints)
				if b.Clicked() {
					s.checkpoints = !s.checkpoints
				}
				portrait.Popover(c, b, &s.checkpoints, func() { slots.Checkpoints(c) })
			}
		}
		if m.Feynman {
			visible := s.rightOpen || s.researchDocked
			toggle := portrait.QuietIconButton(c, portrait.IconSidebar.Mirrored(), "Toggle research").Expanded(visible).KeyShortcuts(ui.Cmd|ui.Alt, ui.Key0).Bind(&s.rightToggle)
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

func documentStateText(doc workspace.DocumentInfo) string {
	var parts []string
	if doc.Edited {
		parts = append(parts, "Edited")
	}
	if doc.Conflict {
		parts = append(parts, "Conflict")
	}
	if doc.ReadOnly {
		parts = append(parts, "Read only")
	}
	if doc.Generating {
		parts = append(parts, "Generating…")
	}
	return strings.Join(parts, " · ")
}

func (s *Shell) welcome(c *ui.Context, m Model, a Actions) {
	width, height := c.Size()
	compact := height < 560 || width < sidebarDocksAt
	title, lede, top := portrait.Type.Display, portrait.Type.Reading, float32(56)
	if compact {
		title, lede, top = portrait.Type.Title, portrait.Type.UI, 12
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(top, 24, 24, 24).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(portrait.Shell.CommandWidth - 160).Gap(6).Children(func() {
				if m.Workspace.Kind == "folder" {
					title.In(portrait.FontDisplay).Apply(ui.Text(c, m.Workspace.Name)).FontWeight(600).TextColor(portrait.Colors.Ink)
					lede.Apply(ui.Text(c, "Open a Markdown file from this folder.")).TextColor(portrait.Colors.Ink3).Wrap().Pretty()
				} else {
					title.In(portrait.FontDisplay).Apply(ui.Text(c, "Welcome")).FontWeight(600).TextColor(portrait.Colors.Ink)
					lede.Apply(ui.Text(c, "Open a folder to browse Markdown files, or start an untitled document.")).TextColor(portrait.Colors.Ink3).Wrap().Pretty()
				}
				ui.Row(c).Padding(top/4, 0, 6, 0).Gap(8).AlignItems(ui.Center).Children(func() {
					if m.Workspace.Kind != "folder" {
						if portrait.ButtonWith(c, "Open Folder", "Open Folder", portrait.ButtonSpec{Look: portrait.Accent, Size: portrait.Sizes.Large}).Clicked() && a.OpenFolder != nil {
							a.OpenFolder()
						}
					}
					if portrait.ButtonWith(c, "New Document", "New Document", portrait.ButtonSpec{Size: portrait.Sizes.Large}).Clicked() && a.NewDocument != nil {
						a.NewDocument()
					}
					if compact {
						return
					}
					ui.Row(c).Grow(1)
					hint := ui.ButtonBase(c.Key("command hint")).Label("Open the Command Bar").Height(portrait.Sizes.Large).Padding(0, 8).Gap(6).Radius(portrait.Radii.M).AlignItems(ui.Center)
					portrait.Press(hint, portrait.Quiet)
					hint.Children(func() {
						portrait.Type.UI.Apply(ui.Text(c, "Search and commands")).TextColor(portrait.Colors.Ink3)
						shortcutKbd(c, ui.Cmd, "K")
					})
					if hint.Clicked() {
						s.toggleCommand(c, a)
					}
				})
				if m.Workspace.Kind == "folder" {
					return
				}
				portrait.Shell.Section.Apply(ui.Text(c, "Recent workspaces")).Label("Recent Workspaces").Role(ui.RoleHeading).FontWeight(600).TextColor(portrait.Colors.Ink3).Padding(top/3, 0, 2, 0)
				if len(m.Recents) == 0 {
					portrait.Type.UI.Apply(ui.Text(c, "No recent workspaces yet.")).TextColor(portrait.Colors.Ink3)
				}
				for _, r := range m.Recents {
					row := portrait.Tooltip(c, ui.ButtonBase(c.Key(r.Root)).Label(r.Name), r.Root).FillWidth().Height(portrait.Shell.CommandRow).Padding(0, 10).Gap(10).Radius(portrait.Radii.M).AlignItems(ui.Center)
					ink := portrait.Press(row, portrait.Quiet)
					row.Children(func() {
						portrait.SpaceFor(r.Root).Swatch(ui.Box(c).Size(portrait.Shell.Swatch, portrait.Shell.Swatch).Radius(portrait.Radii.XS).Shrink(0).AccessHidden(true))
						portrait.Type.UI.Apply(ui.Text(c, r.Name)).FontWeight(500).TextColor(ink).MaxLines(1).Shrink(0)
						portrait.Type.Meta.Apply(ui.Text(c, r.Root)).TextColor(portrait.Colors.Ink3).MaxLines(1).Ellipsis("…").Grow(1).MinWidth(0).AccessHidden(true)
					})
					if row.Clicked() && a.OpenRecent != nil {
						a.OpenRecent(r.Root)
					}
				}
			})
		})
	})
}
