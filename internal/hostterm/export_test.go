package hostterm

import (
	"context"
	"os"
)

// SetCreateTempForTest makes the macOS script land in a test's folder.
func SetCreateTempForTest(fn func(pattern string) (*os.File, error)) func() {
	previous := createTemp
	createTemp = fn
	return func() { createTemp = previous }
}

// OpenForTest runs Open for another platform with an injected environment.
func OpenForTest(ctx context.Context, goos string, getenv func(string) string, commandLine string, Command Command) error {
	return open(ctx, goos, getenv, commandLine, Command)
}
