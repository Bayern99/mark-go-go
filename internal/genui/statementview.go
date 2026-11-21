package genui

import (
	"encoding/json"
	"fmt"
)

const (
	StatementViewVersion = 3
	StatementLanguage    = "openui-lang/1.0"
	StatementCatalogID   = "markgogo.dev:genui/v3"
)

type StatementView struct {
	Sources    map[string]StatementSource
	Request    string
	Statements [][]string
}

type StatementSource struct {
	Heading    string
	Occurrence int
	Hash       string
}

func viewVersion(raw []byte) float64 {
	var head struct{ Version float64 }
	_ = json.Unmarshal(raw, &head)
	return head.Version
}

func IsStatementView(raw []byte) bool { return viewVersion(raw) == StatementViewVersion }

func ReadStatementView(raw []byte) (*StatementView, error) {
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, newValidationError("INVALID_JSON", "The view is not valid JSON.", "/", "")
	}
	file, ok := top.(map[string]any)
	if !ok {
		return nil, newValidationError("INVALID_ENVELOPE", "The view is not a JSON object.", "/", "")
	}
	if language, _ := file["language"].(string); language != StatementLanguage {
		return nil, newValidationError("INVALID_LANGUAGE", fmt.Sprintf("The view's language is not %q.", StatementLanguage), "/language", "")
	}
	sourceValue, present := file["sources"]
	rawSources, ok := sourceValue.(map[string]any)
	if !ok && present {
		return nil, newValidationError("INVALID_SOURCES", "The view's sources are not an object.", "/sources", "")
	}
	if err := validateSourcesObject(rawSources); err != nil {
		return nil, err
	}
	view := &StatementView{Sources: map[string]StatementSource{}}
	for id, value := range rawSources {
		entry := value.(map[string]any)
		source := StatementSource{}
		source.Heading, _ = entry["heading"].(string)
		source.Hash, _ = entry["hash"].(string)
		if occurrence, ok := entry["occurrence"].(float64); ok {
			source.Occurrence = int(occurrence)
		}
		view.Sources[id] = source
	}
	view.Request, _ = file["request"].(string)
	messages, ok := file["a2ui"].([]any)
	if !ok || len(messages) == 0 {
		return nil, newValidationError("EMPTY_MESSAGES", "The view has no A2UI messages.", "/a2ui", "")
	}
	for i, value := range messages {
		path := fmt.Sprintf("/a2ui/%d", i)
		message, ok := value.(map[string]any)
		if !ok {
			return nil, newValidationError("INVALID_MESSAGE", "An A2UI message is not an object.", path, "")
		}
		if version, _ := message["version"].(string); version != TargetA2UIVersion {
			return nil, newValidationError("INVALID_MESSAGE", "An A2UI message is not version 1.0.", path+"/version", "")
		}
		create, creates := message["createSurface"].(map[string]any)
		update, updates := message["updateComponents"].(map[string]any)
		if creates == updates || creates != (i == 0) {
			return nil, newValidationError("INVALID_MESSAGE", "The view needs one createSurface message, followed only by updateComponents messages.", path, "")
		}
		operation, name := update, "updateComponents"
		if creates {
			operation, name = create, "createSurface"
			if catalog, _ := create["catalogId"].(string); catalog != StatementCatalogID {
				return nil, newValidationError("INVALID_CATALOG", fmt.Sprintf("The view's catalog is not %q.", StatementCatalogID), path+"/createSurface/catalogId", "")
			}
		}
		components, ok := operation["components"].([]any)
		if !ok {
			return nil, newValidationError("INVALID_COMPONENTS", "A message's components are not a list of statements.", path+"/"+name+"/components", "")
		}
		statements := make([]string, len(components))
		for j, component := range components {
			statements[j], ok = component.(string)
			if !ok {
				return nil, newValidationError("INVALID_COMPONENTS", "A version 3 component is one statement, written as a string.", fmt.Sprintf("%s/%s/components/%d", path, name, j), "")
			}
		}
		view.Statements = append(view.Statements, statements)
	}
	return view, nil
}
