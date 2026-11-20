package workspace

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/egoist/mygo"
)

func hostSession(t *testing.T, h *Service, id, path string) *docSession {
	t.Helper()
	captured := path
	s := &docSession{host: h, id: id, pathFn: func() string { return captured }}
	h.mu.Lock()
	if h.byID == nil {
		h.byID = make(map[string]*docSession)
	}
	h.byID[id] = s
	h.mu.Unlock()
	return s
}

func TestCommitGenerationWriteGuardsLiveSlot(t *testing.T) {
	h := &Service{}
	s := &docSession{host: h, generating: true}
	slot := &generationSlot{path: "/work/a.md"}
	s.genSlot = slot

	if err := h.commitGenerationWrite(s, slot); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Errorf("live empty path = %v, want path-changed", err)
	}

	s.pathFn = func() string { return slot.path }
	if err := h.commitGenerationWrite(s, slot); err != nil {
		t.Errorf("matching path = %v, want nil", err)
	}

	s.pathFn = func() string { return "/elsewhere/a.md" }
	if err := h.commitGenerationWrite(s, slot); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Errorf("moved path = %v, want path-changed", err)
	}
	s.pathFn = func() string { return slot.path }

	slot.abandoned = true
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("abandoned slot = %v, want errAbandoned", err)
	}
	slot.abandoned = false
	s.closed = true
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("closed session = %v, want errAbandoned", err)
	}
	s.closed = false
	other := &generationSlot{path: slot.path}
	s.genSlot = other
	if err := h.commitGenerationWrite(s, slot); err != errAbandoned {
		t.Errorf("stale slot = %v, want errAbandoned", err)
	}
}

func TestRejectPathChangeWhileGenerating(t *testing.T) {
	h := &Service{}
	s := &docSession{host: h, generating: true, genSlot: &generationSlot{path: "/work/a.md"}}
	if err := h.rejectPathChange(s); err == nil || err.Error() != errGenBusy {
		t.Errorf("rejectPathChange = %v, want %q", err, errGenBusy)
	}
	s.generating = false
	if err := h.rejectPathChange(s); err != nil {
		t.Errorf("idle rejectPathChange = %v", err)
	}
}

func TestCancelGenerationMarksAbandonedWithoutClearingIdleSlot(t *testing.T) {
	h := &Service{}
	s := &docSession{host: h}
	h.cancelGeneration(s)

	called := false
	slot := &generationSlot{cancel: func() { called = true }}
	s.genSlot = slot
	s.generating = true
	h.cancelGeneration(s)
	if !slot.abandoned || !called {
		t.Fatalf("cancelGeneration abandoned=%v called=%v", slot.abandoned, called)
	}
	if s.genSlot != slot {
		t.Fatal("cancel must not drop the slot; Generate's defer still owns cleanup")
	}
}

func TestWaitForGenerationsReturnsWhenIdle(t *testing.T) {
	h := &Service{}
	start := time.Now()
	h.WaitForGenerations(time.Second)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("idle WaitForGenerations must return immediately")
	}
}

func TestServiceDomainLookupSeam(t *testing.T) {
	root := t.TempDir()
	canonicalRoot := fsutil.CanonicalPath(root)
	docPath := filepath.Join(canonicalRoot, "paper.md")
	win := &mygo.Window{}
	h := NewService()
	w := &workspace{id: "ws-1", root: canonicalRoot, win: win, tabs: []string{"doc-1"}, active: "doc-1"}
	h.mu.Lock()
	h.byWindow[win] = w
	h.byWorkspace[w.id] = w
	h.workspaceOrder = []string{w.id}
	h.byID["doc-1"] = &docSession{
		host:   h,
		id:     "doc-1",
		win:    win,
		ws:     w,
		pathFn: func() string { return docPath },
	}
	h.mu.Unlock()

	if _, err := h.WindowWorkspace(context.Background(), root); err != errNoWorkspace {
		t.Fatalf("WindowWorkspace without caller = %v, want errNoWorkspace", err)
	}
	ref, err := h.windowWorkspace(win, root)
	if err != nil || ref.Root != canonicalRoot || ref.Window != win {
		t.Fatalf("windowWorkspace = %+v, %v", ref, err)
	}
	if err := h.editableDocument(win, root, "doc-1"); err != nil {
		t.Fatalf("editableDocument = %v, want nil", err)
	}
	if err := h.editableDocument(win, root, "missing"); err == nil {
		t.Fatal("editableDocument accepted missing document")
	}
}
