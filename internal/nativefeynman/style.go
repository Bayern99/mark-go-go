package nativefeynman

import "github.com/egoist/mygo/ui"

var (
	paperColor   = ui.Hex("#ffffff")
	inkColor     = ui.Hex("#1d1d1f")
	mutedColor   = ui.Hex("#6e6e73")
	accentColor  = ui.Hex("#2f6f62")
	errorColor   = ui.Hex("#a83a3a")
	warningColor = ui.Hex("#9a4d12")
	trackColor   = ui.Hex("#f2f2f4")
	rimColor     = ui.RGBA(0, 0, 0, 0.10)
)

func statusColor(status connectionStatus) ui.Color {
	switch status {
	case statusReady:
		return accentColor
	case statusWorking, statusConnecting:
		return warningColor
	default:
		return mutedColor
	}
}

func controlButton(c *ui.Context, label, description string) ui.Element {
	button := ui.Button(c.Key("research-control-"+label), label).Label(description).Tooltip(description).MinHeight(27).Padding(4, 8).Radius(7)
	if button.Hovered() {
		button.Background(trackColor)
	}
	return button
}

func discreteButton(c *ui.Context, label, description string) ui.Element {
	button := ui.Button(c.Key("research-option-"+label), label).Label(description).Tooltip(description).FillWidth().MinHeight(27).Padding(4, 7).Radius(6).AlignItems(ui.Start)
	if button.Hovered() {
		button.Background(trackColor)
	}
	return button
}

func smallButton(c *ui.Context, label, description string) ui.Element {
	button := ui.Button(c.Key("research-small-"+label), label).Label(description).Tooltip(description).MinHeight(25).Padding(3, 7).Radius(6).FontSize(12)
	if button.Hovered() {
		button.Background(trackColor)
	}
	return button
}

func input(c *ui.Context, key, label string, value *string, placeholder string) ui.Element {
	return ui.TextInput(c.Key(key), value).Label(label).Placeholder(placeholder).FillWidth().MinWidth(0).Height(31).Radius(7).Border(0.7, rimColor).Padding(5, 8)
}

func area(c *ui.Context, key, label string, value *string, placeholder string, rows float32) ui.Element {
	return ui.TextArea(c.Key(key), value).Label(label).Placeholder(placeholder).FillWidth().MinWidth(0).Height(rows).Radius(7).Border(0.7, rimColor).Padding(7, 8)
}

func sectionTitle(c *ui.Context, title string) {
	ui.Text(c, title).Font("Barlow Semi Condensed").FontSize(17).Bold().TextColor(inkColor).Padding(3, 0)
}

func mutedText(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(11).TextColor(mutedColor)
}

func resultText(c *ui.Context, title, text string, errText string) {
	if errText != "" {
		ui.Text(c, errText).Role(ui.RoleAlert).TextColor(errorColor).FontSize(11)
	}
	if text == "" {
		return
	}
	ui.Column(c.Key("result-"+title)).Padding(6, 7).Gap(3).Radius(7).Background(trackColor).Children(func() {
		ui.Text(c, title).FontSize(10).Bold().TextColor(mutedColor)
		ui.Text(c, text).Font("IBM Plex Mono").FontSize(10).TextColor(inkColor).Wrap().FillWidth()
	})
}
