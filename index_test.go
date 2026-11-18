package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeMD(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseMarkdownMetaHeadingsAndNotes(t *testing.T) {
	body := "---\ntitle: ignored\n---\n\n# Quarterly Report\n\nIntro paragraph with enough words to become the excerpt.\n\n<!-- note: keep this figure -->\n\n## Findings\n\nMore body.\n\n```\n# not a heading\n```\n\nSetext Title\n============\n"
	meta := parseMarkdownMeta(body)
	if meta.Title != "Quarterly Report" {
		t.Fatalf("title = %q", meta.Title)
	}
	if !strings.Contains(meta.Excerpt, "Intro paragraph") {
		t.Fatalf("excerpt = %q", meta.Excerpt)
	}
	if meta.Notes != 1 {
		t.Fatalf("notes = %d", meta.Notes)
	}
	var texts []string
	for _, h := range meta.Headings {
		texts = append(texts, h.Text)
		if h.Offset < 0 {
			t.Fatalf("negative offset for %q", h.Text)
		}
	}
	want := []string{"Quarterly Report", "Findings", "Setext Title"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("headings = %v, want %v", texts, want)
	}
}

func TestParseMarkdownMetaUTF16Offset(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{
			name: "ASCII, CJK and emoji headings",
			body: "# one\n# 中\n# 😀\n# mixed café 😀\n",
			// JS anchors count UTF-16 code units: emoji cost two.
			want: []int{0, 6, 10, 15},
		},
		{
			name: "CRLF and mixed line endings",
			body: "# one\r\n# 中\r\n\r\n# 😀\n",
			want: []int{0, 7, 14},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := parseMarkdownMeta(tc.body)
			if len(meta.Headings) != len(tc.want) {
				t.Fatalf("headings = %+v, want %d", meta.Headings, len(tc.want))
			}
			for i, h := range meta.Headings {
				if h.Offset != tc.want[i] {
					t.Fatalf("heading %d (%q) offset = %d, want %d", i, h.Text, h.Offset, tc.want[i])
				}
			}
		})
	}
}

func TestNavSnapshotListsNestedMarkdownAndSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "audit.md", "# Audit\n\nRoot notes.\n")
	writeMD(t, dir, "notes/deep/doc.md", "# Nested\n\nDeep file.\n")
	if err := os.Mkdir(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	snap := host.navSnapshot(w)
	if snap.Status != IndexReady {
		t.Fatalf("status = %s error=%s", snap.Status, snap.Error)
	}
	if snap.Total != 2 {
		t.Fatalf("total = %d, want 2", snap.Total)
	}
	var files, folders []string
	for _, e := range snap.Entries {
		switch e.Kind {
		case "file":
			files = append(files, e.Name)
			if e.Title == "" {
				t.Fatalf("file %s missing title", e.Name)
			}
			if e.ModifiedAt == 0 {
				t.Fatalf("file %s missing mtime", e.Name)
			}
		case "folder":
			folders = append(folders, e.Name)
			if e.MarkdownCount <= 0 {
				t.Fatalf("folder %s has no markdown count", e.Name)
			}
		}
	}
	if !containsAll(files, "audit.md", "doc.md") {
		t.Fatalf("files = %v", files)
	}
	if !containsAll(folders, "notes", "deep") {
		t.Fatalf("folders = %v", folders)
	}
	for _, e := range snap.Entries {
		if e.Name == "empty" || e.Name == "skip.txt" {
			t.Fatalf("placeholder leaked: %+v", e)
		}
	}
}

func TestNavSnapshotKeepsDescendantsUnderTheirFolder(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "alpha.md", "# Alpha\n\nRoot alpha.\n")
	writeMD(t, dir, "beta.md", "# Beta\n\nRoot beta.\n")
	writeMD(t, dir, "notes/alpha/notes.md", "# Alpha notes\n\nNested alpha.\n")
	writeMD(t, dir, "notes/beta/notes.md", "# Beta notes\n\nNested beta.\n")

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	snap := host.navSnapshot(w)
	ids := make([]string, 0, len(snap.Entries))
	for _, e := range snap.Entries {
		ids = append(ids, e.ID)
	}
	want := []string{
		"folder:notes",
		"folder:notes/alpha",
		"file:notes/alpha/notes.md",
		"folder:notes/beta",
		"file:notes/beta/notes.md",
		"file:alpha.md",
		"file:beta.md",
	}
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("entries = %v, want %v", ids, want)
	}
}

