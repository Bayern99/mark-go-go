package openuilang

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Error struct {
	Source      string
	Code        string
	Component   string
	Path        string
	Message     string
	StatementID string
	Severity    string
}

func jsType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case undefinedValue:
		return "undefined"
	}
	return "object"
}

func isCompositeSchema(s *Object) bool {
	for _, key := range []string{"$ref", "anyOf", "oneOf", "allOf"} {
		if _, ok := s.Values[key]; ok {
			return true
		}
	}
	return false
}

func scalarInfo(s *Object) (expected string, enum []any, hasEnum bool) {
	if values, ok := s.Values["enum"].([]any); ok {
		return "", values, true
	}
	if value, ok := s.Values["const"]; ok {
		return "", []any{value}, true
	}
	switch s.Values["type"] {
	case "string":
		return "string", nil, false
	case "number", "integer":
		return "number", nil, false
	case "boolean":
		return "boolean", nil, false
	}
	return "", nil, false
}

func jsonText(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func typeFromSchema(property any) string {
	s, ok := property.(*Object)
	if !ok {
		return ""
	}
	if name, ok := refName(s); ok {
		return name
	}
	expected, enum, hasEnum := scalarInfo(s)
	if hasEnum {
		parts := make([]string, len(enum))
		for i, value := range enum {
			parts[i] = jsonText(value)
		}
		return strings.Join(parts, "|")
	}
	if expected != "" {
		return expected
	}
	name, _ := s.Values["type"].(string)
	return name
}

func paramsSignature(component string, params []paramDef) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = p.name
		if p.required {
			parts[i] += "*"
		}
		if kind := typeFromSchema(p.schema); kind != "" {
			parts[i] += ": " + kind
		}
	}
	return component + "(" + strings.Join(parts, ", ") + ")"
}

type issueSink struct {
	lib       *Library
	errors    []Error
	partial   bool
	statement string
}

func (s *issueSink) issue(code, component, path, message string) {
	s.errors = append(s.errors, Error{Source: "parser", Code: code, Component: component, Path: path, Message: message, StatementID: s.statement})
}

func (s *issueSink) mismatch(component, path, expected, actual string) {
	s.issue("type-mismatch", component, path, fmt.Sprintf("field %q expects %s but got %s", path, expected, actual))
}

func (s *issueSink) required(code, component, path, signature string) {
	message := fmt.Sprintf("missing required field %q", path)
	if code == "null-required" {
		message = fmt.Sprintf("required field %q cannot be null", path)
	}
	if signature != "" {
		message += " — signature: " + signature
	}
	s.issue(code, component, path, message)
}

func (s *issueSink) excess(component string, declared, got int) {
	s.issue("excess-args", component, "", fmt.Sprintf("%s takes %d arg(s), got %d (%d excess dropped)", component, declared, got, got-declared))
}

func (s *issueSink) unknownComponent(component string) {
	message := fmt.Sprintf("Unknown component %q — not found in catalog or builtins", component)
	if s.lib != nil && len(s.lib.componentNames) > 0 {
		message += ". Available components: " + strings.Join(s.lib.componentNames, ", ")
	}
	s.issue("unknown-component", component, "", message)
}

func (s *issueSink) unknownFunction(component string) {
	s.issue("unknown-function", component, "", fmt.Sprintf("Unknown function \"@%s\": not a built-in or action step. The call evaluates to null", component))
}

func slotRefNames(s *Object) ([]string, bool) {
	options, ok := s.Values["anyOf"].([]any)
	if !ok {
		options, ok = s.Values["oneOf"].([]any)
	}
	if !ok {
		options = []any{s}
	}
	names := make([]string, 0, len(options))
	all := len(options) > 0
	for _, option := range options {
		object, _ := option.(*Object)
		name, ok := "", false
		if object != nil {
			name, ok = refName(object)
		}
		if !ok {
			all = false
			name = "undefined"
		}
		names = append(names, name)
	}
	return names, all
}

func (s *issueSink) onlyComponentSlot(schema *Object) bool {
	names, all := slotRefNames(schema)
	if !all || s.lib == nil {
		return false
	}
	for _, name := range names {
		if _, ok := s.lib.components[name]; !ok {
			return false
		}
	}
	return true
}

