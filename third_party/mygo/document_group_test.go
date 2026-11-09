package mygo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func recoveredDocument(t *testing.T, id string) (*docBase, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, id)
	if err := os.WriteFile(path, []byte("unsaved document"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovery := &recoveryStore{
		dir:     dir,
		entries: map[string]recoveryEntry{id: {ID: id}},
	}
	doc := &docBase{
		id:       id,
		kind:     &docKind{recovery: recovery},
		edited:   true,
		backedUp: 1,
	}
	return doc, path
}

func TestGroupCloseDiscardsOnlyAfterEveryDocumentConfirms(t *testing.T) {
	a, recoveryA := recoveredDocument(t, "a")
	b, _ := recoveredDocument(t, "b")
	aAsks := 0
	confirm := func(doc *docBase) (bool, error) {
		if doc == a {
			aAsks++
			return true, nil
		}
		return false, ErrCanceled
	}

	discarded, err := confirmCloseGroup([]*docBase{a, b}, confirm)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("first group close = %v, want cancellation", err)
	}
	if len(discarded) != 0 {
		t.Fatalf("discard decisions after cancellation = %v, want none", discarded)
	}
	if a.discarded || !a.edited || a.backedUp != 1 {
		t.Fatalf("A changed after the group canceled: discarded=%v edited=%v backedUp=%d", a.discarded, a.edited, a.backedUp)
	}
	if _, err := os.Stat(recoveryA); err != nil {
		t.Fatalf("A's recovery copy after cancellation: %v", err)
	}

	discarded, err = confirmCloseGroup([]*docBase{a, b}, func(doc *docBase) (bool, error) {
		if doc == a {
			aAsks++
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("second group close = %v", err)
	}
	if aAsks != 2 {
		t.Fatalf("A was confirmed %d times, want it asked again after cancellation", aAsks)
	}
	if len(discarded) != 1 || discarded[0] != a {
		t.Fatalf("second close discard decisions = %v, want A", discarded)
	}
	if a.discarded {
		t.Fatal("the group must leave decisions provisional until it commits the close")
	}
	if _, err := os.Stat(recoveryA); err != nil {
		t.Fatalf("A's recovery copy before committed close: %v", err)
	}
	a.discard()
	if _, err := os.Stat(recoveryA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("A's recovery copy after committed discard: stat error = %v, want not-exist", err)
	}
}

func TestGroupCloseConfirmationDoesNotBlockPreparedQuit(t *testing.T) {
	g := &docGroup{closing: true}
	closeWindow := &CloseEvent{}
	g.windowClosing(closeWindow)
	if !closeWindow.DefaultPrevented() {
		t.Fatal("a duplicate window close must wait for its confirmation")
	}

	App.quitting = true
	t.Cleanup(func() { App.quitting = false })
	quitClose := &CloseEvent{}
	g.windowClosing(quitClose)
	if quitClose.DefaultPrevented() {
		t.Fatal("an already prepared application quit must be able to close the window")
	}
}
