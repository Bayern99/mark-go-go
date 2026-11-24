package nativefeynman

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

type researchPickerItem struct {
	value       string
	label       string
	detail      string
	description string
	current     bool
}

type researchPicker struct {
	key      string
	label    string
	current  string
	icon     portrait.Icon
	items    []researchPickerItem
	disabled bool
	onSelect func(value string)
}

type researchPickerState struct {
	query, provider string
	cursor, focus   int
	reset           bool
}

func (p *panel) pickerState(key string) *researchPickerState {
	if p.pickerStates == nil {
		p.pickerStates = make(map[string]*researchPickerState)
	}
	if p.pickerStates[key] == nil {
		p.pickerStates[key] = &researchPickerState{focus: -1}
	}
	return p.pickerStates[key]
}

func researchModelKey(model feynman.ResearchModel) string {
	if model.Provider != "" {
		return model.Provider + "/" + model.ID
	}
	return model.ID
}

func researchModelLabel(model feynman.ResearchModel) string {
	if model.Name != "" {
		return model.Name
	}
	return model.ID
}

func researchLevelLabel(level string) string {
	if level == "" {
		return ""
	}
	return strings.ToUpper(level[:1]) + level[1:]
}

func isModelConfig(config feynman.ResearchConfig) bool {
	return config.ID == "model" || config.Category == "model"
}

func isThinkingConfig(config feynman.ResearchConfig) bool {
	return config.ID == "thinking_level" || config.Category == "thinking"
}

func configItems(config feynman.ResearchConfig, current string, description func(feynman.ResearchConfigValue) string) []researchPickerItem {
	items := make([]researchPickerItem, 0, len(config.Options))
	for _, option := range config.Options {
		items = append(items, researchPickerItem{
			value: option.Value, label: option.Name, detail: option.Description,
			description: description(option), current: option.Value == current,
		})
	}
	return items
}

func (p *panel) researchConfigPickers() []researchPicker {
	s := p.state
	disabled := s.status != statusReady || s.cliBusy
	pickers := make([]researchPicker, 0, 2+len(s.configs))

	var modelConfig *feynman.ResearchConfig
	for i := range s.configs {
		if isModelConfig(s.configs[i]) {
			modelConfig = &s.configs[i]
			break
		}
	}
	currentModel := ""
	if s.session.Model != nil {
		currentModel = researchModelKey(*s.session.Model)
	} else if modelConfig != nil {
		currentModel = modelConfig.CurrentValue
	}
	var modelItems []researchPickerItem
	if len(s.models) > 0 {
		modelItems = make([]researchPickerItem, 0, len(s.models))
		for _, model := range s.models {
			value := researchModelKey(model)
			modelItems = append(modelItems, researchPickerItem{
				value: value, label: researchModelLabel(model), detail: model.Provider,
				description: "Use " + model.Provider + " model " + model.ID,
				current:     value == currentModel,
			})
		}
	} else if modelConfig != nil {
		modelItems = configItems(*modelConfig, currentModel, func(option feynman.ResearchConfigValue) string {
			return "Use " + option.Name + " as the research model"
		})
	}
	if len(modelItems) > 0 {
		pickers = append(pickers, researchPicker{
			key: "model", label: "Model", current: currentModel, icon: portrait.IconCube,
			items: modelItems, disabled: disabled, onSelect: p.setResearchModel,
		})
	}

	var thinkingConfig *feynman.ResearchConfig
	for i := range s.configs {
		if isThinkingConfig(s.configs[i]) {
			thinkingConfig = &s.configs[i]
			break
		}
	}
	currentThinking := s.thinkingLevel
	if currentThinking == "" {
		currentThinking = s.session.ThinkingLevel
	}
	if currentThinking == "" && thinkingConfig != nil {
		currentThinking = thinkingConfig.CurrentValue
	}
	var thinkingItems []researchPickerItem
	if len(s.thinking) > 0 {
		thinkingItems = make([]researchPickerItem, 0, len(s.thinking))
		for _, level := range s.thinking {
			thinkingItems = append(thinkingItems, researchPickerItem{
				value: level, label: researchLevelLabel(level),
				description: "Set thinking level to " + level, current: level == currentThinking,
			})
		}
	} else if thinkingConfig != nil {
		thinkingItems = configItems(*thinkingConfig, currentThinking, func(option feynman.ResearchConfigValue) string {
			return "Set thinking level to " + option.Value
		})
	}
	if len(thinkingItems) > 0 {
		pickers = append(pickers, researchPicker{
			key: "thinking", label: "Thinking", current: currentThinking, icon: portrait.IconSlidersHorizontal,
			items: thinkingItems, disabled: disabled,
			onSelect: func(value string) {
				if len(p.state.thinking) > 0 {
					p.setResearchThinking(value)
					return
				}
				p.setResearchConfig("thinking_level", value)
			},
		})
	}

	for i := range s.configs {
		config := &s.configs[i]
		if isModelConfig(*config) || isThinkingConfig(*config) || len(config.Options) == 0 {
			continue
		}
		label := config.Name
		if label == "" {
			label = config.ID
		}
		id := config.ID
		pickers = append(pickers, researchPicker{
			key: "config-" + id, label: label, current: config.CurrentValue, icon: portrait.IconSlidersHorizontal,
			items: configItems(*config, config.CurrentValue, func(option feynman.ResearchConfigValue) string {
				return "Set " + label + " to " + option.Name
			}),
			disabled: disabled,
			onSelect: func(value string) { p.setResearchConfig(id, value) },
		})
	}
	return pickers
}

