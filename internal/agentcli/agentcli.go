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

func linkState(target, source string) State {
	if source == "" {
		return StateForeign
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return StateForeign
	}
	target = filepath.Clean(target)
	if target == source {
		return StateInstalled
	}
	contents := filepath.Dir(filepath.Dir(source))
	if filepath.Base(contents) == "Contents" && strings.HasSuffix(filepath.Base(filepath.Dir(contents)), ".app") {
		for _, folder := range []string{"MacOS", "Resources"} {
			if target == filepath.Join(contents, folder, Name) {
				return StateInstalled
			}
		}
	}
	return StateForeign
}

// current reports whether the installed link already resolves to this build's
// command; a link to a previous location inside the same bundle may not.
func (i Installer) current() bool {
	target, err := os.Readlink(i.Path())
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(i.Dir, target)
	}
	return fsutil.SameFile(target, i.Source)
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
	state := i.State()
	switch state {
	case StateInstalled:
		if i.current() {
			return nil
		}
	case StateForeign:
		return fmt.Errorf("%s already holds another program. Remove it, then install again.", i.Path())
	}
	if err := os.MkdirAll(i.Dir, 0o755); err != nil {
		return err
	}
	// A link resolves against its own folder, so it has to name an absolute
	// source.
	source, err := filepath.Abs(i.Source)
	if err != nil {
		return err
	}
	if state == StateAbsent {
		return os.Symlink(source, i.Path())
	}
	tempDir, err := os.MkdirTemp(i.Dir, ".markgogo-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	temp := filepath.Join(tempDir, Name)
	if err := os.Symlink(source, temp); err != nil {
		return err
	}
	if i.State() != StateInstalled {
		return fmt.Errorf("%s changed during installation.", i.Path())
	}
	return os.Rename(temp, i.Path())
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
