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

func TestALinkIntoAnAppBundleSurvivesAnUpdate(t *testing.T) {
	for _, folder := range []string{"MacOS", "Resources"} {
		t.Run(folder, func(t *testing.T) {
			dir := t.TempDir()
			// The app has been replaced since the link was made, so its target is
			// gone: the link still points into a bundle, and the reader can remove
			// it or install over it.
			bundle := filepath.Join(t.TempDir(), "Mark Go Go.app", "Contents", folder, Name)
			if err := os.Symlink(bundle, filepath.Join(dir, Name)); err != nil {
				t.Fatal(err)
			}
			installer := Installer{Source: writeShipped(t, t.TempDir()), Dir: dir}
			if state := installer.State(); state != StateInstalled {
				t.Fatalf("state = %v, want the app's own link", state)
			}
			if err := installer.Install(); err != nil {
				t.Fatalf("reinstalling over the app's link: %v", err)
			}
			target, err := os.Readlink(installer.Path())
			if err != nil {
				t.Fatalf("installing did not leave a link: %v", err)
			}
			if target != installer.Source {
				t.Fatalf("the reinstalled link points at %s, want this build's %s", target, installer.Source)
			}
			if _, err := os.Stat(installer.Path()); err != nil {
				t.Fatalf("the reinstalled command does not resolve: %v", err)
			}
			if err := installer.Uninstall(); err != nil {
				t.Fatalf("removing the app's link: %v", err)
			}
		})
	}
}
