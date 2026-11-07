package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// followView is WatchView's read/watch order: a first snapshot as soon as
// follow starts, then the watch, then a catch-up snapshot. Tests drive that
// order on a session without MyGo windows. They do not time WatchFiles, and
// they do not treat a directory scan as a measured cause of the #14 wait.
//
// Native large-directory smoke (verification owner, real macOS app):
//  1. Open a Markdown file that already has a sibling view file in a large
//     folder. Issue #14 used ~/Downloads/SuperCollider 深度分析报告.md
//     (~516 top-level entries, ~16k files beneath).
//  2. Switch to GenUI for the first time.
//  3. The existing view, or an accurate loading state, must appear without a
//     long unexplained blank. Time-to-first-snapshot must not wait on
//     unrelated names in that folder.
//  4. While still on GenUI, replace, then delete, the view file; the pane
//     must follow. Rename the document; the pane must follow the new sibling.
//  5. Leave GenUI; further writes must not be delivered.

type followProbe struct {
	s    *docSession
	got  chan ViewSnapshot
	sent atomic.Int32
}

func newFollowProbe() *followProbe {
	p := &followProbe{
		got: make(chan ViewSnapshot, 8),
		s:   &docSession{host: &documentHost{}},
	}
	p.s.emit = func(snap ViewSnapshot) {
		p.sent.Add(1)
		p.got <- snap
	}
	return p
}

func (p *followProbe) begin(path string) int {
	p.s.host.mu.Lock()
	defer p.s.host.mu.Unlock()
	p.s.following = true
	return p.s.retargetLocked(path)
}

func (p *followProbe) recv(t *testing.T) ViewSnapshot {
	t.Helper()
	select {
	case snap := <-p.got:
		return snap
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a view snapshot")
		panic("unreachable")
	}
}

func (p *followProbe) noMore(t *testing.T) {
	t.Helper()
	select {
	case snap := <-p.got:
		t.Fatalf("unexpected extra snapshot: %+v", snap)
	default:
	}
}

// startAfterFirst fails if WatchFiles-equivalent work begins before the
// first snapshot, then blocks until release is closed.
func (p *followProbe) startAfterFirst(t *testing.T, release <-chan struct{}) func(string, int) error {
	t.Helper()
	return func(string, int) error {
		if p.sent.Load() < 1 {
			t.Error("watch started before the first snapshot")
			return errors.New("watch started before the first snapshot")
		}
		<-release
		return nil
	}
}

func TestFollowViewSendsBeforeWatchStarts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	p := newFollowProbe()
	gen := p.begin(path)
	done := make(chan error, 1)
	go func() {
		done <- p.s.followViewWith(path, gen, p.startAfterFirst(t, release))
	}()

	first := p.recv(t)
	if !first.Exists || first.Text != `{"version":2}` {
		t.Fatalf("first snapshot = %+v, want the view file as it was", first)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("followView: %v", err)
	}
	catch := p.recv(t)
	if !catch.Exists || catch.Text != `{"version":2}` {
		t.Fatalf("catch-up snapshot = %+v, want the same file after the watch", catch)
	}
	p.noMore(t)
}

func TestFollowViewCatchUpSeesWriteDuringWatchStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	snaps := followWithMidStart(t, path, func() {
		if err := os.WriteFile(path, []byte("during"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if snaps[0].Text != "before" || snaps[1].Text != "during" {
		t.Fatalf("snapshots = %+v, want before then the write during watch start", snaps)
	}
}

func TestFollowViewCatchUpSeesReplaceAndDelete(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".notes.md.genui.json")
		if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		snaps := followWithMidStart(t, path, func() {
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, []byte("new"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(tmp, path); err != nil {
				t.Fatal(err)
			}
		})
		if snaps[0].Text != "old" || snaps[1].Text != "new" {
			t.Fatalf("snapshots = %+v, want old then replaced new", snaps)
		}
	})
	t.Run("delete", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".notes.md.genui.json")
		if err := os.WriteFile(path, []byte("gone soon"), 0o600); err != nil {
			t.Fatal(err)
		}
		snaps := followWithMidStart(t, path, func() {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
		if !snaps[0].Exists || snaps[1].Exists || snaps[1].Error != "" {
			t.Fatalf("snapshots = %+v, want existing then a missing file", snaps)
		}
	})
}

func TestFollowViewRetargetDropsStaleGeneration(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, ".old.md.genui.json")
	newPath := filepath.Join(dir, ".new.md.genui.json")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	started := make(chan struct{})
	p := newFollowProbe()
	oldGen := p.begin(oldPath)
	done := make(chan error, 1)
	go func() {
		done <- p.s.followViewWith(oldPath, oldGen, func(string, int) error {
			if p.sent.Load() < 1 {
				t.Error("watch started before the first snapshot")
			}
			close(started)
			<-release
			return nil
		})
	}()

	first := p.recv(t)
	if first.Text != "old" {
		t.Fatalf("first snapshot = %q, want old", first.Text)
	}
	<-started
	p.s.host.mu.Lock()
	newGen := p.s.retargetLocked(newPath)
	p.s.host.mu.Unlock()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("stale followView: %v", err)
	}
	p.noMore(t)

	if err := p.s.followViewWith(newPath, newGen, func(string, int) error { return nil }); err != nil {
		t.Fatalf("new followView: %v", err)
	}
	a := p.recv(t)
	b := p.recv(t)
	if a.Text != "new" || b.Text != "new" {
		t.Fatalf("new snapshots = %+v %+v, want the retargeted file twice", a, b)
	}
	p.noMore(t)
}

func TestFollowViewUnwatchStopsEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(path, []byte("alive"), 0o600); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	started := make(chan struct{})
	p := newFollowProbe()
	gen := p.begin(path)
	done := make(chan error, 1)
	go func() {
		done <- p.s.followViewWith(path, gen, func(string, int) error {
			if p.sent.Load() < 1 {
				t.Error("watch started before the first snapshot")
			}
			close(started)
			<-release
			return nil
		})
	}()

	first := p.recv(t)
	if first.Text != "alive" {
		t.Fatalf("first snapshot = %q", first.Text)
	}
	<-started
	p.s.unfollowView()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("followView after unwatch: %v", err)
	}
	p.noMore(t)

	if err := os.WriteFile(path, []byte("after unwatch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p.s.emitView(path, gen) {
		t.Fatal("emitView sent a snapshot after UnwatchView")
	}
	p.noMore(t)
}

func TestFollowViewWatchErrorAfterFirstSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	watchErr := errors.New("cannot watch")
	p := newFollowProbe()
	gen := p.begin(path)
	err := p.s.followViewWith(path, gen, func(string, int) error {
		if p.sent.Load() < 1 {
			t.Error("watch started before the first snapshot")
		}
		return watchErr
	})
	if !errors.Is(err, watchErr) {
		t.Fatalf("followView error = %v, want %v", err, watchErr)
	}
	first := p.recv(t)
	if first.Text != "ok" {
		t.Fatalf("first snapshot = %q, want the file before the watch failed", first.Text)
	}
	p.noMore(t)
}

func TestFollowViewUntitledHasNoFile(t *testing.T) {
	p := newFollowProbe()
	gen := p.begin("")
	started := false
	if err := p.s.followViewWith("", gen, func(string, int) error {
		started = true
		return nil
	}); err != nil {
		t.Fatalf("followView untitled: %v", err)
	}
	if !started {
		t.Fatal("start was not called for the untitled catch-up path")
	}
	first := p.recv(t)
	second := p.recv(t)
	if first.Exists || second.Exists || first.Error != "" || second.Error != "" {
		t.Fatalf("untitled snapshots = %+v %+v, want two empty views", first, second)
	}
}

func TestViewPathSibling(t *testing.T) {
	got := viewPath("/tmp/report.md")
	want := filepath.Join("/tmp", ".report.md.genui.json")
	if got != want {
		t.Fatalf("viewPath = %q, want %q", got, want)
	}
	if viewPath("") != "" {
		t.Fatalf("untitled viewPath = %q, want empty", viewPath(""))
	}
}

func followWithMidStart(t *testing.T, path string, during func()) []ViewSnapshot {
	t.Helper()
	release := make(chan struct{})
	started := make(chan struct{})
	p := newFollowProbe()
	gen := p.begin(path)
	done := make(chan error, 1)
	go func() {
		done <- p.s.followViewWith(path, gen, func(string, int) error {
			if p.sent.Load() < 1 {
				t.Error("watch started before the first snapshot")
			}
			close(started)
			<-release
			return nil
		})
	}()
	first := p.recv(t)
	<-started
	during()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("followView: %v", err)
	}
	second := p.recv(t)
	p.noMore(t)
	return []ViewSnapshot{first, second}
}
