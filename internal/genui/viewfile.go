package genui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

func ViewPath(docPath string) string {
	if docPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(docPath), "."+filepath.Base(docPath)+".genui.json")
}

func ReadView(path string) ViewSnapshot {
	if path == "" {
		return ViewSnapshot{}
	}
	snap := readViewText(path)
	if fi, err := os.Lstat(PrevViewPath(path)); err == nil && fi.Mode().IsRegular() {
		snap.CanUndo = true
	}
	return snap
}

func readViewText(path string) ViewSnapshot {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ViewSnapshot{}
	}
	if err != nil {
		return ViewSnapshot{Exists: true, Error: err.Error()}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxViewBytes+1))
	if err != nil {
		return ViewSnapshot{Exists: true, Error: err.Error()}
	}
	if len(data) > MaxViewBytes {
		return ViewSnapshot{Exists: true, Error: "the view file is larger than 1 MiB"}
	}
	return ViewSnapshot{Exists: true, Text: string(data)}
}

type ViewFollower struct {
	mu        sync.Mutex
	following bool
	viewPath  string
	stopView  func()
	viewGen   int

	emitMu sync.Mutex
	emit   func(ViewSnapshot)
}

func NewViewFollower(emit func(ViewSnapshot)) *ViewFollower {
	return &ViewFollower{emit: emit}
}

func (f *ViewFollower) Watch(path string) error {
	f.mu.Lock()
	f.following = true
	gen := f.retargetLocked(path)
	f.mu.Unlock()
	return f.followView(path, gen)
}

func (f *ViewFollower) Unwatch() {
	f.mu.Lock()
	f.following = false
	f.retargetLocked("")
	f.mu.Unlock()
}

func (f *ViewFollower) RetargetIfWatching(path string) {
	f.mu.Lock()
	if !f.following || path == f.viewPath {
		f.mu.Unlock()
		return
	}
	gen := f.retargetLocked(path)
	f.mu.Unlock()
	go func() {
		if err := f.followView(path, gen); err != nil {
			log.Print(err)
		}
	}()
}

func (f *ViewFollower) BeginForTest(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.following = true
	return f.retargetLocked(path)
}

func (f *ViewFollower) RetargetForTest(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.retargetLocked(path)
}

func (f *ViewFollower) followView(path string, gen int) error {
	return f.FollowWith(path, gen, f.startViewWatch)
}

func (f *ViewFollower) FollowWith(path string, gen int, start func(string, int) error) error {
	if !f.EmitSnapshot(path, gen) {
		return nil
	}
	if err := start(path, gen); err != nil {
		return err
	}
	f.EmitSnapshot(path, gen)
	return nil
}

func (f *ViewFollower) retargetLocked(path string) int {
	if f.stopView != nil {
		f.stopView()
		f.stopView = nil
	}
	f.viewGen++
	f.viewPath = path
	return f.viewGen
}

func (f *ViewFollower) currentLocked(gen int) bool {
	return f.following && f.viewGen == gen
}

func (f *ViewFollower) startViewWatch(path string, gen int) error {
	if path == "" {
		return nil
	}
	name, prev := filepath.Base(path), filepath.Base(PrevViewPath(path))
	ctx, cancel := context.WithCancel(context.Background())
	w, err := mygo.WatchFiles(ctx, filepath.Dir(path), mygo.FileWatchOptions{
		Ignore: func(p string, _ bool) bool { return p != name && p != prev },
	})
	if err != nil {
		cancel()
		f.mu.Lock()
		if f.currentLocked(gen) {
			f.viewPath = ""
		}
		f.mu.Unlock()
		return fmt.Errorf("markgogo: follow the view file: %w", err)
	}

	f.mu.Lock()
	if !f.currentLocked(gen) {
		f.mu.Unlock()
		cancel()
		return nil
	}
	f.stopView = cancel
	f.mu.Unlock()

	go func() {
		for {
			_, err := w.Next(ctx)
			if !f.EmitSnapshot(path, gen) || err != nil {
				return
			}
		}
	}()
	return nil
}

func (f *ViewFollower) EmitSnapshot(path string, gen int) bool {
	f.emitMu.Lock()
	defer f.emitMu.Unlock()
	snap := ReadView(path)
	f.mu.Lock()
	if !f.currentLocked(gen) {
		f.mu.Unlock()
		return false
	}
	emit := f.emit
	f.mu.Unlock()
	if emit != nil {
		emit(snap)
	}
	return true
}