func (p *panel) setResearchModel(value string) {
	s := p.state
	for _, model := range s.models {
		if researchModelKey(model) != value {
			continue
		}
		p.runUpdate(func() (func(), error) {
			got, err := p.host.SetModel(p.window.Context(), s.root, model.Provider, model.ID)
			return func() { s.session.Model = &got }, err
		})
		return
	}
	if provider, modelID, ok := strings.Cut(value, "/"); ok && provider != "" && modelID != "" {
		p.runUpdate(func() (func(), error) {
			got, err := p.host.SetModel(p.window.Context(), s.root, provider, modelID)
			return func() { s.session.Model = &got }, err
		})
		return
	}
	p.setResearchConfig("model", value)
}

func (p *panel) setResearchThinking(level string) {
	s := p.state
	p.runUpdate(func() (func(), error) {
		err := p.host.SetThinkingLevel(p.window.Context(), s.root, level)
		return func() { s.thinkingLevel = level }, err
	})
}

func (p *panel) setResearchConfig(id, value string) {
	s := p.state
	p.runUpdate(func() (func(), error) {
		err := p.host.SetConfig(p.window.Context(), s.root, id, value)
		return func() {
			for i := range s.configs {
				if s.configs[i].ID == id {
					s.configs[i].CurrentValue = value
				}
			}
			if isThinkingConfig(feynman.ResearchConfig{ID: id}) {
				s.thinkingLevel = value
			}
		}, err
	})
}

func currentResearchPickerLabel(picker researchPicker) string {
	for _, item := range picker.items {
		if item.value == picker.current {
			return item.label
		}
	}
	if picker.current != "" {
		return picker.current
	}
	return picker.label
}

func matchingResearchPickerItems(items []researchPickerItem, query string) []researchPickerItem {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	matches := make([]researchPickerItem, 0, len(items))
	for _, item := range items {
		search := strings.ToLower(item.label + " " + item.value + " " + item.detail)
		if strings.Contains(search, query) {
			matches = append(matches, item)
		}
	}
	return matches
}

func (p *panel) closeResearchPickers() {
	for key, open := range p.collapsed {
		if strings.HasPrefix(key, "research-picker-") && open != nil {
			*open = false
		}
	}
}

