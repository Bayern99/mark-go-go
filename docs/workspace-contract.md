# Workspace and document identity contract (#21)

Slice-specific. Downstream tickets (#22, #24, #30) consume this as the public Go/TS seam. Shared `docs/interfaces.md` is updated by #32 after smoke.

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
    ID        string         `json:"id"`
    Root      string         `json:"root"`     // empty = untitled window
    Name      string         `json:"name"`
    Kind      string         `json:"kind"`     // "folder" | "untitled"
    Documents []DocumentInfo `json:"documents"` // tab order
    ActiveID  string         `json:"activeId"`  // empty when no tabs
}

type DocumentInfo struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Path     string `json:"path"`
    Edited   bool   `json:"edited"`
    Conflict bool   `json:"conflict"`
    Deleted  bool   `json:"deleted"`
    ReadOnly bool   `json:"readOnly"`
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
Generate(ctx, documentId string, req GenerateRequest, progress *Channel[GenerateProgress]) error
GetSettings() (Settings, error)
SetSettings(s Settings) error

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
docState: DocumentInfo                // { id, name, path, edited, conflict, deleted, readOnly }
viewChanged: { id: string } & ViewSnapshot
navChanged: NavSnapshot               // incremental tree; status scanning|indexing|ready|error
navSearchReady: NavSearchResult       // incomplete is true until the index is ready
```

`workspaceChanged` is the source of truth for tab order, active tab, root, and per-tab badges. Background `docState` updates the matching tab without activating it.

## Routing (Open / Finder / menu)

1. If an open document is the same file (`path` equal or `os.SameFile`), activate it in its window and focus that window.
2. Else if one or more folder workspaces contain the file, open it as a tab in the workspace whose `root` is the deepest prefix.
3. Else open (or create) a folder workspace at the file's parent directory and add the tab there.

Opening a folder that is already a workspace root focuses that window. Parent and child folders may both be open; they still share one document session per file.

Untitled window: `root == ""`, `kind == "untitled"`. It holds untitled documents. Closing its last tab closes the window. A folder workspace with zero tabs stays open.

## First save / Save As migration

After a successful first save or Save As:

- The document keeps its `documentId`, content, version, view watch, and generation slot.
- The tab leaves the source window and joins the destination workspace that contains the new path (deepest existing folder workspace, else a new window at the parent directory).
- Other untitled documents in an untitled window stay there.
- If the destination path is already another open document, the save is rejected: no silent overwrite, no second session.

## Native commands

File menu Save / Save As / Duplicate / Rename / Move / Revert act on the **active document of the focused window** (MyGo document group). Close (`Cmd+W`) closes the **active document tab**, not the window. The window close button / File > Close Window runs `CloseWorkspace` (asks every dirty document; cancel keeps the window and every tab).

Dialogs use the workspace window as parent. Generation in progress for a document disables that document's Save As / Rename / Move; ordinary Save, edit, and tab switch stay available.

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

#21 only adds `documentId` to the same call/event boundary. #29 owns read/watch ordering: emit-first (the first snapshot does not wait for WatchFiles), then WatchFiles, then a catch-up snapshot once the watch is running, so there is no unwatched gap after the initial read. A watch that cannot start still returns its error on `WatchView`; the first snapshot already sent is not withdrawn. #24 owns multi-document generation isolation (one slot per `documentId`, concurrent slots across documents). #30 owns native Undo/Redo, not generation UX.

## Testing seams (approved)

- Browser: `WorkspaceAdapter` / `HostAdapter` / `App` / `DocumentPane` public operations.
- Go: real temp files through `workspaceHost` public methods (open/activate/close/save/save-as routing, identity preserved, cancel close). Native menus and file dialogs are verified after integration on macOS, not claimed here.
- #22: `NavSnapshot` / `SetFilter` / `PickFolder` / `RecentWorkspaces` and the left-pane tree, filter, splitter, crumbs, and welcome/recents. Opening a folder does not wait for WatchFiles or a full scan; `incomplete` stays true until indexing is ready. Native Open Folder / large-dir smoke is the verification owner.
