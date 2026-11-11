package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo"
)

const (
	maxIndexBytes       = 1 << 20
	maxRecentWorkspaces = 12
	indexBatchSize      = 24
	navScanYieldEvery   = 64
)

// FolderState is the collapsed/expanded state of one directory in the tree.
type FolderState string

const (
	FolderOpen      FolderState = "open"
	FolderCollapsed FolderState = "collapsed"
	FolderEmpty     FolderState = "empty"
)

// IndexStatus is the honest scan/index progress of a workspace root.
type IndexStatus string

const (
	IndexIdle     IndexStatus = "idle"
	IndexScanning IndexStatus = "scanning"
	IndexIndexing IndexStatus = "indexing"
	IndexReady    IndexStatus = "ready"
	IndexError    IndexStatus = "error"
)

// NavEntry is one folder or Markdown file in the workspace tree.
type NavEntry struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"` // "file" | "folder"
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	Parent        string      `json:"parent"`
	Depth         int         `json:"depth"`
	MarkdownCount int         `json:"markdownCount"`
	FolderState   FolderState `json:"folderState,omitempty"`
	Title         string      `json:"title,omitempty"`
	Excerpt       string      `json:"excerpt,omitempty"`
	ModifiedAt    int64       `json:"modifiedAt,omitempty"`
	Notes         int         `json:"notes,omitempty"`
	Edited        bool        `json:"edited,omitempty"`
	Conflict      bool        `json:"conflict,omitempty"`
	Deleted       bool        `json:"deleted,omitempty"`
	DocumentID    string      `json:"documentId,omitempty"`
}

// NavSnapshot is the tree as the page lists it.
type NavSnapshot struct {
	Root           string      `json:"root"`
	Status         IndexStatus `json:"status"`
	Indexed        int         `json:"indexed"`
	Total          int         `json:"total"`
	Error          string      `json:"error,omitempty"`
	Entries        []NavEntry  `json:"entries"`
	CollapsedCount int         `json:"collapsedCount"`
}

// HeadingHit is a heading that matched a filter.
type HeadingHit struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Level  int    `json:"level"`
}

// NavSearchHit is one file that matched a filter, with heading hits.
type NavSearchHit struct {
	Path       string       `json:"path"`
	Name       string       `json:"name"`
	Title      string       `json:"title"`
	Excerpt    string       `json:"excerpt"`
	DocumentID string       `json:"documentId,omitempty"`
	Headings   []HeadingHit `json:"headings"`
}

// NavSearchResult is the current filter result, including incomplete scans.
type NavSearchResult struct {
	Query      string         `json:"query"`
	Status     IndexStatus    `json:"status"`
	Matched    int            `json:"matched"`
	Indexed    int            `json:"indexed"`
	Total      int            `json:"total"`
	Incomplete bool           `json:"incomplete"`
	Hits       []NavSearchHit `json:"hits"`
}

// RecentWorkspace is one remembered folder workspace.
type RecentWorkspace struct {
	Root     string `json:"root"`
	Name     string `json:"name"`
	OpenedAt int64  `json:"openedAt"`
}

// Crumb is one path-bar segment.
type Crumb struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	IsFile bool   `json:"isFile"`
}

var (
	navChanged     = mygo.NewEvent[NavSnapshot]("navChanged")
	navSearchReady = mygo.NewEvent[NavSearchResult]("navSearchReady")
)

type indexedFile struct {
	path     string
	name     string
	parent   string
	modTime  int64
	title    string
	excerpt  string
	notes    int
	headings []NavHeading
	indexed  bool
	missing  bool
}

type indexedFolder struct {
	path     string
	name     string
	parent   string
	mdCount  int
	children []string
}

type workspaceIndex struct {
	mu        sync.Mutex
	root      string
	w         *workspace
	status    IndexStatus
	errText   string
	files     map[string]*indexedFile
	folders   map[string]*indexedFolder
	order     []string // relative paths of files, then folders, for tree walk
	collapsed map[string]bool
	query     string
	stop      context.CancelFunc
	gen       int
	watchStop context.CancelFunc
}

func newWorkspaceIndex(root string, w *workspace) *workspaceIndex {
	return &workspaceIndex{
		root:      root,
		w:         w,
		status:    IndexIdle,
		files:     make(map[string]*indexedFile),
		folders:   make(map[string]*indexedFolder),
		collapsed: make(map[string]bool),
	}
}

