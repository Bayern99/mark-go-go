package workspace

import (
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
