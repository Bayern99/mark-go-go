package feynman

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
)

func TestFeynmanLinksExistingProfileAndKeepsTrustPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell fixture")
	}
	home, data, root, bin := fsutil.CanonicalPath(t.TempDir()), t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	fake := filepath.Join(bin, "feynman")
	if err := os.WriteFile(fake, []byte(`#!/bin/sh
profile="$FEYNMAN_HOME/.feynman"
case "$1" in
  --mode) printf 'shared session' > "$PI_CODING_AGENT_SESSION_DIR/linked.jsonl" ;;
  mcp) printf '%s' "$2" > "$profile/agent/mcp.json" ;;
esac
`), 0700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(home, ".feynman")
	write := func(rel, contents string) {
		t.Helper()
		path := filepath.Join(global, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	existing := map[string]string{
		"agent/settings.json":          `{"defaultProjectTrust":"always"}`,
		"agent/auth.json":              `{"fixture":true}`,
		"agent/models.json":            `{}`,
		"agent/mcp.json":               `{}`,
		"agent/skills/demo/SKILL.md":   "Existing skill",
		"agent/prompts/demo.md":        "Existing prompt",
		"agent/extensions/demo.ts":     "Existing extension",
		"agent/packages/demo/index.ts": "Existing package",
		"memory/note.md":               "Existing memory",
		"sessions/original.jsonl":      "Existing session",
	}
	for rel, contents := range existing {
		write(rel, contents)
	}
	const globalTrust = `{"/unrelated":true}`
	write("agent/trust.json", globalTrust)
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	work := review.WorkDir(data, root)
	cmd, err := h.researchCommand(root, work)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(h.profileHome(), ".feynman")
	for rel := range existing {
		if !fsutil.SameFile(filepath.Join(global, rel), filepath.Join(profile, rel)) {
			t.Errorf("existing %s is not linked", rel)
		}
	}
	trustPath := filepath.Join(profile, "agent", "trust.json")
	raw, err := os.ReadFile(trustPath)
	var decisions map[string]bool
	if err != nil || json.Unmarshal(raw, &decisions) != nil || len(decisions) != 2 ||
		!decisions[fsutil.CanonicalPath(work)] || !hasProfileDeny(decisions, "/") {
		t.Errorf("expected default deny and managed allow, got %s (%v)", raw, err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := h.CLIMCPAdd(ctx, root, CLIMCPAddInput{Name: "test", Transport: "http", URL: "https://example.invalid/mcp"}); err != nil {
		t.Fatal(err)
	}
	assertContents := func(path, want string) {
		t.Helper()
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), want %q", path, got, err, want)
		}
	}
	assertContents(filepath.Join(global, "agent", "mcp.json"), "add")
	if _, err := h.CLIMCPRemove(ctx, root, "test", false); err != nil {
		t.Fatal(err)
	}
	assertContents(filepath.Join(global, "agent", "mcp.json"), "remove")
	assertContents(filepath.Join(global, "sessions", "linked.jsonl"), "shared session")
	assertContents(filepath.Join(global, "agent", "trust.json"), globalTrust)

	write("agent/later.json", "new global file")
	privateState := filepath.Join(profile, "agent", "mcp-auth.json")
	if err := os.WriteFile(privateState, []byte("private state"), 0600); err != nil {
		t.Fatal(err)
	}
	write("agent/mcp-auth.json", "separately created global state")
	if _, err := h.CLIDoctor(ctx, root); err != nil {
		t.Fatal(err)
	}
	if !fsutil.SameFile(filepath.Join(global, "agent", "later.json"), filepath.Join(profile, "agent", "later.json")) {
		t.Error("new nonconflicting global file was not linked on the next launch")
	}
	assertContents(privateState, "private state")
}

func hasProfileDeny(decisions map[string]bool, root string) bool {
	allow, present := decisions[root]
	return present && !allow
}
