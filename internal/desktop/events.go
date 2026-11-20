package desktop

import (
	"context"
	"errors"
	"sync"

	"github.com/egoist/mygo"
)

type windowKey struct{}

func WithWindow(ctx context.Context, win *mygo.Window) context.Context {
	return context.WithValue(ctx, windowKey{}, win)
}

func Window(ctx context.Context) *mygo.Window {
	win, _ := ctx.Value(windowKey{}).(*mygo.Window)
	return win
}

type Event[T any] struct {
	mu      sync.RWMutex
	windows map[*mygo.Window]func(T)
}

func (e *Event[T]) Listen(win *mygo.Window, receive func(T)) {
	e.mu.Lock()
	if e.windows == nil {
		e.windows = make(map[*mygo.Window]func(T))
	}
	e.windows[win] = receive
	e.mu.Unlock()
	win.OnClosed(func() {
		e.mu.Lock()
		delete(e.windows, win)
		e.mu.Unlock()
	})
}

func (e *Event[T]) Emit(win *mygo.Window, value T) error {
	e.mu.RLock()
	receive := e.windows[win]
	e.mu.RUnlock()
	if receive == nil {
		return errors.New("window has no event listener")
	}
	win.Update(func() {
		if !win.IsDestroyed() {
			receive(value)
		}
	})
	return nil
}

func (e *Event[T]) Broadcast(value T) error {
	e.mu.RLock()
	windows := make([]*mygo.Window, 0, len(e.windows))
	for win := range e.windows {
		windows = append(windows, win)
	}
	e.mu.RUnlock()
	for _, win := range windows {
		_ = e.Emit(win, value)
	}
	return nil
}
