package openuilang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const fixtureRoot = "testdata/openui-1.0"

func fixtureLibrary(t *testing.T) *Library {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join(fixtureRoot, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	lib, err := LoadLibrary(spec)
	if err != nil {
		t.Fatal(err)
	}
	lib.RegisterFunction("Upper", func(args *Object) (any, error) {
		text, _ := args.Get("text")
		return strings.ToUpper(stringOrEmpty(text)), nil
	})
	return lib
}

func summarize(value any, program *Program) any {
	switch v := value.(type) {
	case nil, undefinedValue:
		return nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = summarize(item, program)
		}
		return out
	case *Element:
		props := map[string]any{}
		for _, key := range v.Props.Keys {
			if s := summarize(v.Props.Values[key], program); s != nil {
				props[key] = s
			}
		}
		return map[string]any{"type": v.Type, "props": props}
	case *Binding:
		return map[string]any{"$binding": v.Target}
	case *ActionPlan:
		actions := []any{}
		for _, step := range v.Steps {
			var name string
			var args []any
			switch step.Type {
			case "continue_conversation":
				name, args = "ToAssistant", []any{step.Message}
				if step.Context != nil {
					args = append(args, summarize(step.Context, program))
				}
			case "open_url":
				name, args = "OpenUrl", []any{step.URL}
			case "set":
				name, args = "Set", []any{step.Target, summarize(program.EvaluateExpression(step, nil), program)}
			case "reset":
				name = "Reset"
				for _, target := range step.Targets {
					args = append(args, target)
				}
			case "run":
				name, args = "Run", []any{step.StatementID}
			default:
				name, args = step.Name, []any{}
				for _, param := range program.lib.calls[step.Name].params {
					value, _ := step.Params.Get(param.name)
					args = append(args, summarize(value, program))
				}
			}
			actions = append(actions, map[string]any{"action": name, "args": args})
		}
		return map[string]any{"actions": actions}
	case *Object:
		out := map[string]any{}
		for _, key := range v.Keys {
			out[key] = summarize(v.Values[key], program)
		}
		return out
	}
	return value
}

type caseResult struct {
	tree   *Element
	fields map[string]any
}

func evaluateCase(program *Program) caseResult {
	tree, runtime := program.Evaluate(nil)
	all := append(append([]Error{}, program.Errors...), runtime...)
	errors, objects := []any{}, []any{}
	for _, e := range all {
		errors = append(errors, map[string]any{"code": e.Code, "statementId": e.StatementID})
		objects = append(objects, map[string]any{"source": e.Source, "code": e.Code, "statementId": e.StatementID, "component": e.Component, "path": e.Path})
	}
	var root any
	if tree != nil {
		root = summarize(tree, program)
	}
	return caseResult{tree: tree, fields: map[string]any{
		"root": root, "errors": errors, "errorObjects": objects,
		"unresolved": names(program.Unresolved), "orphans": names(program.Orphans),
		"incomplete": program.Incomplete, "state": summarize(program.State, program),
	}}
}

func names(list []string) []any {
	out := []any{}
	for _, name := range list {
		out = append(out, name)
	}
	return out
}

func normalize(value any) any {
	data, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}

func same(a, b any) bool { return reflect.DeepEqual(normalize(a), normalize(b)) }

func asSet(value any) any {
	list, _ := normalize(value).([]any)
	seen := map[string]bool{}
	out := []string{}
	for _, item := range list {
		if name, ok := item.(string); ok && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func claims(expected, actual any) bool {
	want, _ := normalize(expected).([]any)
	pool, _ := normalize(actual).([]any)
	sort.SliceStable(want, func(i, j int) bool {
		return len(want[i].(map[string]any)) > len(want[j].(map[string]any))
	})
	for _, entry := range want {
		found := -1
		for i, candidate := range pool {
			ok := true
			for key, value := range entry.(map[string]any) {
				got := candidate.(map[string]any)[key]
				if got == "" {
					got = nil
				}
				ok = ok && same(value, got)
			}
			if ok {
				found = i
				break
			}
		}
		if found < 0 {
			return false
		}
		pool = append(pool[:found], pool[found+1:]...)
	}
	return true
}

func diffs(id string, expected map[string]any, lib *Library, program *Program) []string {
	result := evaluateCase(program)
	var out []string
	report := func(key string) {
		want, _ := json.Marshal(expected[key])
		got, _ := json.Marshal(result.fields[key])
		out = append(out, key+": expected "+string(want)+", got "+string(got))
	}
	for _, key := range []string{"root", "errors", "unresolved", "orphans", "incomplete", "state", "errorObjects"} {
		want, listed := expected[key]
		got := result.fields[key]
		optional := key == "state" || key == "errorObjects"
		if !listed && optional {
			continue
		}
		ok := false
		switch key {
		case "errors":
			wantList, _ := want.([]any)
			ok = len(wantList) == len(got.([]any)) && claims(want, got)
		case "errorObjects":
			ok = claims(want, got)
		case "unresolved", "orphans":
			ok = same(asSet(want), asSet(got))
		default:
			ok = same(want, got)
		}
		if !ok {
			report(key)
		}
	}
	if strings.Contains(id, "-roundtrip-") && result.tree != nil {
		text := lib.Serialize(result.tree)
		again := evaluateCase(lib.Parse(text)).fields["root"]
		if !same(again, expected["root"]) {
			got, _ := json.Marshal(again)
			out = append(out, "roundtrip: "+text+" parses to "+string(got))
		}
	}
	return out
}

var statementName = regexp.MustCompile(`^(\$?\w+)\s*=([^=]|$)`)

func readCase(t *testing.T, dir string) (map[string]string, map[string]any) {
	t.Helper()
	files := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	var expected map[string]any
	if err := json.Unmarshal([]byte(files["expected.json"]), &expected); err != nil {
		t.Fatal(err)
	}
	return files, expected
}

func TestConformanceProgramCases(t *testing.T) {
	lib := fixtureLibrary(t)
	for _, area := range []string{"grammar", "entry", "evaluation", "errors", "streaming", "editing"} {
		cases, err := os.ReadDir(filepath.Join(fixtureRoot, area))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range cases {
			if !entry.IsDir() {
				continue
			}
			id := area + "/" + entry.Name()
			t.Run(id, func(t *testing.T) {
				files, expected := readCase(t, filepath.Join(fixtureRoot, id))
				if chunks, ok := files["chunks.json"]; ok {
					var parts []string
					if err := json.Unmarshal([]byte(chunks), &parts); err != nil {
						t.Fatal(err)
					}
					stream := lib.NewStream()
					for _, part := range parts {
						stream.Push(part)
					}
					for _, diff := range diffs(id, expected, lib, stream.Finish()) {
						t.Error("streamed " + diff)
					}
					for _, diff := range diffs(id, expected, lib, lib.Parse(strings.Join(parts, ""))) {
						t.Error("batch " + diff)
					}
					return
				}
				input := files["input.oui"]
				if patch, ok := files["patch.oui"]; ok {
					input = Merge(input, patch)
					got := []any{}
					for _, line := range strings.Split(input, "\n") {
						if match := statementName.FindStringSubmatch(line); match != nil {
							got = append(got, match[1])
						}
					}
					if want, listed := expected["statements"]; listed && !same(want, got) {
						t.Errorf("statements: expected %v, got %v", want, got)
					}
				}
				for _, diff := range diffs(id, expected, lib, lib.Parse(input)) {
					t.Error(diff)
				}
			})
		}
	}
}
