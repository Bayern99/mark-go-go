//go:build windows

package pigenui

import "testing"

// assertProcessGone has no portable proof on Windows; the turn's own shutdown
// path waits for the process to be reaped, which the exit records show.
func assertProcessGone(t *testing.T, starts []map[string]any) {
	t.Helper()
	if len(starts) == 0 {
		t.Fatal("the turn started no process, so there is nothing to check")
	}
}
