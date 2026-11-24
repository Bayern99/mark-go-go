package nativefeynman

import (
	"slices"
	"strings"
)

const (
	pinnedModelsPreference = "feynman.models.pinned"
	recentModelsPreference = "feynman.models.recent"
	hiddenModelsPreference = "feynman.models.hidden"
	recentModelLimit       = 5
)

type pickerRowKind int

const (
	pickerChoice pickerRowKind = iota
	pickerProvider
	pickerBack
)

type pickerRow struct {
	kind         pickerRowKind
	section      string
	item         researchPickerItem
	provider     string
	count        int
	showProvider bool
}

func (r pickerRow) label() string {
	switch r.kind {
	case pickerProvider:
		return "Show " + providerLabel(r.provider) + " models"
	case pickerBack:
		return "Back to providers"
	}
	return r.item.description
}

var providerNames = map[string]string{
	"anthropic": "Anthropic", "openai": "OpenAI", "openai-codex": "OpenAI Codex", "google": "Google",
	"google-vertex": "Google Vertex", "xai": "xAI", "openrouter": "OpenRouter", "mistral": "Mistral",
	"deepseek": "DeepSeek", "groq": "Groq", "github-copilot": "GitHub Copilot", "amazon-bedrock": "Amazon Bedrock",
	"azure-openai": "Azure OpenAI", "ollama": "Ollama", "lmstudio": "LM Studio", "local": "Local",
}

func providerLabel(provider string) string {
	if name, ok := providerNames[strings.ToLower(provider)]; ok {
		return name
	}
	if provider == "" {
		return "Other"
	}
	return strings.ToUpper(provider[:1]) + provider[1:]
}

func (p *panel) preference(key string) string {
	if p.window != nil {
		return p.window.Preference(key)
	}
	return p.localPrefs[key]
}

func (p *panel) setPreference(key, value string) {
	if p.window != nil {
		p.window.SetPreference(key, value)
		return
	}
	if p.localPrefs == nil {
		p.localPrefs = map[string]string{}
	}
	p.localPrefs[key] = value
}

func (p *panel) modelList(key string) []string {
	return slices.DeleteFunc(strings.Split(p.preference(key), "\n"), func(v string) bool { return v == "" })
}

func (p *panel) setModelList(key string, values []string) {
	p.setPreference(key, strings.Join(values, "\n"))
}

func (p *panel) modelPinned(model string) bool {
	return slices.Contains(p.modelList(pinnedModelsPreference), model)
}

func (p *panel) toggleModelPinned(model string) {
	pinned := p.modelList(pinnedModelsPreference)
	if i := slices.Index(pinned, model); i >= 0 {
		p.setModelList(pinnedModelsPreference, slices.Delete(pinned, i, i+1))
		return
	}
	p.setModelList(pinnedModelsPreference, append(pinned, model))
}

func (p *panel) modelShown(model string) bool {
	return !slices.Contains(p.modelList(hiddenModelsPreference), model)
}

func (p *panel) setModelShown(model string, shown bool) {
	hidden := slices.DeleteFunc(p.modelList(hiddenModelsPreference), func(v string) bool { return v == model })
	if !shown {
		hidden = append(hidden, model)
	}
	p.setModelList(hiddenModelsPreference, hidden)
}

func (p *panel) rememberModel(model string) {
	recent := slices.DeleteFunc(p.modelList(recentModelsPreference), func(v string) bool { return v == model })
	recent = append([]string{model}, recent...)
	p.setModelList(recentModelsPreference, recent[:min(len(recent), recentModelLimit)])
}

func (p *panel) pickerRows(picker researchPicker, state *researchPickerState) []pickerRow {
	matches := matchingResearchPickerItems(picker.items, state.query)
	if picker.key != "model" || !slices.ContainsFunc(picker.items, func(item researchPickerItem) bool { return item.detail != "" }) {
		rows := make([]pickerRow, len(matches))
		for i, item := range matches {
			rows[i] = pickerRow{item: item}
		}
		return rows
	}
	visible := slices.DeleteFunc(slices.Clone(matches), func(item researchPickerItem) bool { return !item.current && !p.modelShown(item.value) })
	providers := map[string]int{}
	for _, item := range picker.items {
		if item.current || p.modelShown(item.value) {
			providers[item.detail]++
		}
	}
	var rows []pickerRow
	choice := func(section string, item researchPickerItem) {
		rows = append(rows, pickerRow{section: section, item: item, showProvider: len(providers) > 1})
	}
	if strings.TrimSpace(state.query) != "" {
		for _, item := range visible {
			choice("", item)
		}
		return rows
	}
	if state.provider != "" {
		rows = append(rows, pickerRow{kind: pickerBack, provider: state.provider})
		for _, item := range visible {
			if item.detail == state.provider {
				rows = append(rows, pickerRow{item: item})
			}
		}
		return rows
	}
	byValue := func(value string) (researchPickerItem, bool) {
		i := slices.IndexFunc(visible, func(item researchPickerItem) bool { return item.value == value })
		if i < 0 {
			return researchPickerItem{}, false
		}
		return visible[i], true
	}
	pinned := p.modelList(pinnedModelsPreference)
	section := "Pinned"
	for _, value := range pinned {
		if item, ok := byValue(value); ok {
			choice(section, item)
			section = ""
		}
	}
	section = "Recent"
	for _, value := range p.modelList(recentModelsPreference) {
		if item, ok := byValue(value); ok && !slices.Contains(pinned, value) {
			choice(section, item)
			section = ""
		}
	}
	if len(providers) == 1 {
		section = "Models"
		for _, item := range visible {
			rows = append(rows, pickerRow{section: section, item: item})
			section = ""
		}
		return rows
	}
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(providerLabel(a), providerLabel(b)) })
	section = "Providers"
	for _, name := range names {
		rows = append(rows, pickerRow{kind: pickerProvider, section: section, provider: name, count: providers[name]})
		section = ""
	}
	return rows
}