func markdownName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func skipDirName(name string) bool {
	switch name {
	case ".git", "node_modules", ".hg", ".svn", "__pycache__", ".next", "dist", "build":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(path))
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return rel
}

func parentSlash(rel string) string {
	if rel == "" {
		return ""
	}
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}

func (idx *workspaceIndex) stopLocked() {
	if idx.stop != nil {
		idx.stop()
		idx.stop = nil
	}
	if idx.watchStop != nil {
		idx.watchStop()
		idx.watchStop = nil
	}
	idx.gen++
}

func (h *documentHost) indexOf(w *workspace) *workspaceIndex {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.indexOfLocked(w)
}

func (h *documentHost) indexOfLocked(w *workspace) *workspaceIndex {
	if w == nil {
		return nil
	}
	if w.index != nil && w.index.root == w.root {
		return w.index
	}
	if w.index != nil {
		w.index.mu.Lock()
		w.index.stopLocked()
		w.index.mu.Unlock()
	}
	if w.root == "" {
		w.index = nil
		return nil
	}
	idx := newWorkspaceIndex(w.root, w)
	w.index = idx
	return idx
}

func (h *documentHost) startIndex(w *workspace) {
	idx := h.indexOf(w)
	if idx == nil {
		return
	}
	idx.mu.Lock()
	idx.stopLocked()
	ctx, cancel := context.WithCancel(context.Background())
	idx.stop = cancel
	idx.status = IndexScanning
	idx.errText = ""
	gen := idx.gen
	idx.mu.Unlock()
	h.emitNav(w)
	go h.scanIndex(ctx, w, idx, gen)
}

func (h *documentHost) scanIndex(ctx context.Context, w *workspace, idx *workspaceIndex, gen int) {
	root := idx.root
	files := map[string]*indexedFile{}
	folders := map[string]*indexedFolder{}

	ensureFolder := func(rel string) {
		for rel != "" {
			if _, ok := folders[rel]; ok {
				rel = parentSlash(rel)
				continue
			}
			folders[rel] = &indexedFolder{
				path:   filepath.Join(root, filepath.FromSlash(rel)),
				name:   filepath.Base(rel),
				parent: parentSlash(rel),
			}
			rel = parentSlash(rel)
		}
	}

	seen := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		name := d.Name()
		if path != root && d.IsDir() && skipDirName(name) {
			return fs.SkipDir
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel != "" {
				ensureFolder(rel)
			}
			return nil
		}
		if !markdownName(name) {
			return nil
		}
		info, statErr := d.Info()
		mod := int64(0)
		if statErr == nil {
			mod = info.ModTime().UnixMilli()
		}
		ensureFolder(parentSlash(rel))
		files[rel] = &indexedFile{
			path:    path,
			name:    name,
			parent:  parentSlash(rel),
			modTime: mod,
		}
		seen++
		if seen%navScanYieldEvery == 0 {
			idx.mu.Lock()
			live := idx.gen == gen
			idx.mu.Unlock()
			if !live {
				return ctx.Err()
			}
			h.publishPartial(w, idx, gen, files, folders, IndexScanning)
		}
		return nil
	})
	if ctx.Err() != nil {
		return
	}

	idx.mu.Lock()
	if idx.gen != gen {
		idx.mu.Unlock()
		return
	}
	idx.files = files
	idx.folders = folders
	idx.recountLocked()
	if err != nil && !errors.Is(err, context.Canceled) {
		idx.status = IndexError
		idx.errText = err.Error()
	} else {
		idx.status = IndexIndexing
		idx.errText = ""
	}
	idx.mu.Unlock()
	h.emitNav(w)

	h.indexPending(ctx, w, idx, gen)
	if ctx.Err() != nil {
		return
	}
	idx.mu.Lock()
	if idx.gen != gen {
		idx.mu.Unlock()
		return
	}
	if idx.status != IndexError {
		idx.status = IndexReady
	}
	idx.mu.Unlock()
	h.emitNav(w)
	h.emitSearch(w, idx)
	go h.watchIndex(w, idx, gen)
}

