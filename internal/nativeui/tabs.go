package nativeui

import (
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

func stateLabel(edited, conflict, deleted, readOnly, generating bool) string {
	states := documentStates(edited, conflict, deleted, readOnly, generating)
	tags := make([]string, len(states))
	for i, state := range states {
		tags[i] = state.label
	}
	return strings.Join(tags, " · ")
}

func (s *Shell) documents(c *ui.Context, m Model, a Actions) {
	docs := m.Workspace.Documents
	if len(docs) == 0 {
		return
	}
	labels := tabLabels(docs)
	ui.Scroll(c.Key("documents")).FillWidth().MaxHeight(portrait.Shell.Documents).Shrink(0).Role(ui.RoleTabList).Label("Documents").TrackScroll(s.scrollState("documents")).Children(func() {
		for i, d := range docs {
			selected := d.ID == s.activeID
			states := documentStates(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating)
			status := stateLabel(d.Edited, d.Conflict, d.Deleted, d.ReadOnly, d.Generating)
			row := ui.Row(c.Key(d.ID)).FillWidth().Height(portrait.Shell.Row).Radius(portrait.Radii.M).AlignItems(ui.Center).Padding(0, 5, 0, 0).Gap(4)
			hovered := row.Hovered()
			if hovered && !selected {
				row.Background(portrait.Materials.GlassHover)
			}
			row.Children(func() {
				portrait.RowCapsule(c, row, selected, "documents")
				tab := portrait.Tooltip(c, ui.ButtonBase(c.Key("tab")).Label(labels[d.ID]).Height(portrait.Shell.Row).Padding(0, 2, 0, 8).Gap(8).AlignItems(ui.Center).Justify(ui.Start).MinWidth(0).Grow(1).Shrink(1).ZIndex(1).Role(ui.RoleTab).TabStop(selected).AccessSelected(selected), d.Path)
				if status != "" {
					tab.Description(status)
				}
				ink := portrait.Colors.Ink2
				if selected || hovered {
					ink = portrait.Colors.Ink
				}
				tab.Children(func() {
					iconColor := portrait.Colors.Ink2
					if selected {
						ui.Box(c.Key("active-bar")).Size(3, portrait.Type.Small.Size).Radius(portrait.Radii.Pill).Background(portrait.Colors.Accent).Shrink(0).AccessHidden(true)
						iconColor = portrait.Colors.Accent
					}
					portrait.Glyph(c, portrait.IconFileText, portrait.Type.Small.Size, iconColor)
					portrait.Type.UI.Apply(ui.Text(c, labels[d.ID])).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
				})
				if s.tabFocus == d.ID && selected {
					tab.Focus()
					s.tabFocus = ""
					s.focusedTab = true
				}
				if tab.Clicked() && a.Activate != nil {
					s.leftOpen = false
					a.Activate(d.ID)
				}
				if tab.Focused() && len(docs) > 1 && a.Activate != nil {
					switch {
					case tab.Shortcut(0, ui.KeyDown), tab.Shortcut(0, ui.KeyRight):
						s.tabFocus = docs[(i+1)%len(docs)].ID
						a.Activate(s.tabFocus)
					case tab.Shortcut(0, ui.KeyUp), tab.Shortcut(0, ui.KeyLeft):
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
				closeTab := portrait.QuietIconButton(c, portrait.IconX, "Close "+labels[d.ID]).Size(portrait.Sizes.Compact, portrait.Sizes.Compact).Radius(portrait.Radii.S).ZIndex(1)
				if !hovered && !selected && !closeTab.Hovered() && !closeTab.Focused() {
					closeTab.Opacity(0)
				}
				if closeTab.Clicked() && a.CloseDocument != nil {
					a.CloseDocument(d.ID)
				}
			})
			contextOpen := ui.Local(row, "Portrait tab menu", func() bool { return false })
			portrait.ContextMenu(c, row, contextOpen, func(menu *portrait.MenuBuilder) {
				if menu.Item(portrait.MenuItemSpec{Label: "Activate", Icon: portrait.IconFileText}).Clicked() && a.Activate != nil {
					a.Activate(d.ID)
				}
				if menu.Item(portrait.MenuItemSpec{Label: "Close Document", Icon: portrait.IconX}).Clicked() && a.CloseDocument != nil {
					a.CloseDocument(d.ID)
				}
			})
		}
	})
}
