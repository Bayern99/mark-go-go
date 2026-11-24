package portrait

import "github.com/egoist/mygo/ui"

type RowSpec struct {
	Title, Description string
	Reset              func()
	Stacked            bool
}

type Rows struct {
	context func() *ui.Context
	first   bool
}

// SettingsGroup builds a section heading and its rounded, divided row card.
func SettingsGroup(c *ui.Context, title string, build func(*Rows)) ui.Element {
	return ui.Column(c).FillWidth().Gap(Settings.HeadingGap).Children(func() {
		if title != "" {
			GroupHeading(c, title)
		}
		card := ui.Column(c).FillWidth().Radius(Radii.L).Clip().Background(Colors.Fill).Border(Editor.Hairline, Colors.Line)
		rows := &Rows{context: func() *ui.Context { return c }, first: true}
		card.Children(func() { build(rows) })
	})
}

func (r *Rows) Row(spec RowSpec, control func()) ui.Element {
	if !r.first {
		ui.Box(r.context()).FillWidth().Height(Editor.Hairline).Background(Colors.Line).AccessHidden(true)
	}
	r.first = false
	minHeight := Settings.RowCompact
	if spec.Description != "" {
		minHeight = Settings.Row
	}
	row := ui.Column(r.context()).FillWidth().MinHeight(minHeight).Padding(Settings.RowPadY, Settings.RowPadX).
		Justify(ui.Center).Role(ui.RoleGroup).Label(spec.Title)
	row.Children(func() {
		content := ui.Row(r.context()).FillWidth().Gap(Settings.RowGap).AlignItems(ui.Center)
		if spec.Stacked {
			content = ui.Column(r.context()).FillWidth().Gap(Settings.RowPadY).AlignItems(ui.Start)
		}
		content.Children(func() {
			ui.Column(r.context()).Grow(1).MinWidth(0).Gap(Form.HelpGap).Children(func() {
				ui.Row(r.context()).FillWidth().Gap(Form.Gap).AlignItems(ui.Center).Children(func() {
					Label(r.context(), spec.Title).MaxLines(2).Ellipsis("…")
					if spec.Reset != nil {
						reset := ButtonWith(r.context(), "", "Reset "+spec.Title, ButtonSpec{Look: Quiet, Size: Sizes.Compact, Leading: IconArrowCounterClockwise})
						if reset.Clicked() {
							spec.Reset()
						}
					}
				})
				if spec.Description != "" {
					Caption(r.context(), spec.Description).MaxWidth(Settings.DescriptionMax).MaxLines(2).Ellipsis("…").Wrap()
				}
			})
			if control != nil {
				var holder ui.Element
				if spec.Stacked {
					holder = ui.Column(r.context()).FillWidth().Gap(Form.Gap).AlignItems(ui.Start)
				} else {
					holder = ui.Row(r.context()).Shrink(0).AlignItems(ui.Center).Justify(ui.End)
				}
				holder.Children(control)
			}
		})
	})
	return row
}

type DisclosureSpec struct {
	Status  string
	Label   string
	Tone    Tone
	Leading Icon
}

func Disclosure(c *ui.Context, title string, open *bool, spec DisclosureSpec, build func()) ui.Element {
	root := ui.Column(c).FillWidth()
	root.Children(func() {
		parts := ui.CollapsibleBase(c.Key("disclosure"), open)
		label := spec.Label
		if label == "" {
			label = title
		}
		trigger := parts.Trigger.FillWidth().Height(Sizes.Regular).Padding(0, Form.PadX).Gap(Form.Gap).Radius(Radii.M).AlignItems(ui.Center).Label(label)
		RowHover(trigger, trigger.Hovered())
		trigger.Children(func() {
			caret := Glyph(c, IconCaretRight, IconSizes.XS, Colors.Ink3)
			caret.TransformOrigin(0.5, 0.5).Rotate(90 * parts.Progress())
			if spec.Leading.Glyph != "" {
				Glyph(c, spec.Leading, IconSizes.M, Colors.Ink2)
			}
			Label(c, title).Grow(1).MaxLines(1).Ellipsis("…")
			if spec.Status != "" {
				if spec.Tone != (Tone{}) {
					// A tool or step's outcome reads as a status badge, not as
					// a green word beside the title.
					Badge(c, spec.Status, spec.Tone)
				} else {
					Type.Meta.Apply(ui.Text(c, spec.Status)).TextColor(Colors.Ink3).MaxLines(1)
				}
			}
		})
		if panel := parts.Panel(build); panel.Valid() {
			panel.Padding(Form.AreaPadY, Form.PadX).PaddingStart(Form.PadX + Form.AreaPadY).Gap(Form.Gap)
		}
	})
	return root
}