func (idx *workspaceIndex) recountLocked() {
	for _, f := range idx.folders {
		f.mdCount = 0
		f.children = f.children[:0]
	}
	for rel, file := range idx.files {
		if file.missing {
			continue
		}
		p := file.parent
		for p != "" {
			folder := idx.folders[p]
			if folder == nil {
				break
			}
			folder.mdCount++
			p = folder.parent
		}
		_ = rel
	}
}

func (h *documentHost) publishPartial(w *workspace, idx *workspaceIndex, gen int, files map[string]*indexedFile, folders map[string]*indexedFolder, status IndexStatus) {
	idx.mu.Lock()
	if idx.gen != gen {
		idx.mu.Unlock()
		return
	}
	idx.files = files
	idx.folders = folders
	idx.recountLocked()
	idx.status = status
	idx.mu.Unlock()
	h.emitNav(w)
}

func (h *documentHost) indexPending(ctx context.Context, w *workspace, idx *workspaceIndex, gen int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		idx.mu.Lock()
		if idx.gen != gen {
			idx.mu.Unlock()
			return
		}
		var batch []*indexedFile
		for _, file := range idx.files {
			if file.indexed || file.missing {
				continue
			}
			batch = append(batch, file)
			if len(batch) >= indexBatchSize {
				break
			}
		}
		idx.mu.Unlock()
		if len(batch) == 0 {
			return
		}
		open := h.openTexts(w)
		for _, file := range batch {
			select {
			case <-ctx.Done():
				return
			default:
			}
			h.indexOne(file, open)
		}
		h.emitNav(w)
		h.emitSearch(w, idx)
	}
}

func (h *documentHost) openTexts(w *workspace) map[string]string {
	out := map[string]string{}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil || s.doc == nil {
			continue
		}
		p := s.currentPath()
		if p == "" {
			continue
		}
		out[canonicalPath(p)] = s.doc.Content()
	}
	return out
}

func (h *documentHost) indexOne(file *indexedFile, open map[string]string) {
	text, ok := open[canonicalPath(file.path)]
	if !ok {
		data, err := readCapped(file.path, maxIndexBytes)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				file.missing = true
			}
			file.indexed = true
			return
		}
		text = string(data)
		if info, err := os.Stat(file.path); err == nil {
			file.modTime = info.ModTime().UnixMilli()
		}
	}
	meta := parseMarkdownMeta(text)
	file.title = meta.Title
	file.excerpt = meta.Excerpt
	file.notes = meta.Notes
	file.headings = meta.Headings
	file.indexed = true
	file.missing = false
}

func readCapped(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		data = data[:max]
	}
	return data, nil
}