func TestNavFilterUsesUnsavedOpenText(t *testing.T) {
	dir := t.TempDir()
	path := writeMD(t, dir, "notes.md", "# Disk Title\n\nSaved excerpt.\n")

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	rel := relSlash(dir, path)
	open := map[string]string{canonicalPath(path): "# Live Heading\n\nUnsaved body.\n"}
	w.index.mu.Lock()
	file := w.index.files[rel]
	if file == nil {
		w.index.mu.Unlock()
		t.Fatal("indexed file missing")
	}
	host.indexOne(file, open)
	w.index.mu.Unlock()

	got := host.searchLockedResult(w, w.index, "Live Heading")
	if got.Incomplete {
		t.Fatalf("ready search reported incomplete: %+v", got)
	}
	if got.Matched != 1 || len(got.Hits) != 1 {
		t.Fatalf("matched = %+v", got)
	}
	if len(got.Hits[0].Headings) == 0 || got.Hits[0].Headings[0].Text != "Live Heading" {
		t.Fatalf("headings = %+v", got.Hits[0].Headings)
	}
	disk := host.searchLockedResult(w, w.index, "Disk Title")
	if disk.Matched != 0 {
		t.Fatalf("disk heading still indexed after unsaved text: %+v", disk)
	}
}

func TestNavFilterIncompleteWhileScanning(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "a.md", "# Alpha\n")
	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws", root: dir, index: newWorkspaceIndex(dir, nil)}
	w.index.status = IndexScanning
	w.index.files["a.md"] = &indexedFile{path: filepath.Join(dir, "a.md"), name: "a.md", indexed: false}
	got := host.searchLockedResult(w, w.index, "Alpha")
	if !got.Incomplete || got.Status != IndexScanning {
		t.Fatalf("want incomplete scanning, got %+v", got)
	}
}

func TestToggleFolderHidesNestedFiles(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "notes/deep/doc.md", "# Nested\n")
	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	w.index.mu.Lock()
	w.index.collapsed["notes"] = true
	w.index.mu.Unlock()
	snap := host.navSnapshot(w)
	for _, e := range snap.Entries {
		if e.Kind == "file" && e.Name == "doc.md" {
			t.Fatalf("collapsed folder still listed file: %+v", e)
		}
	}
	if snap.CollapsedCount == 0 {
		t.Fatal("collapsed count stayed 0")
	}
}

func TestRememberWorkspacePersists(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	st := &settingsStore{dir: dir}
	if err := st.rememberWorkspace(root); err != nil {
		t.Fatal(err)
	}
	list := st.recentWorkspaces()
	if len(list) != 1 || filepath.Clean(list[0].Root) != filepath.Clean(canonicalPath(root)) {
		t.Fatalf("recent = %+v", list)
	}
}

func TestBreadcrumbsFromRoot(t *testing.T) {
	root := filepath.Join("/tmp", "ws")
	file := filepath.Join(root, "notes", "a.md")
	crumbs := breadcrumbsFor(root, file)
	if len(crumbs) < 2 {
		t.Fatalf("crumbs = %+v", crumbs)
	}
	last := crumbs[len(crumbs)-1]
	if last.Name != "a.md" || !last.IsFile {
		t.Fatalf("last = %+v", last)
	}
}

func containsAll(have []string, want ...string) bool {
next:
	for _, w := range want {
		for _, h := range have {
			if h == w {
				continue next
			}
		}
		return false
	}
	return true
}

func TestNavSnapshotMergesOpenDocumentThroughDirectoryAlias(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeMD(t, root, "notes/alpha.md", "# Alpha\n\nBody text.\n")
	// The open document reports the file through a symlinked spelling of the
	// root, so only the routing boundary resolves it to the indexed path.
	aliasRoot := filepath.Join(t.TempDir(), "ws-alias")
	if err := os.Symlink(root, aliasRoot); err != nil {
		t.Fatal(err)
	}
	aliased := filepath.Join(aliasRoot, "notes", "alpha.md")

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws-alias", root: root}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	s := &docSession{
		host:   host,
		id:     "doc-alpha",
		ws:     w,
		pathFn: func() string { return aliased },
	}
	host.mu.Lock()
	host.byID[s.id] = s
	w.tabs = append(w.tabs, s.id)
	w.active = s.id
	host.mu.Unlock()

	snap := host.navSnapshot(w)
	var entries []NavEntry
	for _, e := range snap.Entries {
		if e.Name == "alpha.md" {
			entries = append(entries, e)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("alpha.md entries = %+v", entries)
	}
	if entries[0].ID != "file:notes/alpha.md" {
		t.Fatalf("entry id = %q", entries[0].ID)
	}
	if entries[0].DocumentID != "doc-alpha" || entries[0].Deleted {
		t.Fatalf("entry did not merge the open document: %+v", entries[0])
	}
	if entries[0].Parent != "notes" || entries[0].Depth != 1 {
		t.Fatalf("entry rel = parent %q depth %d", entries[0].Parent, entries[0].Depth)
	}

	byName := host.searchLockedResult(w, w.index, "alpha")
	if byName.Matched != 1 || len(byName.Hits) != 1 || byName.Hits[0].DocumentID != "doc-alpha" {
		t.Fatalf("name filter = %+v", byName)
	}
	byPath := host.searchLockedResult(w, w.index, "notes/alpha")
	if byPath.Matched != 1 {
		t.Fatalf("relative-path filter = %+v", byPath)
	}
}

