package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

const baseViewText = `{
  "version": 2,
  "generatedAt": "2026-11-06T09:00:00Z",
  "model": "gemini-3.8-flash",
  "request": "Old request",
  "sources": {
    "s1": {"heading": "Report", "occurrence": 1, "hash": "fnv1a32:00000000"},
    "s2": {"heading": "Überblick 概要 🚀", "occurrence": 2, "hash": "fnv1a32:00000000"}
  },
  "a2ui": [
    {"version": "1.0", "createSurface": {"surfaceId": "view", "catalogId": "markgogo.dev:genui/v1", "components": [
      {"id": "root", "component": "Column", "children": ["a", "b"]},
      {"id": "a", "component": "KeyPoints", "items": ["Old A"], "sources": ["s1"]},
      {"id": "b", "component": "KeyPoints", "items": ["Old B"], "sources": ["s2"]}
    ]}}
  ]
}
`

const updateOut = `{"sources":{"s2":{"heading":"Überblick 概要 🚀","occurrence":2},"s3":{"heading":"Report","occurrence":1}},"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Card","child":"c"},{"id":"c","component":"KeyPoints","items":["New C"],"sources":["s2","s3"]}]}}]}`

func writeBase(t *testing.T, prev string) string {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	if err := os.WriteFile(viewPath, []byte(baseViewText), 0o600); err != nil {
		t.Fatal(err)
	}
	if prev != "" {
		if err := os.WriteFile(genui.PrevViewPath(viewPath), []byte(prev), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return viewPath
}

func updateJob(viewPath string, ids ...string) func(*GenerationJob) {
	return func(job *GenerationJob) {
		job.ViewPath = viewPath
		job.Req.Mode = "update"
		job.Req.ReplaceIDs = ids
	}
}

func TestGenerateUpdate(t *testing.T) {
	viewPath := writeBase(t, "")
	spec := fakeSpec{
		Permissions: []fakePermissionSpec{
			{Kind: "read", Title: "Read view", Locations: []string{"view.json"}},
			{Kind: "edit", Title: "Edit view", Locations: []string{"view.json"}},
		},
		Out: updateOut,
	}
	err, _, log := driveGeneration(t, spec, nil, updateJob(viewPath, "b"))
	if err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}

	if got := logEntries(log, "files")[0]["files"].([]any); len(got) != 2 || got[0] != "source.md" || got[1] != "view.json" {
		t.Errorf("agent folder at start = %v, want source.md and view.json", got)
	}
	outcomes := logEntries(log, "outcome")
	if len(outcomes) != 2 || outcomes[0]["optionId"] != "allow" || outcomes[1]["optionId"] != "reject" {
		t.Errorf("permission outcomes = %v, want read allowed and edit rejected", outcomes)
	}

	raw, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Request string                     `json:"request"`
		Sources map[string]json.RawMessage `json:"sources"`
		A2UI    []json.RawMessage          `json:"a2ui"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("parse view: %v", err)
	}
	var base, out struct {
		A2UI []json.RawMessage `json:"a2ui"`
	}
	_ = json.Unmarshal([]byte(baseViewText), &base)
	_ = json.Unmarshal([]byte(updateOut), &out)
	if len(view.A2UI) != 2 || compactJSON(t, string(view.A2UI[0])) != compactJSON(t, string(base.A2UI[0])) ||
		compactJSON(t, string(view.A2UI[1])) != compactJSON(t, string(out.A2UI[0])) {
		t.Errorf("a2ui = %s, want the view's message then the agent's", raw)
	}
	wantSources := map[string]string{
		"s1": `{"heading":"Report","occurrence":1,"hash":"fnv1a32:00000000"}`,
		"s2": `{"heading":"Überblick 概要 🚀","occurrence":2,"hash":"fnv1a32:9da8db51"}`,
		"s3": `{"heading":"Report","occurrence":1,"hash":"fnv1a32:d04be74a"}`,
	}
	if len(view.Sources) != len(wantSources) {
		t.Errorf("sources = %v", view.Sources)
	}
	for key, want := range wantSources {
		if got := compactJSON(t, string(view.Sources[key])); got != want {
			t.Errorf("source %s = %s, want %s", key, got, want)
		}
	}
	if view.Request != "Make the intro friendlier." {
		t.Errorf("request = %q, want this run's", view.Request)
	}

	if prev, err := os.ReadFile(genui.PrevViewPath(viewPath)); err != nil || string(prev) != baseViewText {
		t.Errorf("kept view = %q, %v; want the view before the update", prev, err)
	}
	if info, err := os.Stat(genui.PrevViewPath(viewPath)); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("kept view mode = %v, %v; want 0600, the view's", info.Mode().Perm(), err)
	}
	if snap := genui.ReadView(viewPath); !snap.CanUndo || snap.Text != string(raw) {
		t.Errorf("snapshot = %+v, want the new view with undo", snap)
	}
	tempDirGone(t, log)
}

func TestGenerateUpdateRejected(t *testing.T) {
	const kept = "the view before"
	message := func(body string) string {
		return `{"sources":{},"a2ui":[` + body + `]}`
	}
	cases := []struct {
		name, out, want string
	}{
		{"changes a component not named", message(`{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"a","component":"Text","text":"x"}]}}`),
			`changes the component "a", which it was not asked to replace`},
		{"replaces the root", message(`{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Text","text":"x"},{"id":"root","component":"Column","children":["b"]}]}}`),
			`"root", which it was not asked to replace`},
		{"two messages", message(`{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Text","text":"x"}]}},{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Text","text":"y"}]}}`),
			"exactly one updateComponents message"},
		{"a new surface", message(`{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v1","components":[{"id":"b","component":"Text","text":"x"}]}}`),
			`may only have "version" and "updateComponents"`},
		{"another surface", message(`{"version":"1.0","updateComponents":{"surfaceId":"other","components":[{"id":"b","component":"Text","text":"x"}]}}`),
			`for the surface "other"`},
		{"a source moved", `{"sources":{"s1":{"heading":"Other"}},"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Text","text":"x"}]}}]}`,
			`source "s1" differs from the view's`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewPath := writeBase(t, kept)
			err, _, log := driveGeneration(t, fakeSpec{Out: tc.out}, nil, updateJob(viewPath, "b"))
			f := failureFrom(t, err)
			if f.Kind != genui.FailInvalidOutput || !strings.Contains(f.Message, tc.want) {
				t.Fatalf("failure = %+v, want invalid-output containing %q", f, tc.want)
			}
			sentinelKept(t, viewPath, []byte(baseViewText))
			sentinelKept(t, genui.PrevViewPath(viewPath), []byte(kept))
			tempDirGone(t, log)
		})
	}
}

