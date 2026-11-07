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
- `DocumentsOptions.Command` — if set and returns true, documentRole skips default Save/SaveAs/Duplicate/Rename/Move/Revert/Close. The document is the one active at click time.
- `DocumentsOptions.BeforeWindowClose` — if set and returns true, the window close is prevented so the app can Flush then CloseWorkspace.

`DocumentsOptions.Window` may return an existing window. `attach` appends to that window's `docGroup` instead of overwriting a single mapping. `documentFor(w)` returns the active document.

`Document.Close()`, `File > Close`, and Cmd+W close the active document (`CloseDocument`), not the window. The window close button asks every document in the group without unregistering any; cancel keeps the window and every remaining tab, including ones that already saved.

`notify` writes title, represented file, and edited mark only when the document is active. Background documents still fire `OnStateChange`.

Persistence, recovery copies, conflict, and file watches are unchanged.
