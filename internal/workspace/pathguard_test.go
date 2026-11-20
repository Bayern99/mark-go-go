package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPathChangeRefusesAFileAnotherSessionHas(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "b.md")
	open := filepath.Join(dir, "a.md")
	writeFile(t, source, "# B source\n")
	writeFile(t, open, "# A target\n")

	h := NewService()
	a := hostSession(t, h, "doc-a", open)
	b := hostSession(t, h, "doc-b", source)

	if _, err := h.reserveTarget(b.id, b.currentPath(), a.currentPath()); err != errAlreadyOpen {
		t.Fatalf("Save As onto an open file = %v, want errAlreadyOpen", err)
	}
	if a.id != "doc-a" || b.id != "doc-b" || a.currentPath() != open || b.currentPath() != source {
		t.Fatal("a refused path change must not rewrite a session's path or id")
	}
	if got := readFile(t, open); got != "# A target\n" {
		t.Fatalf("target bytes = %q, want untouched", got)
	}
	if got := readFile(t, source); got != "# B source\n" {
		t.Fatalf("source bytes = %q, want untouched", got)
	}

	if _, err := h.reserveTarget(a.id, a.currentPath(), source); err != errAlreadyOpen {
		t.Fatalf("target session's own change = %v, want errAlreadyOpen", err)
	}
}

func TestPathChangeFollowsAliasesAndLinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "a.md")
	writeFile(t, real, "# A\n")
	aliasDir := filepath.Join(t.TempDir(), "ws-alias")
	if err := os.Symlink(dir, aliasDir); err != nil {
		t.Skip(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	hard := filepath.Join(dir, "hard.md")
	if err := os.Link(real, hard); err != nil {
		t.Skip(err)
	}

	h := NewService()
	hostSession(t, h, "doc-a", real)
	source := filepath.Join(dir, "b.md")
	writeFile(t, source, "# B\n")
	hostSession(t, h, "doc-b", source)

	for _, target := range []string{
		filepath.Join(aliasDir, "a.md"),
		link,
		hard,
	} {
		if _, err := h.reserveTarget("doc-b", source, target); err != errAlreadyOpen {
			t.Errorf("target %q = %v, want errAlreadyOpen", target, err)
		}
	}
}

func TestPathChangeClaimsTargetUntilTheFileOperationEnds(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.md")
	bPath := filepath.Join(dir, "b.md")
	writeFile(t, aPath, "# A\n")
	writeFile(t, bPath, "# B\n")

	h := NewService()
	hostSession(t, h, "doc-a", aPath)
	hostSession(t, h, "doc-b", bPath)
	target := filepath.Join(dir, "shared.md")

	release, err := h.reserveTarget("doc-a", aPath, target)
	if err != nil || release == nil {
		t.Fatalf("first claim err = %v, release nil = %v", err, release == nil)
	}
	if _, err := h.reserveTarget("doc-b", bPath, target); err != errAlreadyOpen {
		t.Fatalf("claim by another session = %v, want errAlreadyOpen", err)
	}
	release()
	if _, err := h.reserveTarget("doc-b", bPath, target); err != nil {
		t.Fatalf("after release = %v, want the target free", err)
	}
}

func TestPathChangeLockFollowsTheDocument(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.md")
	bPath := filepath.Join(dir, "b.md")
	writeFile(t, aPath, "# A\n")
	writeFile(t, bPath, "# B\n")

	h := NewService()
	a := hostSession(t, h, "doc-a", aPath)
	b := hostSession(t, h, "doc-b", bPath)
	a.generating = true

	if err := h.lockedPathChange(a.id); err == nil || err.Error() != errGenBusy {
		t.Fatalf("generating document lock = %v, want %q", err, errGenBusy)
	}
	if err := h.lockedPathChange(b.id); err != nil {
		t.Fatalf("idle document lock = %v, want none", err)
	}
	if _, err := h.reserveTarget(a.id, aPath, aPath); err != nil {
		t.Fatalf("own file while generating = %v, want allowed", err)
	}
	if _, err := h.reserveTarget(a.id, aPath, filepath.Join(dir, "moved.md")); err == nil || err.Error() != errGenBusy {
		t.Fatalf("path change while generating = %v, want %q", err, errGenBusy)
	}
	if _, err := h.reserveTarget(b.id, bPath, filepath.Join(dir, "b-moved.md")); err != nil {
		t.Fatalf("idle document path change = %v, want allowed", err)
	}
}

func TestSaveRefusedWhileGeneratingIsVisibleOnTheDocument(t *testing.T) {
	h := NewService()
	s := hostSession(t, h, "doc-a", "")
	s.generating = true

	err := h.Save(context.Background(), s.id)
	if err == nil || err.Error() != errGenBusy {
		t.Fatalf("Save while generating = %v, want %q", err, errGenBusy)
	}
	if got := s.info().ActionError; got != errGenBusy {
		t.Fatalf("ActionError = %q, want %q", got, errGenBusy)
	}

	h.noteAction(s, mygo.ErrCanceled)
	if got := s.info().ActionError; got != "" {
		t.Fatalf("ActionError after cancel = %q, want empty", got)
	}

	h.noteAction(s, errAlreadyOpen)
	if got := s.info().ActionError; !strings.Contains(got, "already open") {
		t.Fatalf("ActionError = %q, want the refusal", got)
	}
	h.noteAction(s, nil)
	if got := s.info().ActionError; got != "" {
		t.Fatalf("ActionError after success = %q, want empty", got)
	}
}
