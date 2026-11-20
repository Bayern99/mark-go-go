package feynman

import (
	"crypto/rand"
	"encoding/hex"
)

func newRandomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
