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
				ui.Text(c, "Open a Markdown file from this folder.").Padding(24).TextColor(muted)
				return
			}
			ui.Scroll(c).Fill().Padding(32).Children(func() {
				ui.Text(c, "Welcome").Font("Barlow Semi Condensed").FontSize(30).Bold()
				ui.Text(c, "Open a folder to browse Markdown files, or start an untitled document.").TextColor(muted)
				ui.Row(c).Padding(10, 0).Gap(6).Children(func() {
					if portrait.AccentButton(c, "Open Folder", "Open Folder").Clicked() && a.OpenFolder != nil {
						a.OpenFolder()
					}
					if portrait.Button(c, "New Document", "New Document").Clicked() && a.NewDocument != nil {
						a.NewDocument()
					}
				})
				ui.Text(c, "Recent Workspaces").Font("Barlow Semi Condensed").FontSize(18).Bold()
				if len(m.Recents) == 0 {
					ui.Text(c, "No recent workspaces yet.").TextColor(muted)
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
					b.Children(func() { ui.Text(c, crumb.Name).TextColor(muted).MaxLines(1) })
					if b.Clicked() && a.RevealPath != nil {
						a.RevealPath(crumb.Path)
					}
					ui.Text(c, "/").TextColor(muted)
				}
			})
			if m.Version > 0 {
				ui.Text(c, fmt.Sprintf("v%d", m.Version)).FontSize(11).TextColor(muted).Padding(1, 5).Radius(8).Border(0.5, rim)
			}
			portrait.Track(c, "view modes", func(t *portrait.Segments) {
				for i, title := range []string{"Preview", "Markdown", "GenUI"} {
					mode := []string{"preview", "markdown", "genui"}[i]
					if mode == "genui" && slots.GenUI == nil {
						continue
					}
					selected := doc.ViewMode == mode || (mode == "preview" && doc.ViewMode != "genui" && doc.ViewMode != "markdown")
					if t.Segment(title, title, selected).Role(ui.RoleTab).AccessSelected(selected).Clicked() && a.SetMode != nil {
						a.SetMode(mode)
					}
				}
			}).Label("Document views")
			if m.Git && slots.Checkpoints != nil {
				b := portrait.Button(c, "Checkpoints", "Checkpoints").Expanded(s.checkpoints)
				if b.Clicked() {
					s.checkpoints = !s.checkpoints
				}
				portrait.Popover(c, b, &s.checkpoints, func() { slots.Checkpoints(c) })
			}
		})
		if doc.ActionError != "" {
			banner(c, doc.ActionError)
		}
		if doc.RestoreError != "" {
			banner(c, doc.RestoreError)
		}
		if doc.Conflict {
			banner(c, "The file changed on disk. Saving will ask which version to keep.")
		}
		if doc.Deleted {
			banner(c, "The file was deleted on disk. Edits are still available in this window.")
		}
		if doc.ReadOnly {
			ui.Text(c, "Read only").TextColor(muted)
		}

		if m.ReturnToGenUI && slots.GenUI != nil {
			if portrait.Button(c, "← Back to GenUI", "Back to GenUI").Clicked() && a.SetMode != nil {
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
			portrait.Tooltip(c, ui.Text(c, doc.Path).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…").FontSize(11).TextColor(muted), doc.Path)
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
			ui.Text(c, state).FontSize(11).TextColor(muted)
		})
	})
}
