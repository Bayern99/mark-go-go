package feynman

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/review"
)

func TestResearchSessionsJSONLAndTreeForkCloneRenameDelete(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	work := review.WorkDir(data, root)
	sessionsDir := filepath.Join(home, ".feynman", "sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeTestSessionJSONL(t, filepath.Join(sessionsDir, "older.jsonl"), []any{
		map[string]any{
			"type":          "session",
			"version":       3,
			"id":            "older",
			"timestamp":     "2026-11-18T10:00:00Z",
			"cwd":           work,
			"parentSession": filepath.Join(sessionsDir, "root-parent.jsonl"),
		},
		map[string]any{
			"type":      "message",
			"id":        "m1",
			"parentId":  nil,
			"timestamp": "2026-11-18T10:00:01Z",
			"message": map[string]any{
				"role":    "user",
				"content": "Investigate reverberation time in lecture halls",
			},
		},
		map[string]any{
			"type":      "message",
			"id":        "m2",
			"parentId":  "m1",
			"timestamp": "2026-11-18T10:00:02Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "Restored older"}},
			},
		},
		map[string]any{
			"type":      "session_info",
			"id":        "si-1",
			"parentId":  "m2",
			"timestamp": "2026-11-18T10:01:00Z",
			"name":      "Draft name",
		},
		map[string]any{
			"type":      "session_info",
			"id":        "si-2",
			"parentId":  "si-1",
			"timestamp": "2026-11-18T10:02:00Z",
			"name":      "Earlier research",
		},
	})

	writeTestSessionJSONL(t, filepath.Join(sessionsDir, "preview-only.jsonl"), []any{
		map[string]any{
			"type":      "session",
			"version":   3,
			"id":        "preview-only",
			"timestamp": "2026-11-17T08:00:00Z",
			"cwd":       work,
		},
		map[string]any{
			"type":      "message",
			"id":        "p1",
			"parentId":  nil,
			"timestamp": "2026-11-17T08:00:01Z",
			"message": map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": "Compare Room Impulse Response datasets"}},
			},
		},
	})

	writeTestSessionJSONL(t, filepath.Join(sessionsDir, "foreign.jsonl"), []any{
		map[string]any{
			"type":      "session",
			"version":   3,
			"id":        "foreign",
			"timestamp": "2026-11-19T08:00:00Z",
			"cwd":       filepath.Dir(work),
		},
		map[string]any{
			"type":      "session_info",
			"id":        "f1",
			"parentId":  nil,
			"timestamp": "2026-11-19T08:00:01Z",
			"name":      "Outside workspace",
		},
	})

	writeTestSessionJSONL(t, filepath.Join(sessionsDir, "broken.jsonl"), []any{
		map[string]any{
			"type":      "session",
			"version":   3,
			"id":        "broken",
			"timestamp": "2026-11-16T08:00:00Z",
			"cwd":       work,
		},
	})

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)

	events := make(chan ResearchUpdate, 128)
	connect := func() {
		t.Helper()
		if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
			t.Fatal(err)
		}
	}
	connect()

	var early string
	for len(events) > 0 {
		e := <-events
		if e.Kind == "assistant" {
			early += e.Text
		}
	}
	if early != "Created first" {
		t.Fatalf("initial history = %q, want %q", early, "Created first")
	}

	ctx := context.Background()
	records, err := h.Host().Sessions(ctx, root)
	if err != nil {
		t.Fatalf("Sessions failed: %v", err)
	}
	byID := map[string]ResearchConversation{}
	for _, r := range records {
		if r.ID == "foreign" {
			t.Fatalf("foreign session leaked into workspace Sessions: %+v", records)
		}
		byID[r.ID] = r
	}
	if !byID["first"].Current {
		t.Fatalf("expected 'first' to be current: %+v", byID["first"])
	}
	if byID["older"].Title != "Earlier research" || byID["older"].Name != "Earlier research" ||
		byID["older"].Preview != "Investigate reverberation time in lecture halls" ||
		!strings.HasSuffix(byID["older"].ParentSession, "root-parent.jsonl") {
		t.Fatalf("older session header/info/preview mismatch: %+v", byID["older"])
	}
	if byID["preview-only"].Title != "Compare Room Impulse Response datasets" || byID["preview-only"].Preview != "Compare Room Impulse Response datasets" {
		t.Fatalf("preview-only session title mismatch: %+v", byID["preview-only"])
	}

	if err := h.Host().SelectSession(ctx, root, "foreign"); err == nil {
		t.Fatal("opened a session from a different workspace")
	}
	if err := h.Host().DeleteSession(ctx, root, "foreign"); err == nil {
		t.Fatal("deleted a session from a different workspace")
	}
	if _, err := os.Stat(filepath.Join(sessionsDir, "foreign.jsonl")); err != nil {
		t.Fatal("foreign.jsonl should still exist on disk")
	}
	if err := h.Host().DeleteSession(ctx, root, "first"); err == nil {
		t.Fatal("deleted active session")
	}

	if err := h.Host().SetSessionName(ctx, root, "Acoustic Baseline"); err != nil {
		t.Fatalf("SetSessionName failed: %v", err)
	}
	sessionEvt := waitUpdate(t, events, "session")
	if sessionEvt.Text != "Acoustic Baseline" {
		t.Fatalf("expected session rename event 'Acoustic Baseline', got %q", sessionEvt.Text)
	}

	tree, err := h.Host().Tree(ctx, root)
	if err != nil || tree.LeafID != "entry-reply" || len(tree.Tree) != 1 || tree.Tree[0].Entry.ID != "entry-init" || len(tree.Tree[0].Children) != 1 {
		t.Fatalf("Tree mismatch: %+v %v", tree, err)
	}
	forkMsgs, err := h.Host().ForkMessages(ctx, root)
	if err != nil || len(forkMsgs) != 2 || forkMsgs[0].EntryID != "entry-init" {
		t.Fatalf("ForkMessages mismatch: %+v %v", forkMsgs, err)
	}

	forkRes, err := h.Host().Fork(ctx, root, "entry-init")
	if err != nil || forkRes.Cancelled || forkRes.Text != "First research prompt" || !strings.HasPrefix(forkRes.SessionID, "forked-") {
		t.Fatalf("Fork mismatch: %+v %v", forkRes, err)
	}

	cloned, err := h.Host().Clone(ctx, root)
	if err != nil || !cloned {
		t.Fatalf("Clone mismatch: cloned=%v err=%v", cloned, err)
	}

	drainEvents(events)
	if err := h.Host().SelectSession(ctx, root, "older"); err != nil {
		t.Fatalf("SelectSession(older) failed: %v", err)
	}
	var history []string
	for len(events) > 0 {
		e := <-events
		if e.Kind == "reset" {
			history = nil
		}
		if e.Kind == "assistant" {
			history = append(history, e.Text)
		}
	}
	if len(history) != 1 || history[0] != "Restored older" {
		t.Fatalf("unexpected replay after switching to older: %v", history)
	}
	record := filepath.Join(filepath.Dir(work), "session")
	id, err := os.ReadFile(record)
	if err != nil || string(id) != "older" {
		t.Fatalf("switch not persisted: %q %v", id, err)
	}

	if err := h.Host().NewSession(ctx, root, ""); err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	id, _ = os.ReadFile(record)
	currentAfterNew := string(id)
	if currentAfterNew == "" || currentAfterNew == "older" {
		t.Fatalf("new session id not persisted: %q", currentAfterNew)
	}

	if err := h.Host().DeleteSession(ctx, root, "older"); err != nil {
		t.Fatalf("DeleteSession(older) failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sessionsDir, "older.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("older.jsonl was not deleted from disk: %v", err)
	}
	rows, err := h.Host().Sessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == "older" {
			t.Fatal("deleted session remains in Sessions list")
		}
	}

	drainEvents(events)
	if err := h.Host().SelectSession(ctx, root, "broken"); err == nil {
		t.Fatal("unavailable session switched successfully")
	}
	id, _ = os.ReadFile(record)
	if string(id) != currentAfterNew {
		t.Fatalf("failed switch replaced durable session selection: got %q, want %q", id, currentAfterNew)
	}

	connect()
	drainEvents(events)

	if err := os.WriteFile(filepath.Join(work, "pause-load"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Host().SelectSession(ctx, root, "broken") }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(work, "loading")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture never received switch_session for broken")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := h.Host().Prompt(ctx, root, "overlap", "", ""); err == nil {
		t.Fatal("prompt accepted during session switch")
	}
	if err := h.Host().Disconnect(ctx, root); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disconnected switch reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("disconnected switch did not drain")
	}
	id, err = os.ReadFile(record)
	if err != nil || string(id) != currentAfterNew {
		t.Fatalf("lost previous selection: %q %v", id, err)
	}

	if err := os.WriteFile(record, []byte("removed-session-id"), 0600); err != nil {
		t.Fatal(err)
	}
	connect()
	notice := waitUpdate(t, events, "notice")
	if !strings.Contains(notice.Text, "no longer available") {
		t.Fatalf("expected recovery notice, got %q", notice.Text)
	}
}

func writeTestSessionJSONL(t *testing.T, path string, entries []any) {
	t.Helper()
	var buf strings.Builder
	for _, entry := range entries {
		b, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatal(err)
	}
}
