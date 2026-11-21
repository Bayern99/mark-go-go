package openuilang

import (
	"fmt"
	"regexp"
	"strings"
)

type paramDef struct {
	name       string
	required   bool
	hasDefault bool
	defaultVal any
	schema     any
}

type callDef struct {
	kind    string
	params  []paramDef
	builtin bool
	lazy    bool
	schema  *Object
	returns any
}

type Function func(args *Object) (any, error)

type Library struct {
	root           string
	componentNames []string
	components     map[string][]paramDef
	calls          map[string]*callDef
	bindable       map[string]map[string]bool
	functions      map[string]Function
}

func builtinCall(kind string, lazy bool, names ...string) *callDef {
	def := &callDef{kind: kind, builtin: true, lazy: lazy}
	for _, name := range names {
		def.params = append(def.params, paramDef{name: name})
	}
	return def
}

var builtinCalls = map[string]*callDef{
	"Count": builtinCall("function", false, "array"), "First": builtinCall("function", false, "array"),
	"Last": builtinCall("function", false, "array"), "Sum": builtinCall("function", false, "numbers"),
	"Avg": builtinCall("function", false, "numbers"), "Min": builtinCall("function", false, "numbers"),
	"Max":    builtinCall("function", false, "numbers"),
	"Sort":   builtinCall("function", false, "array", "field", "direction"),
	"Filter": builtinCall("function", false, "array", "field", "operator", "value"),
	"Round":  builtinCall("function", false, "number", "decimals"),
	"Abs":    builtinCall("function", false, "number"), "Floor": builtinCall("function", false, "number"),
	"Ceil": builtinCall("function", false, "number"),
	"Each": builtinCall("function", true, "array", "varName", "template"),
	"Run":  builtinCall("action", true, "ref"), "ToAssistant": builtinCall("action", false, "message", "context"),
	"OpenUrl": builtinCall("action", false, "url"), "Set": builtinCall("action", true, "variable", "value"),
	"Reset":  builtinCall("action", true, "variable"),
	"Action": {kind: "action", builtin: true, lazy: true, params: []paramDef{{name: "steps", required: true}}},
}

func isBuiltin(name string) bool { return builtinCalls[name] != nil }

func isReservedCall(name string) bool { return name == "Query" || name == "Mutation" }

var bindingProp = regexp.MustCompile(`(\w+)\??: \$binding<`)

func LoadLibrary(spec []byte) (*Library, error) {
	decoded, err := decodeOrdered(spec)
	if err != nil {
		return nil, fmt.Errorf("library spec: %w", err)
	}
	top, ok := decoded.(*Object)
	if !ok {
		return nil, fmt.Errorf("library spec must be an object")
	}
	schema, ok := top.Values["schema"].(*Object)
	if !ok {
		return nil, fmt.Errorf("library spec has no schema")
	}
	lib := &Library{
		components: map[string][]paramDef{},
		calls:      map[string]*callDef{},
		bindable:   map[string]map[string]bool{},
		functions:  map[string]Function{},
	}
	lib.root, _ = top.Values["root"].(string)
	for name, def := range builtinCalls {
		lib.calls[name] = def
	}
	defs, _ := schema.Values["$defs"].(*Object)
	if defs == nil {
		defs = NewObject()
	}
	listed, hasListed := schema.Values["properties"].(*Object)
	actions, _ := schema.Values["actions"].(*Object)
	functions, _ := schema.Values["functions"].(*Object)
	actionDefs := map[string]any{}
	actionNames := []string{"ActionExpression", "OpenUrl", "ToAssistant"}
	if actions != nil {
		actionNames = append(actionNames, actions.Keys...)
	}
	for _, name := range actionNames {
		body, ok := defs.Values[name].(*Object)
		if _, component := listed.Get(name); ok && !(hasListed && component) {
			actionDefs["#/$defs/"+name] = body
		}
	}
	for _, name := range defs.Keys {
		def, ok := defs.Values[name].(*Object)
		if !ok {
			continue
		}
		if hasListed {
			if _, component := listed.Get(name); !component {
				continue
			}
		} else if _, action := actionDefs["#/$defs/"+name]; action {
			continue
		}
		lib.componentNames = append(lib.componentNames, name)
		lib.components[name] = compileParams(def, actionDefs)
	}
	register := func(kind string, entries *Object) {
		if entries == nil {
			return
		}
		for _, name := range entries.Keys {
			entry, _ := entries.Values[name].(*Object)
			params, _ := entry.Get("params")
			paramSchema, _ := params.(*Object)
			if paramSchema == nil {
				paramSchema = NewObject()
			}
			returns, _ := entry.Get("returns")
			lib.calls[name] = &callDef{kind: kind, params: compileParams(paramSchema, actionDefs), schema: paramSchema, returns: returns}
		}
	}
	register("function", functions)
	register("action", actions)
	if components, ok := top.Values["components"].(*Object); ok {
		for _, name := range components.Keys {
			entry, _ := components.Values[name].(*Object)
			signature, _ := entry.Get("signature")
			text, _ := signature.(string)
			for _, match := range bindingProp.FindAllStringSubmatch(text, -1) {
				if lib.bindable[name] == nil {
					lib.bindable[name] = map[string]bool{}
				}
				lib.bindable[name][match[1]] = true
			}
		}
	}
	return lib, nil
}

func (l *Library) RegisterFunction(name string, fn Function) { l.functions[name] = fn }

func (l *Library) isRuntimeCall(name string) bool {
	return l.calls[name] != nil || isReservedCall(name)
}

func compileParams(def *Object, actionDefs map[string]any) []paramDef {
	properties, _ := def.Values["properties"].(*Object)
	if properties == nil {
		return nil
	}
	required := map[string]bool{}
	if list, ok := def.Values["required"].([]any); ok {
		for _, item := range list {
			if name, ok := item.(string); ok {
				required[name] = true
			}
		}
	}
	var out []paramDef
	for _, key := range properties.Keys {
		property := properties.Values[key]
		param := paramDef{name: key, required: required[key], schema: inlineActionRefs(property, actionDefs, 0)}
		param.defaultVal, param.hasDefault = schemaDefault(property)
		out = append(out, param)
	}
	return out
}

func inlineActionRefs(value any, defs map[string]any, depth int) any {
	if len(defs) == 0 || depth > 32 {
		return value
	}
	switch v := value.(type) {
	case *Object:
		if ref, ok := v.Values["$ref"].(string); ok {
			if body, ok := defs[ref]; ok {
				return inlineActionRefs(body, defs, depth+1)
			}
			return v
		}
		out := NewObject()
		for _, key := range v.Keys {
			out.Set(key, inlineActionRefs(v.Values[key], defs, depth+1))
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = inlineActionRefs(item, defs, depth+1)
		}
		return out
	}
	return value
}

func schemaDefault(property any) (any, bool) {
	object, ok := property.(*Object)
	if !ok {
		return nil, false
	}
	return object.Get("default")
}

func refName(schema *Object) (string, bool) {
	ref, ok := schema.Values["$ref"].(string)
	if !ok {
		return "", false
	}
	return ref[strings.LastIndex(ref, "/")+1:], true
}
