package nativereview

import (
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo/ui"
)

type checkpointState struct {
	id, path, label, selected, errorText string
	items                                []review.Checkpoint
	loaded, busy, comparing              bool
	base                                 reading
	original                             string
	diff                                 []review.DiffHunk
}

func checkpointEmpty(c *ui.Context, title, description string) ui.Element {
	return portrait.Empty(c, portrait.EmptySpec{Icon: portrait.IconFileText, Title: title, Description: description})
}

func (p *panel) drawCheckpoints(c *ui.Context) {
	width, _ := c.Size()
	active := p.win.Active()
	if active.Path == "" {
		checkpointEmpty(c, "Save this document to use Checkpoints.", "Save the document before creating a version snapshot.")
		return
	}
	s := p.checkpointDocs[active.ID]
	if s == nil || s.path != active.Path {
		s = &checkpointState{id: active.ID, path: active.Path}
		p.checkpointDocs[active.ID] = s
		p.loadCheckpoints(s)
	}
	ui.Column(c).Width(min(470, max(260, width-40))).Padding(8).Gap(8).Children(func() {
		portrait.Type.UI.Apply(ui.Text(c, "Checkpoints")).FontWeight(600).TextColor(portrait.Colors.Ink)
		ui.Row(c).Gap(6).Children(func() {
			label := ui.TextInput(c, &s.label).Label("Checkpoint label").Placeholder("Optional label").Grow(1)
			canCreate := !s.busy && !active.Edited && !active.Conflict && !active.Deleted
			if portrait.Button(c, "Create checkpoint", "Create checkpoint").Disabled(!canCreate).Clicked() || label.Submitted() && canCreate {
				if err := p.win.Publish(); err != nil {
					s.errorText = err.Error()
				} else {
					p.createCheckpoint(s)
				}
			}
		})
		if active.Edited || active.Conflict || active.Deleted {
			ui.Text(c, "Save the current document and resolve any conflicts to create a checkpoint.").Wrap()
		}
		if !s.loaded && s.busy {
			ui.Text(c, "Loading checkpoints…")
		}
		if s.loaded && len(s.items) == 0 {
			checkpointEmpty(c, "No checkpoints for this document.", "Create a checkpoint to save a version for comparison or restoration.")
		}
		if len(s.items) > 0 {
			p.drawCheckpointList(c, s)
		}
		if s.comparing {
			drawDiff(c, s.diff, nil)
			valid := s.base.current(p.win)
			if !valid {
				ui.Text(c, "The document changed. Compare again before restoring.").Wrap().Role(ui.RoleStatus)
			}
			if ui.Button(c, "Restore shown version").Disabled(s.busy || !valid || active.ReadOnly || s.base.text == s.original).Clicked() {
				err := s.base.apply(p.win, s.original)
				if err == nil {
					s.comparing = false
					err = p.win.Publish()
				}
				if err != nil {
					s.errorText = err.Error()
				}
			}
		}
		if s.errorText != "" {
			ui.Text(c, s.errorText).Role(ui.RoleAlert).Wrap()
			if ui.Button(c, "Refresh checkpoints").Disabled(s.busy).Clicked() {
				p.loadCheckpoints(s)
			}
		}
	})
}

func (p *panel) drawCheckpointList(c *ui.Context, s *checkpointState) {
	ui.Scroll(c).Height(108).Label("Saved checkpoints").Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			for _, item := range s.items {
				when := time.UnixMilli(item.CreatedAt).Local().Format("2006-01-02 15:04")
				backend := "Git ref"
				if item.Backend == "local" {
					backend = "Local snapshot"
				}
				name := strings.TrimSpace(item.Label)
				if name == "" {
					name = "Snapshot"
				}
				selected := s.selected == item.ID
				row := ui.ButtonBase(c.Key(item.ID)).Label(name+" · "+backend+" · "+when).FillWidth().Height(portrait.Sizes.Regular).Padding(0, 8).Gap(6).Radius(portrait.Radii.M).AlignItems(ui.Center).Selected(selected).Disabled(s.busy)
				portrait.Tooltip(c, row, item.Source)
				portrait.RowHover(row, row.Hovered() && !selected)
				row.Children(func() {
					portrait.RowCapsule(c, row, selected, "checkpoints:"+s.id)
					portrait.Type.UI.Apply(ui.Text(c, name)).FontWeight(500).TextColor(portrait.Colors.Ink).ZIndex(1).MaxLines(1).Ellipsis("…").Grow(1).MinWidth(0)
					portrait.Type.Meta.Apply(ui.Text(c, backend+" · "+when)).TextColor(portrait.Colors.Ink3).ZIndex(1).MaxLines(1).Shrink(0)
				})
				if row.Clicked() {
					s.selected = item.ID
					s.comparing = false
				}
			}
		})
	})
	if portrait.Button(c, "Compare", "Compare the selected checkpoint").Disabled(s.busy || s.selected == "").Clicked() {
		p.compareCheckpoint(s)
	}
}

func (p *panel) loadCheckpoints(s *checkpointState) {
	p.checkpointTask(s, func() func() {
		list, err := p.checkpoints.List(p.ctx, s.id, s.path)
		return func() { s.updateList(list, err) }
	})
}

func (p *panel) createCheckpoint(s *checkpointState) {
	base := read(p.win)
	label := strings.TrimSpace(s.label)
	p.checkpointTask(s, func() func() {
		list, err := p.checkpoints.Create(p.ctx, s.id, s.path, base.text, label, "user")
		return func() {
			if err == nil {
				s.label = ""
				s.comparing = false
				s.selected = ""
			}
			s.updateList(list, err)
		}
	})
}

func (s *checkpointState) updateList(list []review.Checkpoint, err error) {
	if err != nil {
		s.errorText = err.Error()
		return
	}
	s.loaded, s.items = true, list
	if s.selected == "" && len(list) > 0 {
		s.selected = list[0].ID
	}
}

func (p *panel) compareCheckpoint(s *checkpointState) {
	base, id := read(p.win), s.selected
	p.checkpointTask(s, func() func() {
		text, err := p.checkpoints.Read(p.ctx, s.id, s.path, id)
		var diff []review.DiffHunk
		if err == nil {
			diff = review.ComputeDiff(base.text, text)
		}
		return func() {
			if err != nil {
				s.errorText = err.Error()
				return
			}
			s.comparing, s.base, s.original, s.diff = true, base, text, diff
		}
	})
}

func (p *panel) checkpointTask(s *checkpointState, run func() func()) {
	s.busy, s.errorText = true, ""
	go func() {
		result := run()
		p.win.Native().Update(func() {
			if p.ctx.Err() != nil || p.checkpointDocs[s.id] != s {
				return
			}
			s.busy = false
			result()
		})
	}()
}
