package nativeui

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	settingsToolbarHeight float32 = 54
	settingsSwatchHeight  float32 = 32
)

type settingsTabSpec struct {
	id, label string
	icon      portrait.Icon
}

func (s *Shell) settingsDialog(c *ui.Context, width, height float32, m Model, a Actions, slots Slots) {
	if s.settingsOpen {
		ui.Overlay(c, func() {
			ui.Box(c.Key("settings scrim")).Absolute().Left(0).Top(0).Right(0).Bottom(0).IgnoreInput(true).
				Background(portrait.Colors.ModalScrim).Transition(portrait.Overlays.Scrim.Transition(c, c.KeyboardInput()))
		})
	}
	tabs := []settingsTabSpec{
		{id: "appearance", label: "Appearance", icon: portrait.IconPalette},
	}
	if slots.FeynmanSettings != nil {
		tabs = append(tabs,
			settingsTabSpec{id: "models", label: "Models & Auth", icon: portrait.IconCube},
			settingsTabSpec{id: "tools", label: "Tools & MCP", icon: portrait.IconPlugs},
			settingsTabSpec{id: "diagnostics", label: "Diagnostics", icon: portrait.IconSlidersHorizontal},
		)
	}
	if s.settingsTab == "" || (s.settingsTab != "appearance" && slots.FeynmanSettings == nil) {
		s.settingsTab = "appearance"
	}
	panel := ui.DialogBase(c.Key("settings dialog"), &s.settingsOpen, func(back, panel ui.Element) {
		back.AlignItems(ui.Center).Justify(ui.Center)
		w := min(portrait.Shell.SettingsWidth, max(float32(340), width-4*portrait.Shell.PageInset))
		h := min(portrait.Shell.SettingsHeight, max(float32(300), height-4*portrait.Shell.PageInset))
		panel.Width(w).Height(h).Radius(portrait.Radii.XL).Clip().Label("Settings")
		portrait.Materials.Pop.Apply(panel).TransformOrigin(0.5, 0.5).Transition(portrait.Overlays.Popup.Transition(c, c.KeyboardInput()))

		ui.Column(c).Fill().Children(func() {
			ui.Row(c.Key("settings-header")).FillWidth().Height(portrait.Shell.PageBar).Padding(0, 12, 0, 18).AlignItems(ui.Center).Children(func() {
				portrait.Type.Small.Apply(ui.Text(c, "Settings")).FontWeight(600).TextColor(portrait.Colors.Ink).Grow(1)
				if portrait.QuietIconButton(c, portrait.IconX, "Close settings").Clicked() {
					s.settingsOpen = false
				}
			})
			ui.Row(c.Key("settings-toolbar")).FillWidth().Height(settingsToolbarHeight).Padding(0, 16, 8, 16).Gap(6).
				AlignItems(ui.Center).Justify(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(portrait.Colors.Line).
				Role(ui.RoleTabList).Label("Settings sections").Children(func() {
				for _, tab := range tabs {
					selected := s.settingsTab == tab.id
					btn := ui.ButtonBase(c.Key("settings-tab-"+tab.id)).Role(ui.RoleTab).Label(tab.label).AccessSelected(selected).
						Padding(6, 12).Radius(portrait.Radii.M).AlignItems(ui.Center).Justify(ui.Center)
					if selected {
						btn.Background(portrait.Colors.AccentSoft)
					} else if btn.Hovered() {
						btn.Background(portrait.Colors.Fill)
					}
					ink := portrait.Colors.Ink2
					if selected {
						ink = portrait.Colors.Accent
					} else if btn.Hovered() {
						ink = portrait.Colors.Ink
					}
					btn.Children(func() {
						ui.Column(c).AlignItems(ui.Center).Gap(3).Children(func() {
							portrait.Glyph(c, tab.icon, portrait.Type.Reading.Size, ink)
							portrait.Type.Meta.Apply(ui.Text(c, tab.label)).FontWeight(500).TextColor(ink)
						})
					})
					if btn.Clicked() {
						s.settingsTab = tab.id
					}
				}
			})
			ui.Scroll(c.Key("settings-body-" + s.settingsTab)).Grow(1).MinHeight(collapsible).FillWidth().
				TrackScroll(s.scrollState("settings:" + s.settingsTab)).Label("Settings content").Children(func() {
				ui.Column(c).FillWidth().Padding(16, 20, 20, 20).Gap(14).Children(func() {
					if s.settingsTab == "appearance" || slots.FeynmanSettings == nil {
						s.appearanceSettings(c, m, a)
						return
					}
					slots.FeynmanSettings(c, s.settingsTab)
				})
			})
		})
	})
	_ = panel
}

func (s *Shell) appearanceSettings(c *ui.Context, m Model, a Actions) {
	activeSpace := spaceOf(m)
	ui.Column(c.Key("settings-appearance-mode")).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				portrait.Type.UI.Apply(ui.Text(c, "Appearance")).FontWeight(600).TextColor(portrait.Colors.Ink)
				portrait.Type.Meta.Apply(ui.Text(c, "Choose Light, Dark, or follow macOS automatically.")).TextColor(portrait.Colors.Ink3)
			})
			portrait.Track(c, "settings-appearance", func(t *portrait.Segments) {
				for _, opt := range portrait.Appearances {
					selected := opt == m.Appearance || (opt == portrait.System && m.Appearance == "")
					if t.Segment(c, string(opt), "Appearance "+string(opt), selected).Role(ui.RoleTab).AccessSelected(selected).Clicked() && a.SetAppearance != nil {
						a.SetAppearance(opt)
					}
				}
			}).Label("Appearance mode")
		})
	})

	ui.Column(c.Key("settings-wuxing-spaces")).FillWidth().Gap(8).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			portrait.Type.UI.Apply(ui.Text(c, "Wu Xing Space")).FontWeight(600).TextColor(portrait.Colors.Ink)
			portrait.Type.Meta.Apply(ui.Text(c, "Each workspace carries its own traditional pigment scenery and accent.")).TextColor(portrait.Colors.Ink3)
		})
		for row := 0; row < len(portrait.Spaces); row += 2 {
			ui.Row(c.Key("space-row-" + portrait.Spaces[row].Name)).FillWidth().Gap(10).Children(func() {
				for col := 0; col < 2 && row+col < len(portrait.Spaces); col++ {
					sp := portrait.Spaces[row+col]
					selected := sp.Name == activeSpace.Name
					sc := sp.In(s.dark)
					card := ui.ButtonBase(c.Key("settings-space-" + sp.Name)).Label("Space: " + sp.Label).AccessSelected(selected).
						Grow(1).MinWidth(0).Padding(8).Radius(portrait.Radii.L).Background(portrait.Colors.Paper)
					if selected {
						card.Border(1.5, portrait.Colors.Accent)
					} else {
						card.Border(1, portrait.Colors.Line)
						if card.Hovered() {
							card.Background(portrait.Colors.Fill)
						}
					}
					card.Children(func() {
						ui.Column(c).FillWidth().Gap(7).Children(func() {
							preview := ui.Row(c).FillWidth().Height(settingsSwatchHeight).Padding(6).Gap(6).Radius(portrait.Radii.M).AlignItems(ui.Center)
							preview.LinearGradient(ui.LinearGradient{
								Angle: 160,
								Oklab: true,
								Stops: []ui.GradientStop{
									{Color: sc.From.Mix(sc.To, 0.22), At: 0},
									{Color: sc.From.Mix(sc.To, 0.64), At: 0.56},
									{Color: sc.To, At: 1},
								},
							})
							preview.Children(func() {
								ui.Box(c).Size(12, 12).Radius(portrait.Radii.Pill).Background(sc.Mark).Shrink(0)
								ui.Row(c).Grow(1).FillHeight().Radius(portrait.Radii.S).Background(sc.Paper).Padding(0, 8).AlignItems(ui.Center).Children(func() {
									ui.Box(c).Size(24, 5).Radius(portrait.Radii.Pill).Background(sc.Accent)
								})
							})
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
								ui.Column(c).Grow(1).MinWidth(0).Children(func() {
									portrait.Type.UI.Apply(ui.Text(c, sp.Label)).FontWeight(600).TextColor(portrait.Colors.Ink).MaxLines(1).Ellipsis("…")
									portrait.Type.Meta.Apply(ui.Text(c, sp.Element)).TextColor(portrait.Colors.Ink3).MaxLines(1)
								})
								if selected {
									portrait.Glyph(c, portrait.IconCheckCircle, portrait.Type.Reading.Size, portrait.Colors.Accent).Shrink(0)
								}
							})
						})
					})
					if card.Clicked() && a.SetSpace != nil {
						a.SetSpace(sp.Name)
					}
				}
			})
		}
	})

	ui.Column(c.Key("settings-plugins")).FillWidth().Gap(8).Padding(10, 12).Radius(portrait.Radii.L).Background(portrait.Colors.Fill).Children(func() {
		portrait.Type.UI.Apply(ui.Text(c, "Optional Plugins")).FontWeight(600).TextColor(portrait.Colors.Ink)
		feynmanOn := m.Feynman
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				portrait.Type.UI.Apply(ui.Text(c, "Feynman Research")).FontWeight(500).TextColor(portrait.Colors.Ink)
				portrait.Type.Meta.Apply(ui.Text(c, "Side-by-side AI research assistant, models, and MCP tools.")).TextColor(portrait.Colors.Ink3)
			})
			if portrait.Switch(c.Key("settings-plugin-feynman"), &feynmanOn).Label("Enable Feynman").Changed() && a.SetFeynman != nil {
				a.SetFeynman(feynmanOn)
			}
		})
		gitOn := m.Git
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				portrait.Type.UI.Apply(ui.Text(c, "Git Checkpoints")).FontWeight(500).TextColor(portrait.Colors.Ink)
				portrait.Type.Meta.Apply(ui.Text(c, "Save, diff, and restore local document snapshots.")).TextColor(portrait.Colors.Ink3)
			})
			if portrait.Switch(c.Key("settings-plugin-git"), &gitOn).Label("Enable Git").Changed() && a.SetGit != nil {
				a.SetGit(gitOn)
			}
		})
	})
}
