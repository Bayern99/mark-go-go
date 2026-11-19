package genui

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type followProbe struct {
	f    *ViewFollower
	got  chan ViewSnapshot
	sent atomic.Int32
}

func newFollowProbe() *followProbe {
	p := &followProbe{
		got: make(chan ViewSnapshot, 8),
	}
	p.f = NewViewFollower(func(snap ViewSnapshot) {
		p.sent.Add(1)
		p.got <- snap
	})
	return p
}

func (p *followProbe) begin(path string) int {
	return p.f.BeginForTest(path)
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
		done <- p.f.FollowWith(path, gen, p.startAfterFirst(t, release))
	}()

	first := p.recv(t)
	if !first.Exists || first.Text != `{"version":2}` {
		t.Fatalf("first snapshot = %+v, want the view file as it was", first)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("FollowWith: %v", err)
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
		done <- p.f.FollowWith(oldPath, oldGen, func(string, int) error {
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
	newGen := p.f.RetargetForTest(newPath)
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("stale FollowWith: %v", err)
	}
	p.noMore(t)

	if err := p.f.FollowWith(newPath, newGen, func(string, int) error { return nil }); err != nil {
		t.Fatalf("new FollowWith: %v", err)
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
		done <- p.f.FollowWith(path, gen, func(string, int) error {
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
	p.f.Unwatch()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("FollowWith after unwatch: %v", err)
	}
	p.noMore(t)

	if err := os.WriteFile(path, []byte("after unwatch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p.f.EmitSnapshot(path, gen) {
		t.Fatal("EmitSnapshot sent a snapshot after Unwatch")
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
	err := p.f.FollowWith(path, gen, func(string, int) error {
		if p.sent.Load() < 1 {
			t.Error("watch started before the first snapshot")
		}
		return watchErr
	})
	if !errors.Is(err, watchErr) {
		t.Fatalf("FollowWith error = %v, want %v", err, watchErr)
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
	if err := p.f.FollowWith("", gen, func(string, int) error {
		started = true
		return nil
	}); err != nil {
		t.Fatalf("FollowWith untitled: %v", err)
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
	got := ViewPath("/tmp/report.md")
	want := filepath.Join("/tmp", ".report.md.genui.json")
	if got != want {
		t.Fatalf("ViewPath = %q, want %q", got, want)
	}
	if ViewPath("") != "" {
		t.Fatalf("untitled ViewPath = %q, want empty", ViewPath(""))
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
		done <- p.f.FollowWith(path, gen, func(string, int) error {
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
		t.Fatalf("FollowWith: %v", err)
	}
	second := p.recv(t)
	p.noMore(t)
	return []ViewSnapshot{first, second}
}

func TestEmitViewDoesNotHoldFollowerLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var follower *ViewFollower
	unwatched := make(chan struct{})
	follower = NewViewFollower(func(ViewSnapshot) {
		go func() {
			follower.Unwatch()
			close(unwatched)
		}()
		select {
		case <-unwatched:
		case <-time.After(500 * time.Millisecond):
			t.Error("ViewFollower.EmitSnapshot ran while follower.mu was still held")
		}
	})
	gen := follower.BeginForTest(path)
	if !follower.EmitSnapshot(path, gen) {
		t.Fatal("EmitSnapshot returned false")
	}
}

func TestUndoView(t *testing.T) {
	dir := t.TempDir()
	viewPath := filepath.Join(dir, ".notes.md.genui.json")
	if err := os.WriteFile(viewPath, []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PrevViewPath(viewPath), []byte("the view before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UndoView(viewPath); err != nil {
		t.Fatalf("UndoView: %v", err)
	}
	snap := ReadView(viewPath)
	if snap.Text != "the view before" || snap.CanUndo {
		t.Errorf("after undo = %+v, want the kept view without undo", snap)
	}
	if err := UndoView(viewPath); err == nil || err.Error() != "There is no earlier view to go back to." {
		t.Errorf("second undo = %v, want refused", err)
	}
}

func TestAgentToolAllowed(t *testing.T) {
	dir := t.TempDir()
	if !AgentToolAllowed(dir, "think", nil) {
		t.Fatal("think should be allowed")
	}
	if !AgentToolAllowed(dir, "read", []string{"source.md", "view.json"}) {
		t.Fatal("read of source.md and view.json should be allowed")
	}
	if AgentToolAllowed(dir, "read", []string{"../secret.md"}) {
		t.Fatal("read outside working dir should be rejected")
	}
	if !AgentToolAllowed(dir, "edit", []string{"out.json"}) {
		t.Fatal("edit of out.json should be allowed")
	}
	if AgentToolAllowed(dir, "edit", []string{"source.md"}) {
		t.Fatal("edit of source.md should be rejected")
	}
}
