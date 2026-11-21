package openuilang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func textOf(t *testing.T, lib *Library, program string) any {
	t.Helper()
	parsed := lib.Parse("root = Stack([Col(\"v\", [value])])\n" + program)
	tree, _ := parsed.Evaluate(nil)
	if tree == nil {
		t.Fatalf("%q rendered nothing: %+v", program, parsed.Errors)
	}
	children, _ := tree.Props.Get("children")
	column := children.([]any)[0].(*Element)
	data, _ := column.Props.Get("data")
	return normalize(summarize(data.([]any)[0], parsed))
}

func TestExpressionsFollowTheLanguageSpec(t *testing.T) {
	lib := fixtureLibrary(t)
	cases := []struct{ program, want string }{
		{`value = 1 + 2 * 3`, `7`},
		{`value = "a" + 1 + null`, `"a1"`},
		{`value = 7 / 0`, `0`},
		{`value = 7 % 0`, `0`},
		{`value = 5 == "5"`, `true`},
		{`value = null == 0`, `false`},
		{`value = "abc" > 1`, `false`},
		{`value = 0 || "fallback"`, `"fallback"`},
		{`value = 1 && "next"`, `"next"`},
		{`value = !!"x"`, `true`},
		{`value = -"3"`, `-3`},
		{`value = 1 ? 2 ? "a" : "b" : "c"`, `"a"`},
		{`value = [1 -2]`, `[-1]`},
		{`value = 0.1 + 0.2`, `0.30000000000000004`},
		{`value = "n=" + 1e21`, `"n=1e+21"`},
		{`value = "n=" + 0.000001`, `"n=0.000001"`},
		{`value = "n=" + 0.0000001`, `"n=1e-7"`},
		{`value = "n=" + 100`, `"n=100"`},
		{`value = @Count([1, 2, 3])`, `3`},
		{`value = @Count("x")`, `0`},
		{`value = @Sum([1, "2", true, null])`, `4`},
		{`value = @Avg([])`, `0`},
		{`value = @Min([3, 1, 2]) + @Max([3, 1, 2])`, `4`},
		{`value = @First([]) == null`, `true`},
		{`value = @Last([1, 2])`, `2`},
		{`value = @Round(2.5) + @Round(-2.5)`, `1`},
		{`value = @Round([1.234, [5.678]], 1)`, `[1.2,[5.7]]`},
		{`value = @Floor(1.9) + @Ceil(1.1) + @Abs(-1)`, `4`},
		{`value = @Filter([{n: 1}, {n: 2}, {n: 3}], "n", ">=", 2).n`, `[2,3]`},
		{`value = @Filter(["ab", "cd"], "", "contains", "c")`, `["cd"]`},
		{`value = @Filter([1, 2], "", "nope", 1)`, `[]`},
		{`value = @Filter([{a: {b: 1}}, {a: {b: 2}}], "a.b", "==", 2).a.b`, `[2]`},
		{`value = @Sort([{n: "10"}, {n: 9}], "n").n`, `[9,"10"]`},
		{`value = @Sort([3, 1, 2], "", "desc")`, `[3,2,1]`},
		{`value = @Sort(["B", "a", "C"], "")`, `["a","B","C"]`},
		{`value = @Each([1, 2], "n", n * 10)`, `[10,20]`},
		{`value = @Each("x", "n", n)`, `[]`},
		{`value = @Each([[1, 2], [3]], "n", @Each(n, "n", n + 1))`, `[[2,3],[4]]`},
		{"n = 99\nvalue = @Each([1], \"n\", n)", `[1]`},
		{`value = @Each([{t: "a"}], "r", Tag(r.t)).length`, `1`},
		{`value = [1, 2, 3].length`, `3`},
		{`value = {a: {b: [10, 20]}}.a.b[1]`, `20`},
		{`value = null.field == null`, `true`},
		{`value = [1][5] == null`, `true`},
		{`value = {a: 1}["a"]`, `1`},
		{"value = $n > 1\n  ? \"many\"\n  : \"few\"\n$n = 2", `"many"`},
		{`value = 'it\'s'`, `"it's"`},
		{`value = "tab\tA"`, `"tab\tA"`},
		{`value = 1 & 0 | 2`, `2`},
		{`value = (1 + 2) * 3 # trailing`, `9`},
	}
	for _, c := range cases {
		var want any
		if err := json.Unmarshal([]byte(c.want), &want); err != nil {
			t.Fatal(err)
		}
		if got := textOf(t, lib, c.program); !same(got, want) {
			gotJSON, _ := json.Marshal(got)
			t.Errorf("%s = %s, want %s", c.program, gotJSON, c.want)
		}
	}
}

