package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestDocumentInfoReadsOneSessionState(t *testing.T) {
	host := &Service{}
	session := &docSession{host: host, viewMode: "preview"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 2000 {
			host.mu.Lock()
			if i%2 == 0 {
				session.generating = true
				session.viewMode = "markdown"
				session.actionError = "save failed"
				session.restoreError = "could not restore"
			} else {
				session.generating = false
				session.viewMode = "preview"
				session.actionError = ""
				session.restoreError = ""
			}
			host.mu.Unlock()
		}
	}()
	defer func() { <-done }()
	for range 2000 {
		info := session.info()
		if info.Generating {
			if info.ViewMode != "markdown" || info.ActionError != "save failed" || info.RestoreError != "could not restore" {
				t.Fatalf("mixed session state while generating: %+v", info)
			}
		} else if info.ViewMode != "preview" || info.ActionError != "" || info.RestoreError != "" {
			t.Fatalf("mixed session state while idle: %+v", info)
		}
	}
}

func TestSettingsStoreDoesNotHoldLockDuringOnChange(t *testing.T) {
	model := "gemini-4-argon"
	readBack := make(chan Settings, 1)
	var st *settingsStore
	st = &settingsStore{
		dir: t.TempDir(),
		onChange: func(Settings) {
			done := make(chan Settings, 1)
			go func() {
				done <- st.get()
			}()
			select {
			case s := <-done:
				readBack <- s
			case <-time.After(500 * time.Millisecond):
				t.Error("settingsStore.onChange ran while st.mu was still held")
			}
		},
	}
	if _, err := st.apply(SettingsPatch{Model: &model}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-readBack:
		if got.Model != model {
			t.Fatalf("got model %q, want %q", got.Model, model)
		}
	default:
		t.Fatal("onChange did not complete read-back")
	}
}

