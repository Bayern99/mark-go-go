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

func TestDirtyEditNotificationNeverAdoptsAnExternalWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.md")
	writeFile(t, path, "# Shared A\n")
	host, s := diskSession(t, path)

	// Another app writes B, and the user types in Mark Go Go before MyGo's
	// watcher catches up: the first keystroke delivers Edited=true with
	// Conflict=false. syncDisk must NEVER adopt B as this session's baseline,
	// or a subsequent Save would silently overwrite B.
	writeFile(t, path, "# Shared B from another application\n")
	if host.syncDisk(s, mygo.DocumentState{Name: "shared.md", Path: path, Edited: true}) {
		t.Fatal("a dirty edit notification adopted an external write as our baseline")
	}
	if !host.changedOnDisk(s) {
		t.Fatal("external write B went unnoticed after a dirty edit notification")
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
	host.syncDisk(s, mygo.DocumentState{Name: "shared.md", Path: path})

	// Another app writes the file, then even if a dirty notification fires
	// before the watcher catches up, changedOnDisk must still flag conflict.
	writeFile(t, path, "# Shared\n\nAnother app's sentence.\n")
	host.syncDisk(s, mygo.DocumentState{Name: "shared.md", Path: path, Edited: true})
	if !host.changedOnDisk(s) {
		t.Fatal("an external edit went unnoticed")
	}
}

func TestExternalDeleteBeforeWatcherPreservesKnownBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vanished.md")
	writeFile(t, path, "# Vanished\n")
	host, s := diskSession(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// Even a clean notification or direct recordDisk call on a missing file
	// must not erase the known baseline, so Save/Close still detect deletion.
	if host.syncDisk(s, mygo.DocumentState{Name: "vanished.md", Path: path}) {
		t.Fatal("missing file cleared the known baseline")
	}
	if !host.changedOnDisk(s) {
		t.Fatal("deleted file reads as unchanged after failed stat")
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
