package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func statusTone(status connectionStatus) portrait.Tone {
	switch status {
	case statusReady:
		return portrait.Tones.Success
	case statusWorking, statusConnecting:
		return portrait.Tones.Warning
	default:
		return portrait.Tones.Neutral
	}
}

func text(c *ui.Context, step portrait.TypeStyle, value string) ui.Element {
	return step.Apply(ui.Text(c, value))
}

func option(c *ui.Context, key, title, description string, look portrait.Look, small, selected bool) ui.Element {
	size := portrait.Sizes.Regular
	if small {
		size = portrait.Sizes.Compact
	}
	return portrait.ButtonWith(c.Key(key), title, description, portrait.ButtonSpec{
		Look: look, Size: size, Selected: selected,
	}).MinWidth(0)
}

func controlButton(c *ui.Context, label, description string) ui.Element {
	return option(c, "research-control-"+label, label, description, portrait.Raised, false, false)
}

func smallButton(c *ui.Context, label, description string) ui.Element {
	return option(c, "research-small-"+label, label, description, portrait.Raised, true, false)
}

func toolButton(c *ui.Context, label, description string, small, selected bool) ui.Element {
	return option(c, "research-tool-"+label, label, description, portrait.Quiet, small, selected)
}

func choiceButton(c *ui.Context, key, label, description string, selected, small bool) ui.Element {
	return option(c, key, label, description, portrait.Quiet, small, selected).Checked(selected)
}

func discreteButton(c *ui.Context, label, description string, mark portrait.Icon) ui.Element {
	b := portrait.Tooltip(c, ui.ButtonBase(c.Key("research-option-"+label)).Label(description), description).FillWidth().MinHeight(28).Padding(5, 8).Radius(8).Gap(6).AlignItems(ui.Center)
	ink := portrait.Press(b, portrait.Quiet)
	b.Children(func() {
		if mark.Glyph != "" {
			portrait.Glyph(c, mark, 13, portrait.Colors.Accent).Shrink(0)
		}
		text(c, portrait.Type.UI, label).TextColor(ink).Grow(1).MinWidth(0).Wrap()
	})
	return b
}

func chip(c *ui.Context, key string, icon portrait.Icon, label, description string, compact bool) ui.Element {
	b := portrait.Tooltip(c, ui.ButtonBase(c.Key(key)).Label(description), description).Gap(4).AlignItems(ui.Center).Radius(5).MinWidth(0)
	if compact {
		b.Height(20).Padding(0, 5)
	} else {
		b.Height(22).Padding(0, 6)
	}
	portrait.Press(b, portrait.Quiet)
	portrait.Surface{From: portrait.Colors.AccentSoft, To: portrait.Colors.AccentSoft}.Apply(b)
	b.Children(func() {
		if icon.Glyph != "" {
			portrait.Glyph(c, icon, 12, portrait.Colors.Accent)
		}
		text(c, portrait.Type.Meta, label).TextColor(portrait.Colors.Accent).MaxLines(1).Ellipsis("…").MinWidth(0)
		portrait.Glyph(c, portrait.IconX, 12, portrait.Colors.Accent)
	})
	return b
}

func field(c *ui.Context, key string, size float32, build func() ui.Element, configure ...func(ui.Element)) ui.Element {
	frame := portrait.InputFrame(c.Key(key)).Height(size).FillWidth().MinWidth(0)
	frame.Children(func() {
		entry := portrait.Type.UI.Apply(build()).FocusRing(false).Grow(1).MinWidth(0)
		for _, fn := range configure {
			fn(entry)
		}
	})
	return frame
}

func input(c *ui.Context, key, label string, value *string, placeholder string, configure ...func(ui.Element)) ui.Element {
	return sizedInput(c, key, label, value, placeholder, portrait.Sizes.Large, configure...)
}

func filterInput(c *ui.Context, key, label string, value *string, placeholder string, configure ...func(ui.Element)) ui.Element {
	return sizedInput(c, key, label, value, placeholder, portrait.Sizes.Regular, configure...)
}

func sizedInput(c *ui.Context, key, label string, value *string, placeholder string, size float32, configure ...func(ui.Element)) ui.Element {
	return field(c, key, size, func() ui.Element {
		return ui.TextInputBase(c, value).Label(label).Placeholder(placeholder)
	}, configure...)
}

func area(c *ui.Context, key, label string, value *string, placeholder string, rows float32, configure ...func(ui.Element)) ui.Element {
	frame := field(c, key, rows, func() ui.Element {
		return ui.TextAreaBase(c, value).Label(label).Placeholder(placeholder).Height(rows)
	}, configure...)
	return frame.AlignItems(ui.Start)
}

func card(c *ui.Context) ui.Element {
	return portrait.Card(c)
}

func sectionTitle(c *ui.Context, title string) ui.Element {
	return text(c, portrait.Type.Heading, title).FontWeight(600).TextColor(portrait.Colors.Ink)
}

func mutedText(c *ui.Context, value string) {
	text(c, portrait.Type.Meta, value).TextColor(portrait.Colors.Ink3)
}

func resultText(c *ui.Context, title, value string, errText string) {
	if errText != "" {
		portrait.Alert(c, portrait.Tones.Danger, errText, "")
	}
	if value == "" {
		return
	}
	card(c.Key("result-"+title)).Padding(7, 8).Gap(3).Background(portrait.Colors.Code).Children(func() {
		text(c, portrait.Type.Meta, title).FontWeight(600).TextColor(portrait.Colors.Ink2)
		text(c, portrait.Type.Meta.In(portrait.FontMono), value).TextColor(portrait.Colors.Ink).Wrap().FillWidth()
	})
}

func markdownOptions(o *markdown.Options) *markdown.Options {
	base := portrait.Type.UI.Size
	o.Theme = &markdown.Theme{
		FontSize: base, Font: portrait.FontSans, CodeFont: portrait.FontMono, CodeSize: portrait.Type.Code.Size / base,
		CodeLabelSize: portrait.Type.Meta.Size / base, ListNumberSize: portrait.Type.Meta.Size / base, ImagePlaceholderSize: portrait.Type.Meta.Size / base,
		Text: portrait.Colors.Ink, Muted: portrait.Colors.Ink3, Link: portrait.Colors.Accent, Code: portrait.Colors.Ink, CodeBackground: portrait.Colors.Fill,
		QuoteBar: portrait.Colors.Accent, QuoteText: portrait.Colors.Ink2, Rule: portrait.Colors.Line,
		CodeBlockBackground: portrait.Colors.Fill, CodeBlockBorder: portrait.Colors.Line, CodeBlockRadius: portrait.Radii.M,
		TableBorder: portrait.Colors.Line, TableHeader: portrait.Colors.Fill, Checkbox: portrait.Colors.Accent,
		Headings: [6]markdown.HeadingStyle{
			{Size: portrait.Type.Reading.Size / base}, {Size: portrait.Type.Small.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base, Color: portrait.Colors.Ink2},
		},
	}
	return o
}
