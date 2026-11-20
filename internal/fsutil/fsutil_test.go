package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestWriteFileAtomicPreservesModeAndCreatesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := WriteFileAtomicMode(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomicMode: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}

	if err := WriteFileAtomic(path, []byte("second")); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "second" {
		t.Fatalf("contents = %q, %v; want second", got, err)
	}
	info2, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after overwrite: %v", err)
	}
	if runtime.GOOS != "windows" && info2.Mode().Perm() != 0o600 {
		t.Fatalf("preserved mode = %v, want 0600", info2.Mode().Perm())
	}

	missing := filepath.Join(dir, "missing", "file.txt")
	if err := WriteFileAtomic(missing, []byte("nope")); err == nil {
		t.Fatal("expected error writing into non-existent directory")
	}
}

func TestPathCanonicalizationAndContainment(t *testing.T) {
	realDir := t.TempDir()
	realFile := filepath.Join(realDir, "note.md")
	if err := os.WriteFile(realFile, []byte("# Hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	aliasParent := t.TempDir()
	aliasDir := filepath.Join(aliasParent, "alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Skip(err)
	}

	aliasFile := filepath.Join(aliasDir, "note.md")
	if !SameRoot(realDir, aliasDir) {
		t.Fatalf("SameRoot(%q, %q) = false, want true", realDir, aliasDir)
	}
	if !SameFile(realFile, aliasFile) {
		t.Fatalf("SameFile(%q, %q) = false, want true", realFile, aliasFile)
	}
	if !FolderContains(realDir, filepath.Join(aliasDir, "nested", "future.md")) {
		t.Fatal("FolderContains failed across symlinked root for future file")
	}
	if FolderContains(realDir, filepath.Join(aliasParent, "outside.md")) {
		t.Fatal("FolderContains accepted file outside root")
	}

	subDir := filepath.Join(realDir, "sub")
	if err := os.Mkdir(subDir, 0o700); err != nil {
		t.Fatal(err)
	}
	subFile := filepath.Join(subDir, "deep.md")
	if got := DeepestRoot(subFile, []string{realDir, subDir}); got != subDir {
		t.Fatalf("DeepestRoot = %q, want %q", got, subDir)
	}
}

func TestResolveExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit test is POSIX-specific")
	}
	binDir := t.TempDir()
	extraDir := t.TempDir()

	nonExec := filepath.Join(binDir, "tool-a")
	if err := os.WriteFile(nonExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	execExtra := filepath.Join(extraDir, "tool-a")
	if err := os.WriteFile(execExtra, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := ResolveExecutable("tool-a", []string{"PATH=" + binDir}, []string{extraDir})
	if !ok || got != execExtra {
		t.Fatalf("ResolveExecutable = (%q, %v), want (%q, true)", got, ok, execExtra)
	}

	explicit := filepath.Join(binDir, "custom")
	if got, ok := ResolveExecutable(explicit, nil, nil); !ok || got != explicit {
		t.Fatalf("explicit path = (%q, %v), want (%q, true)", got, ok, explicit)
	}

	if got, ok := ResolveExecutable("definitely-missing-binary-xyz", []string{"PATH=" + binDir}, nil); ok || got != "" {
		t.Fatalf("missing binary = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestMarkdownAndSkipDirHelpers(t *testing.T) {
	for _, name := range []string{"note.md", "README.MARKDOWN", "a/b.Md"} {
		if !MarkdownName(name) {
			t.Errorf("MarkdownName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"image.png", "file.txt", "md"} {
		if MarkdownName(name) {
			t.Errorf("MarkdownName(%q) = true, want false", name)
		}
	}
	for _, dir := range []string{".git", "node_modules", ".hidden", "build", "dist"} {
		if !SkipDirName(dir) {
			t.Errorf("SkipDirName(%q) = false, want true", dir)
		}
	}
	for _, dir := range []string{".", "notes", "Research"} {
		if SkipDirName(dir) {
			t.Errorf("SkipDirName(%q) = true, want false", dir)
		}
	}
}

func TestLimitBufferRetainsTailAndIsConcurrencySafe(t *testing.T) {
	buf := NewLimitBuffer(5)
	_, _ = buf.Write([]byte("abcdef"))
	_, _ = buf.Write([]byte("gh\n\n"))
	if got := buf.Tail(); got != "fgh" {
		t.Fatalf("Tail() = %q, want %q", got, "fgh")
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_, _ = buf.Write([]byte("123456"))
				_ = buf.Tail()
			}
		}()
	}
	wg.Wait()
}