func (p *panel) configControls(c *ui.Context) {
	pickers := p.researchConfigPickers()
	if len(pickers) == 0 {
		return
	}
	ui.Row(c.Key("research-configs")).Wrap().Gap(2).Padding(4, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(portrait.Colors.Line).AlignItems(ui.Center).Children(func() {
		for _, picker := range pickers {
			p.configPicker(c, picker)
		}
	})
}

func (p *panel) configPicker(c *ui.Context, picker researchPicker) {
	state := p.pickerState(picker.key)
	open := p.collapsibleState("research-picker-" + picker.key)
	width, height := c.Size()
	currentLabel := currentResearchPickerLabel(picker)
	description := picker.label + ": " + currentLabel
	button := ui.ButtonBase(c.Key("research-config-"+picker.key)).Label(picker.label).Value(currentLabel).
		Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 6).Radius(6).MinWidth(0).
		MaxWidth(max(float32(96), width*0.62)).Expanded(*open).Disabled(picker.disabled)
	button = portrait.Tooltip(c, button, description)
	ink := portrait.Press(button, portrait.Quiet)
	button.Children(func() {
		portrait.Glyph(c, picker.icon, 13, portrait.Colors.Ink3).Shrink(0)
		text(c, portrait.Type.Meta, currentLabel).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…").MinWidth(0)
		portrait.Glyph(c, portrait.IconCaretDown, 11, portrait.Colors.Ink3).Shrink(0)
	})
	if button.Clicked() {
		shouldOpen := !*open
		p.closeResearchPickers()
		*open = shouldOpen
		if shouldOpen {
			state.query, state.provider, state.reset = "", "", true
			state.focus = -1
			p.pickerFocus = picker.key
		}
	}

	popover := ui.PopoverBase(c, button, open, func(panel ui.Element) {
		panel.AttachTo(button, ui.AnchorTopStart, ui.AnchorBottomStart).
			MinWidth(button.Bounds().W).MaxHeight(max(float32(120), height-16)).
			Margin(0, 0, 6, 0).Padding(5).Radius(12).Gap(2).Label(picker.label + " options")
		portrait.Materials.GlassOverlay.Apply(panel)

		rows := p.pickerRows(picker, state)
		if state.reset {
			state.reset = false
			state.cursor = max(0, slices.IndexFunc(rows, func(r pickerRow) bool { return r.kind == pickerChoice && r.item.current }))
		}
		state.cursor = max(0, min(state.cursor, len(rows)-1))
		activate := func(row pickerRow) {
			switch row.kind {
			case pickerProvider:
				state.provider, state.cursor, state.focus = row.provider, 1, 1
				p.pickerFocus = picker.key
			case pickerBack:
				state.provider, state.cursor, state.focus = "", 0, -1
				p.pickerFocus = picker.key
			default:
				chooseResearchPickerValue(picker, row.item.value, open, state, button, p)
			}
		}
		panel.ObserveInput(func(event ui.InputEvent) {
			if event.Kind != ui.InputKeyDown || event.Mods != 0 || len(rows) == 0 {
				return
			}
			switch event.Key {
			case ui.KeyDown:
				state.cursor = (state.cursor + 1) % len(rows)
			case ui.KeyUp:
				state.cursor = (state.cursor + len(rows) - 1) % len(rows)
			default:
				return
			}
			state.focus = state.cursor
			p.pickerFocus = picker.key
		})

		ui.Column(c).Width(min(float32(302), max(float32(120), width-16))).Children(func() {
			portrait.InputFrame(c).Height(portrait.Sizes.Regular).Children(func() {
				portrait.Glyph(c, portrait.IconMagnifyingGlass, 13, portrait.Colors.Ink3)
				filter := portrait.Type.UI.Apply(ui.TextInputBase(c, &state.query)).
					FocusRing(false).Label("Find " + picker.label).
					Placeholder("Find " + strings.ToLower(picker.label) + "…").Grow(1).MinWidth(0)
				if filter.Changed() {
					state.cursor = 0
				}
				if p.pickerFocus == picker.key && state.focus == -1 {
					filter.Focus()
					p.pickerFocus = ""
				}
				if filter.Submitted() && len(rows) > 0 {
					activate(rows[state.cursor])
				}
			})
			ui.Scroll(c.Key("research-picker-options-" + picker.key)).
				MaxHeight(min(float32(250), max(float32(80), height-64))).FillWidth().
				Label(picker.label + " choices").Children(func() {
				if len(rows) == 0 {
					text(c, portrait.Type.Meta, "No matching options.").TextColor(portrait.Colors.Ink3).Padding(6, 8)
					return
				}
				for i, row := range rows {
					choice := p.pickerRowView(c, picker, row)
					if p.pickerFocus == picker.key && state.focus == i {
						choice.Focus().ScrollIntoView()
						p.pickerFocus = ""
					}
					if choice.Clicked() {
						activate(row)
					}
				}
			})
			if picker.key == "model" {
				ui.Row(c.Key("research-picker-manage")).FillWidth().Padding(4, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(portrait.Colors.Line).Children(func() {
					manage := ui.ButtonBase(c.Key("manage-models")).Label("Manage models and providers").FillWidth().Height(portrait.Sizes.Regular).Padding(0, 8).Radius(portrait.Radii.S).AlignItems(ui.Center)
					ink := portrait.Press(manage, portrait.Quiet)
					manage.Children(func() {
						text(c, portrait.Type.Meta, "Manage models & providers…").TextColor(ink).Grow(1)
						portrait.Glyph(c, portrait.IconCaretRight, 11, portrait.Colors.Ink3).Shrink(0)
					})
					if manage.Clicked() {
						*open = false
						state.query, state.provider = "", ""
						p.openSettings("models")
					}
				})
			}
		})
	})
	if popover.Dismissed() {
		state.query, state.provider = "", ""
		p.pickerFocus = ""
		button.Focus()
	}
}

func chooseResearchPickerValue(picker researchPicker, value string, open *bool, state *researchPickerState, button ui.Element, p *panel) {
	*open = false
	state.query, state.provider = "", ""
	if picker.key == "model" {
		p.rememberModel(value)
	}
	p.pickerFocus = ""
	picker.onSelect(value)
	button.Focus()
}

func (p *panel) pickerRowView(c *ui.Context, picker researchPicker, row pickerRow) ui.Element {
	if row.section != "" {
		text(c, portrait.Type.Meta, row.section).FontWeight(600).TextColor(portrait.Colors.Ink3).Padding(6, 8, 2, 8).Role(ui.RoleHeading)
	}
	key := "research-picker-option-" + picker.key + "-" + row.item.value
	if row.kind != pickerChoice {
		key = "research-picker-provider-" + picker.key + "-" + row.provider + "-" + strconv.Itoa(int(row.kind))
	}
	b := ui.ButtonBase(c.Key(key)).Label(row.label()).FillWidth().MinHeight(30).Padding(4, 8).Radius(7).Gap(6).AlignItems(ui.Center)
	if row.kind == pickerChoice {
		b.Checked(row.item.current)
	}
	ink := portrait.Press(b, portrait.Quiet)
	if row.item.current {
		b.Background(portrait.Colors.AccentSoft).NoShadow()
	}
	b.Children(func() {
		switch row.kind {
		case pickerBack:
			text(c, portrait.Type.UI, "‹ Back").TextColor(ink).Grow(1)
			text(c, portrait.Type.Meta, providerLabel(row.provider)).TextColor(portrait.Colors.Ink3).MaxLines(1)
		case pickerProvider:
			text(c, portrait.Type.UI, providerLabel(row.provider)).TextColor(ink).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
			text(c, portrait.Type.Meta, strconv.Itoa(row.count)).TextColor(portrait.Colors.Ink3)
			portrait.Glyph(c, portrait.IconCaretRight, 11, portrait.Colors.Ink3).Shrink(0)
		default:
			text(c, portrait.Type.UI, row.item.label).TextColor(ink).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
			if row.showProvider {
				text(c, portrait.Type.Meta, providerLabel(row.item.detail)).TextColor(portrait.Colors.Ink3).MaxLines(1).Shrink(0)
			} else if row.item.detail != "" && picker.key != "model" {
				text(c, portrait.Type.Meta, row.item.detail).TextColor(portrait.Colors.Ink3).MaxLines(1).Ellipsis("…").MinWidth(0)
			}
			if row.item.current {
				portrait.Glyph(c, portrait.IconCheck, 13, portrait.Colors.Accent).Shrink(0)
			}
		}
	})
	return b
}
