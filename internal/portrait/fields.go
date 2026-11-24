package portrait

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// Field keeps a text field's decorated frame and input together.
type Field struct {
	frame func() ui.Element
	input func() ui.Element
}

func (f Field) Frame() ui.Element { return f.frame() }
func (f Field) Input() ui.Element { return f.input() }

type FieldSpec struct {
	Label, Placeholder string
	Size               float32
	Leading            Icon
	Password           bool
	Invalid, Disabled  bool
	Trailing           func()
}

func TextField(c *ui.Context, value *string, spec FieldSpec) Field {
	height := spec.Size
	if height == 0 {
		height = Form.Field
	}
	label := spec.Label
	if label == "" {
		label = spec.Placeholder
	}
	frame := ui.Row(c).Height(height).Padding(0, Form.PadX).Gap(Form.Gap).Radius(Form.Radius).AlignItems(ui.Center)
	if spec.Disabled {
		frame.Opacity(0.5)
	}
	var input ui.Element
	frame.Children(func() {
		if spec.Leading.Glyph != "" {
			Glyph(c, spec.Leading, IconSizes.S, Colors.Ink3).Shrink(0)
		}
		input = Type.UI.Apply(ui.TextInputBase(c.Key("input"), value)).FocusRing(false).Grow(1).MinWidth(0).
			Label(label).Placeholder(spec.Placeholder).Invalid(spec.Invalid).Disabled(spec.Disabled)
		if spec.Password {
			input.Password()
		}
		if spec.Trailing != nil {
			spec.Trailing()
		}
	})
	applyFieldLook(frame, spec.Invalid)
	return Field{frame: func() ui.Element { return frame }, input: func() ui.Element { return input }}
}

func applyFieldLook(frame ui.Element, invalid bool) {
	if invalid {
		Surface{Colors.Paper, Colors.Paper, Rings.Invalid}.Apply(frame)
	} else if frame.FocusWithin() {
		Surface{Colors.Paper, Colors.Paper, Rings.Field}.Apply(frame)
	} else {
		Surface{Colors.Paper, Colors.Paper, Shadows{in(0, 0, 0, 1, Colors.Line)}}.Apply(frame)
	}
}

type SearchSpec struct {
	Placeholder, Label string
	ClearLabel         string
	Shortcut           []string
	Size               float32
	SingleFocusRing    bool
	OnClear            func()
}

func SearchField(c *ui.Context, value *string, spec SearchSpec) Field {
	if spec.Placeholder == "" {
		spec.Placeholder = "Search"
	}
	label := spec.Label
	if label == "" {
		label = spec.Placeholder
	}
	height := spec.Size
	if height == 0 {
		height = Form.Field
	}
	frame := ui.Row(c).Height(height).Padding(0, Form.PadX).Gap(Form.Gap).Radius(Form.Radius).AlignItems(ui.Center)
	var input ui.Element
	frame.Children(func() {
		Glyph(c, IconMagnifyingGlass, IconSizes.S, Colors.Ink3).Shrink(0)
		input = Type.UI.Apply(ui.TextInputBase(c.Key("input"), value)).FocusRing(false).Grow(1).MinWidth(0).Label(label).Placeholder(spec.Placeholder)
		if *value == "" && len(spec.Shortcut) != 0 {
			Kbd(c, spec.Shortcut...).Shrink(0)
		} else if *value != "" {
			clearLabel := spec.ClearLabel
			if clearLabel == "" {
				clearLabel = "Clear " + label
			}
			clear := ButtonWith(c, "", clearLabel, ButtonSpec{Look: Quiet, Size: Sizes.Compact, Leading: IconX})
			if clear.Clicked() {
				*value = ""
				if spec.OnClear != nil {
					spec.OnClear()
				}
			}
		}
	})
	if spec.SingleFocusRing && frame.FocusWithin() {
		Surface{Colors.Paper, Colors.Paper, Shadows{out(0, 0, 0, 3, Colors.Focus)}}.Apply(frame)
	} else {
		applyFieldLook(frame, false)
	}
	return Field{frame: func() ui.Element { return frame }, input: func() ui.Element { return input }}
}

type AreaSpec struct {
	Label, Placeholder string
	Rows               int
	MinHeight          float32
	Mono, Invalid      bool
	Disabled           bool
}

