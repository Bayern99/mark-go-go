package nativegenui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

type Reference struct {
	Heading    string
	Occurrence int
	Hash       string
}

type Surface struct {
	Nodes   map[string]Node
	Sources map[string]Reference
	Request string
}

type Node struct {
	ID, Component, Title, Child, Variant, Align, Justify string
	Text                                                 literal
	Accessibility                                        struct {
		Label       literal
		Description literal
		Hidden      json.RawMessage
		Live        string
	}
	Children         children
	Columns, Sources []string
	Weight           float64
	Tabs             []struct {
		Title literal
		Child string
	}
	Items   []json.RawMessage
	Metrics []struct{ Label, Value, Unit, Change, Trend string }
	Rows    [][]string
	Events  []struct{ When, Title, Detail string }
}

type literal struct {
	value string
	path  string
	bound bool
}

func (t *literal) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		*t = literal{value: value}
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	if rawPath, ok := object["@path"]; ok {
		if err := json.Unmarshal(rawPath, &t.path); err != nil {
			return err
		}
		t.bound = true
	}
	return nil
}

func (t literal) String() string { return t.value }

type children []string

func (ids *children) UnmarshalJSON(raw []byte) error {
	if len(raw) > 0 && raw[0] == '{' {
		*ids = nil
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	*ids = values
	return nil
}

type operation struct {
	Components []json.RawMessage
	DataModel  json.RawMessage
}

func Open(data string) (*Surface, error) {
	if len(data) > genui.MaxViewBytes {
		return nil, errors.New("GenUI view exceeds the size limit")
	}
	var file struct {
		Sources map[string]Reference
		Request json.RawMessage
		A2UI    []map[string]json.RawMessage
	}
	if err := json.Unmarshal([]byte(data), &file); err != nil {
		return nil, err
	}
	if err := genui.ValidateStoredView([]byte(data)); err != nil {
		return nil, err
	}
	known := genui.CatalogComponents()
	s := &Surface{Nodes: map[string]Node{}, Sources: file.Sources}
	_ = json.Unmarshal(file.Request, &s.Request)
	var dataModel any = map[string]any{}
	components := map[string]json.RawMessage{}
	componentTypes := map[string]string{}
	for _, msg := range file.A2UI {
		if raw, ok := msg["createSurface"]; ok {
			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, err
			}
			if len(op.DataModel) > 0 {
				if err := json.Unmarshal(op.DataModel, &dataModel); err != nil {
					return nil, err
				}
			}
			if err := applyComponents(known, componentTypes, components, op.Components); err != nil {
				return nil, err
			}
		}
		if raw, ok := msg["updateComponents"]; ok {
			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, err
			}
			if err := applyComponents(known, componentTypes, components, op.Components); err != nil {
				return nil, err
			}
		}
	}
	for id, raw := range components {
		n := Node{ID: id, Component: componentTypes[id]}
		if slices.Contains(known, n.Component) {
			if err := json.Unmarshal(raw, &n); err != nil {
				return nil, err
			}
			if err := resolveNode(&n, dataModel); err != nil {
				return nil, err
			}
		}
		s.Nodes[id] = n
	}
	return s, nil
}

func applyComponents(known []string, componentTypes map[string]string, components map[string]json.RawMessage, batch []json.RawMessage) error {
	for _, rawNode := range batch {
		var identity struct{ ID, Component string }
		if err := json.Unmarshal(rawNode, &identity); err != nil {
			return err
		}
		if identity.Component == "" {
			identity.Component = componentTypes[identity.ID]
		}
		n := Node{ID: identity.ID, Component: identity.Component}
		if slices.Contains(known, n.Component) {
			if err := json.Unmarshal(rawNode, &n); err != nil {
				return err
			}
		}
		componentTypes[n.ID] = n.Component
		components[n.ID] = rawNode
	}
	return nil
}

func resolveNode(n *Node, dataModel any) error {
	fields := []*literal{&n.Text, &n.Accessibility.Label, &n.Accessibility.Description}
	for i := range n.Tabs {
		fields = append(fields, &n.Tabs[i].Title)
	}
	for _, field := range fields {
		if err := resolveLiteral(field, dataModel); err != nil {
			return err
		}
	}
	return resolveHidden(&n.Accessibility.Hidden, dataModel)
}

func resolveLiteral(value *literal, dataModel any) error {
	if !value.bound {
		return nil
	}
	resolved, found, err := dataModelAtPath(dataModel, value.path)
	if err != nil {
		return err
	}
	if found {
		value.value, err = dataString(resolved)
	}
	value.path = ""
	value.bound = false
	return err
}

func dataString(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		return text, nil
	}
	if _, ok := value.(map[string]any); ok {
		return compactJSON(value)
	}
	if _, ok := value.([]any); ok {
		return compactJSON(value)
	}
	return fmt.Sprint(value), nil
}

func compactJSON(value any) (string, error) {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(data.String(), "\n"), nil
}

func resolveHidden(value *json.RawMessage, dataModel any) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(*value, &object); err != nil || object == nil {
		return nil
	}
	rawPath, ok := object["@path"]
	if !ok {
		return nil
	}
	var path string
	if err := json.Unmarshal(rawPath, &path); err != nil {
		return err
	}
	resolved, found, err := dataModelAtPath(dataModel, path)
	if err != nil {
		return err
	}
	hidden, _ := resolved.(bool)
	if !found {
		hidden = false
	}
	*value, err = json.Marshal(hidden)
	return err
}

func dataModelAtPath(root any, path string) (any, bool, error) {
	segments := pathSegments(path)
	var current any = root
	for _, segment := range segments {
		if forbiddenDataKey(segment) {
			return nil, false, fmt.Errorf("forbidden data model path segment %q", segment)
		}
		switch value := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = value[segment]
			if !exists {
				return nil, false, nil
			}
		case []any:
			index, ok := arrayIndex(segment)
			if !ok || index >= len(value) {
				return nil, false, nil
			}
			current = value[index]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}

func pathSegments(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	var segments []string
	for _, raw := range strings.Split(path, "/") {
		if raw == "" {
			continue
		}
		segment := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		segments = append(segments, segment)
	}
	return segments
}

func forbiddenDataKey(segment string) bool {
	return segment == "__proto__" || segment == "constructor" || segment == "prototype"
}

func arrayIndex(segment string) (int, bool) {
	if segment == "0" {
		return 0, true
	}
	if segment == "" || segment[0] < '1' || segment[0] > '9' {
		return 0, false
	}
	for _, digit := range segment[1:] {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}
	index, err := strconv.Atoi(segment)
	return index, err == nil
}
