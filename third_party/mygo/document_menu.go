package mygo

import (
	"errors"
	"log"
	"path/filepath"
)

// hasDocuments reports whether the app made documents (NewDocuments),
// which the File menu then has commands for.
func hasDocuments() bool { return defaultDocKind() != nil }

func documentRole(role MenuRole, win *Window) {
	k := defaultDocKind()
	if k == nil {
		return
	}
	switch role {
	case RoleNew:
		go k.newUntitled()
		return
	case RoleOpen:
		go openDocumentsDialog()
		return
	}
	b, cleared := documentSelection(win)
	if b == nil && !cleared {
		focused := FocusedWindow()
		b, cleared = documentSelection(focused)
		if b != nil {
			win = focused
		}
	}
	if cleared {
		return
	}
	if b == nil {
		// Close with no document in the window closes that window, as the
		// item's own performClose did before the group took the role over:
		// BeforeWindowClose still turns it into closing the workspace.
		if role == RoleClose {
			w := win
			if w == nil {
				w = FocusedWindow()
			}
			if w != nil {
				w.Close()
			}
		}
		return
	}
	captured := b
	if captured.kind != nil && captured.kind.command != nil && captured.kind.command(role, captured, win) {
		return
	}
	go func() {
		var err error
		switch role {
		case RoleSave:
			err = captured.saveInteractive(false)
		case RoleSaveAs:
			err = captured.saveInteractive(true)
		case RoleDuplicate:
			captured.duplicate()
		case RoleRename:
			err = captured.rename("")
		case RoleMoveTo:
			err = captured.moveTo("")
		case RoleRevert:
			err = captured.revert()
		case RoleClose:
			err = captured.closeDocument()
		}
		if err != nil && !errors.Is(err, ErrCanceled) {
			what := map[MenuRole]string{RoleRename: "renamed", RoleMoveTo: "moved", RoleRevert: "reverted"}[role]
			showDocumentError(captured.parent(), menuMessages().DocumentFailed(what, captured.Name()), err)
		}
	}()
}

// openDocumentsDialog asks which documents to open, of every kind.
func openDocumentsDialog() {
	var filters []FileFilter
	for _, k := range docKinds() {
		filters = append(filters, k.filters()...)
	}
	paths, err := Dialog.Open(OpenDialogOptions{Parent: FocusedWindow(), Filters: filters, Multiple: true})
	if err != nil {
		return
	}
	for _, p := range paths {
		openDocumentPath(p)
	}
}

// openDocumentPath opens path with the kind of its extension, or the
// first kind.
func openDocumentPath(path string) {
	kind := defaultDocKind()
	for _, k := range docKinds() {
		if k.handles(path) {
			kind = k
			break
		}
	}
	if kind == nil {
		return
	}
	if _, err := kind.open(path); err != nil {
		showDocumentError(nil, menuMessages().DocumentFailed("opened", filepath.Base(path)), err)
	}
}

// RefreshDocumentMenus updates the File menu's document roles from the
// focused window's document, on the main thread, now or at its next
// turn. Call it from any goroutine when a state outside MyGo, as
// DocumentsOptions.PathChange reads, changes what those roles can do.
func RefreshDocumentMenus() { postMain(refreshDocumentMenus) }

// canChangePath reports whether the app allows b to write to another
// file or move its file now.
func (b *docBase) canChangePath() bool {
	k := b.kind
	return k == nil || k.pathChange == nil || k.pathChange(b) == nil
}

// refreshDocumentMenus enables the document roles of the menus for the
// focused window's document. Main thread only.
func refreshDocumentMenus() {
	if !App.IsReady() {
		return
	}
	b := documentFor(FocusedWindow())
	var s DocumentState
	if b != nil {
		s = b.State()
	}
	enable := func(it *MenuItem) {
		var on bool
		switch it.Role {
		case RoleSave, RoleDuplicate, RoleClose:
			on = b != nil
		case RoleSaveAs, RoleRename, RoleMoveTo:
			on = b != nil && b.canChangePath()
		case RoleRevert:
			on = b != nil && s.Edited && s.Path != ""
		default:
			return
		}
		if it.IsEnabled() != on {
			it.SetEnabled(on)
		}
	}
	menus := []*Menu{App.Menu()}
	for _, w := range Windows() {
		menus = append(menus, w.menu)
	}
	for _, m := range menus {
		walkMenu(m, enable)
	}
}

// walkMenu calls fn with every item of m and its submenus.
func walkMenu(m *Menu, fn func(*MenuItem)) {
	if m == nil {
		return
	}
	for _, it := range m.Items() {
		fn(it)
		menuMu.Lock()
		sub := it.sub
		menuMu.Unlock()
		walkMenu(sub, fn)
	}
}

// recentMenuItems are the items of the Open Recent submenu.
func recentMenuItems() []*MenuItem {
	var items []*MenuItem
	for _, p := range App.RecentDocuments() {
		items = append(items, &MenuItem{Label: filepath.Base(p), ToolTip: p, Click: func(*MenuItem, *Window) {
			openRecentDocument(p)
		}})
	}
	if len(items) > 0 {
		items = append(items, Separator())
	}
	return append(items, &MenuItem{Label: menuMessages().MenuClearRecent(), Disabled: len(items) == 0, Click: func(*MenuItem, *Window) {
		go func() {
			if err := App.ClearRecentDocuments(); err != nil {
				log.Printf("mygo: recent documents: %v", err)
			}
		}()
	}})
}

// updateRecentMenus gives the Open Recent submenus the recent documents.
// Main thread only.
func updateRecentMenus() {
	if !App.IsReady() {
		return
	}
	changed := false
	menus := []*Menu{App.Menu()}
	for _, w := range Windows() {
		menus = append(menus, w.menu)
	}
	for _, m := range menus {
		walkMenu(m, func(it *MenuItem) {
			if it.Role != RoleOpenRecent {
				return
			}
			items := recentMenuItems()
			menuMu.Lock()
			sub := &Menu{}
			for _, child := range items {
				sub.items = append(sub.items, prepareItem(child, sub))
			}
			it.Submenu, it.sub = items, sub
			menuMu.Unlock()
			changed = true
		})
	}
	if changed {
		refreshMenus()
	}
}

// The events of a document's pages: its state, and its content read
// again. Their names are MyGo's own.
const (
	documentStateEvent  = "mygo:document-state"
	documentReloadEvent = "mygo:document-reload"
)

// emitDocumentState tells w's page the state of its document. Paths are
// none of untrusted pages' business. Main thread only.
func emitDocumentState(w *Window, s DocumentState) {
	if w.trusted {
		if msg, err := encodeEvent(documentStateEvent, s); err == nil {
			w.enqueue(msg, true)
		}
	}
}

// emitDocumentReload tells w's page that its document's content was read
// again. Main thread only.
func emitDocumentReload(w *Window) {
	if w.trusted {
		if msg, err := encodeEvent(documentReloadEvent, struct{}{}); err == nil {
			w.enqueue(msg, true)
		}
	}
}
