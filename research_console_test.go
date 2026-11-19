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

func TestResearchConsoleEarlyExitReapsSpawnedChild(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()
	docs := &documentHost{settings: &settingsStore{dir: data}}
	research := newResearchHost(docs)
	h := newResearchConsoleHost(docs, research)

	var spawned *exec.Cmd
	h.command = func(_, work string, _ []string) (*exec.Cmd, error) {
		cmd := exec.Command("/bin/sh", "-c", "sleep 10")
		cmd.Dir = work
		spawned = cmd
		// Simulate shutdown racing after command creation / start.
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		return cmd, nil
	}

	err := h.connect(&workspace{root: root}, nil)
	if err == nil {
		t.Fatal("expected connect to fail when console host is closing")
	}
	if spawned == nil || spawned.Process == nil {
		t.Fatal("expected command to be started before early exit")
	}
	if spawned.ProcessState == nil {
		_ = spawned.Process.Kill()
		_ = spawned.Wait()
		t.Fatal("early-exit path did not reap spawned child process (ProcessState is nil)")
	}
}

func TestResearchConsoleWindowCloseTeardownIsAsync(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()
	docs := &documentHost{settings: &settingsStore{dir: data}}
	research := newResearchHost(docs)
	h := newResearchConsoleHost(docs, research)
	defer h.shutdown()

	h.command = func(_, work string, _ []string) (*exec.Cmd, error) {
		cmd := exec.Command("/bin/sh", "-c", "trap '' INT; sleep 10")
		cmd.Dir = work
		return cmd, nil
	}

	if err := h.connect(&workspace{root: root}, nil); err != nil {
		t.Fatal(err)
	}
	canon := canonicalPath(root)
	h.mu.Lock()
	session := h.sessions[canon]
	h.mu.Unlock()
	if session == nil {
		t.Fatal("missing console session")
	}

	started := time.Now()
	h.handleWindowClosed(canon, session)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("window-close teardown blocked caller for %v", elapsed)
	}

	select {
	case <-session.done:
	case <-time.After(5 * time.Second):
		t.Fatal("async window-close teardown did not finish")
	}
}
