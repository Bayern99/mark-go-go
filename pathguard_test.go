package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// The kernel's path-change contract: before MyGo writes to another file
// or moves a document's file, claimPath asks the app (DocumentsOptions.
// PathChange, then ReservePath) and performs the write or rename only
// when it says yes. These tests drive that decision through the host's
// public operations and reservations against real files, then read the
// bytes back; MyGo's own call order is verified on the real macOS app,
// where its dialogs and main loop exist.

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

// TestPathChangeRefusesAFileAnotherSessionHas checks the reservation
// Save As, Rename, Move To and the Save As a read-only or untitled Save
// falls back to all make before touching the disk. An open file is
// refused, both files keep their bytes, both sessions keep their paths
// and ids, and neither tab moves.
func TestPathChangeRefusesAFileAnotherSessionHas(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "b.md")
	open := filepath.Join(dir, "a.md")
	writeFile(t, source, "# B source\n")
	writeFile(t, open, "# A target\n")

	h := newDocumentHost()
	a := hostSession(t, h, "doc-a", open)
	b := hostSession(t, h, "doc-b", source)

	// The source is what Rename and Move To move, and what a read-only
	// Save's Save As dialog would offer by default.
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

	// The other direction is refused the same way, whichever session asks.
	if _, err := h.reserveTarget(a.id, a.currentPath(), source); err != errAlreadyOpen {
		t.Fatalf("target session's own change = %v, want errAlreadyOpen", err)
	}
}

// TestPathChangeFollowsAliasesAndLinks checks a target is the open file
// even when reached through a directory alias, a symlink or a hard link.
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

	h := newDocumentHost()
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

// TestPathChangeClaimsTargetUntilTheFileOperationEnds checks two
// sessions cannot both take a file that no one has yet, and that the
// claim is released afterwards.
func TestPathChangeClaimsTargetUntilTheFileOperationEnds(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.md")
	bPath := filepath.Join(dir, "b.md")
	writeFile(t, aPath, "# A\n")
	writeFile(t, bPath, "# B\n")

	h := newDocumentHost()
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

// TestPathChangeLockFollowsTheDocument checks the generation lock is per
// document: the generating document may not change its file, another one
// may, and an ordinary save to the document's own file still may.
func TestPathChangeLockFollowsTheDocument(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.md")
	bPath := filepath.Join(dir, "b.md")
	writeFile(t, aPath, "# A\n")
	writeFile(t, bPath, "# B\n")

	h := newDocumentHost()
	a := hostSession(t, h, "doc-a", aPath)
	b := hostSession(t, h, "doc-b", bPath)
	a.generating = true

	if err := h.lockedPathChange(a.id); err == nil || err.Error() != errGenerating {
		t.Fatalf("generating document lock = %v, want %q", err, errGenerating)
	}
	if err := h.lockedPathChange(b.id); err != nil {
		t.Fatalf("idle document lock = %v, want none", err)
	}
	// Ordinary Save writes the document's own file, not another one.
	if _, err := h.reserveTarget(a.id, aPath, aPath); err != nil {
		t.Fatalf("own file while generating = %v, want allowed", err)
	}
	// A path change that reaches the kernel (a late Save As answer, the
	// read-only fallback after the user picked a target) is refused.
	if _, err := h.reserveTarget(a.id, aPath, filepath.Join(dir, "moved.md")); err == nil || err.Error() != errGenerating {
		t.Fatalf("path change while generating = %v, want %q", err, errGenerating)
	}
	// Another document is untouched by A's lock.
	if _, err := h.reserveTarget(b.id, bPath, filepath.Join(dir, "b-moved.md")); err != nil {
		t.Fatalf("idle document path change = %v, want allowed", err)
	}
}

// TestSaveRefusedWhileGeneratingIsVisibleOnTheDocument checks the host's
// own Save preflight: an untitled document's save is a Save As, refused
// before any dialog while generating, and the failure reaches the
// document's state so its window can show it. A cancel or a later
// success clears it.
func TestSaveRefusedWhileGeneratingIsVisibleOnTheDocument(t *testing.T) {
	h := newDocumentHost()
	s := hostSession(t, h, "doc-a", "")
	s.generating = true

	err := h.Save(context.Background(), s.id)
	if err == nil || err.Error() != errGenerating {
		t.Fatalf("Save while generating = %v, want %q", err, errGenerating)
	}
	if got := s.info().ActionError; got != errGenerating {
		t.Fatalf("ActionError = %q, want %q", got, errGenerating)
	}

	// A canceled dialog is not a failure and clears the message.
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
