// Package agentcli puts the markgogo command this app carries into the user's
// own bin folder, and removes only the link this app made. It writes nothing
// outside that folder: no sudo, no /usr/local/bin, no shell profile.
package agentcli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

// Name is the command's file name.
const Name = "markgogo"

// State is what the install path holds.
type State int

const (
	// StateAbsent means nothing is installed there yet.
	StateAbsent State = iota
	// StateInstalled means the path is the link this app makes.
	StateInstalled
	// StateForeign means someone else's file is there; the app leaves it alone.
	StateForeign
)

// Installer links a markgogo executable into a folder on the user's PATH.
type Installer struct {
	// Source is the markgogo this build carries; empty when it carries none.
	Source string
	// Dir is where the command is linked, normally ~/.local/bin.
	Dir string
}

// Default is the installer for this build: the copy of markgogo that ships
// with the app, linked into ~/.local/bin. An empty Source means the build
// carries no such command and the page has to say how to install one.
func Default() Installer {
	return Installer{Source: shippedSource(), Dir: userBinDir()}
}

// Path is where the command is installed.
func (i Installer) Path() string {
	if strings.TrimSpace(i.Dir) == "" {
		return ""
	}
	return filepath.Join(i.Dir, Name)
}

// Shipped reports whether this build carries a markgogo to install.
func (i Installer) Shipped() bool {
	return strings.TrimSpace(i.Source) != "" && isExecutable(i.Source)
}

// State reports what the install path holds.
func (i Installer) State() State {
	path := i.Path()
	if path == "" {
		return StateAbsent
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return StateAbsent
	}
	if err != nil {
		return StateForeign
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return StateForeign
	}
	target, err := os.Readlink(path)
	if err != nil {
		return StateForeign
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return linkState(target, i.Source)
}

// linkState says whether a link's target is this app's command. A link into an
// app bundle counts even when the bundle has moved, so an app update leaves a
// link the reader can still install over or remove.
func linkState(target, source string) State {
	if source != "" && fsutil.SameFile(target, source) {
		return StateInstalled
	}
	if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) && inAppBundle(target) {
		return StateInstalled
	}
	return StateForeign
}

// inAppBundle reports whether a path is a command inside a macOS app bundle,
// which is where an earlier copy of this app kept its markgogo.
func inAppBundle(path string) bool {
	return filepath.Base(path) == Name && strings.Contains(filepath.ToSlash(path), ".app/Contents/MacOS/")
}

// Install links this build's command into Dir, creating the folder if it is
// missing. A path it did not make is never overwritten.
func (i Installer) Install() error {
	if i.Path() == "" {
		return errors.New("There is no folder to install the markgogo command in.")
	}
	if !i.Shipped() {
		return errors.New("This build of Mark Go Go does not carry the markgogo command.")
	}
	switch i.State() {
	case StateInstalled:
		return nil
	case StateForeign:
		return fmt.Errorf("%s already holds another program. Remove it, then install again.", i.Path())
	}
	if err := os.MkdirAll(i.Dir, 0o755); err != nil {
		return err
	}
	// Link beside the destination and rename, so a reader never finds a
	// half-written command on PATH.
	temp := i.Path() + ".tmp"
	if err := os.Remove(temp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Symlink(i.Source, temp); err != nil {
		return err
	}
	if err := os.Rename(temp, i.Path()); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

// Uninstall removes the link this app made and reports a path it did not make.
func (i Installer) Uninstall() error {
	path := i.Path()
	if path == "" {
		return nil
	}
	switch i.State() {
	case StateAbsent:
		return nil
	case StateInstalled:
		return os.Remove(path)
	default:
		return fmt.Errorf("%s is not a link Mark Go Go made, so it is left alone.", path)
	}
}

// userBinDir is ~/.local/bin, the folder this app installs into.
func userBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".local", "bin")
}

// shippedSource is the markgogo this build carries: one shipped as a resource,
// else one next to the app's own executable.
func shippedSource() string {
	var candidates []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil && dir != "" {
		candidates = append(candidates, filepath.Join(dir, Name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), Name))
	}
	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate
		}
	}
	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
