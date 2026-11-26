package nativegenui

import (
	"math"
	"strconv"
	"time"

	genuiexport "github.com/Bayern99/mark-go-go/internal/export/genui"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

func (r *Reading) exportSnapshot() (*openuilang.Element, []openuilang.Error) {
	if r.surface == nil || r.surface.Session == nil {
		return nil, nil
	}
	root, problems := r.surface.Session.Render()
	if root == nil {
		return nil, problems
	}
	clone := r.snapshotValue(root, "root", "", false, map[*openuilang.Element]*openuilang.Element{})
	element, _ := clone.(*openuilang.Element)
	return element, problems
}

func snapshotChildPath(component, prop, path string) string {
	switch component {
	case "Popover":
		if prop == "trigger" || prop == "content" {
			return path + "/" + prop
		}
	case "Flashcard":
		if prop == "front" || prop == "back" {
			return path + "/" + prop
		}
	}
	return path
}

func controlValueProp(component, prop string) bool {
	switch component {
	case "Input", "TextArea", "Select", "RadioGroup", "SegmentedControl", "Slider", "DatePicker":
		return prop == "value"
	case "Checkbox":
		return prop == "checked"
	case "Timer":
		return prop == "seconds"
	}
	return false
}

func controlName(element *openuilang.Element, path string) string {
	if element.StatementID != "" {
		return element.StatementID
	}
	if label, ok := element.Props.Values["label"].(string); ok && label != "" {
		return label
	}
	return path
}

func (r *Reading) snapshotControlValue(element *openuilang.Element, path, form string, bound bool, value any) any {
	if !bound {
		name := controlName(element, path)
		if r.touched[form+"\x00"+name] {
			value = r.surface.Session.Field(form, name)
		}
	}
	state := r.states[path]
	if state == nil {
		return value
	}
	switch element.Type {
	case "Input", "TextArea":
		if state.editing {
			return state.text
		}
	case "Timer":
		seconds := math.Max(0, amount(value))
		if state.running && seconds > 0 {
			elapsed := max(0, math.Floor(time.Since(state.date).Seconds()))
			seconds = math.Max(0, seconds-elapsed)
		}
		return seconds
	}
	return value
}

func flattenedChildProp(component, prop string) bool {
	switch component {
	case "Col", "Row", "Box", "Grid", "Card", "Source", "Form", "Pressable", "Carousel", "ListItem":
		return prop == "children"
	case "List":
		return prop == "items"
	}
	return false
}

func (r *Reading) snapshotValue(value any, path, form string, inList bool, memo map[*openuilang.Element]*openuilang.Element) any {
	switch value := value.(type) {
	case *openuilang.Binding:
		if value == nil || r.surface.Session == nil {
			return nil
		}
		resolved, _ := r.surface.Session.Get(value.Target)
		return r.snapshotValue(resolved, path, form, inList, memo)
	case *openuilang.Element:
		if value == nil {
			return (*openuilang.Element)(nil)
		}
		if clone := memo[value]; clone != nil {
			return clone
		}
		clone := &openuilang.Element{Type: value.Type, StatementID: value.StatementID, Props: openuilang.NewObject()}
		memo[value] = clone
		defer delete(memo, value)
		childForm := form
		if value.Type == "Form" {
			childForm = value.StatementID
			if childForm == "" {
				childForm = path
			}
		}
		if value.Props != nil {
			for _, key := range value.Props.Keys {
				prop := value.Props.Values[key]
				if controlValueProp(value.Type, key) {
					binding, bound := prop.(*openuilang.Binding)
					if bound && binding != nil && r.surface.Session != nil {
						prop, _ = r.surface.Session.Get(binding.Target)
					} else if bound {
						prop = nil
					}
					prop = r.snapshotControlValue(value, path, form, bound, prop)
				}
				childPath := snapshotChildPath(value.Type, key, path)
				if value.Type == "ListItem" && !inList && key == "children" {
					childPath += "/0"
				}
				if flattenedChildProp(value.Type, key) {
					flat := flatten(prop, nil)
					items := make([]any, len(flat))
					childInList := value.Type == "List" && key == "items"
					for i, item := range flat {
						items[i] = r.snapshotValue(item, childPath+"/"+strconv.Itoa(i), childForm, childInList, memo)
					}
					clone.Props.Set(key, items)
					continue
				}
				clone.Props.Set(key, r.snapshotValue(prop, childPath, childForm, false, memo))
			}
		}
		return clone
	case *openuilang.Object:
		if value == nil {
			return (*openuilang.Object)(nil)
		}
		clone := openuilang.NewObject()
		for _, key := range value.Keys {
			clone.Set(key, r.snapshotValue(value.Values[key], path, form, false, memo))
		}
		return clone
	case []any:
		clone := make([]any, len(value))
		for i, item := range value {
			clone[i] = r.snapshotValue(item, path+"/"+strconv.Itoa(i), form, false, memo)
		}
		return clone
	default:
		return value
	}
}

func exportSectionSources(surface *Surface, sections map[headingKey]workspace.Section) map[string]genuiexport.SectionSource {
	out := map[string]genuiexport.SectionSource{}
	if surface == nil {
		return out
	}
	for id := range surface.Sources {
		_, section, ok := statementSource(surface, sections, id)
		if ok {
			out[id] = genuiexport.SectionSource{Heading: section.Heading, Occurrence: section.Occurrence}
		}
	}
	return out
}
