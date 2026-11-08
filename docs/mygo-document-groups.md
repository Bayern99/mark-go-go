# MyGo document groups (controlled patch)

Pinned upstream: `github.com/egoist/mygo v0.7.4`.
In-tree copy: `third_party/mygo`.
`go.mod`: `replace github.com/egoist/mygo => ./third_party/mygo`.

## Why

Upstream binds one document to one window (`docRegistry.byWindow map[*Window]*docBase`). Returning the same window from `DocumentsOptions.Window` overwrites that map. Hidden extra windows still own menus and save sheets. Spec #20 / ADR 0005 require a real document group: several documents in one window, one active document, shared close/save routing.

## Reproducing the copy

From a machine that has the module cached:

    rsync -a "$(go env GOMODCACHE)/github.com/egoist/mygo@v0.7.4/" third_party/mygo-upstream/

Then keep the document-group edits that live in this tree under `third_party/mygo`. Do not `go get` a newer mygo and claim groups work. v0.8.x is still one document per window.

## Public API added

- `Document.ID() string` — stable identity, same value as the recovery id, independent of path and window.
- `Documents.ForWindowAll(w) []*Document[T]` — every document in the window, tab order.
- `Documents.SetActive(d)` — makes `d` the active document of its window.
- `Documents.Attach(d, w)` — `d` leaves any previous group and joins `w`; becomes active.
- `Documents.Detach(d)` — `d` leaves its window without closing; the window stays.
- `Document.ConfirmClose() error` — ask-to-save this document only; does not detach or finishClose. `ErrCanceled` on cancel.
- `Document.CloseDocument() error` — ConfirmClose then unregister this document; does not destroy a shared window.
- `Document.OnClosed(fn) (off func())` — fired after the document is actually closed.
- `DocumentsOption.Command` — if set and returns true, documentRole skips default Save/SaveAs/Duplicate/Rename/Move/Revert/Close and Undo/Redo (the web view's empty undo manager). The document is the one active at click time.
- `DocumentsOption.PathChange(doc)` — if set, MyGo asks it before a document writes to another file or moves its file: an error refuses that change with that error before any file is touched, and MyGo disables Save As, Rename and Move To for that document while it stands. Ordinary Save (the document's own file), Duplicate, editing and tab switches stay available. It may be called from any goroutine; keep it short and free of dialogs.
- `DocumentsOption.ReservePath(doc, target) (release func(), err error)` — if set, called with the file a document is about to become, before MyGo writes to it or moves the document's file there. An error refuses the operation before anything touches the disk, and MyGo asks no overwrite question about a target refused this way. The returned `release`, when non-nil, runs once after the file operation ends, successful or not. It may be called from any goroutine; keep it short and free of dialogs.
- `RefreshDocumentMenus()` — posts to the main thread (non-blocking) a refresh of the File menu's document roles from the focused window's document. Call it from any goroutine when an outside state that `PathChange` reads changes what Save As / Rename / Move To can do.
- `DocumentsOptions.BeforeWindowClose` — if set and returns true, the window close is prevented so the app can Flush then CloseWorkspace.
- `DocumentsOptions.AfterStart` — if set, runs after recovery copies and launch files are opened, and before an untitled document would be created. #23 restores workspace layout there.

`DocumentsOptions.Window` may return an existing window. `attach` appends to that window's `docGroup` instead of overwriting a single mapping. `documentFor(w)` returns the active document.

`Document.Close()`, `File > Close`, and Cmd+W close the active document (`CloseDocument`), not the window. The window close button asks every document in the group without unregistering any; cancel keeps the window and every remaining tab, including ones that already saved.

`notify` writes title, represented file, and edited mark only when the document is active. Background documents still fire `OnStateChange`.

Persistence, recovery copies, conflict, and file watches are unchanged.

Undo/Redo of a document window are the page editor's history. `performRole` sends those roles through `DocumentsOptions.Command` instead of `document.execCommand`. On macOS, `MyGoWebView` implements `undo:`/`redo:` so a click or key equivalent on the web view reaches that intercept; an AppKit field that is not a descendant of the web view keeps the system undo manager. `SetEnabled` on the Edit menu items reports the page's real `canUndo`/`canRedo`.