func (h *documentHost) watchIndex(w *workspace, idx *workspaceIndex, gen int) {
	if h.disableWatch {
		return
	}
	idx.mu.Lock()
	if idx.gen != gen {
		idx.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	idx.watchStop = cancel
	root := idx.root
	idx.mu.Unlock()

	watcher, err := mygo.WatchFiles(ctx, root, mygo.FileWatchOptions{
		Recursive: true,
		Ignore: func(path string, isDir bool) bool {
			base := path
			if i := strings.LastIndex(path, "/"); i >= 0 {
				base = path[i+1:]
			}
			if isDir {
				return skipDirName(base)
			}
			return !markdownName(base)
		},
		Debounce: 80 * time.Millisecond,
	})
	if err != nil {
		return
	}
	for {
		evts, nextErr := watcher.Next(ctx)
		if nextErr != nil {
			return
		}
		idx.mu.Lock()
		live := idx.gen == gen
		idx.mu.Unlock()
		if !live {
			return
		}
		h.applyWatch(w, idx, evts)
	}
}

func (h *documentHost) applyWatch(w *workspace, idx *workspaceIndex, evts []mygo.FileEvent) {
	idx.mu.Lock()
	root := idx.root
	changed := false
	for _, ev := range evts {
		rel := ev.Path
		if rel == "." {
			idx.mu.Unlock()
			h.startIndex(w)
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		switch ev.Op {
		case mygo.FileCreate, mygo.FileWrite, mygo.FileRename:
			if ev.IsDir {
				changed = true
				continue
			}
			if !markdownName(filepath.Base(rel)) {
				continue
			}
			file := idx.files[rel]
			if file == nil {
				file = &indexedFile{
					path:   abs,
					name:   filepath.Base(rel),
					parent: parentSlash(rel),
				}
				idx.files[rel] = file
			}
			file.indexed = false
			file.missing = false
			changed = true
			if ev.Op == mygo.FileRename && ev.OldPath != "" && ev.OldPath != "." {
				delete(idx.files, ev.OldPath)
			}
		case mygo.FileRemove:
			if ev.IsDir {
				delete(idx.folders, rel)
				for p := range idx.files {
					if p == rel || strings.HasPrefix(p, rel+"/") {
						delete(idx.files, p)
					}
				}
				changed = true
				continue
			}
			if file, ok := idx.files[rel]; ok {
				file.missing = true
				changed = true
			}
		}
	}
	if changed {
		idx.recountLocked()
	}
	idx.mu.Unlock()
	if !changed {
		return
	}
	open := h.openTexts(w)
	idx.mu.Lock()
	for _, file := range idx.files {
		if !file.indexed && !file.missing {
			h.indexOne(file, open)
		}
	}
	idx.mu.Unlock()
	h.emitNav(w)
	h.emitSearch(w, idx)
}

func (h *documentHost) emitNav(w *workspace) {
	if w == nil || w.win == nil || w.win.IsDestroyed() {
		return
	}
	_ = navChanged.Emit(w.win, h.navSnapshot(w))
}

func (h *documentHost) emitSearch(w *workspace, idx *workspaceIndex) {
	if w == nil || w.win == nil || w.win.IsDestroyed() || idx == nil {
		return
	}
	idx.mu.Lock()
	query := idx.query
	idx.mu.Unlock()
	_ = navSearchReady.Emit(w.win, h.searchLockedResult(w, idx, query))
}

func (h *documentHost) navSnapshot(w *workspace) NavSnapshot {
	idx := h.indexOf(w)
	if idx == nil {
		return NavSnapshot{Root: w.root, Status: IndexIdle, Entries: []NavEntry{}}
	}
	openByRel := map[string]*docSession{}
	extraTabs := []string{}
	extraInfo := map[string]DocumentInfo{}
	h.mu.Lock()
	extraTabs = append(extraTabs, w.tabs...)
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil {
			continue
		}
		info := s.infoLocked()
		extraInfo[id] = info
		if rel := relSlash(idx.root, info.Path); rel != "" {
			openByRel[rel] = s
		}
	}
	h.mu.Unlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()

	sortable := make([]navSortEntry, 0, len(idx.files)+len(idx.folders))
	visible := func(rel string) bool {
		p := parentSlash(rel)
		for p != "" {
			if idx.collapsed[p] {
				return false
			}
			p = parentSlash(p)
		}
		return true
	}

	var folderKeys []string
	for rel, folder := range idx.folders {
		if folder.mdCount == 0 {
			continue
		}
		folderKeys = append(folderKeys, rel)
	}
	sort.Strings(folderKeys)
	for _, rel := range folderKeys {
		folder := idx.folders[rel]
		state := FolderOpen
		if idx.collapsed[rel] {
			state = FolderCollapsed
		}
		if !visible(rel) {
			continue
		}
		sortable = append(sortable, navSortEntry{rel: rel, entry: NavEntry{
			ID:            "folder:" + rel,
			Kind:          "folder",
			Name:          folder.name,
			Path:          folder.path,
			Parent:        folder.parent,
			Depth:         depthOf(rel),
			MarkdownCount: folder.mdCount,
			FolderState:   state,
		}})
	}

	var fileKeys []string
	for rel := range idx.files {
		fileKeys = append(fileKeys, rel)
	}
	sort.Strings(fileKeys)
	seenOpen := map[string]bool{}
	for _, rel := range fileKeys {
		file := idx.files[rel]
		sess := openByRel[rel]
		deleted := file.missing
		var st DocumentInfo
		if sess != nil {
			seenOpen[sess.id] = true
			st = extraInfo[sess.id]
			deleted = deleted || st.Deleted
		}
		if file.missing && sess == nil {
			continue
		}
		if !visible(rel) {
			continue
		}
		entry := NavEntry{
			ID:         "file:" + rel,
			Kind:       "file",
			Name:       file.name,
			Path:       file.path,
			Parent:     file.parent,
			Depth:      depthOf(rel),
			Title:      file.title,
			Excerpt:    file.excerpt,
			ModifiedAt: file.modTime,
			Notes:      file.notes,
		}
		if sess != nil {
			entry.Edited = st.Edited
			entry.Conflict = st.Conflict
			entry.Deleted = deleted
			entry.DocumentID = sess.id
		} else if deleted {
			entry.Deleted = true
		}
		sortable = append(sortable, navSortEntry{rel: rel, entry: entry})
	}

	// Open documents that were deleted from disk still have a navigable row.
	for _, id := range extraTabs {
		if seenOpen[id] {
			continue
		}
		snap, ok := extraInfo[id]
		if !ok {
			continue
		}
		p := snap.Path
		if p == "" || !folderContains(idx.root, p) {
			continue
		}
		rel := relSlash(idx.root, p)
		sortable = append(sortable, navSortEntry{rel: rel, entry: NavEntry{
			ID:         "open:" + id,
			Kind:       "file",
			Name:       snap.Name,
			Path:       p,
			Parent:     parentSlash(rel),
			Depth:      depthOf(rel),
			Edited:     snap.Edited,
			Conflict:   snap.Conflict,
			Deleted:    true,
			DocumentID: id,
		}})
	}

	sort.SliceStable(sortable, func(i, j int) bool {
		return navRelLess(sortable[i].rel, sortable[i].entry.Kind, sortable[j].rel, sortable[j].entry.Kind)
	})
	entries := make([]NavEntry, len(sortable))
	for i := range sortable {
		entries[i] = sortable[i].entry
	}

	indexed, total := 0, 0
	for _, file := range idx.files {
		if file.missing {
			continue
		}
		total++
		if file.indexed {
			indexed++
		}
	}
	collapsed := 0
	for rel, on := range idx.collapsed {
		if !on {
			continue
		}
		if folder := idx.folders[rel]; folder != nil {
			collapsed += folder.mdCount
		}
	}
	return NavSnapshot{
		Root:           idx.root,
		Status:         idx.status,
		Indexed:        indexed,
		Total:          total,
		Error:          idx.errText,
		Entries:        entries,
		CollapsedCount: collapsed,
	}
}

func depthOf(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/")
}

// navSortEntry carries one workspace-relative path, already known from the
// index key or computed once for an open document, so the tree sort never
// normalises or splits paths inside the comparator.
type navSortEntry struct {
	entry NavEntry
	rel   string
}

// navRelLess orders two tree entries depth-first: a folder is followed
// immediately by its descendants. Siblings list folders before files, then
// compare names case-insensitively. It walks the precomputed relative paths
// with strings.Cut so comparing two entries allocates nothing.
func navRelLess(arel, aKind, brel, bKind string) bool {
	for {
		if arel == brel {
			return false
		}
		aSeg, aRest, aMore := strings.Cut(arel, "/")
		bSeg, bRest, bMore := strings.Cut(brel, "/")
		if aSeg != bSeg {
			aLeaf, bLeaf := "folder", "folder"
			if !aMore {
				aLeaf = aKind
			}
			if !bMore {
				bLeaf = bKind
			}
			if aLeaf != bLeaf {
				return aLeaf == "folder"
			}
			if c := compareFold(aSeg, bSeg); c != 0 {
				return c < 0
			}
			return aSeg < bSeg
		}
		if !aMore || !bMore {
			return !aMore && bMore
		}
		arel, brel = aRest, bRest
	}
}

// compareFold orders names case-insensitively without allocating, matching the
// per-rune lowercasing the tree convention used before.
func compareFold(a, b string) int {
	for a != "" && b != "" {
		ar, an := utf8.DecodeRuneInString(a)
		br, bn := utf8.DecodeRuneInString(b)
		if al, bl := unicode.ToLower(ar), unicode.ToLower(br); al != bl {
			if al < bl {
				return -1
			}
			return 1
		}
		a, b = a[an:], b[bn:]
	}
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return -1
	default:
		return 1
	}
}

