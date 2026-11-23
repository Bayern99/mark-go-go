package nativefeynman

import (
	"fmt"
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
	query  string
	cursor int
	focus  int
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
	if len(pickers) == 0 && !p.state.hasStats {
		return
	}
	ui.Row(c.Key("research-configs")).Wrap().Gap(2).Padding(4, 7, 0, 7).AlignItems(ui.Center).Children(func() {
		for _, picker := range pickers {
			p.configPicker(c, picker)
		}
		if p.state.hasStats {
			stats := p.state.stats
			label := fmt.Sprintf("%d tokens · $%.4f", stats.Tokens.Total, stats.Cost)
			if stats.ContextUsage != nil && stats.ContextUsage.ContextWindow > 0 {
				label = fmt.Sprintf("%d / %d tokens · $%.4f", stats.ContextUsage.Tokens, stats.ContextUsage.ContextWindow, stats.Cost)
			}
			text(c, portrait.Type.Meta, label).TextColor(portrait.Colors.Ink3).Padding(0, 5).MaxLines(1).Ellipsis("…")
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
			state.query = ""
			state.cursor = 0
			for i, item := range picker.items {
				if item.value == picker.current {
					state.cursor = i
					break
				}
			}
			state.focus = -1
			p.pickerFocus = picker.key
		}
	}

	popover := ui.PopoverBase(c, button, open, func(panel ui.Element) {
		panel.AttachTo(button, ui.AnchorTopStart, ui.AnchorBottomStart).
			MinWidth(button.Bounds().W).MaxHeight(max(float32(120), height-16)).
			Margin(0, 0, 6, 0).Padding(5).Radius(12).Gap(2).Label(picker.label + " options")
		portrait.Materials.GlassOverlay.Apply(panel)

		matches := matchingResearchPickerItems(picker.items, state.query)
		panel.ObserveInput(func(event ui.InputEvent) {
			if event.Kind != ui.InputKeyDown || event.Mods != 0 || len(matches) == 0 {
				return
			}
			switch event.Key {
			case ui.KeyDown:
				state.cursor = (state.cursor + 1) % len(matches)
			case ui.KeyUp:
				state.cursor = (state.cursor + len(matches) - 1) % len(matches)
			default:
				return
			}
			state.focus = state.cursor
			p.pickerFocus = picker.key
		})

		ui.Column(c).Width(min(float32(302), max(float32(120), width-16))).Children(func() {
			portrait.Field(c).Children(func() {
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
				if filter.Submitted() && len(matches) > 0 {
					chooseResearchPickerValue(picker, matches[state.cursor].value, open, state, button, p)
				}
			})
			ui.Scroll(c.Key("research-picker-options-" + picker.key)).
				MaxHeight(min(float32(250), max(float32(80), height-64))).FillWidth().
				Label(picker.label + " choices").Children(func() {
				if len(matches) == 0 {
					text(c, portrait.Type.Meta, "No matching options.").TextColor(portrait.Colors.Ink3).Padding(6, 8)
					return
				}
				if state.cursor >= len(matches) {
					state.cursor = len(matches) - 1
				}
				for i, item := range matches {
					choice := ui.ButtonBase(c.Key("research-picker-option-"+picker.key+"-"+item.value)).
						Label(item.description).FillWidth().MinHeight(30).Padding(4, 8).Radius(7).
						Gap(6).AlignItems(ui.Center).Checked(item.current)
					choiceInk := portrait.Press(choice, portrait.Quiet)
					if item.current {
						choice.Background(portrait.Colors.AccentSoft).NoShadow()
					}
					choice.Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							text(c, portrait.Type.UI, item.label).TextColor(choiceInk).MaxLines(1).Ellipsis("…")
							if item.detail != "" {
								text(c, portrait.Type.Meta, item.detail).TextColor(portrait.Colors.Ink3).MaxLines(1).Ellipsis("…")
							}
						})
						if item.current {
							portrait.Glyph(c, portrait.IconCheck, 13, portrait.Colors.Accent).Shrink(0)
						}
					})
					if p.pickerFocus == picker.key && state.focus == i {
						choice.Focus().ScrollIntoView()
						p.pickerFocus = ""
					}
					if choice.Clicked() {
						chooseResearchPickerValue(picker, item.value, open, state, button, p)
					}
				}
			})
		})
	})
	if popover.Dismissed() {
		state.query = ""
		p.pickerFocus = ""
		button.Focus()
	}
}

func chooseResearchPickerValue(picker researchPicker, value string, open *bool, state *researchPickerState, button ui.Element, p *panel) {
	*open = false
	state.query = ""
	p.pickerFocus = ""
	picker.onSelect(value)
	button.Focus()
}
