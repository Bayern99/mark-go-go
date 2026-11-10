package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// baseViewText is a current view an update starts from: a Column of
// two KeyPoints, "a" from the section Report and "b" from the second
// Überblick, both with hashes the document no longer has.
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

// updateOut is an agent's update that replaces "b" with a Card holding
// a new component "c" from a new source.
const updateOut = `{"sources":{"s2":{"heading":"Überblick 概要 🚀","occurrence":2},"s3":{"heading":"Report","occurrence":1}},"a2ui":[{"version":"1.0","updateComponents":{"surfaceId":"view","components":[{"id":"b","component":"Card","child":"c"},{"id":"c","component":"KeyPoints","items":["New C"],"sources":["s2","s3"]}]}}]}`

// writeBase writes the base view as the view file of a fresh folder and,
// when prev is not empty, a kept earlier view next to it.
func writeBase(t *testing.T, prev string) string {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	// Owner-only, as a view written by an earlier version may be.
	if err := os.WriteFile(viewPath, []byte(baseViewText), 0o600); err != nil {
		t.Fatal(err)
	}
	if prev != "" {
		if err := os.WriteFile(prevViewPath(viewPath), []byte(prev), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return viewPath
}

func updateJob(viewPath string, ids ...string) func(*generationJob) {
	return func(job *generationJob) {
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
		t.Fatalf("runGeneration: %v", err)
	}

	// The agent's folder held the document and the current view, as
	// the view file was; it may read the view but not write it.
	if got := logEntries(log, "files")[0]["files"].([]any); len(got) != 2 || got[0] != "source.md" || got[1] != "view.json" {
		t.Errorf("agent folder at start = %v, want source.md and view.json", got)
	}
	outcomes := logEntries(log, "outcome")
	if len(outcomes) != 2 || outcomes[0]["optionId"] != "allow" || outcomes[1]["optionId"] != "reject" {
		t.Errorf("permission outcomes = %v, want read allowed and edit rejected", outcomes)
	}

	// The update is appended to the view's messages; every source is
	// hashed from this run's text, and the new one is added.
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
		"s1": `{"heading":"Report","occurrence":1,"hash":"fnv1a32:d04be74a"}`,
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

	// The view it replaced is kept to go back to, as private as it was.
	if prev, err := os.ReadFile(prevViewPath(viewPath)); err != nil || string(prev) != baseViewText {
		t.Errorf("kept view = %q, %v; want the view before the update", prev, err)
	}
	if info, err := os.Stat(prevViewPath(viewPath)); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("kept view mode = %v, %v; want 0600, the view's", info.Mode().Perm(), err)
	}
	if snap := readView(viewPath); !snap.CanUndo || snap.Text != string(raw) {
		t.Errorf("snapshot = %+v, want the new view with undo", snap)
	}
	tempDirGone(t, log)
}

func TestGenerateUpdateRejected(t *testing.T) {
	// An update that does more than it was asked to is refused whole:
	// the view and the kept view stay as they were.
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
			if f.Kind != failInvalidOutput || !strings.Contains(f.Message, tc.want) {
				t.Fatalf("failure = %+v, want invalid-output containing %q", f, tc.want)
			}
			sentinelKept(t, viewPath, []byte(baseViewText))
			sentinelKept(t, prevViewPath(viewPath), []byte(kept))
			tempDirGone(t, log)
		})
	}
}

func TestGenerateUpdateNeedsItsView(t *testing.T) {
	// An update names components of the view there is; without one, or
	// with an id the view does not have, the agent never runs.
	viewPath := writeBase(t, "")
	for name, mutate := range map[string]func(*generationJob){
		"unknown id": updateJob(viewPath, "b", "zz"),
		"no ids":     updateJob(viewPath),
		"no view":    updateJob(filepath.Join(t.TempDir(), ".x.md.genui.json"), "b"),
	} {
		t.Run(name, func(t *testing.T) {
			err, _, log := driveGeneration(t, fakeSpec{Out: updateOut}, nil, mutate)
			if f := failureFrom(t, err); f.Kind != failAgentError || !strings.HasPrefix(f.Message, "The current view cannot be updated: ") {
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
	// Another tool rewrote the view while the agent worked: the update
	// was made for the old one and is not applied.
	viewPath := writeBase(t, "")
	spec := fakeSpec{Out: updateOut, Messages: []string{"done"}}
	err, _, _ := driveGeneration(t, spec, func(p GenerateProgress) error {
		if p.Phase == "checking" {
			return os.WriteFile(viewPath, []byte("rewritten"), 0o644)
		}
		return nil
	}, updateJob(viewPath, "b"))
	if f := failureFrom(t, err); !strings.Contains(f.Message, "changed while the agent worked") {
		t.Errorf("failure = %+v, want the conflict", f)
	}
	sentinelKept(t, viewPath, []byte("rewritten"))
	if _, err := os.Stat(prevViewPath(viewPath)); !os.IsNotExist(err) {
		t.Errorf("a view was kept: %v", err)
	}
}

func TestFullGenerationKeepsTheViewItReplaces(t *testing.T) {
	viewPath := writeBase(t, "older")
	if err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) { job.ViewPath = viewPath }); err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	sentinelKept(t, prevViewPath(viewPath), []byte(baseViewText))

	// A failed run leaves the kept view as it was.
	now, _ := os.ReadFile(viewPath)
	err, _, _ := driveGeneration(t, fakeSpec{Out: "not json"}, nil, func(job *generationJob) { job.ViewPath = viewPath })
	if err == nil {
		t.Fatal("the run with a bad out.json succeeded")
	}
	sentinelKept(t, viewPath, now)
	sentinelKept(t, prevViewPath(viewPath), []byte(baseViewText))
}

func TestUndoView(t *testing.T) {
	viewPath := writeBase(t, "the view before")
	if err := undoView(viewPath); err != nil {
		t.Fatalf("undoView: %v", err)
	}
	// The kept view is the view again, and there is nothing to go back to.
	snap := readView(viewPath)
	if snap.Text != "the view before" || snap.CanUndo {
		t.Errorf("after undo = %+v, want the kept view without undo", snap)
	}
	if err := undoView(viewPath); err == nil || err.Error() != "There is no earlier view to go back to." {
		t.Errorf("second undo = %v, want refused", err)
	}
	sentinelKept(t, viewPath, []byte("the view before"))
}
