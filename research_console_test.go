package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestFindResearchExecutableUsesTheCommandEnvironment(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "feynman")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := findResearchExecutable([]string{"PATH=" + dir}, "feynman"); got != executable {
		t.Fatalf("got %q, want %q", got, executable)
	}
	if got := findResearchExecutable([]string{"PATH=" + dir}, "missing"); got != "" {
		t.Fatalf("got %q for missing executable", got)
	}
}

func TestResearchConsolePTYRoundTrip(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "printf 'ready>'; read line; printf 'reply:%s\\n' \"$line\"")
	term, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	if _, err := term.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	read := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(term)
		read <- string(data)
	}()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	_ = term.Close()
	select {
	case output := <-read:
		if !strings.Contains(output, "reply:hello") {
			t.Fatalf("PTY output %q", output)
		}
	case <-time.After(time.Second):
		t.Fatal("PTY output did not arrive")
	}
}

func TestValidateResearchConsoleArgsPreservesFeynmanCLI(t *testing.T) {
	for _, args := range [][]string{
		{}, {"setup"}, {"model", "list"}, {"alpha", "search", "scaling laws"},
		{"deepresearch", "topic"}, {"--window-size", "120x40"}, {"--format=json"},
		{"--no-themes"}, {"--model=local"}, {"--export", "report.md"}, {"--", "--cwd", "literal prompt"},
	} {
		if err := validateResearchConsoleArgs(args); err != nil {
			t.Fatalf("validate %v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"--cwd", "/tmp"}, {"--cwd=/tmp"}, {"setup", "--session-dir", "/tmp"},
		{"--mode=chat"},
	} {
		if err := validateResearchConsoleArgs(args); err == nil {
			t.Fatalf("accepted host-owned option %v", args)
		}
	}
}
