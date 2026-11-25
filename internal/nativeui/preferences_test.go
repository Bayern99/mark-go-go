package nativeui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func failingPreferenceStore(t *testing.T, values map[string]string) (*Window, string) {
	t.Helper()
	dir := t.TempDir()
	blocked := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := &Window{preferences: &preferenceStore{
		dir: blocked, values: values, windows: map[*Window]bool{},
	}}
	valid := filepath.Join(dir, "valid")
	if err := os.Mkdir(valid, 0o700); err != nil {
		t.Fatal(err)
	}
	return w, valid
}

func TestSetPreferenceRollsBackFailedWriteAndAllowsRetry(t *testing.T) {
	w, valid := failingPreferenceStore(t, map[string]string{"key": "old"})
	w.setPreference("key", "new")
	if got := w.preferences.values["key"]; got != "old" {
		t.Fatalf("failed write left in-memory value %q, want old", got)
	}
	if w.failure == "" {
		t.Fatal("failed preference write did not surface an error")
	}
	// Retrying the same value must not take the unchanged-value fast path.
	w.setPreference("key", "new")
	if got := w.preferences.values["key"]; got != "old" || w.failure == "" {
		t.Fatalf("same-value retry left value %q and failure %q", got, w.failure)
	}
	w.preferences.dir = valid
	w.setPreference("key", "new")
	if w.failure != "" || w.preferences.values["key"] != "new" {
		t.Fatalf("successful retry left value %q and failure %q", w.preferences.values["key"], w.failure)
	}
	data, err := os.ReadFile(w.preferences.path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"key":"new"`) {
		t.Fatalf("saved preferences = %s", data)
	}
}

func TestResetPreferencesRollsBackFailedWrite(t *testing.T) {
	w, valid := failingPreferenceStore(t, map[string]string{"a": "1", "b": "2"})
	w.ResetPreferences("a")
	if w.preferences.values["a"] != "1" || w.preferences.values["b"] != "2" || w.failure == "" {
		t.Fatalf("failed reset left values %v and failure %q", w.preferences.values, w.failure)
	}
	w.preferences.dir = valid
	w.ResetPreferences("a")
	if _, ok := w.preferences.values["a"]; ok || w.preferences.values["b"] != "2" || w.failure != "" {
		t.Fatalf("successful reset left values %v and failure %q", w.preferences.values, w.failure)
	}
}