func TestCloseThenOpenRefreshesNavigationAndSearch(t *testing.T) {
	for _, tc := range []struct {
		name         string
		closedActive bool
	}{
		{name: "active tab", closedActive: true},
		{name: "background tab", closedActive: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			const diskText = "# Reopen target\n\nContent from disk.\n"
			path := writeMD(t, dir, "target.md", diskText)
			writeMD(t, dir, "other.md", "# Other\n")

			host := NewService()
			host.disableWatch = true
			host.settings = &settingsStore{dir: t.TempDir()}
			win := &mygo.Window{}
			w := &workspace{id: "ws-reopen", root: dir, win: win}
			host.byWindow[win] = w
			host.byWorkspace[w.id] = w
			host.workspaceOrder = []string{w.id}
			host.startIndex(w)
			host.waitIndexReady(w, 2*time.Second)
			t.Cleanup(func() {
				host.forgetIndex(w)
				host.CloseAgentCache()
			})
			w.index.mu.Lock()
			w.index.query = "Reopen"
			w.index.mu.Unlock()

			var navEvents []NavSnapshot
			var searchEvents []NavSearchResult
			host.navigationChanged = func(_ *mygo.Window, snapshot NavSnapshot) error {
				navEvents = append(navEvents, snapshot)
				return nil
			}
			host.searchReady = func(_ *mygo.Window, result NavSearchResult) error {
				searchEvents = append(searchEvents, result)
				return nil
			}

			nextID := 0
			host.openDocument = func(path string) (*docSession, error) {
				body, err := os.ReadFile(path)
				if err != nil {
					return nil, err
				}
				nextID++
				s := &docSession{
					host:   host,
					id:     fmt.Sprintf("doc-%d", nextID),
					ws:     w,
					pathFn: func() string { return path },
					textFn: func() string { return string(body) },
				}
				host.mu.Lock()
				host.byID[s.id] = s
				w.tabs = append(w.tabs, s.id)
				w.active = s.id
				host.mu.Unlock()
				host.reindexOpenDocument(s)
				return s, nil
			}

			opened, err := host.Open(context.Background(), path)
			if err != nil {
				t.Fatalf("first Open: %v", err)
			}
			if !tc.closedActive {
				if _, err := host.Open(context.Background(), filepath.Join(dir, "other.md")); err != nil {
					t.Fatalf("Open other document: %v", err)
				}
			}
			closing, err := host.docSession(opened.ID)
			if err != nil {
				t.Fatal(err)
			}
			host.mu.Lock()
			closing.edited = true
			host.mu.Unlock()
			host.reindexOpenDocument(closing)

			entryIs := func(snapshot NavSnapshot, id string, edited bool) bool {
				for _, entry := range snapshot.Entries {
					if entry.Name == "target.md" {
						return entry.DocumentID == id && entry.Edited == edited
					}
				}
				return false
			}
			hitIs := func(result NavSearchResult, id string) bool {
				for _, hit := range result.Hits {
					if hit.Name == "target.md" {
						return hit.DocumentID == id
					}
				}
				return false
			}
			if len(navEvents) == 0 || !entryIs(navEvents[len(navEvents)-1], opened.ID, true) {
				t.Fatalf("open navigation did not contain the dirty document: %+v", navEvents)
			}
			if len(searchEvents) == 0 || !hitIs(searchEvents[len(searchEvents)-1], opened.ID) {
				t.Fatalf("open search did not contain the document id: %+v", searchEvents)
			}

			navBefore, searchBefore := len(navEvents), len(searchEvents)
			host.dropSessionState(closing, true)
			if len(navEvents) != navBefore+1 || len(searchEvents) != searchBefore+1 {
				t.Fatalf("close emitted %d navigation and %d search updates, want one each", len(navEvents)-navBefore, len(searchEvents)-searchBefore)
			}
			if !entryIs(navEvents[len(navEvents)-1], "", false) {
				t.Fatalf("closed navigation retained stale state: %+v", navEvents[len(navEvents)-1].Entries)
			}
			if !hitIs(searchEvents[len(searchEvents)-1], "") {
				t.Fatalf("closed search retained stale document id: %+v", searchEvents[len(searchEvents)-1].Hits)
			}

			reopened, err := host.Open(context.Background(), path)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if reopened.ID == "" || reopened.ID == opened.ID {
				t.Fatalf("reopened id = %q, previous id = %q", reopened.ID, opened.ID)
			}
			if err := host.Activate(context.Background(), reopened.ID); err != nil {
				t.Fatalf("reopened document is not activatable: %v", err)
			}
			session, err := host.docSession(reopened.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got := session.currentText(); got != diskText {
				t.Fatalf("reopened text = %q, want disk text %q", got, diskText)
			}
		})
	}
}

func TestDocumentHostConcurrentDropSessionAndReserveTarget(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.md")
	p2 := filepath.Join(dir, "b.md")
	h := &Service{
		byWindow: map[*mygo.Window]*workspace{},
		byID:     map[string]*docSession{},
		claims:   map[string]string{},
	}
	win := &mygo.Window{}
	w := &workspace{id: "ws-1", root: dir, win: win, tabs: []string{"doc-1", "doc-2"}, active: "doc-1"}
	h.byWindow[win] = w
	h.workspaceOrder = []string{w.id}

	s1 := &docSession{
		host:      h,
		id:        "doc-1",
		win:       win,
		ws:        w,
		pathFn:    func() string { return p1 },
		offReload: func() {},
		offState:  func() {},
		offClosed: func() {},
	}
	s2 := &docSession{
		host:   h,
		id:     "doc-2",
		win:    win,
		ws:     w,
		pathFn: func() string { return p2 },
	}
	h.byID["doc-1"] = s1
	h.byID["doc-2"] = s2

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		h.dropSession(s1)
	}()
	go func() {
		defer wg.Done()
		h.dropSession(s1)
	}()
	go func() {
		defer wg.Done()
		for range 50 {
			_ = s1.info()
			_, _ = h.snapshotWorkspace(win)
		}
	}()
	go func() {
		defer wg.Done()
		for range 20 {
			if rel, err := h.reserveTarget("doc-2", p2, filepath.Join(dir, "c.md")); err == nil && rel != nil {
				rel()
			}
		}
	}()
	wg.Wait()
}
