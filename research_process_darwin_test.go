package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFeynmanRPCSettlesOnlyAnIdleHandledPrompt(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required by pi-acp")
	}
	pi := filepath.Join(t.TempDir(), "local feynman")
	sh := `#!/bin/sh
set -eu
test -z "${MGO_FEYNMAN_UI_TOKEN-}${MGO_FEYNMAN_UI_SOCKET-}"
while read -r line; do
	case "$line" in
		*'"id":"idle"'*) echo '{"type":"response","command":"prompt","id":"idle","success":true,"data":{"disposition":"handled"}}';;
		*'"id":"mgo-idle-idle"'*) echo '{"type":"response","command":"get_state","id":"mgo-idle-idle","success":true,"data":{"isStreaming":false}}';;
		*'"id":"research"'*)
			echo '{"type":"response","command":"prompt","id":"research","success":true,"data":{"disposition":"handled"}}'
			echo '{"type":"agent_start"}'
			;;
		*'"id":"mgo-idle-research"'*)
			echo '{"type":"response","command":"get_state","id":"mgo-idle-research","success":true,"data":{"isStreaming":true}}'
			echo '{"type":"message_end","message":"result"}'
			echo '{"type":"agent_settled","aborted":false}'
			;;
	esac
done
`
	if err := os.WriteFile(pi, []byte(sh), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "scripts/feynman-rpc.mjs", "--mode", "rpc")
	cmd.Env = append(os.Environ(), "MGO_FEYNMAN_EXECUTABLE="+pi, "MGO_FEYNMAN_UI_TOKEN=test-token")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(output)
	for _, turn := range []struct {
		id     string
		events []string
	}{
		{"idle", []string{"response", "agent_settled"}},
		{"research", []string{"response", "agent_start", "message_end", "agent_settled"}},
	} {
		if err := json.NewEncoder(input).Encode(map[string]string{"type": "prompt", "id": turn.id}); err != nil {
			t.Fatal(err)
		}
		for _, want := range turn.events {
			var event struct{ Type string }
			if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &event) != nil || event.Type != want {
				t.Fatalf("Expected %s for %s, received %q (%v)", want, turn.id, scanner.Bytes(), scanner.Err())
			}
		}
	}
	input.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestResearchWorkingCopyProtectsUserFiles(t *testing.T) {
	home, root, data := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)
	bin := t.TempDir()
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	work := researchWorkDir(data, root)
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "original.md")
	if err := os.WriteFile(original, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIXTURE_ORIGINAL", original)
	sh := `#!/bin/sh
set -eu
cat "$FIXTURE_ORIGINAL" > current-document.md
if (echo changed > "$FIXTURE_ORIGINAL"); then exit 7; fi
echo revised >> current-document.md
`
	for name, script := range map[string]string{"pi-acp": sh, "feynman": "#!/bin/sh\nexit 0", "node": "#!/bin/sh\nexit 0"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cmd, err := researchCommand(canonicalPath(root), work)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for file, want := range map[string]string{original: "original", filepath.Join(work, "current-document.md"): "originalrevised\n"} {
		if got, err := os.ReadFile(file); err != nil || string(got) != want {
			t.Fatalf("file=%s content=%q err=%v", file, got, err)
		}
	}
}

func TestConsoleFollowsTheSelectedSavedConversation(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	state := filepath.Join(base, "feynman")
	adapter := filepath.Join(base, "adapter")
	for _, dir := range []string{work, filepath.Join(state, "sessions"), adapter} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "session"), []byte("chosen"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "sessions", "chosen.jsonl")
	index, _ := json.Marshal(map[string]any{"sessions": map[string]any{"chosen": map[string]string{"cwd": work, "sessionFile": path}}})
	if err := os.WriteFile(filepath.Join(adapter, "session-map.json"), index, 0600); err != nil {
		t.Fatal(err)
	}
	if args := currentConsoleConversation(work, state, adapter); len(args) != 1 || args[0] != "--new-session" {
		t.Fatalf("Unpersisted selection: %v", args)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if args := currentConsoleConversation(work, state, adapter); len(args) != 2 || args[0] != "--session" || args[1] != path {
		t.Fatalf("Saved selection: %v", args)
	}
}
