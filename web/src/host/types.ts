/**
 * Contract between the web frontend and whatever owns the document
 * (MyGo on the desktop host, an in-memory shim in a plain browser).
 *
 * Window, workspace root, path and tab are not this identity. A HostAdapter
 * is bound to one `id` for its lifetime; async work must not retarget
 * through the currently active tab. See docs/workspace-contract.md.
 */
export interface DocState {
  name: string
  path: string
  edited: boolean
  conflict: boolean
  deleted: boolean
  readOnly: boolean
}

/** One open document as the workspace lists it. `id` is stable across path and window. */
export interface DocumentInfo extends DocState {
  id: string
}

/** `version` increments every time content is (re)loaded from disk. */
export interface HostDocSnapshot {
  id: string
  text: string
  version: number
  state: DocumentInfo
}

export type WorkspaceKind = 'folder' | 'untitled'

export interface WorkspaceSnapshot {
  id: string
  root: string
  name: string
  kind: WorkspaceKind
  documents: DocumentInfo[]
  activeId: string
}

export interface CloseResult {
  closed: boolean
  canceled: boolean
}

export type NativeCommandName =
  | 'save'
  | 'saveAs'
  | 'close'
  | 'rename'
  | 'moveTo'
  | 'revert'
  | 'duplicate'
  | 'closeWorkspace'
  | 'undo'
  | 'redo'

/** Native File-menu / key-equivalent command, captured with the document id at click time. */
export interface NativeCommand {
  id: string
  command: NativeCommandName
}

export interface Disposable {
  dispose(): void
}

/**
 * The document's view file (`.<name>.md.genui.json` next to it), as the host
 * read it. `exists` is false when there is none, including for an untitled
 * document; `error` says why an existing file could not be read. `canUndo`
 * says the view the last generation replaced is kept, to go back to once.
 */
export interface ViewSnapshot {
  exists: boolean
  text: string
  error: string
  canUndo: boolean
}

/** What plugins may do with the view file: follow it, and go back once. */
export interface ViewFileSource {
  /**
   * Follow the view file on disk. `fn` gets the file as it is now (before
   * directory watch setup), again after the watch is running, then every
   * change, including the document moving. Disposing stops the host's watch
   * once no one else follows. #21 may add `documentId` / `id` on this seam;
   * keep this order and do not retarget through the active tab.
   */
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
  /**
   * Puts back the view the last generation replaced; the kept copy is then
   * gone. The view arrives through `watchView`. Rejects when there is none.
   */
  undoView(): Promise<void>
}

/** One heading section of the text sent to the agent, in UTF-16 offsets of that text. */
export interface SectionRef {
  heading: string
  occurrence: number
  start: number
  end: number
  /** Ranges of the section its hash leaves out, in order: its notes. */
  exclude?: Array<{ start: number; end: number }>
}

export interface GenerateRequest {
  /** The editor text at the moment Generate was clicked; the agent reads only this. */
  text: string
  /** `full` makes a new view; `update` replaces `replaceIds` in the current one. */
  mode: 'full' | 'update'
  /** The components an update replaces: those whose sources changed. */
  replaceIds?: string[]
  /** What the reader asked for; kept in the view file and offered again next time. */
  request?: string
  /** The format document and the catalog definition, for the agent's prompt. */
  format: string
  /** The text's heading sections, which the host hashes into the view's sources. */
  sections: SectionRef[]
}

export type GeneratePhase = 'starting' | 'thinking' | 'reading' | 'writing' | 'checking'

export interface GenerateProgress {
  phase: GeneratePhase
  text: string
}

export type GenerateFailureKind =
  | 'unsaved'
  | 'busy'
  | 'agent-not-found'
  | 'auth-required'
  | 'model-invalid'
  | 'agent-message'
  | 'invalid-output'
  | 'agent-error'
  | 'timeout'
  | 'cancelled'

/** Why a generation ended without a new view, and what the reader can do. */
export interface GenerateFailure {
  kind: GenerateFailureKind
  /** What happened, in a sentence or two. */
  message: string
  /** What the reader can do about it. */
  hint?: string
  /** The agent's own words, verbatim. */
  detail?: string
  /** The models the agent offers, for `model-invalid`. */
  models?: string[]
}

export interface GenerationSettings {
  /** The model the agent is asked to use; empty leaves the agent's own choice. */
  model: string
  /** The agent's command, run as `<agentCommand> acp`. */
  agentCommand: string
  /** Candidate models cached from the last ACP `session/new` handshake, or defaults. */
  models?: string[]
}

/**
 * Making a view with the reader's agent. The host writes the view file when
 * the run succeeds, so the new view arrives through `watchView`; a failed
 * or cancelled run leaves the file as it was.
 */
export interface ViewGenerator {
  /**
   * Runs one generation, reporting progress, until it is done. Rejects with
   * a `GenerationError` whose failure tells the reader what happened.
   * Aborting `signal` cancels the run, which rejects as `cancelled`.
   */
  generate(
    req: GenerateRequest,
    onProgress: (progress: GenerateProgress) => void,
    signal: AbortSignal,
  ): Promise<void>
  getSettings(): Promise<GenerationSettings>
  setSettings(settings: GenerationSettings): Promise<void>
}

export interface HostAdapter extends ViewFileSource, ViewGenerator {
  /** Stable document identity; never retargets through the active tab. */
  readonly id: string
  /** Content of this document. */
  load(): Promise<HostDocSnapshot>
  /** Full Markdown text after an edit. */
  update(text: string): Promise<void>
  /** Last undebounced editor text, before Save/Close/Save As of this document. */
  flush(text: string): Promise<void>
  save(): Promise<void>
  saveAs(): Promise<void>
  rename(): Promise<void>
  moveTo(): Promise<void>
  revert(): Promise<void>
  /** An external change was reloaded by MyGo. */
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  /** edited / conflict / name changes for this document. */
  onState(fn: (state: DocumentInfo) => void): Disposable
  /**
   * The page registers how to read this document's current editor Markdown so
   * native commands can flush this id, not the active tab.
   */
  bindFlush?(getText: () => string): Disposable
}

export interface WorkspaceAdapter {
  snapshot(): Promise<WorkspaceSnapshot>
  subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable
  open(path: string): Promise<DocumentInfo>
  openWorkspace(root: string): Promise<void>
  newDocument(): Promise<DocumentInfo>
  duplicate(id: string): Promise<DocumentInfo>
  activate(id: string): Promise<void>
  closeDocument(id: string, text?: string): Promise<CloseResult>
  closeWorkspace(): Promise<CloseResult>
  /** Permanently bound to `id` for as long as this window hosts it. */
  document(id: string): HostAdapter
}
