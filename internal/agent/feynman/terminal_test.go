package feynman

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMacTerminalOpenTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	var path string
	err := openMacTerminal(ctx, "feynman setup", func(gotCtx context.Context, name string, args ...string) *exec.Cmd {
		if gotDeadline, ok := gotCtx.Deadline(); !ok || !gotDeadline.Equal(deadline) {
			t.Fatalf("open deadline = %v, %v; want %v", gotDeadline, ok, deadline)
		}
		if name != "open" || len(args) != 3 || args[0] != "-b" || args[1] != "com.apple.Terminal" {
			t.Fatalf("unexpected Terminal command: %s %q", name, args)
		}
		path = args[2]
		if filepath.Ext(path) != ".command" {
			t.Fatalf("unexpected Terminal document: %q", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("launcher missing before open: %v", err)
		}
		cmd := exec.CommandContext(gotCtx, os.Args[0], "-test.run=^TestMacTerminalBlockingOpen$")
		cmd.Env = append(os.Environ(), "MGO_FIXTURE_BLOCKING_OPEN=1")
		return cmd
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("open error = %v, want context.DeadlineExceeded", err)
	}
	if path == "" {
		t.Fatal("open was not invoked")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("timed-out handoff left a launcher: %v", err)
	}
}

func TestMacTerminalBlockingOpen(t *testing.T) {
	if os.Getenv("MGO_FIXTURE_BLOCKING_OPEN") != "1" {
		return
	}
	<-time.NewTimer(time.Hour).C
}

func TestMacTerminalHandsOffWizard(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("uses the macOS Terminal launcher")
	}
	bin := t.TempDir()
	fixtureHome := t.TempDir()
	fnmBin := filepath.Join(fixtureHome, ".local", "share", "fnm", "aliases", "default", "bin")
	if err := os.MkdirAll(fnmBin, 0700); err != nil {
		t.Fatal(err)
	}
	launchDir := t.TempDir()
	profile := filepath.Join(t.TempDir(), "Test's Feynman profile")
	output := filepath.Join(bin, "wizard-output")
	request := filepath.Join(bin, "open-request")
	t.Setenv("HOME", fixtureHome)
	t.Setenv("PATH", strings.Join([]string{bin, "/usr/bin", "/bin"}, string(os.PathListSeparator)))
	t.Setenv("TMPDIR", launchDir)
	t.Setenv("MGO_TERMINAL_TEST_OUTPUT", output)
	t.Setenv("MGO_TERMINAL_TEST_REQUEST", request)
	for name, script := range map[string]string{
		filepath.Join(bin, "osascript"): "#!/bin/sh\nexec /bin/sleep 10\n",
		filepath.Join(bin, "open"): `#!/bin/sh
set -eu
[ "$#" -eq 3 ]
[ "$1" = -b ]
[ "$2" = com.apple.Terminal ]
printf '%s\n' "$3" > "$MGO_TERMINAL_TEST_REQUEST"
`,
		filepath.Join(fnmBin, "feynman"): `#!/bin/sh
printf '%s\n' "$FEYNMAN_HOME" "$PI_CODING_AGENT_DIR" "$PI_CODING_AGENT_SESSION_DIR" "$@" > "$MGO_TERMINAL_TEST_OUTPUT"
`,
	} {
		if err := os.WriteFile(name, []byte(script), 0700); err != nil {
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
