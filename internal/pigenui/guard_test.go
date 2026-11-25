package pigenui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGuardPolicyKeepsToolCallsInTheTurn covers the guard's own decisions
// against real files: a read outside the turn's folder, a write that is not
// out.json, a climb with "..", and a symbolic link that leaves the folder.
//
// The policy is plain JavaScript so it runs under Node, which is what Pi itself
// needs; a machine without Node skips this and still runs the Go tests that prove
// a turn never starts without the guard.
func TestGuardPolicyKeepsToolCallsInTheTurn(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed, so the guard policy's own cases cannot run")
	}
	script, err := filepath.Abs(filepath.Join("guard", "policy_test.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, script)
	cmd.Dir = filepath.Join(mustAbs(t, "guard"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guard policy cases failed: %v\n%s", err, output)
	}
}

func TestTheGuardShipsWithAMatchingVersion(t *testing.T) {
	path, cleanup, err := writeGuard()
	if err != nil {
		t.Fatalf("writeGuard: %v", err)
	}
	defer cleanup()
	dir := filepath.Dir(path)
	for _, name := range guardNames {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("the guard's %s was not written: %v", name, err)
		}
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "./policy.js") {
		t.Error("the guard does not import the policy it ships with")
	}
	// The policy holds the version the guard reports; this package holds the one
	// it accepts, and a mismatch stops a turn rather than generating unguarded.
	policy, err := os.ReadFile(filepath.Join(dir, "policy.js"))
	if err != nil {
		t.Fatal(err)
	}
	declared := regexp.MustCompile(`GUARD_VERSION = "([^"]+)"`).FindSubmatch(policy)
	if declared == nil {
		t.Fatal("the guard policy declares no version")
	}
	if string(declared[1]) != GuardVersion {
		t.Fatalf("the guard policy's version is %q, want %q", declared[1], GuardVersion)
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the guard's folder survived its turn: %v", err)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
