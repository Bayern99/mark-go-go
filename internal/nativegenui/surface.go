package nativegenui

import (
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
	ComponentID                                          string `json:"-"`
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
	call  json.RawMessage
	bound bool
}

func (t *literal) UnmarshalJSON(raw []byte) error {
	*t = literal{}
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
	} else if _, ok := object["@call"]; ok {
		t.call = append(t.call[:0], raw...)
		t.bound = true
	}
	return nil
}

func (t literal) String() string { return t.value }

type children struct {
	IDs      []string
	Template *childTemplate
}

type childTemplate struct {
	ComponentID string
	Path        string
}

func (ids *children) UnmarshalJSON(raw []byte) error {
	if len(raw) > 0 && raw[0] == '{' {
		var template childTemplate
		if err := json.Unmarshal(raw, &template); err != nil {
			return err
		}
		*ids = children{Template: &template}
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	*ids = children{IDs: values}
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
	baseNodes := make(map[string]Node, len(components))
	for id, raw := range components {
		n := Node{ID: id, Component: componentTypes[id], ComponentID: id}
		if slices.Contains(known, n.Component) {
			if err := json.Unmarshal(raw, &n); err != nil {
				return nil, err
			}
			n.ComponentID = id
		}
		baseNodes[id] = n
	}
	var err error
	s.Nodes, err = expandNodes(baseNodes, dataModel)
	if err != nil {
		return nil, err
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

const (
	maxTemplateItems     = 10_000
	maxTemplateInstances = 100_000
	maxTemplateDepth     = 64
)

type componentInstanceKey struct {
	componentID string
	dataPath    string
}

func expandNodes(components map[string]Node, dataModel any) (map[string]Node, error) {
	instances := map[string]Node{}
	occurrences := map[componentInstanceKey]int{}
	count := 0
	var renderNode func(string, string, map[componentInstanceKey]bool, int) (string, error)
	renderNode = func(componentID, dataPath string, ancestors map[componentInstanceKey]bool, depth int) (string, error) {
		key := componentInstanceKey{componentID: componentID, dataPath: dataPath}
		if ancestors[key] {
			return "", fmt.Errorf("cyclic component reference: %s", componentID)
		}
		if depth > maxTemplateDepth {
			return "", errors.New("GenUI component nesting exceeds the display limit")
		}
		component, ok := components[componentID]
		if !ok {
			return "", fmt.Errorf("invalid component reference: %s", componentID)
		}
		count++
		if count > maxTemplateInstances {
			return "", errors.New("GenUI view expands to too many components")
		}
		occurrences[key]++
		id := componentInstanceID(componentID, dataPath, occurrences[key])
		n := component
		n.ID = id
		n.ComponentID = componentID
		if err := resolveNode(&n, dataModel, dataPath); err != nil {
			return "", err
		}
		ancestors[key] = true
		defer delete(ancestors, key)

		if n.Child != "" {
			child, err := renderNode(n.Child, dataPath, ancestors, depth+1)
			if err != nil {
				return "", err
			}
			n.Child = child
		}
		for i := range n.Tabs {
			child, err := renderNode(n.Tabs[i].Child, dataPath, ancestors, depth+1)
			if err != nil {
				return "", err
			}
			n.Tabs[i].Child = child
		}

		expandedChildren := make([]string, 0, len(n.Children.IDs))
		if template := n.Children.Template; template != nil {
			listPath := resolveDataPath(template.Path, dataPath)
			value, found, err := dataModelAtPath(dataModel, listPath)
			if err != nil {
				return "", err
			}
			rows, ok := value.([]any)
			if found && ok {
				for i := 0; i < min(len(rows), maxTemplateItems); i++ {
					child, err := renderNode(template.ComponentID, nestedDataPath(listPath, strconv.Itoa(i)), ancestors, depth+1)
					if err != nil {
						return "", err
					}
					expandedChildren = append(expandedChildren, child)
				}
			}
		} else {
			for _, childID := range n.Children.IDs {
				child, err := renderNode(childID, dataPath, ancestors, depth+1)
				if err != nil {
					return "", err
				}
				expandedChildren = append(expandedChildren, child)
			}
		}
		n.Children = children{IDs: expandedChildren}
		instances[id] = n
		return id, nil
	}
	if _, err := renderNode("root", "/", map[componentInstanceKey]bool{}, 1); err != nil {
		return nil, err
	}
	return instances, nil
}

func componentInstanceID(componentID, dataPath string, occurrence int) string {
	id := escapeInstancePart(componentID)
	if dataPath != "" && dataPath != "/" {
		id += "-[" + escapeInstancePart(strings.TrimRight(dataPath, "/")) + "]"
	}
	if occurrence > 1 {
		id += fmt.Sprintf("#%d", occurrence)
	}
	return id
}

var instanceIDReplacer = strings.NewReplacer("~", "~0", "#", "~1", "[", "~2", "]", "~3", ">", "~4", "@", "~5")

func escapeInstancePart(value string) string { return instanceIDReplacer.Replace(value) }

func resolveDataPath(path, context string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	if context == "" {
		context = "/"
	}
	if !strings.HasSuffix(context, "/") {
		context += "/"
	}
	return context + path
}

func nestedDataPath(path, segment string) string {
	return strings.TrimRight(path, "/") + "/" + segment
}

func resolveNode(n *Node, dataModel any, dataPath string) error {
	fields := []*literal{&n.Text, &n.Accessibility.Label, &n.Accessibility.Description}
	for i := range n.Tabs {
		fields = append(fields, &n.Tabs[i].Title)
	}
	for _, field := range fields {
		if err := resolveLiteral(field, dataModel, dataPath); err != nil {
			return err
		}
	}
	return resolveHidden(&n.Accessibility.Hidden, dataModel, dataPath)
}

func resolveLiteral(value *literal, dataModel any, dataPath string) error {
	if !value.bound {
		return nil
	}
	var resolved any
	var err error
	if len(value.call) > 0 {
		resolved, err = resolveDynamicValue(value.call, dataModel, dataPath, 0, 0)
	} else {
		var found bool
		resolved, found, err = dataModelAtPath(dataModel, resolveDataPath(value.path, dataPath))
		if !found {
			resolved = nil
		}
	}
	if err != nil {
		return err
	}
	value.value, err = dataString(resolved)
	value.path = ""
	value.call = nil
	value.bound = false
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
