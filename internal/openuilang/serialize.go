package openuilang

import (
	"strings"
)

type serializer struct {
	lib        *Library
	seen       map[string]bool
	statements []string
}

func (l *Library) Serialize(root *Element) string {
	s := &serializer{lib: l, seen: map[string]bool{}}
	id := root.StatementID
	if id == "" {
		id = "root"
	}
	s.seen[id] = true
	lines := []string{id + " = " + s.element(root)}
	return strings.Join(append(lines, s.statements...), "\n")
}

func (s *serializer) value(value any) string {
	switch v := value.(type) {
	case string:
		return jsonText(v)
	case float64:
		return numberString(v)
	case bool:
		return toString(v)
	case *Element:
		if v.StatementID == "" {
			return s.element(v)
		}
		if !s.seen[v.StatementID] {
			s.seen[v.StatementID] = true
			text := v.StatementID + " = " + s.element(v)
			s.statements = append(s.statements, text)
		}
		return v.StatementID
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = s.value(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Object:
		parts := make([]string, len(v.Keys))
		for i, key := range v.Keys {
			parts[i] = key + ": " + s.value(v.Values[key])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

func (s *serializer) element(el *Element) string {
	params, ok := s.lib.components[el.Type]
	var args []string
	if ok {
		for _, param := range params {
			value, _ := el.Props.Get(param.name)
			args = append(args, s.value(value))
		}
		for len(args) > 0 && args[len(args)-1] == "null" && !params[len(args)-1].required {
			args = args[:len(args)-1]
		}
	} else {
		for _, key := range el.Props.Keys {
			args = append(args, s.value(el.Props.Values[key]))
		}
	}
	return el.Type + "(" + strings.Join(args, ", ") + ")"
}
