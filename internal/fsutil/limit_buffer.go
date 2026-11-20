package fsutil

import (
	"strings"
	"sync"
)

type LimitBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func NewLimitBuffer(max int) *LimitBuffer {
	return &LimitBuffer{max: max}
}

func (b *LimitBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.max <= 0 {
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = append(b.buf[:0], b.buf[len(b.buf)-b.max:]...)
	}
	return len(p), nil
}

func (b *LimitBuffer) Tail() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimRight(string(b.buf), "\n")
}