func (h *documentHost) searchLockedResult(w *workspace, idx *workspaceIndex, query string) NavSearchResult {
	// Open documents are the routing boundary: resolve each one to the same
	// workspace-relative key the index stores, once, instead of recanonicalising
	// every indexed path per filter.
	openByRel := map[string]string{}
	for p, text := range h.openTexts(w) {
		if rel := relSlash(idx.root, p); rel != "" {
			openByRel[rel] = text
		}
	}
	openIDsByRel := map[string]string{}
	h.mu.Lock()
	for _, s := range h.byID {
		if p := s.currentPath(); p != "" {
			if rel := relSlash(idx.root, p); rel != "" {
				openIDsByRel[rel] = s.id
			}
		}
	}
	h.mu.Unlock()

	idx.mu.Lock()
	indexed, total := 0, 0
	type searchFile struct {
		rel  string
		file indexedFile
	}
	// Allocate without reading the map before taking idx.mu; concurrent
	// reindexing may add or remove entries while this filter result builds.
	files := make([]searchFile, 0)
	for rel, file := range idx.files {
		if file.missing {
			continue
		}
		total++
		if file.indexed {
			indexed++
		}
		copy := *file
		copy.headings = append([]NavHeading(nil), file.headings...)
		files = append(files, searchFile{rel: rel, file: copy})
	}
	status := idx.status
	idx.mu.Unlock()

	incomplete := status == IndexScanning || status == IndexIndexing || indexed < total
	q := strings.TrimSpace(query)
	result := NavSearchResult{
		Query:      q,
		Status:     status,
		Indexed:    indexed,
		Total:      total,
		Incomplete: incomplete,
		Hits:       []NavSearchHit{},
	}
	if q == "" {
		return result
	}
	needle := strings.ToLower(q)
	sort.Slice(files, func(i, j int) bool { return files[i].file.path < files[j].file.path })
	for _, sf := range files {
		file, rel := &sf.file, sf.rel
		title, excerpt, headings := file.title, file.excerpt, file.headings
		if text, ok := openByRel[rel]; ok {
			m := parseMarkdownMeta(text)
			title, excerpt, headings = m.Title, m.Excerpt, m.Headings
		}
		var headingHits []HeadingHit
		matched := containsFold(rel, needle) || containsFold(file.name, needle) || containsFold(title, needle) || containsFold(excerpt, needle)
		for _, hd := range headings {
			if containsFold(hd.Text, needle) {
				matched = true
				headingHits = append(headingHits, HeadingHit{Text: hd.Text, Offset: hd.Offset, Level: hd.Level})
			}
		}
		if !matched {
			continue
		}
		hit := NavSearchHit{
			Path:       file.path,
			Name:       file.name,
			Title:      title,
			Excerpt:    excerpt,
			DocumentID: openIDsByRel[rel],
			Headings:   headingHits,
		}
		result.Hits = append(result.Hits, hit)
	}
	result.Matched = len(result.Hits)
	return result
}

