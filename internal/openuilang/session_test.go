package openuilang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type mounted struct {
	el   *Element
	form string
}

func mount(value any, form string, into map[string]mounted) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			mount(item, form, into)
		}
	case *Element:
		if v.StatementID != "" {
			into[v.StatementID] = mounted{v, form}
		}
		if v.Type == "Form" {
			name, _ := v.Props.Get("name")
			form, _ = name.(string)
		}
		for _, key := range v.Props.Keys {
			mount(v.Props.Values[key], form, into)
		}
	}
}

func eventJSON(e Event) map[string]any {
	out := map[string]any{"type": e.Type, "humanFriendlyMessage": e.Message, "formName": nil,
		"params": normalize(summarizeData(e.Params)), "formState": normalize(summarizeData(e.FormState))}
	if e.FormName != "" {
		out["formName"] = e.FormName
	}
	return out
}

func summarizeData(value any) any {
	switch v := value.(type) {
	case *Object:
		if v == nil {
			return nil
		}
		out := map[string]any{}
		for _, key := range v.Keys {
			out[key] = summarizeData(v.Values[key])
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = summarizeData(item)
		}
		return out
	}
	return value
}

var toolCases = map[string]string{
	"actions/006-query-rerun-on-state": "queries are parsed but not run",
	"actions/007-mutation-run":         "mutations are parsed but not run",
}

func TestConformanceInteractions(t *testing.T) {
	lib := fixtureLibrary(t)
	paths, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "*", "interactions.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no interaction cases: %v", err)
	}
	for _, path := range paths {
		dir := filepath.Dir(path)
		id := filepath.ToSlash(dir[len(fixtureRoot)+1:])
		t.Run(id, func(t *testing.T) {
			if reason, skip := toolCases[id]; skip {
				t.Skip(reason)
			}
			files, _ := readCase(t, dir)
			var script struct {
				Interactions []map[string]any `json:"interactions"`
				Expected     struct {
					Events []map[string]any `json:"events"`
					State  map[string]any   `json:"state"`
				} `json:"expected"`
			}
			if err := json.Unmarshal([]byte(files["interactions.json"]), &script); err != nil {
				t.Fatal(err)
			}
			session := NewSession()
			session.Load(lib.Parse(files["input.oui"]))
			var events []Event
			for i, step := range script.Interactions {
				if _, tool := step["tool"]; tool {
					continue
				}
				tree, _ := session.Render()
				rendered := map[string]mounted{}
				mount(tree, "", rendered)
				if typed, ok := step["type"].([]any); ok {
					target, found := rendered[typed[0].(string)]
					if !found {
						t.Fatalf("step %d: nothing rendered for %v", i, typed[0])
					}
					name, _ := target.el.Props.Get("name")
					if value, _ := target.el.Props.Get("value"); value != nil {
						if binding, ok := value.(*Binding); ok {
							session.Set(binding.Target, typed[1])
							continue
						}
					}
					session.SetField(target.form, "input", name.(string), typed[1])
					continue
				}
				target, found := rendered[step["click"].(string)]
				if !found {
					t.Fatalf("step %d: nothing rendered for %v", i, step["click"])
				}
				label, _ := target.el.Props.Get("label")
				action, _ := target.el.Props.Get("action")
				events = append(events, session.Trigger(label.(string), target.form, action)...)
			}
			if len(events) != len(script.Expected.Events) {
				t.Fatalf("expected %d events, got %+v", len(script.Expected.Events), events)
			}
			for i, want := range script.Expected.Events {
				got := eventJSON(events[i])
				for key, value := range want {
					if !same(value, got[key]) {
						t.Errorf("event %d %s: expected %v, got %v", i, key, value, got[key])
					}
				}
			}
			for key, want := range script.Expected.State {
				if got, _ := session.Get(key); !same(want, summarizeData(got)) {
					t.Errorf("state %s: expected %v, got %v", key, want, got)
				}
			}
		})
	}
}

func TestConformanceValidationRules(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join(fixtureRoot, "*", "*", "validation.json"))
	if len(paths) == 0 {
		t.Fatal("no validation cases")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeOrdered(data)
		if err != nil {
			t.Fatal(err)
		}
		cases, _ := decoded.(*Object).Values["cases"].([]any)
		for i, entry := range cases {
			c := entry.(*Object)
			message := ValidateField(c.Values["value"], c.Values["rules"])
			if valid, _ := c.Values["valid"].(bool); (message == "") != valid {
				t.Errorf("%s case %d: rules %v value %v gave %q", path, i, summarizeData(c.Values["rules"]), c.Values["value"], message)
			}
		}
	}
}

func TestReloadKeepsWhatTheReaderChanged(t *testing.T) {
	lib := fixtureLibrary(t)
	session := NewSession()
	session.Load(lib.Parse("root = Stack([Text(\"n \" + $n)])\n$n = 1\n$m = 1"))
	session.Set("$n", 5.0)
	session.Load(lib.Parse(Merge("root = Stack([Text(\"n \" + $n)])\n$n = 1\n$m = 1", "$n = 2\n$m = 3")))
	if got, _ := session.Get("$n"); got != 5.0 {
		t.Errorf("reader's value was overwritten: %v", got)
	}
	if got, _ := session.Get("$m"); got != 3.0 {
		t.Errorf("untouched state should follow the new default: %v", got)
	}
	tree, _ := session.Render()
	children, _ := tree.Props.Get("children")
	if text, _ := children.([]any)[0].(*Element).Props.Get("text"); text != "n 5" {
		t.Errorf("render shows %v", text)
	}
}
