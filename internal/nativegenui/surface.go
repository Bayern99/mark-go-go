package nativegenui

import (
	"encoding/json"
	"errors"
	"slices"

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
type literal string

func (t *literal) UnmarshalJSON(raw []byte) error {
	if len(raw) > 0 && raw[0] == '{' {
		*t = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	*t = literal(value)
	return nil
}

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

type operation struct{ Components []json.RawMessage }

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
	for _, msg := range file.A2UI {
		for _, name := range []string{"createSurface", "updateComponents"} {
			raw, ok := msg[name]
			if !ok {
				continue
			}
			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, err
			}
			for _, rawNode := range op.Components {
				var identity struct{ ID, Component string }
				if err := json.Unmarshal(rawNode, &identity); err != nil {
					return nil, err
				}
				if identity.Component == "" {
					identity.Component = s.Nodes[identity.ID].Component
				}
				n := Node{ID: identity.ID, Component: identity.Component}
				if slices.Contains(known, n.Component) {
					if err := json.Unmarshal(rawNode, &n); err != nil {
						return nil, err
					}
				}
				s.Nodes[n.ID] = n
			}
		}
	}
	return s, nil
}