func TestGenerateUpdateNeedsItsView(t *testing.T) {
	viewPath := writeBase(t, "")
	for name, mutate := range map[string]func(*GenerationJob){
		"unknown id": updateJob(viewPath, "b", "zz"),
		"no ids":     updateJob(viewPath),
		"no view":    updateJob(filepath.Join(t.TempDir(), ".x.md.genui.json"), "b"),
	} {
		t.Run(name, func(t *testing.T) {
			err, _, log := driveGeneration(t, fakeSpec{Out: updateOut}, nil, mutate)
			if f := failureFrom(t, err); f.Kind != genui.FailAgentError || !strings.HasPrefix(f.Message, "The current view cannot be updated: ") {
				t.Errorf("failure = %+v, want the update refused", f)
			}
			if log != nil {
				t.Errorf("the agent ran: %v", log)
			}
		})
	}
	sentinelKept(t, viewPath, []byte(baseViewText))
}

func TestGenerateUpdateViewChangedMeanwhile(t *testing.T) {
	viewPath := writeBase(t, "")
	spec := fakeSpec{Out: updateOut, Messages: []string{"done"}}
	err, _, _ := driveGeneration(t, spec, func(p genui.GenerateProgress) error {
		if p.Phase == "checking" {
			return os.WriteFile(viewPath, []byte("rewritten"), 0o644)
		}
		return nil
	}, updateJob(viewPath, "b"))
	if f := failureFrom(t, err); !strings.Contains(f.Message, "changed while the agent worked") {
		t.Errorf("failure = %+v, want the conflict", f)
	}
	sentinelKept(t, viewPath, []byte("rewritten"))
	if _, err := os.Stat(genui.PrevViewPath(viewPath)); !os.IsNotExist(err) {
		t.Errorf("a view was kept: %v", err)
	}
}

func TestFullGenerationKeepsTheViewItReplaces(t *testing.T) {
	viewPath := writeBase(t, "older")
	if err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *GenerationJob) { job.ViewPath = viewPath }); err != nil {
		t.Fatalf("RunGeneration: %v", err)
	}
	sentinelKept(t, genui.PrevViewPath(viewPath), []byte(baseViewText))

	now, _ := os.ReadFile(viewPath)
	err, _, _ := driveGeneration(t, fakeSpec{Out: "not json"}, nil, func(job *GenerationJob) { job.ViewPath = viewPath })
	if err == nil {
		t.Fatal("the run with a bad out.json succeeded")
	}
	sentinelKept(t, viewPath, now)
	sentinelKept(t, genui.PrevViewPath(viewPath), []byte(baseViewText))
}

func viewModel(t *testing.T, viewPath string) string {
	t.Helper()
	raw, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("parse view: %v", err)
	}
	return view.Model
}

func TestUpdateKeepsTheModelThatGeneratedTheView(t *testing.T) {
	viewPath := writeBase(t, "")
	withArgon := func(mutate func(*GenerationJob)) func(*GenerationJob) {
		return func(job *GenerationJob) {
			mutate(job)
			job.Model = "gemini-4-argon"
		}
	}
	if err, _, _ := driveGeneration(t, fakeSpec{Out: updateOut}, nil, withArgon(updateJob(viewPath, "b"))); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := viewModel(t, viewPath); got != "gemini-3.8-flash" {
		t.Errorf("model after an update by gemini-4-argon = %q, want the view's own gemini-3.8-flash", got)
	}

	if err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, withArgon(func(job *GenerationJob) { job.ViewPath = viewPath })); err != nil {
		t.Fatalf("full generation: %v", err)
	}
	if got := viewModel(t, viewPath); got != "gemini-4-argon" {
		t.Errorf("model after a full generation by gemini-4-argon = %q, want gemini-4-argon", got)
	}
}
