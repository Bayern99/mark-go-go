package nativegenui

import (
	"context"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

// viewWatcher records the documents the panel watches and stops watching.
type viewWatcher struct{ watched, unwatched []string }

func (v *viewWatcher) watch(_ context.Context, id string) error {
	v.watched = append(v.watched, id)
	return nil
}

func (v *viewWatcher) unwatch(_ context.Context, id string) error {
	v.unwatched = append(v.unwatched, id)
	return nil
}

func TestTurningGenUIOffCancelsTheGenerationAndStopsWatching(t *testing.T) {
	win := &nativeui.Window{}
	watcher := &viewWatcher{}
	p := &panel{win: win, views: map[string]*documentView{}, watch: watcher.watch, unwatch: watcher.unwatch}
	cancelled := 0
	p.views["a"] = &documentView{cancel: func() { cancelled++ }}

	win.SetPreference("mgo.plugin.genui", "off")
	p.changed()

	if cancelled != 1 {
		t.Fatalf("a generation in flight was cancelled %d times, want 1", cancelled)
	}
	if len(watcher.unwatched) != 1 || watcher.unwatched[0] != "a" {
		t.Fatalf("views still watching after GenUI went off = %v", watcher.unwatched)
	}
	if len(p.views) != 0 {
		t.Fatalf("turning GenUI off kept %d document views", len(p.views))
	}
}

func TestGenUIWatchesTheSavedViewAgainWhenItIsTurnedBackOn(t *testing.T) {
	win := &nativeui.Window{}
	watcher := &viewWatcher{}
	p := &panel{win: win, views: map[string]*documentView{}, watch: watcher.watch, unwatch: watcher.unwatch}
	doc := workspace.DocumentInfo{ID: "a"}

	win.SetPreference("mgo.plugin.genui", "off")
	p.changed()
	if v := p.viewFor(doc); v != nil {
		t.Fatal("a document opened a GenUI view while the plugin is off")
	}
	win.SetPreference("mgo.plugin.genui", "on")
	p.changed()

	v := p.viewFor(doc)
	if v == nil {
		t.Fatal("the document did not open a GenUI view again")
	}
	if len(watcher.watched) != 1 || watcher.watched[0] != "a" {
		t.Fatalf("watched views = %v, want the saved view watched once", watcher.watched)
	}
	if v.loaded {
		t.Fatal("the returning view claims a reader before its saved view arrives")
	}
	if again := p.viewFor(doc); again != v || len(watcher.watched) != 1 {
		t.Fatalf("a second draw watched again: %v", watcher.watched)
	}
}