func OutputBlock(c *ui.Context, title, value string) ui.Element {
	root := ui.Column(c).FillWidth().Radius(Radii.M).Clip().Background(Colors.Code).Border(Editor.Hairline, Colors.Line)
	root.Children(func() {
		ui.Row(c).FillWidth().Padding(Form.Gap, Form.PadX).AlignItems(ui.Center).Children(func() {
			Caption(c, title).Grow(1)
			copy := ButtonWith(c, "Copy", "Copy "+title, ButtonSpec{Look: Quiet, Size: Sizes.Compact, Leading: IconCopy})
			if copy.Clicked() {
				c.WriteClipboard(value)
			}
		})
		ui.Scroll(c.Key("output-scroll")).FillWidth().MaxHeight(Form.ListMax).Children(func() {
			Type.CodeSmall.Apply(ui.Text(c, value)).TextColor(Colors.Ink2).Padding(0, Form.PadX, Form.AreaPadY, Form.PadX).Selectable().Wrap()
		})
	})
	return root
}

func Chip(c *ui.Context, icon Icon, label string, onRemove func()) ui.Element {
	row := ui.ButtonBase(c).Label(label).Height(Sizes.Compact).Padding(0, Form.Gap).Gap(Form.HelpGap).Radius(Radii.S).AlignItems(ui.Center)
	Press(row, Quiet)
	row.Background(Colors.AccentSoft)
	row.Children(func() {
		if icon.Glyph != "" {
			Glyph(c, icon, IconSizes.XS, Colors.Accent)
		}
		Type.Meta.Apply(ui.Text(c, label)).TextColor(Colors.Accent).MaxLines(1).Ellipsis("…")
		if onRemove != nil {
			remove := ButtonWith(c, "", "Remove "+label, ButtonSpec{Look: Quiet, Size: Sizes.Compact, Leading: IconX})
			if remove.Clicked() {
				onRemove()
			}
		}
	})
	return row
}

type ListRowSpec struct {
	Icon          Icon
	Title, Detail string
	Trailing      func()
	Selected      bool
	Group, Label  string
	// IconColumn keeps the text of every row in a list at one x, whether the
	// row shows an icon or not, as a check mark beside the current one does.
	IconColumn bool
}

func ListRow(c *ui.Context, spec ListRowSpec) ui.Element {
	label := spec.Label
	if label == "" {
		label = spec.Title
	}
	row := ui.ButtonBase(c).Label(label).FillWidth().MinHeight(Sizes.Regular).Padding(Form.HelpGap, Form.PadX).Gap(Form.Gap).
		Radius(Radii.M).AlignItems(ui.Center).AccessSelected(spec.Selected)
	if spec.Selected && spec.Group != "" {
		RowCapsule(c, row, true, spec.Group)
	} else {
		RowHover(row, row.Hovered())
	}
	row.Children(func() {
		switch {
		case spec.Icon.Glyph != "":
			Glyph(c, spec.Icon, IconSizes.M, Colors.Ink2).Shrink(0)
		case spec.IconColumn:
			ui.Box(c).Size(IconSizes.M, IconSizes.M).Shrink(0).AccessHidden(true)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			Label(c, spec.Title).MaxLines(1).Ellipsis("…")
			if spec.Detail != "" {
				Caption(c, spec.Detail).MaxLines(2).Ellipsis("…")
			}
		})
		if spec.Trailing != nil {
			spec.Trailing()
		}
	})
	return row
}
