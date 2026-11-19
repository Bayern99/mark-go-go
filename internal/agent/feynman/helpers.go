package feynman

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

func findExecutable(env []string, name string) string {
	path, ok := fsutil.ResolveExecutable(name, env, nil)
	if !ok {
		return ""
	}
	return path
}

func newRandomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
