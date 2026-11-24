package nativeui

import (
	"fmt"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo/ui"
)

func (s *Shell) document(c *ui.Context, doc workspace.DocumentInfo, m Model, a Actions, slots Slots) {
	ui.Column(c).Fill().MinWidth(0).Role(ui.RoleMain).Label("Document").Children(func() {
		if doc.ID == "" {
			if m.Workspace.Kind == "folder" {
				ui.Text(c, "Open a Markdown file from this folder.").Padding(24).TextColor(portrait.Colors.Ink3)
				return
			}
			ui.Scroll(c).Fill().Padding(32).Children(func() {
				portrait.Type.Title.In(portrait.FontDisplay).Apply(ui.Text(c, "Welcome")).Bold().TextColor(portrait.Colors.Ink)
				ui.Text(c, "Open a folder to browse Markdown files, or start an untitled document.").TextColor(portrait.Colors.Ink3)
				ui.Row(c).Padding(10, 0).Gap(6).Children(func() {
					if portrait.AccentButton(c, "Open Folder", "Open Folder").Clicked() && a.OpenFolder != nil {
						a.OpenFolder()
					}
					if portrait.Button(c, "New Document", "New Document").Clicked() && a.NewDocument != nil {
						a.NewDocument()
					}
				})
				portrait.Type.Meta.Apply(ui.Text(c, "RECENT WORKSPACES")).Label("Recent Workspaces").Role(ui.RoleHeading).Bold().LetterSpacing(0.08 * portrait.Type.Meta.Size).TextColor(portrait.Colors.Ink3)
				if len(m.Recents) == 0 {
					ui.Text(c, "No recent workspaces yet.").TextColor(portrait.Colors.Ink3)
				}
				for _, r := range m.Recents {
					ui.Row(c.Key(r.Root)).Children(func() {
						if portrait.Tooltip(c, portrait.Button(c, r.Name, r.Name), r.Root).Clicked() && a.OpenRecent != nil {
							a.OpenRecent(r.Root)
						}
					})
				}
			})
			return
		}
		ui.Row(c).Height(40).Padding(5, 12).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Clip().AlignItems(ui.Center).Gap(3).Children(func() {
				if len(m.Crumbs) == 0 {
					ui.Text(c, doc.Name).MaxLines(1).Ellipsis("…")
					return
				}
				for _, crumb := range m.Crumbs {
					if crumb.IsFile {
						ui.Text(c, crumb.Name).Bold().MaxLines(1).Ellipsis("…")
						continue
					}
					b := portrait.Tooltip(c, ui.ButtonBase(c.Key(crumb.Path)).Label("Reveal "+crumb.Name), crumb.Path).Padding(2)
					b.Children(func() { ui.Text(c, crumb.Name).TextColor(portrait.Colors.Ink3).MaxLines(1) })
					if b.Clicked() && a.RevealPath != nil {
						a.RevealPath(crumb.Path)
					}
					ui.Text(c, "/").TextColor(portrait.Colors.Ink3)
				}
			})
			if m.Version > 0 {
				portrait.Type.Meta.Apply(ui.Text(c, fmt.Sprintf("v%d", m.Version))).TextColor(portrait.Colors.Ink3).Padding(1, 5).Radius(8).Border(0.5, portrait.Colors.LineStrong)
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
			}).Label("Document views")
			if m.Git && slots.Checkpoints != nil {
				b := portrait.ButtonWith(c, "Checkpoints", "Checkpoints", portrait.ButtonSpec{Look: portrait.Quiet, Selected: s.checkpoints}).Expanded(s.checkpoints)
				if b.Clicked() {
					s.checkpoints = !s.checkpoints
				}
				portrait.Popover(c, b, &s.checkpoints, func() { slots.Checkpoints(c) })
			}
		})
		if doc.ActionError != "" {
			portrait.Alert(c, portrait.Tones.Danger, doc.ActionError, "")
		}
		if doc.RestoreError != "" {
			portrait.Alert(c, portrait.Tones.Danger, doc.RestoreError, "")
		}
		if doc.Conflict {
			portrait.Alert(c, portrait.Tones.Warning, "The file changed on disk. Saving will ask which version to keep.", "")
		}
		if doc.Deleted {
			portrait.Alert(c, portrait.Tones.Warning, "The file was deleted on disk. Edits are still available in this window.", "")
		}
		if doc.ReadOnly {
			ui.Text(c, "Read only").TextColor(portrait.Colors.Ink3)
		}

		if m.ReturnToGenUI && slots.GenUI != nil {
			if portrait.ButtonLeadingIcon(c, portrait.IconArrowLeft, "Back to GenUI", "Back to GenUI").Clicked() && a.SetMode != nil {
				a.SetMode("genui")
			}
		}
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
		ui.Row(c).Height(22).Padding(2, 12).Gap(6).AlignItems(ui.Center).Children(func() {
			portrait.Tooltip(c, portrait.Type.Meta.Apply(ui.Text(c, doc.Path)).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…").TextColor(portrait.Colors.Ink3), doc.Path)
			state := "Saved"
			if doc.Edited {
				state = "Edited"
			}
			if doc.Conflict {
				state += " · conflict"
			}
			if doc.Generating {
				state += " · generating"
			}
			portrait.Type.Meta.Apply(ui.Text(c, state)).TextColor(portrait.Colors.Ink3)
		})
	})
}
