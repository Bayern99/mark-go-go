# Workspace and document identity contract (#21)

Slice-specific. Downstream tickets (#22, #24, #30) consume this as the public Go/TS seam. The consolidated, reader-facing reference is now `docs/interfaces.md` (#32); this file stays the normative seam contract for identities, routing and lifecycle, and the two must not diverge.

## Identities (distinct)

| Identity | Owner | Stable across |
|---|---|---|
| `documentId` | Go (`mygo.Document.ID()`) | path, tab, window, first save, Save As |
| `workspaceId` | Go (`workspaceHost`) | one root's navigation and document ownership |
| `windowId` (`WorkspaceSnapshot.id`) | Go (`workspaceHost`) | the shared native window and its cross-workspace tab group |
| `root` | folder path, empty string for an untitled workspace | not an identity; routing uses it |
| `path` | disk file, empty string while untitled | not an identity |
| `tab` | UI navigation item for one `documentId` in one window | not an identity |

A document adapter binds to one `documentId` for its lifetime. Asynchronous calls, events, view watches, and generation never retarget through the currently active tab.

## Go service `WorkspaceHost` (`mygo.BindAs("WorkspaceHost", host)`)

Document-level methods take `documentId`. Workspace-level methods act on the calling page's window. A window has one shared document group and one selected navigation root; selecting an empty root sets `activeId` to empty without discarding other roots' tabs.

```go
type WorkspaceSummary struct {
    ID   string `json:"id"`
    Root string `json:"root"`
    Name string `json:"name"`
    Kind string `json:"kind"`
}

type WorkspaceSnapshot struct {
    ID           string             `json:"id"`        // windowId
    Root         string             `json:"root"`      // selected root; empty = untitled
    Name         string             `json:"name"`
    Kind         string             `json:"kind"`      // "folder" | "untitled"
    Workspaces   []WorkspaceSummary `json:"workspaces"` // roots in this native window
    Documents    []DocumentInfo     `json:"documents"`  // one cross-workspace tab order
    ActiveID     string             `json:"activeId"`   // empty when the selected root has no tab
    RestoreError string             `json:"restoreError,omitempty"`
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
OpenWorkspace(ctx, root string) error                 // select existing or add root to this window
ActivateWorkspace(ctx, workspaceId string) error       // select navigation + its recent tab
CloseWorkspaceRoot(ctx, workspaceId string) (CloseResult, error) // close only this root
New(ctx) (DocumentInfo, error)                        // untitled document in this window
Duplicate(ctx, documentId string) (DocumentInfo, error)
Activate(ctx, documentId string) error
CloseDocument(ctx, documentId string) (CloseResult, error)
CloseWorkspace(ctx) (CloseResult, error)

// Document-level (never uses the active tab)
Load(ctx, documentId string) (Snapshot, error)
Update(ctx, documentId, text string) error
Flush(ctx, documentId, text string) error             // last undebounced editor text before Save/Close
FlushQuit(ctx, documentId, text string, attempt int) error // editor text from the current quit attempt
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
PrepareQuit(ctx, attempt int, failure string) error // report one window's flush for this quit attempt
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

Canceled native dialogs return `mygo.ErrCanceled` (Save/SaveAs/Rename/Move/Revert) or `CloseResult{Canceled: true}` (Close). Canceling a workspace close keeps its tabs and their unsaved recovery copies. The group applies Delete or Don't Save only when every document has confirmed. Completed saves still stand.

## Events (payloads include `id`)

Emitted to the native window that currently hosts the document. `workspaceChanged` aggregates every root's tabs. `navChanged` / `navSearchReady` are emitted only for the currently selected root; background indexing remains available when that root is selected.

```ts
workspaceChanged: WorkspaceSnapshot
docReloaded: Snapshot                 // { id, text, version, state }
docState: DocumentInfo                // { id, name, path, edited, conflict, deleted, readOnly, generating }
viewChanged: { id: string } & ViewSnapshot
navChanged: NavSnapshot               // incremental tree; status scanning|indexing|ready|error
navSearchReady: NavSearchResult       // incomplete is true until the index is ready
```

`workspaceChanged` is the source of truth for cross-workspace tab order, active tab, selected root, available roots, and per-tab badges. Visible tabs activate directly; the searchable overflow menu prioritizes recent documents, and `Ctrl+Tab` alternates with the most recent other open document. Background `docState` updates the matching tab without activating it.

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
3. Else add a folder workspace at the file's parent directory to the current window and add the tab there. Create a native window only when no usable app window exists.

Opening a folder that is already a workspace root selects it in its existing window. Parent and child folders may both be open; they still share one document session per file. MyGo's own duplicate-open focus emits a state change that the host uses to select the same `documentId` in the page. Roots and files are compared after resolving existing symlink prefixes (`canonicalPath`), so aliases of the same directory — including macOS `/tmp` versus `/private/tmp` — reuse the same workspace and keep `documentId`.

Untitled workspace: `root == ""`, `kind == "untitled"`. It holds untitled documents. Closing its last tab closes the window only when no other root shares it; otherwise the empty untitled root is removed. A folder workspace with zero tabs stays in the window. Opening the first folder replaces an empty Welcome root.

## First save / Save As migration

After a successful first save or Save As:

- The document keeps its `documentId`, content, version, view watch, and generation slot.
- The tab leaves its source root and joins the destination workspace that contains the new path (deepest existing folder workspace, else a new root at the parent directory) in the same native window.
- Other untitled documents in an untitled workspace stay there.
- If the destination path is already another open document, the save is rejected: no silent overwrite, no second session.

## Native commands

File menu Save / Save As / Duplicate / Rename / Move / Revert act on the **active document of the focused window** (MyGo document group). `Cmd+W` and File > Close call `performRole(RoleClose)`, which dispatches the captured active document through `Command` as `nativeCommand close`. Closing a tab selects a remaining tab from the selected root when available. An empty folder root keeps its window and shows the root name as the title. MyGo clears the active document and disables the File menu's document commands, so `Cmd+W` leaves the window open. Closing a standalone untitled window's last document destroys that window. Apps without document support keep the platform `performClose` behaviour.

The window close button runs `CloseWorkspace`, confirms every dirty document and closes the whole group. Cancel keeps the window and every tab. The Workspace menu's `CloseWorkspaceRoot(workspaceId)` confirms and closes only that root's documents and removes its navigation. The other roots stay in the window. Closing the last root leaves Welcome. The controlled MyGo `CloseDocuments` applies a root's discard decisions after every target has confirmed.

Quit sends `flushQuit` with the current attempt ID to each workspace window. Each window sends its editor text through `FlushQuit` and reports completion or an error through `PrepareQuit`. The host accepts these writes only from windows still awaiting acknowledgment in that attempt. It writes `session.json` after all windows flush successfully and continues quitting after the write succeeds. A flush error, missing acknowledgment, or write failure leaves the app open with a message. Choosing Quit again starts a new attempt that can proceed while an older IPC reply is outstanding. MyGo recovery copies hold the unsaved document bytes.

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
      "activeId": "documentId",
      "active": true
    }
  ]
}
```

- Written on layout changes (open, tab move, activate, root select/close, view mode) and once more at quit after every window acked `flushQuit`. Writes pause during recovery and restore. The optional `active` field marks the selected root. Restore adds all saved roots to one native window, reusing the window already opened by a recovery copy or a launch file when available.
- Only workspaces the user did not close are stored; an empty untitled window (Welcome) is not. Closing a tab or a workspace drops it, and cancel keeps the scene as it was.
- Unsaved text stays in MyGo's recovery copies, keyed by `documentId`; the session only names the tabs, their order, the active one and the view mode. Restore reuses a recovery document instead of opening its path again, and never starts a generation the user cancelled.
- A missing root or file becomes `WorkspaceSnapshot.restoreError` / `DocumentInfo.restoreError` and the other workspaces still restore. A file opened by this launch takes focus after the layout is placed.
- `flushQuit` / Save / Save As / Close ask each mounted pane for what it really holds: the text on screen (`editor.getMarkdown()`), else the core document state the host loaded. A pane that has not mounted an editor registers nothing, so a flush never turns an untouched, reloaded or remounted document into an empty buffer; a document the user did clear is delivered as `""` like any other edit. A document opened from disk gets a new MyGo id at restore, so the stored active tab is mapped to the id its document really has now.
- `bindFlush` registers a pane's raw-text reader. Save, Save As, Rename, Move, Revert, Duplicate, and Close use one `WorkspaceHost.Flush` call for the captured document. Closing a workspace uses one `Flush` call for each registered reader. A quit attempt uses one `WorkspaceHost.FlushQuit` call for each registered reader and includes its attempt ID. Documents without a registered reader produce no writes.
- Only a ready editor registers the reader; if an already-registered reader runs while its editor ref is briefly empty, it falls back to the host-loaded core text. A never-mounted pane registers nothing and never delivers `''`.

## MyGo document groups (controlled patch)

Pinned source: `github.com/egoist/mygo v0.7.4` vendored at `third_party/mygo` (`go.mod` replace). Patch adds:

- `docGroup` per window: ordered documents + active document. `documentFor(w)` returns the active document, never overwrites a single-window map with a later attach.
- `Documents.Attach(doc, win)`, `Documents.Detach(doc)`, `Documents.SetActive(doc)`, `Documents.ForWindowAll(w)`.
- `DocumentsOptions.DocumentFocused(doc)` reports MyGo's direct duplicate-open focus so the page selects the same session; that path does not call `Window`/`adopt` again.
- `Document.ID()`, `Document.CloseDocument()` (asks to save that document only; does not destroy a shared window).
- `Documents.CloseDocuments(docs)` confirms exactly the specified documents together, then closes only them without destroying their shared window. Cancellation keeps every target open.
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
  activateWorkspace(id: string): Promise<void>
  closeWorkspaceRoot(id: string): Promise<CloseResult>
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