func containsFold(hay, needle string) bool {
	return needle != "" && strings.Contains(strings.ToLower(hay), needle)
}

// NavSnapshot is the calling window's file tree. Opening a workspace does
// not wait for a full directory scan: scanning and indexing continue in
// the background and arrive through navChanged.
func (h *documentHost) NavSnapshot(ctx context.Context) (NavSnapshot, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return NavSnapshot{}, err
	}
	return h.navSnapshot(w), nil
}

// SetFilter starts or clears a tree filter. Open unsaved text is searched.
func (h *documentHost) SetFilter(ctx context.Context, query string) (NavSearchResult, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return NavSearchResult{}, err
	}
	idx := h.indexOf(w)
	if idx == nil {
		return NavSearchResult{Query: strings.TrimSpace(query), Status: IndexIdle, Hits: []NavSearchHit{}}, nil
	}
	idx.mu.Lock()
	idx.query = strings.TrimSpace(query)
	idx.mu.Unlock()
	result := h.searchLockedResult(w, idx, query)
	_ = navSearchReady.Emit(w.win, result)
	return result, nil
}

// ToggleFolder collapses or expands a directory in the tree.
func (h *documentHost) ToggleFolder(ctx context.Context, rel string) error {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return err
	}
	idx := h.indexOf(w)
	if idx == nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	idx.mu.Lock()
	idx.collapsed[rel] = !idx.collapsed[rel]
	idx.mu.Unlock()
	h.emitNav(w)
	return nil
}

// RevealPath expands ancestors so path is visible in the tree.
func (h *documentHost) RevealPath(ctx context.Context, path string) error {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return err
	}
	idx := h.indexOf(w)
	if idx == nil {
		return nil
	}
	rel := relSlash(idx.root, path)
	idx.mu.Lock()
	for p := parentSlash(rel); p != ""; p = parentSlash(p) {
		delete(idx.collapsed, p)
	}
	idx.mu.Unlock()
	h.emitNav(w)
	return nil
}

