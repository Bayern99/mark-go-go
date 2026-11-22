package genui

import (
	"encoding/json"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

type sourceUse struct {
	source  string
	written bool
}

func (b *BaseView) KeptHashes(merged json.RawMessage) map[string]string {
	if b == nil {
		return nil
	}
	var file struct {
		Sources map[string]struct{ Hash string }
	}
	_ = json.Unmarshal(b.raw, &file)
	var messages []json.RawMessage
	_ = json.Unmarshal(merged, &messages)
	sourceUses := componentSourceUses
	if b.statements {
		sourceUses = b.statementSourceUses
	}
	renewed := map[string]bool{}
	if uses, known := sourceUses(messages); known {
		for _, use := range uses {
			if use.written {
				renewed[use.source] = true
			}
		}
		for _, use := range uses {
			if !use.written {
				delete(renewed, use.source)
			}
		}
	}
	kept := map[string]string{}
	for id, source := range file.Sources {
		if !renewed[id] {
			kept[id] = source.Hash
		}
	}
	return kept
}

type sourcedComponent struct {
	ID       string
	Child    string
	Children json.RawMessage
	Tabs     []struct{ Child string }
	Sources  []string
}

func (c sourcedComponent) children() ([]string, bool) {
	var template struct{ ComponentID string }
	var ids []string
	if len(c.Children) > 0 && json.Unmarshal(c.Children, &ids) != nil && json.Unmarshal(c.Children, &template) != nil {
		return nil, false
	}
	ids = append(ids, c.Child, template.ComponentID)
	for _, tab := range c.Tabs {
		ids = append(ids, tab.Child)
	}
	return ids, true
}

func componentSourceUses(messages []json.RawMessage) ([]sourceUse, bool) {
	components := map[string]sourcedComponent{}
	var written map[string]bool
	for _, raw := range messages {
		var message viewMessage[sourcedComponent]
		if json.Unmarshal(raw, &message) != nil {
			return nil, false
		}
		batch := message.batch()
		written = map[string]bool{}
		if batch == nil {
			continue
		}
		for _, component := range batch.Components {
			components[component.ID] = component
			written[component.ID] = true
		}
	}
	var uses []sourceUse
	reached := map[string]bool{}
	queue := []string{"root"}
	for len(queue) > 0 {
		id := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		component, defined := components[id]
		if reached[id] || !defined {
			continue
		}
		reached[id] = true
		for _, source := range component.Sources {
			uses = append(uses, sourceUse{source, written[id]})
		}
		children, ok := component.children()
		if !ok {
			return nil, false
		}
		queue = append(queue, children...)
	}
	return uses, true
}

func (b *BaseView) statementSourceUses(messages []json.RawMessage) ([]sourceUse, bool) {
	var last statementMessage
	if len(messages) == 0 || json.Unmarshal(messages[len(messages)-1], &last) != nil || last.batch() == nil {
		return nil, false
	}
	earlier, update := b.statementText(), strings.Join(last.batch().Components, "\n")
	written := map[string]bool{}
	for _, name := range openuilang.Names(update) {
		written[name] = true
	}
	var uses []sourceUse
	for _, ids := range openuilang.FirstArgumentStrings(openuilang.Merge(earlier, update), "Source") {
		if !ids.Literal && !written[ids.Statement] {
			return nil, false
		}
		for _, source := range ids.Strings {
			uses = append(uses, sourceUse{source, written[ids.Statement]})
		}
	}
	return uses, true
}
