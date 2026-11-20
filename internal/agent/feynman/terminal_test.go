package feynman

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMacTerminalHandsOffWizard(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("uses the macOS Terminal launcher")
	}
	bin := t.TempDir()
	launchDir := t.TempDir()
	profile := filepath.Join(t.TempDir(), "Test's Feynman profile")
	output := filepath.Join(bin, "wizard-output")
	request := filepath.Join(bin, "open-request")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", launchDir)
	t.Setenv("MGO_TERMINAL_TEST_OUTPUT", output)
	t.Setenv("MGO_TERMINAL_TEST_REQUEST", request)
	for name, script := range map[string]string{
		"osascript": "#!/bin/sh\nexec /bin/sleep 10\n",
		"open": `#!/bin/sh
set -eu
[ "$#" -eq 3 ]
[ "$1" = -b ]
[ "$2" = com.apple.Terminal ]
printf '%s\n' "$3" > "$MGO_TERMINAL_TEST_REQUEST"
`,
		"feynman": `#!/bin/sh
printf '%s\n' "$FEYNMAN_HOME" "$PI_CODING_AGENT_DIR" "$PI_CODING_AGENT_SESSION_DIR" "$@" > "$MGO_TERMINAL_TEST_OUTPUT"
`,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := shellJoinCommand("feynman", "model", "login", "Test's provider")
	if err := openSystemTerminal(ctx, command, profile); err != nil {
		t.Fatalf("hand off to macOS Terminal: %v", err)
	}
	cancel()
	opened, err := os.ReadFile(request)
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSuffix(string(opened), "\n")
	if filepath.Dir(path) != launchDir || filepath.Ext(path) != ".command" {
		t.Fatalf("unexpected Terminal document: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0700 {
		t.Fatalf("unsafe launch file mode: %v", info.Mode())
	}
	if out, err := exec.Command(path).CombinedOutput(); err != nil {
		t.Fatalf("run Terminal document after handoff: %v: %s", err, out)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{profile, filepath.Join(profile, ".feynman", "agent"),
		filepath.Join(profile, ".feynman", "sessions"), "model", "login", "Test's provider"}, "\n") + "\n"
	if string(got) != want {
		t.Errorf("wizard received %q, want %q", got, want)
	}
	entries, err := os.ReadDir(launchDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("handoff did not clean up the launcher: %v, %v", entries, err)
	}

	t.Run("failed open removes the launcher", func(t *testing.T) {
		script := "#!/bin/sh\nprintf 'Terminal could not open document\\n' >&2\nexit 1\n"
		if err := os.WriteFile(filepath.Join(bin, "open"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := openSystemTerminal(ctx, command, profile); err == nil || !strings.Contains(err.Error(), "Terminal could not open document") {
			t.Fatalf("expected launch error, got %v", err)
		}
		entries, err := os.ReadDir(launchDir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("failed handoff left a launcher: %v, %v", entries, err)
		}
	})
}
