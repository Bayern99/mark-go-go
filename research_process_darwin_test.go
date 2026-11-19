package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResearchWorkingCopyProtectsUserFiles(t *testing.T) {
	home, root, data := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)
	bin := t.TempDir()
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	work := researchWorkDir(data, root)
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "original.md")
	if err := os.WriteFile(original, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIXTURE_ORIGINAL", original)
	sh := `#!/bin/sh
set -eu
cat "$FIXTURE_ORIGINAL" > current-document.md
if (echo changed > "$FIXTURE_ORIGINAL"); then exit 7; fi
echo revised >> current-document.md
`
	for name, script := range map[string]string{"pi-acp": sh, "feynman": "#!/bin/sh\nexit 0", "node": "#!/bin/sh\nexit 0"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cmd, err := researchCommand(canonicalPath(root), work)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for file, want := range map[string]string{original: "original", filepath.Join(work, "current-document.md"): "originalrevised\n"} {
		if got, err := os.ReadFile(file); err != nil || string(got) != want {
			t.Fatalf("file=%s content=%q err=%v", file, got, err)
		}
	}
}
