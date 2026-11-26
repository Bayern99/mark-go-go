package pandoc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

const InstallCommand = "brew install pandoc"

var versionPattern = regexp.MustCompile(`^pandoc\s+(\d+)\.(\d+)(?:\.(\d+))?`)
var commandContext = exec.CommandContext
var resolveExecutable = fsutil.ResolveExecutable
var userHomeDir = os.UserHomeDir

type Status struct {
	Available   bool
	Path        string
	Version     string
	Error       string
	InstallHint string
}

func Probe(ctx context.Context, configuredPath string, extraDirs ...[]string) Status {
	status := Status{Path: strings.TrimSpace(configuredPath), InstallHint: InstallCommand}
	if status.Path != "" {
		if !filepath.IsAbs(status.Path) {
			status.Error = "Configured Pandoc path must be an absolute path."
			return status
		}
		info, err := os.Stat(status.Path)
		if err != nil {
			status.Error = fmt.Sprintf("Configured Pandoc path is unavailable: %v", err)
			return status
		}
		if !info.Mode().IsRegular() {
			status.Error = "Configured Pandoc path is not a regular file."
			return status
		}
	} else {
		dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
		for _, extra := range extraDirs {
			dirs = append(dirs, extra...)
		}
		if home, err := userHomeDir(); err == nil && home != "" {
			dirs = append(dirs, filepath.Join(home, ".nix-profile", "bin"))
		}
		path, ok := resolveExecutable("pandoc", nil, dirs)
		if !ok {
			status.Error = "Pandoc 2.15.0 or later was not found."
			return status
		}
		status.Path = path
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := commandContext(probeCtx, status.Path, "--version").Output()
	if err != nil {
		status.Error = fmt.Sprintf("Could not run pandoc --version: %v", err)
		return status
	}
	firstLine, _, _ := strings.Cut(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	major, minor, _, version, ok := parseVersion(firstLine)
	if !ok {
		status.Error = "Could not parse the Pandoc version."
		return status
	}
	status.Version = version
	if major < 2 || major == 2 && minor < 15 {
		status.Error = fmt.Sprintf("Pandoc %s is too old; version 2.15.0 or later is required.", version)
		return status
	}
	status.Available = true
	return status
}

func parseVersion(line string) (major, minor, patch int, version string, ok bool) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return 0, 0, 0, "", false
	}
	major, err1 := strconv.Atoi(match[1])
	minor, err2 := strconv.Atoi(match[2])
	if match[3] != "" {
		patch, _ = strconv.Atoi(match[3])
	}
	if err1 != nil || err2 != nil {
		return 0, 0, 0, "", false
	}
	version = fmt.Sprintf("%d.%d", major, minor)
	if match[3] != "" {
		version = fmt.Sprintf("%s.%d", version, patch)
	}
	return major, minor, patch, version, true
}
