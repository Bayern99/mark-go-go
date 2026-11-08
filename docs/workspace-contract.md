# Workspace and document identity contract (#21)

Slice-specific. Downstream tickets (#22, #24, #30) consume this as the public Go/TS seam. The consolidated, reader-facing reference is now `docs/interfaces.md` (#32); this file stays the normative seam contract for identities, routing and lifecycle, and the two must not diverge.

## Identities (distinct)

| Identity | Owner | Stable across |
|---|---|---|
| `documentId` | Go (`mygo.Document.ID()`) | path, tab, window, first save, Save As |
| `workspaceId` | Go (`workspaceHost`) | document tabs inside one window |
| `root` | folder path, empty string for an untitled window | not an identity; routing uses it |
| `path` | disk file, empty string while untitled | not an identity |
| `tab` | UI navigation item for one `documentId` in one window | not an identity |

A document adapter binds to one `documentId` for its lifetime. Asynchronous calls, events, view watches, and generation never retarget through the currently active tab.

## Go service `WorkspaceHost` (`mygo.BindAs("WorkspaceHost", host)`)

Document-level methods take `documentId`. Workspace-level methods act on the calling page's window.

```go
type WorkspaceSnapshot struct {
    ID           string         `json:"id"`
    Root         string         `json:"root"`     // empty = untitled window
    Name         string         `json:"name"`
    Kind         string         `json:"kind"`     // "folder" | "untitled"
    Documents    []DocumentInfo `json:"documents"` // tab order
    ActiveID     string         `json:"activeId"`  // empty when no tabs
    RestoreError string         `json:"restoreError,omitempty"`
}

type DocumentInfo struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Path         string `json:"path"`
    Edited       bool   `json:"edited"`
    Conflict     bool   `json:"conflict"`
    Deleted      bool   `json:"deleted"`
    ReadOnly     bool   `json:"readOnly"`
    Generating   bool   `json:"generating"` // this document's generation slot is occupied
    ViewMode     string `json:"viewMode,omitempty"`
    RestoreError string `json:"restoreError,omitempty"`
}

type Snapshot struct {
    ID      string       `json:"id"`
    Text    string       `json:"text"`
    Version int          `json:"version"`
    State   DocumentInfo `json:"state"`
}

type CloseResult struct {
    Closed   bool `json:"closed"`
    Canceled bool `json:"canceled"`
}

// Workspace-level
Snapshot(ctx) (WorkspaceSnapshot, error)
Open(ctx, path string) (DocumentInfo, error)          // global routing
OpenWorkspace(ctx, root string) error                 // reuse existing, else new window
New(ctx) (DocumentInfo, error)                        // untitled document in this window
Duplicate(ctx, documentId string) (DocumentInfo, error)
Activate(ctx, documentId string) error
CloseDocument(ctx, documentId string) (CloseResult, error)
CloseWorkspace(ctx) (CloseResult, error)

// Document-level (never uses the active tab)
Load(ctx, documentId string) (Snapshot, error)
Update(ctx, documentId, text string) error
Flush(ctx, documentId, text string) error             // last undebounced editor text before Save/Close
Save(ctx, documentId string) error
SaveAs(ctx, documentId string) error
Rename(ctx, documentId string) error
MoveTo(ctx, documentId string) error
Revert(ctx, documentId string) error
WatchView(ctx, documentId string) error
UnwatchView(ctx, documentId string) error
UndoView(ctx, documentId string) error
SetNativeEditState(ctx, documentId string, canUndo, canRedo bool) error  // editor undo, not UndoView
SetViewMode(ctx, documentId, mode string) error
PrepareQuit(ctx) error   // ack one window's flushQuit; persist after every window acks
Generate(ctx, documentId string, req GenerateRequest, progress *Channel[GenerateProgress]) error
GetSettings() (Settings, error)                  // process-wide store, not per document
SetSettings(p SettingsPatch) (Settings, error)   // writes only the patch's fields; returns the committed value

// Navigation (#22). Opening a workspace does not wait for a full directory scan.
NavSnapshot(ctx) (NavSnapshot, error)
SetFilter(ctx, query string) (NavSearchResult, error)
ToggleFolder(ctx, rel string) error
RevealPath(ctx, path string) error
PickFolder(ctx) (string, error)                 // native directory dialog; ErrCanceled if dismissed
RecentWorkspaces(ctx) ([]RecentWorkspace, error)
```

`Flush` is the page's contract for last-input delivery: the editor must send the current Markdown of `documentId` before Save/Close/SaveAs of that document. The host then uses that document's content, not the active tab.

Canceled native dialogs return `mygo.ErrCanceled` (Save/SaveAs/Rename/Move/Revert) or `CloseResult{Canceled: true}` (Close). The workspace and remaining tabs stay.

## Events (payloads include `id`)

Emitted to the workspace window that currently hosts the document.

```ts
workspaceChanged: WorkspaceSnapshot
docReloaded: Snapshot                 // { id, text, version, state }
docState: DocumentInfo                // { id, name, path, edited, conflict, deleted, readOnly, generating }
viewChanged: { id: string } & ViewSnapshot
navChanged: NavSnapshot               // incremental tree; status scanning|indexing|ready|error
navSearchReady: NavSearchResult       // incomplete is true until the index is ready
```

`workspaceChanged` is the source of truth for tab order, active tab, root, and per-tab badges. Background `docState` updates the matching tab without activating it.

`settingsChanged: Settings` is broadcast to every window, not to one workspace: see Settings below.

## Settings

The generation settings — model, agent command, and the models the agent offered on
its last ACP handshake — are one process-wide store (`settings.json` in
`PathUserData`), not a document's or a window's. `SetSettings` takes an intent
patch: only the fields that are present are written, an empty model means the
agent's own choice, an empty command restores the default, and the cached model
list is only ever written by the handshake. A successful write returns the
committed `Settings` and broadcasts `settingsChanged` to every window, so an idle
panel in another document shows the current global value without a remount. A
run captures the store once when it starts; later changes do not alter it.

## Routing (Open / Finder / menu)

1. If an open document is the same file (`path` equal, canonical path equal, or `os.SameFile`), activate it in its window and focus that window.
2. Else if one or more folder workspaces contain the file, open it as a tab in the workspace whose `root` is the deepest prefix.
3. Else open (or create) a folder workspace at the file's parent directory and add the tab there.

Opening a folder that is already a workspace root focuses that window. Parent and child folders may both be open; they still share one document session per file. Roots and files are compared after resolving existing symlink prefixes (`canonicalPath`), so aliases of the same directory — including macOS `/tmp` versus `/private/tmp` — reuse the same workspace and keep `documentId`.

Untitled window: `root == ""`, `kind == "untitled"`. It holds untitled documents. Closing its last tab closes the window. A folder workspace with zero tabs stays open.

## First save / Save As migration

After a successful first save or Save As:

- The document keeps its `documentId`, content, version, view watch, and generation slot.
- The tab leaves the source window and joins the destination workspace that contains the new path (deepest existing folder workspace, else a new window at the parent directory).
- Other untitled documents in an untitled window stay there.
- If the destination path is already another open document, the save is rejected: no silent overwrite, no second session.

## Native commands

File menu Save / Save As / Duplicate / Rename / Move / Revert act on the **active document of the focused window** (MyGo document group). `Cmd+W` / File > Close (`RoleClose`) closes the **captured active document** of the focused window: with documents, the patched menu delivers a `RoleClose` item to `performRole(RoleClose)` instead of the platform's `performClose`, so the document role asks the app's `Command` (`nativeCommand` `close` for that document). The window close button / File > Close Window runs `CloseWorkspace` (asks every dirty document; cancel keeps the window and every tab) and closes the whole group. A folder workspace with no tabs keeps its window: `RefreshDocumentMenus` disables File > Close there, so the native `Cmd+W` is a no-op rather than a window close. Closing a standalone untitled window's last document destroys that window. If the Go document role is invoked for `RoleClose` while it has no document (a window or menu race), it resolves the given or focused window and closes it; `BeforeWindowClose` still turns that into `CloseWorkspace`. That internal fallback is not a UI promise that `Cmd+W` on an empty folder window closes it. Apps without document support keep the platform `performClose` behaviour. Quit emits `flushQuit` to every workspace window; `PrepareQuit` writes `session.json` only after every window acks, then continues the quit. AskOnQuit stays unset so MyGo recovery copies keep unsaved bytes.

Edit menu Undo / Redo (`nativeCommand` `undo` / `redo`) capture that same active `documentId` at click time and apply the **editor** history of that document. They are not `UndoView`. A focused in-place Composer (`textarea` inside `.composer`) keeps native field undo; read-only GenUI / ComparisonTable focus does not rewrite the document. `SetNativeEditState(documentId, canUndo, canRedo)` reports real editor availability for the focused window's active document only, so a background pane cannot steal menu state. `DocumentsOptions.Equal` already clears the edited mark when Undo returns the on-disk bytes.

Dialogs use the workspace window as parent. Path changes are gated before the disk:

- `DocumentsOptions.PathChange(doc)` is the path-change lock: while the document's generation slot is occupied, Save As / Rename / Move To are disabled in the File menu and any attempt that still reaches the kernel is refused before a dialog or a write. Ordinary Save to its own file, Duplicate, editing and tab switching stay available. `RefreshDocumentMenus()` posts a main-thread, non-blocking menu refresh when that lock changes, so the roles reflect it without blocking a dialog.
- `DocumentsOptions.ReservePath(doc, target)` claims the target before MyGo writes to it or moves the document's file there. It runs after the file/folder picker returns the target and before any write, `os.Rename`, path change, or MyGo's own post-picker Move To overwrite dialog. The document's own file is always allowed; another file is refused while the document generates or while another live session has it, compared by canonical path and `os.SameFile` across the whole app. The claim is held until the returned release runs, on success or failure, so two windows cannot take one target. It does **not** promise to suppress the OS NSSavePanel's built-in Replace confirmation shown during Save As / Rename selection: that OS prompt can appear before the app's rejection, but a target the app refuses is never written and never becomes a second session. A nil release means there was no claim to release — no hook, an empty or own-file target, or no live session for the document — not a refusal. `Duplicate` creates an independent untitled copy and reserves nothing until that copy's own first save.
- A failed native command is recorded on `DocumentInfo.actionError` and shown as a banner in that document's window; success clears the previous message, and a canceled dialog is not a failure.
- UI scoping is part of this seam: a GenUI `.changed-list` jump/highlight queries only cards under that document's own mount, so a hidden tab cannot take it; each editor owns its composer lifecycle, so another document's load / reload / read-only / destroy never cancels this document's open Note or special-block composer or its unsaved draft.

## Session file (#23)

`session.json` in `PathUserData` is the layout this app restores, not a copy of unsaved bytes:

```json
{
  "workspaces": [
    {
      "id": "workspace id",
      "root": "/abs/folder or empty for an untitled window",
      "tabs": [{"id": "documentId", "path": "/abs/file.md", "viewMode": "genui"}],
      "activeId": "documentId"
    }
  ]
}
```

- Written on layout changes (open, tab move, activate, close, view mode) and once more at quit after every window acked `flushQuit`. Writes are skipped while recovery/restore runs so startup cannot overwrite the stored layout with half-restored windows.
- Only workspaces the user did not close are stored; an empty untitled window (Welcome) is not. Closing a tab or a workspace drops it, and cancel keeps the scene as it was.
- Unsaved text stays in MyGo's recovery copies, keyed by `documentId`; the session only names the tabs, their order, the active one and the view mode. Restore reuses a recovery document instead of opening its path again, and never starts a generation the user cancelled.
- A missing root or file becomes `WorkspaceSnapshot.restoreError` / `DocumentInfo.restoreError` and the other workspaces still restore. A file opened by this launch takes focus after the layout is placed.
- `flushQuit` / Save / Save As / Close ask each mounted pane for what it really holds: the text on screen (`editor.getMarkdown()`), else the core document state the host loaded. A pane that has not mounted an editor registers nothing, so a flush never turns an untouched, reloaded or remounted document into an empty buffer; a document the user did clear is delivered as `""` like any other edit. A document opened from disk gets a new MyGo id at restore, so the stored active tab is mapped to the id its document really has now.
- `bindFlush` only **registers** a pane's raw-text reader; it does not write. The page reads it and `mygoHost` issues exactly one `WorkspaceHost.Flush` RPC **per document in each entry call** (the public `Flush(documentId, text)` remains a real method). Save / Save As / Rename / Move / Revert / Duplicate deliver the captured document once and then run on the host; `close` (via `CloseDocument`) delivers that document once; `closeWorkspace` and `flushQuit` deliver **each document that has a registered reader (a mounted editor) once each**. A document with no registered reader writes nothing — never a synthesized empty buffer.
- Only a ready editor registers the reader; if an already-registered reader runs while its editor ref is briefly empty, it falls back to the host-loaded core text. A never-mounted pane registers nothing and never delivers `''`.

## MyGo document groups (controlled patch)

Pinned source: `github.com/egoist/mygo v0.7.4` vendored at `third_party/mygo` (`go.mod` replace). Patch adds:

- `docGroup` per window: ordered documents + active document. `documentFor(w)` returns the active document, never overwrites a single-window map with a later attach.
- `Documents.Attach(doc, win)`, `Documents.Detach(doc)`, `Documents.SetActive(doc)`, `Documents.ForWindowAll(w)`.
- `Document.ID()`, `Document.CloseDocument()` (asks to save that document only; does not destroy a shared window).
- Window close asks every document in the group; cancel leaves the group intact.
- `notify` updates window title / represented file / edited mark from the **active** document only.

Recovery, autosave, conflict, and disk watches remain MyGo's.

## TypeScript seams

```ts
interface WorkspaceAdapter {
  snapshot(): Promise<WorkspaceSnapshot>
  subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable
  open(path: string): Promise<DocumentInfo>
  openWorkspace(root: string): Promise<void>
  newDocument(): Promise<DocumentInfo>
  duplicate(id: string): Promise<DocumentInfo>
  activate(id: string): Promise<void>
  closeDocument(id: string, text?: string): Promise<CloseResult>
  closeWorkspace(): Promise<CloseResult>
  document(id: string): HostAdapter   // permanently bound to id
  navSnapshot(): Promise<NavSnapshot>
  subscribeNav(fn: (snap: NavSnapshot) => void): Disposable
  setFilter(query: string): Promise<NavSearchResult>
  subscribeSearch(fn: (result: NavSearchResult) => void): Disposable
  toggleFolder(rel: string): Promise<void>
  revealPath(path: string): Promise<void>
  pickFolder(): Promise<string>
  recentWorkspaces(): Promise<RecentWorkspace[]>
}

interface HostAdapter extends ViewFileSource, ViewGenerator {
  readonly id: string
  load(): Promise<HostDocSnapshot>        // HostDocSnapshot includes id
  update(text: string): Promise<void>
  flush(text: string): Promise<void>
  save(): Promise<void>
  saveAs(): Promise<void>
  rename(): Promise<void>
  moveTo(): Promise<void>
  revert(): Promise<void>
  setViewMode(mode: string): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocumentInfo) => void): Disposable
}
```

`createMyGoHost()` returns `WorkspaceAdapter`. `createBrowserHost(...)` still implements `HostAdapter` for a single in-memory document (existing tests). `createBrowserWorkspace(docs)` is the multi-document browser adapter used by workspace tests and the application shell.

`App` takes `{ workspace: WorkspaceAdapter }`. A document pane (`DocumentPane`) takes `{ host: HostAdapter }` and is the seam existing editor / GenUI / Outline tests use. `mountApp` constructs a one-document workspace around the given `HostAdapter` so those tests keep working without a one-window-per-document application path.

`PluginContext.documents` remains per-document (`snapshot` / `onDidChange` of that pane's document). `viewFile.generate` / `watch` bind to that pane's `host`, hence to its `id`.

## WatchView / Generate call shape (#29 / #24)

```
WatchView(documentId) / UnwatchView(documentId) / UndoView(documentId)
Generate(documentId, req, progress)
viewChanged: { id, exists, text, error, canUndo }
```

#21 only adds `documentId` to the same call/event boundary. #29 owns read/watch ordering: emit-first (the first snapshot does not wait for WatchFiles), then WatchFiles, then a catch-up snapshot once the watch is running, so there is no unwatched gap after the initial read. A watch that cannot start still returns its error on `WatchView`; the first snapshot already sent is not withdrawn. #24 owns one generation slot per `documentId` (concurrent slots across documents): settings, text, path and request are captured at start; a finishing run refuses to write if the document closed or its path changed; a cancelled close must not cancel the slot. #30 owns native Undo/Redo, not generation UX.

## Testing seams (approved)

- Browser: `WorkspaceAdapter` / `HostAdapter` / `App` / `DocumentPane` public operations.
- Go: real temp files through `workspaceHost` public methods (open/activate/close/save/save-as routing, identity preserved, cancel close). Native menus and file dialogs are verified after integration on macOS, not claimed here.
- #22: `NavSnapshot` / `SetFilter` / `PickFolder` / `RecentWorkspaces` and the left-pane tree, filter, splitter, crumbs, and welcome/recents. Opening a folder does not wait for WatchFiles or a full scan; `incomplete` stays true until indexing is ready. Native Open Folder / large-dir smoke is the verification owner.
- #23: `session.json` / `planSessionRestore` / `SetViewMode` / `PrepareQuit` for workspace roots, tab order, active document, view mode, missing-root/file issues, cancel-close vs explicit close, and quit flush-ack. Unsaved bytes stay MyGo recovery copies; Agent jobs are not replayed.
