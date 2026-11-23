package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func statusColor(status connectionStatus) ui.Color {
	switch status {
	case statusReady:
		return portrait.Colors.Accent
	case statusWorking, statusConnecting:
		return portrait.Colors.Warn
	default:
		return portrait.Colors.Ink3
	}
}

func text(c *ui.Context, step portrait.TypeStyle, value string) ui.Element {
	return step.Apply(ui.Text(c, value))
}

func option(c *ui.Context, key, title, description string, look portrait.Look, small, selected bool) ui.Element {
	b := portrait.Tooltip(c, ui.ButtonBase(c.Key(key)).Label(description), description).Gap(5).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0)
	step := portrait.Type.UI
	if small {
		b.Height(22).Padding(0, 8).Radius(6)
		step = portrait.Type.Meta
	} else {
		b.Height(28).Padding(0, 11).Radius(8)
	}
	ink := portrait.Press(b, look)
	if selected {
		portrait.Surface{From: portrait.Colors.AccentSoft, To: portrait.Colors.AccentSoft}.Apply(b)
		ink = portrait.Colors.Accent
	}
	b.Children(func() {
		text(c, step, title).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
	})
	return b
}

func controlButton(c *ui.Context, label, description string) ui.Element {
	return option(c, "research-control-"+label, label, description, portrait.Raised, false, false)
}

func smallButton(c *ui.Context, label, description string) ui.Element {
	return option(c, "research-small-"+label, label, description, portrait.Raised, true, false)
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

func field(e ui.Element) ui.Element {
	e.FillWidth().MinWidth(0).Radius(7).Border(1, portrait.Colors.LineStrong).Background(portrait.Colors.Paper)
	if e.Focused() {
		portrait.Rings.Field.Apply(e)
	}
	return portrait.Type.UI.Apply(e)
}

func input(c *ui.Context, key, label string, value *string, placeholder string) ui.Element {
	return field(ui.TextInput(c.Key(key), value).Label(label).Placeholder(placeholder).Height(31).Padding(5, 8))
}

func area(c *ui.Context, key, label string, value *string, placeholder string, rows float32) ui.Element {
	return field(ui.TextArea(c.Key(key), value).Label(label).Placeholder(placeholder).Height(rows).Padding(7, 8))
}

func card(e ui.Element) ui.Element {
	return e.Radius(9).Background(portrait.Colors.Fill)
}

func sectionTitle(c *ui.Context, title string) ui.Element {
	return text(c, portrait.Type.Reading.In(portrait.FontDisplay), title).FontWeight(600).TextColor(portrait.Colors.Ink)
}

func mutedText(c *ui.Context, value string) {
	text(c, portrait.Type.Meta, value).TextColor(portrait.Colors.Ink3)
}

func resultText(c *ui.Context, title, value string, errText string) {
	if errText != "" {
		text(c, portrait.Type.Meta, errText).Role(ui.RoleAlert).TextColor(portrait.Colors.Danger).FillWidth().Wrap()
	}
	if value == "" {
		return
	}
	ui.Column(c.Key("result-"+title)).Padding(7, 8).Gap(3).Radius(7).Background(portrait.Colors.Code).Children(func() {
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
		CodeBlockBackground: portrait.Colors.Fill, CodeBlockBorder: portrait.Colors.Line, CodeBlockRadius: 7,
		TableBorder: portrait.Colors.Line, TableHeader: portrait.Colors.Fill, Checkbox: portrait.Colors.Accent,
		Headings: [6]markdown.HeadingStyle{
			{Size: portrait.Type.Reading.Size / base}, {Size: portrait.Type.Small.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base}, {Size: portrait.Type.UI.Size / base, Color: portrait.Colors.Ink2},
		},
	}
	return o
}
