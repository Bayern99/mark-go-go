package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// decisionFixture is a service whose sessions point at real files on disk, so
// every close and move decision is judged against bytes the test wrote.
type decisionFixture struct {
	host *Service
	dir  string
}

func newDecisionFixture(t *testing.T, autosave mygo.Autosave) *decisionFixture {
	t.Helper()
	h := NewService()
	h.disableWatch = true
	h.settings = &settingsStore{dir: t.TempDir()}
	h.SetAutosave(autosave)
	t.Cleanup(h.CloseAgentCache)
	return &decisionFixture{host: h, dir: t.TempDir()}
}

// writeFile puts body at dir/name with mode and returns the path.
func (f *decisionFixture) writeFile(t *testing.T, name, body string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// session records a tab for the file, as an open document would be.
func (f *decisionFixture) session(t *testing.T, id, path string, edited bool) *docSession {
	t.Helper()
	s := &docSession{host: f.host, id: id, pathFn: func() string { return path }, edited: edited}
	f.host.mu.Lock()
	f.host.byID[id] = s
	f.host.mu.Unlock()
	if path != "" {
		f.host.recordDisk(s)
	}
	return s
}

// edited reports the session's own dirty flag, which is what a session
// without an open document publishes.
func (f *decisionFixture) edited(id string) bool {
	s, err := f.host.docSession(id)
	if err != nil {
		return false
	}
	return s.documentState().Edited
}

func ids(docs []DocumentInfo) []string {
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.ID)
	}
	return out
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func readDecisionFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestClosePreflightClassifiesRealDocumentStates names every condition a
// close must distinguish, with a real file behind each one.
func TestClosePreflightClassifiesRealDocumentStates(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveInPlace)
	ctx := context.Background()

	writable := f.writeFile(t, "writable.md", "# writable\n", 0o600)
	f.session(t, "writable", writable, true)

	f.session(t, "untitled", "", true)

	readonly := f.writeFile(t, "readonly.md", "# read only\n", 0o400)
	f.session(t, "readonly", readonly, true)

	conflict := f.writeFile(t, "conflict.md", "# mine\n", 0o600)
	f.session(t, "conflict", conflict, true)
	if err := os.WriteFile(conflict, []byte("# theirs, and longer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(conflict, later, later); err != nil {
		t.Fatal(err)
	}

	deleted := f.writeFile(t, "deleted.md", "# deleted\n", 0o600)
	f.session(t, "deleted", deleted, true)
	if err := os.Remove(deleted); err != nil {
		t.Fatal(err)
	}

	failed := f.writeFile(t, "failed.md", "# autosave failed\n", 0o600)
	failedSession := f.session(t, "failed", failed, true)
	f.host.noteAction(failedSession, errors.New("markgogo: disk full"))

	f.session(t, "clean", f.writeFile(t, "clean.md", "# clean\n", 0o600), false)

	plan, err := f.host.ClosePreflight(ctx, []string{"writable", "untitled", "readonly", "conflict", "deleted", "failed", "clean"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(plan.Silent), []string{"writable"}; !sameIDs(got, want) {
		t.Fatalf("silent documents = %v, want %v: an autosaving writable draft writes itself", got, want)
	}
	if got, want := ids(plan.Prompt), []string{"untitled", "readonly", "conflict", "deleted", "failed"}; !sameIDs(got, want) {
		t.Fatalf("documents that need a decision = %v, want %v", got, want)
	}
	for _, doc := range plan.Prompt {
		switch doc.ID {
		case "untitled":
			if doc.Path != "" {
				t.Errorf("untitled document has path %q", doc.Path)
			}
		case "readonly":
			if !doc.ReadOnly {
				t.Error("read-only document is not reported read only")
			}
		case "deleted":
			if !doc.Deleted {
				t.Error("deleted document is not reported deleted")
			}
		}
	}
}

func TestClosePreflightAsksForEveryDocumentWhenAutosaveIsOff(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	path := f.writeFile(t, "notes.md", "# notes\n", 0o600)
	f.session(t, "notes", path, true)

	plan, err := f.host.ClosePreflight(context.Background(), []string{"notes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Silent) != 0 {
		t.Fatalf("autosave-off plan has silent documents %v, want none", ids(plan.Silent))
	}
	if got, want := ids(plan.Prompt), []string{"notes"}; !sameIDs(got, want) {
		t.Fatalf("autosave-off prompt = %v, want %v", got, want)
	}
}

// TestDiscardLeavesTheFileBytesUnchanged is the e2e smoke test's guarantee at
// the service level: Don't Save closes an autosave-off document without
// writing a byte.
func TestDiscardLeavesTheFileBytesUnchanged(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	path := f.writeFile(t, "Discussion.md", "# Discussion\n\nOriginal draft.\n", 0o600)
	f.session(t, "discussion", path, true)
	before := readDecisionFile(t, path)

	plan, err := f.host.ClosePreflight(context.Background(), []string{"discussion"})
	if err != nil {
		t.Fatal(err)
	}
	sel := SelectClose(plan, false, nil)
	unsaved, err := f.host.ResolveClose(context.Background(), sel)
	if err != nil || len(unsaved) != 0 {
		t.Fatalf("discard resolve: unsaved=%v err=%v", unsaved, err)
	}
	if err := f.host.CommitClose(context.Background(), sel); err != nil {
		t.Fatal(err)
	}
	if got := readDecisionFile(t, path); got != before {
		t.Fatalf("Don't Save changed the file: %q, want %q", got, before)
	}
	if f.edited("discussion") {
		t.Fatal("discarded document is still edited")
	}
}

// TestFailedSaveKeepsEveryDocumentDirty checks the ordering a batch close must
// keep: no document loses its dirty flag before every save has succeeded.
func TestFailedSaveKeepsEveryDocumentDirty(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	saving := f.writeFile(t, "saving.md", "# saving\n", 0o600)
	discarding := f.writeFile(t, "discarding.md", "# discarding\n", 0o600)
	f.session(t, "saving", saving, true)
	f.session(t, "discarding", discarding, true)
	before := readDecisionFile(t, discarding)

	sel := CloseSelection{Save: []string{"saving"}, Discard: []string{"discarding"}}
	unsaved, err := f.host.ResolveClose(context.Background(), sel)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got, want := ids(unsaved), []string{"saving"}; !sameIDs(got, want) {
		t.Fatalf("unsaved documents = %v, want %v", got, want)
	}
	// The window never commits a close whose saves failed.
	if f.edited("saving") == false || f.edited("discarding") == false {
		t.Fatal("a failed close must leave every document edited")
	}
	if got := readDecisionFile(t, discarding); got != before {
		t.Fatalf("aborted close wrote the discarded document: %q", got)
	}
}

func TestCommitCloseDiscardsOnlyTheChosenDocuments(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	keep := f.writeFile(t, "keep.md", "# keep\n", 0o600)
	drop := f.writeFile(t, "drop.md", "# drop\n", 0o600)
	f.session(t, "keep", keep, true)
	f.session(t, "drop", drop, true)

	sel := CloseSelection{Discard: []string{"drop"}}
	if err := f.host.CommitClose(context.Background(), sel); err != nil {
		t.Fatal(err)
	}
	if f.edited("drop") {
		t.Fatal("discarded document is still edited")
	}
	if !f.edited("keep") {
		t.Fatal("CommitClose cleared a document it was not given")
	}
	if got := readDecisionFile(t, drop); got != "# drop\n" {
		t.Fatalf("discard wrote the file: %q", got)
	}
}

// TestMoveRefusesATakenDestinationBeforeTouchingIt covers the entry guard: a
// replacement needs the user's confirmation, and asking for it must not have
// changed either file.
func TestMoveRefusesATakenDestinationBeforeTouchingIt(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	source := f.writeFile(t, "essays/notes.md", "# source\n", 0o600)
	target := f.writeFile(t, "archive/notes.md", "# destination\n", 0o600)
	f.session(t, "notes", source, false)

	err := f.host.MoveToFolder(context.Background(), "notes", filepath.Dir(target), false)
	if !errors.Is(err, ErrMoveNeedsConfirmation) {
		t.Fatalf("move over a taken name = %v, want ErrMoveNeedsConfirmation", err)
	}
	if got := readDecisionFile(t, target); got != "# destination\n" {
		t.Fatalf("refused move changed the destination: %q", got)
	}
	if got := readDecisionFile(t, source); got != "# source\n" {
		t.Fatalf("refused move changed the source: %q", got)
	}
}

// TestMoveRefusesADestinationAnotherTabHas checks the reservation before any
// rename, so an open destination is never lost.
func TestMoveRefusesADestinationAnotherTabHas(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	source := f.writeFile(t, "essays/notes.md", "# source\n", 0o600)
	target := f.writeFile(t, "archive/notes.md", "# open elsewhere\n", 0o600)
	f.session(t, "notes", source, false)
	f.session(t, "other", target, false)

	err := f.host.MoveToFolder(context.Background(), "notes", filepath.Dir(target), true)
	if !errors.Is(err, ErrMoveDestinationOpened) {
		t.Fatalf("move onto an open tab = %v, want ErrMoveDestinationOpened", err)
	}
	if got := readDecisionFile(t, target); got != "# open elsewhere\n" {
		t.Fatalf("refused move changed the open destination: %q", got)
	}
	if got := readDecisionFile(t, source); got != "# source\n" {
		t.Fatalf("refused move changed the source: %q", got)
	}
}

// TestMoveReplacementDisplacesAndRestoresTheDestination exercises the exact
// pair MoveToFolder runs around the move: the destination leaves its name
// first, and comes back untouched when the move fails.
func TestMoveReplacementDisplacesAndRestoresTheDestination(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	target := f.writeFile(t, "archive/notes.md", "# destination\n", 0o600)

	displaced, err := displaceFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if displaced == "" || displaced == target {
		t.Fatalf("displaced path = %q", displaced)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination is still at its name while displaced: %v", err)
	}
	if got := readDecisionFile(t, displaced); got != "# destination\n" {
		t.Fatalf("displaced copy = %q", got)
	}

	restoreFile(displaced, target)
	if got := readDecisionFile(t, target); got != "# destination\n" {
		t.Fatalf("restored destination = %q, want the original bytes", got)
	}
	if _, err := os.Stat(displaced); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary sibling survived the restore: %v", err)
	}

	// The success path drops the displaced copy instead of restoring it.
	displaced, err = displaceFile(target)
	if err != nil {
		t.Fatal(err)
	}
	dropFile(displaced)
	if _, err := os.Stat(displaced); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary sibling survived the drop: %v", err)
	}
}

// TestMoveTargetOfAnUntitledDocumentIsChecked covers the destination MyGo
// computes for a document that has no file yet: the picker must be able to ask
// before that write replaces something.
func TestMoveTargetOfAnUntitledDocumentIsChecked(t *testing.T) {
	f := newDecisionFixture(t, mygo.AutosaveOff)
	directory := filepath.Join(f.dir, "archive")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	taken := filepath.Join(directory, "Untitled 2.md")
	if err := os.WriteFile(taken, []byte("# existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := MoveTarget(DocumentInfo{Name: "Untitled 2"}, directory); got != taken {
		t.Fatalf("untitled move target = %q, want %q", got, taken)
	}
	if _, err := os.Lstat(taken); err != nil {
		t.Fatalf("destination check disturbed the existing file: %v", err)
	}
}
