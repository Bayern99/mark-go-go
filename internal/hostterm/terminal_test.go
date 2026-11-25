package hostterm_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/hostterm"
)

func TestCommandLineQuotesArguments(t *testing.T) {
	got := hostterm.CommandLine("agy", "acp", "--login", "a b", "it's")
	want := `agy acp --login 'a b' 'it'\''s'`
	if got != want {
		t.Fatalf("CommandLine = %q, want %q", got, want)
	}
}

func TestOpenRunsTheCommandInItsOwnTerminal(t *testing.T) {
	called := make(chan []string, 1)
	command := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		called <- append([]string{name}, args...)
		return exec.CommandContext(ctx, "true")
	}
	dir := t.TempDir()
	restore := hostterm.SetCreateTempForTest(func(pattern string) (*os.File, error) {
		return os.CreateTemp(dir, pattern)
	})
	defer restore()

	ctx := context.Background()
	if err := hostterm.OpenForTest(ctx, "darwin", func(string) string { return "" }, "agy acp --login", command); err != nil {
		t.Fatalf("OpenForTest: %v", err)
	}
	argv := <-called
	if argv[0] != "open" || len(argv) != 4 || argv[1] != "-b" || argv[2] != "com.apple.Terminal" {
		t.Fatalf("darwin launch = %v, want open -b com.apple.Terminal <script>", argv)
	}
	script, err := os.ReadFile(filepath.Join(dir, filepath.Base(argv[3])))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "agy acp --login") || !strings.HasPrefix(string(script), "#!/bin/sh") {
		t.Fatalf("script = %q, want a shell script running the login command", script)
	}

	if err := hostterm.OpenForTest(ctx, "windows", func(string) string { return "" }, "codex login", command); err != nil {
		t.Fatalf("windows OpenForTest: %v", err)
	}
	argv = <-called
	if argv[0] != "cmd.exe" || !strings.Contains(strings.Join(argv, " "), "codex login") {
		t.Fatalf("windows launch = %v, want cmd.exe starting the login", argv)
	}

	if err := hostterm.OpenForTest(ctx, "linux", func(string) string { return "" }, "codex login", command); err == nil {
		t.Fatal("a linux launch without any terminal emulator succeeded")
	}
}
