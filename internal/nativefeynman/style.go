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
	return portrait.Tooltip(c, portrait.ListRow(c.Key("research-option-"+label), portrait.ListRowSpec{Icon: mark, Title: label, Detail: description, Label: description}), description)
}

func chip(c *ui.Context, key string, icon portrait.Icon, label, description string, compact bool) ui.Element {
	_ = compact
	return portrait.Tooltip(c, portrait.Chip(c.Key(key), icon, label, nil).Label(description), description)
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
	return sizedInput(c, key, label, value, placeholder, portrait.Form.Field, configure...)
}

func filterInput(c *ui.Context, key, label string, value *string, placeholder string, configure ...func(ui.Element)) ui.Element {
	field := portrait.SearchField(c.Key(key), value, portrait.SearchSpec{Label: label, Placeholder: placeholder, Size: portrait.Form.FieldCompact})
	field.Frame().FillWidth().MinWidth(0)
	for _, fn := range configure {
		fn(field.Input())
	}
	return field.Frame()
}

func sizedInput(c *ui.Context, key, label string, value *string, placeholder string, size float32, configure ...func(ui.Element)) ui.Element {
	field := portrait.TextField(c.Key(key), value, portrait.FieldSpec{Label: label, Placeholder: placeholder, Size: size})
	field.Frame().FillWidth().MinWidth(0)
	for _, fn := range configure {
		fn(field.Input())
	}
	return field.Frame()
}

func area(c *ui.Context, key, label string, value *string, placeholder string, rows float32, configure ...func(ui.Element)) ui.Element {
	lineCount := max(1, int(rows/portrait.Type.UI.Leading))
	minHeight := portrait.Form.AreaMin
	if rows <= portrait.Form.AreaCompact {
		lineCount, minHeight = 1, rows
	}
	entry := portrait.TextArea(c.Key(key), value, portrait.AreaSpec{Label: label, Placeholder: placeholder, Rows: lineCount, MinHeight: minHeight})
	for _, fn := range configure {
		fn(entry)
	}
	return entry
}

func settingsSection(c *ui.Context, title, description string, build func()) {
	portrait.SettingsGroup(c, title, func(rows *portrait.Rows) {
		rows.Row(portrait.RowSpec{Title: title, Description: description, Stacked: true}, func() {
			ui.Column(c).FillWidth().Gap(portrait.Form.Gap).Children(build)
		})
	})
}

func card(c *ui.Context) ui.Element {
	return portrait.Card(c)
}

func sectionTitle(c *ui.Context, title string) ui.Element {
	return portrait.PanelTitle(c, title)
}

func mutedText(c *ui.Context, value string) {
	text(c, portrait.Type.Meta, value).TextColor(portrait.Colors.Ink3)
}

func resultText(c *ui.Context, title, value string, errText string) {
	if errText != "" {
		portrait.Alert(c, portrait.Tones.Danger, errText, "")
	}
	if value != "" {
		portrait.OutputBlock(c.Key("result-"+title), title, value)
	}
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
