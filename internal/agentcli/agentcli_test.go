package agentcli

import (
	"os"
	"path/filepath"
	"testing"
)

// writeShipped lays out a build that carries a markgogo executable.
func writeShipped(t *testing.T, dir string) string {
	t.Helper()
	source := filepath.Join(dir, Name)
	if err := os.WriteFile(source, []byte("#!/bin/sh\necho markgogo\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return source
}

func TestInstallLinksTheCommandAndUninstallRemovesIt(t *testing.T) {
	build, home := t.TempDir(), t.TempDir()
	installer := Installer{Source: writeShipped(t, build), Dir: filepath.Join(home, ".local", "bin")}
	if !installer.Shipped() {
		t.Fatal("a build with an executable beside it does not think it ships the command")
	}
	if state := installer.State(); state != StateAbsent {
		t.Fatalf("state before installing = %v, want absent", state)
	}

	if err := installer.Install(); err != nil {
		t.Fatalf("install: %v", err)
	}
	info, err := os.Lstat(installer.Path())
	if err != nil {
		t.Fatalf("the command is not installed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("installed %s is not a link", installer.Path())
	}
	if state := installer.State(); state != StateInstalled {
		t.Fatalf("state after installing = %v, want installed", state)
	}
	if err := installer.Install(); err != nil {
		t.Fatalf("installing twice: %v", err)
	}

	if err := installer.Uninstall(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Lstat(installer.Path()); !os.IsNotExist(err) {
		t.Fatalf("the command survived uninstall: %v", err)
	}
	if state := installer.State(); state != StateAbsent {
		t.Fatalf("state after uninstalling = %v, want absent", state)
	}
	if err := installer.Uninstall(); err != nil {
		t.Fatalf("uninstalling twice: %v", err)
	}
}

func TestABuildWithoutTheCommandSaysSoAndInstallsNothing(t *testing.T) {
	installer := Installer{Dir: t.TempDir()}
	if installer.Shipped() {
		t.Fatal("a build with no markgogo claims to ship one")
	}
	if err := installer.Install(); err == nil {
		t.Fatal("installed a command this build does not carry")
	}
	if _, err := os.Lstat(installer.Path()); !os.IsNotExist(err) {
		t.Fatalf("an empty build wrote %s: %v", installer.Path(), err)
	}
}

func TestAnotherProgramsCommandIsLeftAlone(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, Name)
	if err := os.WriteFile(foreign, []byte("#!/bin/sh\necho someone else\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Source: writeShipped(t, t.TempDir()), Dir: dir}
	if state := installer.State(); state != StateForeign {
		t.Fatalf("state = %v, want foreign", state)
	}
	if err := installer.Install(); err == nil {
		t.Fatal("install replaced a file it did not make")
	}
	if err := installer.Uninstall(); err == nil {
		t.Fatal("uninstall removed a file it did not make")
	}
	kept, err := os.ReadFile(foreign)
	if err != nil || string(kept) != "#!/bin/sh\necho someone else\n" {
		t.Fatalf("the other program's command = %q, %v", kept, err)
	}
}

func TestReinstallReplacesAnOldLocationInsideThisBundle(t *testing.T) {
	contents := filepath.Join(t.TempDir(), "Mark Go Go.app", "Contents")
	resources := filepath.Join(contents, "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Source: writeShipped(t, resources), Dir: t.TempDir()}
	if err := os.Symlink(filepath.Join(contents, "MacOS", Name), installer.Path()); err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(installer.Path()); err != nil || target != installer.Source {
		t.Fatalf("installed target = %q, %v", target, err)
	}
}

func TestInstallPreservesAnUnrelatedTemporaryFile(t *testing.T) {
	installer := Installer{Source: writeShipped(t, t.TempDir()), Dir: t.TempDir()}
	temp := installer.Path() + ".tmp"
	if err := os.WriteFile(temp, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(temp); err != nil || string(got) != "keep" {
		t.Fatalf("unrelated file = %q, %v", got, err)
	}
}

func TestDanglingLinkToAnotherAppIsForeign(t *testing.T) {
	installer := Installer{Source: writeShipped(t, t.TempDir()), Dir: t.TempDir()}
	target := filepath.Join(t.TempDir(), "Another.app", "Contents", "Resources", Name)
	if err := os.Symlink(target, installer.Path()); err != nil {
		t.Fatal(err)
	}
	if installer.State() != StateForeign || installer.Install() == nil || installer.Uninstall() == nil {
		t.Fatal("the foreign link was treated as an installed command")
	}
	if got, err := os.Readlink(installer.Path()); err != nil || got != target {
		t.Fatalf("foreign target = %q, %v", got, err)
	}
}