func TestEachActionCapturesItsOwnElement(t *testing.T) {
	lib := fixtureLibrary(t)
	program := lib.Parse(`root = Stack(@Each([{id: 1}, {id: 2}], "t", Button("Pick", @Set($picked, t.id))))`)
	tree, errors := program.Evaluate(nil)
	if tree == nil || len(errors) > 0 {
		t.Fatalf("tree %v, errors %+v, parse errors %+v", tree, errors, program.Errors)
	}
	want := `{"type":"Stack","props":{"children":[
		{"type":"Button","props":{"label":"Pick","action":{"actions":[{"action":"Set","args":["$picked",1]}]}}},
		{"type":"Button","props":{"label":"Pick","action":{"actions":[{"action":"Set","args":["$picked",2]}]}}}]}}`
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if got := summarize(tree, program); !same(got, expected) {
		gotJSON, _ := json.Marshal(got)
		t.Errorf("got %s", gotJSON)
	}
}

func TestEvaluateReadsCurrentState(t *testing.T) {
	lib := fixtureLibrary(t)
	program := lib.Parse("root = Stack([Text(\"Total \" + $count * 2), Input(\"q\", null, $query)])\n$count = 2\n$query = \"a\"")
	state := map[string]any{"$count": 5.0}
	tree, _ := program.Evaluate(func(name string) (any, bool) { value, ok := state[name]; return value, ok })
	want := `{"type":"Stack","props":{"children":[
		{"type":"Text","props":{"text":"Total 10"}},
		{"type":"Input","props":{"name":"q","value":{"$binding":"$query"}}}]}}`
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if got := summarize(tree, program); !same(got, expected) {
		gotJSON, _ := json.Marshal(got)
		t.Errorf("got %s", gotJSON)
	}
}

func TestRepeatedReferencesStayBounded(t *testing.T) {
	lib := fixtureLibrary(t)
	source := "root = Stack([Col(\"v\", [@Count(s0)])])\n"
	for i := 0; i < 40; i++ {
		source += "s" + numberString(float64(i)) + " = [s" + numberString(float64(i+1)) + ", s" + numberString(float64(i+1)) + "] + 0\n"
	}
	source += "s40 = 1"
	program := lib.Parse(source)
	for _, e := range program.Errors {
		if e.Code == "parse-exception" {
			t.Fatal("parser failed")
		}
	}
	program.Evaluate(nil)
}

func FuzzParseNeverPanics(f *testing.F) {
	lib, err := func() (*Library, error) {
		spec, err := os.ReadFile(filepath.Join(fixtureRoot, "library.json"))
		if err != nil {
			return nil, err
		}
		return LoadLibrary(spec)
	}()
	if err != nil {
		f.Fatal(err)
	}
	inputs, _ := filepath.Glob(filepath.Join(fixtureRoot, "*", "*", "input.oui"))
	for _, path := range inputs {
		data, _ := os.ReadFile(path)
		f.Add(string(data))
	}
	f.Add("root = Stack([a])\na = Stack([a, b])\nb = @Each(b, \"b\", b)")
	f.Add("root = " + string(make([]byte, 0)) + "((((((((((((((((((((")
	f.Fuzz(func(t *testing.T, input string) {
		program := lib.Parse(input)
		for _, e := range program.Errors {
			if e.Code == "parse-exception" {
				t.Fatalf("parser failed on %q", input)
			}
		}
		program.Evaluate(nil)
	})
}

func TestLoopVariableDoesNotHideItsSource(t *testing.T) {
	lib := fixtureLibrary(t)
	got := textOf(t, lib, "items = [1, 2]\nvalue = @Each(items, \"items\", items * 2)")
	if !same(got, []any{2.0, 4.0}) {
		t.Errorf("got %v", got)
	}
}

func TestSetReadsQueryDefaults(t *testing.T) {
	lib := fixtureLibrary(t)
	program := lib.Parse("root = Stack([Button(\"Pick\", @Set($picked, q.id))])\nq = Query(\"load\", {}, {id: 7})")
	tree, _ := program.Evaluate(nil)
	children, _ := tree.Props.Get("children")
	action, _ := children.([]any)[0].(*Element).Props.Get("action")
	if got := program.EvaluateExpression(action.(*ActionPlan).Steps[0], nil); got != 7.0 {
		t.Errorf("got %v", got)
	}
}

func TestEnumRejectsLists(t *testing.T) {
	lib := fixtureLibrary(t)
	program := lib.Parse(`root = Stack([Tag("x", ["green"])])`)
	if len(program.Errors) != 1 || program.Errors[0].Code != "type-mismatch" {
		t.Fatalf("errors %+v", program.Errors)
	}
	tree, _ := program.Evaluate(nil)
	want := map[string]any{"type": "Stack", "props": map[string]any{"children": []any{map[string]any{"type": "Tag", "props": map[string]any{"text": "x"}}}}}
	if got := summarize(tree, program); !same(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestSharedSubtreesParseQuickly(t *testing.T) {
	lib := fixtureLibrary(t)
	source := "root = $on ? Stack([Text(\"n\" + s0)]) : Stack([])\n$d = s0\n"
	for i := 0; i < 60; i++ {
		next := "s" + numberString(float64(i+1))
		source += "s" + numberString(float64(i)) + " = " + next + " + " + next + "\n"
	}
	program := lib.Parse(source + "s60 = 1")
	for _, e := range program.Errors {
		if e.Code == "parse-exception" {
			t.Fatal("parser failed")
		}
	}
	program.Evaluate(nil)
}
