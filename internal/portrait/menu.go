package portrait

import "github.com/egoist/mygo/ui"

type MenuItemSpec struct {
	Label, Detail string
	Icon          Icon
	Shortcut      []string
	Danger        bool
	Disabled      bool
}

type MenuBuilder struct {
	context func() *ui.Context
	parts   func() ui.MenuParts
}

type MenuSpec struct {
	TopOffset float32
}

// Menu creates an unstyled framework menu trigger and paints its popup with
// Portrait materials. The caller styles and fills the returned trigger.
func Menu(c *ui.Context, open *bool, build func(*MenuBuilder)) ui.Element {
	return MenuWithSpec(c, open, MenuSpec{}, build)
}

// MenuWithSpec also permits a trigger to reserve space between itself and its
// popup, as the workspace switcher does for the search row beneath it.
func MenuWithSpec(c *ui.Context, open *bool, spec MenuSpec, build func(*MenuBuilder)) ui.Element {
	parts := ui.MenuBase(c, open)
	builder := &MenuBuilder{context: func() *ui.Context { return c }, parts: func() ui.MenuParts { return parts }}
	parts.Popup(func(panel ui.Element) {
		if spec.TopOffset != 0 {
			panel.Margin(spec.TopOffset, 0, 0, 0)
		}
		panel.MinWidth(Form.MenuMin).MaxWidth(Form.MenuMax).Padding(Form.MenuPad).Gap(Form.PopoverGap).Radius(Radii.L)
		Materials.GlassOverlay.Apply(panel)
		build(builder)
	})
	return parts.Trigger
}

func ContextMenu(c *ui.Context, target ui.Element, open *bool, build func(*MenuBuilder)) ui.Element {
	parts := ui.ContextMenuBase(c, target, open)
	builder := &MenuBuilder{context: func() *ui.Context { return c }, parts: func() ui.MenuParts { return parts }}
	parts.Popup(func(panel ui.Element) {
		panel.MinWidth(Form.MenuMin).MaxWidth(Form.MenuMax).Padding(Form.MenuPad).Gap(Form.PopoverGap).Radius(Radii.L)
		Materials.GlassOverlay.Apply(panel)
		build(builder)
	})
	return target
}

func (m *MenuBuilder) Item(spec MenuItemSpec) ui.Element {
	return m.item(m.parts().Item(), spec, false)
}

func (m *MenuBuilder) Check(spec MenuItemSpec, checked bool) ui.Element {
	return m.item(m.parts().CheckItem(checked), spec, checked)
}

func (m *MenuBuilder) Radio(spec MenuItemSpec, selected bool) ui.Element {
	return m.item(m.parts().RadioItem(selected), spec, selected)
}

func (m *MenuBuilder) item(item ui.Element, spec MenuItemSpec, checked bool) ui.Element {
	ink := Colors.Ink
	if spec.Danger {
		ink = Colors.Danger
	}
	item.Label(spec.Label).Height(Form.MenuRow).Padding(0, Form.PadX-2).Gap(Form.Gap).Radius(Form.MenuRadius).
		AlignItems(ui.Center).Disabled(spec.Disabled)
	if item.Highlighted() {
		item.Background(Colors.Fill)
	}
	item.Children(func() {
		if spec.Icon.Glyph != "" {
			Glyph(m.context(), spec.Icon, IconSizes.M, ink).Shrink(0)
		}
		ui.Column(m.context()).Grow(1).MinWidth(0).Children(func() {
			Type.UI.Apply(ui.Text(m.context(), spec.Label)).TextColor(ink).MaxLines(1).Ellipsis("…")
			if spec.Detail != "" {
				Type.Meta.Apply(ui.Text(m.context(), spec.Detail)).TextColor(Colors.Ink3).MaxLines(1).Ellipsis("…")
			}
		})
		if len(spec.Shortcut) != 0 {
			Kbd(m.context(), spec.Shortcut...).Shrink(0)
		}
		if checked {
			Glyph(m.context(), IconCheck, IconSizes.S, Colors.Accent).Shrink(0)
		}
	})
	return item
}

func (m *MenuBuilder) Separator() {
	ui.Box(m.context()).FillWidth().Height(Editor.Hairline).Margin(Form.MenuPad, 0).Background(Colors.Line).AccessHidden(true)
}

func (m *MenuBuilder) Section(title string) {
	ui.Row(m.context()).MinHeight(Form.MenuRow).Padding(0, Form.PadX).AlignItems(ui.Center).Children(func() {
		SectionLabel(m.context(), title)
	})
}

func (m *MenuBuilder) Footer(build func()) {
	ui.Row(m.context()).FillWidth().Padding(Form.MenuPad, Form.PadX, 0, Form.PadX).BorderWidth(Editor.Hairline, 0, 0, 0).BorderColor(Colors.Line).Children(build)
}
