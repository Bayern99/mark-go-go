//go:build !windows

package pigenui

import (
	"syscall"
	"testing"
)

// assertProcessGone proves a finished turn left no Pi process behind.
func assertProcessGone(t *testing.T, starts []map[string]any) {
	t.Helper()
	if len(starts) == 0 {
		t.Fatal("the turn started no process, so there is nothing to check")
	}
	for _, start := range starts {
		pid, ok := start["pid"].(float64)
		if !ok {
			t.Fatalf("start record has no pid: %v", start)
		}
		err := syscall.Kill(int(pid), 0)
		if err == nil {
			t.Errorf("process %d is still running after the turn ended", int(pid))
		}
	}
}