// PickFolder opens a native directory dialog and focuses or opens that workspace.
func (h *documentHost) PickFolder(ctx context.Context) (string, error) {
	w, _ := h.workspaceOf(ctx)
	var parent *mygo.Window
	if w != nil {
		parent = w.win
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      parent,
		Title:       "Open Folder",
		ButtonLabel: "Open",
		Directory:   true,
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", mygo.ErrCanceled
	}
	root := paths[0]
	if err := h.OpenWorkspace(ctx, root); err != nil {
		return "", err
	}
	return root, nil
}

// RecentWorkspaces returns persisted folder workspaces, newest first.
func (h *documentHost) RecentWorkspaces(ctx context.Context) ([]RecentWorkspace, error) {
	_ = ctx
	return h.settings.recentWorkspaces(), nil
}

func (h *documentHost) rememberRoot(root string) {
	if root == "" {
		return
	}
	_ = h.settings.rememberWorkspace(root)
}

func (h *documentHost) forgetIndex(w *workspace) {
	if w == nil || w.index == nil {
		return
	}
	w.index.mu.Lock()
	w.index.stopLocked()
	w.index.mu.Unlock()
	w.index = nil
}

func (h *documentHost) reindexOpenDocument(s *docSession) {
	if s == nil || s.ws == nil {
		return
	}
	idx := h.indexOf(s.ws)
	if idx == nil {
		return
	}
	path := s.currentPath()
	rel := relSlash(idx.root, path)
	open := h.openTexts(s.ws)
	idx.mu.Lock()
	file := idx.files[rel]
	if file == nil && path != "" && folderContains(idx.root, path) {
		file = &indexedFile{path: path, name: filepath.Base(path), parent: parentSlash(rel)}
		idx.files[rel] = file
	}
	if file != nil {
		h.indexOne(file, open)
	}
	idx.mu.Unlock()
	h.emitNav(s.ws)
	h.emitSearch(s.ws, idx)
}

func (h *documentHost) showWelcomeIfEmpty() {
	h.mu.Lock()
	n := len(h.byWindow)
	h.mu.Unlock()
	if n > 0 {
		return
	}
	win := h.newWorkspaceWindow("", "Welcome")
	win.Show()
}

func (h *documentHost) waitIndexReady(w *workspace, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		idx := h.indexOf(w)
		if idx == nil {
			return
		}
		idx.mu.Lock()
		status := idx.status
		idx.mu.Unlock()
		if status == IndexReady || status == IndexError {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func breadcrumbsFor(root, path string) []Crumb {
	if path == "" {
		return nil
	}
	abs := canonicalPath(path)
	base := root
	if base == "" {
		base = filepath.Dir(abs)
	} else {
		base = canonicalPath(base)
	}
	var crumbs []Crumb
	cur := abs
	for {
		name := filepath.Base(cur)
		if name == "" || name == "." || name == string(filepath.Separator) {
			break
		}
		crumbs = append([]Crumb{{Name: name, Path: cur, IsFile: cur == abs}}, crumbs...)
		if cur == base {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return crumbs
}

func (st *settingsStore) recentWorkspaces() []RecentWorkspace {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.recentLocked()
}

func (st *settingsStore) recentLocked() []RecentWorkspace {
	if st.dir == "" {
		return nil
	}
	data, err := os.ReadFile(st.recentPath())
	if err != nil {
		return nil
	}
	var list []RecentWorkspace
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	out := list[:0]
	for _, item := range list {
		if item.Root == "" {
			continue
		}
		if info, err := os.Stat(item.Root); err != nil || !info.IsDir() {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (st *settingsStore) recentPath() string {
	return filepath.Join(st.dir, "recent-workspaces.json")
}

func (st *settingsStore) rememberWorkspace(root string) error {
	root = canonicalPath(root)
	if root == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	list := st.recentLocked()
	next := []RecentWorkspace{{Root: root, Name: filepath.Base(root), OpenedAt: time.Now().UnixMilli()}}
	for _, item := range list {
		if sameRoot(item.Root, root) {
			continue
		}
		next = append(next, item)
		if len(next) >= maxRecentWorkspaces {
			break
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(st.recentPath(), append(data, '\n'))
}
