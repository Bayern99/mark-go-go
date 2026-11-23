package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Bayern99/mark-go-go/internal/desktop"
	"github.com/Bayern99/mark-go-go/internal/fsutil"
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

type FolderState string

const (
	FolderOpen      FolderState = "open"
	FolderCollapsed FolderState = "collapsed"
	FolderEmpty     FolderState = "empty"
)

type IndexStatus string

const (
	IndexIdle     IndexStatus = "idle"
	IndexScanning IndexStatus = "scanning"
	IndexIndexing IndexStatus = "indexing"
	IndexReady    IndexStatus = "ready"
	IndexError    IndexStatus = "error"
)

type NavEntry struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
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

type NavSnapshot struct {
	Root           string      `json:"root"`
	Status         IndexStatus `json:"status"`
	Indexed        int         `json:"indexed"`
	Total          int         `json:"total"`
	Error          string      `json:"error,omitempty"`
	Entries        []NavEntry  `json:"entries"`
	CollapsedCount int         `json:"collapsedCount"`
}

type HeadingHit struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Level  int    `json:"level"`
}

type NavSearchHit struct {
	Path       string       `json:"path"`
	Name       string       `json:"name"`
	Title      string       `json:"title"`
	Excerpt    string       `json:"excerpt"`
	DocumentID string       `json:"documentId,omitempty"`
	Headings   []HeadingHit `json:"headings"`
}

type NavSearchResult struct {
	Root       string         `json:"root,omitempty"`
	Query      string         `json:"query"`
	Status     IndexStatus    `json:"status"`
	Matched    int            `json:"matched"`
	Indexed    int            `json:"indexed"`
	Total      int            `json:"total"`
	Incomplete bool           `json:"incomplete"`
	Hits       []NavSearchHit `json:"hits"`
}

type RecentWorkspace struct {
	Root     string `json:"root"`
	Name     string `json:"name"`
	OpenedAt int64  `json:"openedAt"`
}

type Crumb struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	IsFile bool   `json:"isFile"`
}

