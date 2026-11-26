package pandoc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, tc := range []struct {
		line                string
		major, minor, patch int
		version             string
		ok                  bool
	}{
		{"pandoc 3.13", 3, 13, 0, "3.13", true},
		{"pandoc 2.15.0\r", 2, 15, 0, "2.15.0", true},
		{"pandoc 2.9.2", 2, 9, 2, "2.9.2", true},
		{"not pandoc", 0, 0, 0, "", false},
	} {
		major, minor, patch, version, ok := parseVersion(tc.line)
		if major != tc.major || minor != tc.minor || patch != tc.patch || version != tc.version || ok != tc.ok {
			t.Errorf("parseVersion(%q) = %d.%d.%d %q %v", tc.line, major, minor, patch, version, ok)
		}
	}
}

func TestProbeRejectsOldVersion(t *testing.T) {
	installFakeCommand(t)
	status := Probe(context.Background(), os.Args[0])
	if status.Available || status.Version != "2.14.2" || !strings.Contains(status.Error, "2.15.0") {
		t.Fatalf("Probe old version = %+v", status)
	}
}

func TestProbeExplicitInvalidPathDoesNotFallBack(t *testing.T) {
	previousResolve := resolveExecutable
	previousCommand := commandContext
	t.Cleanup(func() {
		resolveExecutable = previousResolve
		commandContext = previousCommand
	})
	resolveCalled := false
	resolveExecutable = func(string, []string, []string) (string, bool) {
		resolveCalled = true
		return os.Args[0], true
	}
	commandContext = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("an invalid explicit path reached the command")
		return nil
	}
	status := Probe(context.Background(), filepath.Join(t.TempDir(), "missing-pandoc"))
	if status.Available || resolveCalled || !strings.Contains(status.Error, "Configured Pandoc path") {
		t.Fatalf("Probe invalid explicit path = %+v, resolve called = %v", status, resolveCalled)
	}
	relative := Probe(context.Background(), "pandoc")
	if relative.Available || resolveCalled || !strings.Contains(relative.Error, "absolute path") {
		t.Fatalf("Probe relative explicit path = %+v", relative)
	}
}

func installFakeCommand(t *testing.T) {
	t.Helper()
	previous := commandContext
	t.Cleanup(func() { commandContext = previous })
	commandContext = func(ctx context.Context, path string, args ...string) *exec.Cmd {
		helperArgs := append([]string{"-test.run=^TestPandocHelperProcess$", "--", path}, args...)
		command := exec.CommandContext(ctx, os.Args[0], helperArgs...)
		command.Env = append(os.Environ(), "MGG_PANDOC_HELPER=1")
		return command
	}
}

func TestPandocHelperProcess(t *testing.T) {
	if os.Getenv("MGG_PANDOC_HELPER") != "1" {
		return
	}
	separator := 0
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	args := os.Args[separator+2:]
	if len(args) == 1 && args[0] == "--version" {
		_, _ = os.Stdout.WriteString("pandoc 2.14.2\n")
		os.Exit(0)
	}
	if record := os.Getenv("MGG_PANDOC_RECORD"); record != "" {
		_ = os.WriteFile(record, []byte(strings.Join(args, "\n")), 0o600)
	}
	if os.Getenv("MGG_PANDOC_FAIL") == "1" {
		_, _ = os.Stderr.WriteString("[WARNING] fake warning\nError producing PDF.\nfake exit 99\n")
		os.Exit(99)
	}
	input := args[len(args)-1]
	data, _ := os.ReadFile(input)
	if inputRecord := os.Getenv("MGG_PANDOC_INPUT"); inputRecord != "" {
		_ = os.WriteFile(inputRecord, data, 0o600)
	}
	for i, arg := range args {
		if arg == "-H" && i+1 < len(args) && os.Getenv("MGG_PANDOC_HEADER") != "" {
			header, _ := os.ReadFile(args[i+1])
			_ = os.WriteFile(os.Getenv("MGG_PANDOC_HEADER"), header, 0o600)
		}
		if arg == "-o" && i+1 < len(args) {
			_ = os.WriteFile(args[i+1], []byte("fake Pandoc output"), 0o600)
		}
	}
	_, _ = os.Stderr.WriteString("[WARNING] collected warning\n")
	os.Exit(0)
}