func resolveInvalid(required, hasDefault bool, defaultValue any) (any, bool) {
	if hasDefault {
		return defaultValue, false
	}
	if required {
		return nil, true
	}
	return undefined, false
}

func (s *issueSink) validate(value, schema any, component, path string) (any, bool) {
	spec, ok := schema.(*Object)
	if !ok || isNullish(value) {
		return value, false
	}
	switch v := value.(type) {
	case *node:
		return value, false
	case *Element:
		if isCompositeSchema(spec) {
			return value, false
		}
		kind, typed := spec.Values["type"].(string)
		_, isEnum := spec.Values["enum"].([]any)
		_, isConst := spec.Values["const"]
		if typed || isEnum || isConst {
			if !typed {
				kind = "a literal value"
			}
			s.mismatch(component, path, kind, fmt.Sprintf("component %q", v.Type))
			return nil, true
		}
		return value, false
	}
	if isCompositeSchema(spec) {
		if !s.onlyComponentSlot(spec) {
			return value, false
		}
		names, _ := slotRefNames(spec)
		actual := jsType(value)
		if actual == "object" {
			actual = "plain object"
		}
		s.mismatch(component, path, strings.Join(names, " | "), actual)
		return nil, true
	}
	switch spec.Values["type"] {
	case "object":
		return s.validateObject(value, spec, component, path)
	case "array":
		return s.validateArray(value, spec, component, path)
	}
	expected, enum, hasEnum := scalarInfo(spec)
	if hasEnum {
		if s.partial || !isPrimitive(value) {
			return value, false
		}
		for _, allowed := range enum {
			if strictEqual(allowed, value) {
				return value, false
			}
		}
		parts := make([]string, len(enum))
		for i, allowed := range enum {
			parts[i] = jsonText(allowed)
		}
		s.mismatch(component, path, "one of ["+strings.Join(parts, ", ")+"]", jsonText(value))
		return nil, true
	}
	if expected != "" && (!isPrimitive(value) || expected != jsType(value)) {
		s.mismatch(component, path, expected, jsType(value))
		return nil, true
	}
	return value, false
}

func (s *issueSink) validateObject(value any, spec *Object, component, path string) (any, bool) {
	object, ok := value.(*Object)
	if !ok {
		if _, plan := value.(*ActionPlan); plan {
			return value, false
		}
		if _, binding := value.(*Binding); binding {
			return value, false
		}
		s.mismatch(component, path, "object", jsType(value))
		return nil, true
	}
	out := object.clone()
	props, _ := spec.Values["properties"].(*Object)
	required := map[string]bool{}
	requiredList, _ := spec.Values["required"].([]any)
	invalid := false
	for _, item := range requiredList {
		key, _ := item.(string)
		required[key] = true
		if s.partial {
			continue
		}
		current, present := object.Get(key)
		if present && !isNullish(current) {
			continue
		}
		var sub any
		if props != nil {
			sub = props.Values[key]
		}
		if fallback, ok := schemaDefault(sub); ok {
			out.Set(key, fallback)
			continue
		}
		code := "missing-required"
		if present {
			code = "null-required"
		}
		s.required(code, component, path+"/"+key, "")
		invalid = true
	}
	if props != nil {
		for _, key := range props.Keys {
			current, present := object.Get(key)
			if !present {
				continue
			}
			sub := props.Values[key]
			next, bad := s.validate(current, sub, component, path+"/"+key)
			if bad {
				fallback, hasDefault := schemaDefault(sub)
				next, bad = resolveInvalid(required[key], hasDefault, fallback)
				if bad {
					invalid = true
					continue
				}
			}
			if _, drop := next.(undefinedValue); drop {
				out.Delete(key)
			} else {
				out.Set(key, next)
			}
		}
	}
	if invalid {
		return nil, true
	}
	return out, false
}

func (s *issueSink) validateArray(value any, spec *Object, component, path string) (any, bool) {
	list, ok := value.([]any)
	if !ok {
		s.mismatch(component, path, "array", jsType(value))
		return nil, true
	}
	items, ok := spec.Values["items"].(*Object)
	if !ok {
		return value, false
	}
	out := make([]any, 0, len(list))
	for i, item := range list {
		next, bad := s.validate(item, items, component, fmt.Sprintf("%s/%d", path, i))
		if !bad {
			out = append(out, next)
		}
	}
	return out, false
}