func TestNavSnapshotKeepsOpenDocumentMissingFromScan(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "kept.md", "# Kept\n")
	draft := filepath.Join(dir, "drafts", "idea.md")

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws-extra", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	s := &docSession{
		host:   host,
		id:     "doc-draft",
		ws:     w,
		pathFn: func() string { return draft },
	}
	host.mu.Lock()
	host.byID[s.id] = s
	w.tabs = append(w.tabs, s.id)
	w.active = s.id
	host.mu.Unlock()

	snap := host.navSnapshot(w)
	var found []NavEntry
	for _, e := range snap.Entries {
		if e.ID == "open:doc-draft" {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("draft entries = %+v", snap.Entries)
	}
	got := found[0]
	if !got.Deleted || got.DocumentID != "doc-draft" || got.Parent != "drafts" || got.Depth != 1 {
		t.Fatalf("draft entry = %+v", got)
	}
}

func TestLargeWorkspaceFiltersFiveHundredMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 500; i++ {
		writeMD(t, dir, filepath.Join("group", fmt.Sprintf("%02d", i%20), fmt.Sprintf("doc-%03d.md", i)), fmt.Sprintf("# Document %03d\n\nBody %d.\n", i, i))
	}
	if err := os.WriteFile(filepath.Join(dir, "asset.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "large", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 5*time.Second)
	snap := host.navSnapshot(w)
	if snap.Status != IndexReady || snap.Total != 500 {
		t.Fatalf("large index status=%s total=%d error=%s", snap.Status, snap.Total, snap.Error)
	}
	result := host.searchLockedResult(w, w.index, "Document 499")
	if result.Incomplete || result.Matched != 1 || len(result.Hits) != 1 || result.Hits[0].Name != "doc-499.md" {
		t.Fatalf("large filter = %+v", result)
	}
	for _, entry := range snap.Entries {
		if entry.Kind == "file" && !markdownName(entry.Name) {
			t.Fatalf("non-Markdown navigation entry: %+v", entry)
		}
	}
}

func TestNavSnapshotAndReindexWithOpenTabDoNotDeadlock(t *testing.T) {
	dir := t.TempDir()
	alpha := writeMD(t, dir, "alpha.md", "# Alpha\n\nFirst document.\n")
	writeMD(t, dir, "beta.md", "# Beta\n\nSecond document.\n")

	host := newDocumentHost()
	host.disableWatch = true
	host.settings = &settingsStore{dir: t.TempDir()}
	w := &workspace{id: "ws-open", root: dir}
	host.startIndex(w)
	host.waitIndexReady(w, 2*time.Second)

	s := &docSession{
		host:   host,
		id:     "doc-alpha",
		ws:     w,
		pathFn: func() string { return alpha },
	}
	host.mu.Lock()
	host.byID[s.id] = s
	w.tabs = append(w.tabs, s.id)
	w.active = s.id
	host.mu.Unlock()

	done := make(chan NavSnapshot, 1)
	go func() {
		host.reindexOpenDocument(s)
		done <- host.navSnapshot(w)
	}()

	var snap NavSnapshot
	select {
	case snap = <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reindexOpenDocument/navSnapshot deadlocked with an open workspace tab")
	}

	var matched bool
	for _, e := range snap.Entries {
		if e.Kind == "file" && e.Name == "alpha.md" {
			if e.DocumentID != "doc-alpha" {
				t.Fatalf("alpha.md DocumentID = %q, want doc-alpha", e.DocumentID)
			}
			matched = true
		}
	}
	if !matched {
		t.Fatalf("alpha.md missing from navSnapshot entries: %+v", snap.Entries)
	}
}