var (
	NavigationChanged = desktop.Event[NavSnapshot]{}
	SearchReady       = desktop.Event[NavSearchResult]{}
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
	order     []string
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

func relSlash(root, path string) string {
	rel, err := filepath.Rel(fsutil.CanonicalPath(root), fsutil.CanonicalPath(path))
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

func (h *Service) indexOf(w *workspace) *workspaceIndex {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.indexOfLocked(w)
}

func (h *Service) indexOfLocked(w *workspace) *workspaceIndex {
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

func (h *Service) startIndex(w *workspace) {
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

func (h *Service) scanIndex(ctx context.Context, w *workspace, idx *workspaceIndex, gen int) {
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
		if path != root && d.IsDir() && fsutil.SkipDirName(name) {
			return fs.SkipDir
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel != "" {
				ensureFolder(rel)
			}
			return nil
		}
		if !fsutil.MarkdownName(name) {
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

func (h *Service) publishPartial(w *workspace, idx *workspaceIndex, gen int, files map[string]*indexedFile, folders map[string]*indexedFolder, status IndexStatus) {
	clonedFiles := make(map[string]*indexedFile, len(files))
	for k, f := range files {
		cp := *f
		clonedFiles[k] = &cp
	}
	clonedFolders := make(map[string]*indexedFolder, len(folders))
	for k, f := range folders {
		cp := *f
		cp.children = append([]string(nil), f.children...)
		clonedFolders[k] = &cp
	}
	idx.mu.Lock()
	if idx.gen != gen {
		idx.mu.Unlock()
		return
	}
	idx.files = clonedFiles
	idx.folders = clonedFolders
	idx.recountLocked()
	idx.status = status
	idx.mu.Unlock()
	h.emitNav(w)
}

type indexBatchItem struct {
	rel  string
	path string
}

type indexedFileUpdate struct {
	rel      string
	modTime  int64
	title    string
	excerpt  string
	notes    int
	headings []NavHeading
	missing  bool
	ok       bool
}

func (h *Service) indexPending(ctx context.Context, w *workspace, idx *workspaceIndex, gen int) {
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
		var batch []indexBatchItem
		for rel, file := range idx.files {
			if file.indexed || file.missing {
				continue
			}
			batch = append(batch, indexBatchItem{rel: rel, path: file.path})
			if len(batch) >= indexBatchSize {
				break
			}
		}
		idx.mu.Unlock()
		if len(batch) == 0 {
			return
		}
		open := h.openTexts(w)
		updates := make([]indexedFileUpdate, 0, len(batch))
		for _, item := range batch {
			select {
			case <-ctx.Done():
				return
			default:
			}
			updates = append(updates, readIndexedFile(item.rel, item.path, open))
		}
		idx.mu.Lock()
		if idx.gen != gen {
			idx.mu.Unlock()
			return
		}
		for _, upd := range updates {
			if file := idx.files[upd.rel]; file != nil {
				applyIndexedFileUpdate(file, upd)
			}
		}
		idx.mu.Unlock()
		h.emitNav(w)
		h.emitSearch(w, idx)
	}
}

func (h *Service) openTexts(w *workspace) map[string]string {
	type docPath struct {
		path string
		text string
	}
	var open []docPath
	h.mu.Lock()
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil || s.doc == nil {
			continue
		}
		p := s.currentPath()
		if p == "" {
			continue
		}
		open = append(open, docPath{path: p, text: s.doc.Content()})
	}
	h.mu.Unlock()
	out := make(map[string]string, len(open))
	for _, item := range open {
		out[fsutil.CanonicalPath(item.path)] = item.text
	}
	return out
}

func readIndexedFile(rel, path string, open map[string]string) indexedFileUpdate {
	upd := indexedFileUpdate{rel: rel}
	text, ok := open[fsutil.CanonicalPath(path)]
	if !ok {
		data, err := readCapped(path, maxIndexBytes)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				upd.missing = true
			}
			return upd
		}
		text = string(data)
		if info, err := os.Stat(path); err == nil {
			upd.modTime = info.ModTime().UnixMilli()
		}
	}
	meta := parseMarkdownMeta(text)
	upd.title = meta.Title
	upd.excerpt = meta.Excerpt
	upd.notes = meta.Notes
	upd.headings = meta.Headings
	upd.ok = true
	return upd
}

func (h *Service) indexOne(file *indexedFile, open map[string]string) {
	applyIndexedFileUpdate(file, readIndexedFile("", file.path, open))
}

func applyIndexedFileUpdate(file *indexedFile, upd indexedFileUpdate) {
	if !upd.ok {
		if upd.missing {
			file.missing = true
		}
		file.indexed = true
		return
	}
	if upd.modTime != 0 {
		file.modTime = upd.modTime
	}
	file.title = upd.title
	file.excerpt = upd.excerpt
	file.notes = upd.notes
	file.headings = upd.headings
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

func (h *Service) watchIndex(w *workspace, idx *workspaceIndex, gen int) {
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
				return fsutil.SkipDirName(base)
			}
			return !fsutil.MarkdownName(base)
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

func (h *Service) applyWatch(w *workspace, idx *workspaceIndex, evts []mygo.FileEvent) {
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
			if !fsutil.MarkdownName(filepath.Base(rel)) {
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
	var pending []indexBatchItem
	if changed {
		idx.recountLocked()
		for rel, file := range idx.files {
			if !file.indexed && !file.missing {
				pending = append(pending, indexBatchItem{rel: rel, path: file.path})
			}
		}
	}
	idx.mu.Unlock()
	if !changed {
		return
	}
	if len(pending) > 0 {
		open := h.openTexts(w)
		updates := make([]indexedFileUpdate, 0, len(pending))
		for _, item := range pending {
			updates = append(updates, readIndexedFile(item.rel, item.path, open))
		}
		idx.mu.Lock()
		for _, upd := range updates {
			if file := idx.files[upd.rel]; file != nil {
				applyIndexedFileUpdate(file, upd)
			}
		}
		idx.mu.Unlock()
	}
	h.emitNav(w)
	h.emitSearch(w, idx)
}

func (h *Service) emitNav(w *workspace) {
	if w == nil || w.win == nil || w.win.IsDestroyed() {
		return
	}
	h.mu.Lock()
	selected := h.byWindow[w.win] == w
	h.mu.Unlock()
	if !selected {
		return
	}
	emit := h.navigationChanged
	if emit == nil {
		emit = NavigationChanged.Emit
	}
	_ = emit(w.win, h.navSnapshot(w))
}

func (h *Service) emitSearch(w *workspace, idx *workspaceIndex) {
	if w == nil || w.win == nil || w.win.IsDestroyed() || idx == nil {
		return
	}
	h.mu.Lock()
	selected := h.byWindow[w.win] == w
	h.mu.Unlock()
	if !selected {
		return
	}
	idx.mu.Lock()
	query := idx.query
	idx.mu.Unlock()
	emit := h.searchReady
	if emit == nil {
		emit = SearchReady.Emit
	}
	_ = emit(w.win, h.searchLockedResult(w, idx, query))
}

func (h *Service) navSnapshot(w *workspace) NavSnapshot {
	idx := h.indexOf(w)
	if idx == nil {
		return NavSnapshot{Root: w.root, Status: IndexIdle, Entries: []NavEntry{}}
	}
	openByRel := map[string]*docSession{}
	extraTabs := []string{}
	extraInfo := map[string]DocumentInfo{}
	extraRel := map[string]string{}
	h.mu.Lock()
	extraTabs = append(extraTabs, w.tabs...)
	for _, id := range w.tabs {
		s := h.byID[id]
		if s == nil {
			continue
		}
		extraInfo[id] = s.infoLocked()
	}
	h.mu.Unlock()
	for _, id := range extraTabs {
		info, ok := extraInfo[id]
		if !ok || info.Path == "" {
			continue
		}
		if rel := relSlash(idx.root, info.Path); rel != "" {
			if s, err := h.docSession(id); err == nil {
				openByRel[rel] = s
			}
			if fsutil.FolderContains(idx.root, info.Path) {
				extraRel[id] = rel
			}
		}
	}

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

	for _, id := range extraTabs {
		if seenOpen[id] {
			continue
		}
		snap, ok := extraInfo[id]
		if !ok {
			continue
		}
		rel, ok := extraRel[id]
		if !ok {
			continue
		}
		p := snap.Path
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

type navSortEntry struct {
	entry NavEntry
	rel   string
}

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

func (h *Service) searchLockedResult(w *workspace, idx *workspaceIndex, query string) NavSearchResult {
	openByRel := map[string]string{}
	for p, text := range h.openTexts(w) {
		if rel := relSlash(idx.root, p); rel != "" {
			openByRel[rel] = text
		}
	}
	type sessionPath struct {
		id   string
		path string
	}
	var sessionPaths []sessionPath
	h.mu.Lock()
	for _, s := range h.byID {
		if p := s.currentPath(); p != "" {
			sessionPaths = append(sessionPaths, sessionPath{id: s.id, path: p})
		}
	}
	h.mu.Unlock()
	openIDsByRel := make(map[string]string, len(sessionPaths))
	for _, sp := range sessionPaths {
		if rel := relSlash(idx.root, sp.path); rel != "" {
			openIDsByRel[rel] = sp.id
		}
	}

	idx.mu.Lock()
	indexed, total := 0, 0
	type searchFile struct {
		rel  string
		file indexedFile
	}
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
		Root:       w.root,
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

func (h *Service) NavSnapshot(ctx context.Context) (NavSnapshot, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return NavSnapshot{}, err
	}
	return h.navSnapshot(w), nil
}

func (h *Service) SetFilter(ctx context.Context, query string) (NavSearchResult, error) {
	w, err := h.workspaceOf(ctx)
	if err != nil {
		return NavSearchResult{}, err
	}
	idx := h.indexOf(w)
	if idx == nil {
		return NavSearchResult{Root: w.root, Query: strings.TrimSpace(query), Status: IndexIdle, Hits: []NavSearchHit{}}, nil
	}
	idx.mu.Lock()
	idx.query = strings.TrimSpace(query)
	idx.mu.Unlock()
	result := h.searchLockedResult(w, idx, query)
	_ = SearchReady.Emit(w.win, result)
	return result, nil
}

func (h *Service) ToggleFolder(ctx context.Context, rel string) error {
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

func (h *Service) RevealPath(ctx context.Context, path string) error {
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

func (h *Service) PickFolder(ctx context.Context) (string, error) {
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

func (h *Service) RecentWorkspaces(ctx context.Context) ([]RecentWorkspace, error) {
	_ = ctx
	return h.settings.recentWorkspaces(), nil
}

func (h *Service) rememberRoot(root string) {
	if root == "" {
		return
	}
	_ = h.settings.rememberWorkspace(root)
}

func (h *Service) forgetIndex(w *workspace) {
	if w == nil {
		return
	}
	h.mu.Lock()
	idx := w.index
	w.index = nil
	h.mu.Unlock()
	if idx == nil {
		return
	}
	idx.mu.Lock()
	idx.stopLocked()
	idx.mu.Unlock()
}

func (h *Service) reindexOpenDocument(s *docSession) {
	if s == nil {
		return
	}
	h.mu.Lock()
	w := s.ws
	h.mu.Unlock()
	if w == nil {
		return
	}
	idx := h.indexOf(w)
	if idx == nil {
		return
	}
	path := s.currentPath()
	rel := relSlash(idx.root, path)
	open := h.openTexts(w)
	upd := readIndexedFile(rel, path, open)
	inFolder := path != "" && fsutil.FolderContains(idx.root, path)
	idx.mu.Lock()
	file := idx.files[rel]
	if file == nil && inFolder {
		file = &indexedFile{path: path, name: filepath.Base(path), parent: parentSlash(rel)}
		idx.files[rel] = file
	}
	if file != nil {
		applyIndexedFileUpdate(file, upd)
	}
	idx.mu.Unlock()
	h.emitNav(w)
	h.emitSearch(w, idx)
}

func (h *Service) showWelcomeIfEmpty() {
	h.mu.Lock()
	n := len(h.byWindow)
	h.mu.Unlock()
	if n > 0 {
		return
	}
	win := h.newWorkspaceWindow("", "Welcome")
	win.Show()
}

func (h *Service) waitIndexReady(w *workspace, timeout time.Duration) {
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

func Breadcrumbs(root, path string) []Crumb {
	if path == "" {
		return nil
	}
	abs := fsutil.CanonicalPath(path)
	base := root
	if base == "" {
		base = filepath.Dir(abs)
	} else {
		base = fsutil.CanonicalPath(base)
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
	root = fsutil.CanonicalPath(root)
	if root == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	list := st.recentLocked()
	next := []RecentWorkspace{{Root: root, Name: filepath.Base(root), OpenedAt: time.Now().UnixMilli()}}
	for _, item := range list {
		if fsutil.SameRoot(item.Root, root) {
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
	return fsutil.WriteFileAtomic(st.recentPath(), append(data, '\n'))
}