func TextArea(c *ui.Context, value *string, spec AreaSpec) ui.Element {
	if spec.Rows == 0 {
		spec.Rows = 3
	}
	height := max(Form.AreaMin, float32(spec.Rows)*Type.UI.Leading+2*Form.AreaPadY)
	if spec.MinHeight != 0 {
		height = max(spec.MinHeight, float32(spec.Rows)*Type.UI.Leading+2*Form.AreaPadY)
	}
	frame := ui.Row(c).Height(height).Padding(Form.AreaPadY, Form.PadX).Radius(Form.Radius).AlignItems(ui.Start)
	if spec.Disabled {
		frame.Opacity(0.5)
	}
	var input ui.Element
	frame.Children(func() {
		style := Type.UI
		if spec.Mono {
			style = Type.CodeSmall
		}
		input = style.Apply(ui.TextAreaBase(c.Key("input"), value)).FocusRing(false).Grow(1).MinWidth(0).MinHeight(0).Height(max(0, height-2*Form.AreaPadY)).
			Label(spec.Label).Placeholder(spec.Placeholder).Invalid(spec.Invalid).Disabled(spec.Disabled)
	})
	applyFieldLook(frame, spec.Invalid)
	return input
}

func FormRow(c *ui.Context, label, help string, control func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(Form.LabelGap).Children(func() {
		if label != "" {
			Type.Meta.Apply(ui.Text(c, label)).FontWeight(500).TextColor(Colors.Ink2)
		}
		if control != nil {
			control()
		}
		if help != "" {
			Caption(c, help).Wrap()
		}
	})
}

type Option struct{ Value, Label string }

type SelectSpec struct {
	Label string
	Size  float32
	Width float32
}

func Select(c *ui.Context, selected *string, options []Option, spec SelectSpec) ui.Element {
	root := ui.Row(c).AlignItems(ui.Center)
	open := ui.Local(root, "select open", func() bool { return false })
	var trigger ui.Element
	root.Children(func() {
		title := *selected
		for _, option := range options {
			if option.Value == *selected {
				title = option.Label
			}
		}
		trigger = Menu(c.Key("menu"), open, func(menu *MenuBuilder) {
			for _, option := range options {
				option := option
				item := menu.Radio(MenuItemSpec{Label: option.Label}, option.Value == *selected)
				if item.Clicked() {
					*selected = option.Value
				}
			}
		})
		trigger.Label(spec.Label).Height(spec.Size).Padding(0, Form.PadX).Radius(Form.Radius).Gap(Form.Gap).AlignItems(ui.Center)
		if spec.Size == 0 {
			trigger.Height(Form.FieldCompact)
		}
		if spec.Width != 0 {
			trigger.Width(spec.Width)
		} else {
			trigger.MinWidth(Form.SelectMin).MaxWidth(Form.SelectMax)
		}
		Press(trigger, Raised)
		trigger.Children(func() {
			Type.UI.Apply(ui.Text(c, title)).TextColor(Colors.Ink).Grow(1).MaxLines(1).Ellipsis("…")
			Glyph(c, IconCaretDown, IconSizes.XS, Colors.Ink3).Shrink(0)
		})
	})
	return trigger
}

func Stepper(c *ui.Context, value *float32, lo, hi, step float32, format func(float32) string) ui.Element {
	if format == nil {
		format = func(v float32) string { return fmt.Sprintf("%g", v) }
	}
	return Group(c, "Value", func(group *GroupBuilder) {
		minus := group.Add(ButtonWith(c, "", "Decrease value", ButtonSpec{Look: Raised, Size: Form.FieldCompact, Leading: IconMinus}).Disabled(*value <= lo))
		if minus.Clicked() {
			*value = max(lo, *value-step)
		}
		ui.Row(c).Height(Form.FieldCompact).MinWidth(44).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			Type.UI.Apply(ui.Text(c, format(*value))).TextColor(Colors.Ink)
		})
		plus := group.Add(ButtonWith(c, "", "Increase value", ButtonSpec{Look: Raised, Size: Form.FieldCompact, Leading: IconPlus}).Disabled(*value >= hi))
		if plus.Clicked() {
			*value = min(hi, *value+step)
		}
	})
}
