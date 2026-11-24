package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// A session's disk tracking is what tells Save and Close whether the file
// changed under unsaved edits, and it must not mistake the document's own
// autosave for another app's write. A session cannot hold a real
// *mygo.Document in a unit test — mygo creates its window on the app's main
// thread, which a go test process does not run — so these tests drive the
// session's own tracking the way mygo's writes and state notifications reach
// it, on real files.

func diskSession(t *testing.T, path string) (*Service, *docSession) {
	t.Helper()
	host := &Service{}
	s := &docSession{host: host, id: "doc", pathFn: func() string { return path }}
	host.recordDisk(s)
	return host, s
}

// writeFile is a write of the session itself (save or autosave) or of another
// app, as the file ends up on disk. Each write is larger or smaller than the
// one before it, so its stamp differs even where the clock is coarse.

func TestAutosaveDuringContinuousTypingIsNotAConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	writeFile(t, path, "# Notes\n")
	host, s := diskSession(t, path)

	// The user keeps typing while mygo's autosave writes the document. The
	// notification that follows the write still reports the document as
	// edited, which is exactly the case that used to leave the stamp behind.
	writeFile(t, path, "# Notes\n\nA paragraph the autosave wrote.\n")
	edited := mygo.DocumentState{Name: "notes.md", Path: path, Edited: true}
	if !host.syncDisk(s, edited) {
		t.Fatal("the notification after an autosave did not refresh the disk stamp")
	}
	if host.changedOnDisk(s) {
		t.Fatal("the session's own autosave reads as a change made on disk")
	}

	// Typing on, and another autosave landing on top of it, keep the stamp
	// level: neither the write nor the edits between writes look external.
	writeFile(t, path, "# Notes\n\nA paragraph the autosave wrote, and more text.\n")
	if !host.syncDisk(s, edited) {
		t.Fatal("the second autosave did not refresh the disk stamp")
	}
	if host.changedOnDisk(s) {
		t.Fatal("a second autosave reads as a change made on disk")
	}
}

func TestCleanWriteAfterEditingSettlesTheStamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.md")
	writeFile(t, path, "# Draft\n")
	host, s := diskSession(t, path)

	writeFile(t, path, "# Draft\n\nWriting.\n")
	if !host.syncDisk(s, mygo.DocumentState{Name: "draft.md", Path: path}) {
		t.Fatal("a write that leaves the document clean did not refresh the stamp")
	}
	if host.changedOnDisk(s) {
		t.Fatal("the session's own save reads as a change made on disk")
	}
}

func TestExternalEditIsStillAConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.md")
	writeFile(t, path, "# Shared\n")
	host, s := diskSession(t, path)

	writeFile(t, path, "# Shared\n\nOur own sentence.\n")
	host.syncDisk(s, mygo.DocumentState{Name: "shared.md", Path: path, Edited: true})

	// Between two of the session's own notifications, another app writes the
	// file: the stamp no longer matches and the session must say so.
	writeFile(t, path, "# Shared\n\nAnother app's sentence.\n")
	if !host.changedOnDisk(s) {
		t.Fatal("an external edit went unnoticed")
	}
}

func TestConflictNotificationKeepsTheForeignFileOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conflict.md")
	writeFile(t, path, "# Conflict\n")
	host, s := diskSession(t, path)

	writeFile(t, path, "# Conflict\n\nAnother app's sentence.\n")
	conflict := mygo.DocumentState{Name: "conflict.md", Path: path, Edited: true, Conflict: true}
	if host.syncDisk(s, conflict) {
		t.Fatal("a conflicted notification adopted the foreign file")
	}
	if !host.changedOnDisk(s) {
		t.Fatal("a conflicted notification hid the external change")
	}
}

func TestDeletedFileIsNeverAdoptedAsTheBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.md")
	writeFile(t, path, "# Gone\n")
	host, s := diskSession(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if host.syncDisk(s, mygo.DocumentState{Name: "gone.md", Path: path, Edited: true, Deleted: true}) {
		t.Fatal("a deleted file was recorded as the session's baseline")
	}
	if !host.changedOnDisk(s) {
		t.Fatal("a deleted file reads as unchanged")
	}
}
