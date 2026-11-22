package openuilang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamingAtEverySplitEqualsBatchParse(t *testing.T) {
	lib := fixtureLibrary(t)
	inputs, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "*", "input.oui"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no inputs: %v", err)
	}
	for _, path := range inputs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		want, _ := json.Marshal(normalize(evaluateCase(lib.Parse(text)).fields))
		for at := 0; at <= len(text); at++ {
			stream := lib.NewStream()
			stream.Push(text[:at])
			stream.Push(text[at:])
			got, _ := json.Marshal(normalize(evaluateCase(stream.Finish()).fields))
			if string(got) != string(want) {
				t.Fatalf("%s split at %d:\n got %s\nwant %s", path, at, got, want)
			}
		}
	}
}

func TestPendingStatementDoesNotReplaceCompletedOne(t *testing.T) {
	lib := fixtureLibrary(t)
	stream := lib.NewStream()
	stream.Push("root = Stack([a])\na = Text(\"done\")\n")
	program := stream.Push("a = Text(\"half")
	tree, _ := program.Evaluate(nil)
	want := map[string]any{"type": "Stack", "props": map[string]any{"children": []any{
		map[string]any{"type": "Text", "props": map[string]any{"text": "done"}}}}}
	if got := summarize(tree, program); !same(got, want) {
		t.Errorf("while pending: got %v", got)
	}
	program = stream.Push("way\")")
	tree, _ = stream.Finish().Evaluate(nil)
	want["props"].(map[string]any)["children"].([]any)[0].(map[string]any)["props"].(map[string]any)["text"] = "halfway"
	if got := summarize(tree, program); !same(got, want) {
		t.Errorf("after the stream ends: got %v", got)
	}
}

func TestEveryPrefixRenders(t *testing.T) {
	lib := fixtureLibrary(t)
	text := "root = Stack([title, list])\n$n = 2\ntitle = Text(\"Items: \" + $n, \"heading\")\nlist = Stack(@Each([\"a\", \"b\"], \"x\", Tag(x, $n > 1 ? \"green\" : \"gray\")), \"row\")\n"
	stream := lib.NewStream()
	for i := 0; i < len(text); i++ {
		program := stream.Push(text[i : i+1])
		for _, e := range program.Errors {
			if e.Code == "parse-exception" {
				t.Fatalf("prefix %q failed to parse", text[:i+1])
			}
		}
		if i >= len("root = Stack([") {
			if tree, _ := program.Evaluate(nil); tree == nil {
				t.Fatalf("prefix %q rendered nothing", text[:i+1])
			}
		}
	}
}

func TestMergeKeepsUntouchedStatementsAndState(t *testing.T) {
	merged := Merge("root = Stack([a, b])\n$n = 1\na = Text(\"A\")\nb = Text(\"B\")\nold = Text(\"unused\")", "b = null\nroot = Stack([a, c])\nc = Text(\"C\")")
	want := "root = Stack([a, c])\n$n = 1\na = Text(\"A\")\nc = Text(\"C\")"
	if merged != want {
		t.Errorf("got %q", merged)
	}
}

func TestStreamEndEqualsBatchParseForProvisionalBoundaries(t *testing.T) {
	lib := fixtureLibrary(t)
	cases := [][]string{
		{"root = $on\n/", "/ note\n? Stack([Text(\"A\")]) : Stack([])\n$on = true"},
		{"root = Stack([a])\na = Text(\"first\")\n", "a = Text(\"second"},
	}
	for _, chunks := range cases {
		stream := lib.NewStream()
		text := ""
		for _, chunk := range chunks {
			stream.Push(chunk)
			text += chunk
		}
		got, _ := json.Marshal(normalize(evaluateCase(stream.Finish()).fields))
		want, _ := json.Marshal(normalize(evaluateCase(lib.Parse(text)).fields))
		if string(got) != string(want) {
			t.Errorf("%q:\n got %s\nwant %s", chunks, got, want)
		}
	}
	program := lib.Parse("root = Stack([a])\na = Text(\"first\")\na = Text(\"second")
	tree, _ := program.Evaluate(nil)
	children, _ := tree.Props.Get("children")
	if text, _ := children.([]any)[0].(*Element).Props.Get("text"); text != "second" {
		t.Errorf("final redefinition should win, got %v", text)
	}
}
