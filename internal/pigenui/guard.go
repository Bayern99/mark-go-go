package pigenui

import (
	"embed"
	"errors"
	"os"
	"path/filepath"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

// guardFiles is the guard extension that ships with this app. It is copied out
// per turn instead of being loaded from the user's Pi folders, so a Pi
// installation's own extensions, packages or settings can never replace it.
//
//go:embed guard/guard.js guard/policy.js
var guardFiles embed.FS

var guardNames = []string{"guard.js", "policy.js"}

// writeGuard copies the guard extension into a folder of its own and returns
// its entry point. The folder sits outside the turn's folder, so the agent can
// neither read nor overwrite the code that constrains it.
func writeGuard() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "markgogo-pi-guard-")
	if err != nil {
		return "", func() {}, guardWriteFailure(err)
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for _, name := range guardNames {
		data, readErr := guardFiles.ReadFile("guard/" + name)
		if readErr != nil {
			_ = os.RemoveAll(dir)
			return "", func() {}, guardWriteFailure(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dir, name), data, 0o600); writeErr != nil {
			_ = os.RemoveAll(dir)
			return "", func() {}, guardWriteFailure(writeErr)
		}
	}
	return filepath.Join(dir, "guard.js"), func() { _ = os.RemoveAll(dir) }, nil
}

func guardWriteFailure(err error) *genui.GenError {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
		return genui.NewFailure(genui.FailAgentError,
			"The guard that restricts Pi's file access could not be written, so nothing was generated.",
			"Free space or permissions in the temporary folder, then try again.")
	}
	return genui.NewFailure(genui.FailAgentError,
		"The app's own guard extension is missing, so nothing was generated.",
		"Reinstall Mark Go Go.")
}
