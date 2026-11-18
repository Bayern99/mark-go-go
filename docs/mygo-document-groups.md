# MyGo document groups (controlled patch)

Pinned upstream: `github.com/egoist/mygo v0.7.4`.
In-tree copy: `third_party/mygo`.
`go.mod`: `replace github.com/egoist/mygo => ./third_party/mygo`.

## Why

Upstream binds one document to one window (`docRegistry.byWindow map[*Window]*docBase`). Returning the same window from `DocumentsOptions.Window` overwrites that map. Hidden extra windows still own menus and save sheets. ADR 0005 requires a real document group: several documents and folder roots in one window, one active document, shared close/save routing.

## Reproducing the copy

From a machine that has the module cached:

    rsync -a "$(go env GOMODCACHE)/github.com/egoist/mygo@v0.7.4/" third_party/mygo-upstream/

Then keep the document-group edits that live in this tree under `third_party/mygo`. Do not `go get` a newer mygo and claim groups work. v0.8.x is still one document per window.

## Public API added

- `Document.ID() string` — stable identity, same value as the recovery id, independent of path and window.
- `Documents.ForWindowAll(w) []*Document[T]` — every document in the window, tab order.
- `Documents.SetActive(d)` — makes `d` the active document of its window.
- `Documents.ClearActive(w, title)` — selects an empty view, clears the represented file and edited mark, sets the window title, and disables File commands that require an active document. The window's other documents stay open.
- `Documents.Attach(d, w)` — `d` leaves any previous group and joins `w`; becomes active.
- `Documents.Detach(d)` — `d` leaves its window without closing; the window stays.
- `Document.ConfirmClose() error` — ask-to-save this document only; does not detach or finishClose. `ErrCanceled` on cancel.
- `Document.CloseDocument() error` — ConfirmClose then unregister this document; does not destroy a shared window.
- `Documents.CloseDocumentGroup(w) error` — confirms the documents together and closes the window. The group must still contain the same documents when confirmation finishes. A canceled attempt keeps the tabs and their unsaved recovery copies.
- `Documents.CloseDocuments(docs) error` — confirms exactly the specified documents as one group, then closes only them without destroying their shared window. Other tabs stay open; cancellation keeps every target open. Used by multi-root `CloseWorkspaceRoot`.
- `Document.OnClosed(fn) (off func())` — fired after the document is actually closed.
- `DocumentsOption.Command` — if set and returns true, documentRole skips default Save/SaveAs/Duplicate/Rename/Move/Revert/Close and Undo/Redo (the web view's empty undo manager). The document is the one active at click time.
- `DocumentsOption.DocumentFocused` — called after MyGo's direct duplicate-open path focuses an existing document. The application uses it to select the same `documentId` in the web tab group; without it, native chrome and the page could disagree because that path never calls `DocumentsOptions.Window`.
- Doc-aware `RoleClose`: when documents exist, the menu snapshot clears the platform role for a `RoleClose` item so a press reaches `performRole(RoleClose)`. The document role asks `DocumentsOptions.Command` (the app's `nativeCommand "close"` for the captured document). `ClearActive` suppresses even a stale command during a menu refresh. In the absence of a document or an explicit empty selection, the Go fallback resolves and closes the given or focused window. `BeforeWindowClose` still intercepts that close. A folder window with no tabs stays. Closing the last document in a standalone untitled window destroys that window. Apps without document support, the window close button, and explicit `CloseWorkspace` keep whole-window behaviour.
- `DocumentsOption.PathChange(doc)` — if set, MyGo asks it before a document writes to another file or moves its file: an error refuses that change with that error before any file is touched, and MyGo disables Save As, Rename and Move To for that document while it stands. Ordinary Save (the document's own file), Duplicate, editing and tab switches stay available. It may be called from any goroutine; keep it short and free of dialogs.
- `DocumentsOption.ReservePath(doc, target) (release func(), err error)` — if set, called with the file a document is about to become, after the file/folder picker returns a target and before MyGo writes to it or moves the document's file there. An error refuses the operation before anything touches the disk, and MyGo asks no further overwrite question of its own (its post-picker Move To dialog) about a target refused this way; the OS Save panel's built-in Replace confirmation during Save As / Rename selection happens outside this promise, and a target refused by the app is still never written. The returned `release`, when non-nil, runs once after the file operation ends, successful or not; **nil means there was no claim to release** (no hook, an empty or own-file target, or no live session), not a refusal. A duplicate is an untitled copy and is reserved only at its own first save. It may be called from any goroutine; keep it short and free of dialogs.
- `RefreshDocumentMenus()` — posts to the main thread (non-blocking) a refresh of the File menu's document roles from the focused window's document. Call it from any goroutine when an outside state that `PathChange` reads changes what Save As / Rename / Move To can do.
- `DocumentsOptions.BeforeWindowClose` — if set and returns true, the window close is prevented so the app can Flush then CloseWorkspace.
- `DocumentsOptions.AfterStart` — if set, runs after recovery copies and launch files are opened, and before an untitled document would be created. #23 restores workspace layout there.

`DocumentsOptions.Window` may return an existing window. `attach` appends to that window's `docGroup` instead of overwriting a single mapping. `documentFor(w)` returns the active document.

`Document.Close()`, `File > Close`, and Cmd+W close the active document via `CloseDocument`. An empty folder window stays, with File > Close disabled and Cmd+W ignored. Closing the last document in a standalone untitled window destroys that window. The window close button asks every document in the group without unregistering any. Cancel keeps the window and every remaining tab, including ones that already saved.

`notify` writes title, represented file, and edited mark only when the document is active. Background documents still fire `OnStateChange`.

The group applies Delete or Don't Save only after every document has confirmed. Saves that completed before a later cancellation remain on disk.

Undo/Redo of a document window are the page editor's history. `performRole` sends those roles through `DocumentsOptions.Command` instead of `document.execCommand`. On macOS, `MyGoWebView` implements `undo:`/`redo:` so a click or key equivalent on the web view reaches that intercept; an AppKit field that is not a descendant of the web view keeps the system undo manager. `SetEnabled` on the Edit menu items reports the page's real `canUndo`/`canRedo`.

For other menu items, `MyGoWebView` follows AppKit's validation order. It calls the WebKit superclass validator only if the superclass implements it and falls back to whether the target handles the action. [AppKit menu validation](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)
